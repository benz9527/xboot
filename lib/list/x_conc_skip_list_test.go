package list

import (
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
	t.Logf("%+v\n", skl)
}
