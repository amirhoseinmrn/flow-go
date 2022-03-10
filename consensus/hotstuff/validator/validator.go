package validator

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/packer"
)

// Validator is responsible for validating QC, Block and Vote
type Validator struct {
	committee hotstuff.Committee
	forks     hotstuff.ForksReader
	verifier  hotstuff.Verifier
}

var _ hotstuff.Validator = (*Validator)(nil)

// New creates a new Validator instance
func New(
	committee hotstuff.Committee,
	forks hotstuff.ForksReader,
	verifier hotstuff.Verifier,
) *Validator {
	return &Validator{
		committee: committee,
		forks:     forks,
		verifier:  verifier,
	}
}

// ValidateQC validates the QC
// qc - the qc to be validated
// block - the block that the qc is pointing to
func (v *Validator) ValidateQC(qc *flow.QuorumCertificate, block *model.Block) error {
	if qc.BlockID != block.BlockID {
		// Sanity check! Failing indicates a bug in the higher-level logic
		return fmt.Errorf("qc.BlockID %s doesn't match block's ID %s", qc.BlockID, block.BlockID)
	}
	if qc.View != block.View { // check view
		return newInvalidBlockError(block, fmt.Errorf("qc's View %d doesn't match referenced block's View %d", qc.View, block.View))
	}

	// Retrieve full Identities of all legitimate consensus participants and the Identities of the qc's signers
	// IdentityList returned by hotstuff.Committee contains only legitimate consensus participants for the specified block (must have positive stake)
	allParticipants, err := v.committee.Identities(block.BlockID)
	if err != nil {
		return fmt.Errorf("could not get consensus participants for block %s: %w", block.BlockID, err)
	}

	signerIndices, err := packer.DecodeSignerIndices(qc.SignerIndices, len(allParticipants))
	if err != nil {
		return newInvalidBlockError(block, fmt.Errorf("qc.SignerIndices is invalid: %w", err))
	}

	signers, err := filterByIndices(allParticipants, signerIndices)
	if err != nil {
		return newInvalidBlockError(block, fmt.Errorf("could not find signers by indices: %w", err))
	}

	// determine whether signers reach minimally required stake threshold for consensus
	threshold := hotstuff.ComputeStakeThresholdForBuildingQC(allParticipants.TotalStake()) // compute required stake threshold
	if signers.TotalStake() < threshold {
		return newInvalidBlockError(block, fmt.Errorf("qc signers have insufficient stake of %d (required=%d)", signers.TotalStake(), threshold))
	}

	// verify whether the signature bytes are valid for the QC in the context of the protocol state
	err = v.verifier.VerifyQC(signers, qc.SigData, block)
	if err != nil {
		// Theoretically, `VerifyQC` could also return a `model.InvalidSignerError`. However,
		// for the time being, we assume that _every_ HotStuff participant is also a member of
		// the random beacon committee. Consequently, `InvalidSignerError` should not occur atm.
		// TODO: if the random beacon committee is a strict subset of the HotStuff committee,
		//       we expect `model.InvalidSignerError` here during normal operations.
		switch {
		case errors.Is(err, model.ErrInvalidFormat):
			return newInvalidBlockError(block, fmt.Errorf("QC's  signature data has an invalid structure: %w", err))
		case errors.Is(err, model.ErrInvalidSignature):
			return newInvalidBlockError(block, fmt.Errorf("QC contains invalid signature(s): %w", err))
		default:
			return fmt.Errorf("cannot verify qc's aggregated signature (qc.BlockID: %x): %w", qc.BlockID, err)
		}
	}

	return nil
}

func filterByIndices(identities flow.IdentityList, indices []int) (flow.IdentityList, error) {
	list := make([]*flow.Identity, 0, len(identities))
	for _, index := range indices {
		if index > len(identities)-1 {
			return nil, fmt.Errorf("signer index %v is out of range %v", index, len(identities)-1)
		}
		list = append(list, identities[index])
	}
	return list, nil
}

