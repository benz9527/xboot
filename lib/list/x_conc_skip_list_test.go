package list

import (
	"fmt"
	randv2 "math/rand/v2"
	"sync"
	"sync/atomic"
	"testing"
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
	skl.doPut(2, &xSkipListObject{id: "3"})
	t.Logf("%+v\n", skl)
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
