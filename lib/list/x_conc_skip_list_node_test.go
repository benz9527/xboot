package list

import (
	"github.com/stretchr/testify/require"
	"sync/atomic"
	"testing"
)

func TestAtomicPointerCAS(t *testing.T) {
	a := &xConcurrentSkipListNode[uint8, *xSkipListObject]{
		weight: &atomic.Pointer[uint8]{},
		object: &atomic.Pointer[*xSkipListObject]{},
		next:   nil,
	}
	w1 := uint8(1)
	o1 := &xSkipListObject{
		id: "1",
	}
	a.weight.Store(&w1)
	a.object.Store(&o1)

	b := a

	w2 := uint8(2)
	o2 := &xSkipListObject{
		id: "2",
	}
	c := &xConcurrentSkipListNode[uint8, *xSkipListObject]{
		weight: &atomic.Pointer[uint8]{},
		object: &atomic.Pointer[*xSkipListObject]{},
		next:   nil,
	}
	c.weight.Store(&w2)
	c.object.Store(&o2)

	ptr := atomic.Pointer[xConcurrentSkipListNode[uint8, *xSkipListObject]]{}
	ptr.Store(a)
	res := ptr.CompareAndSwap(b, c)
	require.True(t, res)
	require.Equal(t, c, ptr.Load())
}
