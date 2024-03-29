package queue

import (
	"sync/atomic"
	"unsafe"

	"golang.org/x/sys/cpu"
)

// 2 ways for lock-free programming
// Ref https://hedzr.com/golang/nolock/two-nolock-skills-in-go/
// 1. Structure entry mode: always copy the whole structure (independent element), for example, golang slog.
// 2. Bumper loop mode: force to synchronize the whole process. Only works for low concurrency.
//    We have to avoid a slow handle process which will result in block next handle process,
//    events heap up or event loss.
//    For example, golang server/client conn.
//    Bumper loop mode is appropriate for event distribution. Low concurrency means that
//    the frequency is higher than 80ms per event.

var (
	_ RingBufferCursor = (*rbCursor)(nil)
)

const cacheLinePadSize = unsafe.Sizeof(cpu.CacheLinePad{})

// rbCursor is a cursor for xRingBuffer.
// Only increase, if it overflows, it will be reset to 0.
// Occupy a whole cache line (flag+tag+data), and a cache line data is 64 bytes.
// L1D cache: cat /sys/devices/system/cpu/cpu0/cache/index0/coherency_line_size
// L1I cache: cat /sys/devices/system/cpu/cpu0/cache/index1/coherency_line_size
// L2 cache: cat /sys/devices/system/cpu/cpu0/cache/index2/coherency_line_size
// L3 cache: cat /sys/devices/system/cpu/cpu0/cache/index3/coherency_line_size
// MESI (Modified-Exclusive-Shared-Invalid)
// RAM data -> L3 cache -> L2 cache -> L1 cache -> CPU register.
// CPU register (cache hit) -> L1 cache -> L2 cache -> L3 cache -> RAM data.
type rbCursor struct {
	// sequence consistency data race free program
	// avoid load into cpu cache will be broken by others data
	// to compose a data race cache line
	_   [cacheLinePadSize - unsafe.Sizeof(*new(uint64))]byte // padding for CPU cache line, avoid false sharing
	val uint64                                               // space waste to exchange for performance
	_   [cacheLinePadSize - unsafe.Sizeof(*new(uint64))]byte // padding for CPU cache line, avoid false sharing
}

func NewXRingBufferCursor() RingBufferCursor {
	return &rbCursor{}
}

func (c *rbCursor) Next() uint64 {
	// Golang atomic store with LOCK prefix, it means that
	// it implements the Happens-Before relationship.
	// But it is not clearly that atomic add satisfies the
	// Happens-Before relationship.
	// https://go.dev/ref/mem
	return atomic.AddUint64(&c.val, 1)
}

func (c *rbCursor) NextN(n uint64) uint64 {
	return atomic.AddUint64(&c.val, n)
}

func (c *rbCursor) Load() uint64 {
	// Golang atomic load does not promise the Happens-Before
	return atomic.LoadUint64(&c.val)
}

func (c *rbCursor) CompareAndSwap(old, new uint64) bool {
	return atomic.CompareAndSwapUint64(&c.val, old, new)
}
