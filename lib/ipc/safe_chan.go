package ipc

import (
	"fmt"
	"io"
	"sync"
	"sync/atomic"
)

type ReadOnlyChannel[T comparable] interface {
	Wait() <-chan T
}

type SendOnlyChannel[T comparable] interface {
	Send(v T, nonBlocking ...bool) error
	IsClosed() bool
}

type ClosableChannel[T comparable] interface {
	io.Closer
	ReadOnlyChannel[T]
	SendOnlyChannel[T]
}

// safeClosableChannel is a generic channel wrapper.
// Why we need this wrapper? For the following reasons:
//  1. We need to make sure the channel is closed only once.
//  2. We need to make sure that we would not close the channel when there is still sending data.
//     Let the related goroutines exit, then the channel auto-collected by GC.
type safeClosableChannel[T comparable] struct {
	queueC   chan T // Receive data to temporary queue.
	isClosed atomic.Bool
	once     *sync.Once
}

func (c *safeClosableChannel[T]) IsClosed() bool {
	return c.isClosed.Load()
}

// Close According to the Go memory model, a send operation on a channel happens before
// the corresponding to receive from that channel completes
// https://go.dev/doc/articles/race_detector
func (c *safeClosableChannel[T]) Close() error {
	c.once.Do(func() {
		// Note: forbid to call close(queueC) directly,
		// because it will cause panic of "send on closed channel"
		c.isClosed.Store(true)
	})
	return nil
}

func (c *safeClosableChannel[T]) Wait() <-chan T {
	return c.queueC
}

func (c *safeClosableChannel[T]) Send(v T, nonBlocking ...bool) error {
	if c.isClosed.Load() {
		return fmt.Errorf("channel has been closed")
	}

	if len(nonBlocking) <= 0 {
		nonBlocking = []bool{false}
	}
	if !nonBlocking[0] {
		c.queueC <- v
	} else {
		// non blocking send
		select {
		case c.queueC <- v:
		default:

		}
	}
	return nil
}

var (
	_ ReadOnlyChannel[struct{}] = &safeClosableChannel[struct{}]{} // type check assertion
	_ SendOnlyChannel[struct{}] = &safeClosableChannel[struct{}]{} // type check assertion
)

func NewSafeClosableChannel[T comparable](chSize ...int) ClosableChannel[T] {
	isNoCacheCh := true
	size := 1
	if len(chSize) > 0 {
		if chSize[0] > 0 {
			size = chSize[0]
			isNoCacheCh = false
		}
	}
	if isNoCacheCh {
		return &safeClosableChannel[T]{
			queueC: make(chan T),
			once:   &sync.Once{},
		}
	}
	return &safeClosableChannel[T]{
		queueC: make(chan T, size),
		once:   &sync.Once{},
	}
}
