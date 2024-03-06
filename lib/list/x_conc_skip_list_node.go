package list

import (
	"sync/atomic"
)

type xConcurrentSkipListNode[W SkipListWeight, O HashObject] struct {
	weight *atomic.Pointer[W]
	object *atomic.Pointer[O]
	next   *atomic.Pointer[xConcurrentSkipListNode[W, O]]
}

func newXConcurrentSkipListNode[W SkipListWeight, O HashObject](
	weight *W, object *O, next *xConcurrentSkipListNode[W, O],
) *xConcurrentSkipListNode[W, O] {
	node := &xConcurrentSkipListNode[W, O]{
		weight: &atomic.Pointer[W]{},
		object: &atomic.Pointer[O]{},
		next:   &atomic.Pointer[xConcurrentSkipListNode[W, O]]{},
	}
	node.weight.Store(weight)
	node.object.Store(object)
	node.next.Store(next)
	return node
}

func newBaseNode[W SkipListWeight, O HashObject]() *xConcurrentSkipListNode[W, O] {
	base := &xConcurrentSkipListNode[W, O]{
		weight: &atomic.Pointer[W]{},
		object: &atomic.Pointer[O]{},
		next:   &atomic.Pointer[xConcurrentSkipListNode[W, O]]{},
	}
	// Splicing the base node with deleted node
	base.weight.Store(nil)
	base.object.Store(nil)
	base.next.Store(nil)
	return base
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
