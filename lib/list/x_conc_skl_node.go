package list

import (
	"sync/atomic"
	"unsafe"
)

const (
	nodeFullyLinked = 1 << iota
	nodeRemovingMarked
	nodeHeadMarked
	nodeRbtree
)

type xConcSkipListNode[W SkipListWeight, O HashObject] struct {
	weight  W
	object  *O
	indexes xConcSkipListIndices[W, O]
	mu      segmentedMutex
	flags   flagBits
	level   uint32
}

func (node *xConcSkipListNode[W, O]) storeObject(obj O) {
	atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&node.object)), unsafe.Pointer(&obj))
}

func (node *xConcSkipListNode[W, O]) loadObject() O {
	return *(*O)(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&node.object))))
}

func (node *xConcSkipListNode[W, O]) loadNext(i int32) *xConcSkipListNode[W, O] {
	return node.indexes.loadForward(i)
}

func (node *xConcSkipListNode[W, O]) storeNext(i int32, next *xConcSkipListNode[W, O]) {
	node.indexes.storeForward(i, next)
}

func (node *xConcSkipListNode[W, O]) atomicLoadNext(i int32) *xConcSkipListNode[W, O] {
	return node.indexes.atomicLoadForward(i)
}

func (node *xConcSkipListNode[W, O]) atomicStoreNext(i int32, next *xConcSkipListNode[W, O]) {
	node.indexes.atomicStoreForward(i, next)
}

func (node *xConcSkipListNode[W, O]) loadPrev(i int32) *xConcSkipListNode[W, O] {
	return node.indexes.loadBackward(i)
}

func (node *xConcSkipListNode[W, O]) storePrev(i int32, prev *xConcSkipListNode[W, O]) {
	node.indexes.storeBackward(i, prev)
}

func (node *xConcSkipListNode[W, O]) atomicLoadPrev(i int32) *xConcSkipListNode[W, O] {
	return node.indexes.atomicLoadBackward(i)
}

func (node *xConcSkipListNode[W, O]) atomicStorePrev(i int32, prev *xConcSkipListNode[W, O]) {
	node.indexes.atomicStoreBackward(i, prev)
}

func newXConcSkipListNode[W SkipListWeight, O HashObject](weight W, obj O, level int32, e mutexEnum) *xConcSkipListNode[W, O] {
	node := &xConcSkipListNode[W, O]{
		weight: weight,
		level:  uint32(level),
		object: &obj,
		mu:     mutexFactory(e),
	}
	node.storeObject(obj)
	node.indexes = newXConcSkipListIndices[W, O](level)
	return node
}

func newXConcSkipListHead[W SkipListWeight, O HashObject](e mutexEnum) *xConcSkipListNode[W, O] {
	head := &xConcSkipListNode[W, O]{
		weight: *new(W),
		level:  xSkipListMaxLevel,
		object: nil,
		mu:     mutexFactory(e),
	}
	head.storeObject(*new(O))
	head.flags.atomicSet(nodeHeadMarked | nodeFullyLinked)
	head.indexes = newXConcSkipListIndices[W, O](xSkipListMaxLevel)
	return head
}

func unlockNodes[W SkipListWeight, O HashObject](version uint64, num int32, nodes ...*xConcSkipListNode[W, O]) {
	var prev *xConcSkipListNode[W, O]
	for i := num; i >= 0; i-- {
		if nodes[i] != prev { // the node could be unlocked by previous loop
			nodes[i].mu.unlock(version)
			prev = nodes[i]
		}
	}
}
