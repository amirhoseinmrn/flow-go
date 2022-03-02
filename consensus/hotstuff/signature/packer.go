package signature

import (
	"fmt"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/consensus/hotstuff/packer"
	"github.com/onflow/flow-go/model/flow"
	pcker "github.com/onflow/flow-go/module/packer"
)

// ConsensusSigDataPacker implements the hotstuff.Packer interface.
// The encoding method is RLP.
type ConsensusSigDataPacker struct {
	packer.SigDataPacker
	committees hotstuff.Committee
}

var _ hotstuff.Packer = &ConsensusSigDataPacker{}

// NewConsensusSigDataPacker creates a new ConsensusSigDataPacker instance
func NewConsensusSigDataPacker(committees hotstuff.Committee) *ConsensusSigDataPacker {
	return &ConsensusSigDataPacker{
		committees: committees,
	}
}

// Pack serializes the block signature data into raw bytes, suitable to creat a QC.
// To pack the block signature data, we first build a compact data type, and then encode it into bytes.
// Expected error returns during normal operations:
//  * none; all errors are symptoms of inconsistent input data or corrupted internal state.
func (p *ConsensusSigDataPacker) Pack(blockID flow.Identifier, sig *hotstuff.BlockSignatureData) ([]byte, []byte, error) {
	// retrieve all authorized consensus participants at the given block
	fullMembers, err := p.committees.Identities(blockID)
	if err != nil {
		return nil, nil, fmt.Errorf("could not find consensus committees by block id(%v): %w", blockID, err)
	}

	// breaking staking and random beacon signers into signerIDs and sig type for compaction
	// each signer must have its signerID and sig type stored at the same index in the two slices
	// For v2, RandomBeaconSigners is nil, as we don't track individually which nodes contributed to the random beacon
	// For v3, RandomBeaconSigners is not nil, each RandomBeaconSigner also signed staking sig, so the returned signerIDs, should
	// include both StakingSigners and RandomBeaconSigners
	signerIndices, sigType, err := encodeSignerIndicesAndSigType(fullMembers.NodeIDs(), sig.StakingSigners, sig.RandomBeaconSigners)
	if err != nil {
		return nil, nil, fmt.Errorf("could not encode signer indices and sig types: %w", err)
	}

	data := packer.SignatureData{
		SigType:                      sigType,
		AggregatedStakingSig:         sig.AggregatedStakingSig,
		AggregatedRandomBeaconSig:    sig.AggregatedRandomBeaconSig,
		ReconstructedRandomBeaconSig: sig.ReconstructedRandomBeaconSig,
	}

	// encode the structured data into raw bytes
	encoded, err := p.Encode(&data)
	if err != nil {
		return nil, nil, fmt.Errorf("could not encode data %v, %w", data, err)
	}

	return signerIndices, encoded, nil
}

// Unpack de-serializes the provided signature data.
// sig is the aggregated signature data
// It returns:
//  - (sigData, nil) if successfully unpacked the signature data
//  - (nil, model.ErrInvalidFormat) if failed to unpack the signature data
func (p *ConsensusSigDataPacker) Unpack(signerIDs []flow.Identifier, sigData []byte) (*hotstuff.BlockSignatureData, error) {
	// decode into typed data
	data, err := p.Decode(sigData)
	if err != nil {
		return nil, fmt.Errorf("could not decode sig data %s: %w", err, model.ErrInvalidFormat)
	}

	stakingSigners, randomBeaconSigners, err := decodeSignerIndicesAndSigType(signerIDs, data.SigType)
	if err != nil {
		return nil, fmt.Errorf("could not decode signer indices and sig type: %w", err)
	}

	return &hotstuff.BlockSignatureData{
		StakingSigners:               stakingSigners,
		RandomBeaconSigners:          randomBeaconSigners,
		AggregatedStakingSig:         data.AggregatedStakingSig,
		AggregatedRandomBeaconSig:    data.AggregatedRandomBeaconSig,
		ReconstructedRandomBeaconSig: data.ReconstructedRandomBeaconSig,
	}, nil
}

// the total number of bytes required to fit the `count` number of bits
func bytesCount(count int) int {
	return (count + 7) >> 3
}

