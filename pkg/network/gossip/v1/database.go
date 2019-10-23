package gnode

import (
	"fmt"
	"sync"

	"github.com/dapperlabs/flow-go/pkg/grpc/shared"
)

//messageDatabase is an on memory database interface that keeps a copy of the messages that a node
//receives. This is done as a support to the hash proposals, i.e., to avoid duplicate messages, nodes may
//send the hash first, and seek a confirmation from the receiver on the entire content of the message.
//Although the interface may remain the same, but the implementation is subject to change especially if a database
//is provided by the application layer.
type messageDatabase interface {
	Get(string) (*shared.GossipMessage, error)
	Put(string, *shared.GossipMessage) error
}

type memoryMsgDatabase struct {
	mu    sync.RWMutex
	store map[string]*shared.GossipMessage
}

// NewDatabase returns an empty data base
func newMemMsgDatabase() *memoryMsgDatabase {
	return &memoryMsgDatabase{
		store: make(map[string]*shared.GossipMessage),
	}
}

// put adds a message to the database with its hash as a key
func (mdb *memoryMsgDatabase) Put(hash string, msg *shared.GossipMessage) error {
	mdb.mu.RLock()
	defer mdb.mu.RUnlock()

	mdb.store[hash] = msg

	return nil
}

// get returns a GossipMessage corresponding to the given hash
func (mdb *memoryMsgDatabase) Get(hash string) (*shared.GossipMessage, error) {
	mdb.mu.RLock()
	defer mdb.mu.RUnlock()

	if mdb.store[hash] == nil {
		return nil, fmt.Errorf("no message for the given hash")
	}

	return mdb.store[hash], nil
}
