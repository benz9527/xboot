package list

// References:
// https://people.csail.mit.edu/shanir/publications/LazySkipList.pdf
// github:
// https://github.com/zhangyunhao116/skipmap

import (
	"errors"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/benz9527/xboot/lib/id"
	"github.com/benz9527/xboot/lib/infra"
)

const (
	// Indicating that the skip-list exclusive lock implementation type.
	sklMutexImplBits = 0x00FF
	// Indicating that the skip-list data node type, including unique, linkedList and rbtree.
	sklVNodeTypeBits = 0x0300
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

func (skl *xConcSkl[K, V]) loadVNodeType() xNodeType {
	return xNodeType(skl.flags.atomicLoadBits(sklVNodeTypeBits))
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

// traverse locates the target key and store the
// nodes encountered during the indices traversal.
func (skl *xConcSkl[K, V]) traverse(
	lvl int32,
	key K,
	aux xConcSklAux[K, V],
) *xConcSklNode[K, V] {
	forward := skl.atomicLoadHead()
	// Vertical iteration.
	for l := lvl - 1; l >= 0; l-- {
		nIdx := forward.atomicLoadNext(l)
		// Horizontal iteration.
		for nIdx != nil && skl.kcmp(key, nIdx.key) > 0 {
			forward = nIdx
			nIdx = forward.atomicLoadNext(l)
		}

		aux.storePred(l, forward)
		aux.storeSucc(l, nIdx)

		if nIdx != nil && skl.kcmp(key, nIdx.key) == 0 {
			return nIdx // Found
		}
		// Downward to next level's indices.
	}
	return nil // Not found
}

// Insert add the val by a key into skip-list.
// Only works for unique element skip-list.
func (skl *xConcSkl[K, V]) Insert(key K, val V) {
	var (
		aux      = skl.pool.loadAux()
		oldIdxHi = skl.Level()
		newIdxHi = skl.rand( /* Avoid loop call */
			int(oldIdxHi),
			int32(atomic.LoadInt64(&skl.len)),
		)
		ver = skl.idGen.NumberUUID()
	)
	defer func() {
		skl.pool.releaseAux(aux)
	}()
	for {
		if node := skl.traverse(maxHeight(oldIdxHi, newIdxHi), key, aux); node != nil {
			// Check node whether is deleting by another G.
			if node.flags.atomicIsSet(nodeRemovingMarkedBit) {
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
			pred, succ, prevPred *xConcSklNode[K, V] = nil, nil, nil
			// Whether it is valid insertion.
			isValid      = true
			lockedLevels = int32(-1)
		)
		for l := int32(0); isValid && l < newIdxHi; l++ {
			pred, succ = aux.loadPred(l), aux.loadSucc(l)
			if pred != prevPred {
				// Try to lock.
				pred.mu.lock(ver)
				lockedLevels = l
				prevPred = pred
			}
			// Check indices and data node:
			//      +------+       +------+      +------+
			// ...  | pred |------>|  new |----->| succ | ...
			//      +------+       +------+      +------+
			// 1. Both the pred and succ isn't removing.
			// 2. The pred's next node is the succ in this level.
			isValid = !pred.flags.atomicIsSet(nodeRemovingMarkedBit) &&
				(succ == nil || !succ.flags.atomicIsSet(nodeRemovingMarkedBit)) &&
				pred.loadNext(l) == succ
		}
		if !isValid {
			// Insert failed due to concurrency, restart the iteration.
			aux.foreachPred(func(list ...*xConcSklNode[K, V]) {
				unlockNodes(ver, lockedLevels, list...)
			})
			continue
		}

		n := newXConcSkipListNode(key, val, newIdxHi, skl.loadMutexImpl(), skl.loadVNodeType(), skl.vcmp)
		for l := int32(0); l < newIdxHi; l++ {
			// Linking
			//      +------+       +------+      +------+
			// ...  | pred |------>|  new |----->| succ | ...
			//      +------+       +------+      +------+
			n.storeNext(l, aux.loadSucc(l))       // Useless to use atomic here.
			aux.loadPred(l).atomicStoreNext(l, n) // Memory barrier, concurrent safety.
		}
		n.flags.atomicSet(nodeFullyLinkedBit)
		if oldIdxHi = skl.Level(); oldIdxHi < newIdxHi {
			atomic.StoreInt32(&skl.idxHi, newIdxHi)
		}

		// Unlock
		aux.foreachPred(func(list ...*xConcSklNode[K, V]) {
			unlockNodes(ver, lockedLevels, list...)
		})
		atomic.AddInt64(&skl.len, 1)
		atomic.AddUint64(&skl.idxSize, uint64(newIdxHi))
		return
	}
}

// Range iterates each node (vnode within the node) by pass in function.
// Once the function return false, the iteration should be stopped.
// This function doesn't guarantee correctness in the case of concurrent
// reads and writes.
func (skl *xConcSkl[K, V]) Range(fn func(idx int64, metadata SkipListIterationItem[K, V]) bool) {
	forward := skl.atomicLoadHead().atomicLoadNext(0)
	idx := int64(0)
	typ := skl.loadVNodeType()
	item := &xSkipListIterationItem[K, V]{}
	switch typ {
	case unique:
		for forward != nil {
			if !forward.flags.atomicAreEqual(nodeFullyLinkedBit|nodeRemovingMarkedBit, fullyLinked) {
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
			if res := fn(idx, item); !res {
				break
			}
			forward = forward.atomicLoadNext(0)
			idx++
		}
	case linkedList:
		for forward != nil {
			if !forward.flags.atomicAreEqual(nodeFullyLinkedBit|nodeRemovingMarkedBit, fullyLinked) {
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
			for vn := forward.loadXNode().parent; vn != nil; vn = vn.parent {
				item.valFn = func() V {
					return *vn.vptr
				}
				var res bool
				if res, idx = fn(idx, item), idx+1; !res {
					break
				}
			}
			forward = forward.atomicLoadNext(0)
		}
	case rbtree:
		// TODO
	default:
		panic("unknown skip-list node type")
	}
}

// Get returns the val stored in the map for a key, or nil if no
// val is present.
// The ok result indicates whether val was found in the map.
func (skl *xConcSkl[K, V]) Get(key K) (val V, ok bool) {
	forward := skl.atomicLoadHead()
	typ := skl.loadVNodeType()
	for l := skl.Level() - 1; l >= 0; l-- {
		nIdx := forward.atomicLoadNext(l)
		for nIdx != nil && skl.kcmp(key, nIdx.key) > 0 {
			forward = nIdx
			nIdx = forward.atomicLoadNext(l)
		}

		// Check if the key already in the skip list.
		if nIdx != nil && skl.kcmp(key, nIdx.key) == 0 {
			if nIdx.flags.atomicAreEqual(nodeFullyLinkedBit|nodeRemovingMarkedBit, fullyLinked) {
				switch typ {
				case unique:
					vn := nIdx.loadXNode()
					return *vn.vptr, true
				case linkedList:
					vn := nIdx.loadXNode()
					return *vn.parent.vptr, true
				case rbtree:
					// TODO
				default:
					panic("unknown v-node type")
				}
			}
			return
		}
	}
	return
}

// rmTraverse locates the target key and store the
// nodes encountered during the indices traversal.
// Returns with the target key found level index.
func (skl *xConcSkl[K, V]) rmTraverse(
	weight K,
	aux xConcSklAux[K, V],
) (foundAtLevel int32) {
	// foundAtLevel represents the index of the first layer at which it found a node.
	foundAtLevel = -1
	forward := skl.atomicLoadHead()
	for l := skl.Level() - 1; l >= 0; l-- {
		nIdx := forward.atomicLoadNext(l)
		for nIdx != nil && skl.kcmp(weight, nIdx.key) > 0 {
			forward = nIdx
			nIdx = forward.atomicLoadNext(l)
		}

		// Ready to downward to next level.
		aux.storePred(l, forward)
		aux.storeSucc(l, nIdx)

		if foundAtLevel == -1 && nIdx != nil && skl.kcmp(weight, nIdx.key) == 0 {
			// key matched
			foundAtLevel = l
		}
	}
	return
}

// RemoveFirst deletes the val for a key, only the first value.
func (skl *xConcSkl[K, V]) RemoveFirst(key K) (ele SkipListElement[K, V], err error) {
	var (
		aux          = skl.pool.loadAux()
		rmTarget     *xConcSklNode[K, V]
		isMarked     bool // represents if this operation mark the node
		topLevel     = int32(-1)
		ver          = skl.idGen.NumberUUID()
		typ          = skl.loadVNodeType()
		foundAtLevel = int32(-1)
	)
	defer func() {
		skl.pool.releaseAux(aux)
	}()
	switch typ {
	// FIXME: Merge these 2 deletion loops logic
	case unique:
		for {
			foundAtLevel = skl.rmTraverse(key, aux)
			if isMarked || foundAtLevel != -1 &&
				aux.loadSucc(foundAtLevel).flags.atomicAreEqual(nodeFullyLinkedBit|nodeRemovingMarkedBit, fullyLinked) &&
				(int32(aux.loadSucc(foundAtLevel).level)-1) == foundAtLevel {
				if !isMarked {
					// Don't mark at once.
					// Suspend successors' operations
					rmTarget = aux.loadSucc(foundAtLevel)
					topLevel = foundAtLevel
					if !rmTarget.mu.tryLock(ver) {
						if rmTarget.flags.atomicIsSet(nodeRemovingMarkedBit) {
							// Double check.
							return nil, errors.New("remove lock acquire failed and node (v-node) has been marked as removing")
						}
						isMarked = false
						continue
					}

					// Segment Locked
					if rmTarget.flags.atomicIsSet(nodeRemovingMarkedBit) {
						// Double check.
						rmTarget.mu.unlock(ver)
						return nil, errors.New("node (v-node) has been marked as removing")
					}

					rmTarget.flags.atomicSet(nodeRemovingMarkedBit)
					isMarked = true
				}

				var (
					lockedLayers         = int32(-1)
					isValid              = true
					pred, succ, prevPred *xConcSklNode[K, V]
				)
				// Segment lock
				for l := int32(0); isValid && (l <= topLevel); l++ {
					pred, succ = aux.loadPred(l), aux.loadSucc(l)
					if pred != prevPred {
						pred.mu.lock(ver)
						lockedLayers = l
						prevPred = pred
					}
					// Check:
					// 1. the previous node exists.
					// 2. no other nodes are inserted into the skip list in this layer.
					isValid = !pred.flags.atomicIsSet(nodeRemovingMarkedBit) && pred.atomicLoadNext(l) == succ
				}
				if !isValid {
					aux.foreachPred(func(list ...*xConcSklNode[K, V]) {
						unlockNodes(ver, lockedLayers, list...)
					})
					continue
				}

				ele = &xSkipListElement[K, V]{
					key: key,
					val: *rmTarget.loadXNode().vptr,
				}
				atomic.AddInt64(&rmTarget.count, -1)
				atomic.AddInt64(&skl.len, -1)

				if atomic.LoadInt64(&rmTarget.count) <= 0 {
					// Here should no data race and try to reduce levels.
					// The physical deletion.
					for l := topLevel; l >= 0; l-- {
						// Fully unlinked.
						aux.loadPred(l).atomicStoreNext(l, rmTarget.loadNext(l))
					}
					atomic.AddUint64(&skl.idxSize, ^uint64(rmTarget.level-1))
				}

				rmTarget.mu.unlock(ver)
				aux.foreachPred(func(list ...*xConcSklNode[K, V]) {
					unlockNodes(ver, lockedLayers, list...)
				})
				return ele, nil
			}
			break
		}
	case linkedList, rbtree:
		for {
			foundAtLevel = skl.rmTraverse(key, aux)
			if isMarked || foundAtLevel != -1 {
				fullyLinkedButNotRemove := aux.loadSucc(foundAtLevel).flags.atomicAreEqual(nodeFullyLinkedBit|nodeRemovingMarkedBit, fullyLinked)
				succMatch := (int32(aux.loadSucc(foundAtLevel).level) - 1) == foundAtLevel
				if !succMatch {
					break
				} else if !fullyLinkedButNotRemove {
					continue
				}

				if fullyLinkedButNotRemove && !isMarked {
					// Don't mark at once.
					// Suspend successors' operations
					rmTarget = aux.loadSucc(foundAtLevel)
					topLevel = foundAtLevel
					if !rmTarget.mu.tryLock(ver) {
						continue
					}

					// Segment Locked
					if !rmTarget.flags.atomicIsSet(nodeRemovingMarkedBit) {
						rmTarget.flags.atomicSet(nodeRemovingMarkedBit)
					}
					isMarked = true
				}

				var (
					lockedLayers         = int32(-1)
					isValid              = true
					pred, succ, prevPred *xConcSklNode[K, V]
				)
				// Segment deletion.
				for l := int32(0); isValid && (l <= topLevel); l++ {
					pred, succ = aux.loadPred(l), aux.loadSucc(l)
					if pred != prevPred {
						pred.mu.lock(ver)
						lockedLayers = l
						prevPred = pred
					}
					// Check:
					// 1. the previous node exists.
					// 2. no other nodes are inserted into the skip list in this layer.
					isValid = !pred.flags.atomicIsSet(nodeRemovingMarkedBit) && pred.atomicLoadNext(l) == succ
				}
				if !isValid {
					aux.foreachPred(func(list ...*xConcSklNode[K, V]) {
						unlockNodes(ver, lockedLayers, list...)
					})
					continue
				}

				switch typ {
				case linkedList:
					// locked
					if n := rmTarget.root.linkedListNext(); n != nil {
						ele = &xSkipListElement[K, V]{
							key: key,
							val: *n.vptr,
						}
						atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&rmTarget.root.parent)), unsafe.Pointer(n.parent))
						atomic.AddInt64(&rmTarget.count, -1)
						atomic.AddInt64(&skl.len, -1)
						rmTarget.flags.atomicUnset(nodeRemovingMarkedBit)
					} else {
						atomic.StoreInt64(&rmTarget.count, 0)
					}
				case rbtree:
					// locked
					// TODO
				}

				if atomic.LoadInt64(&rmTarget.count) <= 0 {
					// Here should no data race and try to reduce levels.
					// The physical deletion.
					for l := topLevel; l >= 0; l-- {
						// Fully unlinked.
						aux.loadPred(l).atomicStoreNext(l, rmTarget.loadNext(l))
					}
					atomic.AddUint64(&skl.idxSize, ^uint64(rmTarget.level-1))
				}

				rmTarget.mu.unlock(ver)
				aux.foreachPred(func(list ...*xConcSklNode[K, V]) {
					unlockNodes(ver, lockedLayers, list...)
				})
				return ele, nil
			}
			break
		}
	default:
		panic("unknown v-node type")
	}

	if foundAtLevel == -1 {
		return nil, errors.New("not found remove target")
	}
	return nil, errors.New("others unknown reasons")
}

func NewXConcSkipList[K infra.OrderedKey, V comparable](cmp SklWeightComparator[K], rand SklRand) *xConcSkl[K, V] {
	//h := newXConcSklHead[K, V]()
	//h.flags.atomicSet(nodeFullyLinkedBit)
	//return &xConcSkl[K, V]{
	//	head:  h,
	//	idxHi: 0,
	//	len:   0,
	//	kcmp:   kcmp,
	//	rand:  rand,
	//}
	return nil
}

func maxHeight(i, j int32) int32 {
	if i > j {
		return i
	}
	return j
}

type xConcSklPool[K infra.OrderedKey, V comparable] struct {
	auxPool *sync.Pool
}

func newXConcSklPool[K infra.OrderedKey, V comparable]() *xConcSklPool[K, V] {
	p := &xConcSklPool[K, V]{
		auxPool: &sync.Pool{
			New: func() any {
				return make(xConcSklAux[K, V], 2*xSkipListMaxLevel)
			},
		},
	}
	return p
}

func (p *xConcSklPool[K, V]) loadAux() xConcSklAux[K, V] {
	return p.auxPool.Get().(xConcSklAux[K, V])
}

func (p *xConcSklPool[K, V]) releaseAux(aux xConcSklAux[K, V]) {
	// Override only
	p.auxPool.Put(aux)
}
