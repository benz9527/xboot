package list

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

import (
	"sync"
	"sync/atomic"
)

const (
	xSkipListMaxLevel    = 32   // 2^32 - 1 elements
	xSkipListProbability = 0.25 // P = 1/4, a skip list node element has 1/4 probability to have a level
)

var (
	_ SkipListElement[uint8, *emptyHashObject] = (*xSkipListElement[uint8, *emptyHashObject])(nil)
	_ SkipListNode[uint8, *emptyHashObject]    = (*xSkipListNode[uint8, *emptyHashObject])(nil)
	_ SkipListLevel[uint8, *emptyHashObject]   = (*skipListLevel[uint8, *emptyHashObject])(nil)
	_ SkipList[uint8, *emptyHashObject]        = (*xSkipList[uint8, *emptyHashObject])(nil)
)

type xSkipListElement[W SkipListWeight, O HashObject] struct {
	weight W
	object O
}

func (e *xSkipListElement[W, O]) Weight() W {
	return e.weight
}

func (e *xSkipListElement[W, O]) Object() O {
	return e.object
}

type skipListLevel[W SkipListWeight, O HashObject] struct {
	// Works for the forward iteration direction.
	successor SkipListNode[W, O]
	// Ignore the node level span metadata (for rank).
}

func (lvl *skipListLevel[W, O]) forwardSuccessor() SkipListNode[W, O] {
	if lvl == nil {
		return nil
	}
	return lvl.successor
}

func (lvl *skipListLevel[W, O]) setForwardSuccessor(succ SkipListNode[W, O]) {
	if lvl == nil {
		return
	}
	lvl.successor = succ
}

func newSkipListLevel[W SkipListWeight, V HashObject](succ SkipListNode[W, V]) SkipListLevel[W, V] {
	return &skipListLevel[W, V]{
		successor: succ,
	}
}

// The cache level array index > 0, it is the Y axis, and it means that it is the interval after
//
//	the bisection search. Used to locate an element quickly.
//
// The cache level array index == 0, it is the X axis, and it means that it is the bits container.
type xSkipListNode[W SkipListWeight, O HashObject] struct {
	// The cache part.
	// When the current node works as a data node, it doesn't contain levels metadata.
	// If a node is a level node, the cache is from levels[0], but it is differed
	//  to the sentinel's levels[0].
	levelList []SkipListLevel[W, O] // The cache level array.
	element   SkipListElement[W, O]
	// Works for a backward iteration direction.
	predecessor SkipListNode[W, O]
}

func (node *xSkipListNode[W, O]) Element() SkipListElement[W, O] {
	if node == nil {
		return nil
	}
	return node.element
}

func (node *xSkipListNode[W, O]) setElement(e SkipListElement[W, O]) {
	if node == nil {
		return
	}
	node.element = e
}

func (node *xSkipListNode[W, O]) backwardPredecessor() SkipListNode[W, O] {
	if node == nil {
		return nil
	}
	return node.predecessor
}

func (node *xSkipListNode[W, O]) setBackwardPredecessor(pred SkipListNode[W, O]) {
	if node == nil {
		return
	}
	node.predecessor = pred
}

func (node *xSkipListNode[W, O]) levels() []SkipListLevel[W, O] {
	if node == nil {
		return nil
	}
	return node.levelList
}

func (node *xSkipListNode[W, O]) Free() {
	node.element = nil
	node.predecessor = nil
	node.levelList = nil
}

func newXSkipListNode[W SkipListWeight, O HashObject](level int32, weight W, obj O) SkipListNode[W, O] {
	e := &xSkipListNode[W, O]{
		element: &xSkipListElement[W, O]{
			weight: weight,
			object: obj,
		},
		// Fill zero to all level span.
		// Initialization.
		levelList: make([]SkipListLevel[W, O], level),
	}
	for i := int32(0); i < level; i++ {
		e.levelList[i] = newSkipListLevel[W, O](nil)
	}
	return e
}

