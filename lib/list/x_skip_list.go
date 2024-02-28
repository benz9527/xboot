package list

import (
	"math"
	"math/bits"
	randv2 "math/rand/v2"
	"sync/atomic"
	"time"
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

// SkipListRandomLevel is the skip list level element.
// Dynamic level calculation.
func SkipListRandomLevel(random func() uint64, maxLevel int) int {
	// goland math random (math.Float64()) contains global mutex lock
	// Ref
	// https://cs.opensource.google/go/go/+/refs/tags/go1.21.5:src/math/rand/rand.go
	// https://cs.opensource.google/go/go/+/refs/tags/go1.21.5:src/math/bits/bits.go
	// 1. Avoid to use global mutex lock
	// 2. Avoid to generate random number each time
	total := uint64(1)<<maxLevel - 1 // maxLevel => n, 2^n -1, there will be 2^n-1 elements in the skip list
	rest := random() % total
	// Bits right shift equals to manipulate a high level bit
	// Calculate the minimum bits of the random number
	level := maxLevel - bits.Len64(rest) + 1
	return level
}

func MaxLevels(totalElements int64, P float64) int {
	// Ref https://www.cl.cam.ac.uk/teaching/2005/Algorithms/skiplists.pdf
	// MaxLevels = log(1/P) * log(totalElements)
	// P = 1/4, totalElements = 2^32 - 1
	return int(math.Ceil(math.Log(1/P) * math.Log(float64(totalElements))))
}

var (
	_ SkipListNodeElement[struct{}] = (*skipListNodeElement[struct{}])(nil) // Type check assertion
	_ SkipListLevel[struct{}]       = (*skipListLevel[struct{}])(nil)       // Type check assertion
	_ SkipList[struct{}]            = (*xSkipList[struct{}])(nil)           // Type check assertion
)

type skipListNodeElement[E comparable] struct {
	object E
	// The next node element in the vertical direction.
	// A node with level 0 without any next node element at all usually.
	// A node work as level node (i.e. the cache node) must contain vBackward metadata.
	vBackward SkipListNodeElement[E]
	// The cache part.
	// When current node work as data node, it doesn't contain levels metadata.
	// If a node is a level node, the cache is from levels[0], but it is differ
	//  to the sentinel's levels[0].
	levels []SkipListLevel[E]
}

func (e *skipListNodeElement[E]) GetObject() E {
	return e.object
}

func (e *skipListNodeElement[E]) GetVerticalBackward() SkipListNodeElement[E] {
	return e.vBackward
}

func (e *skipListNodeElement[E]) SetVerticalBackward(backward SkipListNodeElement[E]) {
	e.vBackward = backward
}

func (e *skipListNodeElement[E]) GetLevels() []SkipListLevel[E] {
	return e.levels
}

func (e *skipListNodeElement[E]) Free() {
	e.object = *new(E)
	e.vBackward = nil
	e.levels = nil
}

func newSkipListNodeElement[E comparable](level int, obj E) SkipListNodeElement[E] {
	e := &skipListNodeElement[E]{
		object: obj,
		// Fill zero to all level span.
		// Initialization.
		levels: make([]SkipListLevel[E], level),
	}
	for i := 0; i < level; i++ {
		e.levels[i] = newSkipListLevel[E](0, nil)
	}
	return e
}

type skipListLevel[E comparable] struct {
	// The next node element in horizontal direction.
	hForward SkipListNodeElement[E]
	span     int64
}

func (lvl *skipListLevel[E]) GetSpan() int64 {
	return atomic.LoadInt64(&lvl.span)
}

func (lvl *skipListLevel[E]) SetSpan(span int64) {
	atomic.StoreInt64(&lvl.span, span)
}

func (lvl *skipListLevel[E]) GetHorizontalForward() SkipListNodeElement[E] {
	return lvl.hForward
}

func (lvl *skipListLevel[E]) SetHorizontalForward(forward SkipListNodeElement[E]) {
	lvl.hForward = forward
}

func newSkipListLevel[E comparable](span int64, forward SkipListNodeElement[E]) SkipListLevel[E] {
	return &skipListLevel[E]{
		span:     span,
		hForward: forward,
	}
}

// This is a sentinel node and used to point to nodes in memory.
type xSkipList[E comparable] struct {
	// The head.levels[0].hForward is the first data node of skip-list.
	// From head.levels[1], they point to the levels node (i.e. the cache metadata)
	head            SkipListNodeElement[E]
	tail            SkipListNodeElement[E] // sentinel node
	localCompareTo  compareTo[E]
	randomGenerator *randv2.Rand
	// The real max level in used. And the max limitation is xSkipListMaxLevel.
	level int
	len   int64
}

func NewClassicSkipList[E comparable](compareTo compareTo[E]) SkipList[E] {
	sl := &xSkipList[E]{
		level:          1,
		len:            0,
		localCompareTo: compareTo,
		randomGenerator: randv2.New(
			randv2.NewPCG(
				uint64(time.Now().UnixNano()),
				uint64(time.Now().UnixNano()),
			)),
	}
	sl.head = newSkipListNodeElement[E](xSkipListMaxLevel, *new(E))
	// Initialization.
	// The head must be initialized with array element size with xSkipListMaxLevel.
	for i := 0; i < xSkipListMaxLevel; i++ {
		sl.head.GetLevels()[i].SetSpan(0)
		sl.head.GetLevels()[i].SetHorizontalForward(nil)
	}
	sl.head.SetVerticalBackward(nil)
	sl.tail = nil
	return sl
}

func (sl *xSkipList[E]) randomLevel() int {
	level := 1
	for float64(sl.randomGenerator.Int64()&0xFFFF) < xSkipListProbability*0xFFFF {
		level += 1
	}
	if level < xSkipListMaxLevel {
		return level
	}
	return xSkipListMaxLevel
}

func (sl *xSkipList[E]) GetLevel() int {
	return sl.level
}

func (sl *xSkipList[E]) Len() int64 {
	return atomic.LoadInt64(&sl.len)
}

func (sl *xSkipList[E]) Insert(obj E) SkipListNodeElement[E] {
	var (
		update     [xSkipListMaxLevel]SkipListNodeElement[E]
		x          SkipListNodeElement[E]
		levelSpans [xSkipListMaxLevel]int64
		levelIdx   int
		level      int
	)
	// Iteration from the sentinel head.
	x = sl.head
	// Starts at the top level to find the insertion position of the current new element
	for levelIdx = sl.level - 1; levelIdx >= 0; levelIdx-- {
		if levelIdx == sl.level-1 {
			// So the 1st loop, it's going to have to go over here,
			//   and the current spacing at the top level is 0.
			// Because we don't calculate the spacing at level 0, we need to go back 1 level in height
			levelSpans[levelIdx] = 0
		} else {
			// Starting from the 2nd loop, the spacing of the current level
			//   is the spacing of the index node in the previous level.
			levelSpans[levelIdx] = levelSpans[levelIdx+1]
		}

		for x.GetLevels()[levelIdx].GetHorizontalForward() != nil &&
			sl.localCompareTo(x.GetLevels()[levelIdx].GetHorizontalForward().GetObject(), obj) < 0 {
			levelSpans[levelIdx] += x.GetLevels()[levelIdx].GetSpan()
			x = x.GetLevels()[levelIdx].GetHorizontalForward()
		}
		update[levelIdx] = x
	}

	if x.GetLevels()[0].GetHorizontalForward() != nil &&
		sl.localCompareTo(x.GetLevels()[0].GetHorizontalForward().GetObject(), obj) == 0 {
		return nil
	}

	level = sl.randomLevel()
	if level > sl.level {
		for lvl := sl.level; lvl < level; lvl++ {
			levelSpans[lvl] = 0
			update[lvl] = sl.head
			update[lvl].GetLevels()[lvl].SetSpan(sl.len)
		}
		sl.level = level
	}

	x = newSkipListNodeElement[E](level, obj)
	for levelIdx = 0; levelIdx < level; levelIdx++ {
		x.GetLevels()[levelIdx].SetHorizontalForward(update[levelIdx].GetLevels()[levelIdx].GetHorizontalForward())
		update[levelIdx].GetLevels()[levelIdx].SetHorizontalForward(x)

		x.GetLevels()[levelIdx].SetSpan(update[levelIdx].GetLevels()[levelIdx].GetSpan() - (levelSpans[0] - levelSpans[levelIdx]))
		update[levelIdx].GetLevels()[levelIdx].SetSpan(levelSpans[0] - levelSpans[levelIdx] + 1)
	}

	for levelIdx = level; levelIdx < sl.level; levelIdx++ {
		update[levelIdx].GetLevels()[levelIdx].SetSpan(update[levelIdx].GetLevels()[levelIdx].GetSpan() + 1)
	}

	if update[0] == sl.head {
		x.SetVerticalBackward(nil)
	} else {
		x.SetVerticalBackward(update[0])
	}

	if x.GetLevels()[0].GetHorizontalForward() != nil {
		x.GetLevels()[0].GetHorizontalForward().SetVerticalBackward(x)
	} else {
		sl.tail = x
	}
	sl.len++
	return x
}

func (sl *xSkipList[E]) Remove(obj E) SkipListNodeElement[E] {
	var (
		update [xSkipListMaxLevel]SkipListNodeElement[E]
		x      SkipListNodeElement[E]
		idx    int
	)
	x = sl.head
	for idx = sl.level - 1; idx >= 0; idx-- {
		for x.GetLevels()[idx].GetHorizontalForward() != nil &&
			sl.localCompareTo(x.GetLevels()[idx].GetHorizontalForward().GetObject(), obj) < 0 {
			x = x.GetLevels()[idx].GetHorizontalForward()
		}
		update[idx] = x
	}

	x = x.GetLevels()[0].GetHorizontalForward()
	if x != nil && sl.localCompareTo(x.GetObject(), obj) == 0 {
		sl.deleteNode(x, update)
		return x
	}
	return nil
}

func (sl *xSkipList[E]) deleteNode(x SkipListNodeElement[E], update [32]SkipListNodeElement[E]) {
	var idx int
	for idx = 0; idx < sl.level; idx++ {
		if update[idx].GetLevels()[idx].GetHorizontalForward() == x {
			update[idx].GetLevels()[idx].SetSpan(update[idx].GetLevels()[idx].GetSpan() + x.GetLevels()[idx].GetSpan() - 1)
			update[idx].GetLevels()[idx].SetHorizontalForward(x.GetLevels()[idx].GetHorizontalForward())
		} else {
			update[idx].GetLevels()[idx].SetSpan(update[idx].GetLevels()[idx].GetSpan() - 1)
		}
	}
	if x.GetLevels()[0].GetHorizontalForward() != nil {
		x.GetLevels()[0].GetHorizontalForward().SetVerticalBackward(x.GetVerticalBackward())
	} else {
		sl.tail = x.GetVerticalBackward()
	}
	for sl.level > 1 && sl.head.GetLevels()[sl.level-1].GetHorizontalForward() == nil {
		sl.level--
	}
	sl.len--
}

func (sl *xSkipList[E]) Find(obj E) SkipListNodeElement[E] {
	var (
		x   SkipListNodeElement[E]
		idx int
	)
	x = sl.head
	for idx = sl.level - 1; idx >= 0; idx-- {
		for x.GetLevels()[idx].GetHorizontalForward() != nil &&
			sl.localCompareTo(x.GetLevels()[idx].GetHorizontalForward().GetObject(), obj) < 0 {
			x = x.GetLevels()[idx].GetHorizontalForward()
		}
	}
	x = x.GetLevels()[0].GetHorizontalForward()
	if x != nil && sl.localCompareTo(x.GetObject(), obj) == 0 {
		return x
	}
	return nil
}

func (sl *xSkipList[E]) PopHead() (obj E) {
	x := sl.head
	x = x.GetLevels()[0].GetHorizontalForward()
	if x == nil {
		return obj
	}
	obj = x.GetObject()
	sl.Remove(obj)
	return
}

func (sl *xSkipList[E]) PopTail() (obj E) {
	x := sl.tail
	if x == nil {
		return *new(E)
	}
	obj = x.GetObject()
	sl.Remove(obj)
	return
}

func (sl *xSkipList[E]) Free() {
	var (
		x, next SkipListNodeElement[E]
		idx     int
	)
	x = sl.head.GetLevels()[0].GetHorizontalForward()
	for x != nil {
		next = x.GetLevels()[0].GetHorizontalForward()
		x.Free()
		x = nil
		x = next
	}
	for idx = 0; idx < xSkipListMaxLevel; idx++ {
		sl.head.GetLevels()[idx].SetHorizontalForward(nil)
		sl.head.GetLevels()[idx].SetSpan(0)
	}
	sl.tail = nil
	sl.level = 0
	sl.len = 0
}

func (sl *xSkipList[E]) ForEach(fn func(idx int64, v E)) {
	var (
		x   SkipListNodeElement[E]
		idx int64
	)
	x = sl.head.GetLevels()[0].GetHorizontalForward()
	for x != nil {
		next := x.GetLevels()[0].GetHorizontalForward()
		fn(idx, x.GetObject())
		idx++
		x = next
	}
}
