package list

import (
	"sync"

	"github.com/benz9527/xboot/lib/infra"
)

type xConcSklPool[K infra.OrderedKey, V comparable] struct {
	auxPool *sync.Pool
}

func newXConcSklPool[K infra.OrderedKey, V comparable]() *xConcSklPool[K, V] {
	p := &xConcSklPool[K, V]{
		auxPool: &sync.Pool{
			New: func() any {
				return make(xConcSklAux[K, V], 2*xSkipListMaxLevel)
			},
		},
	}
	return p
}

func (p *xConcSklPool[K, V]) loadAux() xConcSklAux[K, V] {
	return p.auxPool.Get().(xConcSklAux[K, V])
}

func (p *xConcSklPool[K, V]) releaseAux(aux xConcSklAux[K, V]) {
	// Override only
	p.auxPool.Put(aux)
}
