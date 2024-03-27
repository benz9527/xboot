package list

import (
	"unsafe"

	"github.com/benz9527/xboot/lib/infra"
)

// References:
// https://github.com/ortuman/nuke
// https://github.com/dgraph-io/badger/blob/master/skl/arena.go

type xConcSklBuffer struct {
	ptr        unsafe.Pointer
	offset     uintptr // current index offset
	maxCount   uintptr // max element count
	eSize      uintptr // fixed element size
	eAlignment uintptr // fixed element alignment
}

func (buf *xConcSklBuffer) availableBytes() uintptr {
	return buf.maxCount*buf.eSize - buf.offset
}

func (buf *xConcSklBuffer) alloc() (unsafe.Pointer, bool) {
	if /* lazy init */ buf.ptr == nil {
		buffer := make([]byte, buf.maxCount*buf.eSize)
		buf.ptr = unsafe.Pointer(unsafe.SliceData(buffer))
	}
	alignOffset := uintptr(0)
	for alignedPtr := uintptr(buf.ptr) + buf.offset; alignedPtr%buf.eAlignment != 0; alignedPtr++ {
		alignOffset++
	}
	allocSize := buf.eSize + alignOffset

	if /* scale */ buf.availableBytes() < allocSize {
		return nil, false
	}

	ptr := unsafe.Pointer(uintptr(buf.ptr) + buf.offset + alignOffset)
	buf.offset += allocSize

	// Translated into runtime.memclrNoHeapPointers by compiler.
	// An assembler optimized implementation.
	// go/src/runtime/memclr_$GOARCH.s (since https://codereview.appspot.com/137880043)
	bytes := unsafe.Slice((*byte)(ptr), buf.eSize)
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

func newXConcSklBuffer(elements, size, alignment uintptr) *xConcSklBuffer {
	return &xConcSklBuffer{
		maxCount:   elements,
		eSize:      size,
		eAlignment: alignment,
		offset:     uintptr(0),
	}
}

// The pool is used to recycle the auxiliary data structure.
type xConcSklArena[K infra.OrderedKey, V any] struct {
	preAllocNodes     uint32
	allocNodesIncr    uint32
	nodeQueue         []*xConcSklNode[K, V]
	releasedNodeQueue []*xConcSklNode[K, V]
}

func newXConcSklArena[K infra.OrderedKey, V any](allocNodes, allocNodesIncr uint32) *xConcSklArena[K, V] {
	p := &xConcSklArena[K, V]{
		allocNodesIncr: allocNodesIncr,
		nodeQueue:      make([]*xConcSklNode[K, V], allocNodes),
	}
	return p
}
