package list

//func TestUnsafePointerCAS(t *testing.T) {
//	a := &xConcurrentSkipListNode[uint8, *xSkipListObject]{
//		weight: 1,
//		object: &xSkipListObject{
//			id: "1",
//		},
//		next: nil,
//	}
//	b := a
//	c := &xConcurrentSkipListNode[uint8, *xSkipListObject]{
//		weight: 2,
//		object: &xSkipListObject{
//			id: "2",
//		},
//		isMarker: false,
//		next:     nil,
//	}
//
//	ptr := atomic.Pointer[xConcurrentSkipListNode[uint8, *xSkipListObject]]{}
//	ptr.Store(a)
//	res := ptr.CompareAndSwap(b, c)
//	require.True(t, res)
//	require.Equal(t, c, ptr.Load())
//}
