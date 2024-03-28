package list

import (
	"reflect"
	"runtime"
	"sync"
	"sync/atomic"
	"unsafe"

	ibits "github.com/benz9527/xboot/lib/bits"
	"github.com/benz9527/xboot/lib/infra"
)

var insertReplaceDisabled = []bool{false}

var (
	_ SklElement[uint8, uint8]       = (*xSklElement[uint8, uint8])(nil)
	_ SklIterationItem[uint8, uint8] = (*xSklIter[uint8, uint8])(nil)
)

type xSklElement[K infra.OrderedKey, V any] struct {
	key K
	val V
}

func (e *xSklElement[K, V]) Key() K {
	return e.key
}

func (e *xSklElement[K, V]) Val() V {
	return e.val
}

type xSklIter[K infra.OrderedKey, V any] struct {
	keyFn           func() K
	valFn           func() V
	nodeLevelFn     func() uint32
	nodeItemCountFn func() int64
}

func (x *xSklIter[K, V]) Key() K               { return x.keyFn() }
func (x *xSklIter[K, V]) Val() V               { return x.valFn() }
func (x *xSklIter[K, V]) NodeLevel() uint32    { return x.nodeLevelFn() }
func (x *xSklIter[K, V]) NodeItemCount() int64 { return x.nodeItemCountFn() }

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

