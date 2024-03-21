package list

import (
	"errors"
	"sync"

	"github.com/benz9527/xboot/lib/id"
	"github.com/benz9527/xboot/lib/infra"
)

// References:
// https://people.csail.mit.edu/shanir/publications/DCAS.pdf
// https://www.cl.cam.ac.uk/teaching/0506/Algorithms/skiplists.pdf
// https://people.csail.mit.edu/shanir/publications/LazySkipList.pdf
//
// github:
// classic: https://github.com/antirez/disque/blob/master/src/skiplist.h
// classic: https://github.com/antirez/disque/blob/master/src/skiplist.c
// zskiplist: https://github1s.com/redis/redis/blob/unstable/src/t_zset.c
// https://github.com/AdoptOpenJDK/openjdk-jdk11/blob/master/src/java.base/share/classes/java/util/concurrent/ConcurrentSkipListMap.java
// https://github.com/zhangyunhao116/skipmap
// https://github.com/liyue201/gostl
// https://github.com/chen3feng/stl4go
// https://github.com/slclub/skiplist/blob/master/skipList.go
// https://github.com/andy-kimball/arenaskl
// https://github.com/dgraph-io/badger/tree/master/skl
// https://github.com/BazookaMusic/goskiplist/blob/master/skiplist.go
// https://github.com/boltdb/bolt/blob/master/freelist.go
// https://github.com/xuezhaokun/150-algorithm/tree/master
//
// test:
// https://github.com/chen3feng/skiplist-survey
//
//
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

const (
	sklMaxLevel    = 32                     // level 0 is the data node level.
	sklMaxSize     = 1<<(sklMaxLevel-1) - 1 //  2^31 - 1 elements
	sklProbability = 0.25                   // P = 1/4, a skip list node element has 1/4 probability to have a level
)

var (
	ErrXSklNotFound             = errors.New("[x-skl] key or value not found")
	ErrXSklDisabledValReplace   = errors.New("[x-skl] value replace is disabled")
	ErrXSklConcRWLoadFailed     = errors.New("[x-skl] concurrent read-write causes load failed")
	ErrXSklConcRWLoadEmpty      = errors.New("[x-skl] concurrent read-write causes load empty")
	ErrXSklConcRemoving         = errors.New("[x-skl] concurrent removing")
	ErrXSklConcRemoveTryLock    = errors.New("[x-skl] concurrent remove acquires segmented lock failed")
	ErrXSklUnknownReason        = errors.New("[x-skl] unknown reason error")
	ErrXSklIsFull               = errors.New("[x-skl] is full")
	ErrXSklIsEmpty              = errors.New("[x-skl] is empty")
	errXSklRbtreeRedViolation   = errors.New("[x-skl] red-black tree violation")
	errXSklRbtreeBlackViolation = errors.New("[x-skl] red-black tree violation")
)

type SklType uint8

const (
	XComSkl SklType = iota
	XConcSkl
)

type sklOptions[K infra.OrderedKey, V comparable] struct {
	keyComparator          infra.OrderedKeyComparator[K]
	valComparator          SklValComparator[V]
	optimisticLockVerGen   id.UUIDGen
	randLevelGen           SklRand
	comRWMutex             *sync.RWMutex
	concDataNodeMode       *xNodeMode
	concSegMutexImpl       *mutexImpl
	sklType                SklType
	isConcRbtreeBorrowSucc bool
}

type SklOption[K infra.OrderedKey, V comparable] func(*sklOptions[K, V])

func WithSklRandLevelGen[K infra.OrderedKey, V comparable](gen SklRand) SklOption[K, V] {
	return func(opts *sklOptions[K, V]) {
		if opts.randLevelGen != nil {
			panic("[x-skl] random level generator has been set")
		}
		if gen == nil {
			panic("[x-skl] random level generator is nil")
		}
		opts.randLevelGen = gen
	}
}

