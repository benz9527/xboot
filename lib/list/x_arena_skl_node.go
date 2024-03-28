package list

import (
	"runtime"
	"sync/atomic"
	"unsafe"

	"github.com/benz9527/xboot/lib/infra"
)

var _ SklElement[uint8, uint8] = (*xArenaSklElement[uint8, uint8])(nil)

// xConcSklElement is used to keepalive of the Go memory objects' lifecycle.
type xArenaSklElement[K infra.OrderedKey, V any] struct {
	indices []*xArenaSklNode[K, V]
	nodeRef *xArenaSklNode[K, V]
	prev    *xArenaSklElement[K, V] // double-linked-list
	next    *xArenaSklElement[K, V]
	key     K
	val     atomic.Value
}

func (e *xArenaSklElement[K, V]) Key() K {
	return e.key
}

func (e *xArenaSklElement[K, V]) Val() V {
	return e.val.Load().(V)
}

func newXConcSklHeadElement[K infra.OrderedKey, V any]() *xArenaSklElement[K, V] {
	node := &xArenaSklNode[K, V]{
		level: sklMaxLevel,
	}
	node.flags.set(nodeIsHeadFlagBit | nodeInsertedFlagBit)
	node.flags.setBitsAs(xNodeModeFlagBits, uint32(unique))
	head := &xArenaSklElement[K, V]{
		indices: make([]*xArenaSklNode[K, V], sklMaxLevel),
	}
	head.nodeRef = node
	node.elementRef = head
	return head
}

func newXArenaSklDataElement[K infra.OrderedKey, V any](
	key K,
	val V,
	lvl uint32,
	arena *autoGrowthArena[xArenaSklNode[K, V]],
) *xArenaSklElement[K, V] {
	e := &xArenaSklElement[K, V]{
		key:     key,
		indices: make([]*xArenaSklNode[K, V], lvl),
	}
	e.val.Store(val)

	node, _ := arena.allocate()
	node.level = lvl
	node.elementRef = e
	e.nodeRef = node
	node.count = 1
	return e
}

// If it is unique x-node type store value directly.
// Otherwise, it is a sentinel node for linked-list or rbtree.
// @field count, the number of duplicate elements.
// @field mu, lock-free, spin-lock, optimistic-lock.
type xArenaSklNode[K infra.OrderedKey, V any] struct {
	elementRef *xArenaSklElement[K, V] // size 8, 1 byte, recursive
	mu         uint64                  // size 8, 2 byte
	count      int64                   // size 8, 1 byte
	level      uint32                  // size 4
	flags      flagBits                // size 4
}

func (node *xArenaSklNode[K, V]) lock(version uint64) {
	backoff := uint8(1)
	for !atomic.CompareAndSwapUint64(&node.mu, unlocked, version) {
		if backoff <= 32 {
			for i := uint8(0); i < backoff; i++ {
				infra.ProcYield(5)
			}
		} else {
			runtime.Gosched()
		}
		backoff <<= 1
	}
}

func (node *xArenaSklNode[K, V]) tryLock(version uint64) bool {
	return atomic.CompareAndSwapUint64(&node.mu, unlocked, version)
}

func (node *xArenaSklNode[K, V]) unlock(version uint64) bool {
	return atomic.CompareAndSwapUint64(&node.mu, version, unlocked)
}

func (node *xArenaSklNode[K, V]) loadNextNode(i int32) *xArenaSklNode[K, V] {
	return node.elementRef.indices[i]
}

func (node *xArenaSklNode[K, V]) storeNextNode(i int32, next *xArenaSklNode[K, V]) {
	node.elementRef.indices[i] = next
}

func (node *xArenaSklNode[K, V]) atomicLoadNextNode(i int32) *xArenaSklNode[K, V] {
	return (*xArenaSklNode[K, V])(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&node.elementRef.indices[i]))))
}

func (node *xArenaSklNode[K, V]) atomicStoreNextNode(i int32, next *xArenaSklNode[K, V]) {
	atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&node.elementRef.indices[i])), unsafe.Pointer(next))
}

func unlockArenaNodes[K infra.OrderedKey, V any](version uint64, num int32, nodes ...*xArenaSklNode[K, V]) {
	var prev *xArenaSklNode[K, V]
	for i := num; i >= 0; i-- {
		if nodes[i] != prev {
			nodes[i].unlock(version)
			prev = nodes[i]
		}
	}
}
