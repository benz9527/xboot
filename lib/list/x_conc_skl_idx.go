package list

import (
	"sync/atomic"
	"unsafe"

	"github.com/benz9527/xboot/lib/infra"
)

type xConcSklIndex[K infra.OrderedKey, V any] struct {
	forward *xConcSklNode[K, V]
}

type xConcSklIndices[W infra.OrderedKey, O any] []*xConcSklIndex[W, O]

func (indices xConcSklIndices[W, O]) loadForwardIndex(i int32) *xConcSklNode[W, O] {
	return indices[i].forward
}

func (indices xConcSklIndices[W, O]) storeForwardIndex(i int32, n *xConcSklNode[W, O]) {
	indices[i].forward = n
}

func (indices xConcSklIndices[W, O]) atomicLoadForwardIndex(i int32) *xConcSklNode[W, O] {
	ptr := (*xConcSklNode[W, O])(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&indices[i].forward))))
	return ptr
}

func (indices xConcSklIndices[W, O]) atomicStoreForwardIndex(i int32, n *xConcSklNode[W, O]) {
	atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&indices[i].forward)), unsafe.Pointer(n))
}

func newXConcSklIndices[K infra.OrderedKey, V any](level int32) xConcSklIndices[K, V] {
	indices := make(xConcSklIndices[K, V], level)
	for i := int32(0); i < level; i++ {
		indices[i] = &xConcSklIndex[K, V]{
			forward: nil,
		}
	}
	return indices
}

// Auxiliary: records the traverse predecessors and successors info.
type xConcSklAux[K infra.OrderedKey, V any] []*xConcSklNode[K, V]

// Left part.
func (aux xConcSklAux[K, V]) loadPred(i int32) *xConcSklNode[K, V] {
	return aux[i]
}

func (aux xConcSklAux[K, V]) storePred(i int32, pred *xConcSklNode[K, V]) {
	aux[i] = pred
}

func (aux xConcSklAux[K, V]) foreachPred(fn func(list ...*xConcSklNode[K, V])) {
	fn(aux[0:sklMaxLevel]...)
}

// Right part.
func (aux xConcSklAux[K, V]) loadSucc(i int32) *xConcSklNode[K, V] {
	return aux[sklMaxLevel+i]
}

func (aux xConcSklAux[K, V]) storeSucc(i int32, succ *xConcSklNode[K, V]) {
	aux[sklMaxLevel+i] = succ
}

func (aux xConcSklAux[K, V]) foreachSucc(fn func(list ...*xConcSklNode[K, V])) {
	fn(aux[sklMaxLevel:]...)
}
