package list

import (
	"reflect"
	"unsafe"

	"github.com/benz9527/xboot/lib/infra"
)

// References:
// https://github.com/ortuman/nuke
// https://github.com/dgraph-io/badger/blob/master/skl/arena.go

type xConcSklBuffer struct {
	ptr      unsafe.Pointer
	offset   uintptr // current index offset
	cap      uintptr // capacity, indicates how many objects could be stored
	objSize  uintptr // fixed object size
	objAlign uintptr // fixed object alignment
}

func (buf *xConcSklBuffer) availableBytes() uintptr {
	return buf.cap*buf.objSize - buf.offset
}

func (buf *xConcSklBuffer) allocate() (unsafe.Pointer, bool) {
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

func (buf *xConcSklBuffer) reset() {
	if buf.offset == 0 {
		return
	}
	// Overwrite allow
	buf.offset = 0
}

func (buf *xConcSklBuffer) free() {
	buf.reset()
	buf.ptr = nil
}

func newXConcSklBuffer(cap, size, alignment uintptr) *xConcSklBuffer {
	return &xConcSklBuffer{
		cap:      cap,
		objSize:  size,
		objAlign: alignment,
		offset:   uintptr(0),
	}
}

// T must not be a pointer type.
type autoGrowthArena[T any] struct {
	buffers  []*xConcSklBuffer
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
		buf := newXConcSklBuffer(
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
	buffers := make([]*xConcSklBuffer, 0, 32)
	buffers = append(buffers, newXConcSklBuffer(uintptr(capPerBuf), objSize, objAlign))
	return &autoGrowthArena[T]{
		buffers:  buffers,
		recycled: make([]*T, 0, initRecycleCap),
	}
}

// The pool is used to recycle the auxiliary data structure.
type xConcSklArenaPool[K infra.OrderedKey, V any] struct {
	sklNodeArena *autoGrowthArena[xConcSklNode[K, V]]
	xNodeArena   *autoGrowthArena[xNode[V]]
}

func (pool *xConcSklArenaPool[K, V]) free() {
	pool.sklNodeArena.free()
	pool.xNodeArena.free()
}

func newXConcSklArenaPool[K infra.OrderedKey, V any](unifiedCap uint32) *xConcSklArenaPool[K, V] {
	return &xConcSklArenaPool[K, V]{
		sklNodeArena: newAutoGrowthArena[xConcSklNode[K, V]](unifiedCap, 256),
		xNodeArena:   newAutoGrowthArena[xNode[V]](unifiedCap, 256),
	}
}
