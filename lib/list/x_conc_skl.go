package list

// References:
// https://people.csail.mit.edu/shanir/publications/LazySkipList.pdf
// github:
// https://github.com/zhangyunhao116/skipmap

import (
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/benz9527/xboot/lib/infra"
)

const (
	sklMutexType = 0x00FF
	sklDuplicate = 0x0100
	sklRbtree    = 0x0200
)

type xConcSkipList[K infra.OrderedKey, V comparable] struct {
	head    *xConcSkipListNode[K, V]
	pool    *xConcSkipListPool[K, V]
	kcmp    infra.OrderedKeyComparator[K]
	vcmp    SkipListValueComparator[V]
	rand    SkipListRand
	id      *monotonicNonZeroID
	flags   flagBits
	len     int64  // skip-list's node size
	idxSize uint64 // skip-list's index count
	idxHi   int32  // skip-list's indexes' height
}

func (skl *xConcSkipList[K, V]) loadMutexEnum() mutexEnum {
	return mutexEnum(skl.flags.atomicLoadBits(sklMutexType))
}

func (skl *xConcSkipList[K, V]) isDuplicate() bool {
	return skl.flags.isSet(sklDuplicate)
}

// Default duplicated elements are store as singly linked list.
func (skl *xConcSkipList[K, V]) isRbtree() bool {
	return skl.flags.atomicAreEqual(sklDuplicate|sklRbtree, 0x3)
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

// traverse0 works for
// 1. Unique element skip-list.
// 2. Duplicate elements linked by rbtree in skip-list.
func (skl *xConcSkipList[K, V]) traverse0(
	level int32,
	weight K,
	obj V,
	aux xConcSkipListAuxiliary[K, V],
) *xConcSkipListNode[K, V] {
	forward := (*xConcSkipListNode[K, V])(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&skl.head))))
	// Vertical iteration.
	for l := level - 1; l >= 0; l-- {
		nIdx := forward.atomicLoadNext(l)
		// Horizontal iteration.
		for nIdx != nil && skl.kcmp(weight, nIdx.weight) > 0 {
			forward = nIdx
			nIdx = forward.atomicLoadNext(l)
		}

		aux.storePred(l, forward)
		aux.storeSucc(l, nIdx)

		if nIdx != nil && skl.kcmp(weight, nIdx.weight) == 0 {
			if !skl.isDuplicate() || skl.isRbtree() {
				return nIdx // Found
			}
		}
		// Downward to next level's indices.
	}
	return nil // Not found
}

// insert0 add the object by a weight into skip-list.
// Only works for unique element skip-list.
func (skl *xConcSkipList[K, V]) insert0(weight K, obj V) {
	var (
		aux      = skl.pool.loadAux()
		oldIdxHi = skl.Level()
		newIdxHi = skl.rand( /* Avoid loop call */
			int(oldIdxHi),
			int32(atomic.LoadInt64(&skl.len)),
		)
		ver = skl.id.next()
	)
	defer func() {
		skl.pool.releaseAux(aux)
	}()
	for {
		if node := skl.traverse0(maxHeight(oldIdxHi, newIdxHi), weight, obj, aux); node != nil {
			// Check node whether is deleting by another G.
			if node.flags.atomicIsSet(nodeRemovingMarked) {
				continue
			}
			node.storeObject(obj)
			return
		}
		// Node not present. Add this node into skip list.
		var (
			pred, succ, prevPred *xConcSkipListNode[K, V] = nil, nil, nil
			lockedLevels                                  = int32(-1)
			isValidInsert                                 = true
		)
		for l := int32(0); isValidInsert && l < newIdxHi; l++ {
			pred, succ = aux.loadPred(l), aux.loadSucc(l)
			if pred != prevPred {
				// the node in this layer could be locked by previous loop
				// Lock the traversed indexes' node.
				pred.mu.lock(ver)
				lockedLevels = l
				prevPred = pred
			}
			// Check:
			//      +------+       +------+      +------+
			// ...  | pred |------>|  new |----->| succ | ...
			//      +------+       +------+      +------+
			// 1. Both the pred and succ isn't removing.
			// 2. The pred's next node is the succ in this level.
			isValidInsert = !pred.flags.atomicIsSet(nodeRemovingMarked) &&
				(succ == nil || !succ.flags.atomicIsSet(nodeRemovingMarked)) &&
				pred.loadNext(l) == succ
		}
		if !isValidInsert {
			aux.foreachPred(func(list ...*xConcSkipListNode[K, V]) {
				unlockNodes(ver, lockedLevels, list...)
			})
			// Insert failed due to concurrency, restart the iteration.
			continue
		}

		n := newXConcSkipListNode(weight, obj, newIdxHi, skl.loadMutexEnum())
		for l := int32(0); l < newIdxHi; l++ {
			// Linking
			//      +------+       +------+      +------+
			// ...  | pred |------>|  new |----->| succ | ...
			//      +------+       +------+      +------+
			n.storeNext(l, aux.loadSucc(l))       // Useless to use atomic here.
			aux.loadPred(l).atomicStoreNext(l, n) // Memory barrier, concurrent safety.
		}
		n.flags.atomicSet(nodeFullyLinked)
		if oldIdxHi = skl.Level(); oldIdxHi < newIdxHi {
			atomic.StoreInt32(&skl.idxHi, newIdxHi)
		}
		aux.foreachPred(func(list ...*xConcSkipListNode[K, V]) {
			unlockNodes(ver, lockedLevels, list...)
		})
		atomic.AddInt64(&skl.len, 1)
		atomic.AddUint64(&skl.idxSize, uint64(newIdxHi))
		return
	}
}

