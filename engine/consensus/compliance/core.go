// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package compliance

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/attribute"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/compliance"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/state"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/logging"
)

// Core contains the central business logic for the main consensus' compliance engine.
// It is responsible for handling communication for the embedded consensus algorithm.
// NOTE: Core is designed to be non-thread safe and cannot be used in concurrent environment
// user of this object needs to ensure single thread access.
type Core struct {
	log               zerolog.Logger // used to log relevant actions with context
	config            compliance.Config
	metrics           module.EngineMetrics
	tracer            module.Tracer
	mempool           module.MempoolMetrics
	complianceMetrics module.ComplianceMetrics
	cleaner           storage.Cleaner
	headers           storage.Headers
	payloads          storage.Payloads
	state             protocol.MutableState
	pending           module.PendingBlockBuffer // pending block cache
	sync              module.BlockRequester
	hotstuff          module.HotStuff
	voteAggregator    hotstuff.VoteAggregator
}

// NewCore instantiates the business logic for the main consensus' compliance engine.
func NewCore(
	log zerolog.Logger,
	collector module.EngineMetrics,
	tracer module.Tracer,
	mempool module.MempoolMetrics,
	complianceMetrics module.ComplianceMetrics,
	cleaner storage.Cleaner,
	headers storage.Headers,
	payloads storage.Payloads,
	state protocol.MutableState,
	pending module.PendingBlockBuffer,
	sync module.BlockRequester,
	voteAggregator hotstuff.VoteAggregator,
	opts ...compliance.Opt,
) (*Core, error) {

	config := compliance.DefaultConfig()
	for _, apply := range opts {
		apply(&config)
	}

	e := &Core{
		log:               log.With().Str("compliance", "core").Logger(),
		config:            config,
		metrics:           collector,
		tracer:            tracer,
		mempool:           mempool,
		complianceMetrics: complianceMetrics,
		cleaner:           cleaner,
		headers:           headers,
		payloads:          payloads,
		state:             state,
		pending:           pending,
		sync:              sync,
		hotstuff:          nil, // use `WithConsensus`
		voteAggregator:    voteAggregator,
	}

	e.mempool.MempoolEntries(metrics.ResourceProposal, e.pending.Size())

	return e, nil
}

