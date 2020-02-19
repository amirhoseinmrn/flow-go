package coldstuff

import (
	"errors"
	"fmt"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/consensus/coldstuff/round"
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/engine/consensus/consensus"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/protocol"
	"github.com/dapperlabs/flow-go/utils/logging"
)

type ColdStuff interface {
	consensus.HotStuff

	SubmitCommit(commit *Commit)
}

// internal implementation of ColdStuff
type coldStuff struct {
	log       zerolog.Logger
	state     protocol.State
	me        module.Local
	round     *round.Round
	comms     Communicator
	builder   module.Builder
	finalizer module.Finalizer

	// round config
	interval time.Duration
	timeout  time.Duration

	// incoming consensus entities
	proposals chan *flow.Header
	votes     chan *Vote
	commits   chan *Commit

	// stops the consent loop
	done chan struct{}
}

func New(
	log zerolog.Logger,
	state protocol.State,
	me module.Local,
	comms Communicator,
	builder module.Builder,
	finalizer module.Finalizer,
	interval time.Duration,
	timeout time.Duration,
) (ColdStuff, error) {
	cold := coldStuff{
		log:       log,
		me:        me,
		state:     state,
		builder:   builder,
		comms:     comms,
		interval:  interval,
		timeout:   timeout,
		proposals: make(chan *flow.Header, 1),
		votes:     make(chan *Vote, 1),
		commits:   make(chan *Commit, 1),
	}

	return &cold, nil
}

func (e *coldStuff) Start() (exit func(), done <-chan struct{}) {
	done = e.done
	exit = func() {
		close(e.done)
	}

	return
}

func (e *coldStuff) SubmitProposal(proposal *flow.Header, parentView uint64) {
	// Ignore HotStuff-only values
	_ = parentView

	e.proposals <- proposal
}

func (e *coldStuff) SubmitVote(originID, blockID flow.Identifier, view uint64, sig crypto.Signature) {
	// Ignore HotStuff-only values
	_ = view
	_ = sig

	e.votes <- &Vote{
		OriginID: originID,
		BlockID:  blockID,
	}
}

func (e *coldStuff) SubmitCommit(commit *Commit) {
	e.commits <- commit
}

func (e *coldStuff) loop() error {

	localID := e.me.NodeID()
	log := e.log.With().Hex("local_id", logging.ID(localID)).Logger()

ConsentLoop:
	for {

		var err error
		e.round, err = round.New(e.state, e.me)
		if err != nil {
			return fmt.Errorf("could not initialize round: %w", err)
		}

		// calculate the time at which we can generate the next valid block
		limit := e.round.Parent().Timestamp.Add(e.interval)

		select {
		case <-e.done:
			return nil
		case <-time.After(time.Until(limit)):
			if e.round.Leader().NodeID == localID {
				// if we are the leader, we:
				// 1) send a block proposal
				// 2) wait for sufficient block votes
				// 3) send a block commit

				err = e.sendProposal()
				if err != nil {
					log.Error().Err(err).Msg("could not send proposal")
					continue ConsentLoop
				}

				err = e.waitForVotes()
				if err != nil {
					log.Error().Err(err).Msg("could not receive votes")
					continue ConsentLoop
				}

				err = e.sendCommit()
				if err != nil {
					log.Error().Err(err).Msg("could not send commit")
					continue ConsentLoop
				}

			} else {
				// if we are not the leader, we:
				// 1) wait for a block proposal
				// 2) vote on the block proposal
				// 3) wait for a block commit

				err = e.waitForProposal()
				if err != nil {
					log.Error().Err(err).Msg("could not receive proposal")
					continue ConsentLoop
				}

				err = e.voteOnProposal()
				if err != nil {
					log.Error().Err(err).Msg("could not vote on proposal")
					continue ConsentLoop
				}

				err = e.waitForCommit()
				if err != nil {
					log.Error().Err(err).Msg("could not receive commit")
					continue ConsentLoop
				}
			}

			// regardless of path, if we successfully reach here, we finished a
			// full successful consensus round and can commit the current
			// block candidate
			err = e.commitCandidate()
			if err != nil {
				log.Error().Err(err).Msg("could not commit candidate")
				continue
			}
		}
	}
}

