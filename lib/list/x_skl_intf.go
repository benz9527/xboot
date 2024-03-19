package list

import (
	"github.com/benz9527/xboot/lib/infra"
)

type SkipList[K infra.OrderedKey, V comparable] interface {
	Levels() int32
	Len() int64
	IndexCount() uint64
	Insert(key K, val V, ifNotPresent ...bool) error
	LoadFirst(key K) (SklElement[K, V], error)
	RemoveFirst(key K) (SklElement[K, V], error)
	Foreach(action func(i int64, item SklIterationItem[K, V]) bool)
	PopHead() (SklElement[K, V], error)
	PeekHead() SklElement[K, V]
}

type XSkipList[K infra.OrderedKey, V comparable] interface {
	SkipList[K, V]
	LoadIfMatched(weight K, matcher func(that V) bool) ([]SklElement[K, V], error)
	LoadAll(weight K) ([]SklElement[K, V], error)
	RemoveIfMatched(key K, matcher func(that V) bool) ([]SklElement[K, V], error)
	RemoveAll(key K) ([]SklElement[K, V], error)
}

type SklElement[K infra.OrderedKey, V comparable] interface {
	Key() K
	Val() V
}

type SklIterationItem[K infra.OrderedKey, V comparable] interface {
	SklElement[K, V]
	NodeLevel() uint32
	NodeItemCount() int64
}

type SklValComparator[V comparable] func(i, j V) int64
