package list

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

type HashObject interface {
	comparable
	Hash() uint64
}

type emptyHashObject struct{}

func (o *emptyHashObject) Hash() uint64 { return 0 }

type SkipListWeight interface {
	~string | ~int8 | ~int16 | ~int32 | ~int64 | ~int |
		~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uint |
		~float32 | ~float64 |
		~complex64 | ~complex128
	// ~uint8 == byte
}

type SkipListElement[W SkipListWeight, O HashObject] interface {
	Weight() W
	Object() O
}

type SkipListNode[W SkipListWeight, O HashObject] interface {
	Free()
	Element() SkipListElement[W, O]
	setElement(e SkipListElement[W, O])
	backwardPredecessor() SkipListNode[W, O]
	setBackwardPredecessor(pred SkipListNode[W, O])
	levels() []SkipListLevel[W, O]
}

type SkipListLevel[W SkipListWeight, O HashObject] interface {
	forwardSuccessor() SkipListNode[W, O]
	setForwardSuccessor(succ SkipListNode[W, O])
}

type SkipList[W SkipListWeight, O HashObject] interface {
	Level() uint32
	Len() uint32
	Free()
	ForEach(fn func(idx int64, weight W, object O))
	Insert(weight W, object O) (SkipListNode[W, O], bool)
	RemoveFirst(weight W) SkipListElement[W, O]
	RemoveAll(weight W) []SkipListElement[W, O]
	RemoveIfMatch(weight W, cmp SkipListObjectMatcher[O]) []SkipListElement[W, O]
	FindFirst(weight W) SkipListElement[W, O]
	FindAll(weight W) []SkipListElement[W, O]
	FindIfMatch(weight W, cmp SkipListObjectMatcher[O]) []SkipListElement[W, O]
	PopHead() SkipListElement[W, O]
}

// SkipListWeightComparator
// Assume j is the weight of the new element.
//  1. i == j (i-j == 0, return 0), contains the same element.
//     If insert a new element, we have to linear probe the next position that can be inserted.
//  2. i > j (i-j > 0, return 1), find left part.
//  3. i < j (i-j < 0, return -1), find right part.
type SkipListWeightComparator[W SkipListWeight] func(i, j W) int

// SkipListObjectMatcher
// return true, if object is satisfied the query condition.
// return false is used to find predecessor, for the reason that those same weight elements
//
//	need to do iteration.
type SkipListObjectMatcher[O HashObject] func(obj O) bool

type SkipListRand func(maxLevel int, currentElements uint32) int32
