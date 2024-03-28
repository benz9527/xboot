package list

import (
	"sync/atomic"
	"unsafe"

	"github.com/benz9527/xboot/lib/id"
	"github.com/benz9527/xboot/lib/infra"
)

var _ SkipList[uint8, uint8] = (*xArenaSkl[uint8, uint8])(nil)

type xArenaSkl[K infra.OrderedKey, V any] struct {
	head       *xArenaSklElement[K, V]
	arena      *autoGrowthArena[xArenaSklNode[K, V]] // recycle resources
	kcmp       infra.OrderedKeyComparator[K]         // key comparator
	rand       SklRand
	optVer     id.UUIDGen // optimistic version generator
	nodeLen    int64      // skip-list's node count.
	indexCount uint64     // skip-list's index count.
	levels     int32      // skip-list's max height value inside the indexCount.
}

func (skl *xArenaSkl[K, V]) atomicLoadHead() *xArenaSklElement[K, V] {
	return (*xArenaSklElement[K, V])(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&skl.head))))
}

// traverse locates the target key and store the nodes encountered during the indexCount traversal.
func (skl *xArenaSkl[K, V]) traverse(
	lvl int32,
	key K,
	aux []*xArenaSklNode[K, V],
) *xArenaSklNode[K, V] {
	for /* vertical */ forward, l := skl.atomicLoadHead().nodeRef, lvl-1; l >= 0; l-- {
		nIdx := forward.atomicLoadNextNode(l)
		for /* horizontal */ nIdx != nil {
			if res := skl.kcmp(key, nIdx.elementRef.key); /* horizontal next */ res > 0 {
				forward = nIdx
				nIdx = forward.atomicLoadNextNode(l)
			} else if /* found */ res == 0 {
				aux[l] = forward          /* pred */
				aux[sklMaxLevel+l] = nIdx /* succ */
				return nIdx
			} else /* not found, vertical next */ {
				break
			}
		}

		aux[l] = forward          /* pred */
		aux[sklMaxLevel+l] = nIdx /* succ */
	}
	return nil
}

// rmTraverse locates the remove target key and stores the nodes encountered
// during the indices traversal.
// Returns with the target key found level index.
func (skl *xArenaSkl[K, V]) rmTraverse(
	weight K,
	aux []*xArenaSklNode[K, V],
) (foundAt int32) {
	// foundAt represents the index of the first layer at which it found a node.
	foundAt = -1
	forward := skl.atomicLoadHead().nodeRef
	for /* vertical */ l := skl.Levels() - 1; l >= 0; l-- {
		nIdx := forward.atomicLoadNextNode(l)
		for /* horizontal */ nIdx != nil && skl.kcmp(weight, nIdx.elementRef.key) > 0 {
			forward = nIdx
			nIdx = forward.atomicLoadNextNode(l)
		}

		aux[l] = forward
		aux[sklMaxLevel+l] = nIdx

		if foundAt == -1 && nIdx != nil && skl.kcmp(weight, nIdx.elementRef.key) == 0 {
			foundAt = l
		}
		// Downward to next level.
	}
	return
}

// Classic Skip-List basic APIs

// Len skip-list's node count.
func (skl *xArenaSkl[K, V]) Len() int64 {
	return atomic.LoadInt64(&skl.nodeLen)
}

func (skl *xArenaSkl[K, V]) IndexCount() uint64 {
	return atomic.LoadUint64(&skl.indexCount)
}

// Levels skip-list's max height value inside the indexCount.
func (skl *xArenaSkl[K, V]) Levels() int32 {
	return atomic.LoadInt32(&skl.levels)
}

