package ipc

import (
	"sync/atomic"
	"time"

	"github.com/benz9527/xboot/lib/bits"
	"github.com/benz9527/xboot/lib/infra"
	"github.com/benz9527/xboot/lib/queue"
)

type disruptorStatus int32

const (
	disruptorReady disruptorStatus = iota
	disruptorRunning
)

type xDisruptor[T any] struct {
	pub interface {
		Publisher[T]
		stopper
	}
	sub interface {
		Subscriber[T]
		stopper
	}
	status disruptorStatus
}

func NewXDisruptor[T any](
	capacity uint64,
	strategy BlockStrategy,
	handler EventHandler[T],
) Disruptor[T] {
	capacity = bits.RoundupPowOf2ByCeil(capacity)
	if capacity < 2 {
		capacity = 2
	}
	seq := NewXSequencer(capacity)
	// Can't start from 0, because 0 will be treated as nil value
	seq.GetWriteCursor().Next()
	seq.GetReadCursor().Next()
	rb := queue.NewXRingBuffer[T](capacity)
	pub := newXPublisher[T](seq, rb, strategy)
	sub := newXSubscriber[T](rb, handler, seq, strategy)
	d := &xDisruptor[T]{
		pub:    pub,
		sub:    sub,
		status: disruptorReady,
	}
	return d
}

func (dis *xDisruptor[T]) Start() error {
	if atomic.CompareAndSwapInt32((*int32)(&dis.status), int32(disruptorReady), int32(disruptorRunning)) {
		if err := dis.sub.Start(); err != nil {
			atomic.StoreInt32((*int32)(&dis.status), int32(disruptorReady))
			return infra.WrapErrorStack(err)
		}
		if err := dis.pub.Start(); err != nil {
			atomic.StoreInt32((*int32)(&dis.status), int32(disruptorReady))
			return infra.WrapErrorStack(err)
		}
		return nil
	}
	return infra.NewErrorStack("[disruptor] already started")
}

func (dis *xDisruptor[T]) Stop() error {
	if atomic.CompareAndSwapInt32((*int32)(&dis.status), int32(disruptorRunning), int32(disruptorReady)) {
		if err := dis.pub.Stop(); err != nil {
			atomic.CompareAndSwapInt32((*int32)(&dis.status), int32(disruptorRunning), int32(disruptorReady))
			return infra.WrapErrorStack(err)
		}
		if err := dis.sub.Stop(); err != nil {
			atomic.CompareAndSwapInt32((*int32)(&dis.status), int32(disruptorRunning), int32(disruptorReady))
			return infra.WrapErrorStack(err)
		}
		return nil
	}
	return infra.NewErrorStack("[disruptor] already stopped")
}

func (dis *xDisruptor[T]) IsStopped() bool {
	return atomic.LoadInt32((*int32)(&dis.status)) != int32(disruptorRunning)
}

func (dis *xDisruptor[T]) Publish(event T) (uint64, bool, error) {
	return dis.pub.Publish(event)
}

func (dis *xDisruptor[T]) PublishTimeout(event T, timeout time.Duration) {
	dis.pub.PublishTimeout(event, timeout)
}

func (dis *xDisruptor[T]) RegisterSubscriber(sub Subscriber[T]) error {
	// Single pipeline disruptor only support one subscriber to consume the events.
	// It will be registered at the construction.
	return nil
}