// OnBlockProposal handles incoming block proposals.
func (c *Core) OnBlockProposal(originID flow.Identifier, proposal *messages.BlockProposal, inBlockRangeResponse bool) error {
	block := proposal.Block.ToInternal()
	header := block.Header

	var traceID string

	span, _, isSampled := c.tracer.StartBlockSpan(context.Background(), header.ID(), trace.CONCompOnBlockProposal)
	if isSampled {
		span.SetAttributes(
			attribute.Int64("view", int64(header.View)),
			attribute.String("origin_id", originID.String()),
			attribute.String("proposer", header.ProposerID.String()),
		)
		traceID = span.SpanContext().TraceID().String()
	}
	defer span.End()

	log := c.log.With().
		Hex("origin_id", originID[:]).
		Str("chain_id", header.ChainID.String()).
		Uint64("block_height", header.Height).
		Uint64("block_view", header.View).
		Hex("block_id", logging.Entity(header)).
		Hex("parent_id", header.ParentID[:]).
		Hex("payload_hash", header.PayloadHash[:]).
		Time("timestamp", header.Timestamp).
		Hex("proposer", header.ProposerID[:]).
		Hex("parent_signer_indices", header.ParentVoterIndices).
		Str("traceID", traceID). // traceID is used to connect logs to traces
		Logger()
	log.Info().Msg("block proposal received")

	// first, we reject all blocks that we don't need to process:
	// 1) blocks already in the cache; they will already be processed later
	// 2) blocks already on disk; they were processed and await finalization

	// ignore proposals that are already cached
	_, cached := c.pending.ByID(header.ID())
	if cached {
		log.Debug().Msg("skipping already cached proposal")
		return nil
	}

	// ignore proposals that were already processed
	_, err := c.headers.ByBlockID(header.ID())
	if err == nil {
		log.Debug().Msg("skipping already processed proposal")
		return nil
	}
	if !errors.Is(err, storage.ErrNotFound) {
		return fmt.Errorf("could not check proposal: %w", err)
	}

	// ignore proposals which are too far ahead of our local finalized state
	// instead, rely on sync engine to catch up finalization more effectively, and avoid
	// large subtree of blocks to be cached.
	final, err := c.state.Final().Head()
	if err != nil {
		return fmt.Errorf("could not get latest finalized header: %w", err)
	}
	if header.Height > final.Height && header.Height-final.Height > c.config.SkipNewProposalsThreshold {
		log.Debug().
			Uint64("final_height", final.Height).
			Msg("dropping block too far ahead of locally finalized height")
		return nil
	}

	// there are two possibilities if the proposal is neither already pending
	// processing in the cache, nor has already been processed:
	// 1) the proposal is unverifiable because the parent is unknown
	// => we cache the proposal
	// 2) the proposal is connected to finalized state through an unbroken chain
	// => we verify the proposal and forward it to hotstuff if valid

	// if the parent is a pending block (disconnected from the incorporated state), we cache this block as well.
	// we don't have to request its parent block or its ancestor again, because as a
	// pending block, its parent block must have been requested.
	// if there was problem requesting its parent or ancestors, the sync engine's forward
	// syncing with range requests for finalized blocks will request for the blocks.
	_, found := c.pending.ByID(header.ParentID)
	if found {

		// add the block to the cache
		_ = c.pending.Add(originID, block)
		c.mempool.MempoolEntries(metrics.ResourceProposal, c.pending.Size())

		return nil
	}

	// if the proposal is connected to a block that is neither in the cache, nor
	// in persistent storage, its direct parent is missing; cache the proposal
	// and request the parent
	_, err = c.headers.ByBlockID(header.ParentID)
	if errors.Is(err, storage.ErrNotFound) {

		_ = c.pending.Add(originID, block)

		c.mempool.MempoolEntries(metrics.ResourceProposal, c.pending.Size())

		log.Debug().Msg("requesting missing parent for proposal")

		c.sync.RequestBlock(header.ParentID, header.Height-1)

		return nil
	}
	if err != nil {
		return fmt.Errorf("could not check parent: %w", err)
	}

	// At this point, we should be able to connect the proposal to the finalized
	// state and should process it to see whether to forward to hotstuff or not.
	// processBlockAndDescendants is a recursive function. Here we trace the
	// execution of the entire recursion, which might include processing the
	// proposal's pending children. There is another span within
	// processBlockProposal that measures the time spent for a single proposal.
	err = c.processBlockAndDescendants(block, inBlockRangeResponse)
	c.mempool.MempoolEntries(metrics.ResourceProposal, c.pending.Size())
	if err != nil {
		return fmt.Errorf("could not process block proposal: %w", err)
	}

	// most of the heavy database checks are done at this point, so this is a
	// good moment to potentially kick-off a garbage collection of the DB
	// NOTE: this is only effectively run every 1000th calls, which corresponds
	// to every 1000th successfully processed block
	c.cleaner.RunGC()

	return nil
}

// processBlockAndDescendants is a recursive function that processes a block and
// its pending proposals for its children. By induction, any children connected
// to a valid proposal are validly connected to the finalized state and can be
// processed as well.
func (c *Core) processBlockAndDescendants(block *flow.Block, inRangeBlockResponse bool) error {
	blockID := block.ID()

	// process block itself
	err := c.processBlockProposal(block, inRangeBlockResponse)
	// child is outdated by the time we started processing it
	// => node was probably behind and is catching up. Log as warning
	if engine.IsOutdatedInputError(err) {
		c.log.Info().Msg("dropped processing of abandoned fork; this might be an indicator that the node is slightly behind")
		return nil
	}
	// the block is invalid; log as error as we desire honest participation
	// ToDo: potential slashing
	if engine.IsInvalidInputError(err) {
		c.log.Warn().
			Err(err).
			Msg("received invalid block from other node (potential slashing evidence?)")
		return nil
	}
	if err != nil {
		// unexpected error: potentially corrupted internal state => abort processing and escalate error
		return fmt.Errorf("failed to process block %x: %w", blockID, err)
	}

	// process all children
	// do not break on invalid or outdated blocks as they should not prevent us
	// from processing other valid children
	children, has := c.pending.ByParentID(blockID)
	if !has {
		return nil
	}
	for _, child := range children {
		cpr := c.processBlockAndDescendants(child.Message, inRangeBlockResponse)
		if cpr != nil {
			// unexpected error: potentially corrupted internal state => abort processing and escalate error
			return cpr
		}
	}

	// drop all of the children that should have been processed now
	c.pending.DropForParent(blockID)

	return nil
}

