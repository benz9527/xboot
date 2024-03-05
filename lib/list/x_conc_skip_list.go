package list

import (
	"sync/atomic"
)

var (
	_ SkipList[uint8, *emptyHashObject] = (*xConcurrentSkipList[uint8, *emptyHashObject])(nil)
)

// Ignore to support the backward (tail) ref.
type xConcurrentSkipList[W SkipListWeight, O HashObject] struct {
	cmp   SkipListWeightComparator[W]
	head  *atomic.Pointer[xConcurrentSkipListIndex[W, O]]
	level uint32
	len   uint32
}

// Tries to unlink marked as deleted node from predecessor node (if both exist),
// by first splicing in a marker if not already present.
// Upon return, deleted node is sure to be unlinked from the predecessor node,
// possibly via the actions of some other goroutines.
func (xcsl *xConcurrentSkipList[W, O]) unlinkNode(
	predecessorNode, deletedNode *atomic.Pointer[xConcurrentSkipListNode[W, O]],
) bool {
	if predecessorNode.Load() != nil && deletedNode.Load() != nil {
		var nextNode, splicingNode *xConcurrentSkipListNode[W, O]
		for {
			nextNode = deletedNode.Load().next.Load()
			// TODO How to identify the marker node instead of through the nil weight
			if nextNode.weight.Load() == nil { // Next node is a marker node
				splicingNode = nextNode.next.Load() // The marker node next node
				break
			} else if nextCompareAndSet(deletedNode, nextNode, newMarkerNode(nextNode)) {
				// Add marker, waiting for helping deletion
				splicingNode = nextNode // Splicing the marker node
				break
			}
		}
		return nextCompareAndSet(predecessorNode, deletedNode.Load(), splicingNode)
	}
	return false
}

// Returns an index node with weight (key) strictly less than given weight.
// Also unlinks indexes to deleted nodes found along the way.
// Callers rely on this side-effect of clearing indices to deleted nodes.
func (xcsl *xConcurrentSkipList[W, O]) findPredecessor0(weigh W) *atomic.Pointer[xConcurrentSkipListNode[W, O]] {
	// Start from top of head
	predecessor := xcsl.head
	if predecessor == nil {
		return nil
	}

	for {
		for rightIndex := predecessor.Load().right.Load(); rightIndex != nil; rightIndex = predecessor.Load().right.Load() {
			node := rightIndex.node.Load()
			var w *W
			if node == nil /* No more next node */ {
				// Iterating next right index
				rightCompareAndSwap(predecessor, rightIndex, rightIndex.right.Load())
			} else {
				w = node.weight.Load()
				o := node.object.Load()
				// It is a marker node (data racing, modifying by other g)
				if w == nil || o == nil {
					// Unlink index to the deleted node
					// Reread the right index and restart the loop.
					rightCompareAndSwap(predecessor, rightIndex, rightIndex.right.Load())
				}
			}
			res := xcsl.cmp(weigh, *w)
			if res > 0 {
				// Continue to iterate the same level right index
				predecessor.Store(rightIndex)
			} else {
				// Down to the next level
				break
			}
		}

		if downIndex := predecessor.Load().down.Load(); downIndex != nil {
			// Iterating this level's indexes
			predecessor.Store(downIndex)
		} else {
			// Only one node left
			return predecessor.Load().node
		}
	}
}

// Returns the node holding key or nil if no such, clearing out any deleted nodes seen
// along the way.
// Repeatedly traverses at base-level looking for key starting at predecessor returned
// from findPredecessor0, processing base-level deletions as encountered.
// Restarts occur, at a traversal step encountering next node, if next node's weight (key)
// field is nil, indicating it is a marker node, so its predecessor is deleted before continuing,
// which we help do by re-finding a valid predecessor.
func (xcsl *xConcurrentSkipList[W, O]) findNode(weight W) *atomic.Pointer[xConcurrentSkipListNode[W, O]] {
	var predecessorNode *atomic.Pointer[xConcurrentSkipListNode[W, O]]
outer:
	for predecessorNode = xcsl.findPredecessor0(weight); predecessorNode != nil; predecessorNode = xcsl.findPredecessor0(weight) {
		for {
			nextNode := predecessorNode.Load().next
			n := nextNode.Load()
			var (
				w *W
				o *O
			)
			if n == nil {
				break outer
			} else {
				if w = n.weight.Load(); w == nil {
					break // predecessorNode is deleted
				}
				if o = n.object.Load(); o == nil {
					// The next node is deleted
					xcsl.unlinkNode(predecessorNode, nextNode)
				} else {
					res := xcsl.cmp(weight, nextNode.Load().Weight())
					if res > 0 {
						predecessorNode.Store(n)
					} else if res == 0 {
						return nextNode
					} else {
						break outer
					}
				}
			}
		}
	}
	return nil
}

