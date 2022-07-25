package consensus

import (
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/blockproducer"
	"github.com/onflow/flow-go/consensus/hotstuff/eventhandler"
	"github.com/onflow/flow-go/consensus/hotstuff/eventloop"
	"github.com/onflow/flow-go/consensus/hotstuff/forks"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/consensus/hotstuff/pacemaker"
	"github.com/onflow/flow-go/consensus/hotstuff/pacemaker/timeout"
	"github.com/onflow/flow-go/consensus/hotstuff/safetyrules"
	"github.com/onflow/flow-go/consensus/hotstuff/signature"
	validatorImpl "github.com/onflow/flow-go/consensus/hotstuff/validator"
	"github.com/onflow/flow-go/consensus/hotstuff/verification"
	"github.com/onflow/flow-go/consensus/recovery"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/storage"
)

// NewParticipant initialize the EventLoop instance with needed dependencies
func NewParticipant(
	log zerolog.Logger,
	metrics module.HotstuffMetrics,
	builder module.Builder,
	communicator hotstuff.Communicator,
	finalized *flow.Header,
	pending []*flow.Header,
	modules *HotstuffModules,
	options ...Option,
) (module.HotStuff, error) {

	// initialize the default configuration
	defTimeout := timeout.DefaultConfig
	cfg := ParticipantConfig{
		TimeoutInitial:        time.Duration(defTimeout.ReplicaTimeout) * time.Millisecond,
		TimeoutMinimum:        time.Duration(defTimeout.MinReplicaTimeout) * time.Millisecond,
		TimeoutMaximum:        time.Duration(defTimeout.MaxReplicaTimeout) * time.Millisecond,
		TimeoutIncreaseFactor: defTimeout.TimeoutIncrease,
		TimeoutDecreaseFactor: defTimeout.TimeoutDecrease,
		BlockRateDelay:        time.Duration(defTimeout.BlockRateDelayMS) * time.Millisecond,
	}

	// apply the configuration options
	for _, option := range options {
		option(&cfg)
	}

	// prune vote aggregator to initial view
	modules.VoteAggregator.PruneUpToView(finalized.View)
	modules.TimeoutAggregator.PruneUpToView(finalized.View)

	// recover the hotstuff state, mainly to recover all pending blocks in Forks
	err := recovery.Participant(log, modules.Forks, modules.VoteAggregator, modules.Validator, finalized, pending)
	if err != nil {
		return nil, fmt.Errorf("could not recover hotstuff state: %w", err)
	}

	// initialize the timeout config
	timeoutConfig, err := timeout.NewConfig(
		cfg.TimeoutInitial,
		cfg.TimeoutMinimum,
		cfg.TimeoutMaximum,
		cfg.TimeoutIncreaseFactor,
		cfg.TimeoutDecreaseFactor,
		cfg.BlockRateDelay,
	)
	if err != nil {
		return nil, fmt.Errorf("could not initialize timeout config: %w", err)
	}

	// initialize the pacemaker
	controller := timeout.NewController(timeoutConfig)
	pacemaker, err := pacemaker.New(controller, modules.Notifier, modules.Persist)
	if err != nil {
		return nil, fmt.Errorf("could not initialize flow pacemaker: %w", err)
	}

	// initialize block producer
	producer, err := blockproducer.New(modules.Signer, modules.Committee, builder)
	if err != nil {
		return nil, fmt.Errorf("could not initialize block producer: %w", err)
	}

	// initialize the safetyRules
	safetyRules, err := safetyrules.New(modules.Signer, modules.Persist, modules.Committee)
	if err != nil {
		return nil, fmt.Errorf("could not initialize safety rules: %w", err)
	}

	// initialize the event handler
	eventHandler, err := eventhandler.NewEventHandler(
		log,
		pacemaker,
		producer,
		modules.Forks,
		modules.Persist,
		communicator,
		modules.Committee,
		modules.VoteAggregator,
		modules.TimeoutAggregator,
		safetyRules,
		modules.Notifier,
	)
	if err != nil {
		return nil, fmt.Errorf("could not initialize event handler: %w", err)
	}

	// initialize and return the event loop
	loop, err := eventloop.NewEventLoop(log, metrics, eventHandler, cfg.StartupTime)
	if err != nil {
		return nil, fmt.Errorf("could not initialize event loop: %w", err)
	}

	// add observer, event loop needs to receive events from distributor
	modules.QCCreatedDistributor.AddConsumer(loop)
	modules.TimeoutCollectorDistributor.AddConsumer(loop)

	return loop, nil
}

