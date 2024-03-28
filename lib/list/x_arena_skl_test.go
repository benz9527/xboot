package list

import (
	"fmt"
	"testing"

	"github.com/benz9527/xboot/lib/id"
	"github.com/stretchr/testify/require"
)

func TestXArenaSkl_SerialProcessing(t *testing.T) {
	skl := &xArenaSkl[uint64, *xSklObject]{
		head: newXConcSklHeadElement[uint64, *xSklObject](),
		kcmp: func(i, j uint64) int64 {
			if i == j {
				return 0
			} else if i < j {
				return -1
			}
			return 1
		},
		rand:    randomLevelV2,
		arena:   newAutoGrowthArena[xArenaSklNode[uint64, *xSklObject]](100, 100),
		levels:  1,
		nodeLen: 0,
	}
	idGen, _ := id.MonotonicNonZeroID()
	skl.optVer = idGen

	size := 5
	for i := uint64(0); i < uint64(size); i++ {
		for j := uint64(0); j < 10; j++ {
			w := (i+1)*100 + j
			_ = skl.Insert(w, &xSklObject{id: fmt.Sprintf("%d", w)})
		}
	}
	t.Logf("nodeLen: %d, indexCount: %d\n", skl.Len(), skl.IndexCount())

	skl.Foreach(func(i int64, item SklIterationItem[uint64, *xSklObject]) bool {
		t.Logf("idx: %d, item key: %v, item value: %v; item level: %d; item count: %d\n", 
		i, item.Key(), item.Val(), item.NodeLevel(), item.NodeItemCount())
		return true
	})

	obj, err := skl.LoadFirst(401)
	require.NoError(t, err)
	require.Equal(t, "401", obj.Val().id)

	for i := uint64(0); i < uint64(size); i++ {
		for j := uint64(0); j < 10; j++ {
			w := (i+1)*100 + j
			_, _ = skl.RemoveFirst(w)
		}
	}
	require.Equal(t, int64(0), skl.Len())
	require.Equal(t, uint64(0), skl.IndexCount())
}

func BenchmarkXArenaSkl(b *testing.B) {
	testByBytes := []byte(`abc`)

	b.StopTimer()
	skl := &xArenaSkl[int, []byte]{
		head: newXConcSklHeadElement[int, []byte](),
		kcmp: func(i, j int) int64 {
			if i == j {
				return 0
			} else if i < j {
				return -1
			}
			return 1
		},
		rand:    randomLevelV2,
		arena:   newAutoGrowthArena[xArenaSklNode[int, []byte]](1_000_000, 100),
		levels:  1,
		nodeLen: 0,
	}
	idGen, _ := id.MonotonicNonZeroID()
	skl.optVer = idGen

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		skl.Insert(i, testByBytes)
	}
}