package list

import (
	"container/list"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLinkedList_AppendValue(t *testing.T) {
	dlist := NewLinkedList[int]()
	elements := dlist.AppendValue(1, 2, 3, 4, 5)
	assert.Equal(t, len(elements), 5)
	dlist.Foreach(func(idx int64, e *NodeElement[int]) {
		t.Logf("index: %d, e: %v", idx, e)
		assert.Equal(t, elements[idx], e)
		t.Logf("addr: %p, return addr: %p", elements[idx], e)
	})

	dlist2 := list.New()
	dlist2.PushBack(1)
	dlist2.PushBack(2)
	dlist2.PushBack(3)
	dlist2.PushBack(4)
	dlist2.PushBack(5)

	assert.Equal(t, dlist.Len(), int64(dlist2.Len()))

	dlistItr := dlist.Front()
	dlist2Itr := dlist2.Front()
	for dlist2Itr != nil {
		assert.Equal(t, dlistItr.Value, dlist2Itr.Value)
		dlist2Itr = dlist2Itr.Next()
		dlistItr = dlistItr.Next()
	}
}

func TestDoublyLinkedList_InsertBefore(t *testing.T) {
	dlist := NewLinkedList[int]()
	elements := dlist.AppendValue(1)
	_2n := dlist.InsertBefore(2, elements[0])
	_3n := dlist.InsertBefore(3, _2n)
	_4n := dlist.InsertBefore(4, _3n)
	dlist.InsertBefore(5, _4n)
	assert.Equal(t, int64(5), dlist.Len())

	dlist2 := list.New()
	_1n_2 := dlist2.PushBack(1)
	_2n_2 := dlist2.InsertBefore(2, _1n_2)
	_3n_2 := dlist2.InsertBefore(3, _2n_2)
	_4n_2 := dlist2.InsertBefore(4, _3n_2)
	dlist2.InsertBefore(5, _4n_2)

	assert.Equal(t, dlist.Len(), int64(dlist2.Len()))

	dlistItr := dlist.Front()
	dlist2Itr := dlist2.Front()
	for dlist2Itr != nil {
		assert.Equal(t, dlistItr.Value, dlist2Itr.Value)
		dlist2Itr = dlist2Itr.Next()
		dlistItr = dlistItr.Next()
	}
}

func TestDoublyLinkedList_InsertAfter(t *testing.T) {
	dlist := NewLinkedList[int]()
	elements := dlist.AppendValue(1)
	_2n := dlist.InsertAfter(2, elements[0])
	_3n := dlist.InsertAfter(3, _2n)
	_4n := dlist.InsertAfter(4, _3n)
	dlist.InsertAfter(5, _4n)
	assert.Equal(t, int64(5), dlist.Len())

	dlist2 := list.New()
	_1n_2 := dlist2.PushBack(1)
	_2n_2 := dlist2.InsertAfter(2, _1n_2)
	_3n_2 := dlist2.InsertAfter(3, _2n_2)
	_4n_2 := dlist2.InsertAfter(4, _3n_2)
	dlist2.InsertAfter(5, _4n_2)

	assert.Equal(t, dlist.Len(), int64(dlist2.Len()))

	dlistItr := dlist.Front()
	dlist2Itr := dlist2.Front()
	for dlist2Itr != nil {
		assert.Equal(t, dlistItr.Value, dlist2Itr.Value)
		dlist2Itr = dlist2Itr.Next()
		dlistItr = dlistItr.Next()
	}
}

func TestLinkedList_AppendValueThenRemove(t *testing.T) {
	t.Log("test linked list append value")
	dlist := NewLinkedList[int]()
	dlist2 := list.New()
	checkItems := func() {
		dlistItr := dlist.Front()
		dlist2Itr := dlist2.Front()
		for dlist2Itr != nil {
			require.Equal(t, dlistItr.Value, dlist2Itr.Value)
			dlist2Itr = dlist2Itr.Next()
			dlistItr = dlistItr.Next()
		}
	}

	elements := dlist.AppendValue(1, 2, 3, 4, 5)
	require.Equal(t, len(elements), 5)
	dlist.Foreach(func(idx int64, e *NodeElement[int]) {
		require.Equal(t, elements[idx], e)
		addr1, addr2 := fmt.Sprintf("%p", elements[idx]), fmt.Sprintf("%p", e)
		require.Equal(t, addr1, addr2)
	})

	dlist2.PushBack(1)
	dlist2.PushBack(2)
	_3n := dlist2.PushBack(3)
	dlist2.PushBack(4)
	dlist2.PushBack(5)
	assert.Equal(t, dlist.Len(), int64(dlist2.Len()))
	checkItems()

	t.Log("test linked list remove middle")
	dlist.Remove(elements[2])
	dlist2.Remove(_3n)
	checkItems()

	t.Log("test linked list remove head")
	dlist.Remove(dlist.Front())
	dlist2.Remove(dlist2.Front())
	checkItems()

	t.Log("test linked list remove tail")
	dlist.Remove(dlist.Back())
	dlist2.Remove(dlist2.Back())
	checkItems()

	t.Log("test linked list remove nil")
	dlist.Remove(nil)
	// dlist2.Remove(nil) // nil panic
	checkItems()

	t.Log("check released elements")
	require.Equal(t, int64(dlist2.Len()), dlist.Len())
	expected := []int{2, 4}
	dlist.Foreach(func(idx int64, e *NodeElement[int]) {
		require.Equal(t, expected[idx], e.Value)
	})

	require.Nil(t, elements[0].prev)
	require.Nil(t, elements[0].next)
	require.Nil(t, elements[0].listRef)

	require.Nil(t, elements[2].prev)
	require.Nil(t, elements[2].next)
	require.Nil(t, elements[2].listRef)

	require.Nil(t, elements[4].prev)
	require.Nil(t, elements[4].next)
	require.Nil(t, elements[4].listRef)
}

func BenchmarkNewLinkedList_AppendValue(b *testing.B) {
	dlist := NewLinkedList[int]()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dlist.AppendValue(i)
	}
	b.ReportAllocs()
}

