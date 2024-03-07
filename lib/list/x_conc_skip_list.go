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
func (skl *xConcurrentSkipList[W, O]) unlinkNode(
	predecessorNode, deletedNode *xConcurrentSkipListNode[W, O],
) bool {
	if predecessorNode != nil && deletedNode != nil {
		var nextNode, splicingNode *xConcurrentSkipListNode[W, O]
		for {
			nextNode = deletedNode.next.Load()
			// TODO How to identify the marker node instead of through the nil weight
			if nextNode != nil && nextNode.weight.Load() == nil { // Next node is a marker node
				splicingNode = nextNode.next.Load() // The marker node next node
				break
			} else if ptrCAS[xConcurrentSkipListNode[W, O]](deletedNode.next, nextNode, newMarkerNode(nextNode)) {
				// Add marker, waiting for helping deletion
				splicingNode = nextNode // Splicing the marker node
				break
			}
		}
		return ptrCAS[xConcurrentSkipListNode[W, O]](predecessorNode.next, deletedNode, splicingNode)
	}
	return false
}

// Adds an element if not present, or replaces an object if present.
func (skl *xConcurrentSkipList[W, O]) doPut(weight W, obj O, update ...bool) *O {
	var (
		pNode *xConcurrentSkipListNode[W, O] // predecessor node
		x     *xConcurrentSkipListNode[W, O]
	)

	for {
		// record the number of traversed levels
		levels := 0
		// start index
		sIdx := skl.head
		// traverse index
	verticalIndexes:
		for tIdx := sIdx.Load(); tIdx != nil; {
			// next right index (successor), at the same horizontal level
		horizonIndexes:
			for rIdx := tIdx.right.Load(); rIdx != nil; rIdx = tIdx.right.Load() {
				// Quickly access node info.
				if xNode := rIdx.node.Load(); xNode == nil {
					// (help deletion) Unlinks the deleted node in indexes.
					ptrCAS[xConcurrentSkipListIndex[W, O]](tIdx.right, rIdx, rIdx.right.Load())
					// If CAS is failed, it will be looped again, for the reason that traverse index isn't changed.
				} else {
					w := xNode.weight.Load()
					o := xNode.object.Load()
					if w == nil || o == nil {
						// It is a marker node. (help deletion) Unlinks the deleted node.
						ptrCAS[xConcurrentSkipListIndex[W, O]](tIdx.right, rIdx, rIdx.right.Load())
						// If CAS is failed, it will be looped again, for the reason that traverse index isn't changed.
					} else {
						if skl.cmp(weight, *w) > 0 {
							// Continue next right index
							tIdx = rIdx
						} else {
							// Descending to next down index
							break horizonIndexes
						}
					}
				}
			}

			// next down index (successor), at the vertical level
			if dIdx := tIdx.down.Load(); dIdx != nil {
				levels++
				// Continue next right index
				tIdx = dIdx
			} else {
				// Located a data node
				pNode = tIdx.node.Load()
				break verticalIndexes
			}
		}

		// If here occurs ABA/Concurrency CAS issue, restart iteration to find an insert position
		if pNode != nil {
			var (
				n *xConcurrentSkipListNode[W, O] // Temporarily store for splicing
			)
		findInsertPos:
			for {
				res := 0
				if n = pNode.next.Load(); n == nil {
					res = -1
				} else {
					nw, no := n.weight.Load(), n.object.Load()
					if nw == nil {
						// can't append; restart iteration
						break findInsertPos
					} else if no == nil {
						// (help deletion)
						skl.unlinkNode(pNode, n)
						res = 1
						continue
					}

					if res == 0 {
						res = skl.cmp(weight, *nw)
						if res > 0 {
							// Nodes has been spliced already
							pNode = n
						} else if res == 0 && ptrCAS[O](n.object, no, &obj) {
							// Updated old node.
							if len(update) <= 0 {
								update = []bool{false}
							}
							if update[0] {
								n.object.Store(&obj)
							}
							return no
						}
					}
				}

				newNode := newXConcurrentSkipListNode[W, O](&weight, &obj, nil)
				if res < 0 && ptrCAS[xConcurrentSkipListNode[W, O]](pNode.next, n, newNode) {
					// Nodes should have been spliced already
					x = newNode
					break findInsertPos
				}
			}

			if x != nil {
				//lr := cryptoRandInt32()
				//if lr&0x3 == 0 {
				//	// probability 1/4 enter into here
				//	hr := int64(cryptoRandInt32())
				//	rnd := hr<<32 | (int64(lr) & 0xffffffff)
				//	skips := levels
				//	var idx *xConcurrentSkipListIndex[W, O]
				//	for {
				//		idx = newXConcurrentSkipListIndex[W, O](x, nil, idx)
				//		if rnd >= 0 {
				//			break
				//		}
				//		if skips--; skips < 0 {
				//			break
				//		}
				//		rnd <<= 1
				//	}
				//
				//	if skl.addIndexes(skips, sIdx, idx) && skips < 0 && skl.head.Load() == sIdx.Load() {
				//		hx := newXConcurrentSkipListIndex[W, O](x, nil, idx)
				//		newPredecessor := newXConcurrentSkipListIndex[W, O](sIdx.Load().node.Load(), hx, sIdx.Load())
				//		ptrCAS[xConcurrentSkipListIndex[W, O]](skl.head, sIdx.Load(), newPredecessor)
				//	}
				//	if x.object.Load() == nil {
				//		skl.findPredecessor0(weight)
				//	}
				//}
				atomic.AddUint32(&skl.len, 1)
				return nil
			}
		}
	}
}