func (e *coldStuff) sendProposal() error {
	log := e.log.With().
		Str("action", "send_proposal").
		Logger()

	// get our own ID to tally our stake
	myIdentity, err := e.state.Final().Identity(e.me.NodeID())
	if err != nil {
		return fmt.Errorf("could not get own current ID: %w", err)
	}

	// define the block header build function
	setProposer := func(header *flow.Header) {
		header.ProposerID = myIdentity.NodeID
	}

	// get the payload for the next hash
	candidate, err := e.builder.BuildOn(e.round.Parent().ID(), setProposer)
	if err != nil {
		return fmt.Errorf("could not build on parent: %w", err)
	}

	log = log.With().
		Uint64("number", candidate.Height).
		Hex("candidate_id", logging.Entity(candidate)).
		Logger()

	// TODO this should be done by builder
	// store the block proposal
	//err = e.headers.Store(candidate)
	//if err != nil {
	//	return fmt.Errorf("could not store candidate: %w", err)
	//}

	// cache the candidate block
	e.round.Propose(candidate)

	// send the block proposal
	err = e.comms.BroadcastProposal(candidate)
	if err != nil {
		return fmt.Errorf("could not submit proposal: %w", err)
	}

	// add our own vote to the engine
	e.round.Tally(myIdentity.NodeID, myIdentity.Stake)

	log.Info().Msg("block proposal sent")

	return nil

}

// waitForVotes will wait for received votes and validate them until we have
// reached a quorum on the currently cached block candidate. It assumse we are
// the leader and will timeout after the configured timeout.
func (e *coldStuff) waitForVotes() error {

	candidate := e.round.Candidate()

	log := e.log.With().
		Uint64("number", candidate.Height).
		Hex("candidate_id", logging.Entity(candidate)).
		Str("action", "wait_votes").
		Logger()

	for {
		select {

		// process each vote that we receive
		case w := <-e.votes:
			voterID, voteID := w.OriginID, w.BlockID

			// discard votes by double voters
			voted := e.round.Voted(voterID)
			if voted {
				log.Warn().Hex("voter_id", voterID[:]).Msg("invalid double vote")
				continue
			}

			// discard votes by self
			if voterID == e.me.NodeID() {
				log.Warn().Hex("voter_id", voterID[:]).Msg("invalid self-vote")
				continue
			}

			// discard votes that are not by staked consensus nodes
			id, err := e.state.Final().Identity(voterID)
			if errors.Is(err, badger.ErrKeyNotFound) {
				log.Warn().Hex("voter_id", voterID[:]).Msg("vote by unknown node")
				continue
			}
			if err != nil {
				log.Error().Err(err).Hex("voter_id", voterID[:]).Msg("could not verify voter ID")
				break
			}
			if id.Role != flow.RoleConsensus {
				log.Warn().Str("role", id.Role.String()).Msg("vote by non-consensus node")
				continue
			}

			// discard votes that are on the wrong candidate
			if voteID != candidate.ID() {
				log.Warn().Hex("vote_id", voteID[:]).Msg("invalid candidate vote")
				continue
			}

			// tally the voting stake of the voter ID
			e.round.Tally(voterID, id.Stake)
			votes := e.round.Votes()

			log.Info().Uint64("vote_quorum", e.round.Quorum()).Uint64("vote_count", votes).Msg("block vote received")

			// if we reached the quorum, continue to next step
			if votes >= e.round.Quorum() {
				log.Info().Msg("sufficient votes received")
				return nil
			}

		case <-time.After(e.timeout):
			return errors.New("timed out while waiting for votes")
		}
	}
}

// sendCommit is called after we have successfully waited for a vote quorum. It
// will send a block commit message with the block hash that instructs all nodes
// to forward their blockchain and start a new consensus round.
func (e *coldStuff) sendCommit() error {

	candidate := e.round.Candidate()

	log := e.log.With().
		Uint64("number", candidate.Height).
		Hex("candidate_id", logging.Entity(candidate)).
		Str("action", "send_commit").
		Logger()

	// send a commit for the cached block hash
	commit := &Commit{
		BlockID: candidate.ID(),
	}
	err := e.comms.BroadcastCommit(commit)
	if err != nil {
		return fmt.Errorf("could not submit commit: %w", err)
	}

	log.Info().Msg("block commit sent")

	return nil
}

