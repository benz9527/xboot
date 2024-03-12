package list

// References:
// https://people.csail.mit.edu/shanir/publications/LazySkipList.pdf
// github:
// https://github.com/zhangyunhao116/skipmap

import (
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/benz9527/xboot/lib/id"
	"github.com/benz9527/xboot/lib/infra"
)

const (
	sklMutexImplBits = 0x00FF // Indicating that the skip-list exclusive lock implementation type.
	sklVNodeTypeBits = 0x0300 // Indicating that the skip-list data node type, including unique, linkedList and rbtree.
)

type xConcSkipList[K infra.OrderedKey, V comparable] struct {
	head    *xConcSkipListNode[K, V]
	pool    *xConcSkipListPool[K, V]
	kcmp    infra.OrderedKeyComparator[K]
	vcmp    SkipListValueComparator[V]
	rand    SkipListRand
	idGen   id.Generator
	flags   flagBits
	len     int64  // skip-list's node size
	idxSize uint64 // skip-list's index count
	idxHi   int32  // skip-list's indexes' height
}

func (skl *xConcSkipList[K, V]) loadMutexImpl() mutexImpl {
	return mutexImpl(skl.flags.atomicLoadBits(sklMutexImplBits))
}

func (skl *xConcSkipList[K, V]) loadVNodeType() vNodeType {
	return vNodeType(skl.flags.atomicLoadBits(sklVNodeTypeBits))
}

func (skl *xConcSkipList[K, V]) atomicLoadHead() *xConcSkipListNode[K, V] {
	return (*xConcSkipListNode[K, V])(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&skl.head))))
}

func (skl *xConcSkipList[K, V]) Len() int {
	return int(atomic.LoadInt64(&skl.len))
}

func (skl *xConcSkipList[K, V]) Indexes() uint64 {
	return atomic.LoadUint64(&skl.idxSize)
}

func (skl *xConcSkipList[K, V]) Level() int32 {
	return atomic.LoadInt32(&skl.idxHi)
}

