package queue

import (
	"fmt"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"
)

type employee struct {
	name   string
	age    int
	salary int64
}

func TestPriorityQueueItemAlignmentAndSize(t *testing.T) {
	item := NewPriorityQueueItem[*employee](&employee{age: 10, name: "p0"}, 1)
	t.Logf("item alignment size: %d\n", unsafe.Alignof(item))
	prototype := item.(*pqItem[*employee])
	t.Logf("item prototype alignment size: %d\n", unsafe.Alignof(prototype))
	t.Logf("item prototype value alignment size: %d\n", unsafe.Alignof(prototype.value))
	t.Logf("item prototype priority alignment size: %d\n", unsafe.Alignof(prototype.priority))
	t.Logf("item prototype index alignment size: %d\n", unsafe.Alignof(prototype.index))
	t.Logf("item prototype size: %d\n", unsafe.Sizeof(prototype))
	t.Logf("item prototype value size: %d\n", unsafe.Sizeof(prototype.value))
	t.Logf("item prototype priority size: %d\n", unsafe.Sizeof(prototype.priority))
	t.Logf("item prototype index size: %d\n", unsafe.Sizeof(prototype.index))
}

func TestPriorityQueue_MinValueAsHighPriority(t *testing.T) {
	pq := NewArrayPriorityQueue[*employee](
		WithArrayPriorityQueueEnableThreadSafe[*employee](),
		WithArrayPriorityQueueCapacity[*employee](32),
		WithArrayPriorityQueueComparator[*employee](func(i, j ReadOnlyPQItem[*employee]) CmpEnum {
			res := i.Priority() - j.Priority()
			if res > 0 {
				return iGTj
			} else if res < 0 {
				return iLTj
			}
			return iEQj
		}),
	)
	pq.Push(NewPriorityQueueItem[*employee](&employee{age: 10, name: "p0"}, 1))
	pq.Push(NewPriorityQueueItem[*employee](&employee{age: 101, name: "p1"}, 101))
	pq.Push(NewPriorityQueueItem[*employee](&employee{age: 10, name: "p2"}, 10))
	pq.Push(NewPriorityQueueItem[*employee](&employee{age: 200, name: "p3"}, 200))
	pq.Push(NewPriorityQueueItem[*employee](&employee{age: 3, name: "p4"}, 3))
	pq.Push(NewPriorityQueueItem[*employee](&employee{age: 1, name: "p5"}, 1))
	pq.Push(NewPriorityQueueItem[*employee](&employee{age: 5, name: "p6"}, 5))

	expectedPriorities := []int64{1, 1, 3, 5, 10, 101, 200}
	for i, priority := range expectedPriorities {
		item := pq.Pop()
		t.Logf("%v， priority: %d", item.Value(), item.Priority())
		assert.Equal(t, priority, item.Priority(), "priority", i)
	}
}

func TestPriorityQueue_MaxValueAsHighPriority(t *testing.T) {
	pq := NewArrayPriorityQueue[*employee](
		WithArrayPriorityQueueEnableThreadSafe[*employee](),
		WithArrayPriorityQueueCapacity[*employee](32),
		WithArrayPriorityQueueComparator[*employee](func(i, j ReadOnlyPQItem[*employee]) CmpEnum {
			res := j.Priority() - i.Priority()
			if res > 0 {
				return iGTj
			} else if res < 0 {
				return iLTj
			}
			return iEQj
		}),
	)
	pq.Push(NewPriorityQueueItem[*employee](&employee{age: 10, name: "p0"}, 1))
	pq.Push(NewPriorityQueueItem[*employee](&employee{age: 101, name: "p1"}, 101))
	pq.Push(NewPriorityQueueItem[*employee](&employee{age: 10, name: "p2"}, 10))
	pq.Push(NewPriorityQueueItem[*employee](&employee{age: 200, name: "p3"}, 200))
	pq.Push(NewPriorityQueueItem[*employee](&employee{age: 3, name: "p4"}, 3))
	pq.Push(NewPriorityQueueItem[*employee](&employee{age: 1, name: "p5"}, 1))
	pq.Push(NewPriorityQueueItem[*employee](&employee{age: 5, name: "p6"}, 5))
	pq.Push(NewPriorityQueueItem[*employee](&employee{age: 200, name: "p7"}, 201))

	expectedPriorities := []int64{201, 200, 101, 10, 5, 3, 1, 1}
	for i, priority := range expectedPriorities {
		item := pq.Pop()
		t.Logf("%v， priority: %d", item.Value(), item.Priority())
		assert.Equal(t, priority, item.Priority(), "priority", i)
	}
}

