package list

import (
	"errors"
	"sync"
	"sync/atomic"

	"github.com/benz9527/xboot/lib/infra"
)

const (
	xSkipListMaxLevel    = 32   // 2^32 - 1 elements
	xSkipListProbability = 0.25 // P = 1/4, a skip list node element has 1/4 probability to have a level
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

func (skl *xComSkl[K, V]) loadTraverse() []*xComSklNode[K, V] {
	traverse, ok := skl.pool.Get().([]*xComSklNode[K, V])
	if !ok {
		panic("[x-com-skl] load unknown traverse elements from pool")
	}
	return traverse
}

func (skl *xComSkl[K, V]) putTraverse(traverse []*xComSklNode[K, V]) {
	for i := 0; i < xSkipListMaxLevel; i++ {
		traverse[i] = nil
	}
	skl.pool.Put(traverse)
}

func (skl *xComSkl[K, V]) Len() int64 {
	return atomic.LoadInt64(&skl.nodeLen)
}

func (skl *xComSkl[K, V]) IndexCount() uint64 {
	return atomic.LoadUint64(&skl.indexCount)
}

func (skl *xComSkl[K, V]) Levels() int32 {
	return atomic.LoadInt32(&skl.levels)
}

// findPredecessor0 is used to find the (succ) first element whose key equals to target value.
// Preparing for linear probing. V(N)
// @return value 1: the pred node
// @return value 2: the query traverse path (nodes)
func (skl *xComSkl[K, V]) findPredecessor0(key K) (*xComSklNode[K, V], []*xComSklNode[K, V]) {
	var (
		predecessor *xComSklNode[K, V]
		traverse    = skl.loadTraverse()
	)
	predecessor = skl.head
	for i := skl.Levels() - 1; i >= 0; i-- {
		// Note: Will start probing linearly from a local position in some interval
		// V(N)
		for predecessor.levels()[i].forward() != nil {
			cur := predecessor.levels()[i].forward()
			res := skl.kcmp(key, cur.Element().Key())
			// find pred node
			if res > 0 {
				predecessor = cur
			} else {
				// downward to the next level, continue to find
				break
			}
		}
		traverse[i] = predecessor
	}

	if predecessor == nil {
		return nil, traverse // not found
	}

	target := predecessor.levels()[0].forward()
	// Check next element's key
	if target != nil && skl.kcmp(key, target.Element().Key()) == 0 {
		return predecessor, traverse
	}
	return nil, traverse // not found
}

func (skl *xComSkl[K, V]) removeNode(x *xComSklNode[K, V], traverse []*xComSklNode[K, V]) {
	for i := int32(0); i < skl.Levels(); i++ {
		if traverse[i].levels()[i].forward() == x {
			traverse[i].levels()[i].setForward(x.levels()[i].forward())
		}
	}
	if next := x.levels()[0].forward(); next != nil {
		// Adjust the pred.
		next.setBackward(x.backward())
	} else {
		skl.tail = x.backward()
	}
	for skl.Levels() > 1 && skl.head.levels()[skl.Levels()-1].forward() == nil {
		atomic.AddInt32(&skl.levels, -1)
	}
	atomic.AddInt64(&skl.nodeLen, -1)
}

// Classic Skip-List basic APIs

func (skl *xComSkl[K, V]) Insert(key K, val V) error {
	var (
		predecessor = skl.head
		traverse    = skl.loadTraverse()
	)
	defer func() {
		skl.putTraverse(traverse)
	}()

	// Iteration from top to bottom.
	// First to iterate the cache and access the data finally.
	for i := atomic.LoadInt32(&skl.levels) - 1; i >= 0; i-- { // move down level
		for predecessor.levels()[i].forward() != nil {
			cur := predecessor.levels()[i].forward()
			res := skl.kcmp(cur.Element().Key(), key)
			if res < 0 || (res == 0 && skl.vcmp(val, cur.Element().Val()) > 0) {
				predecessor = cur // Changes the node iteration path to locate different node.
			} else if res == 0 && skl.vcmp(val, cur.Element().Val()) == 0 {
				return errors.New("")
			} else {
				break
			}
		}
		// 1. (key duplicated) If new element hash is lower than current node's (do pre-append to current node)
		// 2. (key duplicated) If new element hash is greater than current node's (do append next to current node)
		// 3. (key duplicated) If new element hash equals to current node's (replace an element, because the hash
		//      value and element are not strongly correlated)
		// 4. (new key) If a new element does not exist, (do append next to the current node)
		traverse[i] = predecessor
	}

	// Each duplicated key element may contain its cache levels.
	// It means that duplicated key elements query through the cache (V(logN))
	// But duplicated elements query (linear probe) will be degraded into V(N)
	lvl := skl.rand(xSkipListMaxLevel, skl.Len())
	if lvl > skl.Levels() {
		for i := skl.Levels(); i < lvl; i++ {
			// Update the whole traverse path, from top to bottom.
			traverse[i] = skl.head // avoid nil pointer
		}
		atomic.StoreInt32(&skl.levels, lvl)
	}

	newNode := newXComSklNode[K, V](lvl, key, val)
	// Insert new node and update the new node levels metadata.
	for i := int32(0); i < lvl; i++ {
		next := traverse[i].levels()[i].forward()
		newNode.levels()[i].setForward(next)
		// May pre-append to adjust 2 elements' order
		traverse[i].levels()[i].setForward(newNode)
	}
	if traverse[0] == skl.head {
		newNode.setBackward(nil)
	} else {
		newNode.setBackward(traverse[0])
	}
	if newNode.levels()[0].forward() == nil {
		skl.tail = newNode
	} else {
		newNode.levels()[0].forward().setBackward(newNode)
	}
	atomic.AddInt64(&skl.nodeLen, 1)
	return nil
}

func (skl *xComSkl[K, V]) LoadFirst(key K) (SkipListElement[K, V], error) {
	e, traverse := skl.findPredecessor0(key)
	defer func() {
		skl.putTraverse(traverse)
	}()
	if e.levels() == nil {
		return nil, errors.New("")
	}

	return e.levels()[0].forward().Element(), nil
}

func (skl *xComSkl[K, V]) RemoveFirst(key K) (SkipListElement[K, V], error) {
	predecessor, traverse := skl.findPredecessor0(key)
	defer func() {
		skl.putTraverse(traverse)
	}()
	if predecessor == nil {
		return nil, errors.New("not found")
	}

	target := predecessor.levels()[0].forward()
	if target != nil && skl.kcmp(key, target.Element().Key()) == 0 {
		skl.removeNode(target, traverse)
		return target.Element(), nil
	}
	return nil, errors.New("not found")
}

func (skl *xComSkl[K, V]) Foreach(action func(i int64, item SkipListIterationItem[K, V]) bool) {
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
		if !action(i, item) {
			break
		}
		i++
		x = next
	}
}