// NewValidator creates new instance of hotstuff validator needed for votes & proposal validation
func NewValidator(metrics module.HotstuffMetrics, committee hotstuff.DynamicCommittee) hotstuff.Validator {
	packer := signature.NewConsensusSigDataPacker(committee)
	verifier := verification.NewCombinedVerifier(committee, packer)

	// initialize the Validator
	validator := validatorImpl.New(committee, verifier)
	return validatorImpl.NewMetricsWrapper(validator, metrics) // wrapper for measuring time spent in Validator component
}

// NewForks recovers trusted root and creates new forks manager
func NewForks(final *flow.Header, headers storage.Headers, updater module.Finalizer, notifier hotstuff.FinalizationConsumer, rootHeader *flow.Header, rootQC *flow.QuorumCertificate) (*forks.Forks, error) {
	// recover the trusted root
	trustedRoot, err := recoverTrustedRoot(final, headers, rootHeader, rootQC)
	if err != nil {
		return nil, fmt.Errorf("could not recover trusted root: %w", err)
	}

	// initialize the forks
	forks, err := forks.New(trustedRoot, updater, notifier)
	if err != nil {
		return nil, fmt.Errorf("could not initialize forks: %w", err)
	}

	return forks, nil
}

// recoverTrustedRoot based on our local state returns root block and QC that can be used to initialize base state
func recoverTrustedRoot(final *flow.Header, headers storage.Headers, rootHeader *flow.Header, rootQC *flow.QuorumCertificate) (*forks.BlockQC, error) {
	if final.View < rootHeader.View {
		return nil, fmt.Errorf("finalized Block has older view than trusted root")
	}

	// if finalized view is genesis block, then use genesis block as the trustedRoot
	if final.View == rootHeader.View {
		if final.ID() != rootHeader.ID() {
			return nil, fmt.Errorf("finalized Block conflicts with trusted root")
		}
		return makeRootBlockQC(rootHeader, rootQC), nil
	}

	// get the parent for the latest finalized block
	parent, err := headers.ByBlockID(final.ParentID)
	if err != nil {
		return nil, fmt.Errorf("could not get parent for finalized: %w", err)
	}

	// find a valid child of the finalized block in order to get its QC
	children, err := headers.ByParentID(final.ID())
	if err != nil {
		// a finalized block must have a valid child, if err happens, we exit
		return nil, fmt.Errorf("could not get children for finalized block (ID: %v, view: %v): %w", final.ID(), final.View, err)
	}
	if len(children) == 0 {
		return nil, fmt.Errorf("finalized block has no children")
	}

	child := model.BlockFromFlow(children[0], final.View)

	// create the root block to use
	trustedRoot := &forks.BlockQC{
		Block: model.BlockFromFlow(final, parent.View),
		QC:    child.QC,
	}

	return trustedRoot, nil
}

func makeRootBlockQC(header *flow.Header, qc *flow.QuorumCertificate) *forks.BlockQC {
	// By convention of Forks, the trusted root block does not need to have a qc
	// (as is the case for the genesis block). For simplify of the implementation, we always omit
	// the QC of the root block. Thereby, we have one algorithm which handles all cases,
	// instead of having to distinguish between a genesis block without a qc
	// and a later-finalized root block where we can retrieve the qc.
	rootBlock := &model.Block{
		View:        header.View,
		BlockID:     header.ID(),
		ProposerID:  header.ProposerID,
		QC:          nil, // QC is omitted
		PayloadHash: header.PayloadHash,
		Timestamp:   header.Timestamp,
	}
	return &forks.BlockQC{
		QC:    qc,
		Block: rootBlock,
	}
}