// This is a sentinel node and used to point to nodes in memory.
type xSkipList[W SkipListWeight, O HashObject] struct {
	cmp          SkipListWeightComparator[W]
	rand         SkipListRand
	traversePool *sync.Pool
	// The sentinel node.
	// The head.levels[0].successor is the first data node of skip-list.
	// From head.levels[1], they point to the levels node (i.e., the cache metadata)
	head SkipListNode[W, O]
	// The sentinel node.
	tail SkipListNode[W, O]
	// The real max level in used. And the max limitation is xSkipListMaxLevel.
	level int32
	len   int32
}

func (skl *xSkipList[W, O]) loadTraverse() []SkipListNode[W, O] {
	traverse, ok := skl.traversePool.Get().([]SkipListNode[W, O])
	if !ok {
		panic("load unknown traverse element from pool")
	}
	return traverse
}

func (skl *xSkipList[W, O]) putTraverse(traverse []SkipListNode[W, O]) {
	for i := 0; i < xSkipListMaxLevel; i++ {
		traverse[i] = nil
	}
	skl.traversePool.Put(traverse)
}

func (skl *xSkipList[W, O]) Level() int32 {
	return atomic.LoadInt32(&skl.level)
}

func (skl *xSkipList[W, O]) Len() int32 {
	return atomic.LoadInt32(&skl.len)
}

func (skl *xSkipList[W, O]) ForEach(fn func(idx int64, weight W, obj O)) {
	var (
		x   SkipListNode[W, O]
		idx int64
	)
	x = skl.head.levels()[0].forwardSuccessor()
	for x != nil {
		next := x.levels()[0].forwardSuccessor()
		fn(idx, x.Element().Weight(), x.Element().Object())
		idx++
		x = next
	}
}

func (skl *xSkipList[W, O]) Insert(weight W, obj O) (SkipListNode[W, O], bool) {
	var (
		predecessor = skl.head
		traverse    = skl.loadTraverse()
	)
	defer func() {
		skl.putTraverse(traverse)
	}()

	// Iteration from top to bottom.
	// First to iterate the cache and access the data finally.
	for i := atomic.LoadInt32(&skl.level) - 1; i >= 0; i-- { // move down level
		for predecessor.levels()[i].forwardSuccessor() != nil {
			cur := predecessor.levels()[i].forwardSuccessor()
			res := skl.cmp(cur.Element().Weight(), weight)
			if res < 0 || (res == 0 && cur.Element().Object().Hash() < obj.Hash()) {
				predecessor = cur // Changes the node iteration path to locate different node.
			} else if res == 0 && cur.Element().Object().Hash() == obj.Hash() {
				return nil, false
			} else {
				break
			}
		}
		// 1. (weight duplicated) If new element hash is lower than current node's (do pre-append to current node)
		// 2. (weight duplicated) If new element hash is greater than current node's (do append next to current node)
		// 3. (weight duplicated) If new element hash equals to current node's (replace an element, because the hash
		//      value and element are not strongly correlated)
		// 4. (new weight) If a new element does not exist, (do append next to the current node)
		traverse[i] = predecessor
	}

	// Each duplicated weight element may contain its cache levels.
	// It means that duplicated weight elements query through the cache (O(logN))
	// But duplicated elements query (linear probe) will be degraded into O(N)
	lvl := skl.rand(xSkipListMaxLevel, skl.Len())
	if lvl > skl.Level() {
		for i := skl.Level(); i < lvl; i++ {
			// Update the whole traverse path, from top to bottom.
			traverse[i] = skl.head // avoid nil pointer
		}
		atomic.StoreInt32(&skl.level, lvl)
	}

	newNode := newXSkipListNode[W, O](lvl, weight, obj)
	// Insert new node and update the new node levels metadata.
	for i := int32(0); i < lvl; i++ {
		next := traverse[i].levels()[i].forwardSuccessor()
		newNode.levels()[i].setForwardSuccessor(next)
		// May pre-append to adjust 2 elements' order
		traverse[i].levels()[i].setForwardSuccessor(newNode)
	}
	if traverse[0] == skl.head {
		newNode.setBackwardPredecessor(nil)
	} else {
		newNode.setBackwardPredecessor(traverse[0])
	}
	if newNode.levels()[0].forwardSuccessor() == nil {
		skl.tail = newNode
	} else {
		newNode.levels()[0].forwardSuccessor().setBackwardPredecessor(newNode)
	}
	atomic.AddInt32(&skl.len, 1)
	return newNode, true
}