func (skl *xComSkl[K, V]) PeekHead() (element SkipListElement[K, V]) {
	target := skl.head
	if target == nil || skl.Len() <= 0 {
		return nil
	}
	if target = target.levels()[0].forward(); target == nil {
		return nil
	}
	return target.Element()
}

func (skl *xComSkl[K, V]) PopHead() (element SkipListElement[K, V], err error) {
	target := skl.head
	if target == nil || skl.Len() <= 0 {
		return nil, errors.New("")
	}
	if target = target.levels()[0].forward(); target == nil {
		return nil, errors.New("")
	}
	element = target.Element()
	return skl.RemoveFirst(element.Key())
}

// Duplicated element Skip-List basic APIs

func (skl *xComSkl[K, V]) LoadIfMatched(key K, matcher func(that V) bool) ([]SkipListElement[K, V], error) {
	predecessor, traverse := skl.findPredecessor0(key)
	defer func() {
		skl.putTraverse(traverse)
	}()
	if predecessor == nil {
		return nil, errors.New("not found")
	}

	elements := make([]SkipListElement[K, V], 0, 16)
	for cur := predecessor.levels()[0].forward(); cur != nil && skl.kcmp(key, cur.Element().Key()) == 0; cur = cur.levels()[0].forward() {
		if matcher(cur.Element().Val()) {
			elements = append(elements, cur.Element())
		}
	}
	return elements, nil
}

