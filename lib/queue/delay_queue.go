package queue

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/benz9527/xboot/lib/ipc"
)

type dqItem[E comparable] struct {
	PQItem[E]
}

func (item *dqItem[E]) Expiration() int64 {
	return item.Priority()
}

func NewDelayQueueItem[E comparable](val E, exp int64) DQItem[E] {
	return &dqItem[E]{
		PQItem: NewPriorityQueueItem[E](val, exp),
	}
}

type sleepEnum = int32

const (
	wokeUp sleepEnum = iota
	fallAsleep
)

type ArrayDelayQueue[E comparable] struct {
	pq                  PriorityQueue[E]
	itemCounter         atomic.Int64
	workCtx             context.Context
	lock                *sync.Mutex
	exclusion           *sync.Mutex // Avoid multiple poll
	wakeUpC             chan struct{}
	waitForNextExpItemC chan struct{}
	sleeping            int32
}

func (dq *ArrayDelayQueue[E]) popIfExpired(expiredBoundary int64) (item ReadOnlyPQItem[E], deltaMs int64) {
	if dq.pq.Len() == 0 {
		return nil, 0
	}

	item = dq.pq.Peek()
	// priority as expiration
	exp := item.Priority()
	if exp > expiredBoundary {
		// not matched
		return nil, exp - expiredBoundary
	}
	dq.lock.Lock()
	item = dq.pq.Pop()
	dq.lock.Unlock()
	return item, 0
}

func (dq *ArrayDelayQueue[E]) Offer(item E, expiration int64) {
	e := NewDelayQueueItem[E](item, expiration)
	dq.lock.Lock()
	dq.pq.Push(e)
	dq.lock.Unlock()
	if e.Index() == 0 {
		// Highest priority item, wake up the consumer
		if atomic.CompareAndSwapInt32(&dq.sleeping, fallAsleep, wokeUp) {
			dq.wakeUpC <- struct{}{}
		}
	}
	dq.itemCounter.Add(1)
}

func (dq *ArrayDelayQueue[E]) poll(nowFn func() int64, sender ipc.SendOnlyChannel[E]) {
	var timer *time.Timer
	defer func() {
		// FIXME recover defer execution order
		if err := recover(); err != nil {
			slog.Error("delay queue panic recover", "error", err)
		}
		// before exit
		atomic.StoreInt32(&dq.sleeping, wokeUp)

		if timer != nil {
			timer.Stop()
			timer = nil
		}
	}()
	for {
		now := nowFn()
		item, deltaMs := dq.popIfExpired(now)
		if item == nil {
			// No expired item in the queue
			// 1. without any item in the queue
			// 2. all items in the queue are not expired
			atomic.StoreInt32(&dq.sleeping, fallAsleep)

			if deltaMs == 0 {
				// Queue is empty, waiting for new item
				select {
				case <-dq.workCtx.Done():
					return
				case <-dq.wakeUpC:
					// Waiting for an immediately executed item
					continue
				}
			} else if deltaMs > 0 {
				if timer != nil {
					timer.Stop()
				}
				// Avoid to use time.After(), it will create a new timer every time
				// what's worse, the underlay timer will not be GC.
				// Asynchronous timer.
				timer = time.AfterFunc(time.Duration(deltaMs)*time.Millisecond, func() {
					if atomic.SwapInt32(&dq.sleeping, wokeUp) == fallAsleep {
						dq.waitForNextExpItemC <- struct{}{}
					}
				})

				select {
				case <-dq.workCtx.Done():
					return
				case <-dq.wakeUpC:
					continue
				case <-dq.waitForNextExpItemC:
					// Waiting for this item to be expired
					if timer != nil {
						timer.Stop()
						timer = nil
					}
					continue
				}
			}
		}

		// This is an expired item
		if timer != nil {
			// Woke up, stop the wait next expired timer
			timer.Stop()
			timer = nil
		}

		select {
		case <-dq.workCtx.Done():
			return
		default:
			// Waiting for the consumer to consume this item
			// If an external channel is closed, here will be panic
			if !sender.IsClosed() {
				if err := sender.Send(item.Value()); err != nil {
					return
				}
				dq.itemCounter.Add(-1)
			} else {
				return
			}
		}
	}
}

func (dq *ArrayDelayQueue[E]) Len() int64 {
	if dq == nil {
		return 0
	}
	return dq.itemCounter.Load()
}

func NewArrayDelayQueue[E comparable](ctx context.Context, capacity int) DelayQueue[E] {
	dq := &ArrayDelayQueue[E]{
		pq: NewArrayPriorityQueue[E](
			WithArrayPriorityQueueEnableThreadSafe[E](),
			WithArrayPriorityQueueCapacity[E](capacity),
		),
		workCtx:             ctx,
		lock:                &sync.Mutex{},
		exclusion:           &sync.Mutex{},
		wakeUpC:             make(chan struct{}),
		waitForNextExpItemC: make(chan struct{}),
	}
	return dq
}
