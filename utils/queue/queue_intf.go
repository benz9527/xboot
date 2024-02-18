// Reference:
// https://github.com/nsqio/nsq/blob/master/internal/pqueue/pqueue.go

package queue

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
