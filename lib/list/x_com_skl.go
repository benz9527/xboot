package list

import (
	"sync"
	"sync/atomic"

	"github.com/benz9527/xboot/lib/infra"
)

var (
	_ XSkipList[uint8, uint8] = (*xComSkl[uint8, uint8])(nil)
)

// A common implementation of skip-list.
// @field head A sentinel node.
// The head.indices[0].succ is the first data node of skip-list.
// From head.indices[1], all of them are cache used to implement binary search.
// @field tail A sentinel node. Points the skip-list tail node.
type xComSkl[K infra.OrderedKey, V comparable] struct {
	kcmp       infra.OrderedKeyComparator[K]
	vcmp       SklValComparator[V]
	rand       SklRand
	pool       *sync.Pool
	head       *xComSklNode[K, V]
	tail       *xComSklNode[K, V]
	nodeLen    int64  // skip-list's node count.
	indexCount uint64 // skip-list's index count.
	levels     int32  // skip-list's max height value inside the indexCount.
}

// loadAux is to load auxiliary array for traversal.
func (skl *xComSkl[K, V]) loadAux() []*xComSklNode[K, V] {
	aux, ok := skl.pool.Get().([]*xComSklNode[K, V])
	if /* dead code */ !ok {
		panic("[x-com-skl] load unknown traverse elements from pool")
	}
	return aux
}

// putAux is to recycle auxiliary array after traversal.
func (skl *xComSkl[K, V]) putAux(aux []*xComSklNode[K, V]) {
	for i := 0; i < sklMaxLevel; i++ {
		aux[i] = nil
	}
	skl.pool.Put(aux)
}

// findPredecessor0 is used to find the (succ) first element whose key value equals to target key value.
// Preparing for linear probing. O(N)
// @return value 1: the pred node
// @return value 2: the query traverse path (nodes)
func (skl *xComSkl[K, V]) findPredecessor0(key K) (*xComSklNode[K, V], []*xComSklNode[K, V]) {
	var (
		forward *xComSklNode[K, V]
		aux     = skl.loadAux()
	)
	forward = skl.head
	for /* vertical */ i := skl.Levels() - 1; i >= 0; i-- {
		for /* horizontal */ forward.levels()[i].forward() != nil {
			cur := forward.levels()[i].forward()
			if /* greater, forward next */ cur != nil && skl.kcmp(key, cur.Element().Key()) > 0 {
				// Linear probing (forward next) at level 0 most likely.
				forward = cur
			} else /* lower or equal, downward to next level */ {
				break
			}
		}
		aux[i] = forward
	}

	if /* not found */ forward == nil {
		return nil, aux
	}

	target := forward.levels()[0].forward()
	if /* found */ target != nil && skl.kcmp(key, target.Element().Key()) == 0 {
		return forward, aux
	}
	return /* not found */ nil, aux
}

// removeNode will reduce the levels.
func (skl *xComSkl[K, V]) removeNode(x *xComSklNode[K, V], aux []*xComSklNode[K, V]) {
	for i := int32(0); i < skl.Levels(); i++ {
		if aux[i].levels()[i].forward() == x {
			aux[i].levels()[i].setForward(x.levels()[i].forward())
		}
	}
	if /* unlink */ next := x.levels()[0].forward(); next != nil {
		next.setBackward(x.backward())
	} else {
		skl.tail = x.backward()
	}
	atomic.AddUint64(&skl.indexCount, ^uint64(len(x.indices)-1))
	for /* reduce levels */ skl.Levels() > 1 && skl.head.levels()[skl.Levels()-1].forward() == nil {
		atomic.AddInt32(&skl.levels, -1)
	}
	atomic.AddInt64(&skl.nodeLen, -1)
}

// Classic Skip-List basic APIs

func (skl *xComSkl[K, V]) Len() int64 {
	return atomic.LoadInt64(&skl.nodeLen)
}

func (skl *xComSkl[K, V]) IndexCount() uint64 {
	return atomic.LoadUint64(&skl.indexCount)
}

func (skl *xComSkl[K, V]) Levels() int32 {
	return atomic.LoadInt32(&skl.levels)
}