func WithXComSklEnableConc[K infra.OrderedKey, V comparable]() SklOption[K, V] {
	return func(opts *sklOptions[K, V]) {
		switch opts.sklType {
		case XConcSkl:
			panic("[x-skl] x-conc-skl is disabled")
		default:

		}
		if opts.comRWMutex != nil {
			panic("[x-skl] x-com-skl conc has been enabled")
		}
		opts.comRWMutex = &sync.RWMutex{}
	}
}

func WithXComSklValComparator[K infra.OrderedKey, V comparable](cmp SklValComparator[V]) SklOption[K, V] {
	return func(opts *sklOptions[K, V]) {
		switch opts.sklType {
		case XConcSkl:
			panic("[x-skl] x-com-skl is disabled")
		default:

		}
		if opts.valComparator != nil {
			panic("[x-skl] x-com-skl value comparator has been set")
		} else if cmp == nil {
			panic("[x-skl] x-com-skl value comparator is nil")
		}
		opts.valComparator = cmp
	}
}

func WithXConcSklOptimisticVersionGen[K infra.OrderedKey, V comparable](verGen id.UUIDGen) SklOption[K, V] {
	return func(opts *sklOptions[K, V]) {
		switch opts.sklType {
		case XComSkl:
			panic("[x-skl] x-conc-skl is disabled")
		default:

		}
		if opts.optimisticLockVerGen != nil {
			panic("[x-skl] x-conc-skl version generator has been set")
		} else if verGen == nil {
			panic("[x-skl] x-conc-skl version generator is nil")
		}
		opts.optimisticLockVerGen = verGen
	}
}

func WithXConcSklDataNodeUniqueMode[K infra.OrderedKey, V comparable]() SklOption[K, V] {
	return func(opts *sklOptions[K, V]) {
		switch opts.sklType {
		case XComSkl:
			panic("[x-skl] x-conc-skl is disabled")
		default:

		}
		if opts.concDataNodeMode != nil {
			panic("[x-skl] x-conc-skl data node mode has been set")
		} else if opts.valComparator != nil {
			panic("[x-skl] x-conc-skl value comparator should not be set")
		}
		mode := unique
		opts.concDataNodeMode = &mode
	}
}

func WithXConcSklDataNodeLinkedListMode[K infra.OrderedKey, V comparable](cmp SklValComparator[V]) SklOption[K, V] {
	return func(opts *sklOptions[K, V]) {
		switch opts.sklType {
		case XComSkl:
			panic("[x-skl] x-conc-skl is disabled")
		default:

		}
		if opts.concDataNodeMode != nil {
			panic("[x-skl] x-conc-skl data node mode has been set")
		} else if opts.valComparator != nil {
			panic("[x-skl] x-conc-skl value comparator has been set")
		} else if cmp == nil {
			panic("[x-skl] x-conc-skl value comparator is nil")
		}

		mode := linkedList
		opts.concDataNodeMode = &mode
		opts.valComparator = cmp
	}
}

func WithXConcSklDataNodeRbtreeMode[K infra.OrderedKey, V comparable](cmp SklValComparator[V], borrowSucc ...bool) SklOption[K, V] {
	return func(opts *sklOptions[K, V]) {
		switch opts.sklType {
		case XComSkl:
			panic("[x-skl] x-conc-skl is disabled")
		default:

		}
		if opts.concDataNodeMode != nil {
			panic("[x-skl] x-conc-skl data node mode has been set")
		} else if opts.valComparator != nil {
			panic("[x-skl] x-conc-skl value comparator has been set")
		} else if cmp == nil {
			panic("[x-skl] x-conc-skl value comparator is nil")
		}

		opts.isConcRbtreeBorrowSucc = len(borrowSucc) > 0 && borrowSucc[0]
		mode := rbtree
		opts.concDataNodeMode = &mode
		opts.valComparator = cmp
	}
}

