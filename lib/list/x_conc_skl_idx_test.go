package list

import (
	"sync/atomic"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/require"
)

func TestXConcSkipList_Indexes(t *testing.T) {
	idx := make(xConcSkipListIndices[uint8, *xSkipListObject], 2)
	idx[0] = &xConcSkipListIndex[uint8, *xSkipListObject]{
		forward:  nil,
		backward: nil,
	}
	idx[0].forward = &xConcSklNode[uint8, *xSkipListObject]{}
	idx[0] = &xConcSkipListIndex[uint8, *xSkipListObject]{
		forward:  nil,
		backward: nil,
	}
	ptr := atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&idx[0].forward)))
	t.Logf("%v\n", ptr)
}

func TestDecreaseIndexSize(t *testing.T) {
	idxSize := uint64(100)
	atomic.AddUint64(&idxSize, ^uint64(50-1))
	require.Equal(t, uint64(50), idxSize)
}