func (skl *xComSkl[K, V]) Insert(key K, val V, ifNotPresent ...bool) error {
	if skl.Len() >= sklMaxSize {
		return ErrXSklIsFull
	}

	var (
		pred = skl.head
		aux  = skl.loadAux()
	)
	defer func() {
		skl.putAux(aux)
	}()

	if len(ifNotPresent) <= 0 {
		ifNotPresent = insertReplaceDisabled
	}

	for /* vertical */ i := atomic.LoadInt32(&skl.levels) - 1; i >= 0; i-- {
		for /* horizontal */ pred.levels()[i].forward() != nil {
			cur := pred.levels()[i].forward()
			res := skl.kcmp(key, cur.Element().Key())
			if /* next insert position */ res > 0 || (res == 0 && skl.vcmp(val, cur.Element().Val()) > 0) {
				pred = cur
			} else /* replace */ if res == 0 && skl.vcmp(val, cur.Element().Val()) == 0 {
				if /* disabled */ ifNotPresent[0] {
					return ErrXSklDisabledValReplace
				}
				cur.element = &xSklElement[K, V]{
					key: key,
					val: val,
				}
				return nil
			} else {
				break
			}
		}
		// 1. (key duplicated) If new element hash is lower than current node's (do pre-append to current node)
		// 2. (key duplicated) If new element hash is greater than current node's (do append next to current node)
		// 3. (key duplicated) If new element hash equals to current node's (replace an element, because the hash
		//      value and element are not strongly correlated)
		// 4. (new key) If a new element does not exist, (do append next to the current node)
		aux[i] = pred
	}

	// Each duplicated key element may contain its cache levels.
	// It means that duplicated key elements query through the cache (O(logN))
	// But duplicated elements query (linear probe) will be degraded into O(N)
	lvl := skl.rand(sklMaxLevel, skl.Len())
	if lvl > skl.Levels() {
		for i := skl.Levels(); i < lvl; i++ {
			// Update the whole traverse path, from top to bottom.
			aux[i] = skl.head // avoid nil pointer
		}
		atomic.StoreInt32(&skl.levels, lvl)
	}
	atomic.AddUint64(&skl.indexCount, uint64(lvl))

	newNode := newXComSklNode[K, V](lvl, key, val)
	for i := int32(0); i < lvl; i++ {
		next := aux[i].levels()[i].forward()
		newNode.levels()[i].setForward(next)
		aux[i].levels()[i].setForward(newNode)
	}
	if aux[0] == skl.head {
		newNode.setBackward(nil)
	} else {
		newNode.setBackward(aux[0])
	}
	if newNode.levels()[0].forward() == nil {
		skl.tail = newNode
	} else {
		newNode.levels()[0].forward().setBackward(newNode)
	}
	atomic.AddInt64(&skl.nodeLen, 1)
	return nil
}

func (skl *xComSkl[K, V]) LoadFirst(key K) (SklElement[K, V], error) {
	if skl.Len() <= 0 {
		return nil, ErrXSklIsEmpty
	}

	e, traverse := skl.findPredecessor0(key)
	defer func() {
		skl.putAux(traverse)
	}()

	if e.levels() == nil {
		return nil, ErrXSklNotFound
	}
	return e.levels()[0].forward().Element(), nil
}

func (skl *xComSkl[K, V]) RemoveFirst(key K) (SklElement[K, V], error) {
	if skl.Len() <= 0 {
		return nil, ErrXSklIsEmpty
	}

	pred, aux := skl.findPredecessor0(key)
	defer func() {
		skl.putAux(aux)
	}()
	if pred == nil {
		return nil, ErrXSklNotFound
	}

	target := pred.levels()[0].forward()
	if target != nil && skl.kcmp(key, target.Element().Key()) == 0 {
		skl.removeNode(target, aux)
		return target.Element(), nil
	}
	return nil, ErrXSklNotFound
}

func (skl *xComSkl[K, V]) Foreach(action func(i int64, item SklIterationItem[K, V]) bool) {
	if skl.Len() <= 0 {
		return
	}

	var (
		x    *xComSklNode[K, V]
		i    int64
		item = &xSklIter[K, V]{}
	)
	x = skl.head.levels()[0].forward()
	for x != nil {
		next := x.levels()[0].forward()
		item.keyFn = x.element.Key
		item.valFn = x.element.Val
		item.nodeLevelFn = func() uint32 {
			return uint32(len(x.levels()))
		}
		item.nodeItemCountFn = func() int64 {
			return 1
		}
		if !action(i, item) {
			break
		}
		i++
		x = next
	}
}

