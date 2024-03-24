package list

import (
	"github.com/benz9527/xboot/lib/infra"
)

type xComSklIndex[K infra.OrderedKey, V any] struct {
	// Works for the forward iteration direction.
	succ *xComSklNode[K, V]
	// Ignore the node level span metadata (for rank).
}

func (idx *xComSklIndex[K, V]) forward() *xComSklNode[K, V] {
	if idx == nil {
		return nil
	}
	return idx.succ
}

func (idx *xComSklIndex[K, V]) setForward(succ *xComSklNode[K, V]) {
	if idx == nil {
		return
	}
	idx.succ = succ
}

func newXComSklIndex[K infra.OrderedKey, V any](succ *xComSklNode[K, V]) *xComSklIndex[K, V] {
	return &xComSklIndex[K, V]{
		succ: succ,
	}
}