// traverse1 only works for duplicate elements linked by doubly linked-list in skip-list.
func (skl *xConcSkipList[K, V]) traverse1(
	level int32,
	weight K,
	obj V,
	aux xConcSkipListAuxiliary[K, V],
) *xConcSkipListNode[K, V] {
	forward := (*xConcSkipListNode[K, V])(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&skl.head))))
	// Vertical iteration.
	for l := level - 1; l >= 0; l-- {
		nIdx := forward.atomicLoadNext(l)
		// Horizontal iteration.
		for nIdx != nil && skl.kcmp(weight, nIdx.weight) > 0 {
			forward = nIdx
			nIdx = forward.atomicLoadNext(l)
		}

		aux.storePred(l, forward)
		aux.storeSucc(l, nIdx)

		if nIdx != nil && skl.kcmp(weight, nIdx.weight) == 0 {
			status := int8(2)
			for l == 0 && nIdx != nil && skl.kcmp(weight, nIdx.weight) == 0 {
				// Horizontal iteration.
				res := int64(obj.Hash() - (*nIdx.object).Hash())
				if res == 0 {
					return nIdx // Found
				} else if res < 0 {
					if status == 1 {
						aux.storePred(l, forward)
						aux.storeSucc(l, nIdx)
						goto notFound // Back and forth, not found
					}
					forward = nIdx
					nIdx = forward.atomicLoadPrev(l)
					status = -1
				} else {
					if status == -1 {
						aux.storePred(l, nIdx)
						aux.storeSucc(l, forward)
						goto notFound // Back and forth, not found
					}
					forward = nIdx
					nIdx = forward.atomicLoadNext(l)
					status = 1
				}
			}
			if status == 1 {
				aux.storePred(l, forward)
				aux.storeSucc(l, nIdx)
			} else if status == -1 {
				aux.storePred(l, nIdx)
				aux.storeSucc(l, forward)
			}
		}
		// Downward to next level's indices.
	}
notFound:
	return nil // Not found
}

