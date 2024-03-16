package list

// References:
// https://people.csail.mit.edu/shanir/publications/LazySkipList.pdf
// github:
// https://github.com/zhangyunhao116/skipmap

import (
	"errors"
	"sync/atomic"
	"unsafe"

	"github.com/benz9527/xboot/lib/id"
	"github.com/benz9527/xboot/lib/infra"
)

const (
	// Indicating that the skip-list exclusive lock implementation type.
	sklMutexImplBits = 0x00FF
	// Indicating that the skip-list data node type, including unique, linkedList and rbtree.
	sklXNodeModeBits = 0x0300
	// Indicating that the skip-list data node in rbtree mode and 0 is rm by pred and 1 is rm by succ
	sklRbtreeRmReplaceFnFlagBit = 0x0400
)

type xConcSkl[K infra.OrderedKey, V comparable] struct {
	head    *xConcSklNode[K, V]
	pool    *xConcSklPool[K, V]
	kcmp    infra.OrderedKeyComparator[K]
	vcmp    SklValComparator[V]
	rand    SklRand
	idGen   id.Generator
	flags   flagBits
	len     int64  // skip-list's node size
	idxSize uint64 // skip-list's index count
	idxHi   int32  // skip-list's indexes' height
}

func (skl *xConcSkl[K, V]) loadMutexImpl() mutexImpl {
	return mutexImpl(skl.flags.atomicLoadBits(sklMutexImplBits))
}

func (skl *xConcSkl[K, V]) loadXNodeMode() xNodeMode {
	return xNodeMode(skl.flags.atomicLoadBits(sklXNodeModeBits))
}

func (skl *xConcSkl[K, V]) atomicLoadHead() *xConcSklNode[K, V] {
	return (*xConcSklNode[K, V])(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&skl.head))))
}

func (skl *xConcSkl[K, V]) Len() int64 {
	return atomic.LoadInt64(&skl.len)
}

func (skl *xConcSkl[K, V]) Indices() uint64 {
	return atomic.LoadUint64(&skl.idxSize)
}

func (skl *xConcSkl[K, V]) Level() int32 {
	return atomic.LoadInt32(&skl.idxHi)
}

// traverse locates the target key and store the nodes encountered during the indices traversal.
func (skl *xConcSkl[K, V]) traverse(
	lvl int32,
	key K,
	aux xConcSklAux[K, V],
) *xConcSklNode[K, V] {
	forward := skl.atomicLoadHead()
	for /* vertical */ l := lvl - 1; l >= 0; l-- {
		nIdx := forward.atomicLoadNext(l)
		for /* horizontal */ nIdx != nil && skl.kcmp(key, nIdx.key) > 0 {
			forward = nIdx
			nIdx = forward.atomicLoadNext(l)
		}

		aux.storePred(l, forward)
		aux.storeSucc(l, nIdx)

		if /* found */ nIdx != nil && skl.kcmp(key, nIdx.key) == 0 {
			return nIdx
		}
		// Not found at current level, downward to next level's indices.
	}
	return nil
}

// Insert add the val by a key into skip-list.
// Only works for unique element skip-list.
func (skl *xConcSkl[K, V]) Insert(key K, val V) {
	var (
		aux      = skl.pool.loadAux()
		oldIdxHi = skl.Level()
		newIdxHi = /* avoid loop call */ skl.rand(
			int(oldIdxHi),
			int32(atomic.LoadInt64(&skl.len)),
		)
		ver = skl.idGen.NumberUUID()
	)
	defer func() {
		skl.pool.releaseAux(aux)
	}()
	for {
		if node := skl.traverse(max(oldIdxHi, newIdxHi), key, aux); node != nil {
			if /* conc rm */ node.flags.atomicIsSet(nodeRemovingFlagBit) {
				continue
			}
			if isAppend, err := node.storeVal(ver, val); err != nil {
				panic(err)
			} else if isAppend {
				atomic.AddInt64(&skl.len, 1)
			}
			return
		}
		// Node not present. Add this node into skip list.
		var (
			pred, succ, prev *xConcSklNode[K, V] = nil, nil, nil
			isValid                              = true
			lockedLevels                         = int32(-1)
		)
		for l := int32(0); isValid && l < newIdxHi; l++ {
			pred, succ = aux.loadPred(l), aux.loadSucc(l)
			if /* lock */ pred != prev {
				pred.mu.lock(ver)
				lockedLevels = l
				prev = pred
			}
			// Check indices and data node:
			//      +------+       +------+      +------+
			// ...  | pred |------>|  new |----->| succ | ...
			//      +------+       +------+      +------+
			// 1. Both the pred and succ isn't removing.
			// 2. The pred's next node is the succ in this level.
			isValid = !pred.flags.atomicIsSet(nodeRemovingFlagBit) &&
				(succ == nil || !succ.flags.atomicIsSet(nodeRemovingFlagBit)) &&
				pred.loadNext(l) == succ
		}
		if /* conc insert */ !isValid {
			aux.foreachPred( /* unlock */ func(list ...*xConcSklNode[K, V]) {
				unlockNodes(ver, lockedLevels, list...)
			})
			continue
		}

		n := newXConcSkipListNode(key, val, newIdxHi, skl.loadMutexImpl(), skl.loadXNodeMode(), skl.vcmp)
		for l := int32(0); l < newIdxHi; l++ {
			// Linking
			//      +------+       +------+      +------+
			// ...  | pred |------>|  new |----->| succ | ...
			//      +------+       +------+      +------+
			n.storeNext(l, aux.loadSucc(l))       // Useless to use atomic here.
			aux.loadPred(l).atomicStoreNext(l, n) // Memory barrier, concurrency safety.
		}
		n.flags.atomicSet(nodeInsertedFlagBit)
		if oldIdxHi = skl.Level(); oldIdxHi < newIdxHi {
			atomic.StoreInt32(&skl.idxHi, newIdxHi)
		}

		aux.foreachPred( /* unlock */ func(list ...*xConcSklNode[K, V]) {
			unlockNodes(ver, lockedLevels, list...)
		})
		atomic.AddInt64(&skl.len, 1)
		atomic.AddUint64(&skl.idxSize, uint64(newIdxHi))
		return
	}
}

