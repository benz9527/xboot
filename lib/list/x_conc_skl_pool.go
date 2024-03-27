package list

import (
	"github.com/benz9527/xboot/lib/infra"
)

// FIXME: How to recycle the x-conc-skl node and indices and avoid the data race?

// The pool is used to recycle the auxiliary data structure.
type xConcSklPool[K infra.OrderedKey, V any] struct {
	preAllocNodes     uint32
	allocNodesIncr    uint32
	nodeQueue         []*xConcSklNode[K, V]
	releasedNodeQueue []*xConcSklNode[K, V]
}

func newXConcSklPool[K infra.OrderedKey, V any](allocNodes, allocNodesIncr uint32) *xConcSklPool[K, V] {
	p := &xConcSklPool[K, V]{
		allocNodesIncr: allocNodesIncr,
		nodeQueue:      make([]*xConcSklNode[K, V], allocNodes),
	}
	return p
}