// insert1 add the object by a weight into skip-list.
// Duplicate elements linked by doubly linked-list in skip-list.
func (skl *xConcSkipList[K, V]) insert1(weight K, obj V) {
	var (
		aux      = skl.pool.loadAux()
		oldIdxHi = skl.Level()
		newIdxHi = skl.rand( /* Avoid loop call */
			int(oldIdxHi),
			int32(atomic.LoadInt64(&skl.len)),
		)
		ver = skl.id.next()
	)
	defer func() {
		skl.pool.releaseAux(aux)
	}()
	for {
		if node := skl.traverse1(maxHeight(oldIdxHi, newIdxHi), weight, obj, aux); node != nil {
			// Check node whether is deleting by another G.
			if node.flags.atomicIsSet(nodeRemovingMarked) {
				continue
			}
			node.storeObject(obj)
			return
		}
		// Node not present. Add this node into skip list.
		var (
			pred, succ, prevPred *xConcSkipListNode[K, V] = nil, nil, nil
			lockedLevels                                  = int32(-1)
			isValidInsert                                 = true
		)
		for l := int32(0); isValidInsert && l < newIdxHi; l++ {
			pred, succ = aux.loadPred(l), aux.loadSucc(l)
			if pred != prevPred {
				// the node in this layer could be locked by previous loop
				// Lock the traversed indexes' node.
				pred.mu.lock(ver)
				lockedLevels = l
				prevPred = pred
			}
			// Check:
			//      +------+       +------+      +------+
			// ...  | pred |------>|  new |----->| succ | ...
			//      +------+       +------+      +------+
			// 1. Both the pred and succ isn't removing.
			// 2. The pred's next node is the succ in this level.
			isValidInsert = !pred.flags.atomicIsSet(nodeRemovingMarked) &&
				(succ == nil || !succ.flags.atomicIsSet(nodeRemovingMarked)) &&
				pred.loadNext(l) == succ
		}
		if !isValidInsert {
			aux.foreachPred(func(list ...*xConcSkipListNode[K, V]) {
				unlockNodes(ver, lockedLevels, list...)
			})
			// Insert failed due to concurrency, restart the iteration.
			continue
		}

		n := newXConcSkipListNode(weight, obj, newIdxHi, skl.loadMutexEnum())
		for l := int32(0); l < newIdxHi; l++ {
			// Linking
			p, s := aux.loadPred(l), aux.loadSucc(l)
			// doubly linked forward:
			//      +------+       +------+      +------+
			// ...  | pred |------>|  new |----->| succ | ...
			//      +------+       +------+      +------+
			// doubly linked backward:
			//      +------+       +------+      +------+
			// ...  | pred |<------|  new |<-----| succ | ...
			//      +------+       +------+      +------+
			// Here must use atomic for all. Otherwise, there will be data race.
			n.atomicStoreNext(l, s)
			n.atomicStorePrev(l, p)
			p.atomicStoreNext(l, n)
			if s != nil {
				s.atomicStorePrev(l, n)
			}
		}
		n.flags.atomicSet(nodeFullyLinked)
		if oldIdxHi = skl.Level(); oldIdxHi < newIdxHi {
			atomic.StoreInt32(&skl.idxHi, newIdxHi)
		}
		aux.foreachPred(func(list ...*xConcSkipListNode[K, V]) {
			unlockNodes(ver, lockedLevels, list...)
		})
		atomic.AddInt64(&skl.len, 1)
		atomic.AddUint64(&skl.idxSize, uint64(newIdxHi))
		return
	}
}

// insert2 add the object by a weight into skip-list.
// Duplicate elements linked by rbtree in skip-list.
func (skl *xConcSkipList[K, V]) insert2(weight K, obj V) {
	// TODO implement the rbtree
}

// Insert sets the object for a weight.
func (skl *xConcSkipList[K, V]) Insert(weight K, obj V) {
	if !skl.isDuplicate() {
		skl.insert0(weight, obj)
		return
	}
	if !skl.isRbtree() {
		skl.insert1(weight, obj)
	} else {
		skl.insert2(weight, obj)
	}
}

func (skl *xConcSkipList[K, V]) Foreach(fn func(idx int64, weight K, object V)) {
	forward := (*xConcSkipListNode[K, V])(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&skl.head)))).atomicLoadNext(0)
	idx := int64(0)
	for forward != nil {
		if !forward.flags.atomicAreEqual(nodeFullyLinked|nodeRemovingMarked, nodeFullyLinked) {
			forward = forward.atomicLoadNext(0)
			continue
		}
		fn(idx, forward.weight, forward.loadObject())
		forward = forward.atomicLoadNext(0)
		idx++
	}
}

