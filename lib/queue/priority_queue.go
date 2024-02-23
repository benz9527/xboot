package queue

import (
	"container/heap"
	"sync"
	"sync/atomic"
)

type pqItem[E comparable] struct {
	priority int64
	index    int64
	value    E
}

func (item *pqItem[E]) Index() int64 {
	if item == nil {
		return -1
	}
	return atomic.LoadInt64(&item.index)
}

func (item *pqItem[E]) Value() (val E) {
	if item == nil {
		// return empty value by default
		return
	}
	return item.value
}

func (item *pqItem[E]) Priority() int64 {
	if item == nil {
		return -1
	}
	return atomic.LoadInt64(&item.priority)
}

func (item *pqItem[E]) SetIndex(idx int64) {
	if item == nil {
		return
	}
	atomic.SwapInt64(&item.index, idx)
}

func (item *pqItem[E]) SetPriority(pri int64) {
	if item == nil {
		return
	}
	atomic.SwapInt64(&item.priority, pri)
}

func NewPriorityQueueItem[E comparable](val E, pri int64) PQItem[E] {
	return &pqItem[E]{
		priority: pri,
		value:    val,
		index:    0,
	}
}

type arrayPQ[E comparable] struct {
	capacity   int
	arr        []PQItem[E]
	comparator PQItemLessThenComparator[E]
}

func (pq *arrayPQ[E]) Len() int { return len(pq.arr) }
func (pq *arrayPQ[E]) Less(i, j int) bool {
	res := pq.comparator(pq.arr[i], pq.arr[j])
	return res == iLTj
}
func (pq *arrayPQ[E]) Swap(i, j int) {
	pq.arr[i], pq.arr[j] = pq.arr[j], pq.arr[i]
	pq.arr[i].SetIndex(int64(i))
	pq.arr[j].SetIndex(int64(j))
}

func (pq *arrayPQ[E]) Pop() interface{} {
	prev := pq.arr
	n := len(prev)
	if n <= 0 {
		return nil
	}

	item := prev[n-1]
	item.SetIndex(-1)
	prev[n-1] = *new(PQItem[E]) // nil object
	pq.arr = prev[:n-1]
	return item
}
func (pq *arrayPQ[E]) Push(i interface{}) {
	item, ok := i.(PQItem[E])
	if !ok {
		return
	}

	prev := pq.arr
	n := len(prev)
	item.SetIndex(int64(n))
	pq.arr = append(pq.arr, item.(PQItem[E]))
}

type ArrayPriorityQueue[E comparable] struct {
	queue *arrayPQ[E]
	lock  *sync.Mutex
}

func (pq *ArrayPriorityQueue[E]) Len() int64 {
	if pq.lock != nil {
		pq.lock.Lock()
		defer pq.lock.Unlock()
	}
	return int64(len(pq.queue.arr))
}

func (pq *ArrayPriorityQueue[E]) Pop() (nilItem ReadOnlyPQItem[E]) {
	if pq.lock != nil {
		pq.lock.Lock()
		defer pq.lock.Unlock()
	}
	if len(pq.queue.arr) == 0 {
		return nil
	}
	item := heap.Pop(pq.queue)
	return item.(ReadOnlyPQItem[E])
}

func (pq *ArrayPriorityQueue[E]) Push(item PQItem[E]) {
	if pq.lock != nil {
		pq.lock.Lock()
		defer pq.lock.Unlock()
	}
	heap.Push(pq.queue, item)
}

func (pq *ArrayPriorityQueue[E]) Peek() ReadOnlyPQItem[E] {
	if pq.lock != nil {
		pq.lock.Lock()
		defer pq.lock.Unlock()
	}
	if len(pq.queue.arr) == 0 {
		return nil
	}
	return pq.queue.arr[0]
}

type ArrayPriorityQueueOption[E comparable] func(*ArrayPriorityQueue[E])

func NewArrayPriorityQueue[E comparable](opts ...ArrayPriorityQueueOption[E]) PriorityQueue[E] {
	pq := &ArrayPriorityQueue[E]{
		queue: new(arrayPQ[E]),
	}
	for _, o := range opts {
		if o != nil {
			o(pq)
		}
	}
	if pq.queue.capacity <= 0 {
		pq.queue.capacity = 64
	}
	if pq.queue.comparator == nil {
		pq.queue.comparator = func(i, j ReadOnlyPQItem[E]) CmpEnum {
			res := i.Priority() - j.Priority()
			if res > 0 {
				return iGTj
			} else if res < 0 {
				return iLTj
			}
			return iEQj
		}
	}
	return pq
}

func WithArrayPriorityQueueCapacity[E comparable](capacity int) ArrayPriorityQueueOption[E] {
	return func(pq *ArrayPriorityQueue[E]) {
		if capacity <= 0 {
			capacity = 64
		}
		pq.queue.capacity = capacity
	}
}

func WithArrayPriorityQueueComparator[E comparable](fn PQItemLessThenComparator[E]) ArrayPriorityQueueOption[E] {
	return func(pq *ArrayPriorityQueue[E]) {
		if fn == nil {
			fn = func(i, j ReadOnlyPQItem[E]) CmpEnum {
				res := i.Priority() - j.Priority()
				if res > 0 {
					return iGTj
				} else if res < 0 {
					return iLTj
				}
				return iEQj
			}
		}
		pq.queue.comparator = fn
	}
}

func WithArrayPriorityQueueEnableThreadSafe[E comparable]() ArrayPriorityQueueOption[E] {
	return func(pq *ArrayPriorityQueue[E]) {
		pq.lock = &sync.Mutex{}
	}
}
