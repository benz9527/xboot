package list

import (
	"github.com/benz9527/xboot/lib/infra"
)

// The classic and unique skip list.
type SkipList[K infra.OrderedKey, V any] interface {
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

// The X means the extended interface and it could store duplicate keys and values.
type XSkipList[K infra.OrderedKey, V any] interface {
	SkipList[K, V]
	LoadIfMatch(key K, matcher func(that V) bool) ([]SklElement[K, V], error)
	LoadAll(key K) ([]SklElement[K, V], error)
	RemoveIfMatch(key K, matcher func(that V) bool) ([]SklElement[K, V], error)
	RemoveAll(key K) ([]SklElement[K, V], error)
}

type SklElement[K infra.OrderedKey, V any] interface {
	Key() K
	Val() V
}

type SklIterationItem[K infra.OrderedKey, V any] interface {
	SklElement[K, V]
	NodeLevel() uint32
	NodeItemCount() int64
}

type SklValComparator[V any] func(i, j V) int64
