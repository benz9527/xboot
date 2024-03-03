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
	saferand "crypto/rand"
	"encoding/binary"
	"math"
	"math/bits"
	randv2 "math/rand/v2"
	"sync"
	"sync/atomic"
)

const (
	xSkipListMaxLevel    = 32   // 2^32 - 1 elements
	xSkipListProbability = 0.25 // P = 1/4, a skip list node element has 1/4 probability to have a level
)

func maxLevels(totalElements int64, P float64) int {
	// Ref https://www.cl.cam.ac.uk/teaching/2005/Algorithms/skiplists.pdf
	// maxLevels = log(1/P) * log(totalElements)
	// P = 1/4, totalElements = 2^32 - 1
	if totalElements <= 0 {
		return 0
	}
	return int(math.Ceil(math.Log(1/P) * math.Log(float64(totalElements))))
}

func randomLevel(maxLevel int, currentElements int32) int32 {
	level := 1
	// goland math random (math.Float64()) contains global mutex lock
	// Ref
	// https://cs.opensource.google/go/go/+/refs/tags/go1.21.5:src/math/rand/rand.go
	// https://cs.opensource.google/go/go/+/refs/tags/go1.21.5:src/math/bits/bits.go
	// 1. Avoid to use global mutex lock
	// 2. Avoid to generate random number each time
	for float64(randv2.Int64()&0xFFFF) < xSkipListProbability*0xFFFF {
		level += 1
	}
	if level < xSkipListMaxLevel {
		return int32(level)
	}
	return xSkipListMaxLevel
}

// randomLevelV2 is the skip list level element.
// Dynamic level calculation.
func randomLevelV2(maxLevel int, currentElements int32) int32 {
	// Call function maxLevels to get total?
	// maxLevel => n, 2^n -1, there will be 2^n-1 elements in the skip list
	var total uint64
	if maxLevel == xSkipListMaxLevel {
		total = uint64(math.MaxUint32)
	} else {
		total = uint64(1)<<maxLevel - 1
	}
	// goland math random (math.Float64()) contains global mutex lock
	// Ref
	// https://cs.opensource.google/go/go/+/refs/tags/go1.21.5:src/math/rand/rand.go
	// https://cs.opensource.google/go/go/+/refs/tags/go1.21.5:src/math/bits/bits.go
	// 1. Avoid to use global mutex lock
	// 2. Avoid to generate random number each time
	rest := randv2.Uint64() & total
	// Bits right shift equal to manipulate a high-level bit
	// Calculate the minimum bits of the random number
	tmp := bits.Len64(rest) // Lookup table.
	level := maxLevel - tmp + 1
	// Avoid the value of randomly generated level deviates
	//   far from the number of elements within the skip-list.
	// level should be greater than but approximate to log(currentElements)
	for level > 1 && uint64(1)<<(level-1) > uint64(currentElements) {
		level--
	}
	return int32(level)
}

// randomLevelV3 is the skip list level element.
// Dynamic level calculation.
// Concurrency safe.
func randomLevelV3(maxLevel int, currentElements int32) int32 {
	// Call function maxLevels to get total?
	// maxLevel => n, 2^n -1, there will be 2^n-1 elements in the skip list
	var total uint64
	if maxLevel == xSkipListMaxLevel {
		total = uint64(math.MaxUint32)
	} else {
		total = uint64(1)<<maxLevel - 1
	}
	// goland math random (math.Float64()) contains global mutex lock
	// Ref
	// https://cs.opensource.google/go/go/+/refs/tags/go1.21.5:src/math/rand/rand.go
	// https://cs.opensource.google/go/go/+/refs/tags/go1.21.5:src/math/bits/bits.go
	// 1. Avoid to use global mutex lock
	// 2. Avoid to generate random number each time
	randUint64 := [8]byte{}
	if _, err := saferand.Read(randUint64[:]); err != nil {
		panic(err)
	}
	num := binary.LittleEndian.Uint64(randUint64[:])
	rest := num & total
	// Bits right shift equal to manipulate a high-level bit
	// Calculate the minimum bits of the random number
	tmp := bits.Len64(rest) // Lookup table.
	level := maxLevel - tmp + 1
	// Avoid the value of randomly generated level deviates
	//   far from the number of elements within the skip-list.
	// level should be greater than but approximate to log(currentElements)
	for level > 1 && uint64(1)<<(level-1) > uint64(currentElements) {
		level--
	}
	return int32(level)
}

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
	// The next node element in horizontal direction.
	hForward SkipListNode[W, O]
	// Ignore the node level span metadata.
}

