package list

import (
	"runtime"
	"sync"
	"sync/atomic"

	ibits "github.com/benz9527/xboot/lib/bits"
	"github.com/benz9527/xboot/lib/infra"
)

var (
	_ SkipListElement[uint8, uint8]       = (*xSkipListElement[uint8, uint8])(nil)
	_ SkipListIterationItem[uint8, uint8] = (*xSkipListIterationItem[uint8, uint8])(nil)
)

type xSkipListElement[K infra.OrderedKey, V comparable] struct {
	key K
	val V
}

func (e *xSkipListElement[K, V]) Key() K {
	return e.key
}

func (e *xSkipListElement[K, V]) Val() V {
	return e.val
}

type xSkipListIterationItem[K infra.OrderedKey, V comparable] struct {
	keyFn           func() K
	valFn           func() V
	nodeLevelFn     func() uint32
	nodeItemCountFn func() int64
}

func (x *xSkipListIterationItem[K, V]) Key() K               { return x.keyFn() }
func (x *xSkipListIterationItem[K, V]) Val() V               { return x.valFn() }
func (x *xSkipListIterationItem[K, V]) NodeLevel() uint32    { return x.nodeLevelFn() }
func (x *xSkipListIterationItem[K, V]) NodeItemCount() int64 { return x.nodeItemCountFn() }

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

type segmentedMutex interface {
	lock(version uint64)
	tryLock(version uint64) bool
	unlock(version uint64) bool
}

type mutexImpl uint8

const (
	xSklLockFree mutexImpl = iota
	goNativeMutex
)

func mutexFactory(e mutexImpl) segmentedMutex {
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