// serializeToBitVector encodes the given sigTypes into a bit vector.
// We append tailing `0`s to the vector to represent it as bytes.
func serializeToBitVector(sigTypes []hotstuff.SigType) ([]byte, error) {
	totalBytes := bytesCount(len(sigTypes))
	bytes := make([]byte, 0, totalBytes)
	// a sig type can be converted into one bit, the type at index 0 being converted into the least significant bit:
	// the random beacon type is mapped to 1, while the staking type is mapped to 0.
	// the remaining unfilled bits in the last byte will be 0
	const initialMask = byte(1 << 7)

	b := byte(0)
	mask := initialMask
	// iterate through every 8 sig types, and convert it into a byte
	for pos, sigType := range sigTypes {
		if sigType == hotstuff.SigTypeRandomBeacon {
			b ^= mask // set a bit to one
		} else if sigType != hotstuff.SigTypeStaking {
			return nil, fmt.Errorf("invalid sig type: %v at pos %v", sigType, pos)
		}

		mask >>= 1     // move to the next bit
		if mask == 0 { // this happens every 8 loop iterations
			bytes = append(bytes, b)
			b = byte(0)
			mask = initialMask
		}
	}
	// add the last byte when the length is not multiple of 8
	if mask != initialMask {
		bytes = append(bytes, b)
	}
	return bytes, nil
}

// deserializeFromBitVector decodes the sig types from the given bit vector
// - serialized: bit-vector, one bit for each signer (tailing `0`s appended to make full bytes)
// - count: the total number of sig types to be deserialized from the given bytes
// It returns:
// - (sigTypes, nil) if successfully deserialized sig types
// - (nil, model.ErrInvalidFormat) if the number of serialized bytes doesn't match the given number of sig types
// - (nil, model.ErrInvalidFormat) if the remaining bits in the last byte are not all 0s
func deserializeFromBitVector(serialized []byte, count int) ([]hotstuff.SigType, error) {
	types := make([]hotstuff.SigType, 0, count)

	// validate the length of serialized vector
	// it must be equal to the bytes required to fit exactly `count` number of bits
	totalBytes := bytesCount(count)
	if len(serialized) != totalBytes {
		return nil, fmt.Errorf("encoding sig types of %d signers requires %d bytes but got %d bytes: %w",
			count, totalBytes, len(serialized), model.ErrInvalidFormat)
	}

	// parse each bit in the bit-vector, bit 0 is SigTypeStaking, bit 1 is SigTypeRandomBeacon
	var byt byte
	var offset int
	for i := 0; i < count; i++ {
		byt = serialized[i>>3]
		offset = 7 - (i & 7)
		mask := byte(1 << offset)
		if byt&mask == 0 {
			types = append(types, hotstuff.SigTypeStaking)
		} else {
			types = append(types, hotstuff.SigTypeRandomBeacon)
		}
	}

	// remaining bits (if any), they must be all `0`s
	remainings := byt << (8 - offset)
	if remainings != byte(0) {
		return nil, fmt.Errorf("the remaining bits are expected to be all 0s, but are %v: %w",
			remainings, model.ErrInvalidFormat)
	}

	return types, nil
}

