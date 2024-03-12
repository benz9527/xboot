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
	weight  K
	object  *V
	indexes xConcSkipListIndices[K, V]
	mu      segmentedMutex
	flags   flagBits
	level   uint32
}

func (node *xConcSkipListNode[K, V]) storeObject(obj V) {
	atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&node.object)), unsafe.Pointer(&obj))
}

func (node *xConcSkipListNode[K, V]) loadObject() V {
	return *(*V)(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&node.object))))
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

func newXConcSkipListNode[K infra.OrderedKey, V comparable](weight K, obj V, level int32, e mutexEnum) *xConcSkipListNode[K, V] {
	node := &xConcSkipListNode[K, V]{
		weight: weight,
		level:  uint32(level),
		object: &obj,
		mu:     mutexFactory(e),
	}
	node.storeObject(obj)
	node.indexes = newXConcSkipListIndices[K, V](level)
	return node
}

func newXConcSkipListHead[K infra.OrderedKey, V comparable](e mutexEnum) *xConcSkipListNode[K, V] {
	head := &xConcSkipListNode[K, V]{
		weight: *new(K),
		level:  xSkipListMaxLevel,
		object: nil,
		mu:     mutexFactory(e),
	}
	head.storeObject(*new(V))
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
