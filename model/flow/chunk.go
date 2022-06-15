package flow

import "github.com/onflow/flow-go/module/executiondatasync/execution_data"

type ChunkBody struct {
	CollectionIndex uint

	// execution info
	StartState      StateCommitment // start state when starting executing this chunk
	EventCollection Identifier      // Events generated by executing results
	BlockID         Identifier      // Block id of the execution result this chunk belongs to

	// Computation consumption info
	TotalComputationUsed uint64 // total amount of computation used by running all txs in this chunk
	NumberOfTransactions uint64 // number of transactions inside the collection
}

type Chunk struct {
	ChunkBody

	Index uint64 // chunk index inside the ER (starts from zero)
	// EndState inferred from next chunk or from the ER
	EndState StateCommitment
}

// ID returns a unique id for this entity
func (ch *Chunk) ID() Identifier {
	return MakeID(ch.ChunkBody)
}

// Checksum provides a cryptographic commitment for a chunk content
func (ch *Chunk) Checksum() Identifier {
	return MakeID(ch)
}

// ChunkDataPack holds all register touches (any read, or write).
//
// Note that we have to capture a read proof for each write before updating the registers.
// `Proof` includes proofs for all registers read to execute the chunck.
// Register proofs order must not be correlated to the order of register reads during
// the chunk execution in order to enforce the SPoCK secret high entropy.
type ChunkDataPack struct {
	ChunkID           Identifier
	StartState        StateCommitment
	Proof             StorageProof
	Collection        *Collection
	ExecutionDataRoot execution_data.BlockExecutionDataRoot
}

// ID returns the unique identifier for the concrete view, which is the ID of
// the chunk the view is for.
func (c *ChunkDataPack) ID() Identifier {
	return c.ChunkID
}

// Checksum returns the checksum of the chunk data pack.
func (c *ChunkDataPack) Checksum() Identifier {
	return MakeID(c)
}

// TODO: This is the basic version of the list, we need to substitute it with something like Merkle tree at some point
type ChunkList []*Chunk

func (cl ChunkList) Fingerprint() Identifier {
	return MerkleRoot(GetIDs(cl)...)
}

func (cl *ChunkList) Insert(ch *Chunk) {
	*cl = append(*cl, ch)
}

func (cl ChunkList) Items() []*Chunk {
	return cl
}

// Empty returns true if the chunk list is empty. Otherwise it returns false.
func (cl ChunkList) Empty() bool {
	return len(cl) == 0
}

func (cl ChunkList) Indices() []uint64 {
	indices := make([]uint64, len(cl))
	for i, chunk := range cl {
		indices[i] = chunk.Index
	}

	return indices
}

// ByChecksum returns an entity from the list by entity fingerprint
func (cl ChunkList) ByChecksum(cs Identifier) (*Chunk, bool) {
	for _, ch := range cl {
		if ch.Checksum() == cs {
			return ch, true
		}
	}
	return nil, false
}

// ByIndex returns an entity from the list by index
// if requested chunk is within range of list, it returns chunk and true
// if requested chunk is out of the range, it returns nil and false
// boolean return value indicates whether requested chunk is within range
func (cl ChunkList) ByIndex(i uint64) (*Chunk, bool) {
	if i >= uint64(len(cl)) {
		// index out of range
		return nil, false
	}
	return cl[i], true
}

// Len returns the number of Chunks in the list. It is also part of the sort
// interface that makes ChunkList sortable
func (cl ChunkList) Len() int {
	return len(cl)
}
