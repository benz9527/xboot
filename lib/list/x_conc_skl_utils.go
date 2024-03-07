package list

import (
	"runtime"
	"sync"
	"sync/atomic"
	"unsafe"

	"golang.org/x/sys/cpu"

	ibits "github.com/benz9527/xboot/lib/bits"
	"github.com/benz9527/xboot/lib/infra"
)

// Store the concurrent state.
type flagBits struct {
	bits uint32
}

// Bit flag set from 0 to 1.
func (f *flagBits) atomicSet(bits uint32) {
	for {
		old := atomic.LoadUint32(&f.bits)
		if old&bits != bits {
			n := old | bits
			if atomic.CompareAndSwapUint32(&f.bits, old, n) {
				return
			}
			continue
		}
		return
	}
}

// Bit flag set from 1 to 0.
func (f *flagBits) atomicUnset(bits uint32) {
	for {
		old := atomic.LoadUint32(&f.bits)
		check := old & bits
		if check != 0 {
			n := old ^ check
			if atomic.CompareAndSwapUint32(&f.bits, old, n) {
				return
			}
			continue
		}
		return
	}
}

func (f *flagBits) atomicIsSet(bit uint32) bool {
	return (atomic.LoadUint32(&f.bits) & bit) != 0
}

func (f *flagBits) atomicAreEqual(bits, expect uint32) bool {
	if ibits.HammingWeightBySWARV2[uint32](bits) <= 1 {
		panic("it is not a multi-bits")
	}
	n := 0
	for (bits>>n)&0x1 != 0x1 {
		n++
	}
	if n > 0 {
		expect <<= n
	}
	return (atomic.LoadUint32(&f.bits) & bits) == expect
}

func (f *flagBits) atomicLoadBits(bits uint32) uint32 {
	if ibits.HammingWeightBySWARV2[uint32](bits) <= 1 {
		panic("it is not a multi-bits")
	}
	n := 0
	for (bits>>n)&0x1 != 0x1 {
		n++
	}
	res := atomic.LoadUint32(&f.bits) & bits
	if n > 0 {
		res >>= n
	}
	return res
}

func (f *flagBits) setBitsAs(bits, target uint32) {
	if ibits.HammingWeightBySWARV2[uint32](bits) <= 1 {
		panic("it is not a multi-bits")
	}
	n := 0
	for (bits>>n)&0x1 != 0x1 {
		n++
	}
	if n > 0 {
		target <<= n
	}
	check := f.bits & bits
	f.bits = f.bits ^ check
	f.bits = f.bits | target
}

func (f *flagBits) isSet(bit uint32) bool {
	return (f.bits & bit) != 0
}

func (f *flagBits) loadBits(bits uint32) uint32 {
	if ibits.HammingWeightBySWARV2[uint32](bits) <= 1 {
		panic("it is not a multi-bits")
	}
	n := 0
	for (bits>>n)&0x1 != 0x1 {
		n++
	}
	res := f.bits & bits
	if n > 0 {
		res >>= n
	}
	return res
}

func (f *flagBits) areEqual(bits, expect uint32) bool {
	return f.loadBits(bits) == expect
}

const cacheLinePadSize = unsafe.Sizeof(cpu.CacheLinePad{})

// monotonicNonZeroID is a spin lock version generator.
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

func (c *monotonicNonZeroID) next() uint64 {
	// Golang atomic store with LOCK prefix, it means that
	// it implements the Happens-Before relationship.
	// But it is not clearly that atomic add satisfies the
	// Happens-Before relationship.
	// https://go.dev/ref/mem
	var v uint64
	if v = atomic.AddUint64(&c.val, 1); v == 0 {
		v = atomic.AddUint64(&c.val, 1)
	}
	return v
}

func newMonotonicNonZeroID() *monotonicNonZeroID {
	return &monotonicNonZeroID{val: 0}
}

type segmentedMutex interface {
	lock(version uint64)
	tryLock(version uint64) bool
	unlock(version uint64) bool
}

type mutexEnum uint8

const (
	xSklLockFree mutexEnum = iota
	goNativeMutex
)

func mutexFactory(e mutexEnum) segmentedMutex {
	switch e {
	case goNativeMutex:
		return new(goSyncMutex)
	case xSklLockFree:
		fallthrough
	default:
		return new(spinMutex)
	}
}

const (
	unlocked = 0
)

type spinMutex uint64

func (lock *spinMutex) lock(version uint64) {
	backoff := uint8(1)
	for !atomic.CompareAndSwapUint64((*uint64)(lock), unlocked, version) {
		if backoff <= 32 {
			for i := uint8(0); i < backoff; i++ {
				infra.ProcYield(20)
			}
		} else {
			runtime.Gosched()
		}
		backoff <<= 1
	}
}

func (lock *spinMutex) tryLock(version uint64) bool {
	return atomic.CompareAndSwapUint64((*uint64)(lock), unlocked, version)
}

func (lock *spinMutex) unlock(version uint64) bool {
	return atomic.CompareAndSwapUint64((*uint64)(lock), version, unlocked)
}

type goSyncMutex struct {
	mu sync.Mutex
}

func (m *goSyncMutex) lock(version uint64) {
	m.mu.Lock()
}

func (m *goSyncMutex) tryLock(version uint64) bool {
	return m.mu.TryLock()
}

func (m *goSyncMutex) unlock(version uint64) bool {
	m.mu.Unlock()
	return true
}
