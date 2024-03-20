package list

import (
	"errors"
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
	ErrXSklNotFound           = errors.New("[x-skl] key or value not found")
	ErrXSklDisabledValReplace = errors.New("[x-skl] value replace is disabled")
	ErrXSklConcRWLoadFailed   = errors.New("[x-skl] concurrent read-write causes load failed")
	ErrXSklConcRWLoadEmpty    = errors.New("[x-skl] concurrent read-write causes load empty")
	ErrXSklConcRemoving       = errors.New("[x-skl] concurrent removing")
	ErrXSklConcRemoveTryLock  = errors.New("[x-skl] concurrent remove acquires segmented lock failed")
	ErrXSklUnknownReason      = errors.New("[x-skl] unknown reason error")
	ErrXSklIsFull             = errors.New("[x-skl] is full")
	ErrXSklIsEmpty            = errors.New("[x-skl] there is no element")
)