func WithSklConcBySpin[K infra.OrderedKey, V comparable]() SklOption[K, V] {
	return func(opts *sklOptions[K, V]) {
		switch opts.sklType {
		case XComSkl:
			panic("[x-skl] x-conc-skl is disabled")
		default:

		}
		if opts.concSegMutexImpl != nil {
			panic("[x-skl] x-conc-skl segement mutex has been set")
		}
		impl := xSklSpinMutex
		opts.concSegMutexImpl = &impl
	}
}

func WithSklConcByGoNative[K infra.OrderedKey, V comparable]() SklOption[K, V] {
	return func(opts *sklOptions[K, V]) {
		switch opts.sklType {
		case XComSkl:
			panic("[x-skl] x-conc-skl is disabled")
		default:

		}
		if opts.concSegMutexImpl != nil {
			panic("[x-skl] x-conc-skl segement mutex has been set")
		}
		impl := xSklGoMutex
		opts.concSegMutexImpl = &impl
	}
}

var (
	_ SkipList[uint8, struct{}] = (*sklDelegator[uint8, struct{}])(nil)
)

type sklDelegator[K infra.OrderedKey, V comparable] struct {
	rwmu *sync.RWMutex
	impl SkipList[K, V]
}

func (skl *sklDelegator[K, V]) Len() int64         { return skl.impl.Len() }
func (skl *sklDelegator[K, V]) Levels() int32      { return skl.impl.Levels() }
func (skl *sklDelegator[K, V]) IndexCount() uint64 { return skl.impl.IndexCount() }
func (skl *sklDelegator[K, V]) Insert(key K, val V, ifNotPresent ...bool) error {
	if skl.rwmu != nil {
		skl.rwmu.Lock()
		defer skl.rwmu.Unlock()
	}
	return skl.impl.Insert(key, val, ifNotPresent...)
}
func (skl *sklDelegator[K, V]) Foreach(action func(int64, SklIterationItem[K, V]) bool) {
	if skl.rwmu != nil {
		skl.rwmu.RLock()
		defer skl.rwmu.RUnlock()
	}
	skl.impl.Foreach(action)
}
func (skl *sklDelegator[K, V]) LoadFirst(key K) (SklElement[K, V], error) {
	if skl.rwmu != nil {
		skl.rwmu.RLock()
		defer skl.rwmu.RUnlock()
	}
	return skl.impl.LoadFirst(key)
}
func (skl *sklDelegator[K, V]) PeekHead() SklElement[K, V] { return skl.impl.PeekHead() }
func (skl *sklDelegator[K, V]) PopHead() (SklElement[K, V], error) {
	if skl.rwmu != nil {
		skl.rwmu.Lock()
		defer skl.rwmu.Unlock()
	}
	return skl.impl.PopHead()
}
func (skl *sklDelegator[K, V]) RemoveFirst(key K) (SklElement[K, V], error) {
	if skl.rwmu != nil {
		skl.rwmu.Lock()
		defer skl.rwmu.Unlock()
	}
	return skl.impl.RemoveFirst(key)
}

func NewSkl[K infra.OrderedKey, V comparable](typ SklType, cmp infra.OrderedKeyComparator[K], opts ...SklOption[K, V]) SkipList[K, V] {
	if cmp == nil {
		panic("[x-skl] key comparator is nil")
	}

	sklOpts := &sklOptions[K, V]{
		sklType:       typ,
		keyComparator: cmp,
	}
	for _, o := range opts {
		o(sklOpts)
	}

	if sklOpts.randLevelGen == nil {
		sklOpts.randLevelGen = randomLevelV2
	}

	switch typ {
	case XComSkl:
		if sklOpts.valComparator != nil {
			panic("[x-skl] x-com-skl init the unique data node mode but value comparator is set")
		}
	case XConcSkl:
		if *sklOpts.concDataNodeMode != unique || sklOpts.valComparator != nil {
			panic("[x-skl] x-conc-skl init the unique data node mode with the wrong mode or value comparator is set")
		}
		if sklOpts.optimisticLockVerGen == nil {
			gen, _ := id.MonotonicNonZeroID() // fallback to monotonic non-zero id
			sklOpts.optimisticLockVerGen = gen
		}
		if sklOpts.concSegMutexImpl == nil {
			impl := xSklSpinMutex // fallback to spin mutex
			sklOpts.concSegMutexImpl = &impl
		}
		if sklOpts.concDataNodeMode == nil {
			mode := unique // fallback to unique
			sklOpts.concDataNodeMode = &mode
		}
	default:
		panic("[x-skl] unknown skip list type")
	}

	d := &sklDelegator[K, V]{
		impl: skipListFactory(sklOpts),
	}
	if typ == XComSkl && sklOpts.comRWMutex != nil {
		d.rwmu = sklOpts.comRWMutex
	}
	return d
}

