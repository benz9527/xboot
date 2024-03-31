package list

import (
	"sync/atomic"
	"unsafe"

	"github.com/benz9527/xboot/lib/id"
	"github.com/benz9527/xboot/lib/infra"
)

const (
	// Indicating that the skip-list data node type, including unique, linkedList and rbtree.
	xConcSklXNodeModeFlagBits = 0x0003
	// Indicating that the skip-list data node mode is rbtree and do delete operation will borrow pred(0) or succ node(1).
	xConcSklRbtreeRmBorrowFlagBit = 0x0004
	// Indication that the skip-list key sort direction.
	xConcSklKeyCmpFlagBit = 0x0008 /* 0: asc; 1: desc */
)

var _ XSkipList[uint8, uint8] = (*xConcSkl[uint8, uint8])(nil)

type xConcSkl[K infra.OrderedKey, V any] struct {
	head       *xConcSklNode[K, V]
	vcmp       SklValComparator[V] // value comparator
	rand       SklRand
	optVer     id.UUIDGen // optimistic version generator
	nodeLen    int64      // skip-list's node count.
	indexCount uint64     // skip-list's index count.
	flags      uint32
	levels     int32 // skip-list's max height value inside the indexCount.
}

func (skl *xConcSkl[K, V]) loadXNodeMode() xNodeMode {
	return xNodeMode(atomicLoadBits(&skl.flags, xConcSklXNodeModeFlagBits))
}

func (skl *xConcSkl[K, V]) atomicLoadHead() *xConcSklNode[K, V] {
	return (*xConcSklNode[K, V])(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&skl.head))))
}

