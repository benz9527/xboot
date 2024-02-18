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

// safeChannel is a generic channel wrapper.
// Why we need this wrapper? For the following reasons:
// 1. We need to make sure the channel is closed only once.
type safeChannel[T comparable] struct {
	queueC   chan T // Receive data to temporary queue.
	isClosed atomic.Bool
	once     *sync.Once
}

func (c *safeChannel[T]) IsClosed() bool {
	return c.isClosed.Load()
}

// Close According to the Go memory model, a send operation on a channel happens before
// the corresponding to receive from that channel completes
// https://go.dev/doc/articles/race_detector
func (c *safeChannel[T]) Close() error {
	c.once.Do(func() {
		c.isClosed.Store(true)
	})
	return nil
}

func (c *safeChannel[T]) Wait() <-chan T {
	return c.queueC
}

func (c *safeChannel[T]) Send(v T, nonBlocking ...bool) error {
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
	_ ReadOnlyChannel[struct{}] = &safeChannel[struct{}]{} // type check assertion
	_ SendOnlyChannel[struct{}] = &safeChannel[struct{}]{} // type check assertion
)

func NewSafeChannel[T comparable](chSize ...int) ClosableChannel[T] {
	isNoCacheCh := true
	size := 1
	if len(chSize) > 0 {
		if chSize[0] > 0 {
			size = chSize[0]
			isNoCacheCh = false
		}
	}
	if isNoCacheCh {
		return &safeChannel[T]{
			queueC: make(chan T),
			once:   &sync.Once{},
		}
	}
	return &safeChannel[T]{
		queueC: make(chan T, size),
		once:   &sync.Once{},
	}
}