var (
	_ XSkipList[uint8, struct{}] = (*xSklDelegator[uint8, struct{}])(nil)
)

type xSklDelegator[K infra.OrderedKey, V comparable] struct {
	rwmu *sync.RWMutex
	impl XSkipList[K, V]
}

func (skl *xSklDelegator[K, V]) Len() int64         { return skl.impl.Len() }
func (skl *xSklDelegator[K, V]) Levels() int32      { return skl.impl.Levels() }
func (skl *xSklDelegator[K, V]) IndexCount() uint64 { return skl.impl.IndexCount() }
func (skl *xSklDelegator[K, V]) Insert(key K, val V, ifNotPresent ...bool) error {
	if skl.rwmu != nil {
		skl.rwmu.Lock()
		defer skl.rwmu.Unlock()
	}
	return skl.impl.Insert(key, val, ifNotPresent...)
}
func (skl *xSklDelegator[K, V]) Foreach(action func(int64, SklIterationItem[K, V]) bool) {
	if skl.rwmu != nil {
		skl.rwmu.RLock()
		defer skl.rwmu.RUnlock()
	}
	skl.impl.Foreach(action)
}
func (skl *xSklDelegator[K, V]) LoadFirst(key K) (SklElement[K, V], error) {
	if skl.rwmu != nil {
		skl.rwmu.RLock()
		defer skl.rwmu.RUnlock()
	}
	return skl.impl.LoadFirst(key)
}
func (skl *xSklDelegator[K, V]) PeekHead() SklElement[K, V] { return skl.impl.PeekHead() }
func (skl *xSklDelegator[K, V]) PopHead() (SklElement[K, V], error) {
	if skl.rwmu != nil {
		skl.rwmu.Lock()
		defer skl.rwmu.Unlock()
	}
	return skl.impl.PopHead()
}
func (skl *xSklDelegator[K, V]) RemoveFirst(key K) (SklElement[K, V], error) {
	if skl.rwmu != nil {
		skl.rwmu.Lock()
		defer skl.rwmu.Unlock()
	}
	return skl.impl.RemoveFirst(key)
}
func (skl *xSklDelegator[K, V]) LoadAll(key K) ([]SklElement[K, V], error) {
	if skl.rwmu != nil {
		skl.rwmu.RLock()
		defer skl.rwmu.RUnlock()
	}
	return skl.impl.LoadAll(key)
}
func (skl *xSklDelegator[K, V]) LoadIfMatched(weight K, matcher func(V) bool) ([]SklElement[K, V], error) {
	if skl.rwmu != nil {
		skl.rwmu.RLock()
		defer skl.rwmu.RUnlock()
	}
	return skl.impl.LoadIfMatched(weight, matcher)
}
func (skl *xSklDelegator[K, V]) RemoveAll(key K) ([]SklElement[K, V], error) {
	if skl.rwmu != nil {
		skl.rwmu.Lock()
		defer skl.rwmu.Unlock()
	}
	return skl.impl.RemoveAll(key)
}
func (skl *xSklDelegator[K, V]) RemoveIfMatched(key K, matcher func(V) bool) ([]SklElement[K, V], error) {
	if skl.rwmu != nil {
		skl.rwmu.Lock()
		defer skl.rwmu.Unlock()
	}
	return skl.impl.RemoveIfMatched(key, matcher)
}