// Insert add the val by a key into skip-list.
// Only works for unique element skip-list.
func (skl *xArenaSkl[K, V]) Insert(key K, val V, ifNotPresent ...bool) error {
	if skl.Len() >= sklMaxSize {
		return ErrXSklIsFull
	}

	var (
		aux     = make([]*xArenaSklNode[K, V], 2*sklMaxLevel)
		oldLvls = skl.Levels()
		newLvls = skl.rand(int(oldLvls), skl.Len()) // avoid loop call
		ver     = skl.optVer.Number()
	)

	if len(ifNotPresent) <= 0 {
		ifNotPresent = insertReplaceDisabled
	}

	for {
		if node := skl.traverse(max(oldLvls, newLvls), key, aux); node != nil {
			if /* conc rm */ node.flags.atomicIsSet(nodeRemovingFlagBit) {
				continue
			} else if /* conc d-check */ skl.Len() >= sklMaxSize {
				return ErrXSklIsFull
			}

			if ifNotPresent[0] {
				return ErrXSklDisabledValReplace
			}
			node.elementRef.val.Store(val)
			atomic.AddInt64(&skl.nodeLen, 1)
			return nil
		}
		// Node not present. Add this node into skip list.
		var (
			pred, succ, prev *xArenaSklNode[K, V]
			isValid          = true
			lockedLevels     = int32(-1)
		)
		for l := int32(0); isValid && l < newLvls; l++ {
			pred, succ = aux[l], aux[sklMaxLevel+l]
			if /* lock */ pred != prev {
				pred.lock(ver)
				lockedLevels = l
				prev = pred
			}
			// Check indexCount and data node:
			//      +------+       +------+      +------+
			// ...  | pred |------>|  new |----->| succ | ...
			//      +------+       +------+      +------+
			// 1. Both the pred and succ isn't removing.
			// 2. The pred's next node is the succ in this level.
			isValid = !pred.flags.atomicIsSet(nodeRemovingFlagBit) &&
				(succ == nil || !succ.flags.atomicIsSet(nodeRemovingFlagBit)) &&
				pred.atomicLoadNextNode(l) == succ
		}
		if /* conc insert */ !isValid {
			unlockArenaNodes(ver, lockedLevels, aux[0:sklMaxLevel]...)
			continue
		} else if /* conc d-check */ skl.Len() >= sklMaxSize {
			unlockArenaNodes(ver, lockedLevels, aux[0:sklMaxLevel]...)
			return ErrXSklIsFull
		}
		// node := skl.arena.allocateXConcSklNode(uint32(newLvls))
		// node.init(key, val, skl.loadXNodeMode(), skl.vcmp, skl.arena.xNodeArena)
		e := newXArenaSklDataElement[K, V](key, val, uint32(newLvls), skl.arena)
		e.prev = aux[0].elementRef
		aux[0].elementRef.next = e
		if aux[sklMaxLevel] != nil {
			e.next = aux[sklMaxLevel].elementRef
			aux[sklMaxLevel].elementRef.prev = e
		}
		for /* linking */ l := int32(0); l < newLvls; l++ {
			//      +------+       +------+      +------+
			// ...  | pred |------>|  new |----->| succ | ...
			//      +------+       +------+      +------+
			e.nodeRef.storeNextNode(l, aux[sklMaxLevel+l]) // Useless to use atomic here.
			aux[l].atomicStoreNextNode(l, e.nodeRef)       // Memory barrier, concurrency safety.
		}
		e.nodeRef.flags.atomicSet(nodeInsertedFlagBit)
		if oldLvls = skl.Levels(); oldLvls < newLvls {
			atomic.StoreInt32(&skl.levels, newLvls)
		}

		unlockArenaNodes(ver, lockedLevels, aux[0:sklMaxLevel]...)
		atomic.AddInt64(&skl.nodeLen, 1)
		atomic.AddUint64(&skl.indexCount, uint64(newLvls))
		return nil
	}
}

// Foreach iterates each node (xNode within the node) by pass in function.
// Once the function return false, the iteration should be stopped.
// This function doesn't guarantee correctness in the case of concurrent
// reads and writes.
func (skl *xArenaSkl[K, V]) Foreach(action func(i int64, item SklIterationItem[K, V]) bool) {
	i := int64(0)
	item := &xSklIter[K, V]{}
	forward := skl.atomicLoadHead().nodeRef.atomicLoadNextNode(0)
	for forward != nil {
		if !forward.flags.atomicAreEqual(nodeInsertedFlagBit|nodeRemovingFlagBit, insertFullyLinked) {
			forward = forward.atomicLoadNextNode(0)
			continue
		}
		item.nodeLevelFn = func() uint32 {
			return atomic.LoadUint32(&forward.level)
		}
		item.nodeItemCountFn = func() int64 {
			return atomic.LoadInt64(&forward.count)
		}
		item.keyFn = func() K {
			return forward.elementRef.key
		}
		item.valFn = func() V {
			ele := forward.elementRef
			if ele == nil {
				return *new(V)
			}
			return ele.val.Load().(V)
		}
		if res := action(i, item); !res {
			break
		}
		forward = forward.atomicLoadNextNode(0)
		i++
	}
}

// LoadFirst returns the first value stored in the skip-list for a key,
// or nil if no val is present.
func (skl *xArenaSkl[K, V]) LoadFirst(key K) (element SklElement[K, V], err error) {
	if skl.Len() <= 0 {
		return nil, ErrXSklIsEmpty
	}

	forward := skl.atomicLoadHead().nodeRef
	for /* vertical */ l := skl.Levels() - 1; l >= 0; l-- {
		nIdx := forward.atomicLoadNextNode(l)
		for /* horizontal */ nIdx != nil && skl.kcmp(key, nIdx.elementRef.key) > 0 {
			forward = nIdx
			nIdx = forward.atomicLoadNextNode(l)
		}

		if /* found */ nIdx != nil && skl.kcmp(key, nIdx.elementRef.key) == 0 {
			if nIdx.flags.atomicAreEqual(nodeInsertedFlagBit|nodeRemovingFlagBit, insertFullyLinked) {
				if /* conc rw empty */ atomic.LoadInt64(&nIdx.count) <= 0 {
					return nil, ErrXSklConcRWLoadEmpty
				}
				if x := nIdx.elementRef; x == nil {
					return nil, ErrXSklConcRWLoadEmpty
				} else {
					return &xSklElement[K, V]{
						key: key,
						val: x.val.Load().(V),
					}, nil
				}
			}
			return nil, ErrXSklConcRWLoadFailed
		}
	}
	return nil, ErrXSklNotFound
}