// Range iterates each node (xnode within the node) by pass in function.
// Once the function return false, the iteration should be stopped.
// This function doesn't guarantee correctness in the case of concurrent
// reads and writes.
func (skl *xConcSkl[K, V]) Range(fn func(idx int64, metadata SkipListIterationItem[K, V]) bool) {
	forward := skl.atomicLoadHead().atomicLoadNext(0)
	index := int64(0)
	typ := skl.loadXNodeMode()
	item := &xSklIter[K, V]{}
	switch typ {
	case unique:
		for forward != nil {
			if !forward.flags.atomicAreEqual(nodeInsertedFlagBit|nodeRemovingFlagBit, insertFullyLinked) {
				forward = forward.atomicLoadNext(0)
				continue
			}
			item.nodeLevelFn = func() uint32 {
				return atomic.LoadUint32(&forward.level)
			}
			item.nodeItemCountFn = func() int64 {
				return atomic.LoadInt64(&forward.count)
			}
			item.keyFn = func() K {
				return forward.key
			}
			item.valFn = func() V {
				vn := forward.loadXNode()
				if vn == nil {
					return *new(V)
				}
				return *vn.vptr
			}
			if res := fn(index, item); !res {
				break
			}
			forward = forward.atomicLoadNext(0)
			index++
		}
	case linkedList:
		for forward != nil {
			if !forward.flags.atomicAreEqual(nodeInsertedFlagBit|nodeRemovingFlagBit, insertFullyLinked) {
				forward = forward.atomicLoadNext(0)
				continue
			}
			item.nodeLevelFn = func() uint32 {
				return atomic.LoadUint32(&forward.level)
			}
			item.nodeItemCountFn = func() int64 {
				return atomic.LoadInt64(&forward.count)
			}
			item.keyFn = func() K {
				return forward.key
			}
			for x := forward.loadXNode().parent; x != nil; x = x.parent {
				item.valFn = func() V {
					return *x.vptr
				}
				var res bool
				if res, index = fn(index, item), index+1; !res {
					break
				}
			}
			forward = forward.atomicLoadNext(0)
		}
	case rbtree:
		for forward != nil {
			if !forward.flags.atomicAreEqual(nodeInsertedFlagBit|nodeRemovingFlagBit, insertFullyLinked) {
				forward = forward.atomicLoadNext(0)
				continue
			}
			item.nodeLevelFn = func() uint32 {
				return atomic.LoadUint32(&forward.level)
			}
			item.nodeItemCountFn = func() int64 {
				return atomic.LoadInt64(&forward.count)
			}
			item.keyFn = func() K {
				return forward.key
			}
			forward.rbPreorderTraversal(func(idx int64, color color, val V) bool {
				item.valFn = func() V {
					return val
				}
				var res bool
				if res, index = fn(index, item), index+1; !res {
					return false
				}
				return true
			})
			forward = forward.atomicLoadNext(0)
		}
	default:
		panic("[x-conc-skl] unknown node type")
	}
}

