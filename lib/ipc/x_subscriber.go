package ipc

import (
	"fmt"
	"runtime"
	"sync/atomic"

	"github.com/benz9527/xboot/lib/queue"
)

type subscriberStatus int32

const (
	subReady subscriberStatus = iota
	subRunning
)

const (
	activeSpin  = 4
	passiveSpin = 2
)

type xSubscriber[T any] struct {
	rb       queue.RingBuffer[T]
	seq      Sequencer
	strategy BlockStrategy
	handler  EventHandler[T]
	status   subscriberStatus
	spin     int32
}

func newXSubscriber[T any](
	rb queue.RingBuffer[T],
	handler EventHandler[T],
	seq Sequencer,
	strategy BlockStrategy,
) *xSubscriber[T] {
	ncpu := runtime.NumCPU()
	spin := 0
	if ncpu > 1 {
		spin = activeSpin
	}
	return &xSubscriber[T]{
		status:   subReady,
		seq:      seq,
		rb:       rb,
		strategy: strategy,
		handler:  handler,
		spin:     int32(spin),
	}
}

func (sub *xSubscriber[T]) Start() error {
	if atomic.CompareAndSwapInt32((*int32)(&sub.status), int32(subReady), int32(subRunning)) {
		go sub.eventsHandle()
		return nil
	}
	return fmt.Errorf("subscriber already started")
}

func (sub *xSubscriber[T]) Stop() error {
	if atomic.CompareAndSwapInt32((*int32)(&sub.status), int32(subRunning), int32(subReady)) {
		sub.strategy.Done()
		return nil
	}
	return fmt.Errorf("subscriber already stopped")
}

func (sub *xSubscriber[T]) IsStopped() bool {
	return atomic.LoadInt32((*int32)(&sub.status)) == int32(subReady)
}

func (sub *xSubscriber[T]) eventsHandle() {
	readCursor := sub.seq.GetReadCursor().Load()
	spin := sub.spin
	for {
		if sub.IsStopped() {
			return
		}
		spinCount := int32(0)
		for {
			if sub.IsStopped() {
				return
			}
			e := sub.rb.LoadEntryByCursor(readCursor)
			if e.GetCursor() == readCursor {
				_ = sub.HandleEvent(e.GetValue())
				spinCount = 0
				// FIXME handle error
				readCursor = sub.seq.GetReadCursor().Next()
				break
			} else {
				if spinCount < spin {
					procYield(30)
				} else if spinCount < spin+passiveSpin {
					runtime.Gosched()
				} else {
					sub.strategy.WaitFor(func() bool {
						e := sub.rb.LoadEntryByCursor(readCursor)
						return e.GetCursor() == readCursor
					})
					spinCount = 0
				}
				spinCount++
			}
		}
	}
}

func (sub *xSubscriber[T]) HandleEvent(event T) error {
	//defer sub.strategy.Done() // Slow performance issue
	err := sub.handler(event)
	return err
}
