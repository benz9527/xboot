package tree

import "github.com/benz9527/xboot/lib/infra"

// go install golang.org/x/tools/cmd/stringer@latest

//go:generate stringer -type=RBColor
type RBColor uint8

const (
	Black RBColor = iota
	Red
)

//go:generate stringer -type=RBDirection
type RBDirection int8

const (
	Left RBDirection = -1 + iota
	Root
	Right
)

type RBNode[K infra.OrderedKey, V any] interface {
	Key() K
	Val() V
	HasKeyVal() bool
	Color() RBColor
	Left() RBNode[K, V]
	Right() RBNode[K, V]
	Parent() RBNode[K, V]
}

type RBTree[K infra.OrderedKey, V any] interface {
	Len() int64
	Root() RBNode[K, V]
	Insert(key K, val V, ifNotPresent ...bool) error
	Remove(key K) (RBNode[K, V], error)
	RemoveMin() (RBNode[K, V], error)
	Foreach(action func(idx int64, color RBColor, key K, val V) bool)
	Release()
}