// traverse locates the target key and store the
// nodes encountered during the indices traversal.
func (skl *xConcSkipList[K, V]) traverse(
	lvl int32,
	key K,
	val V,
	aux xConcSkipListAuxiliary[K, V],
) *xConcSkipListNode[K, V] {
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
func (skl *xConcSkipList[K, V]) Insert(key K, val V) {
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
		if node := skl.traverse(maxHeight(oldIdxHi, newIdxHi), key, val, aux); node != nil {
			// Check node whether is deleting by another G.
			if node.flags.atomicIsSet(nodeRemovingMarkedBit) {
				continue
			}
			node.storeVal(val)
			return
		}
		// Node not present. Add this node into skip list.
		var (
			pred, succ, prevPred *xConcSkipListNode[K, V] = nil, nil, nil
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
			aux.foreachPred(func(list ...*xConcSkipListNode[K, V]) {
				unlockNodes(ver, lockedLevels, list...)
			})
			continue
		}

		n := newXConcSkipListNode(key, val, newIdxHi, skl.loadMutexImpl(), skl.loadVNodeType())
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
		aux.foreachPred(func(list ...*xConcSkipListNode[K, V]) {
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
func (skl *xConcSkipList[K, V]) Range(fn func(idx int64, metadata SkipListIterationItem[K, V]) bool) {
	forward := (*xConcSkipListNode[K, V])(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&skl.head)))).atomicLoadNext(0)
	idx := int64(0)
	typ := skl.loadVNodeType()
	item := &xSkipListIterationItem[K, V]{}
	switch typ {
	case unique:
		for forward != nil {
			if !forward.flags.atomicAreEqual(nodeFullyLinkedBit|nodeRemovingMarkedBit, nodeFullyLinkedBit) {
				forward = forward.atomicLoadNext(0)
				continue
			}
			item.keyFn = func() K {
				return forward.key
			}
			item.valFn = func() V {
				vn := forward.loadValNode()
				if vn == nil {
					return *new(V)
				}
				return *vn.val
			}
			item.nodeLevelFn = func() uint32 {
				return atomic.LoadUint32(&forward.level)
			}
			item.nodeItemCountFn = func() int64 {
				return atomic.LoadInt64(&forward.count)
			}
			if res := fn(idx, item); !res {
				break
			}
			forward = forward.atomicLoadNext(0)
			idx++
		}
	case linkedList:

	case rbtree:

	default:
		panic("unknown skip-list node type")
	}

}

// Get returns the val stored in the map for a key, or nil if no
// val is present.
// The ok result indicates whether val was found in the map.
func (skl *xConcSkipList[K, V]) Get(key K) (val V, ok bool) {
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
			if nIdx.flags.atomicAreEqual(nodeFullyLinkedBit|nodeRemovingMarkedBit, nodeFullyLinkedBit) {
				switch typ {
				case unique:
					vn := nIdx.loadValNode()
					return *vn.val, true
				case linkedList:
					// TODO
				case rbtree:
					// TODO
				default:
					panic("unknown skip-list node type")
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
func (skl *xConcSkipList[K, V]) rmTraverse(
	weight K,
	aux xConcSkipListAuxiliary[K, V],
) int32 {
	// foundAtLevel represents the index of the first layer at which it found a node.
	foundAtLevel, forward := int32(-1), (*xConcSkipListNode[K, V])(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&skl.head))))
	for l := skl.Level() - 1; l >= 0; l-- {
		nIdx := forward.atomicLoadNext(l)
		for nIdx != nil && skl.kcmp(weight, nIdx.key) > 0 {
			forward = nIdx
			nIdx = forward.atomicLoadNext(l)
		}

		// key matched
		aux.storePred(l, forward)
		aux.storeSucc(l, nIdx)

		// Check if the key already in the skip list.
		if foundAtLevel == int32(-1) && nIdx != nil && skl.kcmp(weight, nIdx.key) == 0 {
			foundAtLevel = l
		}
	}
	return foundAtLevel
}

// RemoveFirst deletes the val for a key, only the first value.
func (skl *xConcSkipList[K, V]) RemoveFirst(key K) (SkipListElement[K, V], bool) {
	var (
		aux      = skl.pool.loadAux()
		rmTarget *xConcSkipListNode[K, V]
		isMarked bool // represents if this operation mark the node
		topLevel = int32(-1)
		ver      = skl.idGen.NumberUUID()
		typ      = skl.loadVNodeType()
	)
	defer func() {
		skl.pool.releaseAux(aux)
	}()
	for {
		foundAtLevel := skl.rmTraverse(key, aux)
		if isMarked || /* this process mark this node, or we can find this node in the skip list */
			foundAtLevel != -1 &&
				aux.loadSucc(foundAtLevel).flags.atomicAreEqual(nodeFullyLinkedBit|nodeRemovingMarkedBit, nodeFullyLinkedBit) &&
				(int32(aux.loadSucc(foundAtLevel).level)-1) == foundAtLevel {
			if !isMarked {
				// Don't mark at once.
				// Suspend successors' operations
				rmTarget = aux.loadSucc(foundAtLevel)
				topLevel = foundAtLevel
				if !rmTarget.mu.tryLock(ver) {
					if rmTarget.flags.atomicIsSet(nodeRemovingMarkedBit) {
						// Double check.
						return nil, false
					}
					isMarked = false
					continue
				}

				if rmTarget.flags.atomicIsSet(nodeRemovingMarkedBit) {
					// Double check.
					rmTarget.mu.unlock(ver)
					return nil, false
				}

				rmTarget.flags.atomicSet(nodeRemovingMarkedBit)
				isMarked = true
			}

			// The physical deletion.
			var (
				lockedLayers         = int32(-1)
				isValid              = true
				pred, succ, prevPred *xConcSkipListNode[K, V]
			)
			for l := int32(0); isValid && (l <= topLevel); l++ {
				pred, succ = aux.loadPred(l), aux.loadSucc(l)
				if pred != prevPred {
					pred.mu.lock(ver)
					// Fully unlinked.
					lockedLayers = l
					prevPred = pred
				}
				// Check:
				// 1. the previous node exists.
				// 2. no other nodes are inserted into the skip list in this layer.
				isValid = !pred.flags.atomicIsSet(nodeRemovingMarkedBit) && pred.atomicLoadNext(l) == succ
			}
			if !isValid {
				aux.foreachPred(func(list ...*xConcSkipListNode[K, V]) {
					unlockNodes(ver, lockedLayers, list...)
				})
				continue
			}

			switch typ {
			case linkedList:
				// TODO
			case rbtree:
				// TODO
			case unique:
				for l := topLevel; l >= 0; l-- {
					// Here should no data race and try to reduce levels.
					aux.loadPred(l).atomicStoreNext(l, rmTarget.loadNext(l))
				}
				atomic.AddInt64(&rmTarget.count, -1)
			default:
				panic("unknown skip-list node type")
			}

			rmTarget.mu.unlock(ver)
			aux.foreachPred(func(list ...*xConcSkipListNode[K, V]) {
				unlockNodes(ver, lockedLayers, list...)
			})
			atomic.AddInt64(&skl.len, -1)
			atomic.AddUint64(&skl.idxSize, ^uint64(rmTarget.level-1))
			return &xSkipListElement[K, V]{
				key: key,
				val: *rmTarget.loadValNode().val,
			}, true
		}
		return nil, false
	}
}

func NewXConcSkipList[K infra.OrderedKey, V comparable](cmp SkipListWeightComparator[K], rand SkipListRand) *xConcSkipList[K, V] {
	//h := newXConcSkipListHead[K, V]()
	//h.flags.atomicSet(nodeFullyLinkedBit)
	//return &xConcSkipList[K, V]{
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

type xConcSkipListPool[K infra.OrderedKey, V comparable] struct {
	auxPool *sync.Pool
}

func newXConcSkipListPool[K infra.OrderedKey, V comparable]() *xConcSkipListPool[K, V] {
	p := &xConcSkipListPool[K, V]{
		auxPool: &sync.Pool{
			New: func() any {
				return make(xConcSkipListAuxiliary[K, V], 2*xSkipListMaxLevel)
			},
		},
	}
	return p
}

func (p *xConcSkipListPool[K, V]) loadAux() xConcSkipListAuxiliary[K, V] {
	return p.auxPool.Get().(xConcSkipListAuxiliary[K, V])
}

func (p *xConcSkipListPool[K, V]) releaseAux(aux xConcSkipListAuxiliary[K, V]) {
	// Override only
	p.auxPool.Put(aux)
}