// findPredecessor0 is used to find the (successor) first element whose weight equals to target value.
// Preparing for linear probing. O(N)
// @return value 1: the predecessor node
// @return value 2: the query traverse path (nodes)
func (skl *xSkipList[W, O]) findPredecessor0(weight W) (SkipListNode[W, O], []SkipListNode[W, O]) {
	var (
		predecessor SkipListNode[W, O]
		traverse    = skl.loadTraverse()
	)
	predecessor = skl.head
	for i := skl.Level() - 1; i >= 0; i-- {
		// Note: Will start probing linearly from a local position in some interval
		// O(N)
		for predecessor.levels()[i].forwardSuccessor() != nil {
			cur := predecessor.levels()[i].forwardSuccessor()
			res := skl.cmp(cur.Element().Weight(), weight)
			// find predecessor node
			if res < 0 {
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

	target := predecessor.levels()[0].forwardSuccessor()
	// Check next element's weight
	if target != nil && skl.cmp(target.Element().Weight(), weight) == 0 {
		return predecessor, traverse
	}
	return nil, traverse // not found
}

func (skl *xSkipList[W, O]) removeNode(x SkipListNode[W, O], traverse []SkipListNode[W, O]) {
	for i := int32(0); i < skl.Level(); i++ {
		if traverse[i].levels()[i].forwardSuccessor() == x {
			traverse[i].levels()[i].setForwardSuccessor(x.levels()[i].forwardSuccessor())
		}
	}
	if next := x.levels()[0].forwardSuccessor(); next != nil {
		// Adjust the predecessor.
		next.setBackwardPredecessor(x.backwardPredecessor())
	} else {
		skl.tail = x.backwardPredecessor()
	}
	for skl.Level() > 1 && skl.head.levels()[skl.Level()-1].forwardSuccessor() == nil {
		atomic.AddInt32(&skl.level, -1)
	}
	atomic.AddInt32(&skl.len, -1)
}

func (skl *xSkipList[W, O]) RemoveFirst(weight W) SkipListElement[W, O] {
	predecessor, traverse := skl.findPredecessor0(weight)
	defer func() {
		skl.putTraverse(traverse)
	}()
	if predecessor == nil {
		return nil // not found
	}

	target := predecessor.levels()[0].forwardSuccessor()
	if target != nil && skl.cmp(target.Element().Weight(), weight) == 0 {
		skl.removeNode(target, traverse)
		return target.Element()
	}
	return nil // not found
}

func (skl *xSkipList[W, O]) RemoveAll(weight W) []SkipListElement[W, O] {
	predecessor, traverse := skl.findPredecessor0(weight)
	defer func() {
		skl.putTraverse(traverse)
	}()
	if predecessor == nil {
		return nil // not found
	}

	elements := make([]SkipListElement[W, O], 0, 16)
	for cur := predecessor.levels()[0].forwardSuccessor(); cur != nil && skl.cmp(cur.Element().Weight(), weight) == 0; {
		skl.removeNode(cur, traverse)
		elements = append(elements, cur.Element())
		free := cur
		cur = cur.levels()[0].forwardSuccessor()
		free.Free()
	}
	return elements
}

func (skl *xSkipList[W, O]) RemoveIfMatch(weight W, cmp SkipListObjectMatcher[O]) []SkipListElement[W, O] {
	predecessor, traverse := skl.findPredecessor0(weight)
	defer func() {
		skl.putTraverse(traverse)
	}()
	if predecessor == nil {
		return nil // not found
	}

	elements := make([]SkipListElement[W, O], 0, 16)
	for cur := predecessor.levels()[0].forwardSuccessor(); cur != nil && skl.cmp(cur.Element().Weight(), weight) == 0; {
		if cmp(cur.Element().Object()) {
			skl.removeNode(cur, traverse)
			elements = append(elements, cur.Element())
			next := cur.levels()[0].forwardSuccessor()
			cur.Free()
			cur = next
		} else {
			// Merge the traverse path.
			for i := 0; i < len(cur.levels()); i++ {
				traverse[i] = cur
			}
			cur = cur.levels()[0].forwardSuccessor()
		}
	}
	return elements
}

func (skl *xSkipList[W, O]) FindFirst(weight W) SkipListElement[W, O] {
	e, traverse := skl.findPredecessor0(weight)
	defer func() {
		skl.putTraverse(traverse)
	}()
	if e.levels() == nil {
		return nil
	}

	return e.levels()[0].forwardSuccessor().Element()
}

func (skl *xSkipList[W, O]) FindAll(weight W) []SkipListElement[W, O] {
	predecessor, traverse := skl.findPredecessor0(weight)
	defer func() {
		skl.putTraverse(traverse)
	}()
	if predecessor == nil {
		return nil // not found
	}

	elements := make([]SkipListElement[W, O], 0, 16)
	for cur := predecessor.levels()[0].forwardSuccessor(); cur != nil && skl.cmp(cur.Element().Weight(), weight) == 0; cur = cur.levels()[0].forwardSuccessor() {
		elements = append(elements, cur.Element())
	}
	return elements
}

func (skl *xSkipList[W, O]) FindIfMatch(weight W, cmp SkipListObjectMatcher[O]) []SkipListElement[W, O] {
	predecessor, traverse := skl.findPredecessor0(weight)
	defer func() {
		skl.putTraverse(traverse)
	}()
	if predecessor == nil {
		return nil // not found
	}

	elements := make([]SkipListElement[W, O], 0, 16)
	for cur := predecessor.levels()[0].forwardSuccessor(); cur != nil && skl.cmp(cur.Element().Weight(), weight) == 0; cur = cur.levels()[0].forwardSuccessor() {
		if cmp(cur.Element().Object()) {
			elements = append(elements, cur.Element())
		}
	}
	return elements
}

func (skl *xSkipList[W, O]) PopHead() (e SkipListElement[W, O]) {
	target := skl.head
	if target == nil || skl.Len() <= 0 {
		return
	}
	if target = target.levels()[0].forwardSuccessor(); target == nil {
		return
	}
	e = target.Element()
	e = skl.RemoveFirst(e.Weight())
	return
}

func (skl *xSkipList[W, O]) Free() {
	var (
		x, next SkipListNode[W, O]
		idx     int
	)
	x = skl.head.levels()[0].forwardSuccessor()
	for x != nil {
		next = x.levels()[0].forwardSuccessor()
		x.Free()
		x = nil
		x = next
	}
	for idx = 0; idx < xSkipListMaxLevel; idx++ {
		skl.head.levels()[idx].setForwardSuccessor(nil)
	}
	skl.tail = nil
	skl.traversePool = nil
}

func NewXSkipList[W SkipListWeight, O HashObject](cmp SkipListWeightComparator[W], rand SkipListRand) SkipList[W, O] {
	if cmp == nil || rand == nil {
		panic("empty inner core function to new x-skip-list")
	}

	xsl := &xSkipList[W, O]{
		// Start from 1 means the x-skip-list cache levels at least a one level is fixed
		level: 1,
		len:   0,
		cmp:   cmp,
		rand:  rand,
	}
	xsl.head = newXSkipListNode[W, O](xSkipListMaxLevel, *new(W), *new(O))
	// Initialization.
	// The head must be initialized with array element size with xSkipListMaxLevel.
	for i := 0; i < xSkipListMaxLevel; i++ {
		xsl.head.levels()[i].setForwardSuccessor(nil)
	}
	xsl.head.setBackwardPredecessor(nil)
	xsl.tail = nil
	xsl.traversePool = &sync.Pool{
		New: func() any {
			return make([]SkipListNode[W, O], xSkipListMaxLevel)
		},
	}
	return xsl
}
