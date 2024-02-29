package list

import (
	"math"
	"math/bits"
	randv2 "math/rand/v2"
	"sync/atomic"
)

// References:
// https://people.csail.mit.edu/shanir/publications/DCAS.pdf
// https://www.cl.cam.ac.uk/teaching/0506/Algorithms/skiplists.pdf
// github:
// classic: https://github.com/antirez/disque/blob/master/src/skiplist.h
// classic: https://github.com/antirez/disque/blob/master/src/skiplist.c
// https://github.com/liyue201/gostl
// https://github.com/chen3feng/stl4go
// test:
// https://github.com/chen3feng/skiplist-survey

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

func randomLevel() int32 {
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
	// Bits right shift equals to manipulate a high level bit
	// Calculate the minimum bits of the random number
	tmp := bits.Len64(rest)
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

type xSkipListElement[W SkipListWeight, V HashObject] struct {
	weight W
	object V
}

func (e *xSkipListElement[W, V]) Weight() W {
	return e.weight
}

func (e *xSkipListElement[W, V]) Object() V {
	return e.object
}

type skipListLevel[W SkipListWeight, V HashObject] struct {
	// The next node element in horizontal direction.
	hForward SkipListNode[W, V]
	// Ignore the node level span metadata.
}

func (lvl *skipListLevel[W, V]) horizontalForward() SkipListNode[W, V] {
	return lvl.hForward
}

func (lvl *skipListLevel[W, V]) setHorizontalForward(forward SkipListNode[W, V]) {
	lvl.hForward = forward
}

func newSkipListLevel[W SkipListWeight, V HashObject](forward SkipListNode[W, V]) SkipListLevel[W, V] {
	return &skipListLevel[W, V]{
		hForward: forward,
	}
}

type xSkipListNode[W SkipListWeight, V HashObject] struct {
	// The cache part.
	// When current node work as data node, it doesn't contain levels metadata.
	// If a node is a level node, the cache is from levels[0], but it is differ
	//  to the sentinel's levels[0].
	lvls    []SkipListLevel[W, V]
	element SkipListElement[W, V]
	// The next node element in the vertical direction.
	// A node with level 0 without any next node element at all usually.
	// A node work as level node (i.e. the cache node) must contain vBackward metadata.
	vBackward SkipListNode[W, V]
}

func (node *xSkipListNode[W, V]) Element() SkipListElement[W, V] {
	return node.element
}

func (node *xSkipListNode[W, V]) setElement(e SkipListElement[W, V]) {
	node.element = e
}

func (node *xSkipListNode[W, V]) verticalBackward() SkipListNode[W, V] {
	return node.vBackward
}

func (node *xSkipListNode[W, V]) setVerticalBackward(backward SkipListNode[W, V]) {
	node.vBackward = backward
}

func (node *xSkipListNode[W, V]) levels() []SkipListLevel[W, V] {
	return node.lvls
}

func (node *xSkipListNode[W, V]) Free() {
	node.element = nil
	node.vBackward = nil
	node.lvls = nil
}

func newXSkipListNode[W SkipListWeight, V HashObject](level int32, weight W, obj V) SkipListNode[W, V] {
	e := &xSkipListNode[W, V]{
		element: &xSkipListElement[W, V]{
			weight: weight,
			object: obj,
		},
		// Fill zero to all level span.
		// Initialization.
		lvls: make([]SkipListLevel[W, V], level),
	}
	for i := int32(0); i < level; i++ {
		e.lvls[i] = newSkipListLevel[W, V](nil)
	}
	return e
}

// This is a sentinel node and used to point to nodes in memory.
type xSkipList[W SkipListWeight, V HashObject] struct {
	// sentinel node
	// The head.levels[0].hForward is the first data node of skip-list.
	// From head.levels[1], they point to the levels node (i.e. the cache metadata)
	head SkipListNode[W, V]
	// sentinel node
	tail SkipListNode[W, V]
	// The real max level in used. And the max limitation is xSkipListMaxLevel.
	level *atomic.Int32
	len   *atomic.Int32
	cmp   SkipListComparator[W]
}

func (xsl *xSkipList[W, V]) Level() int32 {
	return xsl.level.Load()
}

func (xsl *xSkipList[W, V]) Len() int32 {
	return xsl.len.Load()
}

func (xsl *xSkipList[W, V]) ForEach(fn func(idx int64, weight W, obj V)) {
	var (
		x   SkipListNode[W, V]
		idx int64
	)
	x = xsl.head.levels()[0].horizontalForward()
	for x != nil {
		next := x.levels()[0].horizontalForward()
		fn(idx, x.Element().Weight(), x.Element().Object())
		idx++
		x = next
	}
}

func (xsl *xSkipList[W, V]) Insert(weight W, obj V) SkipListNode[W, V] {
	var (
		x          SkipListNode[W, V]
		update     [xSkipListMaxLevel]SkipListNode[W, V]
		levelIndex int32
	)
	x = xsl.head
	// Iteration from top to bottom.
	// First to iterate the cache and access the data finally.
	for levelIndex = xsl.level.Load() - 1; levelIndex >= 0; levelIndex-- { // move down level
		for x.levels()[levelIndex].horizontalForward() != nil {
			cur := x.levels()[levelIndex].horizontalForward()
			res := xsl.cmp(cur.Element().Weight(), weight)
			if res < 0 || (res == 0 && cur.Element().Object().Hash() < obj.Hash()) {
				x = cur // Changes the node iteration path to locate different node.
			} else if res == 0 && cur.Element().Object().Hash() == obj.Hash() {
				// Replaces the value.
				cur.setElement(&xSkipListElement[W, V]{weight: weight, object: obj})
				return cur
			} else {
				break
			}
		}
		// 1. (weight duplicated) If new element hash is lower than current node's (do pre-append to current node)
		// 2. (weight duplicated) If new element hash is greater than current node's (do append next to current node)
		// 3. (weight duplicated) If new element hash equals to current node's (replace element, because the hash
		//      value and element are not strongly correlated)
		// 4. (new weight) If new element is not exist, (do append next to current node)
		update[levelIndex] = x
	}

	// Each duplicated weight elements may contain its cache levels.
	// It means that duplicated weight elements query through the cache (O(logN))
	lvl := randomLevelV2(xSkipListMaxLevel, xsl.len.Load())
	if lvl > xsl.level.Load() {
		xsl.level.Store(lvl)
	}
	x = newXSkipListNode[W, V](lvl, weight, obj)
	for i := int32(0); i < lvl; i++ {
		cache := update[i]
		if cache == nil {
			break
		}
		x.levels()[i].setHorizontalForward(cache.levels()[i].horizontalForward())
		// may pre-append to adjust 2 elements' order
		cache.levels()[i].setHorizontalForward(x)
	}
	if update[0] == xsl.head {
		x.setVerticalBackward(nil)
	} else {
		x.setVerticalBackward(update[0])
	}
	if x.levels()[0].horizontalForward() == nil {
		xsl.tail = x
	} else {
		x.levels()[0].horizontalForward().setVerticalBackward(x)
	}
	xsl.len.Add(1)
	return x
}

//func (sl *xSkipList[W, V]) Remove(object E) SkipListNode[E] {
//	var (
//		update [xSkipListMaxLevel]SkipListNode[E]
//		x      SkipListNode[E]
//		idx    int
//	)
//	x = sl.head
//	for idx = sl.level - 1; idx >= 0; idx-- {
//		for x.levels()[idx].horizontalForward() != nil &&
//			sl.localCompareTo(x.levels()[idx].horizontalForward().GetObject(), object) < 0 {
//			x = x.levels()[idx].horizontalForward()
//		}
//		update[idx] = x
//	}
//
//	x = x.levels()[0].horizontalForward()
//	if x != nil && sl.localCompareTo(x.GetObject(), object) == 0 {
//		sl.deleteNode(x, update)
//		return x
//	}
//	return nil
//}
//
//func (sl *xSkipList[W, V]) Find(object E) SkipListNode[W, V] {
//	var (
//		x   SkipListNode[W, V]
//		idx int
//	)
//	x = sl.head
//	for idx = sl.level - 1; idx >= 0; idx-- {
//		for x.levels()[idx].horizontalForward() != nil &&
//			sl.localCompareTo(x.levels()[idx].horizontalForward().GetObject(), object) < 0 {
//			x = x.levels()[idx].horizontalForward()
//		}
//	}
//	x = x.levels()[0].horizontalForward()
//	if x != nil && sl.localCompareTo(x.GetObject(), object) == 0 {
//		return x
//	}
//	return nil
//}
//
//func (sl *xSkipList[W,V]) PopHead() (object E) {
//	x := sl.head
//	x = x.levels()[0].horizontalForward()
//	if x == nil {
//		return object
//	}
//	object = x.GetObject()
//	sl.Remove(object)
//	return
//}
//
//func (sl *xSkipList[W, V]) PopTail() (object E) {
//	x := sl.tail
//	if x == nil {
//		return *new(E)
//	}
//	object = x.GetObject()
//	sl.Remove(object)
//	return
//}
//
//
//func (sl *xSkipList[W, V]) deleteNode(x SkipListNode[W, V], update [32]SkipListNode[W, V]) {
//	var idx int32
//	for idx = 0; idx < sl.level.Load(); idx++ {
//		if update[idx].levels()[idx].horizontalForward() == x {
//			update[idx].levels()[idx].setSpan(update[idx].levels()[idx].Span() + x.levels()[idx].Span() - 1)
//			update[idx].levels()[idx].setHorizontalForward(x.levels()[idx].horizontalForward())
//		} else {
//			update[idx].levels()[idx].setSpan(update[idx].levels()[idx].Span() - 1)
//		}
//	}
//	if x.levels()[0].horizontalForward() != nil {
//		x.levels()[0].horizontalForward().setVerticalBackward(x.verticalBackward())
//	} else {
//		sl.tail = x.verticalBackward()
//	}
//	for sl.level > 1 && sl.head.levels()[sl.level-1].horizontalForward() == nil {
//		sl.level--
//	}
//	sl.len.Add(-1)
//}

func (xsl *xSkipList[W, V]) Free() {
	var (
		x, next SkipListNode[W, V]
		idx     int
	)
	x = xsl.head.levels()[0].horizontalForward()
	for x != nil {
		next = x.levels()[0].horizontalForward()
		x.Free()
		x = nil
		x = next
	}
	for idx = 0; idx < xSkipListMaxLevel; idx++ {
		xsl.head.levels()[idx].setHorizontalForward(nil)
	}
	xsl.tail = nil
	xsl.level = nil
	xsl.len = nil
}

func NewXSkipList[K SkipListWeight, V HashObject](cmp SkipListComparator[K]) SkipList[K, V] {
	sl := &xSkipList[K, V]{
		level: &atomic.Int32{},
		len:   &atomic.Int32{},
		cmp:   cmp,
	}
	sl.level.Store(1)
	sl.head = newXSkipListNode[K, V](xSkipListMaxLevel, *new(K), *new(V))
	// Initialization.
	// The head must be initialized with array element size with xSkipListMaxLevel.
	for i := 0; i < xSkipListMaxLevel; i++ {
		sl.head.levels()[i].setHorizontalForward(nil)
	}
	sl.head.setVerticalBackward(nil)
	sl.tail = nil
	return sl
}
