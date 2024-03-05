package list

import "sync/atomic"

type xConcurrentSkipListIndex[W SkipListWeight, O HashObject] struct {
	node  *atomic.Pointer[xConcurrentSkipListNode[W, O]]
	right *atomic.Pointer[xConcurrentSkipListIndex[W, O]]
	down  *atomic.Pointer[xConcurrentSkipListIndex[W, O]]
}

func newXConcurrentSkipListIndex[W SkipListWeight, O HashObject](
	node *xConcurrentSkipListNode[W, O],
	right, down *xConcurrentSkipListIndex[W, O],
) *xConcurrentSkipListIndex[W, O] {
	idx := &xConcurrentSkipListIndex[W, O]{
		node:  &atomic.Pointer[xConcurrentSkipListNode[W, O]]{},
		right: &atomic.Pointer[xConcurrentSkipListIndex[W, O]]{},
		down:  &atomic.Pointer[xConcurrentSkipListIndex[W, O]]{},
	}
	idx.node.Store(node)
	idx.right.Store(right)
	idx.down.Store(down)
	return idx
}

func rightCompareAndSwap[W SkipListWeight, O HashObject](
	addr *atomic.Pointer[xConcurrentSkipListIndex[W, O]],
	old, new *xConcurrentSkipListIndex[W, O],
) bool {
	return addr.Load().right.CompareAndSwap(old, new)
}

func headCompareAndSwap[W SkipListWeight, O HashObject](
	addr *atomic.Pointer[xConcurrentSkipListIndex[W, O]],
	old, new *xConcurrentSkipListIndex[W, O],
) bool {
	return addr.CompareAndSwap(old, new)
}