func BenchmarkSDKLinkedList_PushBack(b *testing.B) {
	dlist := list.New()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dlist.PushBack(i)
	}
	b.ReportAllocs()
}

func TestLinkedList_PushBack(t *testing.T) {
	dlist := NewLinkedList[int]()
	element := dlist.PushBack(1)
	require.Equal(t, int64(1), dlist.Len())
	require.Equal(t, element.Value, 1)

	element = dlist.PushBack(2)
	require.Equal(t, int64(2), dlist.Len())
	require.Equal(t, element.Value, 2)

	expected := []int{1, 2}
	dlist.Foreach(func(idx int64, e *NodeElement[int]) {
		require.Equal(t, expected[idx], e.Value)
	})

	reverseExpected := []int{2, 1}
	dlist.ReverseForeach(func(idx int64, e *NodeElement[int]) {
		require.Equal(t, reverseExpected[idx], e.Value)
	})
}

func TestLinkedList_PushFront(t *testing.T) {
	dlist := NewLinkedList[int]()
	element := dlist.PushFront(1)
	require.Equal(t, int64(1), dlist.Len())
	require.Equal(t, element.Value, 1)

	element = dlist.PushFront(2)
	require.Equal(t, int64(2), dlist.Len())
	require.Equal(t, element.Value, 2)

	expected := []int{2, 1}
	dlist.Foreach(func(idx int64, e *NodeElement[int]) {
		require.Equal(t, expected[idx], e.Value)
	})

	reverseExpected := []int{1, 2}
	dlist.ReverseForeach(func(idx int64, e *NodeElement[int]) {
		require.Equal(t, reverseExpected[idx], e.Value)
	})
}

