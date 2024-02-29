package list

import (
	"log/slog"
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

func (lvl *skipListLevel[W, O]) horizontalForward() SkipListNode[W, O] {
	if lvl == nil {
		return nil
	}
	return lvl.hForward
}

func (lvl *skipListLevel[W, O]) setHorizontalForward(forward SkipListNode[W, O]) {
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

type xSkipListNode[W SkipListWeight, O HashObject] struct {
	// The cache part.
	// When current node work as data node, it doesn't contain levels metadata.
	// If a node is a level node, the cache is from levels[0], but it is differ
	//  to the sentinel's levels[0].
	lvls    []SkipListLevel[W, O]
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

func (node *xSkipListNode[W, O]) verticalBackward() SkipListNode[W, O] {
	if node == nil {
		return nil
	}
	return node.vBackward
}

func (node *xSkipListNode[W, O]) setVerticalBackward(backward SkipListNode[W, O]) {
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
	// sentinel node
	// The head.levels[0].hForward is the first data node of skip-list.
	// From head.levels[1], they point to the levels node (i.e. the cache metadata)
	head SkipListNode[W, O]
	// sentinel node
	tail SkipListNode[W, O]
	// The real max level in used. And the max limitation is xSkipListMaxLevel.
	level *atomic.Int32
	len   *atomic.Int32
	cmp   SkipListWeightComparator[W]
}

func (xsl *xSkipList[W, O]) Level() int32 {
	return xsl.level.Load()
}

func (xsl *xSkipList[W, O]) Len() int32 {
	return xsl.len.Load()
}

func (xsl *xSkipList[W, O]) ForEach(fn func(idx int64, weight W, obj O)) {
	var (
		x   SkipListNode[W, O]
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

func (xsl *xSkipList[W, O]) Insert(weight W, obj O) SkipListNode[W, O] {
	var (
		x          SkipListNode[W, O]
		traverse   [xSkipListMaxLevel]SkipListNode[W, O]
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
				cur.setElement(&xSkipListElement[W, O]{weight: weight, object: obj})
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
		traverse[levelIndex] = x
	}

	// Each duplicated weight elements may contain its cache levels.
	// It means that duplicated weight elements query through the cache (O(logN))
	lvl := randomLevelV2(xSkipListMaxLevel, xsl.len.Load())
	if lvl > xsl.level.Load() {
		for i := xsl.Level(); i < lvl; i++ {
			traverse[i] = xsl.head
		}
		xsl.level.Store(lvl)
	}
	x = newXSkipListNode[W, O](lvl, weight, obj)
	for i := int32(0); i < lvl; i++ {
		cache := traverse[i]
		if cache == nil {
			break
		}
		x.levels()[i].setHorizontalForward(cache.levels()[i].horizontalForward())
		// may pre-append to adjust 2 elements' order
		cache.levels()[i].setHorizontalForward(x)
	}
	if traverse[0] == xsl.head {
		x.setVerticalBackward(nil)
	} else {
		x.setVerticalBackward(traverse[0])
	}
	if x.levels()[0].horizontalForward() == nil {
		xsl.tail = x
	} else {
		x.levels()[0].horizontalForward().setVerticalBackward(x)
	}
	xsl.len.Add(1)
	return x
}

func (xsl *xSkipList[W, O]) RemoveFirst(weight W, cmp SkipListObjectMatcher[O]) SkipListElement[W, O] {
	var (
		x        SkipListNode[W, O]
		traverse [xSkipListMaxLevel]SkipListNode[W, O]
	)
	x = xsl.head
	for levelIndex := xsl.level.Load() - 1; levelIndex >= 0; levelIndex-- {
		for x.levels()[levelIndex].horizontalForward() != nil {
			cur := x.levels()[levelIndex].horizontalForward()
			res := xsl.cmp(cur.Element().Weight(), weight)
			// find predecessor node
			if res < 0 || (res == 0 && !cmp(cur.Element().Object())) {
				x = cur
			} else {
				break
			}
		}
		traverse[levelIndex] = x
	}

	if x == nil {
		slog.Warn("remove not found (pre)", "weight", weight)
		return nil // not found
	}

	slog.Info("remove (middle)", "weight", weight, "x w", x.Element().Weight(), "x obj", x.Element().Object())
	x = x.levels()[0].horizontalForward()
	if x != nil && xsl.cmp(x.Element().Weight(), weight) == 0 && cmp(x.Element().Object()) {
		// TODO lock-free
		xsl.removeNode(x, traverse)
		return x.Element()
	}
	slog.Warn("remove not found (post)", "weight", weight, "x weight", x.Element().Weight(), "x obj", x.Element().Object())
	return nil // not found
}

func (xsl *xSkipList[W, O]) removeNode(x SkipListNode[W, O], traverse [xSkipListMaxLevel]SkipListNode[W, O]) {
	var levelIndex int32
	for levelIndex = 0; levelIndex < xsl.level.Load(); levelIndex++ {
		if traverse[levelIndex].levels()[levelIndex].horizontalForward() == x {
			traverse[levelIndex].levels()[levelIndex].setHorizontalForward(x.levels()[levelIndex].horizontalForward())
		}
	}
	if x.levels()[0].horizontalForward() != nil {
		// Adjust the cache levels
		x.levels()[0].horizontalForward().setVerticalBackward(x.verticalBackward())
	} else {
		xsl.tail = x.verticalBackward()
	}
	for xsl.level.Load() > 1 && xsl.head.levels()[xsl.level.Load()-1].horizontalForward() == nil {
		xsl.level.Add(-1)
	}
	xsl.len.Add(-1)
}

func (xsl *xSkipList[W, O]) FindFirst(weight W, cmp SkipListObjectMatcher[O]) SkipListElement[W, O] {
	var (
		x SkipListNode[W, O]
	)
	x = xsl.head
outer:
	for levelIndex := xsl.level.Load() - 1; levelIndex >= 0; levelIndex-- {
		for x.levels()[levelIndex].horizontalForward() != nil {
			cur := x.levels()[levelIndex].horizontalForward()
			res := xsl.cmp(cur.Element().Weight(), weight)
			// find predecessor node
			if res < 0 || (res == 0 && !cmp(cur.Element().Object())) {
				x = cur
			} else if res == 0 && cmp(cur.Element().Object()) {
				break outer
			} else {
				break
			}
		}
		x = x.levels()[levelIndex].horizontalForward()
	}

	if x == nil {
		return nil // not found
	}

	x = x.levels()[0].horizontalForward()
	if x != nil && xsl.cmp(x.Element().Weight(), weight) == 0 && cmp(x.Element().Object()) {
		return x.Element()
	}
	return nil // not found
}

func (xsl *xSkipList[W, O]) PopHead() (e SkipListElement[W, O]) {
	x := xsl.head
	if x == nil {
		return e
	}
	if x = x.levels()[0].horizontalForward(); x == nil {
		return e
	}
	e = x.Element()
	slog.Info("pop head", "e w", e.Weight(), "e val", e.Object(), "e hash", e.Object().Hash())
	e1 := xsl.RemoveFirst(e.Weight(), func(obj O) bool {
		tarval := e.Object().Hash()
		slobjval := obj.Hash()
		slog.Info("target obj cmp sl obj", "tar hash", tarval, "sl hash", slobjval)
		return tarval == slobjval
	})
	if e1 == nil {
		panic("unable remove element")
	}
	return
}

func (xsl *xSkipList[W, O]) PopTail() (e SkipListElement[W, O]) {
	x := xsl.tail
	if x == nil {
		return
	}
	e = x.Element()
	xsl.RemoveFirst(e.Weight(), func(obj O) bool {
		tarval := e.Object().Hash()
		slobjval := obj.Hash()
		slog.Info("target obj cmp sl obj", "tar obj", e, "tar hash", tarval, "sl obj", obj, "sl hash", slobjval)
		return tarval == slobjval
	})
	return
}

func (xsl *xSkipList[W, O]) Free() {
	var (
		x, next SkipListNode[W, O]
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

func NewXSkipList[W SkipListWeight, O HashObject](cmp SkipListWeightComparator[W]) SkipList[W, O] {
	xsl := &xSkipList[W, O]{
		level: &atomic.Int32{},
		len:   &atomic.Int32{},
		cmp:   cmp,
	}
	xsl.level.Store(1)
	xsl.head = newXSkipListNode[W, O](xSkipListMaxLevel, *new(W), *new(O))
	// Initialization.
	// The head must be initialized with array element size with xSkipListMaxLevel.
	for i := 0; i < xSkipListMaxLevel; i++ {
		xsl.head.levels()[i].setHorizontalForward(nil)
	}
	xsl.head.setVerticalBackward(nil)
	xsl.tail = nil
	return xsl
}
