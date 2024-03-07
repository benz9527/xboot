package list

import (
	"strconv"
	"sync/atomic"
	"unsafe"
)

type xConcSkipListIndex[W SkipListWeight, O HashObject] struct {
	forward  *xConcSkipListNode[W, O]
	backward *xConcSkipListNode[W, O] // Doubly linked list
}

type xConcSkipListIndices[W SkipListWeight, O HashObject] []*xConcSkipListIndex[W, O]

func (indices xConcSkipListIndices[W, O]) must(i int32) {
	l := len(indices)
	if int(i) >= l {
		panic("[x-conc-skl-indices] " + strconv.Itoa(int(i)) + " out of len " + strconv.Itoa(l))
	}
}

func (indices xConcSkipListIndices[W, O]) loadForward(i int32) *xConcSkipListNode[W, O] {
	indices.must(i)
	return indices[i].forward
}

func (indices xConcSkipListIndices[W, O]) loadBackward(i int32) *xConcSkipListNode[W, O] {
	indices.must(i)
	return indices[i].backward
}

func (indices xConcSkipListIndices[W, O]) storeForward(i int32, n *xConcSkipListNode[W, O]) {
	indices.must(i)
	indices[i].forward = n
}

func (indices xConcSkipListIndices[W, O]) storeBackward(i int32, n *xConcSkipListNode[W, O]) {
	indices.must(i)
	indices[i].backward = n
}

func (indices xConcSkipListIndices[W, O]) atomicLoadForward(i int32) *xConcSkipListNode[W, O] {
	indices.must(i)
	ptr := (*xConcSkipListNode[W, O])(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&indices[i].forward))))
	return ptr
}

func (indices xConcSkipListIndices[W, O]) atomicLoadBackward(i int32) *xConcSkipListNode[W, O] {
	indices.must(i)
	ptr := (*xConcSkipListNode[W, O])(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&indices[i].backward))))
	return ptr
}

func (indices xConcSkipListIndices[W, O]) atomicStoreForward(i int32, n *xConcSkipListNode[W, O]) {
	indices.must(i)
	atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&indices[i].forward)), unsafe.Pointer(n))
}

func (indices xConcSkipListIndices[W, O]) atomicStoreBackward(i int32, n *xConcSkipListNode[W, O]) {
	indices.must(i)
	atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&indices[i].backward)), unsafe.Pointer(n))
}

func newXConcSkipListIndices[W SkipListWeight, O HashObject](level int32) xConcSkipListIndices[W, O] {
	indices := make(xConcSkipListIndices[W, O], level)
	for i := int32(0); i < level; i++ {
		indices[i] = &xConcSkipListIndex[W, O]{
			forward:  nil,
			backward: nil,
		}
	}
	return indices
}

// Records the traverse predecessors and successors info.
type xConcSkipListAuxiliary[W SkipListWeight, O HashObject] []*xConcSkipListNode[W, O]

// Left part.
func (aux xConcSkipListAuxiliary[W, O]) loadPred(i int32) *xConcSkipListNode[W, O] {
	return aux[i]
}

func (aux xConcSkipListAuxiliary[W, O]) storePred(i int32, pred *xConcSkipListNode[W, O]) {
	aux[i] = pred
}

func (aux xConcSkipListAuxiliary[W, O]) foreachPred(fn func(list ...*xConcSkipListNode[W, O])) {
	fn(aux[0:xSkipListMaxLevel]...)
}

// Right part.
func (aux xConcSkipListAuxiliary[W, O]) loadSucc(i int32) *xConcSkipListNode[W, O] {
	return aux[xSkipListMaxLevel+i]
}

func (aux xConcSkipListAuxiliary[W, O]) storeSucc(i int32, succ *xConcSkipListNode[W, O]) {
	aux[xSkipListMaxLevel+i] = succ
}

func (aux xConcSkipListAuxiliary[W, O]) foreachSucc(fn func(list ...*xConcSkipListNode[W, O])) {
	fn(aux[xSkipListMaxLevel:]...)
}