func (skl *xComSkl[K, V]) LoadAll(key K) ([]SkipListElement[K, V], error) {
	predecessor, traverse := skl.findPredecessor0(key)
	defer func() {
		skl.putTraverse(traverse)
	}()
	if predecessor == nil {
		return nil, errors.New("not found")
	}

	elements := make([]SkipListElement[K, V], 0, 16)
	for cur := predecessor.levels()[0].forward(); cur != nil && skl.kcmp(key, cur.Element().Key()) == 0; cur = cur.levels()[0].forward() {
		elements = append(elements, cur.Element())
	}
	return elements, nil
}

func (skl *xComSkl[K, V]) RemoveIfMatched(key K, matcher func(that V) bool) ([]SkipListElement[K, V], error) {
	predecessor, traverse := skl.findPredecessor0(key)
	defer func() {
		skl.putTraverse(traverse)
	}()
	if predecessor == nil {
		return nil, errors.New("not found")
	}

	elements := make([]SkipListElement[K, V], 0, 16)
	for cur := predecessor.levels()[0].forward(); cur != nil && skl.kcmp(key, cur.Element().Key()) == 0; {
		if matcher(cur.Element().Val()) {
			skl.removeNode(cur, traverse)
			elements = append(elements, cur.Element())
			next := cur.levels()[0].forward()
			cur.Free()
			cur = next
		} else {
			// Merge the traverse path.
			for i := 0; i < len(cur.levels()); i++ {
				traverse[i] = cur
			}
			cur = cur.levels()[0].forward()
		}
	}
	return elements, nil
}

func (skl *xComSkl[K, V]) RemoveAll(key K) ([]SkipListElement[K, V], error) {
	predecessor, traverse := skl.findPredecessor0(key)
	defer func() {
		skl.putTraverse(traverse)
	}()
	if predecessor == nil {
		return nil, errors.New("not found")
	}

	elements := make([]SkipListElement[K, V], 0, 16)
	for cur := predecessor.levels()[0].forward(); cur != nil && skl.kcmp(key, cur.Element().Key()) == 0; {
		skl.removeNode(cur, traverse)
		elements = append(elements, cur.Element())
		free := cur
		cur = cur.levels()[0].forward()
		free.Free()
	}
	return elements, nil
}

func (skl *xComSkl[K, V]) Free() {
	var (
		x, next *xComSklNode[K, V]
		idx     int
	)
	x = skl.head.levels()[0].forward()
	for x != nil {
		next = x.levels()[0].forward()
		x.Free()
		x = nil
		x = next
	}
	for idx = 0; idx < xSkipListMaxLevel; idx++ {
		skl.head.levels()[idx].setForward(nil)
	}
	skl.tail = nil
	skl.pool = nil
}

func newXComSkl[K infra.OrderedKey, V comparable](kcmp infra.OrderedKeyComparator[K], vcmp SklValComparator[V], rand SklRand) *xComSkl[K, V] {
	if kcmp == nil || vcmp == nil || rand == nil {
		panic("[x-com-skl] empty internal core function")
	}

	xsl := &xComSkl[K, V]{
		// Start from 1 means the x-com-skl cache levels at least a one level is fixed
		levels:  1,
		nodeLen: 0,
		kcmp:    kcmp,
		vcmp:    vcmp,
		rand:    rand,
	}
	xsl.head = newXComSklNode[K, V](xSkipListMaxLevel, *new(K), *new(V))
	// Initialization.
	// The head must be initialized with array element size with xSkipListMaxLevel.
	for i := 0; i < xSkipListMaxLevel; i++ {
		xsl.head.levels()[i].setForward(nil)
	}
	xsl.head.setBackward(nil)
	xsl.tail = nil
	xsl.pool = &sync.Pool{
		New: func() any {
			return make([]*xComSklNode[K, V], xSkipListMaxLevel)
		},
	}
	return xsl
}
