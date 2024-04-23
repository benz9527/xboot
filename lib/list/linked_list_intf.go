package list

// Note that the singly linked list is not thread safe.
// And the singly linked list could be implemented by using the doubly linked list.
// So it is a meaningless exercise to implement the singly linked list.

// BasicLinkedList is a singly linked list interface.
type BasicLinkedList[T comparable] interface {
	Len() int64
	// Append appends the elements to the list l and returns the new elements.
	Append(elements ...*NodeElement[T]) []*NodeElement[T]
	// AppendValue appends the values to the list l and returns the new elements.
	AppendValue(values ...T) []*NodeElement[T]
	// InsertAfter inserts a value v as a new element immediately after element dstE and returns new element.
	// If e is nil, the value v will not be inserted.
	InsertAfter(v T, dstE *NodeElement[T]) *NodeElement[T]
	// InsertBefore inserts a value v as a new element immediately before element dstE and returns new element.
	// If e is nil, the value v will not be inserted.
	InsertBefore(v T, dstE *NodeElement[T]) *NodeElement[T]
	// Remove removes targetE from l if targetE is an element of list l and returns targetE or nil if the list is empty.
	Remove(targetE *NodeElement[T]) *NodeElement[T]
	// Foreach traverses the list l and executes function fn for each element.
	// If fn returns an error, the traversal stops and returns the error.
	Foreach(fn func(idx int64, e *NodeElement[T]) error) error
	// FindFirst finds the first element that satisfies the compareFn and returns the element and true if found.
	// If compareFn is not provided, it will use the default compare function that compares the value of element.
	FindFirst(v T, compareFn ...func(e *NodeElement[T]) bool) (*NodeElement[T], bool)
}

// LinkedList is the doubly linked list interface.
type LinkedList[T comparable] interface {
	BasicLinkedList[T]
	// ReverseForeach iterates the list in reverse order, calling fn for each element,
	// until either all elements have been visited.
	ReverseForeach(fn func(idx int64, e *NodeElement[T]))
	// Front returns the first element of doubly linked list l or nil if the list is empty.
	Front() *NodeElement[T]
	// Back returns the last element of doubly linked list l or nil if the list is empty.
	Back() *NodeElement[T]
	// PushFront inserts a new element e with value v at the front of list l and returns e.
	PushFront(v T) *NodeElement[T]
	// PushBack inserts a new element e with value v at the back of list l and returns e.
	PushBack(v T) *NodeElement[T]
	// MoveToFront moves an element e to the front of list l.
	MoveToFront(targetE *NodeElement[T]) bool
	// MoveToBack moves an element e to the back of list l.
	MoveToBack(targetE *NodeElement[T]) bool
	// MoveBefore moves an element srcE in front of element dstE.
	MoveBefore(srcE, dstE *NodeElement[T]) bool
	// MoveAfter moves an element srcE next to element dstE.
	MoveAfter(srcE, dstE *NodeElement[T]) bool
	// PushFrontList inserts a copy of another linked list at the front of list l.
	PushFrontList(srcList LinkedList[T])
	// PushBackList inserts a copy of another linked list at the back of list l.
	PushBackList(srcList LinkedList[T])
}
