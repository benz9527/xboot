// Reference:
// https://github.com/nsqio/nsq/blob/master/internal/pqueue/pqueue.go

package queue

import "github.com/benz9527/xboot/lib/ipc"

type PriorityQueue[E comparable] interface {
	Len() int64
	Push(item PQItem[E])
	Pop() ReadOnlyPQItem[E]
	Peek() ReadOnlyPQItem[E]
}

type ReadOnlyPQItem[E comparable] interface {
	Index() int64
	Value() E
	Priority() int64
}

type CmpEnum int64

const (
	iLTj CmpEnum = -1 + iota
	iEQj
	iGTj
)

// PQItemLessThenComparator
// Priority queue item comparator
// if return 1, i > j
// if return 0, i == j
// if return -1, i < j
type PQItemLessThenComparator[E comparable] func(i, j ReadOnlyPQItem[E]) CmpEnum

type PQItem[E comparable] interface {
	ReadOnlyPQItem[E]
	SetIndex(idx int64)
	SetPriority(pri int64)
}

type DelayQueue[E comparable] interface {
	Offer(item E, expiration int64)
	// PollToChan Asynchronous function
	PollToChan(nowFn func() int64, C ipc.SendOnlyChannel[E])
	Len() int64
}

type DQItem[E comparable] interface {
	Expiration() int64
	PQItem[E]
}