func (skl *xComSkl[K, V]) PeekHead() SklElement[K, V] {
	target := skl.head
	if target == nil || skl.Len() <= 0 {
		return nil
	}
	if target = target.levels()[0].forward(); target == nil {
		return nil
	}
	return target.Element()
}

func (skl *xComSkl[K, V]) PopHead() (element SklElement[K, V], err error) {
	target := skl.head
	if skl.Len() <= 0 || target == nil {
		return nil, ErrXSklIsEmpty
	}
	if target = target.levels()[0].forward(); target == nil {
		return nil, ErrXSklIsEmpty
	}
	element = target.Element()
	return skl.RemoveFirst(element.Key())
}

// Duplicated element Skip-List basic APIs

func (skl *xComSkl[K, V]) LoadIfMatched(key K, matcher func(that V) bool) ([]SklElement[K, V], error) {
	if skl.Len() <= 0 {
		return nil, ErrXSklIsEmpty
	}

	pred, aux := skl.findPredecessor0(key)
	defer func() {
		skl.putAux(aux)
	}()
	if pred == nil {
		return nil, ErrXSklNotFound
	}

	elements := make([]SklElement[K, V], 0, 16)
	for cur := pred.levels()[0].forward(); cur != nil && skl.kcmp(key, cur.Element().Key()) == 0; cur = cur.levels()[0].forward() {
		if matcher(cur.Element().Val()) {
			elements = append(elements, cur.Element())
		}
	}
	return elements, nil
}

func (skl *xComSkl[K, V]) LoadAll(key K) ([]SklElement[K, V], error) {
	if skl.Len() <= 0 {
		return nil, ErrXSklIsEmpty
	}

	pred, aux := skl.findPredecessor0(key)
	defer func() {
		skl.putAux(aux)
	}()
	if pred == nil {
		return nil, ErrXSklNotFound
	}

	elements := make([]SklElement[K, V], 0, 16)
	for cur := pred.levels()[0].forward(); cur != nil && skl.kcmp(key, cur.Element().Key()) == 0; cur = cur.levels()[0].forward() {
		elements = append(elements, cur.Element())
	}
	return elements, nil
}

func (skl *xComSkl[K, V]) RemoveIfMatched(key K, matcher func(that V) bool) ([]SklElement[K, V], error) {
	if skl.Len() <= 0 {
		return nil, ErrXSklIsEmpty
	}

	pred, aux := skl.findPredecessor0(key)
	defer func() {
		skl.putAux(aux)
	}()
	if pred == nil {
		return nil, ErrXSklNotFound
	}

	elements := make([]SklElement[K, V], 0, 16)
	for cur := pred.levels()[0].forward(); cur != nil && skl.kcmp(key, cur.Element().Key()) == 0; {
		if matcher(cur.Element().Val()) {
			skl.removeNode(cur, aux)
			elements = append(elements, cur.Element())
			next := cur.levels()[0].forward()
			cur.Free()
			cur = next
		} else {
			// Merge the traverse path.
			for i := 0; i < len(cur.levels()); i++ {
				aux[i] = cur
			}
			cur = cur.levels()[0].forward()
		}
	}
	return elements, nil
}

func (skl *xComSkl[K, V]) RemoveAll(key K) ([]SklElement[K, V], error) {
	if skl.Len() <= 0 {
		return nil, ErrXSklIsEmpty
	}

	pred, aux := skl.findPredecessor0(key)
	defer func() {
		skl.putAux(aux)
	}()
	if pred == nil {
		return nil, ErrXSklNotFound
	}

	elements := make([]SklElement[K, V], 0, 16)
	for cur := pred.levels()[0].forward(); cur != nil && skl.kcmp(key, cur.Element().Key()) == 0; {
		skl.removeNode(cur, aux)
		elements = append(elements, cur.Element())
		free := cur
		cur = cur.levels()[0].forward()
		free.Free()
	}
	return elements, nil
}

func (skl *xComSkl[K, V]) Release() {
	var (
		x, next *xComSklNode[K, V]
		idx     int
	)
	x = skl.head.levels()[0].forward()
	for x != nil {
		next = x.levels()[0].forward()
		x.Free()
		x = next
	}
	for idx = 0; idx < sklMaxLevel; idx++ {
		skl.head.levels()[idx].setForward(nil)
	}
	skl.tail = nil
	skl.pool = nil
}
