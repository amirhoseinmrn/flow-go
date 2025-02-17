package hotstuff

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
)

// Committee accounts for the fact that we might have multiple HotStuff instances
// (collector committees and main consensus committee). Each HotStuff instance is supposed to
// have a dedicated Committee state.
// A Committee provides subset of the protocol.State, which is restricted to exactly those
// nodes that participate in the current HotStuff instance: the state of all legitimate HotStuff
// participants for the specified block. Legitimate HotStuff participants have NON-ZERO WEIGHT.
//
// The intended use case is to support collectors running HotStuff within Flow. Specifically,
// the collectors produced their own blocks, independently of the Consensus Nodes (aka the main consensus).
// Given a collector block, some logic is required to find the main consensus block
// for determining the valid collector-HotStuff participants.
type Committee interface {
	// Identities returns a IdentityList with legitimate HotStuff participants for the specified block.
	// The returned list of HotStuff participants
	//   * contains nodes that are allowed to sign the specified block (legitimate participants with NON-ZERO WEIGHT)
	//   * is ordered in the canonical order
	//   * contains no duplicates.
	// TODO: document possible error returns -> https://github.com/dapperlabs/flow-go/issues/6327
	Identities(blockID flow.Identifier) (flow.IdentityList, error)

	// Identity returns the full Identity for specified HotStuff participant.
	// The node must be a legitimate HotStuff participant with NON-ZERO WEIGHT at the specified block.
	// ERROR conditions:
	//  * model.InvalidSignerError if participantID does NOT correspond to an authorized HotStuff participant at the specified block.
	Identity(blockID flow.Identifier, participantID flow.Identifier) (*flow.Identity, error)

	// LeaderForView returns the identity of the leader for a given view.
	// CAUTION: per liveness requirement of HotStuff, the leader must be fork-independent.
	//          Therefore, a node retains its proposer view slots even if it is slashed.
	//          Its proposal is simply considered invalid, as it is not from a legitimate participant.
	// Returns the following expected errors for invalid inputs:
	//  * epoch containing the requested view has not been set up (protocol.ErrNextEpochNotSetup)
	//  * epoch is too far in the past (leader.InvalidViewError)
	LeaderForView(view uint64) (flow.Identifier, error)

	// Self returns our own node identifier.
	// TODO: ultimately, the own identity of the node is necessary for signing.
	//       Ideally, we would move the method for checking whether an Identifier refers to this node to the signer.
	//       This would require some refactoring of EventHandler (postponed to later)
	Self() flow.Identifier

	// DKG returns the DKG info for the given block.
	DKG(blockID flow.Identifier) (DKG, error)
}

// BlockSignerDecoder defines how to convert the SignerIndices field within a particular
// block header to the identifiers of the nodes which signed the block.
type BlockSignerDecoder interface {
	// DecodeSignerIDs decodes the signer indices from the given block header into full node IDs.
	// Expected Error returns during normal operations:
	//  * state.UnknownBlockError if block has not been ingested yet
	//    TODO: this sentinel could be changed to `ErrViewForUnknownEpoch` once we merge the active pacemaker
	//  * signature.InvalidSignerIndicesError if signer indices included in the header do
	//    not encode a valid subset of the consensus committee
	DecodeSignerIDs(header *flow.Header) (flow.IdentifierList, error)
}

type DKG interface {
	protocol.DKG
}

// ComputeWeightThresholdForBuildingQC returns the weight that is minimally required for building a QC
func ComputeWeightThresholdForBuildingQC(totalWeight uint64) uint64 {
	// Given totalWeight, we need the smallest integer t such that 2 * totalWeight / 3 < t
	// Formally, the minimally required weight is: 2 * Floor(totalWeight/3) + max(1, totalWeight mod 3)
	floorOneThird := totalWeight / 3 // integer division, includes floor
	res := 2 * floorOneThird
	divRemainder := totalWeight % 3
	if divRemainder <= 1 {
		res = res + 1
	} else {
		res += divRemainder
	}
	return res
}