func TestPriorityQueue_MinValueAsHighPriority_Peek(t *testing.T) {
	pq := NewArrayPriorityQueue[*employee](
		WithArrayPriorityQueueEnableThreadSafe[*employee](),
		WithArrayPriorityQueueCapacity[*employee](32),
		WithArrayPriorityQueueComparator[*employee](func(i, j ReadOnlyPQItem[*employee]) CmpEnum {
			res := i.Priority() - j.Priority()
			if res > 0 {
				return iGTj
			} else if res < 0 {
				return iLTj
			}
			return iEQj
		}),
	)
	pq.Push(NewPriorityQueueItem[*employee](&employee{age: 200, name: "p3"}, 200))
	pq.Push(NewPriorityQueueItem[*employee](&employee{age: 200, name: "p7"}, 201))
	pq.Push(NewPriorityQueueItem[*employee](&employee{age: 10, name: "p2"}, 10))
	pq.Push(NewPriorityQueueItem[*employee](&employee{age: 3, name: "p4"}, 3))
	pq.Push(NewPriorityQueueItem[*employee](&employee{age: 10, name: "p0"}, 1))
	pq.Push(NewPriorityQueueItem[*employee](&employee{age: 101, name: "p1"}, 101))
	pq.Push(NewPriorityQueueItem[*employee](&employee{age: 1, name: "p5"}, 1))
	pq.Push(NewPriorityQueueItem[*employee](&employee{age: 5, name: "p6"}, 5))

	expectedPriorities := []int64{1, 1, 3, 5, 10, 101, 200, 201}
	for i, priority := range expectedPriorities {
		peekItem := pq.Peek()
		t.Logf("peek item: %v， priority: %d", peekItem.Value(), peekItem.Priority())
		item := pq.Pop()
		t.Logf("%v， priority: %d", item.Value(), item.Priority())
		assert.Equal(t, priority, item.Priority(), "priority", i)
	}
}

func TestPriorityQueue_MaxValueAsHighPriority_Peek(t *testing.T) {
	pq := NewArrayPriorityQueue[*employee](
		WithArrayPriorityQueueEnableThreadSafe[*employee](),
		WithArrayPriorityQueueCapacity[*employee](32),
		WithArrayPriorityQueueComparator[*employee](func(i, j ReadOnlyPQItem[*employee]) CmpEnum {
			res := j.Priority() - i.Priority()
			if res > 0 {
				return iGTj
			} else if res < 0 {
				return iLTj
			}
			return iEQj
		}),
	)
	pq.Push(NewPriorityQueueItem[*employee](&employee{age: 10, name: "p0"}, 1))
	pq.Push(NewPriorityQueueItem[*employee](&employee{age: 101, name: "p1"}, 101))
	pq.Push(NewPriorityQueueItem[*employee](&employee{age: 10, name: "p2"}, 10))
	pq.Push(NewPriorityQueueItem[*employee](&employee{age: 200, name: "p3"}, 200))
	pq.Push(NewPriorityQueueItem[*employee](&employee{age: 3, name: "p4"}, 3))
	pq.Push(NewPriorityQueueItem[*employee](&employee{age: 1, name: "p5"}, 1))
	pq.Push(NewPriorityQueueItem[*employee](&employee{age: 5, name: "p6"}, 5))
	pq.Push(NewPriorityQueueItem[*employee](&employee{age: 200, name: "p7"}, 201))

	expectedPriorities := []int64{201, 200, 101, 10, 5, 3, 1, 1}
	for i, priority := range expectedPriorities {
		peekItem := pq.Peek()
		t.Logf("peek item: %v， priority: %d", peekItem.Value(), peekItem.Priority())
		item := pq.Pop()
		t.Logf("%v， priority: %d", item.Value(), item.Priority())
		assert.Equal(t, priority, item.Priority(), "priority", i)
	}
}

func BenchmarkArrayPriorityQueue_Push(b *testing.B) {
	var list = make([]PQItem[*employee], 0, b.N)
	for i := 0; i < b.N; i++ {
		e := NewPriorityQueueItem[*employee](&employee{age: i, name: fmt.Sprintf("p%d", i)}, int64(i))
		list = append(list, e)
	}
	b.ResetTimer()
	pq := NewArrayPriorityQueue[*employee](
		WithArrayPriorityQueueEnableThreadSafe[*employee](),
		WithArrayPriorityQueueCapacity[*employee](32),
		WithArrayPriorityQueueComparator[*employee](func(i, j ReadOnlyPQItem[*employee]) CmpEnum {
			res := j.Priority() - i.Priority()
			if res > 0 {
				return iGTj
			} else if res < 0 {
				return iLTj
			}
			return iEQj
		}),
	)
	for i := 0; i < b.N; i++ {
		pq.Push(list[i])
	}
	b.ReportAllocs()
}

func BenchmarkArrayPriorityQueue_Pop(b *testing.B) {
	var list = make([]PQItem[*employee], 0, b.N)
	for i := 0; i < b.N; i++ {
		e := NewPriorityQueueItem[*employee](&employee{age: i, name: fmt.Sprintf("p%d", i)}, int64(i))
		list = append(list, e)
	}
	pq := NewArrayPriorityQueue[*employee](
		WithArrayPriorityQueueEnableThreadSafe[*employee](),
		WithArrayPriorityQueueCapacity[*employee](32),
	)
	for i := 0; i < b.N; i++ {
		pq.Push(list[i])
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = pq.Pop()
	}
	b.ReportAllocs()
}
