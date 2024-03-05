package list

import "sync/atomic"

type xConcurrentSkipListIndex[W SkipListWeight, O HashObject] struct {
	node  *atomic.Pointer[xConcurrentSkipListNode[W, O]]
	right *atomic.Pointer[xConcurrentSkipListIndex[W, O]]
	down  *atomic.Pointer[xConcurrentSkipListIndex[W, O]]
}

func newXConcurrentSkipListIndex[W SkipListWeight, O HashObject](
	node *xConcurrentSkipListNode[W, O],
	down, right *xConcurrentSkipListIndex[W, O],
) *xConcurrentSkipListIndex[W, O] {
	idx := &xConcurrentSkipListIndex[W, O]{
		node:  &atomic.Pointer[xConcurrentSkipListNode[W, O]]{},
		down:  &atomic.Pointer[xConcurrentSkipListIndex[W, O]]{},
		right: &atomic.Pointer[xConcurrentSkipListIndex[W, O]]{},
	}
	return idx
}

func rightCompareAndSwap[W SkipListWeight, O HashObject](
	addr *atomic.Pointer[xConcurrentSkipListIndex[W, O]],
	old, new *xConcurrentSkipListIndex[W, O],
) bool {
	return addr.CompareAndSwap(old, new)
}

func headCompareAndSwap[W SkipListWeight, O HashObject](
	addr *atomic.Pointer[xConcurrentSkipListIndex[W, O]],
	old, new *xConcurrentSkipListIndex[W, O],
) bool {
	return addr.CompareAndSwap(old, new)
}
