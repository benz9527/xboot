package list

import (
	"strconv"
	"sync/atomic"
	"unsafe"

	"github.com/benz9527/xboot/lib/infra"
)

type xConcSklIndex[K infra.OrderedKey, V comparable] struct {
	forward *xConcSklNode[K, V]
}

type xConcSklIndices[W infra.OrderedKey, O comparable] []*xConcSklIndex[W, O]

func (indices xConcSklIndices[W, O]) must(i int32) {
	l := len(indices)
	if int(i) >= l {
		panic("[x-conc-skl-indexCount] " + strconv.Itoa(int(i)) + " out of nodeLen " + strconv.Itoa(l))
	}
}

func (indices xConcSklIndices[W, O]) loadForward(i int32) *xConcSklNode[W, O] {
	indices.must(i)
	return indices[i].forward
}

func (indices xConcSklIndices[W, O]) storeForward(i int32, n *xConcSklNode[W, O]) {
	indices.must(i)
	indices[i].forward = n
}

func (indices xConcSklIndices[W, O]) atomicLoadForward(i int32) *xConcSklNode[W, O] {
	indices.must(i)
	ptr := (*xConcSklNode[W, O])(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&indices[i].forward))))
	return ptr
}

func (indices xConcSklIndices[W, O]) atomicStoreForward(i int32, n *xConcSklNode[W, O]) {
	indices.must(i)
	atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&indices[i].forward)), unsafe.Pointer(n))
}

func newXConcSklIndices[K infra.OrderedKey, V comparable](level int32) xConcSklIndices[K, V] {
	indices := make(xConcSklIndices[K, V], level)
	for i := int32(0); i < level; i++ {
		indices[i] = &xConcSklIndex[K, V]{
			forward: nil,
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
