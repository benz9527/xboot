package list

import (
	"sync/atomic"
)

type xSkipListElement[W SkipListWeight, O HashObject] struct {
	weight W
	object O
}

func (e *xSkipListElement[W, O]) Weight() W {
	return e.weight
}

func (e *xSkipListElement[W, O]) Object() O {
	return e.object
}

type skipListLevel[W SkipListWeight, O HashObject] struct {
	// Works for the forward iteration direction.
	successor SkipListNode[W, O]
	// Ignore the node level span metadata (for rank).
}

func (lvl *skipListLevel[W, O]) forwardSuccessor() SkipListNode[W, O] {
	if lvl == nil {
		return nil
	}
	return lvl.successor
}

func (lvl *skipListLevel[W, O]) setForwardSuccessor(succ SkipListNode[W, O]) {
	if lvl == nil {
		return
	}
	lvl.successor = succ
}

func newSkipListLevel[W SkipListWeight, V HashObject](succ SkipListNode[W, V]) SkipListLevel[W, V] {
	return &skipListLevel[W, V]{
		successor: succ,
	}
}

// The cache level array index > 0, it is the Y axis, and it means that it is the interval after
//
//	the bisection search. Used to locate an element quickly.
//
// The cache level array index == 0, it is the X axis, and it means that it is the data container.
type xSkipListNode[W SkipListWeight, O HashObject] struct {
	// The cache part.
	// When the current node works as a data node, it doesn't contain levels metadata.
	// If a node is a level node, the cache is from levels[0], but it is differed
	//  to the sentinel's levels[0].
	levelList []SkipListLevel[W, O] // The cache level array.
	element   atomic.Pointer[SkipListElement[W, O]]
	// Works for a backward iteration direction.
	predecessor SkipListNode[W, O]
}

func (node *xSkipListNode[W, O]) Element() SkipListElement[W, O] {
	if node == nil {
		return nil
	}
	return *node.element.Load()
}

func (node *xSkipListNode[W, O]) setElement(e SkipListElement[W, O]) {
	if node == nil {
		return
	}
	node.element.Store(&e)
}

func (node *xSkipListNode[W, O]) backwardPredecessor() SkipListNode[W, O] {
	if node == nil {
		return nil
	}
	return node.predecessor
}

func (node *xSkipListNode[W, O]) setBackwardPredecessor(pred SkipListNode[W, O]) {
	if node == nil {
		return
	}
	node.predecessor = pred
}

func (node *xSkipListNode[W, O]) levels() []SkipListLevel[W, O] {
	if node == nil {
		return nil
	}
	return node.levelList
}

func (node *xSkipListNode[W, O]) Free() {
	node.predecessor = nil
	node.levelList = nil
}

func newXSkipListNode[W SkipListWeight, O HashObject](level uint32, weight W, obj O) SkipListNode[W, O] {
	e := &xSkipListNode[W, O]{
		element: atomic.Pointer[SkipListElement[W, O]]{},
		// Fill zero to all level span.
		// Initialization.
		levelList: make([]SkipListLevel[W, O], level),
	}
	e.setElement(&xSkipListElement[W, O]{
		weight: weight,
		object: obj,
	})
	for i := uint32(0); i < level; i++ {
		e.levelList[i] = newSkipListLevel[W, O](nil)
	}
	return e
}