// waitForProposal waits for a block proposal to be received and validates it in
// a number of ways. It should be called at the beginning of a round if we are
// not the leader. It will timeout if no proposal was received by the leader
// after the configured timeout.
func (e *coldStuff) waitForProposal() error {
	log := e.log.With().
		Str("action", "wait_proposal").
		Logger()

	for {
		select {

		// process each proposal we receive
		case candidate := <-e.proposals:
			proposerID := candidate.ProposerID

			// TODO this should be done automatically by CCL
			// store every proposal
			//err := e.headers.Store(candidate)
			//if err != nil {
			//	log.Error().Err(err).Msg("could not store candidate")
			//	continue
			//}

			// discard proposals by non-leaders
			leaderID := e.round.Leader().NodeID
			if proposerID != leaderID {
				log.Warn().Hex("candidate_leader", proposerID[:]).Hex("expected_leader", leaderID[:]).Msg("invalid leader")
				continue
			}

			// discard proposals with the wrong height
			number := e.round.Parent().Height + 1
			if candidate.Height != e.round.Parent().Height+1 {
				log.Warn().Uint64("candidate_height", candidate.Height).Uint64("expected_height", number).Msg("invalid height")
				continue
			}

			// discard proposals with the wrong parent
			parentID := e.round.Parent().ID()
			if candidate.ParentID != parentID {
				log.Warn().Hex("candidate_parent", candidate.ParentID[:]).Hex("expected_parent", parentID[:]).Msg("invalid parent")
				continue
			}

			// discard proposals with invalid timestamp
			limit := e.round.Parent().Timestamp.Add(e.interval)
			if candidate.Timestamp.Before(limit) {
				log.Warn().Time("candidate_timestamp", candidate.Timestamp).Time("candidate_limit", limit).Msg("invalid timestamp")
				continue
			}

			// cache the candidate for the round
			e.round.Propose(candidate)

			log.Info().
				Uint64("number", candidate.Height).
				Hex("candidate_id", logging.Entity(candidate)).
				Msg("block proposal received")

			return nil

		case <-time.After(e.timeout):
			return errors.New("timed out while waiting for proposal")
		}
	}
}

// voteOnProposal is called after we have received a new block proposal as
// non-leader. It assumes that all checks were already done and simply sends a
// vote to the leader of the current round that accepts the candidate block.
func (e *coldStuff) voteOnProposal() error {

	candidate := e.round.Candidate()

	log := e.log.With().
		Uint64("number", candidate.Height).
		Hex("candidate_id", logging.Entity(candidate)).
		Str("action", "send_vote").
		Logger()

	// send vote for proposal to leader
	vote := &Vote{
		BlockID: candidate.ID(),
	}
	err := e.comms.SendVote(vote, e.round.Leader().NodeID)
	if err != nil {
		return fmt.Errorf("could not submit vote: %w", err)
	}

	log.Info().Msg("block vote sent")

	return nil
}

// waitForCommit is called after we have submitted our vote for the leader and
// awaits his confirmation that we can commit the block. The confirmation is
// only sent once a quorum of votes was received by the leader.
func (e *coldStuff) waitForCommit() error {

	candidate := e.round.Candidate()

	log := e.log.With().
		Uint64("number", candidate.Height).
		Hex("candidate_id", logging.Entity(candidate)).
		Str("action", "wait_commit").
		Logger()

	for {
		select {
		case w := <-e.commits:
			committerID, commitID := w.OriginID, w.BlockID

			// discard commits not from leader
			leaderID := e.round.Leader().NodeID
			if committerID != leaderID {
				log.Warn().Hex("commit_leader", committerID[:]).Hex("expected_leader", leaderID[:]).Msg("invalid commit leader")
				continue
			}

			// discard commits not for candidate hash
			if commitID != candidate.ID() {
				log.Warn().Hex("commit_id", commitID[:]).Msg("invalid commit hash")
				continue
			}

			log.Info().Msg("block commit received")

			return nil

		case <-time.After(e.timeout):
			return errors.New("timed out while waiting for commit")
		}
	}
}

// commitCandidate commits the current block candidate to the blockchain and
// starts the next consensus round.
func (e *coldStuff) commitCandidate() error {

	candidate := e.round.Candidate()

	log := e.log.With().
		Uint64("number", candidate.Height).
		Hex("candidate_id", logging.Entity(candidate)).
		Str("action", "exec_commit").
		Logger()

	// TODO extend should be done automatically in CCL
	// TODO finalize+clean+expulse should be done in Finalizer callback
	//// commit the block to our chain state
	//err := e.state.Mutate().Extend(candidate.ID())
	//if err != nil {
	//	return fmt.Errorf("could not extend state: %w", err)
	//}
	//
	//// finalize the state
	//err = e.state.Mutate().Finalize(candidate.ID())
	//if err != nil {
	//	return fmt.Errorf("could not finalize state: %w", err)
	//}
	//
	//// hand the finalized block to expulsion engine to spread to all nodes
	//e.exp.Submit(e.round.Leader().NodeID, e.round.Candidate())
	//
	//// make sure all pending ambiguous state is now cleared up
	//err = e.cleaner.CleanAfter(candidate.ID())
	//if err != nil {
	//	return fmt.Errorf("could not drop ambiguous state: %w", err)
	//}

	log.Info().Msg("block candidate committed")

	return nil
}