func (lvl *skipListLevel[W, O]) forwardSuccessor() SkipListNode[W, O] {
	if lvl == nil {
		return nil
	}
	return lvl.hForward
}

func (lvl *skipListLevel[W, O]) setForwardSuccessor(forward SkipListNode[W, O]) {
	if lvl == nil {
		return
	}
	lvl.hForward = forward
}

func newSkipListLevel[W SkipListWeight, V HashObject](forward SkipListNode[W, V]) SkipListLevel[W, V] {
	return &skipListLevel[W, V]{
		hForward: forward,
	}
}

// The cache level array index > 0, it is the Y axis, and it means that it is the interval after
//
//	the bisection search. Used to locate an element quickly.
//
// The cache level array index == 0, it is the X axis, and it means that it is the data container.
type xSkipListNode[W SkipListWeight, O HashObject] struct {
	// The cache part.
	// When current node work as data node, it doesn't contain levels metadata.
	// If a node is a level node, the cache is from levels[0], but it is differ
	//  to the sentinel's levels[0].
	lvls    []SkipListLevel[W, O] // The cache level array.
	element SkipListElement[W, O]
	// The next node element in the vertical direction.
	// A node with level 0 without any next node element at all usually.
	// A node work as level node (i.e. the cache node) must contain vBackward metadata.
	vBackward SkipListNode[W, O]
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
	return node.vBackward
}

func (node *xSkipListNode[W, O]) setBackwardPredecessor(backward SkipListNode[W, O]) {
	if node == nil {
		return
	}
	node.vBackward = backward
}

func (node *xSkipListNode[W, O]) levels() []SkipListLevel[W, O] {
	if node == nil {
		return nil
	}
	return node.lvls
}

func (node *xSkipListNode[W, O]) Free() {
	node.element = nil
	node.vBackward = nil
	node.lvls = nil
}

func newXSkipListNode[W SkipListWeight, O HashObject](level int32, weight W, obj O) SkipListNode[W, O] {
	e := &xSkipListNode[W, O]{
		element: &xSkipListElement[W, O]{
			weight: weight,
			object: obj,
		},
		// Fill zero to all level span.
		// Initialization.
		lvls: make([]SkipListLevel[W, O], level),
	}
	for i := int32(0); i < level; i++ {
		e.lvls[i] = newSkipListLevel[W, O](nil)
	}
	return e
}

// This is a sentinel node and used to point to nodes in memory.
type xSkipList[W SkipListWeight, O HashObject] struct {
	cmp          SkipListWeightComparator[W]
	rand         SkipListRand
	traversePool *sync.Pool
	// The sentinel node.
	// The head.levels[0].hForward is the first data node of skip-list.
	// From head.levels[1], they point to the levels node (i.e. the cache metadata)
	head SkipListNode[W, O]
	// The sentinel node.
	tail SkipListNode[W, O]
	// The real max level in used. And the max limitation is xSkipListMaxLevel.
	level int32
	len   int32
}

func (xsl *xSkipList[W, O]) loadTraverse() []SkipListNode[W, O] {
	traverse, ok := xsl.traversePool.Get().([]SkipListNode[W, O])
	if !ok {
		panic("load unknown traverse element from pool")
	}
	return traverse
}

func (xsl *xSkipList[W, O]) putTraverse(traverse []SkipListNode[W, O]) {
	for i := 0; i < xSkipListMaxLevel; i++ {
		traverse[i] = nil
	}
	xsl.traversePool.Put(traverse)
}

func (xsl *xSkipList[W, O]) Level() int32 {
	return atomic.LoadInt32(&xsl.level)
}

func (xsl *xSkipList[W, O]) Len() int32 {
	return atomic.LoadInt32(&xsl.len)
}

func (xsl *xSkipList[W, O]) ForEach(fn func(idx int64, weight W, obj O)) {
	var (
		x   SkipListNode[W, O]
		idx int64
	)
	x = xsl.head.levels()[0].forwardSuccessor()
	for x != nil {
		next := x.levels()[0].forwardSuccessor()
		fn(idx, x.Element().Weight(), x.Element().Object())
		idx++
		x = next
	}
}