// RemoveFirst deletes the val for a key, only the first value.
func (skl *xArenaSkl[K, V]) RemoveFirst(key K) (element SklElement[K, V], err error) {
	if skl.Len() <= 0 {
		return nil, ErrXSklIsEmpty
	}

	var (
		aux      = make([]*xArenaSklNode[K, V], 2*sklMaxLevel)
		rmNode   *xArenaSklNode[K, V]
		isMarked bool // represents if this operation mark the node
		topLevel = int32(-1)
		ver      = skl.optVer.Number()
		foundAt  = int32(-1)
	)

	for {
		foundAt = skl.rmTraverse(key, aux)
		if isMarked || foundAt != -1 &&
			aux[sklMaxLevel+foundAt].flags.atomicAreEqual(nodeInsertedFlagBit|nodeRemovingFlagBit, insertFullyLinked) &&
			(int32(aux[sklMaxLevel+foundAt].level)-1) == foundAt {
			if !isMarked {
				rmNode = aux[sklMaxLevel+foundAt]
				topLevel = foundAt
				if !rmNode.tryLock(ver) {
					if /* d-check */ rmNode.flags.atomicIsSet(nodeRemovingFlagBit) {
						return nil, ErrXSklConcRemoveTryLock
					}
					isMarked = false
					continue
				}

				if /* node locked, d-check */ rmNode.flags.atomicIsSet(nodeRemovingFlagBit) {
					rmNode.unlock(ver)
					return nil, ErrXSklConcRemoving
				}

				rmNode.flags.atomicSet(nodeRemovingFlagBit)
				isMarked = true
			}

			var (
				lockedLayers         = int32(-1)
				isValid              = true
				pred, succ, prevPred *xArenaSklNode[K, V]
			)
			for /* node locked */ l := int32(0); isValid && (l <= topLevel); l++ {
				pred, succ = aux[l], aux[sklMaxLevel+l]
				if /* lock indexCount */ pred != prevPred {
					pred.lock(ver)
					lockedLayers = l
					prevPred = pred
				}
				// Check:
				// 1. the previous node exists.
				// 2. no other nodes are inserted into the skip list in this layer.
				isValid = !pred.flags.atomicIsSet(nodeRemovingFlagBit) && pred.atomicLoadNextNode(l) == succ
			}
			if /* conc rm */ !isValid {
				unlockArenaNodes(ver, lockedLayers, aux[0:sklMaxLevel]...)
				continue
			}

			element = &xSklElement[K, V]{
				key: key,
				val: rmNode.elementRef.val.Load().(V),
			}
			atomic.AddInt64(&rmNode.count, -1)
			atomic.AddInt64(&skl.nodeLen, -1)

			if atomic.LoadInt64(&rmNode.count) <= 0 {
				for /* re-linking, reduce levels */ l := topLevel; l >= 0; l-- {
					aux[l].atomicStoreNextNode(l, rmNode.loadNextNode(l))
				}
				atomic.AddUint64(&skl.indexCount, ^uint64(rmNode.level-1))
			}

			rmNode.unlock(ver)
			unlockArenaNodes(ver, lockedLayers, aux[0:sklMaxLevel]...)
			return element, nil
		}
		break
	}

	if foundAt == -1 {
		return nil, ErrXSklNotFound
	}
	return nil, ErrXSklUnknownReason
}

func (skl *xArenaSkl[K, V]) PeekHead() (element SklElement[K, V]) {
	forward := skl.atomicLoadHead().nodeRef.atomicLoadNextNode(0)
	for {
		if !forward.flags.atomicAreEqual(nodeInsertedFlagBit|nodeRemovingFlagBit, insertFullyLinked) {
			forward = forward.atomicLoadNextNode(0)
			continue
		}
		node := forward.elementRef
		if node == nil {
			return nil
		}
		element = &xSklElement[K, V]{
			key: forward.elementRef.key,
			val: node.val.Load().(V),
		}
		break
	}

	return element
}

func (skl *xArenaSkl[K, V]) PopHead() (element SklElement[K, V], err error) {
	forward := skl.atomicLoadHead().nodeRef.atomicLoadNextNode(0)
	if forward == nil {
		return nil, ErrXSklIsEmpty
	}
	return skl.RemoveFirst(forward.elementRef.key)
}