func (f *flagBits) set(bits uint32) {
	f.bits = f.bits | bits
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

func (f *flagBits) atomicSetBitsAs(bits, target uint32) {
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

	for {
		old := atomic.LoadUint32(&f.bits)
		check := old & bits
		if check != 0 {
			n := old ^ check
			n = n | target
			if atomic.CompareAndSwapUint32(&f.bits, old, n) {
				return
			}
			continue
		}
		return
	}
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

type segmentMutex interface {
	lock(version uint64)
	tryLock(version uint64) bool
	unlock(version uint64) bool
}

type mutexImpl uint8

const (
	xSklSpinMutex mutexImpl = 1 + iota // Lock-free, spin-lock, optimistic-lock
	xSklFakeMutex                      // No lock
)

func (mu mutexImpl) String() string {
	switch mu {
	case xSklSpinMutex:
		return "spin"
	case xSklFakeMutex:
		return "fake"
	default:
		return "unknown"
	}
}

type spinMutex uint64

func (m *spinMutex) lock(version uint64) {
	backoff := uint8(1)
	for !atomic.CompareAndSwapUint64((*uint64)(m), unlocked, version) {
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

func (m *spinMutex) tryLock(version uint64) bool {
	return atomic.CompareAndSwapUint64((*uint64)(m), unlocked, version)
}

func (m *spinMutex) unlock(version uint64) bool {
	return atomic.CompareAndSwapUint64((*uint64)(m), version, unlocked)
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

type fakeMutex struct{}

func (m *fakeMutex) lock(version uint64)         {}
func (m *fakeMutex) tryLock(version uint64) bool { return true }
func (m *fakeMutex) unlock(version uint64) bool  { return true }

// References:
// https://github.com/ortuman/nuke
// https://github.com/dgraph-io/badger/blob/master/skl/arena.go

type arenaBuffer struct {
	ptr      unsafe.Pointer
	offset   uintptr // current index offset
	cap      uintptr // capacity, indicates how many objects could be stored
	objSize  uintptr // fixed object size
	objAlign uintptr // fixed object alignment
}

func (buf *arenaBuffer) availableBytes() uintptr {
	return buf.cap*buf.objSize - buf.offset
}

func (buf *arenaBuffer) allocate() (unsafe.Pointer, bool) {
	if /* lazy init */ buf.ptr == nil {
		buffer := make([]byte, buf.cap*buf.objSize)
		buf.ptr = unsafe.Pointer(unsafe.SliceData(buffer))
	}
	alignOffset := uintptr(0)
	for alignedPtr := uintptr(buf.ptr) + buf.offset; alignedPtr%buf.objAlign != 0; alignedPtr++ {
		alignOffset++
	}
	allocatedSize := buf.objSize + alignOffset

	if /* scale */ buf.availableBytes() < allocatedSize {
		return nil, false
	}

	ptr := unsafe.Pointer(uintptr(buf.ptr) + buf.offset + alignOffset)
	buf.offset += allocatedSize

	// Translated into runtime.memclrNoHeapPointers by compiler.
	// An assembler optimized implementation.
	// go/src/runtime/memclr_$GOARCH.s (since https://codereview.appspot.com/137880043)
	bytes := unsafe.Slice((*byte)(ptr), buf.objSize)
	for i := range bytes {
		bytes[i] = 0
	}
	return ptr, true
}

func (buf *arenaBuffer) reset() {
	if buf.offset == 0 {
		return
	}
	// Overwrite allow
	buf.offset = 0
}

func (buf *arenaBuffer) free() {
	buf.reset()
	buf.ptr = nil
}

func newArenaBuffer(cap, size, alignment uintptr) *arenaBuffer {
	return &arenaBuffer{
		cap:      cap,
		objSize:  size,
		objAlign: alignment,
		offset:   uintptr(0),
	}
}

// T must not be a pointer type.
type autoGrowthArena[T any] struct {
	buffers  []*arenaBuffer
	recycled []*T
}

func (arena *autoGrowthArena[T]) bufLen() int {
	return len(arena.buffers)
}

func (arena *autoGrowthArena[T]) recLen() int {
	return len(arena.recycled)
}

func (arena *autoGrowthArena[T]) objLen() uint64 {
	l := uint64(0)
	for i := 0; i < len(arena.buffers); i++ {
		if arena.buffers[i].availableBytes() <= uintptr(0) {
			l += uint64(arena.buffers[i].cap)
		} else {
			l += uint64(arena.buffers[i].offset / arena.buffers[i].objSize)
		}
	}
	return l
}

func (arena *autoGrowthArena[T]) allocate() (*T, bool) {
	var ptr unsafe.Pointer
	allocated := false
	for i := 0; i < len(arena.buffers); i++ {
		if arena.buffers[i].availableBytes() <= uintptr(0) {
			continue
		} else {
			ptr, allocated = arena.buffers[i].allocate()
			break
		}
	}
	rl := len(arena.recycled)
	if !allocated && rl <= 0 {
		buf := newArenaBuffer(
			arena.buffers[0].cap,
			arena.buffers[0].objSize,
			arena.buffers[0].objAlign,
		)
		arena.buffers = append(arena.buffers, buf)
		ptr, allocated = buf.allocate()
	} else if !allocated && rl > 0 {
		allocated = true
		p := arena.recycled[0]
		arena.recycled = arena.recycled[1:]
		return p, allocated
	}
	if !allocated || ptr == nil {
		return nil, false
	}
	return (*T)(ptr), allocated
}

func (arena *autoGrowthArena[T]) free() {
	for _, buf := range arena.buffers {
		buf.free()
	}
}

func (arena *autoGrowthArena[T]) reset(indices ...int) {
	l := len(arena.buffers)
	if len(indices) > 0 {
		for i := range indices {
			if i < l {
				arena.buffers[i].reset()
			}
		}
		return
	}
	for _, buf := range arena.buffers {
		buf.reset()
	}
}

func (arena *autoGrowthArena[T]) recycle(objs ...*T) {
	arena.recycled = append(arena.recycled, objs...)
}

func newAutoGrowthArena[T any](capPerBuf, initRecycleCap uint32) *autoGrowthArena[T] {
	o := *new(T)
	if reflect.TypeOf(o).Kind() == reflect.Ptr {
		panic("forbid to pass ptr generic type for auto growth arena")
	}

	objSize, objAlign := unsafe.Sizeof(o), unsafe.Alignof(o)
	buffers := make([]*arenaBuffer, 0, 32)
	buffers = append(buffers, newArenaBuffer(uintptr(capPerBuf), objSize, objAlign))
	return &autoGrowthArena[T]{
		buffers:  buffers,
		recycled: make([]*T, 0, initRecycleCap),
	}
}
