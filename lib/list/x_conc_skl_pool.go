package list

import (
	"sync"

	"github.com/benz9527/xboot/lib/infra"
)

// FIXME: How to recycle the x-conc-skl node and indices and avoid the data race?

// The pool is used to recycle the auxiliary data structure.
type xConcSklPool[K infra.OrderedKey, V any] struct {
	nodes             uint64
	nodeQueue         []*xConcSklNode[K, V]
	releasedNodeQueue []*xConcSklNode[K, V]
	auxPool           *sync.Pool
	allocNodesIncr    uint32
}

func newXConcSklPool[K infra.OrderedKey, V any](allocNodes, allocNodesIncr uint32) *xConcSklPool[K, V] {
	p := &xConcSklPool[K, V]{
		auxPool: &sync.Pool{
			New: func() any {
				return make(xConcSklAux[K, V], 2*sklMaxLevel)
			},
		},
		allocNodesIncr: allocNodesIncr,
		nodeQueue:      make([]*xConcSklNode[K, V], allocNodes),
		nodes:          uint64(allocNodes),
	}
	for i := 0; i < int(allocNodes); i++ {
		p.nodeQueue[i] = new(xConcSklNode[K, V])
	}
	return p
}

func (p *xConcSklPool[K, V]) allocateNodes() {
	p.nodeQueue = make([]*xConcSklNode[K, V], p.allocNodesIncr)
	for i := 0; i < int(p.allocNodesIncr); i++ {
		p.nodeQueue[i] = new(xConcSklNode[K, V])
	}
	p.nodes = uint64(p.allocNodesIncr)
}

func (p *xConcSklPool[K, V]) loadNode(lvl int32) *xConcSklNode[K, V] {
	total := int(lvl + 1)
	if p.nodes < uint64(total) {
		take := p.nodeQueue[:]
		k := total - len(take)
		p.allocateNodes()
		take = append(take, p.nodeQueue[:k]...)
		p.nodeQueue = p.nodeQueue[k:]
		p.nodes -= uint64(k)
		take[0].indices = take[1:]
		take[0].level = uint32(lvl)
		return take[0]
	}
	take := p.nodeQueue[:total]
	p.nodeQueue = p.nodeQueue[total:]
	p.nodes -= uint64(total)
	take[0].indices = take[1:]
	take[0].level = uint32(lvl)
	return take[0]
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
