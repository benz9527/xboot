package list

import (
	"github.com/benz9527/xboot/lib/infra"
)

// FIXME: How to recycle the x-conc-skl node and indices and avoid the data race?

// The pool is used to recycle the auxiliary data structure.
type xConcSklArena[K infra.OrderedKey, V any] struct {
	preAllocNodes     uint32
	allocNodesIncr    uint32
	nodeQueue         []*xConcSklNode[K, V]
	releasedNodeQueue []*xConcSklNode[K, V]
}

func newXConcSklArena[K infra.OrderedKey, V any](allocNodes, allocNodesIncr uint32) *xConcSklArena[K, V] {
	p := &xConcSklArena[K, V]{
		allocNodesIncr: allocNodesIncr,
		nodeQueue:      make([]*xConcSklNode[K, V], allocNodes),
	}
	return p
}
