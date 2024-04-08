package list

type NodeElement[T comparable] struct {
	prev, next *NodeElement[T]
	listRef    *doublyLinkedList[T]
	Value      T // The type of value may be a small size type.
	// It should be placed at the end of the struct to avoid taking too much padding.
}

func NewNodeElement[T comparable](v T) *NodeElement[T] {
	return newNodeElement[T](v, nil)
}

func newNodeElement[T comparable](v T, list *doublyLinkedList[T]) *NodeElement[T] {
	return &NodeElement[T]{
		Value:   v,
		listRef: list,
	}
}

func (e *NodeElement[T]) HasNext() bool {
	if e == nil {
		return false
	}
	return e.next != nil && e.next != e.listRef.getRoot()
}

func (e *NodeElement[T]) HasPrev() bool {
	if e == nil {
		return false
	}
	return e.prev != nil && e.prev != e.listRef.getRoot()
}

func (e *NodeElement[T]) Next() *NodeElement[T] {
	if e == nil {
		return nil
	}
	return e.next
}

func (e *NodeElement[T]) Prev() *NodeElement[T] {
	if e == nil {
		return nil
	}
	return e.prev
}
