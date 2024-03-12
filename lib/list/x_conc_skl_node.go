package list

import (
	"sync/atomic"
	"unsafe"

	"github.com/benz9527/xboot/lib/infra"
)

const (
	nodeFullyLinked = 1 << iota
	nodeRemovingMarked
	nodeHeadMarked
	nodeRbtree
)

type xConcSkipListNode[K infra.OrderedKey, V comparable] struct {
	key     K
	val     *V
	indexes xConcSkipListIndices[K, V]
	mu      segmentedMutex
	flags   flagBits
	level   uint32
}

func (node *xConcSkipListNode[K, V]) storeVal(val V) {
	atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&node.val)), unsafe.Pointer(&val))
}

func (node *xConcSkipListNode[K, V]) loadVal() V {
	return *(*V)(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&node.val))))
}

func (node *xConcSkipListNode[K, V]) loadNext(i int32) *xConcSkipListNode[K, V] {
	return node.indexes.loadForward(i)
}

func (node *xConcSkipListNode[K, V]) storeNext(i int32, next *xConcSkipListNode[K, V]) {
	node.indexes.storeForward(i, next)
}

func (node *xConcSkipListNode[K, V]) atomicLoadNext(i int32) *xConcSkipListNode[K, V] {
	return node.indexes.atomicLoadForward(i)
}

func (node *xConcSkipListNode[K, V]) atomicStoreNext(i int32, next *xConcSkipListNode[K, V]) {
	node.indexes.atomicStoreForward(i, next)
}

func (node *xConcSkipListNode[K, V]) loadPrev(i int32) *xConcSkipListNode[K, V] {
	return node.indexes.loadBackward(i)
}

func (node *xConcSkipListNode[K, V]) storePrev(i int32, prev *xConcSkipListNode[K, V]) {
	node.indexes.storeBackward(i, prev)
}

func (node *xConcSkipListNode[K, V]) atomicLoadPrev(i int32) *xConcSkipListNode[K, V] {
	return node.indexes.atomicLoadBackward(i)
}

func (node *xConcSkipListNode[K, V]) atomicStorePrev(i int32, prev *xConcSkipListNode[K, V]) {
	node.indexes.atomicStoreBackward(i, prev)
}

func newXConcSkipListNode[K infra.OrderedKey, V comparable](key K, val V, level int32, e mutexEnum) *xConcSkipListNode[K, V] {
	node := &xConcSkipListNode[K, V]{
		key:   key,
		val:   &val,
		level: uint32(level),
		mu:    mutexFactory(e),
	}
	node.storeVal(val)
	node.indexes = newXConcSkipListIndices[K, V](level)
	return node
}

func newXConcSkipListHead[K infra.OrderedKey, V comparable](e mutexEnum) *xConcSkipListNode[K, V] {
	head := &xConcSkipListNode[K, V]{
		key:   *new(K),
		level: xSkipListMaxLevel,
		val:   nil,
		mu:    mutexFactory(e),
	}
	head.storeVal(*new(V))
	head.flags.atomicSet(nodeHeadMarked | nodeFullyLinked)
	head.indexes = newXConcSkipListIndices[K, V](xSkipListMaxLevel)
	return head
}

func unlockNodes[K infra.OrderedKey, V comparable](version uint64, num int32, nodes ...*xConcSkipListNode[K, V]) {
	var prev *xConcSkipListNode[K, V]
	for i := num; i >= 0; i-- {
		if nodes[i] != prev { // the node could be unlocked by previous loop
			nodes[i].mu.unlock(version)
			prev = nodes[i]
		}
	}
}