// traverse locates the target key and store the nodes encountered during the indexCount traversal.
func (skl *xConcSkl[K, V]) traverse(
	lvl int32,
	key K,
	isDesc bool,
	aux []*xConcSklNode[K, V],
) *xConcSklNode[K, V] {
	for /* vertical */ forward, l := skl.atomicLoadHead(), lvl-1; l >= 0; l-- {
		nIdx := forward.atomicLoadNextNode(l)
		for /* horizontal */ nIdx != nil {
			if /* horizontal next */ (!isDesc && key > nIdx.key) || (isDesc && key < nIdx.key) {
				forward = nIdx
				nIdx = forward.atomicLoadNextNode(l)
			} else if /* found */ key == nIdx.key {
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
func (skl *xConcSkl[K, V]) rmTraverse(
	key K,
	isDesc bool,
	aux []*xConcSklNode[K, V],
) (foundAt int32) {
	// foundAt represents the index of the first layer at which it found a node.
	foundAt = -1
	forward := skl.atomicLoadHead()
	for /* vertical */ l := skl.Levels() - 1; l >= 0; l-- {
		nIdx := forward.atomicLoadNextNode(l)
		for /* horizontal */ nIdx != nil && ((!isDesc && key > nIdx.key) || (isDesc && key < nIdx.key)) {
			forward = nIdx
			nIdx = forward.atomicLoadNextNode(l)
		}

		aux[l] = forward
		aux[sklMaxLevel+l] = nIdx

		if foundAt == -1 && nIdx != nil && key == nIdx.key {
			foundAt = l
		}
		// Downward to next level.
	}
	return
}

// Classic Skip-List basic APIs

// Len skip-list's node count.
func (skl *xConcSkl[K, V]) Len() int64 {
	return atomic.LoadInt64(&skl.nodeLen)
}

func (skl *xConcSkl[K, V]) IndexCount() uint64 {
	return atomic.LoadUint64(&skl.indexCount)
}

// Levels skip-list's max height value inside the indexCount.
func (skl *xConcSkl[K, V]) Levels() int32 {
	return atomic.LoadInt32(&skl.levels)
}

// Insert add the val by a key into skip-list.
// Only works for unique element skip-list.
func (skl *xConcSkl[K, V]) Insert(key K, val V, ifNotPresent ...bool) error {
	if skl.Len() >= sklMaxSize {
		return ErrXSklIsFull
	}

	var (
		aux     = make([]*xConcSklNode[K, V], 2*sklMaxLevel)
		oldLvls = skl.Levels()
		newLvls = skl.rand(int(oldLvls), skl.Len()) // avoid loop call
		ver     = skl.optVer.Number()
		isDesc  = isSet(skl.flags, xConcSklKeyCmpFlagBit)
	)

	if len(ifNotPresent) <= 0 {
		ifNotPresent = insertReplaceDisabled
	}

	for {
		if node := skl.traverse(max(oldLvls, newLvls), key, isDesc, aux); node != nil {
			if /* conc rm */ atomicIsSet(&node.flags, nodeRemovingFlagBit) {
				continue
			} else if /* conc d-check */ skl.Len() >= sklMaxSize {
				return ErrXSklIsFull
			}

			if isAppend, err := node.storeVal(ver, val, skl.vcmp, ifNotPresent...); err != nil {
				return err
			} else if isAppend {
				atomic.AddInt64(&skl.nodeLen, 1)
			}
			return nil
		}
		// Node not present. Add this node into skip list.
		var (
			pred, succ, prev *xConcSklNode[K, V]
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
			isValid = !atomicIsSet(&pred.flags, nodeRemovingFlagBit) &&
				(succ == nil || !atomicIsSet(&succ.flags, nodeRemovingFlagBit)) &&
				pred.atomicLoadNextNode(l) == succ
		}
		if /* conc insert */ !isValid {
			unlockNodes(ver, lockedLevels, aux[0:sklMaxLevel]...)
			continue
		} else if /* conc d-check */ skl.Len() >= sklMaxSize {
			unlockNodes(ver, lockedLevels, aux[0:sklMaxLevel]...)
			return ErrXSklIsFull
		}

		node := newXConcSklNode(key, val, newLvls, skl.loadXNodeMode(), skl.vcmp)
		for /* linking */ l := int32(0); l < newLvls; l++ {
			//      +------+       +------+      +------+
			// ...  | pred |------>|  new |----->| succ | ...
			//      +------+       +------+      +------+
			node.storeNextNode(l, aux[sklMaxLevel+l]) // Useless to use atomic here.
			aux[l].atomicStoreNextNode(l, node)       // Memory barrier, concurrency safety.
		}
		atomicSet(&node.flags, nodeInsertedFlagBit)
		if oldLvls = skl.Levels(); oldLvls < newLvls {
			atomic.StoreInt32(&skl.levels, newLvls)
		}

		unlockNodes(ver, lockedLevels, aux[0:sklMaxLevel]...)
		atomic.AddInt64(&skl.nodeLen, 1)
		atomic.AddUint64(&skl.indexCount, uint64(newLvls))
		return nil
	}
}

// Foreach iterates each node (xNode within the node) by pass in function.
// Once the function return false, the iteration should be stopped.
// This function doesn't guarantee correctness in the case of concurrent
// reads and writes.
func (skl *xConcSkl[K, V]) Foreach(action func(i int64, item SklIterationItem[K, V]) bool) {
	i := int64(0)
	item := &xSklIter[K, V]{}
	switch forward, mode := skl.atomicLoadHead().atomicLoadNextNode(0), skl.loadXNodeMode(); mode {
	case unique:
		for forward != nil {
			if !atomicAreEqual(&forward.flags, nodeInsertedFlagBit|nodeRemovingFlagBit, insertFullyLinked) {
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
				return forward.key
			}
			item.valFn = func() V {
				node := forward.atomicLoadRoot()
				if node == nil {
					return *new(V)
				}
				return *node.vptr
			}
			if res := action(i, item); !res {
				break
			}
			forward = forward.atomicLoadNextNode(0)
			i++
		}
	case linkedList:
		for forward != nil {
			if !atomicAreEqual(&forward.flags, nodeInsertedFlagBit|nodeRemovingFlagBit, insertFullyLinked) {
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
				return forward.key
			}
			for x := forward.atomicLoadRoot().parent; x != nil; x = x.parent {
				item.valFn = func() V {
					return *x.vptr
				}
				var res bool
				if res, i = action(i, item), i+1; !res {
					break
				}
			}
			forward = forward.atomicLoadNextNode(0)
		}
	case rbtree:
		for forward != nil {
			if !atomicAreEqual(&forward.flags, nodeInsertedFlagBit|nodeRemovingFlagBit, insertFullyLinked) {
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
				return forward.key
			}
			forward.rbDFS(func(idx int64, color color, val V) bool {
				item.valFn = func() V {
					return val
				}
				var res bool
				if res, i = action(i, item), i+1; !res {
					return false
				}
				return true
			})
			forward = forward.atomicLoadNextNode(0)
		}
	default:
		panic("[x-conc-skl] unknown node type")
	}
}

// LoadFirst returns the first value stored in the skip-list for a key,
// or nil if no val is present.
func (skl *xConcSkl[K, V]) LoadFirst(key K) (element SklElement[K, V], err error) {
	if skl.Len() <= 0 {
		return nil, ErrXSklIsEmpty
	}
	isDesc := isSet(skl.flags, xConcSklKeyCmpFlagBit)

	forward := skl.atomicLoadHead()
	mode := skl.loadXNodeMode()
	for /* vertical */ l := skl.Levels() - 1; l >= 0; l-- {
		nIdx := forward.atomicLoadNextNode(l)
		for /* horizontal */ nIdx != nil && ((!isDesc && key > nIdx.key) || (isDesc && key < nIdx.key)) {
			forward = nIdx
			nIdx = forward.atomicLoadNextNode(l)
		}

		if /* found */ nIdx != nil && key == nIdx.key {
			if atomicAreEqual(&nIdx.flags, nodeInsertedFlagBit|nodeRemovingFlagBit, insertFullyLinked) {
				if /* conc rw empty */ atomic.LoadInt64(&nIdx.count) <= 0 {
					return nil, ErrXSklConcRWLoadEmpty
				}
				switch mode {
				case unique:
					if x := nIdx.atomicLoadRoot(); x == nil {
						return nil, ErrXSklConcRWLoadEmpty
					} else {
						return &xSklElement[K, V]{
							key: key,
							val: *x.vptr,
						}, nil
					}
				case linkedList:
					if x := nIdx.atomicLoadRoot(); x == nil {
						return nil, ErrXSklConcRWLoadEmpty
					} else {
						return &xSklElement[K, V]{
							key: key,
							val: *x.parent.vptr,
						}, nil
					}
				case rbtree:
					if x := nIdx.root.minimum(); x == nil {
						return nil, ErrXSklConcRWLoadEmpty
					} else {
						return &xSklElement[K, V]{
							key: key,
							val: *x.vptr,
						}, nil
					}
				default:
					panic("[x-conc-skl] unknown x-node type")
				}
			}
			return nil, ErrXSklConcRWLoadFailed
		}
	}
	return nil, ErrXSklNotFound
}

// RemoveFirst deletes the val for a key, only the first value.
func (skl *xConcSkl[K, V]) RemoveFirst(key K) (element SklElement[K, V], err error) {
	if skl.Len() <= 0 {
		return nil, ErrXSklIsEmpty
	}

	var (
		aux      = make([]*xConcSklNode[K, V], 2*sklMaxLevel)
		rmNode   *xConcSklNode[K, V]
		isMarked bool // represents if this operation mark the node
		topLevel = int32(-1)
		ver      = skl.optVer.Number()
		foundAt  = int32(-1)
		isDesc   = isSet(skl.flags, xConcSklKeyCmpFlagBit)
	)

	switch mode := skl.loadXNodeMode(); mode {
	// FIXME: Merge these 2 deletion loops logic
	case unique:
		for {
			foundAt = skl.rmTraverse(key, isDesc, aux)
			if isMarked || foundAt != -1 &&
				atomicAreEqual(&aux[sklMaxLevel+foundAt].flags, nodeInsertedFlagBit|nodeRemovingFlagBit, insertFullyLinked) &&
				(int32(aux[sklMaxLevel+foundAt].level)-1) == foundAt {
				if !isMarked {
					rmNode = aux[sklMaxLevel+foundAt]
					topLevel = foundAt
					if !rmNode.tryLock(ver) {
						if /* d-check */ atomicIsSet(&rmNode.flags, nodeRemovingFlagBit) {
							return nil, ErrXSklConcRemoveTryLock
						}
						isMarked = false
						continue
					}

					if /* node locked, d-check */ atomicIsSet(&rmNode.flags, nodeRemovingFlagBit) {
						rmNode.unlock(ver)
						return nil, ErrXSklConcRemoving
					}

					atomicSet(&rmNode.flags, nodeRemovingFlagBit)
					isMarked = true
				}

				var (
					lockedLayers         = int32(-1)
					isValid              = true
					pred, succ, prevPred *xConcSklNode[K, V]
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
					isValid = !atomicIsSet(&pred.flags, nodeRemovingFlagBit) && pred.atomicLoadNextNode(l) == succ
				}
				if /* conc rm */ !isValid {
					unlockNodes(ver, lockedLayers, aux[0:sklMaxLevel]...)
					continue
				}

				element = &xSklElement[K, V]{
					key: key,
					val: *rmNode.atomicLoadRoot().vptr,
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
				unlockNodes(ver, lockedLayers, aux[0:sklMaxLevel]...)
				return element, nil
			}
			break
		}
	case linkedList, rbtree:
		for {
			foundAt = skl.rmTraverse(key, isDesc, aux)
			if isMarked || foundAt != -1 {
				fullyLinkedButNotRemove := atomicAreEqual(&aux[sklMaxLevel+foundAt].flags, nodeInsertedFlagBit|nodeRemovingFlagBit, insertFullyLinked)
				succMatch := (int32(aux[sklMaxLevel+foundAt].level) - 1) == foundAt
				if !succMatch {
					break
				} else if !fullyLinkedButNotRemove {
					continue
				}

				if fullyLinkedButNotRemove && !isMarked {
					rmNode = aux[sklMaxLevel+foundAt]
					topLevel = foundAt
					if !rmNode.tryLock(ver) {
						continue
					}

					if /* node locked */ !atomicIsSet(&rmNode.flags, nodeRemovingFlagBit) {
						atomicSet(&rmNode.flags, nodeRemovingFlagBit)
					}
					isMarked = true
				}

				var (
					lockedLayers     = int32(-1)
					isValid          = true
					pred, succ, prev *xConcSklNode[K, V]
				)
				for /* node locked */ l := int32(0); isValid && (l <= topLevel); l++ {
					pred, succ = aux[l], aux[sklMaxLevel+l]
					if /* lock indexCount */ pred != prev {
						pred.lock(ver)
						lockedLayers = l
						prev = pred
					}
					// Check:
					// 1. the previous node exists.
					// 2. no other nodes are inserted into the skip list in this layer.
					isValid = !atomicIsSet(&pred.flags, nodeRemovingFlagBit) && pred.atomicLoadNextNode(l) == succ
				}
				if /* conc rm */ !isValid {
					unlockNodes(ver, lockedLayers, aux[0:sklMaxLevel]...)
					continue
				}

				switch mode {
				case linkedList:
					if /* locked */ x := rmNode.root.linkedListNext(); x != nil {
						element = &xSklElement[K, V]{
							key: key,
							val: *x.vptr,
						}
						atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&rmNode.root.parent)), unsafe.Pointer(x.parent))
						atomic.AddInt64(&rmNode.count, -1)
						atomic.AddInt64(&skl.nodeLen, -1)
						atomicUnset(&rmNode.flags, nodeRemovingFlagBit)
					} else {
						atomic.StoreInt64(&rmNode.count, 0)
					}
				case rbtree:
					if /* locked */ x, _err := rmNode.rbRemoveMin(); _err == nil && x != nil {
						element = &xSklElement[K, V]{
							key: key,
							val: *x.vptr,
						}
						atomic.AddInt64(&skl.nodeLen, -1)
					}
					atomicUnset(&rmNode.flags, nodeRemovingFlagBit)
				}

				if atomic.LoadInt64(&rmNode.count) <= 0 {
					for /* re-linking, reduce levels */ l := topLevel; l >= 0; l-- {
						aux[l].atomicStoreNextNode(l, rmNode.loadNextNode(l))
					}
					atomic.AddUint64(&skl.indexCount, ^uint64(rmNode.level-1))
				}

				rmNode.unlock(ver)
				unlockNodes(ver, lockedLayers, aux[0:sklMaxLevel]...)
				return element, nil
			}
			break
		}
	default: /* impossible */
		panic("[x-conc-skl] unknown x-node type")
	}

	if foundAt == -1 {
		return nil, ErrXSklNotFound
	}
	return nil, ErrXSklUnknownReason
}

func (skl *xConcSkl[K, V]) PeekHead() (element SklElement[K, V]) {
	switch forward, mode := skl.atomicLoadHead().atomicLoadNextNode(0), skl.loadXNodeMode(); mode {
	case unique:
		for {
			if !atomicAreEqual(&forward.flags, nodeInsertedFlagBit|nodeRemovingFlagBit, insertFullyLinked) {
				forward = forward.atomicLoadNextNode(0)
				continue
			}
			node := forward.atomicLoadRoot()
			if node == nil {
				return nil
			}
			element = &xSklElement[K, V]{
				key: forward.key,
				val: *node.vptr,
			}
			break
		}
	case linkedList:
		for {
			if !atomicAreEqual(&forward.flags, nodeInsertedFlagBit|nodeRemovingFlagBit, insertFullyLinked) {
				forward = forward.atomicLoadNextNode(0)
				continue
			}
			x := forward.atomicLoadRoot().parent
			if x == nil {
				return nil
			}
			element = &xSklElement[K, V]{
				key: forward.key,
				val: *x.vptr,
			}
			break
		}
	case rbtree:
		for {
			if !atomicAreEqual(&forward.flags, nodeInsertedFlagBit|nodeRemovingFlagBit, insertFullyLinked) {
				forward = forward.atomicLoadNextNode(0)
				continue
			}
			x := forward.root.minimum()
			if x == nil {
				return nil
			}
			element = &xSklElement[K, V]{
				key: forward.key,
				val: *x.vptr,
			}
			break
		}
	default:
		panic("[x-conc-skl] unknown node type")
	}
	return element
}

func (skl *xConcSkl[K, V]) PopHead() (element SklElement[K, V], err error) {
	forward := skl.atomicLoadHead().atomicLoadNextNode(0)
	if forward == nil {
		return nil, ErrXSklIsEmpty
	}
	return skl.RemoveFirst(forward.key)
}

// Duplicated element Skip-List basic APIs

func (skl *xConcSkl[K, V]) LoadIfMatch(key K, matcher func(that V) bool) ([]SklElement[K, V], error) {
	if skl.Len() <= 0 {
		return nil, ErrXSklIsEmpty
	}

	var (
		forward  = skl.atomicLoadHead()
		mode     = skl.loadXNodeMode()
		elements = make([]SklElement[K, V], 0, 32)
		isDesc   = isSet(skl.flags, xConcSklKeyCmpFlagBit)
	)
	for /* vertical */ l := skl.Levels() - 1; l >= 0; l-- {
		nIdx := forward.atomicLoadNextNode(l)
		for /* horizontal */ nIdx != nil && ((!isDesc && key > nIdx.key) || (isDesc && key < nIdx.key)) {
			forward = nIdx
			nIdx = forward.atomicLoadNextNode(l)
		}

		if /* found */ nIdx != nil && key == nIdx.key {
			if atomicAreEqual(&nIdx.flags, nodeInsertedFlagBit|nodeRemovingFlagBit, insertFullyLinked) {
				if /* conc rw */ atomic.LoadInt64(&nIdx.count) <= 0 {
					return nil, ErrXSklConcRWLoadEmpty
				}
				switch mode {
				case unique:
					panic("[x-conc-skl] unique mode skip-list not implements the load if match method")
				case linkedList:
					for x := nIdx.atomicLoadRoot().parent.linkedListNext(); x != nil; x = x.linkedListNext() {
						v := *x.vptr
						if matcher(v) {
							elements = append(elements, &xSklElement[K, V]{
								key: key,
								val: v,
							})
						}
					}
					return elements, nil
				case rbtree:
					nIdx.rbDFS(func(idx int64, color color, v V) bool {
						if matcher(v) {
							elements = append(elements, &xSklElement[K, V]{
								key: key,
								val: v,
							})
						}
						return true
					})
					return elements, nil
				default:
					panic("[x-conc-skl] unknown x-node type")
				}
			}
			return nil, ErrXSklConcRWLoadFailed
		}
	}
	return nil, ErrXSklNotFound
}

func (skl *xConcSkl[K, V]) LoadAll(key K) ([]SklElement[K, V], error) {
	if skl.Len() <= 0 {
		return nil, ErrXSklIsEmpty
	}

	var (
		forward  = skl.atomicLoadHead()
		mode     = skl.loadXNodeMode()
		elements = make([]SklElement[K, V], 0, 32)
		isDesc   = isSet(skl.flags, xConcSklKeyCmpFlagBit)
	)
	for /* vertical */ l := skl.Levels() - 1; l >= 0; l-- {
		nIdx := forward.atomicLoadNextNode(l)
		for /* horizontal */ nIdx != nil && ((!isDesc && key > nIdx.key) || (isDesc && key < nIdx.key)) {
			forward = nIdx
			nIdx = forward.atomicLoadNextNode(l)
		}

		if /* found */ nIdx != nil && key == nIdx.key {
			if atomicAreEqual(&nIdx.flags, nodeInsertedFlagBit|nodeRemovingFlagBit, insertFullyLinked) {
				if /* conc rw */ atomic.LoadInt64(&nIdx.count) <= 0 {
					return nil, ErrXSklConcRWLoadEmpty
				}
				switch mode {
				case unique:
					panic("[x-conc-skl] unique mode skip-list not implements the load all method")
				case linkedList:
					for x := nIdx.atomicLoadRoot().parent.linkedListNext(); x != nil; x = x.linkedListNext() {
						elements = append(elements, &xSklElement[K, V]{
							key: key,
							val: *x.vptr,
						})
					}
					return elements, nil
				case rbtree:
					nIdx.rbDFS(func(idx int64, color color, v V) bool {
						elements = append(elements, &xSklElement[K, V]{
							key: key,
							val: v,
						})
						return true
					})
					return elements, nil
				default:
					panic("[x-conc-skl] unknown x-node type")
				}
			}
			return nil, ErrXSklConcRWLoadFailed
		}
	}
	return nil, ErrXSklNotFound
}

func (skl *xConcSkl[K, V]) RemoveIfMatch(key K, matcher func(that V) bool) ([]SklElement[K, V], error) {
	if skl.Len() <= 0 {
		return nil, ErrXSklIsEmpty
	}

	var (
		aux      = make([]*xConcSklNode[K, V], 2*sklMaxLevel)
		rmNode   *xConcSklNode[K, V]
		isMarked bool // represents if this operation mark the node
		topLevel = int32(-1)
		ver      = skl.optVer.Number()
		foundAt  = int32(-1)
		elements = make([]SklElement[K, V], 0, 32)
		isDesc   = isSet(skl.flags, xConcSklKeyCmpFlagBit)
	)

	switch mode := skl.loadXNodeMode(); mode {
	// FIXME: Merge these 2 deletion loops logic
	case unique:
		panic("[x-conc-skl] unique mode skip-list not implements the remove if match method")
	case linkedList, rbtree:
		for {
			foundAt = skl.rmTraverse(key, isDesc, aux)
			if isMarked || foundAt != -1 {
				fullyLinkedButNotRemove := atomicAreEqual(&aux[sklMaxLevel+foundAt].flags, nodeInsertedFlagBit|nodeRemovingFlagBit, insertFullyLinked)
				succMatch := (int32(aux[sklMaxLevel+foundAt].level) - 1) == foundAt
				if !succMatch {
					break
				} else if !fullyLinkedButNotRemove {
					continue
				}

				if fullyLinkedButNotRemove && !isMarked {
					rmNode = aux[sklMaxLevel+foundAt]
					topLevel = foundAt
					if !rmNode.tryLock(ver) {
						continue
					}

					if /* node locked */ !atomicIsSet(&rmNode.flags, nodeRemovingFlagBit) {
						atomicSet(&rmNode.flags, nodeRemovingFlagBit)
					}
					isMarked = true
				}

				var (
					lockedLayers     = int32(-1)
					isValid          = true
					pred, succ, prev *xConcSklNode[K, V]
				)
				for /* node locked */ l := int32(0); isValid && (l <= topLevel); l++ {
					pred, succ = aux[l], aux[sklMaxLevel+l]
					if /* lock indexCount */ pred != prev {
						pred.lock(ver)
						lockedLayers = l
						prev = pred
					}
					// Check:
					// 1. the previous node exists.
					// 2. no other nodes are inserted into the skip list in this layer.
					isValid = !atomicIsSet(&pred.flags, nodeRemovingFlagBit) && pred.atomicLoadNextNode(l) == succ
				}
				if /* conc rm */ !isValid {
					unlockNodes(ver, lockedLayers, aux[0:sklMaxLevel]...)
					continue
				}

				switch mode {
				case linkedList:
					if x := rmNode.root.linkedListNext(); x == nil {
						atomic.AddInt64(&rmNode.count, 0)
					} else {
						first, prev := x, x
						for ; /* locked */ x != nil; x = x.linkedListNext() {
							if matcher(*x.vptr) {
								if x == first {
									first = x.linkedListNext()
									atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&rmNode.root.parent)), unsafe.Pointer(first))
								} else {
									prev.parent = x.linkedListNext()
								}
								elements = append(elements, &xSklElement[K, V]{
									key: key,
									val: *x.vptr,
								})
								atomic.AddInt64(&rmNode.count, -1)
								atomic.AddInt64(&skl.nodeLen, -1)
							} else {
								prev = x
							}
						}
						atomicUnset(&rmNode.flags, nodeRemovingFlagBit)
					}
				case rbtree:
					// TODO fix bad efficiency
					rmNode.rbDFS( /* locked */ func(idx int64, color color, v V) bool {
						if matcher(v) {
							elements = append(elements, &xSklElement[K, V]{
								key: key,
								val: v,
							})
						}
						return true
					})
					for _, e := range elements {
						if _, err := rmNode.rbRemove(e.Val(), skl.vcmp); err == nil {
							atomic.AddInt64(&rmNode.count, -1)
							atomic.AddInt64(&skl.nodeLen, -1)
						}
					}
					atomicUnset(&rmNode.flags, nodeRemovingFlagBit)
				}

				if atomic.LoadInt64(&rmNode.count) <= 0 {
					for /* re-linking, reduce levels */ l := topLevel; l >= 0; l-- {
						aux[l].atomicStoreNextNode(l, rmNode.loadNextNode(l))
					}
					atomic.AddUint64(&skl.indexCount, ^uint64(rmNode.level-1))
				}

				rmNode.unlock(ver)
				unlockNodes(ver, lockedLayers, aux[0:sklMaxLevel]...)
				return elements, nil
			}
			break
		}
	default:
		panic("[x-conc-skl] unknown x-node type")
	}

	if foundAt == -1 {
		return nil, ErrXSklNotFound
	}
	return nil, ErrXSklUnknownReason
}

