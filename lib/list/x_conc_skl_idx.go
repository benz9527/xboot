package list

import (
	"strconv"
	"sync/atomic"
	"unsafe"

	"github.com/benz9527/xboot/lib/infra"
)

type xConcSkipListIndex[K infra.OrderedKey, V comparable] struct {
	forward  *xConcSkipListNode[K, V]
	backward *xConcSkipListNode[K, V] // Doubly linked list
}

type xConcSkipListIndices[W infra.OrderedKey, O comparable] []*xConcSkipListIndex[W, O]

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

func newXConcSkipListIndices[K infra.OrderedKey, V comparable](level int32) xConcSkipListIndices[K, V] {
	indices := make(xConcSkipListIndices[K, V], level)
	for i := int32(0); i < level; i++ {
		indices[i] = &xConcSkipListIndex[K, V]{
			forward:  nil,
			backward: nil,
		}
	}
	return indices
}

// Records the traverse predecessors and successors info.
type xConcSkipListAuxiliary[K infra.OrderedKey, V comparable] []*xConcSkipListNode[K, V]

// Left part.
func (aux xConcSkipListAuxiliary[K, V]) loadPred(i int32) *xConcSkipListNode[K, V] {
	return aux[i]
}

func (aux xConcSkipListAuxiliary[K, V]) storePred(i int32, pred *xConcSkipListNode[K, V]) {
	aux[i] = pred
}

func (aux xConcSkipListAuxiliary[K, V]) foreachPred(fn func(list ...*xConcSkipListNode[K, V])) {
	fn(aux[0:xSkipListMaxLevel]...)
}

// Right part.
func (aux xConcSkipListAuxiliary[K, V]) loadSucc(i int32) *xConcSkipListNode[K, V] {
	return aux[xSkipListMaxLevel+i]
}

func (aux xConcSkipListAuxiliary[K, V]) storeSucc(i int32, succ *xConcSkipListNode[K, V]) {
	aux[xSkipListMaxLevel+i] = succ
}

func (aux xConcSkipListAuxiliary[K, V]) foreachSucc(fn func(list ...*xConcSkipListNode[K, V])) {
	fn(aux[xSkipListMaxLevel:]...)
}
