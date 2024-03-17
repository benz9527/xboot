package list

import "github.com/benz9527/xboot/lib/infra"

// References:
// https://people.csail.mit.edu/shanir/publications/DCAS.pdf
// https://www.cl.cam.ac.uk/teaching/0506/Algorithms/skiplists.pdf
// github:
// classic: https://github.com/antirez/disque/blob/master/src/skiplist.h
// classic: https://github.com/antirez/disque/blob/master/src/skiplist.c
// zskiplist: https://github1s.com/redis/redis/blob/unstable/src/t_zset.c
// https://github.com/liyue201/gostl
// https://github.com/chen3feng/stl4go
// test:
// https://github.com/chen3feng/skiplist-survey

// Pastes from JDK
// Head nodes          Index nodes
// +-+    right        +-+                      +-+
// |2|---------------->| |--------------------->| |->null
// +-+                 +-+                      +-+
//  | down              |                        |
//  v                   v                        v
// +-+            +-+  +-+       +-+            +-+       +-+
// |1|----------->| |->| |------>| |----------->| |------>| |->null
// +-+            +-+  +-+       +-+            +-+       +-+
//  v              |    |         |              |         |
// Nodes  next     v    v         v              v         v
// +-+  +-+  +-+  +-+  +-+  +-+  +-+  +-+  +-+  +-+  +-+  +-+
// | |->|A|->|B|->|C|->|D|->|E|->|F|->|G|->|H|->|I|->|J|->|K|->null
// +-+  +-+  +-+  +-+  +-+  +-+  +-+  +-+  +-+  +-+  +-+  +-+

type SkipListNode[W infra.OrderedKey, O comparable] interface {
	Free()
	Element() SkipListElement[W, O]
	setElement(e SkipListElement[W, O])
	backwardPredecessor() SkipListNode[W, O]
	setBackwardPredecessor(pred SkipListNode[W, O])
	levels() []SkipListLevel[W, O]
}

type SkipListLevel[W infra.OrderedKey, O comparable] interface {
	forwardSuccessor() SkipListNode[W, O]
	setForwardSuccessor(succ SkipListNode[W, O])
}

const (
	xSkipListMaxLevel    = 32   // 2^32 - 1 elements
	xSkipListProbability = 0.25 // P = 1/4, a skip list node element has 1/4 probability to have a level
)