func BenchmarkDoublyLinkedList_PushBack(b *testing.B) {
	dlist := NewLinkedList[int]()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dlist.PushBack(i)
	}
	b.ReportAllocs()
}

func BenchmarkDoublyLinkedList_Append(b *testing.B) {
	dlist := NewLinkedList[int]()
	elements := make([]*NodeElement[int], 0, b.N)
	for i := 0; i < b.N; i++ {
		elements = append(elements, newNodeElement[int](i, dlist.(*doublyLinkedList[int])))
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dlist.Append(elements[i])
	}
	b.StopTimer()
	b.ReportAllocs()
	require.Equal(b, int64(b.N), dlist.Len())
}

func TestDoublyLinkedList_InsertBefore2(t *testing.T) {
	dlist := NewLinkedList[int]()
	_2n := dlist.InsertBefore(2, dlist.Front())
	require.Equal(t, int64(1), dlist.Len())
	require.Equal(t, _2n.Value, 2)
	dlist.Foreach(func(idx int64, e *NodeElement[int]) {
		t.Logf("index: %d, addr: %p, e: %v", idx, e, e)
	})
	dlist.ReverseForeach(func(idx int64, e *NodeElement[int]) {
		t.Logf("reverse: index: %d, addr: %p, e: %v", idx, e, e)
	})
}

