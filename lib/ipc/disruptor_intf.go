package ipc

import (
	"time"
	"unsafe"

	"golang.org/x/sys/cpu"

	"github.com/benz9527/xboot/lib/queue"
)

const cacheLinePadSize = unsafe.Sizeof(cpu.CacheLinePad{})

type stopper interface {
	Start() error
	Stop() error
	IsStopped() bool
}

type Publisher[T any] interface {
	Publish(event T) (uint64, bool, error)
	PublishTimeout(event T, timeout time.Duration)
}

type Producer[T any] Publisher[T]

type BlockStrategy interface {
	WaitFor(eqFn func() bool)
	Done()
}

type EventHandler[T any] func(event T) error // OnEvent

type Subscriber[T any] interface {
	HandleEvent(event T) error
}

type Sequencer interface {
	Capacity() uint64
	GetReadCursor() queue.RingBufferCursor
	GetWriteCursor() queue.RingBufferCursor
}

type Disruptor[T any] interface {
	Publisher[T]
	stopper
	RegisterSubscriber(sub Subscriber[T]) error
}
