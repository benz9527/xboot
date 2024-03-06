package list

import (
	"fmt"
	randv2 "math/rand/v2"
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
	ele = skl.doRemove(2, ele.Object())
	require.NotNil(t, ele)
	require.Equal(t, 2, ele.Weight())
	require.Equal(t, "3", ele.Object().id)
}

func TestXConcurrentSkipList_DataRace(t *testing.T) {
	skl := &xConcurrentSkipList[int, *xSkipListObject]{
		cmp: func(i, j int) int {
			return i - j
		},
		head:  &atomic.Pointer[xConcurrentSkipListIndex[int, *xSkipListObject]]{},
		level: 0,
		len:   0,
	}
	size := 5
	var wg sync.WaitGroup
	wg.Add(size)
	for i := 0; i < size; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				w := randv2.Int()
				skl.doPut(w, &xSkipListObject{id: fmt.Sprintf("%d", w)})
			}
		}()
	}
	wg.Wait()
	t.Logf("%+v\n", skl)
}
