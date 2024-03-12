package id

import (
	"golang.org/x/sys/cpu"
	"strconv"
	"sync/atomic"
	"unsafe"
)

const cacheLinePadSize = unsafe.Sizeof(cpu.CacheLinePad{})

// monotonicNonZeroID is an ID generator.
// Only increase, if it overflows, it will be reset to 1.
// Occupy a whole cache line (flag+tag+data), and a cache line data is 64 bytes.
// L1D cache: cat /sys/devices/system/cpu/cpu0/cache/index0/coherency_line_size
// L1I cache: cat /sys/devices/system/cpu/cpu0/cache/index1/coherency_line_size
// L2 cache: cat /sys/devices/system/cpu/cpu0/cache/index2/coherency_line_size
// L3 cache: cat /sys/devices/system/cpu/cpu0/cache/index3/coherency_line_size
// MESI (Modified-Exclusive-Shared-Invalid)
// RAM data -> L3 cache -> L2 cache -> L1 cache -> CPU register.
// CPU register (cache hit) -> L1 cache -> L2 cache -> L3 cache -> RAM data.
type monotonicNonZeroID struct {
	// sequence consistency data race free program
	// avoid load into cpu cache will be broken by others data
	// to compose a data race cache line
	_   [cacheLinePadSize - unsafe.Sizeof(*new(uint64))]byte // padding for CPU cache line, avoid false sharing
	val uint64                                               // space waste to exchange for performance
	_   [cacheLinePadSize - unsafe.Sizeof(*new(uint64))]byte // padding for CPU cache line, avoid false sharing
}

func (id *monotonicNonZeroID) next() uint64 {
	// Golang atomic store with LOCK prefix, it means that
	// it implements the Happens-Before relationship.
	// But it is not clearly that atomic add satisfies the
	// Happens-Before relationship.
	// https://go.dev/ref/mem
	var v uint64
	if v = atomic.AddUint64(&id.val, 1); v == 0 {
		v = atomic.AddUint64(&id.val, 1)
	}
	return v
}

func MonotonicNonZeroID() (Generator, error) {
	src := &monotonicNonZeroID{val: 0}
	id := new(defaultID)
	id.number = func() uint64 {
		return src.next()
	}
	id.str = func() string {
		return strconv.FormatUint(src.next(), 10)
	}
	return id, nil
}
