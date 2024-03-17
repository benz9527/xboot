package list

import (
	"github.com/benz9527/xboot/lib/infra"
)

type xComSklIndex[K infra.OrderedKey, V comparable] struct {
	// Works for the forward iteration direction.
	succ *xComSklNode[K, V]
	// Ignore the node level span metadata (for rank).
}

func (idx *xComSklIndex[K, V]) forwardSuccessor() *xComSklNode[K, V] {
	if idx == nil {
		return nil
	}
	return idx.succ
}

func (idx *xComSklIndex[K, V]) setForwardSuccessor(succ *xComSklNode[K, V]) {
	if idx == nil {
		return
	}
	idx.succ = succ
}

func newSkipListLevel[K infra.OrderedKey, V comparable](succ *xComSklNode[K, V]) *xComSklIndex[K, V] {
	return &xComSklIndex[K, V]{
		succ: succ,
	}
}
