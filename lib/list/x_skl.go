package list

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
