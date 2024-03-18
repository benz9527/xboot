package list

import (
	"github.com/benz9527/xboot/lib/infra"
)

// Note that the singly linked list is not thread safe.
// And the singly linked list could be implemented by using the doubly linked list.
// So it is a meaningless exercise to implement the singly linked list.

// NodeElement is the basic interface for node element.
// Alignment of interface is 8 bytes and size of interface is 16 bytes.
type NodeElement[T comparable] interface {
	HasNext() bool
	GetNext() NodeElement[T]
	HasPrev() bool
	GetPrev() NodeElement[T]
	GetValue() T
	SetValue(v T) // Concurrent data race error
}

// BasicLinkedList is a singly linked list interface.
type BasicLinkedList[T comparable] interface {
	Len() int64
	// Append appends the elements to the list l and returns the new elements.
	Append(elements ...NodeElement[T]) []NodeElement[T]
	// AppendValue appends the values to the list l and returns the new elements.
	AppendValue(values ...T) []NodeElement[T]
	// InsertAfter inserts a value v as a new element immediately after element dstE and returns new element.
	// If e is nil, the value v will not be inserted.
	InsertAfter(v T, dstE NodeElement[T]) NodeElement[T]
	// InsertBefore inserts a value v as a new element immediately before element dstE and returns new element.
	// If e is nil, the value v will not be inserted.
	InsertBefore(v T, dstE NodeElement[T]) NodeElement[T]
	// Remove removes targetE from l if targetE is an element of list l and returns targetE or nil if the list is empty.
	Remove(targetE NodeElement[T]) NodeElement[T]
	// ForEach traverses the list l and executes function fn for each element.
	ForEach(fn func(idx int64, e NodeElement[T]))
	// FindFirst finds the first element that satisfies the compareFn and returns the element and true if found.
	// If compareFn is not provided, it will use the default compare function that compares the value of element.
	FindFirst(v T, compareFn ...func(e NodeElement[T]) bool) (NodeElement[T], bool)
}

// LinkedList is the doubly linked list interface.
type LinkedList[T comparable] interface {
	BasicLinkedList[T]
	// ReverseForEach iterates the list in reverse order, calling fn for each element,
	// until either all elements have been visited.
	ReverseForEach(fn func(idx int64, e NodeElement[T]))
	// Front returns the first element of doubly linked list l or nil if the list is empty.
	Front() NodeElement[T]
	// Back returns the last element of doubly linked list l or nil if the list is empty.
	Back() NodeElement[T]
	// PushFront inserts a new element e with value v at the front of list l and returns e.
	PushFront(v T) NodeElement[T]
	// PushBack inserts a new element e with value v at the back of list l and returns e.
	PushBack(v T) NodeElement[T]
	// MoveToFront moves an element e to the front of list l.
	MoveToFront(targetE NodeElement[T])
	// MoveToBack moves an element e to the back of list l.
	MoveToBack(targetE NodeElement[T])
	// MoveBefore moves an element srcE in front of element dstE.
	MoveBefore(srcE, dstE NodeElement[T])
	// MoveAfter moves an element srcE next to element dstE.
	MoveAfter(srcE, dstE NodeElement[T])
	// PushFrontList inserts a copy of another linked list at the front of list l.
	PushFrontList(srcList LinkedList[T])
	// PushBackList inserts a copy of another linked list at the back of list l.
	PushBackList(srcList LinkedList[T])
}

type SkipListElement[K infra.OrderedKey, V comparable] interface {
	Key() K
	Val() V
}

type SkipListIterationItem[K infra.OrderedKey, V comparable] interface {
	SkipListElement[K, V]
	NodeLevel() uint32
	NodeItemCount() int64
}

type SkipList[K infra.OrderedKey, V comparable] interface {
	Levels() int32
	Len() int64
	IndexCount() uint64
	Insert(key K, val V, ifNotPresent ...bool) error
	LoadFirst(key K) (SkipListElement[K, V], bool)
	RemoveFirst(key K) (SkipListElement[K, V], error)
	Foreach(action func(i int64, item SkipListIterationItem[K, V]) bool)
	PopHead() (SkipListElement[K, V], error)
	PeekHead() SkipListElement[K, V]
}

type XSkipList[K infra.OrderedKey, V comparable] interface {
	SkipList[K, V]
	LoadIfMatched(weight K, matcher func(that V) bool) ([]SkipListElement[K, V], error)
	LoadAll(weight K) ([]SkipListElement[K, V], error)
	RemoveIfMatched(key K, matcher func(that V) bool) ([]SkipListElement[K, V], error)
	RemoveAll(key K) ([]SkipListElement[K, V], error)
}

type SklValComparator[V comparable] func(i, j V) int64
