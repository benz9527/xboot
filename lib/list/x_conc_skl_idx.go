package list

import (
	"strconv"
	"sync/atomic"
	"unsafe"

	"github.com/benz9527/xboot/lib/infra"
)

type xConcSkipListIndex[K infra.OrderedKey, V comparable] struct {
	forward  *xConcSklNode[K, V]
	backward *xConcSklNode[K, V] // Doubly linked list
}

type xConcSklIndices[W infra.OrderedKey, O comparable] []*xConcSkipListIndex[W, O]

func (indices xConcSklIndices[W, O]) must(i int32) {
	l := len(indices)
	if int(i) >= l {
		panic("[x-conc-skl-indices] " + strconv.Itoa(int(i)) + " out of len " + strconv.Itoa(l))
	}
}

func (indices xConcSklIndices[W, O]) loadForward(i int32) *xConcSklNode[W, O] {
	indices.must(i)
	return indices[i].forward
}

func (indices xConcSklIndices[W, O]) loadBackward(i int32) *xConcSklNode[W, O] {
	indices.must(i)
	return indices[i].backward
}

func (indices xConcSklIndices[W, O]) storeForward(i int32, n *xConcSklNode[W, O]) {
	indices.must(i)
	indices[i].forward = n
}

func (indices xConcSklIndices[W, O]) storeBackward(i int32, n *xConcSklNode[W, O]) {
	indices.must(i)
	indices[i].backward = n
}

func (indices xConcSklIndices[W, O]) atomicLoadForward(i int32) *xConcSklNode[W, O] {
	indices.must(i)
	ptr := (*xConcSklNode[W, O])(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&indices[i].forward))))
	return ptr
}

func (indices xConcSklIndices[W, O]) atomicLoadBackward(i int32) *xConcSklNode[W, O] {
	indices.must(i)
	ptr := (*xConcSklNode[W, O])(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&indices[i].backward))))
	return ptr
}

func (indices xConcSklIndices[W, O]) atomicStoreForward(i int32, n *xConcSklNode[W, O]) {
	indices.must(i)
	atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&indices[i].forward)), unsafe.Pointer(n))
}

func (indices xConcSklIndices[W, O]) atomicStoreBackward(i int32, n *xConcSklNode[W, O]) {
	indices.must(i)
	atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&indices[i].backward)), unsafe.Pointer(n))
}

func newXConcSkipListIndices[K infra.OrderedKey, V comparable](level int32) xConcSklIndices[K, V] {
	indices := make(xConcSklIndices[K, V], level)
	for i := int32(0); i < level; i++ {
		indices[i] = &xConcSkipListIndex[K, V]{
			forward:  nil,
			backward: nil,
		}
	}
	return indices
}

// Auxiliary: records the traverse predecessors and successors info.
type xConcSklAux[K infra.OrderedKey, V comparable] []*xConcSklNode[K, V]

// Left part.
func (aux xConcSklAux[K, V]) loadPred(i int32) *xConcSklNode[K, V] {
	return aux[i]
}

func (aux xConcSklAux[K, V]) storePred(i int32, pred *xConcSklNode[K, V]) {
	aux[i] = pred
}

func (aux xConcSklAux[K, V]) foreachPred(fn func(list ...*xConcSklNode[K, V])) {
	fn(aux[0:xSkipListMaxLevel]...)
}

// Right part.
func (aux xConcSklAux[K, V]) loadSucc(i int32) *xConcSklNode[K, V] {
	return aux[xSkipListMaxLevel+i]
}

func (aux xConcSklAux[K, V]) storeSucc(i int32, succ *xConcSklNode[K, V]) {
	aux[xSkipListMaxLevel+i] = succ
}

func (aux xConcSklAux[K, V]) foreachSucc(fn func(list ...*xConcSklNode[K, V])) {
	fn(aux[xSkipListMaxLevel:]...)
}