func TestLinkedList_InsertAfterAndMove(t *testing.T) {
	dlist := NewLinkedList[int]()
	dlist2 := list.New()
	checkItems := func() {
		dlistItr := dlist.Front()
		dlist2Itr := dlist2.Front()
		require.NotNil(t, dlistItr)
		require.NotNil(t, dlist2Itr)
		require.Equal(t, int64(dlist2.Len()), dlist.Len())
		for dlist2Itr != nil {
			require.Equal(t, dlist2Itr.Value, dlistItr.Value)
			dlist2Itr = dlist2Itr.Next()
			dlistItr = dlistItr.Next()
		}
	}

	elements := dlist.AppendValue(1, 2, 3, 4, 5)
	_6n := dlist.InsertAfter(6, elements[len(elements)-1])
	_7n := dlist.InsertBefore(7, elements[0])
	require.Equal(t, int64(7), dlist.Len())
	expected := []int{7, 1, 2, 3, 4, 5, 6}
	dlist.Foreach(func(idx int64, e *NodeElement[int]) {
		require.Equal(t, expected[idx], e.Value)
	})
	reverseExpected := []int{6, 5, 4, 3, 2, 1, 7}
	dlist.ReverseForeach(func(idx int64, e *NodeElement[int]) {
		require.Equal(t, reverseExpected[idx], e.Value)
	})

	dlist2.PushBack(1)
	dlist2.PushBack(2)
	dlist2.PushBack(3)
	dlist2.PushBack(4)
	dlist2.PushBack(5)
	_6n_2 := dlist2.InsertAfter(6, dlist2.Back())
	_7n_2 := dlist2.InsertBefore(7, dlist2.Front())
	require.Equal(t, int64(dlist2.Len()), dlist.Len())
	checkItems()

	t.Log("test move after")
	dlist.MoveToBack(_7n)
	dlist2.MoveToBack(_7n_2)
	checkItems()
	expected = []int{1, 2, 3, 4, 5, 6, 7}
	dlist.Foreach(func(idx int64, e *NodeElement[int]) {
		require.Equal(t, expected[idx], e.Value)
	})
	reverseExpected = []int{7, 6, 5, 4, 3, 2, 1}
	dlist.ReverseForeach(func(idx int64, e *NodeElement[int]) {
		require.Equal(t, reverseExpected[idx], e.Value)
	})

	t.Log("test move to front")
	dlist.MoveToFront(_6n)
	dlist2.MoveToFront(_6n_2)
	checkItems()
	expected = []int{6, 1, 2, 3, 4, 5, 7}
	dlist.Foreach(func(idx int64, e *NodeElement[int]) {
		require.Equal(t, expected[idx], e.Value)
	})
	reverseExpected = []int{7, 5, 4, 3, 2, 1, 6}
	dlist.ReverseForeach(func(idx int64, e *NodeElement[int]) {
		require.Equal(t, reverseExpected[idx], e.Value)
	})

	t.Log("test move before")
	dlist.MoveBefore(_6n, _7n)
	dlist2.MoveBefore(_6n_2, _7n_2)
	checkItems()
	expected = []int{1, 2, 3, 4, 5, 6, 7}
	dlist.Foreach(func(idx int64, e *NodeElement[int]) {
		require.Equal(t, expected[idx], e.Value)
	})
	reverseExpected = []int{7, 6, 5, 4, 3, 2, 1}
	dlist.ReverseForeach(func(idx int64, e *NodeElement[int]) {
		require.Equal(t, reverseExpected[idx], e.Value)
	})

	t.Log("test move after")
	dlist.MoveAfter(_7n, dlist.Front())
	dlist2.MoveAfter(_7n_2, dlist2.Front())
	checkItems()
	expected = []int{1, 7, 2, 3, 4, 5, 6}
	dlist.Foreach(func(idx int64, e *NodeElement[int]) {
		require.Equal(t, expected[idx], e.Value)
	})
	reverseExpected = []int{6, 5, 4, 3, 2, 7, 1}
	dlist.ReverseForeach(func(idx int64, e *NodeElement[int]) {
		require.Equal(t, reverseExpected[idx], e.Value)
	})

	t.Log("test push front list")
	dlist_1 := NewLinkedList[int]()
	dlist_1.AppendValue(8, 9, 10)
	dlist2_1 := list.New()
	dlist2_1.PushBack(8)
	dlist2_1.PushBack(9)
	dlist2_1.PushBack(10)

	dlist.PushFrontList(dlist_1)
	dlist2.PushFrontList(dlist2_1)
	checkItems()
	expected = []int{8, 9, 10, 1, 7, 2, 3, 4, 5, 6}
	dlist.Foreach(func(idx int64, e *NodeElement[int]) {
		require.Equal(t, expected[idx], e.Value)
	})
	reverseExpected = []int{6, 5, 4, 3, 2, 7, 1, 10, 9, 8}
	dlist.ReverseForeach(func(idx int64, e *NodeElement[int]) {
		require.Equal(t, reverseExpected[idx], e.Value)
	})

	t.Log("test push back list")
	dlist_2 := NewLinkedList[int]()
	dlist_2.AppendValue(11, 12, 13)
	dlist2_2 := list.New()
	dlist2_2.PushBack(11)
	dlist2_2.PushBack(12)
	dlist2_2.PushBack(13)

	dlist.PushBackList(dlist_2)
	dlist2.PushBackList(dlist2_2)
	checkItems()
	expected = []int{8, 9, 10, 1, 7, 2, 3, 4, 5, 6, 11, 12, 13}
	dlist.Foreach(func(idx int64, e *NodeElement[int]) {
		require.Equal(t, expected[idx], e.Value)
	})
	reverseExpected = []int{13, 12, 11, 6, 5, 4, 3, 2, 7, 1, 10, 9, 8}
	dlist.ReverseForeach(func(idx int64, e *NodeElement[int]) {
		require.Equal(t, reverseExpected[idx], e.Value)
	})
}

