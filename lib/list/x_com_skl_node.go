package list

import (
	"github.com/benz9527/xboot/lib/infra"
)

// The cache level array index > 0, it is the Y axis, and it means that it is the interval after
// the bisection search. Used to locate an element quickly.
// The cache level array index == 0, it is the X axis, and it means that it is the bits container.
type xComSklNode[K infra.OrderedKey, V any] struct {
	// The cache part.
	// When the current node works as a data node, it doesn't contain levels metadata.
	// If a node is a level node, the cache is from levels[0], but it is differed
	// to the sentinel's levels[0].
	indices []*xComSklIndex[K, V] // The cache level array.
	element SklElement[K, V]
	// Works for a backward iteration direction.
	pred *xComSklNode[K, V]
}

func (node *xComSklNode[K, V]) Element() SklElement[K, V] {
	if node == nil {
		return nil
	}
	return node.element
}

func (node *xComSklNode[K, V]) setElement(e SklElement[K, V]) {
	if node == nil {
		return
	}
	node.element = e
}

func (node *xComSklNode[K, V]) backward() *xComSklNode[K, V] {
	if node == nil {
		return nil
	}
	return node.pred
}

func (node *xComSklNode[K, V]) setBackward(pred *xComSklNode[K, V]) {
	if node == nil {
		return
	}
	node.pred = pred
}

func (node *xComSklNode[K, V]) levels() []*xComSklIndex[K, V] {
	if node == nil {
		return nil
	}
	return node.indices
}

func (node *xComSklNode[K, V]) Free() {
	node.element = nil
	node.pred = nil
	node.indices = nil
}

func newXComSklNode[K infra.OrderedKey, V any](level int32, key K, obj V) *xComSklNode[K, V] {
	e := &xComSklNode[K, V]{
		element: &xSklElement[K, V]{
			key: key,
			val: obj,
		},
		indices: make([]*xComSklIndex[K, V], level),
	}
	for i := int32(0); i < level; i++ {
		e.indices[i] = newXComSklIndex[K, V](nil)
	}
	return e
}