// Add indexes after the new node insertion.
// Descends iteratively to the highest level of insertion,
// then recursively, to chain index nodes to lower ones.
// Returns nil on (staleness) failure, disabling higher-level
// insertions. Recursion depths are exponentially less probable.
//func (skl *xConcurrentSkipList[W, O]) addIndexes(
//	skips int,
//	q *atomic.Pointer[xConcurrentSkipListIndex[W, O]],
//	x *xConcurrentSkipListIndex[W, O],
//) bool {
//	if x == nil {
//		return false
//	}
//	z := x.node
//	if z.Load() == nil {
//		return false
//	}
//	w := z.Load().weight
//	if w.Load() == nil {
//		return false
//	}
//	if q.Load() == nil {
//		return false
//	}
//
//	retrying := false
//	for {
//		rightIndex := q.Load().right.Load()
//		res := 0
//		if rightIndex != nil {
//			p := rightIndex.node.Load()
//			if p == nil || p.weight.Load() == nil || p.object.Load() == nil {
//				ptrCAS[xConcurrentSkipListIndex[W, O]](q.Load().right, rightIndex, rightIndex.right.Load())
//				res = 0
//			} else if res = skl.cmp(*w.Load(), *p.weight.Load()); res > 0 {
//				q.Store(rightIndex)
//			} else if res == 0 {
//				break // stale
//			}
//		} else {
//			res = -1
//		}
//
//		if res < 0 {
//			d := q.Load().down
//			if d.Load() != nil && skips > 0 {
//				skips--
//				q.Store(d.Load())
//			} else if d.Load() != nil && !retrying && !skl.addIndexes(0, d, x.down.Load()) {
//				break
//			} else {
//				x.right.Store(rightIndex)
//				if ptrCAS[xConcurrentSkipListIndex[W, O]](q.Load().right, rightIndex, x) {
//					return true
//				} else {
//					retrying = true
//				}
//			}
//		}
//	}
//	return false
//}

// Returns an index node with weight (key) strictly less than given weight.
// Also unlinks indexes to deleted nodes found along the way.
// Callers rely on this side effect of clearing indices to deleted nodes.
//func (skl *xConcurrentSkipList[W, O]) findPredecessor0(weigh W) *atomic.Pointer[xConcurrentSkipListNode[W, O]] {
//	// Start from top of head
//	predecessor := skl.head
//	if predecessor == nil {
//		return nil
//	}
//
//	for {
//		for rightIndex := predecessor.Load().right; rightIndex.Load() != nil; rightIndex = predecessor.Load().right {
//			node := rightIndex.Load().node
//			var w *W
//			if node.Load() == nil /* No more next node */ {
//				// Iterating next right index
//				ptrCAS[xConcurrentSkipListIndex[W, O]](predecessor.Load().right, rightIndex.Load(), rightIndex.Load().right.Load())
//			} else {
//				w = node.Load().weight.Load()
//				o := node.Load().object.Load()
//				// It is a marker node (data racing, modifying by other g)
//				if w == nil || o == nil {
//					// Unlink index to the deleted node.
//					// Reread the right index and restart the loop.
//					ptrCAS[xConcurrentSkipListIndex[W, O]](predecessor.Load().right, rightIndex.Load(), rightIndex.Load().right.Load())
//				}
//			}
//			res := skl.cmp(weigh, *w)
//			if res > 0 {
//				// Continue to iterate the same level right index
//				predecessor.Store(rightIndex.Load())
//			} else {
//				// Down to the next level
//				break
//			}
//		}
//
//		if downIndex := predecessor.Load().down.Load(); downIndex != nil {
//			// Iterating this level's indexes
//			predecessor.Store(downIndex)
//		} else {
//			// Only one node left
//			return predecessor.Load().node
//		}
//	}
//}