// Get returns the first value stored in the skip-list for a key,
// or nil if no val is present.
// The ok result indicates whether the value was found in the skip-list.
func (skl *xConcSkl[K, V]) Get(key K) (val V, ok bool) {
	forward := skl.atomicLoadHead()
	typ := skl.loadXNodeMode()
	for /* vertical */ l := skl.Level() - 1; l >= 0; l-- {
		nIdx := forward.atomicLoadNext(l)
		for /* horizontal */ nIdx != nil && skl.kcmp(key, nIdx.key) > 0 {
			forward = nIdx
			nIdx = forward.atomicLoadNext(l)
		}

		if /* found */ nIdx != nil && skl.kcmp(key, nIdx.key) == 0 {
			if nIdx.flags.atomicAreEqual(nodeInsertedFlagBit|nodeRemovingFlagBit, insertFullyLinked) {
				if /* conc rw */ atomic.LoadInt64(&nIdx.count) <= 0 {
					return *new(V), false
				}
				switch typ {
				case unique:
					x := nIdx.loadXNode()
					return *x.vptr, true
				case linkedList:
					x := nIdx.loadXNode()
					return *x.parent.vptr, true
				case rbtree:
					x := nIdx.root.minimum()
					return *x.vptr, true
				default:
					panic("[x-conc-skl] unknown x-node type")
				}
			}
			return
		}
	}
	return
}

// rmTraverse locates the target key and stores the nodes encountered
// during the indices traversal.
// Returns with the target key found level index.
func (skl *xConcSkl[K, V]) rmTraverse(
	weight K,
	aux xConcSklAux[K, V],
) (foundAt int32) {
	// foundAt represents the index of the first layer at which it found a node.
	foundAt = -1
	forward := skl.atomicLoadHead()
	for /* vertical */ l := skl.Level() - 1; l >= 0; l-- {
		nIdx := forward.atomicLoadNext(l)
		for /* horizontal */ nIdx != nil && skl.kcmp(weight, nIdx.key) > 0 {
			forward = nIdx
			nIdx = forward.atomicLoadNext(l)
		}

		aux.storePred(l, forward)
		aux.storeSucc(l, nIdx)

		if foundAt == -1 && nIdx != nil && skl.kcmp(weight, nIdx.key) == 0 {
			foundAt = l
		}
		// Downward to next level.
	}
	return
}

