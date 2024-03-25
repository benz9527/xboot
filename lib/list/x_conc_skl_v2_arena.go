package list

import (
	"sync/atomic"
	"unsafe"
)

var (
	xConcSklNodeSize = int(unsafe.Sizeof(xConcSklNode[int64, uint64]{}))
	offsetSize = int(unsafe.Sizeof(uint32(0)))
	nodeAlign = int(unsafe.Sizeof(uint64(0)))-1
)

type xConcSklArena struct {
	n          uint32
	shouldGrow bool
	buffer     []byte
}

func newArena(n int64) *xConcSklArena {
	arena := &xConcSklArena{
		n:      1, // non-zero offset
		buffer: make([]byte, n),
	}
	return arena
}

func (arena *xConcSklArena) allocate(size uint32) uint32 {
	offset := atomic.AddUint32(&arena.n, size)
	if !arena.shouldGrow {
		return offset - size
	}
	s := len(arena.buffer)
	if int64(offset) > int64(s-xConcSklNodeSize) {
		// double size increase
		growth := uint32(s)
		if growth > 1<<30 {
			growth = size
		}
		if growth < size {
			growth = size
		}
		nbuf := make([]byte, s+int(growth))
		copy(nbuf, arena.buffer)
		arena.buffer = nbuf
	}
	return offset - size
}

func (arena *xConcSklArena) size() int64 {
	return int64(atomic.LoadUint32(&arena.n))
}

func (arena *xConcSklArena) putNode(level int) uint32 {
	unusedSize := (sklMaxLevel-level)*offsetSize
	alloc := uint32(xConcSklNodeSize-unusedSize+nodeAlign)
	startOffset := arena.allocate(alloc)
	m := (startOffset+uint32(nodeAlign)) & ^uint32(nodeAlign)
	return m
}

func (arena *xConcSklArena) put(bytes []byte) uint32 {
	bsize := uint32(len(bytes))
	startOffset := arena.allocate(bsize)
	buf := arena.buffer[startOffset:startOffset+bsize]
	copy(buf, bytes)
	return startOffset
}

func (arena *xConcSklArena) getNodeOffset() uint32 {
	return 0
}

func (arena *xConcSklArena) getNode(offset uint32) any {
	if offset == 0 {
		return nil
	}
	return unsafe.Pointer(&arena.buffer[offset])
}