// Returns the node holding key or nil if no such, clearing out any deleted nodes seen
// along the way.
// Repeatedly traverses at base-level looking for key starting at predecessor returned
// from findPredecessor0, processing base-level deletions as encountered.
// Restarts occur, at a traversal step encountering next node, if next node's weight (key)
// field is nil, indicating it is a marker node, so its predecessor is deleted before continuing,
// which we help do by re-finding a valid predecessor.
//func (skl *xConcurrentSkipList[W, O]) findNode(weight W) *atomic.Pointer[xConcurrentSkipListNode[W, O]] {
//	var predecessorNode *atomic.Pointer[xConcurrentSkipListNode[W, O]]
//outer:
//	for predecessorNode = skl.findPredecessor0(weight); predecessorNode != nil; predecessorNode = skl.findPredecessor0(weight) {
//		for {
//			nextNode := predecessorNode.Load().next
//			n := nextNode.Load()
//			var (
//				w *W
//				o *O
//			)
//			if n == nil {
//				break outer
//			} else {
//				if w = n.weight.Load(); w == nil {
//					break // predecessorNode is deleted
//				}
//				if o = n.object.Load(); o == nil {
//					// The next node is deleted
//					skl.unlinkNode(predecessorNode, nextNode)
//				} else {
//					res := skl.cmp(weight, *nextNode.Load().weight.Load())
//					if res > 0 {
//						predecessorNode.Store(n)
//					} else if res == 0 {
//						return nextNode
//					} else {
//						break outer
//					}
//				}
//			}
//		}
//	}
//	return nil
//}
//
//func (skl *xConcurrentSkipList[W, O]) doGet(weight W) SkipListElement[W, O] {
//	var ele SkipListElement[W, O]
//	predecessor := skl.head
//	if predecessor.Load() == nil {
//		return ele
//	}
//outer:
//	for {
//		for rightIndex := predecessor.Load().right.Load(); rightIndex != nil; rightIndex = predecessor.Load().right.Load() {
//			p := rightIndex.node
//			var (
//				w   *atomic.Pointer[W]
//				o   *atomic.Pointer[O]
//				res = 0
//			)
//			if p.Load() == nil {
//				ptrCAS[xConcurrentSkipListIndex[W, O]](predecessor.Load().right, rightIndex, rightIndex.right.Load())
//			} else {
//				w = p.Load().weight
//				o = p.Load().object
//				if w.Load() == nil || o.Load() == nil {
//					ptrCAS[xConcurrentSkipListIndex[W, O]](predecessor.Load().right, rightIndex, rightIndex.right.Load())
//				} else if res = skl.cmp(weight, *w.Load()); res > 0 {
//					predecessor.Store(rightIndex)
//				} else if res == 0 {
//					ele = &xSkipListElement[W, O]{
//						weight: *w.Load(),
//						object: *o.Load(),
//					}
//					break outer
//				} else {
//					break
//				}
//			}
//		}
//
//		if downIndex := predecessor.Load().down.Load(); downIndex != nil {
//			// Iterating this level's indexes
//			predecessor.Store(downIndex)
//		} else {
//			if baseNode := predecessor.Load().node.Load(); baseNode != nil {
//				for nextNode := baseNode.next.Load(); nextNode != nil; nextNode = baseNode.next.Load() {
//					w := nextNode.weight.Load()
//					o := nextNode.object.Load()
//					res := 0
//					if w == nil || o == nil {
//						baseNode = nextNode
//					} else if w != nil {
//						res = skl.cmp(weight, *w)
//						if res > 0 {
//							baseNode = nextNode
//						} else if res == 0 {
//							ele = &xSkipListElement[W, O]{
//								weight: *w,
//								object: *o,
//							}
//							break
//						}
//					}
//				}
//			}
//			break
//		}
//	}
//	return ele
//}
//
//func (skl *xConcurrentSkipList[W, O]) tryReduceLevel() {
//	h := skl.head
//	if h.Load() == nil {
//		return
//	}
//	if h.Load().right.Load() != nil {
//		return
//	}
//	d := h.Load().down
//	if d.Load() == nil || d.Load().right.Load() != nil {
//		return
//	}
//	e := d.Load().down
//	if e.Load() == nil || e.Load().right.Load() != nil {
//		return
//	}
//	// double check
//	if ptrCAS[xConcurrentSkipListIndex[W, O]](skl.head, h.Load(), d.Load()) && h.Load().right.Load() != nil {
//		// try to backout
//		ptrCAS[xConcurrentSkipListIndex[W, O]](skl.head, d.Load(), h.Load())
//	}
//}
//
//func (skl *xConcurrentSkipList[W, O]) doRemove(weight W, object O) SkipListElement[W, O] {
//	var ele SkipListElement[W, O]
//outer:
//	for predecessorNode := skl.findPredecessor0(weight); predecessorNode.Load() != nil && ele == nil; predecessorNode = skl.findPredecessor0(weight) {
//		for {
//			res := 0
//			nextNode := predecessorNode.Load().next
//			if nextNode.Load() == nil {
//				break outer
//			} else {
//				w := nextNode.Load().weight.Load()
//				if w == nil {
//					break
//				}
//				o := nextNode.Load().object.Load()
//				if o == nil {
//					skl.unlinkNode(predecessorNode, nextNode)
//				} else {
//					if res = skl.cmp(weight, *w); res > 0 {
//						// Iterating next node.
//						predecessorNode.Store(nextNode.Load())
//					} else if res < 0 {
//						break outer
//					} else {
//						if (*o).Hash() != object.Hash() {
//							break outer
//						} else if ptrCAS[O](nextNode.Load().object, o, nil) {
//							ele = &xSkipListElement[W, O]{
//								weight: *w,
//								object: *o,
//							}
//							skl.unlinkNode(predecessorNode, nextNode)
//							break
//						}
//					}
//				}
//			}
//		}
//	}
//	if ele != nil {
//		skl.tryReduceLevel()
//		atomic.AddUint32(&skl.len, ^uint32(0)) // -1
//	}
//	return ele
//}

