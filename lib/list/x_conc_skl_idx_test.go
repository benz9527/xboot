package list

import (
	"sync/atomic"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/require"
)

func TestXConcSkl_Indexes(t *testing.T) {
	idx := make(xConcSklIndices[uint8, *xSklObject], 2)
	idx[0] = &xConcSklIndex[uint8, *xSklObject]{
		forward: nil,
	}
	idx[0].forward = &xConcSklNode[uint8, *xSklObject]{}
	idx[0] = &xConcSklIndex[uint8, *xSklObject]{
		forward: nil,
	}
	ptr := atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&idx[0].forward)))
	t.Logf("%v\n", ptr)
}

func TestDecreaseIndexSize(t *testing.T) {
	idxSize := uint64(100)
	atomic.AddUint64(&idxSize, ^uint64(50-1))
	require.Equal(t, uint64(50), idxSize)
}