// processBlockProposal processes the given block proposal. The proposal must connect to
// the finalized state.
func (c *Core) processBlockProposal(block *flow.Block, inRangeBlockResponse bool) error {
	header := block.Header

	startTime := time.Now()
	defer c.complianceMetrics.BlockProposalDuration(time.Since(startTime))

	span, ctx, isSampled := c.tracer.StartBlockSpan(context.Background(), header.ID(), trace.ConCompProcessBlockProposal)
	if isSampled {
		span.SetAttributes(
			attribute.String("proposer", header.ProposerID.String()),
		)
	}
	defer span.End()

	log := c.log.With().
		Str("chain_id", header.ChainID.String()).
		Uint64("block_height", header.Height).
		Uint64("block_view", header.View).
		Hex("block_id", logging.Entity(header)).
		Hex("parent_id", header.ParentID[:]).
		Hex("payload_hash", header.PayloadHash[:]).
		Time("timestamp", header.Timestamp).
		Hex("proposer", header.ProposerID[:]).
		Hex("parent_signer_indices", header.ParentVoterIndices).
		Logger()
	log.Info().Msg("processing block proposal")

	// see if the block is a valid extension of the protocol state
	err := c.state.Extend(ctx, block)
	// if the block proposes an invalid extension of the protocol state, then the block is invalid
	if state.IsInvalidExtensionError(err) {
		return engine.NewInvalidInputErrorf("invalid extension of protocol state (block: %x, height: %d): %w",
			header.ID(), header.Height, err)
	}
	// protocol state aborted processing of block as it is on an abandoned fork: block is outdated
	if state.IsOutdatedExtensionError(err) {
		return engine.NewOutdatedInputErrorf("outdated extension of protocol state: %w", err)
	}
	if err != nil {
		return fmt.Errorf("could not extend protocol state (block: %x, height: %d): %w", header.ID(), header.Height, err)
	}

	// retrieve the parent
	parent, err := c.headers.ByBlockID(header.ParentID)
	if err != nil {
		return fmt.Errorf("could not retrieve proposal parent: %w", err)
	}

	// submit the model to hotstuff for processing
	log.Info().Msg("forwarding block proposal to hotstuff")

	// when the block is in range response, we should wait for hotstuff to finish processing the block,
	// otherwise processing the next block might fail because the current block hasn't been added
	// to protocol state yet.
	if inRangeBlockResponse {
		select {
		case <-c.hotstuff.SubmitProposal(header, parent.View):
			break
		case <-c.hotstuff.Done():
			break
		}
	} else {
		c.hotstuff.SubmitProposal(header, parent.View)
	}

	return nil
}

// OnBlockVote handles incoming block votes.
func (c *Core) OnBlockVote(originID flow.Identifier, vote *messages.BlockVote) error {

	span, _, isSampled := c.tracer.StartBlockSpan(context.Background(), vote.BlockID, trace.CONCompOnBlockVote)
	if isSampled {
		span.SetAttributes(
			attribute.String("origin_id", originID.String()),
		)
	}
	defer span.End()

	v := &model.Vote{
		View:     vote.View,
		BlockID:  vote.BlockID,
		SignerID: originID,
		SigData:  vote.SigData,
	}

	c.log.Info().
		Uint64("block_view", vote.View).
		Hex("block_id", vote.BlockID[:]).
		Hex("voter", v.SignerID[:]).
		Str("vote_id", v.ID().String()).
		Msg("block vote received, forwarding block vote to hotstuff vote aggregator")

	// forward the vote to hotstuff for processing
	c.voteAggregator.AddVote(v)

	return nil
}

// ProcessFinalizedView performs pruning of stale data based on finalization event
// removes pending blocks below the finalized view
func (c *Core) ProcessFinalizedView(finalizedView uint64) {
	// remove all pending blocks at or below the finalized view
	c.pending.PruneByView(finalizedView)

	// always record the metric
	c.mempool.MempoolEntries(metrics.ResourceProposal, c.pending.Size())
}