func (xsl *xSkipList[W, O]) Insert(weight W, obj O) (SkipListNode[W, O], bool) {
	var (
		predecessor = xsl.head
		traverse    = xsl.loadTraverse()
	)
	defer func() {
		xsl.putTraverse(traverse)
	}()

	// Iteration from top to bottom.
	// First to iterate the cache and access the data finally.
	for i := atomic.LoadInt32(&xsl.level) - 1; i >= 0; i-- { // move down level
		for predecessor.levels()[i].forwardSuccessor() != nil {
			cur := predecessor.levels()[i].forwardSuccessor()
			res := xsl.cmp(cur.Element().Weight(), weight)
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
		// 3. (weight duplicated) If new element hash equals to current node's (replace element, because the hash
		//      value and element are not strongly correlated)
		// 4. (new weight) If a new element does not exist, (do append next to current node)
		traverse[i] = predecessor
	}

	// Each duplicated weight element may contain its cache levels.
	// It means that duplicated weight elements query through the cache (O(logN))
	// But duplicated elements query (linear probe) will be degraded into O(N)
	lvl := xsl.rand(xSkipListMaxLevel, xsl.Len())
	if lvl > xsl.Level() {
		for i := xsl.Level(); i < lvl; i++ {
			// Update the whole traverse path, from top to bottom.
			traverse[i] = xsl.head // avoid nil pointer
		}
		atomic.StoreInt32(&xsl.level, lvl)
	}

	newNode := newXSkipListNode[W, O](lvl, weight, obj)
	// Insert new node and update the new node levels metadata.
	for i := int32(0); i < lvl; i++ {
		next := traverse[i].levels()[i].forwardSuccessor()
		newNode.levels()[i].setForwardSuccessor(next)
		// May pre-append to adjust 2 elements' order
		traverse[i].levels()[i].setForwardSuccessor(newNode)
	}
	if traverse[0] == xsl.head {
		newNode.setBackwardPredecessor(nil)
	} else {
		newNode.setBackwardPredecessor(traverse[0])
	}
	if newNode.levels()[0].forwardSuccessor() == nil {
		xsl.tail = newNode
	} else {
		newNode.levels()[0].forwardSuccessor().setBackwardPredecessor(newNode)
	}
	atomic.AddInt32(&xsl.len, 1)
	return newNode, true
}

// findPredecessor0 is used to find the (forward) first element whose weight equals to target value.
// Preparing for linear probing. O(N)
// @return value 1: the predecessor node
// @return value 2: the query traverse path (nodes)
func (xsl *xSkipList[W, O]) findPredecessor0(weight W) (SkipListNode[W, O], []SkipListNode[W, O]) {
	var (
		predecessor SkipListNode[W, O]
		traverse    = xsl.loadTraverse()
	)
	predecessor = xsl.head
	for i := xsl.Level() - 1; i >= 0; i-- {
		// Note: Will start probing linearly from a local position in some interval
		// O(N)
		for predecessor.levels()[i].forwardSuccessor() != nil {
			cur := predecessor.levels()[i].forwardSuccessor()
			res := xsl.cmp(cur.Element().Weight(), weight)
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
	if target != nil && xsl.cmp(target.Element().Weight(), weight) == 0 {
		return predecessor, traverse
	}
	return nil, traverse // not found
}

func (xsl *xSkipList[W, O]) removeNode(x SkipListNode[W, O], traverse []SkipListNode[W, O]) {
	for i := int32(0); i < xsl.Level(); i++ {
		if traverse[i].levels()[i].forwardSuccessor() == x {
			traverse[i].levels()[i].setForwardSuccessor(x.levels()[i].forwardSuccessor())
		}
	}
	if next := x.levels()[0].forwardSuccessor(); next != nil {
		// Adjust the backward.
		next.setBackwardPredecessor(x.backwardPredecessor())
	} else {
		xsl.tail = x.backwardPredecessor()
	}
	for xsl.Level() > 1 && xsl.head.levels()[xsl.Level()-1].forwardSuccessor() == nil {
		atomic.AddInt32(&xsl.level, -1)
	}
	atomic.AddInt32(&xsl.len, -1)
}

func (xsl *xSkipList[W, O]) RemoveFirst(weight W) SkipListElement[W, O] {
	predecessor, traverse := xsl.findPredecessor0(weight)
	defer func() {
		xsl.putTraverse(traverse)
	}()
	if predecessor == nil {
		return nil // not found
	}

	target := predecessor.levels()[0].forwardSuccessor()
	if target != nil && xsl.cmp(target.Element().Weight(), weight) == 0 {
		xsl.removeNode(target, traverse)
		return target.Element()
	}
	return nil // not found
}

func (xsl *xSkipList[W, O]) RemoveAll(weight W) []SkipListElement[W, O] {
	predecessor, traverse := xsl.findPredecessor0(weight)
	defer func() {
		xsl.putTraverse(traverse)
	}()
	if predecessor == nil {
		return nil // not found
	}

	elements := make([]SkipListElement[W, O], 0, 16)
	for cur := predecessor.levels()[0].forwardSuccessor(); cur != nil && xsl.cmp(cur.Element().Weight(), weight) == 0; {
		xsl.removeNode(cur, traverse)
		elements = append(elements, cur.Element())
		free := cur
		cur = cur.levels()[0].forwardSuccessor()
		free.Free()
	}
	return elements
}

func (xsl *xSkipList[W, O]) RemoveIfMatch(weight W, cmp SkipListObjectMatcher[O]) []SkipListElement[W, O] {
	predecessor, traverse := xsl.findPredecessor0(weight)
	defer func() {
		xsl.putTraverse(traverse)
	}()
	if predecessor == nil {
		return nil // not found
	}

	elements := make([]SkipListElement[W, O], 0, 16)
	for cur := predecessor.levels()[0].forwardSuccessor(); cur != nil && xsl.cmp(cur.Element().Weight(), weight) == 0; {
		if cmp(cur.Element().Object()) {
			xsl.removeNode(cur, traverse)
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

func (xsl *xSkipList[W, O]) FindFirst(weight W) SkipListElement[W, O] {
	e, traverse := xsl.findPredecessor0(weight)
	defer func() {
		xsl.putTraverse(traverse)
	}()
	if e.levels() == nil {
		return nil
	}

	return e.levels()[0].forwardSuccessor().Element()
}

func (xsl *xSkipList[W, O]) FindAll(weight W) []SkipListElement[W, O] {
	predecessor, traverse := xsl.findPredecessor0(weight)
	defer func() {
		xsl.putTraverse(traverse)
	}()
	if predecessor == nil {
		return nil // not found
	}

	elements := make([]SkipListElement[W, O], 0, 16)
	for cur := predecessor.levels()[0].forwardSuccessor(); cur != nil && xsl.cmp(cur.Element().Weight(), weight) == 0; cur = cur.levels()[0].forwardSuccessor() {
		elements = append(elements, cur.Element())
	}
	return elements
}

func (xsl *xSkipList[W, O]) FindIfMatch(weight W, cmp SkipListObjectMatcher[O]) []SkipListElement[W, O] {
	predecessor, traverse := xsl.findPredecessor0(weight)
	defer func() {
		xsl.putTraverse(traverse)
	}()
	if predecessor == nil {
		return nil // not found
	}

	elements := make([]SkipListElement[W, O], 0, 16)
	for cur := predecessor.levels()[0].forwardSuccessor(); cur != nil && xsl.cmp(cur.Element().Weight(), weight) == 0; cur = cur.levels()[0].forwardSuccessor() {
		if cmp(cur.Element().Object()) {
			elements = append(elements, cur.Element())
		}
	}
	return elements
}

func (xsl *xSkipList[W, O]) PopHead() (e SkipListElement[W, O]) {
	target := xsl.head
	if target == nil || xsl.Len() <= 0 {
		return
	}
	if target = target.levels()[0].forwardSuccessor(); target == nil {
		return
	}
	e = target.Element()
	e = xsl.RemoveFirst(e.Weight())
	return
}

func (xsl *xSkipList[W, O]) Free() {
	var (
		x, next SkipListNode[W, O]
		idx     int
	)
	x = xsl.head.levels()[0].forwardSuccessor()
	for x != nil {
		next = x.levels()[0].forwardSuccessor()
		x.Free()
		x = nil
		x = next
	}
	for idx = 0; idx < xSkipListMaxLevel; idx++ {
		xsl.head.levels()[idx].setForwardSuccessor(nil)
	}
	xsl.tail = nil
	xsl.traversePool = nil
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
