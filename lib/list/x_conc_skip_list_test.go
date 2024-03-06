package list

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestXConcurrentSkipList(t *testing.T) {
	skl := &xConcurrentSkipList[int, *xSkipListObject]{
		cmp: func(i, j int) int {
			return i - j
		},
		head:  &atomic.Pointer[xConcurrentSkipListIndex[int, *xSkipListObject]]{},
		level: 0,
		len:   0,
	}
	skl.doPut(1, &xSkipListObject{id: "1"})
	skl.doPut(2, &xSkipListObject{id: "2"})
	ele := skl.doGet(2)
	require.NotNil(t, ele)
	require.Equal(t, 2, ele.Weight())
	require.Equal(t, "2", ele.Object().id)
	skl.doPut(2, &xSkipListObject{id: "3"}, true)
	ele = skl.doGet(2)
	require.NotNil(t, ele)
	require.Equal(t, 2, ele.Weight())
	require.Equal(t, "3", ele.Object().id)

	skl.ForEach(func(idx int64, w int, o *xSkipListObject) {
		t.Logf("idx: %d, w: %v, o: %v\n", idx, w, o)
	})

	ele = skl.doRemove(2, ele.Object())
	require.NotNil(t, ele)
	require.Equal(t, 2, ele.Weight())
	require.Equal(t, "3", ele.Object().id)
	require.Equal(t, uint32(1), skl.Len())
}

func TestXConcurrentSkipList_SerialProcessing(t *testing.T) {
	skl := &xConcurrentSkipList[uint64, *xSkipListObject]{
		cmp: func(i, j uint64) int {
			res := int(i - j)
			return res
		},
		head:  &atomic.Pointer[xConcurrentSkipListIndex[uint64, *xSkipListObject]]{},
		level: 0,
		len:   0,
	}
	size := 5
	// FIXME: Indexes incorrect to locate the weigh element's location.
	for i := uint64(0); i < uint64(size); i++ {
		for j := uint64(0); j < 10; j++ {
			w := (i+1)*100 + j
			skl.doPut(w, &xSkipListObject{id: fmt.Sprintf("%d", w)})
		}
	}
	t.Logf("%d\n", skl.Len())

	skl.ForEach(func(idx int64, w uint64, o *xSkipListObject) {
		t.Logf("idx: %d, w: %v, o: %v\n", idx, w, o)
	})

	for i := uint64(0); i < uint64(size); i++ {
		for j := uint64(0); j < 10; j++ {
			w := (i+1)*100 + j
			ele := skl.doRemove(w, &xSkipListObject{id: fmt.Sprintf("%d", w)})
			t.Logf("element: %v\n", ele)
		}
	}
	t.Logf("%+v\n", skl)
}

func TestXConcurrentSkipList_DataRace(t *testing.T) {
	skl := &xConcurrentSkipList[uint64, *xSkipListObject]{
		cmp: func(i, j uint64) int {
			res := int(i - j)
			return res
		},
		head:  &atomic.Pointer[xConcurrentSkipListIndex[uint64, *xSkipListObject]]{},
		level: 0,
		len:   0,
	}
	size := 5
	var wg sync.WaitGroup
	wg.Add(size)
	for i := uint64(0); i < uint64(size); i++ {
		go func(idx uint64) {
			defer wg.Done()
			for j := uint64(0); j < 10; j++ {
				w := idx*100 + j
				skl.doPut(w, &xSkipListObject{id: fmt.Sprintf("%d", w)})
			}
		}(i + 1)
	}
	wg.Wait()
	t.Logf("%+v\n", skl)
	wg.Add(size)
	for i := uint64(0); i < uint64(size); i++ {
		go func(idx uint64) {
			defer wg.Done()
			for j := uint64(0); j < 10; j++ {
				w := idx*100 + j
				ele := skl.doRemove(w, &xSkipListObject{id: fmt.Sprintf("%d", w)})
				t.Logf("element: %v\n", ele)
			}
		}(i + 1)
	}
	wg.Wait()
	t.Logf("%+v\n", skl)
}