func (xcsl *xConcurrentSkipList[W, O]) doPut(weight W, obj O) *atomic.Pointer[O] {
	for {
		var b = &atomic.Pointer[xConcurrentSkipListNode[W, O]]{}
		levels := 0
		h := xcsl.head
		if h == nil || h.Load() == nil {
			// Initialize
			base := newXConcurrentSkipListNode[W, O](*new(W), *new(O), nil)
			_h := newXConcurrentSkipListIndex[W, O](base, nil, nil)
			if headCompareAndSwap(xcsl.head, nil, _h) {
				b.Store(base)
			} else {
				b.Store(nil)
			}
		} else {
			for q := h; q != nil && q.Load() != nil; {
				for r := q.Load().right; r != nil; {
					p := r.Load().node.Load()
					if p == nil || p.weight.Load() == nil || p.object.Load() == nil {
						rightCompareAndSwap(q, r.Load(), r.Load().right.Load())
					} else if xcsl.cmp(weight, p.Weight()) > 0 {
						q = r
					} else {
						break
					}
				}
				if d := q.Load().down; d.Load() != nil {
					levels++
					q = d
				} else {
					b = q.Load().node
					break
				}
			}
		}
		if b != nil && b.Load() != nil {
			z := &atomic.Pointer[xConcurrentSkipListNode[W, O]]{}
			c := 0
			for {
				if n := b.Load().next; n.Load() == nil {
					w := b.Load().weight
					if w == nil || w.Load() == nil {
						xcsl.cmp(weight, weight)
					}
					c = -1
				} else {
					w := n.Load().weight
					o := n.Load().object
					if w.Load() == nil {
						break
					} else if o == nil {
						xcsl.unlinkNode(b, n)
						c = 1
					} else if c = xcsl.cmp(weight, *w.Load()); c > 0 {
						b = n
					} else if c == 0 || objectCompareAndSet[W, O](n, *o.Load(), obj) {
						return o
					}
					p := newXConcurrentSkipListNode[W, O](weight, obj, nil)
					if c < 0 && nextCompareAndSet[W, O](b, n.Load(), p) {
						z.Store(p)
						break
					}
				}

				if z.Load() != nil {

				}
			}
		}
	}
}

func (xcsl *xConcurrentSkipList[W, O]) addIndices(skips int, q, x *atomic.Pointer[xConcurrentSkipListIndex[W, O]]) bool {
	return false
}

func (xcsl *xConcurrentSkipList[W, O]) Level() uint32 {
	//TODO implement me
	panic("implement me")
}

func (xcsl *xConcurrentSkipList[W, O]) Len() uint32 {
	//TODO implement me
	panic("implement me")
}

func (xcsl *xConcurrentSkipList[W, O]) Free() {
	//TODO implement me
	panic("implement me")
}

func (xcsl *xConcurrentSkipList[W, O]) ForEach(fn func(idx int64, weight W, object O)) {
	//TODO implement me
	panic("implement me")
}

func (xcsl *xConcurrentSkipList[W, O]) Insert(weight W, obj O) (SkipListNode[W, O], bool) {
	//TODO implement me
	panic("implement me")
}

func (xcsl *xConcurrentSkipList[W, O]) RemoveFirst(weight W) SkipListElement[W, O] {
	//TODO implement me
	panic("implement me")
}

func (xcsl *xConcurrentSkipList[W, O]) RemoveAll(weight W) []SkipListElement[W, O] {
	//TODO implement me
	panic("implement me")
}

func (xcsl *xConcurrentSkipList[W, O]) RemoveIfMatch(weight W, cmp SkipListObjectMatcher[O]) []SkipListElement[W, O] {
	//TODO implement me
	panic("implement me")
}

func (xcsl *xConcurrentSkipList[W, O]) FindFirst(weight W) SkipListElement[W, O] {
	//TODO implement me
	panic("implement me")
}

func (xcsl *xConcurrentSkipList[W, O]) FindAll(weight W) []SkipListElement[W, O] {
	//TODO implement me
	panic("implement me")
}

func (xcsl *xConcurrentSkipList[W, O]) FindIfMatch(weight W, cmp SkipListObjectMatcher[O]) []SkipListElement[W, O] {
	//TODO implement me
	panic("implement me")
}

func (xcsl *xConcurrentSkipList[W, O]) PopHead() SkipListElement[W, O] {
	//TODO implement me
	panic("implement me")
}

func NewXConcurrentSkipList[W SkipListWeight, O HashObject]() SkipList[W, O] {
	xcsl := &xConcurrentSkipList[W, O]{}
	return xcsl
}