func TestDoublyLinkedList_APIsCoverage(t *testing.T) {
	dlist := NewLinkedList[int]()
	dlist2 := list.New()
	checkItems := func() {
		dlistItr := dlist.Front()
		dlist2Itr := dlist2.Front()
		require.NotNil(t, dlistItr)
		require.NotNil(t, dlist2Itr)
		require.Equal(t, int64(dlist2.Len()), dlist.Len())
		for dlist2Itr != nil {
			require.Equal(t, dlist2Itr.Value, dlistItr.Value)
			dlist2Itr = dlist2Itr.Next()
			dlistItr = dlistItr.Next()
		}
	}

	e1 := dlist.PushFront(1)
	e2 := dlist.PushBack(2)
	dlist.MoveBefore(e2, e1)

	e1_1 := dlist2.PushFront(1)
	e2_1 := dlist2.PushBack(2)
	dlist2.MoveBefore(e2_1, e1_1)

	checkItems()

	e3 := dlist.InsertAfter(3, e1)
	e3_1 := dlist2.InsertAfter(3, e1_1)
	checkItems()

	expected := []int{2, 1, 3}
	dlist.Foreach(func(idx int64, e *NodeElement[int]) {
		require.Equal(t, expected[idx], e.Value)
	})

	dlist.MoveBefore(e3, e1)
	dlist2.MoveBefore(e3_1, e1_1)
	checkItems()

	expected = []int{2, 3, 1}
	dlist.Foreach(func(idx int64, e *NodeElement[int]) {
		require.Equal(t, expected[idx], e.Value)
	})

	dlist.MoveBefore(e2, e3)
	dlist2.MoveBefore(e2_1, e3_1)
	checkItems()
	expected = []int{2, 3, 1}
	dlist.Foreach(func(idx int64, e *NodeElement[int]) {
		require.Equal(t, expected[idx], e.Value)
	})

	dlist.MoveBefore(e2, e1)
	dlist2.MoveBefore(e2_1, e1_1)
	checkItems()
	expected = []int{3, 2, 1}
	dlist.Foreach(func(idx int64, e *NodeElement[int]) {
		require.Equal(t, expected[idx], e.Value)
	})

	dlist.MoveBefore(e2, e3)
	dlist2.MoveBefore(e2_1, e3_1)
	checkItems()
	expected = []int{2, 3, 1}
	dlist.Foreach(func(idx int64, e *NodeElement[int]) {
		require.Equal(t, expected[idx], e.Value)
	})

	dlist.MoveAfter(e2, e3)
	dlist2.MoveAfter(e2_1, e3_1)
	checkItems()
	expected = []int{3, 2, 1}
	dlist.Foreach(func(idx int64, e *NodeElement[int]) {
		require.Equal(t, expected[idx], e.Value)
	})

	dlist.MoveAfter(e1, e2)
	dlist2.MoveAfter(e1_1, e2_1)
	checkItems()
	expected = []int{3, 2, 1}
	dlist.Foreach(func(idx int64, e *NodeElement[int]) {
		require.Equal(t, expected[idx], e.Value)
	})

	dlist.MoveAfter(e2, e1)
	dlist2.MoveAfter(e2_1, e1_1)
	checkItems()
	expected = []int{3, 1, 2}
	dlist.Foreach(func(idx int64, e *NodeElement[int]) {
		require.Equal(t, expected[idx], e.Value)
	})

	moved := dlist.MoveAfter(e2, e2)
	require.False(t, moved)
	checkItems()

	moved = dlist.MoveBefore(e1, e1)
	require.False(t, moved)
	checkItems()

	moved = dlist.MoveBefore(nil, e1)
	require.False(t, moved)
	checkItems()

	moved = dlist.MoveBefore(e1, nil)
	require.False(t, moved)
	checkItems()

	moved = dlist.MoveAfter(nil, e1)
	require.False(t, moved)
	checkItems()

	moved = dlist.MoveAfter(e1, nil)
	require.False(t, moved)
	checkItems()

	moved = dlist.MoveToBack(nil)
	require.False(t, moved)
	checkItems()

	moved = dlist.MoveToFront(nil)
	require.False(t, moved)
	checkItems()

	var nilE *NodeElement[int] = nil
	require.Nil(t, nilE.Prev())
	require.Nil(t, nilE.Next())
	require.False(t, nilE.HasNext())
	require.False(t, nilE.HasPrev())
}
