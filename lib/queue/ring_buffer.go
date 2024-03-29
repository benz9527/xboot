package queue

// circular queue

import (
	"runtime"
	"sync/atomic"

	"github.com/benz9527/xboot/lib/bits"
)

var (
	_ RingBufferEntry[struct{}] = (*rbEntry[struct{}])(nil)
	_ RingBuffer[struct{}]      = (*xRingBuffer[struct{}])(nil)
)

type rbEntry[T any] struct {
	// index of the element in the ring buffer and
	// it is also the "lock" to protect the value
	// in lock-free mode by atomic operation
	cursor uint64
	value  T
}

func (e *rbEntry[T]) GetValue() T {
	return e.value
}

func (e *rbEntry[T]) GetCursor() uint64 {
	return atomic.LoadUint64(&e.cursor)
}

func (e *rbEntry[T]) Store(cursor uint64, value T) {
	// atomic operation should be called at the end of the function
	// otherwise, the value of cursor may be changed by other goroutines
	// Go1.19 atomic guarantees the sequentially consistent order
	// Go1.18 atomic.Pointer[T]
	e.value = value
	atomic.StoreUint64(&e.cursor, cursor)
}

type xRingBuffer[T any] struct {
	capacityMask uint64
	buffer       []RingBufferEntry[T]
}

const _10M = 10 * 1024 * 1024

func NewXRingBuffer[T any](capacity uint64) RingBuffer[T] {
	if capacity > _10M {
		panic("capacity is too large")
	}
	if bits.IsPowOf2(capacity) {
		capacity = bits.RoundupPowOf2ByCeil(capacity)
		if capacity > _10M {
			panic("capacity is too large")
		}
	}

	rb := &xRingBuffer[T]{
		capacityMask: capacity - 1,
		buffer:       make([]RingBufferEntry[T], capacity),
	}
	for i := uint64(0); i < capacity; i++ {
		rb.buffer[i] = &rbEntry[T]{}
	}
	runtime.SetFinalizer(rb, func(rb *xRingBuffer[T]) {
		clear(rb.buffer)
	})
	return rb
}

func (rb *xRingBuffer[T]) Capacity() uint64 {
	return rb.capacityMask + 1
}

func (rb *xRingBuffer[T]) LoadEntryByCursor(cursor uint64) RingBufferEntry[T] {
	return rb.buffer[cursor&rb.capacityMask]
}