func NewXSkl[K infra.OrderedKey, V comparable](typ SklType, cmp infra.OrderedKeyComparator[K], opts ...SklOption[K, V]) XSkipList[K, V] {
	if cmp == nil {
		panic("[x-skl] key comparator is nil")
	}

	sklOpts := &sklOptions[K, V]{
		sklType:       typ,
		keyComparator: cmp,
	}
	for _, o := range opts {
		o(sklOpts)
	}

	if sklOpts.randLevelGen == nil {
		sklOpts.randLevelGen = randomLevelV2
	}

	switch typ {
	case XComSkl:
		if sklOpts.valComparator == nil {
			panic("[x-skl] x-com-skl non-unique mode, the value comparator must be set")
		}
	case XConcSkl:
		if sklOpts.concDataNodeMode == nil {
			mode := rbtree // fallback to rbtree mode
			sklOpts.concDataNodeMode = &mode
		} else if *sklOpts.concDataNodeMode == unique {
			panic("[x-skl] x-conc-skl init the non-unique data node mode with the wrong mode")
		}

		if sklOpts.valComparator == nil {
			panic("[x-skl] x-conc-skl non-unique data node mode, the value comparator must be set")
		}

		if sklOpts.optimisticLockVerGen == nil {
			gen, _ := id.MonotonicNonZeroID() // fallback to monotonic non-zero id
			sklOpts.optimisticLockVerGen = gen
		}
		if sklOpts.concSegMutexImpl == nil {
			impl := xSklSpinMutex // fallback to spin mutex
			sklOpts.concSegMutexImpl = &impl
		}
	default:
		panic("[x-skl] unknown skip list type")
	}

	d := &xSklDelegator[K, V]{
		impl: skipListFactory(sklOpts),
	}
	if typ == XComSkl && sklOpts.comRWMutex != nil {
		d.rwmu = sklOpts.comRWMutex
	}
	return d
}

func skipListFactory[K infra.OrderedKey, V comparable](opts *sklOptions[K, V]) XSkipList[K, V] {
	var impl XSkipList[K, V]
	switch opts.sklType {
	case XComSkl:
		skl := &xComSkl[K, V]{
			// Start from 1 means the x-com-skl cache levels at least a one level is fixed
			levels:  1,
			nodeLen: 0,
			kcmp:    opts.keyComparator,
			vcmp:    opts.valComparator,
			rand:    opts.randLevelGen,
		}
		skl.head = newXComSklNode[K, V](sklMaxLevel, *new(K), *new(V))
		// Initialization.
		// The head must be initialized with array element size with xSkipListMaxLevel.
		for i := 0; i < sklMaxLevel; i++ {
			skl.head.levels()[i].setForward(nil)
		}
		skl.head.setBackward(nil)
		skl.tail = nil
		skl.pool = &sync.Pool{
			New: func() any {
				return make([]*xComSklNode[K, V], sklMaxLevel)
			},
		}
		impl = skl
	case XConcSkl:
		skl := &xConcSkl[K, V]{
			// Start from 1 means the x-com-skl cache levels at least a one level is fixed
			levels:  1,
			nodeLen: 0,
			head:    newXConcSklHead[K, V](*opts.concSegMutexImpl, unique),
			pool:    newXConcSklPool[K, V](),
			kcmp:    opts.keyComparator,
			vcmp:    opts.valComparator,
			idGen:   opts.optimisticLockVerGen,
			rand:    randomLevelV3,
			flags:   flagBits{},
		}
		skl.flags.setBitsAs(xConcSklMutexImplBits, uint32(*opts.concSegMutexImpl))
		skl.flags.setBitsAs(xConcSklXNodeModeBits, uint32(*opts.concDataNodeMode))
		if *opts.concDataNodeMode == rbtree && opts.isConcRbtreeBorrowSucc {
			skl.flags.set(xConcSklRbtreeRmBorrowFlagBit)
		}
		impl = skl
	default:
		panic("[x-skl] unknown skip list type")
	}
	return impl
}