// RemoveFirst deletes the val for a key, only the first value.
func (skl *xConcSkl[K, V]) RemoveFirst(key K) (ele SkipListElement[K, V], err error) {
	var (
		aux      = skl.pool.loadAux()
		rmTarget *xConcSklNode[K, V]
		isMarked bool // represents if this operation mark the node
		topLevel = int32(-1)
		ver      = skl.idGen.NumberUUID()
		typ      = skl.loadXNodeMode()
		foundAt  = int32(-1)
	)
	defer func() {
		skl.pool.releaseAux(aux)
	}()

	switch typ {
	// FIXME: Merge these 2 deletion loops logic
	case unique:
		for {
			foundAt = skl.rmTraverse(key, aux)
			if isMarked || foundAt != -1 &&
				aux.loadSucc(foundAt).flags.atomicAreEqual(nodeInsertedFlagBit|nodeRemovingFlagBit, insertFullyLinked) &&
				(int32(aux.loadSucc(foundAt).level)-1) == foundAt {
				if !isMarked {
					rmTarget = aux.loadSucc(foundAt)
					topLevel = foundAt
					if !rmTarget.mu.tryLock(ver) {
						if /* d-check */ rmTarget.flags.atomicIsSet(nodeRemovingFlagBit) {
							return nil, errors.New("remove lock acquire failed and node (x-node) has been marked as removing")
						}
						isMarked = false
						continue
					}

					if /* node locked, d-check */ rmTarget.flags.atomicIsSet(nodeRemovingFlagBit) {
						rmTarget.mu.unlock(ver)
						return nil, errors.New("node (x-node) has been marked as removing")
					}

					rmTarget.flags.atomicSet(nodeRemovingFlagBit)
					isMarked = true
				}

				var (
					lockedLayers         = int32(-1)
					isValid              = true
					pred, succ, prevPred *xConcSklNode[K, V]
				)
				for /* node locked */ l := int32(0); isValid && (l <= topLevel); l++ {
					pred, succ = aux.loadPred(l), aux.loadSucc(l)
					if /* lock indices */ pred != prevPred {
						pred.mu.lock(ver)
						lockedLayers = l
						prevPred = pred
					}
					// Check:
					// 1. the previous node exists.
					// 2. no other nodes are inserted into the skip list in this layer.
					isValid = !pred.flags.atomicIsSet(nodeRemovingFlagBit) && pred.atomicLoadNext(l) == succ
				}
				if /* conc rm */ !isValid {
					aux.foreachPred( /* unlock */ func(list ...*xConcSklNode[K, V]) {
						unlockNodes(ver, lockedLayers, list...)
					})
					continue
				}

				ele = &xSklElement[K, V]{
					key: key,
					val: *rmTarget.loadXNode().vptr,
				}
				atomic.AddInt64(&rmTarget.count, -1)
				atomic.AddInt64(&skl.len, -1)

				if atomic.LoadInt64(&rmTarget.count) <= 0 {
					for /* re-linking, reduce levels */ l := topLevel; l >= 0; l-- {
						aux.loadPred(l).atomicStoreNext(l, rmTarget.loadNext(l))
					}
					atomic.AddUint64(&skl.idxSize, ^uint64(rmTarget.level-1))
				}

				rmTarget.mu.unlock(ver)
				aux.foreachPred( /* unlock */ func(list ...*xConcSklNode[K, V]) {
					unlockNodes(ver, lockedLayers, list...)
				})
				return ele, nil
			}
			break
		}
	case linkedList, rbtree:
		for {
			foundAt = skl.rmTraverse(key, aux)
			if isMarked || foundAt != -1 {
				fullyLinkedButNotRemove := aux.loadSucc(foundAt).flags.atomicAreEqual(nodeInsertedFlagBit|nodeRemovingFlagBit, insertFullyLinked)
				succMatch := (int32(aux.loadSucc(foundAt).level) - 1) == foundAt
				if !succMatch {
					break
				} else if !fullyLinkedButNotRemove {
					continue
				}

				if fullyLinkedButNotRemove && !isMarked {
					rmTarget = aux.loadSucc(foundAt)
					topLevel = foundAt
					if !rmTarget.mu.tryLock(ver) {
						continue
					}

					if /* node locked */ !rmTarget.flags.atomicIsSet(nodeRemovingFlagBit) {
						rmTarget.flags.atomicSet(nodeRemovingFlagBit)
					}
					isMarked = true
				}

				var (
					lockedLayers     = int32(-1)
					isValid          = true
					pred, succ, prev *xConcSklNode[K, V]
				)
				for /* node locked */ l := int32(0); isValid && (l <= topLevel); l++ {
					pred, succ = aux.loadPred(l), aux.loadSucc(l)
					if /* lock indices */ pred != prev {
						pred.mu.lock(ver)
						lockedLayers = l
						prev = pred
					}
					// Check:
					// 1. the previous node exists.
					// 2. no other nodes are inserted into the skip list in this layer.
					isValid = !pred.flags.atomicIsSet(nodeRemovingFlagBit) && pred.atomicLoadNext(l) == succ
				}
				if /* conc rm */ !isValid {
					aux.foreachPred( /* unlock */ func(list ...*xConcSklNode[K, V]) {
						unlockNodes(ver, lockedLayers, list...)
					})
					continue
				}

				switch typ {
				case linkedList:
					if /* locked */ x := rmTarget.root.linkedListNext(); x != nil {
						ele = &xSklElement[K, V]{
							key: key,
							val: *x.vptr,
						}
						atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&rmTarget.root.parent)), unsafe.Pointer(x.parent))
						atomic.AddInt64(&rmTarget.count, -1)
						atomic.AddInt64(&skl.len, -1)
						rmTarget.flags.atomicUnset(nodeRemovingFlagBit)
					} else {
						atomic.StoreInt64(&rmTarget.count, 0)
					}
				case rbtree:
					if /* locked */ x, _err := rmTarget.rbRemoveMin(); _err == nil && x != nil {
						ele = &xSklElement[K, V]{
							key: key,
							val: *x.vptr,
						}
						atomic.AddInt64(&skl.len, -1)
					}
					rmTarget.flags.atomicUnset(nodeRemovingFlagBit)
				}

				if atomic.LoadInt64(&rmTarget.count) <= 0 {
					for /* re-linking, reduce levels */ l := topLevel; l >= 0; l-- {
						aux.loadPred(l).atomicStoreNext(l, rmTarget.loadNext(l))
					}
					atomic.AddUint64(&skl.idxSize, ^uint64(rmTarget.level-1))
				}

				rmTarget.mu.unlock(ver)
				aux.foreachPred( /* unlock */ func(list ...*xConcSklNode[K, V]) {
					unlockNodes(ver, lockedLayers, list...)
				})
				return ele, nil
			}
			break
		}
	default:
		panic("[x-conc-skl] unknown x-node type")
	}

	if foundAt == -1 {
		return nil, errors.New("not found remove target")
	}
	return nil, errors.New("others unknown reasons")
}

func NewXConcSkipList[K infra.OrderedKey, V comparable](cmp SklWeightComparator[K], rand SklRand) *xConcSkl[K, V] {
	//h := newXConcSklHead[K, V]()
	//h.flags.atomicSet(nodeInsertedFlagBit)
	//return &xConcSkl[K, V]{
	//	head:  h,
	//	idxHi: 0,
	//	len:   0,
	//	kcmp:   kcmp,
	//	rand:  rand,
	//}
	return nil
}
