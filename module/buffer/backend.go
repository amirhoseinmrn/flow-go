package buffer

import (
	"sync"

	"github.com/onflow/flow-go/model/flow"
)

// backend implements a simple cache of pending blocks, indexed by parent ID.
type backend[P flow.GenericPayload] struct {
	mu sync.RWMutex
	// map of pending block IDs, keyed by parent ID for ByParentID lookups
	blocksByParent map[flow.Identifier][]flow.Identifier
	// set of pending blocks, keyed by ID to avoid duplication
	blocksByID map[flow.Identifier]flow.Slashable[flow.GenericBlock[P]]
}

// newBackend returns a new pending block cache.
func newBackend[P flow.GenericPayload]() *backend[P] {
	cache := &backend[P]{
		blocksByParent: make(map[flow.Identifier][]flow.Identifier),
		blocksByID:     make(map[flow.Identifier]flow.Slashable[flow.GenericBlock[P]]),
	}
	return cache
}

// add adds the item to the cache, returning false if it already exists and
// true otherwise.
func (b *backend[P]) add(originID flow.Identifier, block *flow.GenericBlock[P]) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	blockID := block.ID()

	_, exists := b.blocksByID[blockID]
	if exists {
		return false
	}

	b.blocksByID[blockID] = flow.Slashable[flow.GenericBlock[P]]{
		OriginID: originID,
		Message:  block,
	}
	parentID := block.Header.ParentID
	b.blocksByParent[parentID] = append(b.blocksByParent[parentID], blockID)

	return true
}

func (b *backend[P]) byID(id flow.Identifier) (flow.Slashable[flow.GenericBlock[P]], bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	item, exists := b.blocksByID[id]
	return item, exists
}

// byParentID returns a list of cached blocks with the given parent. If no such
// blocks exist, returns false.
func (b *backend[P]) byParentID(parentID flow.Identifier) ([]flow.Slashable[flow.GenericBlock[P]], bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	forParent, exists := b.blocksByParent[parentID]
	if !exists {
		return nil, false
	}

	items := make([]flow.Slashable[flow.GenericBlock[P]], 0, len(forParent))
	for _, blockID := range forParent {
		items = append(items, b.blocksByID[blockID])
	}

	return items, true
}

// dropForParent removes all cached blocks with the given parent (non-recursively).
func (b *backend[P]) dropForParent(parentID flow.Identifier) {

	b.mu.Lock()
	defer b.mu.Unlock()

	children, exists := b.blocksByParent[parentID]
	if !exists {
		return
	}

	for _, childID := range children {
		delete(b.blocksByID, childID)
	}
	delete(b.blocksByParent, parentID)
}

// pruneByView prunes any items in the cache that have view less than or
// equal to the given view. The pruning view should be the finalized view.
func (b *backend[P]) pruneByView(view uint64) {

	b.mu.Lock()
	defer b.mu.Unlock()

	for id, item := range b.blocksByID {
		if item.Message.Header.View <= view {
			delete(b.blocksByID, id)
			delete(b.blocksByParent, item.Message.Header.ParentID)
		}
	}
}

// size returns the number of elements stored in teh backend
func (b *backend[P]) size() uint {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return uint(len(b.blocksByID))
}
