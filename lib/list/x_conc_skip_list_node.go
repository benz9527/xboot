package list

import (
	"sync/atomic"
)

type xConcurrentSkipListNode[W SkipListWeight, O HashObject] struct {
	weight *atomic.Pointer[W]
	object *atomic.Pointer[O]
	next   *atomic.Pointer[xConcurrentSkipListNode[W, O]]
}

func (node *xConcurrentSkipListNode[W, O]) Weight() W {
	w := node.weight.Load()
	return *w
}

func isNilWeight[W SkipListWeight](weight *atomic.Pointer[W]) bool {
	w := weight.Load()
	return w == nil
}

func newXConcurrentSkipListNode[W SkipListWeight, O HashObject](
	weight W, object O, next *xConcurrentSkipListNode[W, O],
) *xConcurrentSkipListNode[W, O] {
	node := &xConcurrentSkipListNode[W, O]{
		weight: &atomic.Pointer[W]{},
		object: &atomic.Pointer[O]{},
		next:   &atomic.Pointer[xConcurrentSkipListNode[W, O]]{},
	}
	if weight == *new(W) {
		node.weight.Store(nil)
	} else {
		node.weight.Store(&weight)
	}
	if object == *new(O) {
		node.object.Store(nil)
	} else {
		node.object.Store(&object)
	}
	node.next.Store(next)
	return node
}

func newMarkerNode[W SkipListWeight, O HashObject](
	deletedNode *xConcurrentSkipListNode[W, O],
) *xConcurrentSkipListNode[W, O] {
	marker := &xConcurrentSkipListNode[W, O]{
		weight: &atomic.Pointer[W]{},
		object: &atomic.Pointer[O]{},
		next:   &atomic.Pointer[xConcurrentSkipListNode[W, O]]{},
	}
	// Splicing the marker node with deleted node
	marker.next.Store(deletedNode)
	return marker
}

func nextCompareAndSet[W SkipListWeight, O HashObject](
	addr *atomic.Pointer[xConcurrentSkipListNode[W, O]],
	old, new *xConcurrentSkipListNode[W, O],
) bool {
	return addr.Load().next.CompareAndSwap(old, new)
}

func objectCompareAndSet[W SkipListWeight, O HashObject](
	addr *atomic.Pointer[xConcurrentSkipListNode[W, O]], old, new O,
) bool {
	return addr.Load().object.CompareAndSwap(&old, &new)
}
