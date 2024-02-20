package queue

import (
	"context"
	"testing"
	"time"
	"unsafe"

	"github.com/stretchr/testify/assert"

	"github.com/benz9527/xboot/lib/ipc"
)

func TestDelayQueueAlignmentAndSize(t *testing.T) {
	ctx := context.TODO()
	dq := NewArrayDelayQueue[*employee](ctx, 32)
	t.Logf("dq aligment size: %d\n", unsafe.Alignof(dq))
	prototype := dq.(*ArrayDelayQueue[*employee])
	t.Logf("dq prototype alignment size: %d\n", unsafe.Alignof(prototype))
	t.Logf("dq prototype wake up channel alignment size: %d\n", unsafe.Alignof(prototype.wakeUpC))
	t.Logf("dq prototype priority queue alignment size: %d\n", unsafe.Alignof(prototype.pq))
	t.Logf("dq prototype sleeping alignment size: %d\n", unsafe.Alignof(prototype.sleeping))

	t.Logf("dq size: %d\n", unsafe.Sizeof(dq))
	t.Logf("dq prototype size: %d\n", unsafe.Sizeof(prototype))
	t.Logf("dq prototype wake up channel size: %d\n", unsafe.Sizeof(prototype.wakeUpC))
	t.Logf("dq prototype priority queue size: %d\n", unsafe.Sizeof(prototype.pq))
	t.Logf("dq prototype sleeping size: %d\n", unsafe.Sizeof(prototype.sleeping))
}

func TestArrayDelayQueue_PollToChan(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	dq := NewArrayDelayQueue[*employee](ctx, 32)
	receiver := ipc.NewSafeClosableChannel[*employee]()
	go dq.PollToChan(
		func() int64 {
			return time.Now().UnixMilli()
		},
		receiver,
	)

	ms := time.Now().UnixMilli()
	dq.Offer(&employee{age: 10, name: "p0", salary: ms + 110}, ms+110)
	dq.Offer(&employee{age: 101, name: "p1", salary: ms + 501}, ms+501)
	dq.Offer(&employee{age: 10, name: "p2", salary: ms + 155}, ms+155)
	dq.Offer(&employee{age: 200, name: "p3", salary: ms + 210}, ms+210)
	dq.Offer(&employee{age: 3, name: "p4", salary: ms + 60}, ms+60)
	dq.Offer(&employee{age: 1, name: "p5", salary: ms + 110}, ms+110)
	dq.Offer(&employee{age: 5, name: "p6", salary: ms + 250}, ms+250)
	dq.Offer(&employee{age: 200, name: "p7", salary: ms + 301}, ms+301)

	expectedCount := 8
	actualCount := 0
	defer func() {
		assert.LessOrEqual(t, actualCount, expectedCount)
	}()

	time.AfterFunc(300*time.Millisecond, func() {
		_ = receiver.Close()
	})
	itemC := receiver.Wait()
	for {
		select {
		default:
			if receiver.IsClosed() {
				return
			}
		case item, ok := <-itemC:
			if !ok {
				t.Log("receiver channel closed")
				time.Sleep(100 * time.Millisecond)
				return
			}
			now := time.Now().UTC().UnixMilli()
			t.Logf("current time ms: %d, item: %v, diff: %d\n", now, item, now-item.salary)
			actualCount++
		}
	}
}

func BenchmarkDelayQueue_PollToChan(b *testing.B) {

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(b.N+10)*time.Millisecond)
	defer cancel()

	dq := NewArrayDelayQueue[*employee](ctx, 32)

	receiver := ipc.NewSafeClosableChannel[*employee]()
	go dq.PollToChan(
		func() int64 {
			return time.Now().UnixMilli()
		},
		receiver,
	)
	go func(ctx context.Context) {
		<-ctx.Done()
		_ = receiver.Close()
	}(ctx)
	ms := time.Now().UnixMilli()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dq.Offer(&employee{age: i, name: "p", salary: int64(i)}, ms+int64(i))
	}

	defer func() {
		b.StopTimer()
		b.ReportAllocs()
	}()
	itemC := receiver.Wait()
	for {
		select {
		default:
			if receiver.IsClosed() {
				return
			}
		case <-itemC:

		}
	}
}