// encodeSignerIndicesAndSigType encodes the given stakingSigners and beaconSigners into signer indices and sig types.
// The given fullMembers provides the canonical order of signer index for each signer.
// As an example consider the case where we have a committee C of 10 nodes in canonical oder
//            C = [A,B,C,D,E,F,G,H,I,J]
// where nodes [B,F] are stakingSigners and beaconSigners are [C,E,G,I,J].
//  * First return parameter: QC.signerIndices
//    - We start with a bit vector v that has |C| number of bits
//    - If a node contributed either as staking signer or beacon signer,
//      we set the respective bit to 1:
//          [A,B,C,D,E,F,G,H,I,J]
//             ↓     ↓ ↓     ↓ ↓
//           0,1,1,0,1,1,1,0,1,1
//    - Lastly, right-pad the resulting bit vector with 0 to full bytes. We have 10 committee members,
//      so we pad to 2 bytes:
//           01101110 11000000
//  * second return parameter: signature types
//    - Here, we restrict our focus on the signers, which we encoded in the previous step.
//      In out example, nodes [B,C,E,F,G,I,J] signed in canonical order. This is exactly the same order,
//      as we have represented the signer in the last step.
//    - For these 5 nodes in their canonical order, we encode each node's signature type as
// 			bit-value 1: node was in beaconSigners
// 			bit-value 0: node was in stakingSigners
//      This results in the bit vector
//            [B,C,E,F,G,I,J]
//             ↓ ↓ ↓ ↓ ↓ ↓ ↓
//             0,1,0,1,1,1,1
//    - Again, we right-pad with zeros to full bytes, As we only had 7 signers, the sigType slice is 1byte long
//             01011110
// Prerequisites:
//  * The inputs `stakingSigners` and `beaconSigners` are treated as sets, i.e. they
//    should not contain any duplicates.
//  * A node can be listed in either `stakingSigners` or `beaconSigners`. A node appearing in both lists
//    constitutes an illegal input.
//  * `stakingSigners` must be a subset of `fullMembers`
//  * `beaconSigners` must be a subset of `fullMembers`
// Violation of any of these prerequisites results in an error return.
func encodeSignerIndicesAndSigType(fullMembers []flow.Identifier, stakingSigners flow.IdentifierList, beaconSigners flow.IdentifierList) ([]byte, []byte, error) {
	stakingSignersLookup := stakingSigners.Lookup()
	beaconSignersLookup := beaconSigners.Lookup()

	indices := make([]int, 0, len(fullMembers))
	sigType := make([]hotstuff.SigType, 0, len(stakingSigners)+len(beaconSigners))

	for i, member := range fullMembers {
		if _, ok := stakingSignersLookup[member]; ok {
			indices = append(indices, i)
			delete(stakingSignersLookup, member)

			sigType = append(sigType, hotstuff.SigTypeStaking)
			continue
		}

		if _, ok := beaconSignersLookup[member]; ok {
			indices = append(indices, i)
			delete(beaconSignersLookup, member)

			sigType = append(sigType, hotstuff.SigTypeRandomBeacon)
			continue
		}
	}

	if len(stakingSignersLookup) > 0 {
		return nil, nil, fmt.Errorf("unknown staking signers %v", stakingSignersLookup)
	}
	if len(beaconSignersLookup) > 0 {
		return nil, nil, fmt.Errorf("unknown or duplicated beacon signers %v", beaconSignersLookup)
	}

	signerIndices, err := pcker.EncodeSignerIndices(indices, len(fullMembers))
	if err != nil {
		return nil, nil, fmt.Errorf("could not encode signer indices: %w", err)
	}

	serialized, err := serializeToBitVector(sigType)
	if err != nil {
		return nil, nil, fmt.Errorf("could not serialize sig types to bytes: %w", err)
	}

	return signerIndices, serialized, nil
}

// decodeSignerIndicesAndSigType decodes sigType and use it to split the given signerIDs into two groups: staking sigers and random beacon signers.
// it returns model.ErrInvalidFormat if decode failed or the decoded data doesn't match with the given signer IDs.
func decodeSignerIndicesAndSigType(signerIDs []flow.Identifier, sigType []byte) ([]flow.Identifier, []flow.Identifier, error) {
	// deserialize the compact sig types
	sigTypes, err := deserializeFromBitVector(sigType, len(signerIDs))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to deserialize sig types from bytes: %w", err)
	}

	if len(signerIDs) != len(sigTypes) {
		return nil, nil, fmt.Errorf("mismatching sigerIDs and sigTypes, %v signerIDs and %v sigTypes: %w",
			len(signerIDs), len(sigTypes), model.ErrInvalidFormat)
	}

	// read each signer's signerID and sig type from two different slices
	// group signers by its sig type
	stakingSigners := make([]flow.Identifier, 0, len(signerIDs))
	randomBeaconSigners := make([]flow.Identifier, 0, len(signerIDs))

	for i, sigType := range sigTypes {
		signerID := signerIDs[i]

		if sigType == hotstuff.SigTypeStaking {
			stakingSigners = append(stakingSigners, signerID)
		} else if sigType == hotstuff.SigTypeRandomBeacon {
			randomBeaconSigners = append(randomBeaconSigners, signerID)
		} else {
			return nil, nil, fmt.Errorf("unknown sigType %v, %w", sigType, model.ErrInvalidFormat)
		}
	}

	return stakingSigners, randomBeaconSigners, nil
}