func (skl *xConcSkl[K, V]) RemoveAll(key K) ([]SklElement[K, V], error) {
	if skl.Len() <= 0 {
		return nil, ErrXSklIsEmpty
	}

	var (
		aux      = make([]*xConcSklNode[K, V], 2*sklMaxLevel)
		rmNode   *xConcSklNode[K, V]
		isMarked bool // represents if this operation mark the node
		topLevel = int32(-1)
		ver      = skl.optVer.Number()
		foundAt  = int32(-1)
		elements = make([]SklElement[K, V], 0, 32)
		isDesc   = isSet(skl.flags, xConcSklKeyCmpFlagBit)
	)

	switch mode := skl.loadXNodeMode(); mode {
	// FIXME: Merge these 2 deletion loops logic
	case unique:
		panic("[x-conc-skl] unique mode skip-list not implements the remove all method")
	case linkedList, rbtree:
		for {
			foundAt = skl.rmTraverse(key, isDesc, aux)
			if isMarked || foundAt != -1 {
				fullyLinkedButNotRemove := atomicAreEqual(&aux[sklMaxLevel+foundAt].flags, nodeInsertedFlagBit|nodeRemovingFlagBit, insertFullyLinked)
				succMatch := (int32(aux[sklMaxLevel+foundAt].level) - 1) == foundAt
				if !succMatch {
					break
				} else if !fullyLinkedButNotRemove {
					continue
				}

				if fullyLinkedButNotRemove && !isMarked {
					rmNode = aux[sklMaxLevel+foundAt]
					topLevel = foundAt
					if !rmNode.tryLock(ver) {
						continue
					}

					if /* node locked */ !atomicIsSet(&rmNode.flags, nodeRemovingFlagBit) {
						atomicSet(&rmNode.flags, nodeRemovingFlagBit)
					}
					isMarked = true
				}

				var (
					lockedLayers     = int32(-1)
					isValid          = true
					pred, succ, prev *xConcSklNode[K, V]
				)
				for /* node locked */ l := int32(0); isValid && (l <= topLevel); l++ {
					pred, succ = aux[l], aux[sklMaxLevel+l]
					if /* lock indexCount */ pred != prev {
						pred.lock(ver)
						lockedLayers = l
						prev = pred
					}
					// Check:
					// 1. the previous node exists.
					// 2. no other nodes are inserted into the skip list in this layer.
					isValid = !atomicIsSet(&pred.flags, nodeRemovingFlagBit) && pred.atomicLoadNextNode(l) == succ
				}
				if /* conc rm */ !isValid {
					unlockNodes(ver, lockedLayers, aux[0:sklMaxLevel]...)
					continue
				}

				switch mode {
				case linkedList:
					if x := rmNode.root.linkedListNext(); x == nil {
						atomic.AddInt64(&rmNode.count, 0)
					} else {
						atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&rmNode.root.parent)), unsafe.Pointer(nil))
						for /* locked */ x != nil {
							elements = append(elements, &xSklElement[K, V]{
								key: key,
								val: *x.vptr,
							})
							prev := x
							x = x.linkedListNext()
							prev.parent = nil
						}
						atomic.StoreInt64(&rmNode.count, 0)
						atomic.AddInt64(&skl.nodeLen, -atomic.LoadInt64(&rmNode.count))
					}
				case rbtree:
					rmNode.rbDFS( /* locked */ func(idx int64, color color, v V) bool {
						elements = append(elements, &xSklElement[K, V]{
							key: key,
							val: v,
						})
						return true
					})
					rmNode.rbRelease()
					atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&rmNode.root)), unsafe.Pointer(nil))
					atomic.StoreInt64(&rmNode.count, 0)
					atomic.AddInt64(&skl.nodeLen, -atomic.LoadInt64(&rmNode.count))
				}

				if atomic.LoadInt64(&rmNode.count) <= 0 {
					for /* re-linking, reduce levels */ l := topLevel; l >= 0; l-- {
						aux[l].atomicStoreNextNode(l, rmNode.loadNextNode(l))
					}
					atomic.AddUint64(&skl.indexCount, ^uint64(rmNode.level-1))
				}

				rmNode.unlock(ver)
				unlockNodes(ver, lockedLayers, aux[0:sklMaxLevel]...)
				return elements, nil
			}
			break
		}
	default:
		panic("[x-conc-skl] unknown x-node type")
	}

	if foundAt == -1 {
		return nil, ErrXSklNotFound
	}
	return nil, ErrXSklUnknownReason
}
