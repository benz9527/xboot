package list

import (
	"runtime"
	"testing"
	"unsafe"

	"github.com/benz9527/xboot/lib/infra"
)

type offsetIndices []uint32

type node2[K infra.OrderedKey, V any] struct {
	// If it is unique x-node type store value directly.
	// Otherwise, it is a sentinel node for linked-list or rbtree.
	root    *xNode[V]      // ptr size 8
	key     K              // string 16 int64 8
	indices offsetIndices  // size 24
	mu      segmentMutex   // interface size 16
	flags   flagBits       // size 4
	count   int64          // size 8
	level   uint32         // size 4
	ptr     uintptr        // size 8
	uptr    unsafe.Pointer // size 8
}

func TestNode2StructSize(t *testing.T) {
	x := node2[int64, []byte]{}
	t.Logf("root size: %d\n", unsafe.Sizeof(x.root))
	t.Logf("key size: %d\n", unsafe.Sizeof(x.key))
	t.Logf("indices size: %d\n", unsafe.Sizeof(x.indices))
	t.Logf("mutex size: %d\n", unsafe.Sizeof(x.mu))
	t.Logf("flagbits size: %d\n", unsafe.Sizeof(x.flags))
	t.Logf("count size: %d\n", unsafe.Sizeof(x.count))
	t.Logf("level size: %d\n", unsafe.Sizeof(x.level))
	t.Logf("ptr size: %d\n", unsafe.Sizeof(x.ptr))
	t.Logf("uptr size: %d\n", unsafe.Sizeof(x.uptr))

	uptrBytes := make([]byte, 2)
	s := "123"
	x.uptr = unsafe.Pointer(&s)
	uptrBytes = (*[2]byte)(x.uptr)[:]
	t.Logf("uptr bytes: %v\n", uptrBytes)
	uptr := unsafe.Pointer(&uptrBytes[0])
	t.Logf("uptr :%v\n", uptr)
	t.Logf("uptr value: %s\n", *(*string)(uptr))

	m := map[string]string{}
	m[s] = s
	s = ""
	runtime.GC()

	t.Logf("uptr value again after gc: %s\n", *(*string)(uptr))
	t.Logf("get key from map: %v\n", m[s])
	t.Logf("get key from map by origin value: %v\n", m["123"])

	x2 := node2[string, []byte]{}
	t.Logf("2 root size: %d\n", unsafe.Sizeof(x2.root))
	t.Logf("2 key size: %d\n", unsafe.Sizeof(x2.key))
	t.Logf("2 indices size: %d\n", unsafe.Sizeof(x2.indices))
	t.Logf("2 mutex size: %d\n", unsafe.Sizeof(x2.mu))
	t.Logf("2 flagbits size: %d\n", unsafe.Sizeof(x2.flags))
	t.Logf("2 count size: %d\n", unsafe.Sizeof(x2.count))
	t.Logf("2 level size: %d\n", unsafe.Sizeof(x2.level))

	x3 := node2[string, map[string][]byte]{}
	t.Logf("3 root size: %d\n", unsafe.Sizeof(x3.root))
	t.Logf("3 key size: %d\n", unsafe.Sizeof(x3.key))
	t.Logf("3 indices size: %d\n", unsafe.Sizeof(x3.indices))
	t.Logf("3 mutex size: %d\n", unsafe.Sizeof(x3.mu))
	t.Logf("3 flagbits size: %d\n", unsafe.Sizeof(x3.flags))
	t.Logf("3 count size: %d\n", unsafe.Sizeof(x3.count))
	t.Logf("3 level size: %d\n", unsafe.Sizeof(x3.level))

	x4 := xConcSklV2Node[string, map[string][]byte]{}
	t.Logf("x4 size: %v\n", unsafe.Sizeof(x4))
}
