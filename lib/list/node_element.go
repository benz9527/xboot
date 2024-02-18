package list

var (
	_ NodeElement[struct{}] = (*nodeElement[struct{}])(nil) // Type check assertion
)

type nodeElement[T comparable] struct {
	prev, next NodeElement[T]
	list       BasicLinkedList[T]
	value      T // The type of value may be a small size type.
	// It should be placed at the end of the struct to avoid taking too much padding.
}

func NewNodeElement[T comparable](v T) NodeElement[T] {
	return newNodeElement[T](v, nil)
}

func newNodeElement[T comparable](v T, list BasicLinkedList[T]) *nodeElement[T] {
	return &nodeElement[T]{
		value: v,
		list:  list,
	}
}

func (e *nodeElement[T]) HasNext() bool {
	return e.next != nil
}

func (e *nodeElement[T]) HasPrev() bool {
	return e.prev != nil
}

func (e *nodeElement[T]) GetNext() NodeElement[T] {
	if e.next == nil {
		return nil
	}
	if _, ok := e.next.(*nodeElement[T]); !ok {
		return nil
	}
	return e.next
}

func (e *nodeElement[T]) GetPrev() NodeElement[T] {
	if e.prev == nil {
		return nil
	}
	if _, ok := e.prev.(*nodeElement[T]); !ok {
		return nil
	}
	return e.prev
}

func (e *nodeElement[T]) GetValue() T {
	return e.value
}

func (e *nodeElement[T]) SetValue(v T) {
	e.value = v
}