func (skl *xConcurrentSkipList[W, O]) Level() uint32 {
	return atomic.LoadUint32(&skl.level)
}

func (skl *xConcurrentSkipList[W, O]) Len() uint32 {
	return atomic.LoadUint32(&skl.len)
}

func (skl *xConcurrentSkipList[W, O]) Free() {
	//TODO implement me
	panic("implement me")
}

func (skl *xConcurrentSkipList[W, O]) ForEach(fn func(idx int64, weight W, object O)) {
	var pIdx *xConcurrentSkipListIndex[W, O]
	for pIdx = skl.head.Load(); pIdx != nil; {
		if dIdx := pIdx.down.Load(); dIdx != nil {
			pIdx = dIdx
		} else {
			break
		}
	}
	idx := int64(0)
	for node := pIdx.node.Load(); node != nil; {
		nextNode := node.next.Load()
		if nextNode == nil {
			w, o := node.weight.Load(), node.object.Load()
			if w != nil && o != nil {
				fn(idx, *w, *o)
			}
			break
		} else {
			w, o := node.weight.Load(), node.object.Load()
			if nextNode.weight.Load() != nil && nextNode.object.Load() != nil &&
				w != nil && o != nil {
				fn(idx, *w, *o)
				idx++
			}
			node = nextNode
		}
	}
}

func (skl *xConcurrentSkipList[W, O]) Insert(weight W, obj O) (SkipListNode[W, O], bool) {
	//TODO implement me
	panic("implement me")
}

func (skl *xConcurrentSkipList[W, O]) RemoveFirst(weight W) SkipListElement[W, O] {
	//TODO implement me
	panic("implement me")
}

func (skl *xConcurrentSkipList[W, O]) RemoveAll(weight W) []SkipListElement[W, O] {
	//TODO implement me
	panic("implement me")
}

func (skl *xConcurrentSkipList[W, O]) RemoveIfMatch(weight W, cmp SkipListObjectMatcher[O]) []SkipListElement[W, O] {
	//TODO implement me
	panic("implement me")
}

func (skl *xConcurrentSkipList[W, O]) FindFirst(weight W) SkipListElement[W, O] {
	//TODO implement me
	panic("implement me")
}

func (skl *xConcurrentSkipList[W, O]) FindAll(weight W) []SkipListElement[W, O] {
	//TODO implement me
	panic("implement me")
}

func (skl *xConcurrentSkipList[W, O]) FindIfMatch(weight W, cmp SkipListObjectMatcher[O]) []SkipListElement[W, O] {
	//TODO implement me
	panic("implement me")
}

func (skl *xConcurrentSkipList[W, O]) PopHead() SkipListElement[W, O] {
	//TODO implement me
	panic("implement me")
}

func NewXConcurrentSkipList[W SkipListWeight, O HashObject]() SkipList[W, O] {
	skl := &xConcurrentSkipList[W, O]{}
	// Empty skip-list (empty indexes). Initialize first base index.
	base := newBaseNode[W, O]()
	idx := newXConcurrentSkipListIndex[W, O](base, nil, nil)
	ptrCAS[xConcurrentSkipListIndex[W, O]](skl.head, nil, idx)
	return skl
}
