package list

import (
	"sync"

	"github.com/benz9527/xboot/lib/infra"
)

// FIXME: How to recycle the x-conc-skl node and indices and avoid the data race?

// The pool is used to recycle the auxiliary data structure.
type xConcSklPool[K infra.OrderedKey, V any] struct {
	releasedNodeQueue []*xConcSklNode[K, V]
	auxPool           *sync.Pool
	nodePool          *sync.Pool
}

func newXConcSklPool[K infra.OrderedKey, V any](allocNodes, allocNodesIncr uint32) *xConcSklPool[K, V] {
	p := &xConcSklPool[K, V]{
		auxPool: &sync.Pool{
			New: func() any {
				return make(xConcSklAux[K, V], 2*sklMaxLevel)
			},
		},
		nodePool: &sync.Pool{
			New: func() any {
				return new(xConcSklNode[K, V])
			},
		},
	}
	return p
}

func (p *xConcSklPool[K, V]) loadNode() *xConcSklNode[K, V] {
	return p.nodePool.Get().(*xConcSklNode[K, V])
}

func (p *xConcSklPool[K, V]) loadAux() xConcSklAux[K, V] {
	return p.auxPool.Get().(xConcSklAux[K, V])
}

func (p *xConcSklPool[K, V]) releaseAux(aux xConcSklAux[K, V]) {
	// Override only
	p.auxPool.Put(aux)
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