// Get returns the object stored in the map for a weight, or nil if no
// object is present.
// The ok result indicates whether object was found in the map.
func (skl *xConcSkipList[K, V]) Get(weight K) (obj V, ok bool) {
	forward := (*xConcSkipListNode[K, V])(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&skl.head))))
	for l := skl.Level() - 1; l >= 0; l-- {
		nIdx := forward.atomicLoadNext(l)
		for nIdx != nil && skl.kcmp(weight, nIdx.weight) > 0 {
			forward = nIdx
			nIdx = forward.atomicLoadNext(l)
		}

		// Check if the weight already in the skip list.
		if nIdx != nil && skl.kcmp(weight, nIdx.weight) == 0 {
			if nIdx.flags.atomicAreEqual(nodeFullyLinked|nodeRemovingMarked, nodeFullyLinked) {
				return nIdx.loadObject(), true
			}
			return
		}
	}
	return
}

// rmTraverse0 works for
// 1. Unique element skip-list.
// 2. Duplicate elements linked by rbtree in skip-list.
func (skl *xConcSkipList[K, V]) rmTraverse0(
	weight K,
	aux xConcSkipListAuxiliary[K, V],
) int32 {
	// foundAtLevel represents the index of the first layer at which it found a node.
	foundAtLevel, forward := int32(-1), (*xConcSkipListNode[K, V])(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&skl.head))))
	for l := skl.Level() - 1; l >= 0; l-- {
		nIdx := forward.atomicLoadNext(l)
		for nIdx != nil && skl.kcmp(weight, nIdx.weight) > 0 {
			forward = nIdx
			nIdx = forward.atomicLoadNext(l)
		}

		// weight matched
		aux.storePred(l, forward)
		aux.storeSucc(l, nIdx)

		// Check if the weight already in the skip list.
		if foundAtLevel == int32(-1) && nIdx != nil && skl.kcmp(weight, nIdx.weight) == 0 {
			foundAtLevel = l
		}
	}
	return foundAtLevel
}

// remove0 deletes the object for a weight.
// Only for unique element skip-list.
// Only remove the first weight matched element.
func (skl *xConcSkipList[K, V]) remove0(weight K) (SkipListElement[K, V], bool) {
	var (
		aux      = skl.pool.loadAux()
		rmTarget *xConcSkipListNode[K, V]
		isMarked bool // represents if this operation mark the node
		topLevel = int32(-1)
		ver      = skl.id.next()
	)
	defer func() {
		skl.pool.releaseAux(aux)
	}()
	for {
		foundAtLevel := skl.rmTraverse0(weight, aux)
		if isMarked || /* this process mark this node, or we can find this node in the skip list */
			foundAtLevel != -1 &&
				aux.loadSucc(foundAtLevel).flags.atomicAreEqual(nodeFullyLinked|nodeRemovingMarked, nodeFullyLinked) &&
				(int32(aux.loadSucc(foundAtLevel).level)-1) == foundAtLevel {
			if !isMarked {
				// Don't mark at once.
				// Suspend successors' operations
				rmTarget = aux.loadSucc(foundAtLevel)
				topLevel = foundAtLevel
				if !rmTarget.mu.tryLock(ver) {
					if rmTarget.flags.atomicIsSet(nodeRemovingMarked) {
						// Double check.
						return nil, false
					}
					isMarked = false
					continue
				}

				if rmTarget.flags.atomicIsSet(nodeRemovingMarked) {
					// Double check.
					rmTarget.mu.unlock(ver)
					return nil, false
				}

				rmTarget.flags.atomicSet(nodeRemovingMarked)
				isMarked = true
			}

			// The physical deletion.
			var (
				lockedLayers         = int32(-1)
				isValidRemove        = true
				pred, succ, prevPred *xConcSkipListNode[K, V]
			)
			for l := int32(0); isValidRemove && (l <= topLevel); l++ {
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
				isValidRemove = !pred.flags.atomicIsSet(nodeRemovingMarked) && pred.atomicLoadNext(l) == succ
			}
			if !isValidRemove {
				aux.foreachPred(func(list ...*xConcSkipListNode[K, V]) {
					unlockNodes(ver, lockedLayers, list...)
				})
				continue
			}

			for l := topLevel; l >= 0; l-- {
				// Here should no data race.
				aux.loadPred(l).atomicStoreNext(l, rmTarget.loadNext(l))
			}
			rmTarget.mu.unlock(ver)
			aux.foreachPred(func(list ...*xConcSkipListNode[K, V]) {
				unlockNodes(ver, lockedLayers, list...)
			})
			atomic.AddInt64(&skl.len, -1)
			atomic.AddUint64(&skl.idxSize, ^uint64(rmTarget.level-1))
			return &xSkipListElement[K, V]{
				weight: weight,
				object: *rmTarget.object,
			}, true
		}
		return nil, false
	}
}