//var (
//	_ SkipListElement[uint8, *emptyHashObject] = (*xSklElement[uint8, *emptyHashObject])(nil)
//	_ SkipListNode[uint8, *emptyHashObject]    = (*xSkipListNode[uint8, *emptyHashObject])(nil)
//	_ SkipListLevel[uint8, *emptyHashObject]   = (*skipListLevel[uint8, *emptyHashObject])(nil)
//	_ SkipList[uint8, *emptyHashObject]        = (*xSkipList[uint8, *emptyHashObject])(nil)
//)
//
//
//type skipListLevel[K SkipListWeight, V HashObject] struct {
//	// Works for the forward iteration direction.
//	successor SkipListNode[K, V]
//	// Ignore the node level span metadata (for rank).
//}
//
//func (lvl *skipListLevel[K, V]) forwardSuccessor() SkipListNode[K, V] {
//	if lvl == nil {
//		return nil
//	}
//	return lvl.successor
//}
//
//func (lvl *skipListLevel[K, V]) setForwardSuccessor(succ SkipListNode[K, V]) {
//	if lvl == nil {
//		return
//	}
//	lvl.successor = succ
//}
//
//func newSkipListLevel[K SkipListWeight, V HashObject](succ SkipListNode[K, V]) SkipListLevel[K, V] {
//	return &skipListLevel[K, V]{
//		successor: succ,
//	}
//}
//
//// The cache level array index > 0, it is the Y axis, and it means that it is the interval after
////
////	the bisection search. Used to locate an element quickly.
////
//// The cache level array index == 0, it is the X axis, and it means that it is the bits container.
//type xSkipListNode[K SkipListWeight, V HashObject] struct {
//	// The cache part.
//	// When the current node works as a data node, it doesn't contain levels metadata.
//	// If a node is a level node, the cache is from levels[0], but it is differed
//	//  to the sentinel's levels[0].
//	levelList []SkipListLevel[K, V] // The cache level array.
//	element   SkipListElement[K, V]
//	// Works for a backward iteration direction.
//	predecessor SkipListNode[K, V]
//}
//
//func (node *xSkipListNode[K, V]) Element() SkipListElement[K, V] {
//	if node == nil {
//		return nil
//	}
//	return node.element
//}
//
//func (node *xSkipListNode[K, V]) setElement(e SkipListElement[K, V]) {
//	if node == nil {
//		return
//	}
//	node.element = e
//}
//
//func (node *xSkipListNode[K, V]) backwardPredecessor() SkipListNode[K, V] {
//	if node == nil {
//		return nil
//	}
//	return node.predecessor
//}
//
//func (node *xSkipListNode[K, V]) setBackwardPredecessor(pred SkipListNode[K, V]) {
//	if node == nil {
//		return
//	}
//	node.predecessor = pred
//}
//
//func (node *xSkipListNode[K, V]) levels() []SkipListLevel[K, V] {
//	if node == nil {
//		return nil
//	}
//	return node.levelList
//}
//
//func (node *xSkipListNode[K, V]) Free() {
//	node.element = nil
//	node.predecessor = nil
//	node.levelList = nil
//}
//
//func newXSkipListNode[K SkipListWeight, V HashObject](level int32, key K, obj V) SkipListNode[K, V] {
//	e := &xSkipListNode[K, V]{
//		element: &xSklElement[K, V]{
//			key: key,
//			val: obj,
//		},
//		// Fill zero to all level span.
//		// Initialization.
//		levelList: make([]SkipListLevel[K, V], level),
//	}
//	for i := int32(0); i < level; i++ {
//		e.levelList[i] = newSkipListLevel[K, V](nil)
//	}
//	return e
//}
//
//// This is a sentinel node and used to point to nodes in memory.
//type xSkipList[K SkipListWeight, V HashObject] struct {
//	cmp          SklWeightComparator[K]
//	rand         SklRand
//	traversePool *sync.Pool
//	// The sentinel node.
//	// The head.levels[0].successor is the first data node of skip-list.
//	// From head.levels[1], they point to the levels node (i.e., the cache metadata)
//	head SkipListNode[K, V]
//	// The sentinel node.
//	tail SkipListNode[K, V]
//	// The real max level in used. And the max limitation is xSkipListMaxLevel.
//	level int32
//	nodeLen   int32
//}
//
//func (skl *xSkipList[K, V]) loadTraverse() []SkipListNode[K, V] {
//	traverse, ok := skl.traversePool.LoadFirst().([]SkipListNode[K, V])
//	if !ok {
//		panic("load unknown traverse element from pool")
//	}
//	return traverse
//}
//
//func (skl *xSkipList[K, V]) putTraverse(traverse []SkipListNode[K, V]) {
//	for i := 0; i < xSkipListMaxLevel; i++ {
//		traverse[i] = nil
//	}
//	skl.traversePool.Put(traverse)
//}
//
//func (skl *xSkipList[K, V]) Levels() int32 {
//	return atomic.LoadInt32(&skl.level)
//}
//
//func (skl *xSkipList[K, V]) Len() int32 {
//	return atomic.LoadInt32(&skl.nodeLen)
//}
//
//func (skl *xSkipList[K, V]) ForEach(fn func(idx int64, key K, obj V)) {
//	var (
//		x   SkipListNode[K, V]
//		idx int64
//	)
//	x = skl.head.levels()[0].forwardSuccessor()
//	for x != nil {
//		next := x.levels()[0].forwardSuccessor()
//		fn(idx, x.Element().Key(), x.Element().Val())
//		idx++
//		x = next
//	}
//}
//
//func (skl *xSkipList[K, V]) Insert(key K, obj V) (SkipListNode[K, V], bool) {
//	var (
//		predecessor = skl.head
//		traverse    = skl.loadTraverse()
//	)
//	defer func() {
//		skl.putTraverse(traverse)
//	}()
//
//	// Iteration from top to bottom.
//	// First to iterate the cache and access the data finally.
//	for i := atomic.LoadInt32(&skl.level) - 1; i >= 0; i-- { // move down level
//		for predecessor.levels()[i].forwardSuccessor() != nil {
//			cur := predecessor.levels()[i].forwardSuccessor()
//			res := skl.cmp(cur.Element().Key(), key)
//			if res < 0 || (res == 0 && cur.Element().Val().Hash() < obj.Hash()) {
//				predecessor = cur // Changes the node iteration path to locate different node.
//			} else if res == 0 && cur.Element().Val().Hash() == obj.Hash() {
//				return nil, false
//			} else {
//				break
//			}
//		}
//		// 1. (key duplicated) If new element hash is lower than current node's (do pre-append to current node)
//		// 2. (key duplicated) If new element hash is greater than current node's (do append next to current node)
//		// 3. (key duplicated) If new element hash equals to current node's (replace an element, because the hash
//		//      value and element are not strongly correlated)
//		// 4. (new key) If a new element does not exist, (do append next to the current node)
//		traverse[i] = predecessor
//	}
//
//	// Each duplicated key element may contain its cache levels.
//	// It means that duplicated key elements query through the cache (V(logN))
//	// But duplicated elements query (linear probe) will be degraded into V(N)
//	lvl := skl.rand(xSkipListMaxLevel, skl.Len())
//	if lvl > skl.Levels() {
//		for i := skl.Levels(); i < lvl; i++ {
//			// Update the whole traverse path, from top to bottom.
//			traverse[i] = skl.head // avoid nil pointer
//		}
//		atomic.StoreInt32(&skl.level, lvl)
//	}
//
//	newNode := newXSkipListNode[K, V](lvl, key, obj)
//	// Insert new node and update the new node levels metadata.
//	for i := int32(0); i < lvl; i++ {
//		next := traverse[i].levels()[i].forwardSuccessor()
//		newNode.levels()[i].setForwardSuccessor(next)
//		// May pre-append to adjust 2 elements' order
//		traverse[i].levels()[i].setForwardSuccessor(newNode)
//	}
//	if traverse[0] == skl.head {
//		newNode.setBackwardPredecessor(nil)
//	} else {
//		newNode.setBackwardPredecessor(traverse[0])
//	}
//	if newNode.levels()[0].forwardSuccessor() == nil {
//		skl.tail = newNode
//	} else {
//		newNode.levels()[0].forwardSuccessor().setBackwardPredecessor(newNode)
//	}
//	atomic.AddInt32(&skl.nodeLen, 1)
//	return newNode, true
//}
//
//// findPredecessor0 is used to find the (successor) first element whose key equals to target value.
//// Preparing for linear probing. V(N)
//// @return value 1: the predecessor node
//// @return value 2: the query traverse path (nodes)
//func (skl *xSkipList[K, V]) findPredecessor0(key K) (SkipListNode[K, V], []SkipListNode[K, V]) {
//	var (
//		predecessor SkipListNode[K, V]
//		traverse    = skl.loadTraverse()
//	)
//	predecessor = skl.head
//	for i := skl.Levels() - 1; i >= 0; i-- {
//		// Note: Will start probing linearly from a local position in some interval
//		// V(N)
//		for predecessor.levels()[i].forwardSuccessor() != nil {
//			cur := predecessor.levels()[i].forwardSuccessor()
//			res := skl.cmp(cur.Element().Key(), key)
//			// find predecessor node
//			if res < 0 {
//				predecessor = cur
//			} else {
//				// downward to the next level, continue to find
//				break
//			}
//		}
//		traverse[i] = predecessor
//	}
//
//	if predecessor == nil {
//		return nil, traverse // not found
//	}
//
//	target := predecessor.levels()[0].forwardSuccessor()
//	// Check next element's key
//	if target != nil && skl.cmp(target.Element().Key(), key) == 0 {
//		return predecessor, traverse
//	}
//	return nil, traverse // not found
//}
//
//func (skl *xSkipList[K, V]) removeNode(x SkipListNode[K, V], traverse []SkipListNode[K, V]) {
//	for i := int32(0); i < skl.Levels(); i++ {
//		if traverse[i].levels()[i].forwardSuccessor() == x {
//			traverse[i].levels()[i].setForwardSuccessor(x.levels()[i].forwardSuccessor())
//		}
//	}
//	if next := x.levels()[0].forwardSuccessor(); next != nil {
//		// Adjust the predecessor.
//		next.setBackwardPredecessor(x.backwardPredecessor())
//	} else {
//		skl.tail = x.backwardPredecessor()
//	}
//	for skl.Levels() > 1 && skl.head.levels()[skl.Levels()-1].forwardSuccessor() == nil {
//		atomic.AddInt32(&skl.level, -1)
//	}
//	atomic.AddInt32(&skl.nodeLen, -1)
//}
//
//func (skl *xSkipList[K, V]) RemoveFirst(key K) SkipListElement[K, V] {
//	predecessor, traverse := skl.findPredecessor0(key)
//	defer func() {
//		skl.putTraverse(traverse)
//	}()
//	if predecessor == nil {
//		return nil // not found
//	}
//
//	target := predecessor.levels()[0].forwardSuccessor()
//	if target != nil && skl.cmp(target.Element().Key(), key) == 0 {
//		skl.removeNode(target, traverse)
//		return target.Element()
//	}
//	return nil // not found
//}
//
//func (skl *xSkipList[K, V]) RemoveAll(key K) []SkipListElement[K, V] {
//	predecessor, traverse := skl.findPredecessor0(key)
//	defer func() {
//		skl.putTraverse(traverse)
//	}()
//	if predecessor == nil {
//		return nil // not found
//	}
//
//	elements := make([]SkipListElement[K, V], 0, 16)
//	for cur := predecessor.levels()[0].forwardSuccessor(); cur != nil && skl.cmp(cur.Element().Key(), key) == 0; {
//		skl.removeNode(cur, traverse)
//		elements = append(elements, cur.Element())
//		free := cur
//		cur = cur.levels()[0].forwardSuccessor()
//		free.Free()
//	}
//	return elements
//}
//
//func (skl *xSkipList[K, V]) RemoveIfMatch(key K, cmp SklObjMatcher[V]) []SkipListElement[K, V] {
//	predecessor, traverse := skl.findPredecessor0(key)
//	defer func() {
//		skl.putTraverse(traverse)
//	}()
//	if predecessor == nil {
//		return nil // not found
//	}
//
//	elements := make([]SkipListElement[K, V], 0, 16)
//	for cur := predecessor.levels()[0].forwardSuccessor(); cur != nil && skl.cmp(cur.Element().Key(), key) == 0; {
//		if cmp(cur.Element().Val()) {
//			skl.removeNode(cur, traverse)
//			elements = append(elements, cur.Element())
//			next := cur.levels()[0].forwardSuccessor()
//			cur.Free()
//			cur = next
//		} else {
//			// Merge the traverse path.
//			for i := 0; i < nodeLen(cur.levels()); i++ {
//				traverse[i] = cur
//			}
//			cur = cur.levels()[0].forwardSuccessor()
//		}
//	}
//	return elements
//}
//
//func (skl *xSkipList[K, V]) LoadFirst(key K) SkipListElement[K, V] {
//	e, traverse := skl.findPredecessor0(key)
//	defer func() {
//		skl.putTraverse(traverse)
//	}()
//	if e.levels() == nil {
//		return nil
//	}
//
//	return e.levels()[0].forwardSuccessor().Element()
//}
//
//func (skl *xSkipList[K, V]) FindAll(key K) []SkipListElement[K, V] {
//	predecessor, traverse := skl.findPredecessor0(key)
//	defer func() {
//		skl.putTraverse(traverse)
//	}()
//	if predecessor == nil {
//		return nil // not found
//	}
//
//	elements := make([]SkipListElement[K, V], 0, 16)
//	for cur := predecessor.levels()[0].forwardSuccessor(); cur != nil && skl.cmp(cur.Element().Key(), key) == 0; cur = cur.levels()[0].forwardSuccessor() {
//		elements = append(elements, cur.Element())
//	}
//	return elements
//}
//
//func (skl *xSkipList[K, V]) FindIfMatch(key K, cmp SklObjMatcher[V]) []SkipListElement[K, V] {
//	predecessor, traverse := skl.findPredecessor0(key)
//	defer func() {
//		skl.putTraverse(traverse)
//	}()
//	if predecessor == nil {
//		return nil // not found
//	}
//
//	elements := make([]SkipListElement[K, V], 0, 16)
//	for cur := predecessor.levels()[0].forwardSuccessor(); cur != nil && skl.cmp(cur.Element().Key(), key) == 0; cur = cur.levels()[0].forwardSuccessor() {
//		if cmp(cur.Element().Val()) {
//			elements = append(elements, cur.Element())
//		}
//	}
//	return elements
//}
//
//func (skl *xSkipList[K, V]) PopHead() (e SkipListElement[K, V]) {
//	target := skl.head
//	if target == nil || skl.Len() <= 0 {
//		return
//	}
//	if target = target.levels()[0].forwardSuccessor(); target == nil {
//		return
//	}
//	e = target.Element()
//	e = skl.RemoveFirst(e.Key())
//	return
//}
//
//func (skl *xSkipList[K, V]) Free() {
//	var (
//		x, next SkipListNode[K, V]
//		idx     int
//	)
//	x = skl.head.levels()[0].forwardSuccessor()
//	for x != nil {
//		next = x.levels()[0].forwardSuccessor()
//		x.Free()
//		x = nil
//		x = next
//	}
//	for idx = 0; idx < xSkipListMaxLevel; idx++ {
//		skl.head.levels()[idx].setForwardSuccessor(nil)
//	}
//	skl.tail = nil
//	skl.traversePool = nil
//}
//
//func NewXSkipList[K SkipListWeight, V HashObject](cmp SklWeightComparator[K], rand SklRand) SkipList[K, V] {
//	if cmp == nil || rand == nil {
//		panic("empty inner core function to new x-skip-list")
//	}
//
//	xsl := &xSkipList[K, V]{
//		// Start from 1 means the x-skip-list cache levels at least a one level is fixed
//		level: 1,
//		nodeLen:   0,
//		cmp:   cmp,
//		rand:  rand,
//	}
//	xsl.head = newXSkipListNode[K, V](xSkipListMaxLevel, *new(K), *new(V))
//	// Initialization.
//	// The head must be initialized with array element size with xSkipListMaxLevel.
//	for i := 0; i < xSkipListMaxLevel; i++ {
//		xsl.head.levels()[i].setForwardSuccessor(nil)
//	}
//	xsl.head.setBackwardPredecessor(nil)
//	xsl.tail = nil
//	xsl.traversePool = &sync.Pool{
//		New: func() any {
//			return make([]SkipListNode[K, V], xSkipListMaxLevel)
//		},
//	}
//	return xsl
//}
