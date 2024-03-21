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
	xSkipListMaxLevel    = 32                           // level 0 is the data node level.
	XSkipListMaxSize     = 1<<(xSkipListMaxLevel-1) - 1 //  2^31 - 1 elements
	xSkipListProbability = 0.25                         // P = 1/4, a skip list node element has 1/4 probability to have a level
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
	ErrXSklIsEmpty              = errors.New("[x-skl] there is no element")
	errXsklRbtreeRedViolation   = errors.New("[x-skl] red-black tree violation")
	errXsklRbtreeBlackViolation = errors.New("[x-skl] red-black tree violation")
)

type XSklOption[K infra.OrderedKey, V comparable] interface {
	apply(*xSklOptions[K, V])
}

type xSklOptions[K infra.OrderedKey, V comparable] struct {
	keyComparator        infra.OrderedKeyComparator[K]
	valComparator        SklValComparator[V]
	optimisticLockVerGen id.UUIDGen
	dataNodeMode         xNodeMode
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
		defer skl.rwmu.Lock()
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

func NewSkl[K infra.OrderedKey, V comparable]() SkipList[K, V] {
	return nil
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
		defer skl.rwmu.Lock()
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
		defer skl.rwmu.Lock()
	}
	return skl.impl.PopHead()
}
func (skl *xSklDelegator[K, V]) RemoveFirst(key K) (SklElement[K, V], error) {
	if skl.rwmu != nil {
		skl.rwmu.Lock()
		defer skl.rwmu.Lock()
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
		defer skl.rwmu.Lock()
	}
	return skl.impl.RemoveAll(key)
}
func (skl *xSklDelegator[K, V]) RemoveIfMatched(key K, matcher func(V) bool) ([]SklElement[K, V], error) {
	if skl.rwmu != nil {
		skl.rwmu.Lock()
		defer skl.rwmu.Lock()
	}
	return skl.impl.RemoveIfMatched(key, matcher)
}

func NewXSkl[K infra.OrderedKey, V comparable]() XSkipList[K, V] {
	return nil
}