// rmTraverse1 only works for duplicate elements linked by doubly linked-list in skip-list.
// Find the first node which weight equals to target.
func (skl *xConcSkipList[K, V]) rmTraverse1(
	weight K,
	aux xConcSkipListAuxiliary[K, V],
) int32 {
	// foundAtLevel represents the index of the first layer at which it found a node.
	foundAtLevel, forward := int32(-1), (*xConcSkipListNode[K, V])(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&skl.head))))
	for l := skl.Level() - 1; l >= 0; l-- {
		nIdx := forward.atomicLoadNext(l)
		for nIdx != nil && skl.kcmp(weight, nIdx.weight) > 0 {
			forward = nIdx
			nIdx = forward.atomicLoadNext(l)
		}

		aux.storePred(l, forward)
		aux.storeSucc(l, nIdx)

		// Traverse left part.
		// But at the same level, if we find the right part first,
		// it is impossible for us to backward to find from left part again.
		// Only works for entering the next level, we're still iterating
		// current indices.
		if nIdx != nil && skl.kcmp(weight, nIdx.weight) == 0 {
			backward := nIdx.atomicLoadPrev(l)
			for backward != nil && skl.kcmp(weight, backward.weight) == 0 {
				nIdx = backward
				backward = nIdx.atomicLoadPrev(l)
			}
			// Try to arrive at the first node's predecessor
			if l == 0 {
				foundAtLevel = int32(nIdx.level)
				for i := int32(1); i <= foundAtLevel; i++ {
					aux.storeSucc(i, nIdx)
				}
				aux.storePred(0, backward)
				aux.storeSucc(0, nIdx)
			} else if l > 0 {
				aux.storePred(l, nIdx)
				if nIdx != nil {
					aux.storeSucc(l, nIdx.atomicLoadNext(l))
				}
			}
		}
	}
	return foundAtLevel
}