// ValidateProposal validates the block proposal
// A block is considered as valid if it's a valid extension of existing forks.
// Note it doesn't check if it's conflicting with finalized block
func (v *Validator) ValidateProposal(proposal *model.Proposal) error {
	qc := proposal.Block.QC
	block := proposal.Block

	// validate the proposer's vote and get his identity
	_, err := v.ValidateVote(proposal.ProposerVote(), block)
	if model.IsInvalidVoteError(err) {
		return newInvalidBlockError(block, fmt.Errorf("invalid proposer signature: %w", err))
	}
	if err != nil {
		return fmt.Errorf("error verifying leader signature for block %x: %w", block.BlockID, err)
	}

	// check the proposer is the leader for the proposed block's view
	leader, err := v.committee.LeaderForView(block.View)
	if err != nil {
		return fmt.Errorf("error determining leader for block %x: %w", block.BlockID, err)
	}
	if leader != block.ProposerID {
		return newInvalidBlockError(block, fmt.Errorf("proposer %s is not leader (%s) for view %d", block.ProposerID, leader, block.View))
	}

	// check that we have the parent for the proposal
	parent, found := v.forks.GetBlock(qc.BlockID)
	if !found {
		// Forks is _allowed_ to (but obliged to) prune blocks whose view is below the newest finalized block.
		if qc.View >= v.forks.FinalizedView() {
			// If the parent block is equal or above the finalized view, then Forks should have it. Otherwise, we are missing a block!
			return model.MissingBlockError{View: qc.View, BlockID: qc.BlockID}
		}

		// Forks has already pruned the parent block. I.e., we can't validate that the qc matches
		// a known (and valid) parent. Nevertheless, we just store this block, because there might already
		// exists children of this block, which we could receive later. However, we know for sure that the
		// block's fork cannot keep growing anymore because it conflicts with a finalized block.
		// TODO: note other components will expect Validator has validated, and might re-validate it,
		return model.ErrUnverifiableBlock
	}

	// validate QC - keep the most expensive the last to check
	return v.ValidateQC(qc, parent)
}

// ValidateVote validates the vote and returns the identity of the voter who signed
// vote - the vote to be validated
// block - the voting block. Assuming the block has been validated.
func (v *Validator) ValidateVote(vote *model.Vote, block *model.Block) (*flow.Identity, error) {
	// block hash must match
	if vote.BlockID != block.BlockID {
		// Sanity check! Failing indicates a bug in the higher-level logic
		return nil, fmt.Errorf("wrong block ID. expected (%s), got (%d)", block.BlockID, vote.BlockID)
	}
	// view must match with the block's view
	if vote.View != block.View {
		return nil, newInvalidVoteError(vote, fmt.Errorf("vote's view %d is inconsistent with referenced block (view %d)", vote.View, block.View))
	}

	voter, err := v.committee.Identity(block.BlockID, vote.SignerID)
	if model.IsInvalidSignerError(err) {
		return nil, newInvalidVoteError(vote, err)
	}
	if err != nil {
		return nil, fmt.Errorf("error retrieving voter Identity at block %x: %w", block.BlockID, err)
	}

	// check whether the signature data is valid for the vote in the hotstuff context
	err = v.verifier.VerifyVote(voter, vote.SigData, block)
	if err != nil {
		// Theoretically, `VerifyVote` could also return a `model.InvalidSignerError`. However,
		// for the time being, we assume that _every_ HotStuff participant is also a member of
		// the random beacon committee. Consequently, `InvalidSignerError` should not occur atm.
		// TODO: if the random beacon committee is a strict subset of the HotStuff committee,
		//       we expect `model.InvalidSignerError` here during normal operations.
		if errors.Is(err, model.ErrInvalidFormat) || errors.Is(err, model.ErrInvalidSignature) {
			return nil, newInvalidVoteError(vote, err)
		}
		return nil, fmt.Errorf("cannot verify signature for vote (%x): %w", vote.ID(), err)
	}

	return voter, nil
}

func newInvalidBlockError(block *model.Block, err error) error {
	return model.InvalidBlockError{
		BlockID: block.BlockID,
		View:    block.View,
		Err:     err,
	}
}

func newInvalidVoteError(vote *model.Vote, err error) error {
	return model.InvalidVoteError{
		VoteID: vote.ID(),
		View:   vote.View,
		Err:    err,
	}
}