// remove1 deletes the object for a weight.
// Only works for duplicate elements linked by doubly linked-list in skip-list.
// Only remove the first weight matched element.
func (skl *xConcSkipList[K, V]) remove1(weight K) (SkipListElement[K, V], bool) {
	var (
		aux      = skl.pool.loadAux()
		rmTarget *xConcSkipListNode[K, V]
		isMarked bool // represents if this operation mark the node
		topLevel = int32(-1)
		ver      = skl.id.next()
	)
	defer func() {
		skl.pool.releaseAux(aux)
	}()
	for {
		foundAtLevel := skl.rmTraverse1(weight, aux)
		if isMarked || /* this process mark this node, or we can find this node in the skip list */
			foundAtLevel != -1 &&
				aux.loadSucc(foundAtLevel).flags.atomicAreEqual(nodeFullyLinked|nodeRemovingMarked, nodeFullyLinked) &&
				(int32(aux.loadSucc(foundAtLevel).level)-1) == foundAtLevel {
			if !isMarked {
				// Don't mark at once.
				// Suspend successors' operations
				rmTarget = aux.loadSucc(foundAtLevel)
				topLevel = foundAtLevel
				if !rmTarget.mu.tryLock(ver) {
					if rmTarget.flags.atomicIsSet(nodeRemovingMarked) {
						// Double check.
						return nil, false
					}
					isMarked = false
					continue
				}

				if rmTarget.flags.atomicIsSet(nodeRemovingMarked) {
					// Double check.
					rmTarget.mu.unlock(ver)
					return nil, false
				}

				rmTarget.flags.atomicSet(nodeRemovingMarked)
				isMarked = true
			}

			// The physical deletion.
			var (
				lockedLayers         = int32(-1)
				isValidRemove        = true
				pred, succ, prevPred *xConcSkipListNode[K, V]
			)
			for l := int32(0); isValidRemove && (l <= topLevel); l++ {
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
				isValidRemove = !pred.flags.atomicIsSet(nodeRemovingMarked) && pred.atomicLoadNext(l) == succ
			}
			if !isValidRemove {
				aux.foreachPred(func(list ...*xConcSkipListNode[K, V]) {
					unlockNodes(ver, lockedLayers, list...)
				})
				continue
			}

			for l := topLevel; l >= 0; l-- {
				// Here should no data race.
				aux.loadPred(l).atomicStoreNext(l, rmTarget.loadNext(l))
			}
			rmTarget.mu.unlock(ver)
			aux.foreachPred(func(list ...*xConcSkipListNode[K, V]) {
				unlockNodes(ver, lockedLayers, list...)
			})
			atomic.AddInt64(&skl.len, -1)
			atomic.AddUint64(&skl.idxSize, ^uint64(rmTarget.level-1))
			return &xSkipListElement[K, V]{
				weight: weight,
				object: *rmTarget.object,
			}, true
		}
		return nil, false
	}
}

// remove2 deletes the object for a weight.
// Duplicate elements linked by rbtree in skip-list.
// Only remove the first weight matched element.
func (skl *xConcSkipList[K, V]) remove2(weight K) (SkipListElement[K, V], bool) {
	panic("not implement")
}

// RemoveFirst deletes the object for a weight.
func (skl *xConcSkipList[K, V]) RemoveFirst(weight K) (SkipListElement[K, V], bool) {
	if !skl.isDuplicate() {
		return skl.remove0(weight)
	} else if !skl.isRbtree() {
		return skl.remove1(weight)
	}
	return skl.remove2(weight)
}

// Range calls f sequentially for each weight and object present in the skip-list.
// If f returns false, range stops the iteration.
//
// Range does not necessarily correspond to any consistent snapshot of the Map's
// contents: each weight will not be visited more than once, but if the object for any weight
// is stored or deleted concurrently, Range may reflect any mapping for that weight
// from any point during the Range call.
func (skl *xConcSkipList[K, V]) Range(fn func(weight K, obj V) bool) {
	x := skl.head.atomicLoadNext(0)
	for x != nil {
		if !x.flags.atomicAreEqual(nodeFullyLinked|nodeRemovingMarked, nodeFullyLinked) {
			x = x.atomicLoadNext(0)
			continue
		}
		if !fn(x.weight, x.loadObject()) {
			break
		}
		x = x.atomicLoadNext(0)
	}
}

func NewXConcSkipList[W SkipListWeight, O HashObject](cmp SkipListWeightComparator[W], rand SkipListRand) *xConcSkipList[W, O] {
	//h := newXConcSkipListHead[K, V]()
	//h.flags.atomicSet(nodeFullyLinked)
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

type xConcSkipListPool[W SkipListWeight, O HashObject] struct {
	auxPool *sync.Pool
}

func newXConcSkipListPool[W SkipListWeight, O HashObject]() *xConcSkipListPool[W, O] {
	p := &xConcSkipListPool[W, O]{
		auxPool: &sync.Pool{
			New: func() any {
				return make(xConcSkipListAuxiliary[W, O], 2*xSkipListMaxLevel)
			},
		},
	}
	return p
}

func (p *xConcSkipListPool[W, O]) loadAux() xConcSkipListAuxiliary[W, O] {
	return p.auxPool.Get().(xConcSkipListAuxiliary[W, O])
}

func (p *xConcSkipListPool[W, O]) releaseAux(aux xConcSkipListAuxiliary[W, O]) {
	// Override only
	p.auxPool.Put(aux)
}
