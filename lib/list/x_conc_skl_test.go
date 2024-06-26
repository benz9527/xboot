package list

import (
	"errors"
	"fmt"
	randv2 "math/rand/v2"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/benz9527/xboot/lib/id"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestXConcSkl_SerialProcessing(t *testing.T) {
	opts := make([]SklOption[uint64, *xSklObject], 0, 2)
	opts = append(opts, WithXConcSklDataNodeUniqueMode[uint64, *xSklObject](),
		WithSklKeyCmpDesc[uint64, *xSklObject]())

	skl, err := NewSkl[uint64, *xSklObject](
		XConcSkl,
		opts...,
	)
	require.NoError(t, err)

	size := 5
	for i := uint64(0); i < uint64(size); i++ {
		for j := uint64(0); j < 10; j++ {
			w := (i+1)*100 + j
			_ = skl.Insert(w, &xSklObject{id: fmt.Sprintf("%d", w)})
		}
	}
	t.Logf("nodeLen: %d, indexCount: %d\n", skl.Len(), skl.IndexCount())

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

func TestXConcSkl_DataRace(t *testing.T) {
	opts := make([]SklOption[uint64, *xSklObject], 0, 2)
	opts = append(opts, WithXConcSklDataNodeUniqueMode[uint64, *xSklObject]())

	skl, err := NewSkl[uint64, *xSklObject](
		XConcSkl,
		opts...,
	)
	require.NoError(t, err)

	ele, err := skl.PopHead()
	require.Nil(t, ele)
	require.True(t, errors.Is(err, ErrXSklIsEmpty))

	size := 5
	size2 := 10
	var wg sync.WaitGroup
	wg.Add(size * size2)
	for i := uint64(0); i < uint64(size); i++ {
		for j := uint64(0); j < uint64(size2); j++ {
			go func(idx uint64) {
				w := idx
				time.Sleep(time.Duration(cryptoRandUint32()%5) * time.Millisecond)
				_ = skl.Insert(w, &xSklObject{id: fmt.Sprintf("%d", w)})
				wg.Done()
			}((i+1)*100 + j)
		}
	}
	wg.Wait()
	t.Logf("nodeLen: %d, indexCount: %d\n", skl.Len(), skl.IndexCount())

	obj, err := skl.LoadFirst(401)
	require.NoError(t, err)
	require.Equal(t, "401", obj.Val().id)

	wg.Add(size * size2)
	for i := uint64(0); i < uint64(size); i++ {
		for j := uint64(0); j < uint64(size2); j++ {
			go func(idx uint64) {
				w := idx
				time.Sleep(time.Duration(cryptoRandUint32()%5) * time.Millisecond)
				_, _ = skl.RemoveFirst(w)
				wg.Done()
			}((i+1)*100 + j)
		}
	}
	wg.Wait()
	require.Equal(t, int64(0), skl.Len())
	require.Equal(t, uint64(0), skl.IndexCount())
}

func TestXConcSkl_LinkedList_SerialProcessing(t *testing.T) {
	skl := &xConcSkl[uint64, *xSklObject]{
		head:    newXConcSklHead[uint64, *xSklObject](),
		levels:  1,
		nodeLen: 0,
		vcmp: func(i, j *xSklObject) int64 {
			// avoid calculation overflow
			_i, _j := i.Hash(), j.Hash()
			if _i == _j {
				return 0
			} else if _i > _j {
				return 1
			}
			return -1
		},
		rand:  randomLevelV3,
		flags: 0,
	}
	idGen, _ := id.MonotonicNonZeroID()
	skl.optVer = idGen
	skl.flags = setBitsAs(skl.flags, xConcSklXNodeModeFlagBits, uint32(linkedList))

	ele, err := skl.PopHead()
	require.Nil(t, ele)
	require.True(t, errors.Is(err, ErrXSklIsEmpty))

	skl.Insert(4, &xSklObject{id: fmt.Sprintf("%d", 9)})
	skl.Insert(4, &xSklObject{id: fmt.Sprintf("%d", 5)})
	skl.Insert(4, &xSklObject{id: fmt.Sprintf("%d", 8)})
	skl.Insert(4, &xSklObject{id: fmt.Sprintf("%d", 7)})
	skl.Insert(4, &xSklObject{id: fmt.Sprintf("%d", 1)})
	skl.Insert(4, &xSklObject{id: fmt.Sprintf("%d", 2)})
	skl.Insert(4, &xSklObject{id: fmt.Sprintf("%d", 4)})
	skl.Insert(4, &xSklObject{id: fmt.Sprintf("%d", 6)})
	skl.Insert(2, &xSklObject{id: fmt.Sprintf("%d", 9)})
	skl.Insert(2, &xSklObject{id: fmt.Sprintf("%d", 5)})
	skl.Insert(2, &xSklObject{id: fmt.Sprintf("%d", 8)})
	skl.Insert(2, &xSklObject{id: fmt.Sprintf("%d", 7)})
	skl.Insert(2, &xSklObject{id: fmt.Sprintf("%d", 1)})
	skl.Insert(2, &xSklObject{id: fmt.Sprintf("%d", 2)})
	skl.Insert(2, &xSklObject{id: fmt.Sprintf("%d", 4)})
	skl.Insert(2, &xSklObject{id: fmt.Sprintf("%d", 6)})
	skl.Insert(3, &xSklObject{id: fmt.Sprintf("%d", 100)})
	skl.Insert(3, &xSklObject{id: fmt.Sprintf("%d", 200)})
	skl.Insert(3, &xSklObject{id: fmt.Sprintf("%d", 2)})
	skl.Insert(1, &xSklObject{id: fmt.Sprintf("%d", 9)})
	skl.Insert(1, &xSklObject{id: fmt.Sprintf("%d", 200)})
	skl.Insert(1, &xSklObject{id: fmt.Sprintf("%d", 2)})

	t.Logf("nodeLen: %d, indexCount: %d\n", skl.Len(), skl.IndexCount())

	skl.Foreach(func(idx int64, item SklIterationItem[uint64, *xSklObject]) bool {
		t.Logf("idx: %d, key: %v, value: %v, levels: %d, count: %d\n", idx, item.Key(), item.Val(), item.NodeLevel(), item.NodeItemCount())
		return true
	})

	aux := make(xConcSklAux[uint64, *xSklObject], 2*sklMaxLevel)
	foundResult := skl.rmTraverse(1, false, aux)
	assert.LessOrEqual(t, int32(0), foundResult)
	require.True(t, isSet(aux.loadPred(0).flags, nodeIsHeadFlagBit))
	require.Equal(t, uint64(1), aux.loadSucc(0).key)
	require.Equal(t, "9", (*aux.loadSucc(0).atomicLoadRoot().linkedListNext().vptr).id)

	foundResult = skl.rmTraverse(2, false, aux)
	assert.LessOrEqual(t, int32(0), foundResult)
	require.Equal(t, uint64(1), aux.loadPred(0).key)
	require.Equal(t, "9", (*aux.loadPred(0).atomicLoadRoot().linkedListNext().vptr).id)
	require.Equal(t, uint64(2), aux.loadSucc(0).key)
	require.Equal(t, "9", (*aux.loadSucc(0).atomicLoadRoot().linkedListNext().vptr).id)

	foundResult = skl.rmTraverse(3, false, aux)
	assert.LessOrEqual(t, int32(0), foundResult)
	require.Equal(t, uint64(2), aux.loadPred(0).key)
	require.Equal(t, "9", (*aux.loadPred(0).atomicLoadRoot().linkedListNext().vptr).id)
	require.Equal(t, uint64(3), aux.loadSucc(0).key)
	require.Equal(t, "2", (*aux.loadSucc(0).atomicLoadRoot().linkedListNext().vptr).id)

	foundResult = skl.rmTraverse(4, false, aux)
	assert.LessOrEqual(t, int32(0), foundResult)
	require.Equal(t, uint64(3), aux.loadPred(0).key)
	require.Equal(t, "2", (*aux.loadPred(0).atomicLoadRoot().linkedListNext().vptr).id)
	require.Equal(t, uint64(4), aux.loadSucc(0).key)
	require.Equal(t, "9", (*aux.loadSucc(0).atomicLoadRoot().linkedListNext().vptr).id)

	foundResult = skl.rmTraverse(100, false, aux)
	assert.Equal(t, int32(-1), foundResult)

	foundResult = skl.rmTraverse(0, false, aux)
	assert.Equal(t, int32(-1), foundResult)

	removed, err := skl.RemoveAll(3)
	require.NoError(t, err)
	require.Equal(t, uint64(3), removed[0].Key())
	require.Equal(t, "2", removed[0].Val().id)
	require.Equal(t, uint64(3), removed[1].Key())
	require.Equal(t, "200", removed[1].Val().id)
	require.Equal(t, uint64(3), removed[2].Key())
	require.Equal(t, "100", removed[2].Val().id)
	skl.Foreach(func(idx int64, item SklIterationItem[uint64, *xSklObject]) bool {
		if item.Key() == uint64(3) {
			t.FailNow()
			return false
		}
		return true
	})

	removed, err = skl.RemoveIfMatch(2, func(that *xSklObject) bool {
		return that.id == "8"
	})
	require.NoError(t, err)
	require.Equal(t, 1, len(removed))
	require.Equal(t, uint64(2), removed[0].Key())
	require.Equal(t, "8", removed[0].Val().id)
	skl.Foreach(func(idx int64, item SklIterationItem[uint64, *xSklObject]) bool {
		if item.Key() == uint64(2) && item.Val().id == "8" {
			t.FailNow()
			return false
		}
		return true
	})

	loaded, err := skl.LoadAll(1)
	require.NoError(t, err)
	require.Equal(t, 3, len(loaded))
	require.Equal(t, uint64(1), loaded[0].Key())
	require.Equal(t, "9", loaded[0].Val().id)
	require.Equal(t, uint64(1), loaded[1].Key())
	require.Equal(t, "2", loaded[1].Val().id)
	require.Equal(t, uint64(1), loaded[2].Key())
	require.Equal(t, "200", loaded[2].Val().id)

	loaded, err = skl.LoadIfMatch(2, func(that *xSklObject) bool {
		return that.id == "9"
	})
	require.NoError(t, err)
	require.Equal(t, 1, len(loaded))
	require.Equal(t, uint64(2), loaded[0].Key())
	require.Equal(t, "9", loaded[0].Val().id)
}

func TestXConcSkl_Rbtree_SerialProcessing(t *testing.T) {
	skl := &xConcSkl[uint64, *xSklObject]{
		head:    newXConcSklHead[uint64, *xSklObject](),
		levels:  1,
		nodeLen: 0,
		vcmp: func(i, j *xSklObject) int64 {
			// avoid calculation overflow
			_i, _j := i.Hash(), j.Hash()
			if _i == _j {
				return 0
			} else if _i > _j {
				return 1
			}
			return -1
		},
		rand:  randomLevelV3,
		flags: 0,
	}
	idGen, _ := id.MonotonicNonZeroID()
	skl.optVer = idGen
	skl.flags = setBitsAs(skl.flags, xConcSklXNodeModeFlagBits, uint32(rbtree))

	ele, err := skl.PopHead()
	require.Nil(t, ele)
	require.True(t, errors.Is(err, ErrXSklIsEmpty))

	skl.Insert(4, &xSklObject{id: fmt.Sprintf("%d", 9)})
	skl.Insert(4, &xSklObject{id: fmt.Sprintf("%d", 5)})
	skl.Insert(4, &xSklObject{id: fmt.Sprintf("%d", 8)})
	skl.Insert(4, &xSklObject{id: fmt.Sprintf("%d", 7)})
	skl.Insert(4, &xSklObject{id: fmt.Sprintf("%d", 1)})
	skl.Insert(4, &xSklObject{id: fmt.Sprintf("%d", 2)})
	skl.Insert(4, &xSklObject{id: fmt.Sprintf("%d", 4)})
	skl.Insert(4, &xSklObject{id: fmt.Sprintf("%d", 6)})
	skl.Insert(2, &xSklObject{id: fmt.Sprintf("%d", 9)})
	skl.Insert(2, &xSklObject{id: fmt.Sprintf("%d", 5)})
	skl.Insert(2, &xSklObject{id: fmt.Sprintf("%d", 8)})
	skl.Insert(2, &xSklObject{id: fmt.Sprintf("%d", 7)})
	skl.Insert(2, &xSklObject{id: fmt.Sprintf("%d", 1)})
	skl.Insert(2, &xSklObject{id: fmt.Sprintf("%d", 2)})
	skl.Insert(2, &xSklObject{id: fmt.Sprintf("%d", 4)})
	skl.Insert(2, &xSklObject{id: fmt.Sprintf("%d", 6)})
	skl.Insert(3, &xSklObject{id: fmt.Sprintf("%d", 100)})
	skl.Insert(3, &xSklObject{id: fmt.Sprintf("%d", 200)})
	skl.Insert(3, &xSklObject{id: fmt.Sprintf("%d", 2)})
	skl.Insert(1, &xSklObject{id: fmt.Sprintf("%d", 9)})
	skl.Insert(1, &xSklObject{id: fmt.Sprintf("%d", 200)})
	skl.Insert(1, &xSklObject{id: fmt.Sprintf("%d", 2)})

	t.Logf("nodeLen: %d, indexCount: %d\n", skl.Len(), skl.IndexCount())

	skl.Foreach(func(idx int64, item SklIterationItem[uint64, *xSklObject]) bool {
		t.Logf("idx: %d, key: %v, value: %v, levels: %d, count: %d\n", idx, item.Key(), item.Val(), item.NodeLevel(), item.NodeItemCount())
		return true
	})

	removed, err := skl.RemoveAll(3)
	require.NoError(t, err)
	require.Equal(t, uint64(3), removed[0].Key())
	require.Equal(t, "2", removed[0].Val().id)
	require.Equal(t, uint64(3), removed[1].Key())
	require.Equal(t, "200", removed[1].Val().id)
	require.Equal(t, uint64(3), removed[2].Key())
	require.Equal(t, "100", removed[2].Val().id)
	skl.Foreach(func(idx int64, item SklIterationItem[uint64, *xSklObject]) bool {
		if item.Key() == uint64(3) {
			t.FailNow()
			return false
		}
		return true
	})

	removed, err = skl.RemoveIfMatch(2, func(that *xSklObject) bool {
		return that.id == "8"
	})
	require.NoError(t, err)
	require.Equal(t, 1, len(removed))
	require.Equal(t, uint64(2), removed[0].Key())
	require.Equal(t, "8", removed[0].Val().id)
	skl.Foreach(func(idx int64, item SklIterationItem[uint64, *xSklObject]) bool {
		if item.Key() == uint64(2) && item.Val().id == "8" {
			t.FailNow()
			return false
		}
		return true
	})

	loaded, err := skl.LoadAll(1)
	require.NoError(t, err)
	require.Equal(t, 3, len(loaded))
	require.Equal(t, uint64(1), loaded[0].Key())
	require.Equal(t, "9", loaded[0].Val().id)
	require.Equal(t, uint64(1), loaded[1].Key())
	require.Equal(t, "2", loaded[1].Val().id)
	require.Equal(t, uint64(1), loaded[2].Key())
	require.Equal(t, "200", loaded[2].Val().id)

	loaded, err = skl.LoadIfMatch(2, func(that *xSklObject) bool {
		return that.id == "9"
	})
	require.NoError(t, err)
	require.Equal(t, 1, len(loaded))
	require.Equal(t, uint64(2), loaded[0].Key())
	require.Equal(t, "9", loaded[0].Val().id)
}

func xConcSklDuplicateDataRaceRunCore(t *testing.T, mode xNodeMode, rmBySucc bool) {
	opts := []SklOption[uint64, int64]{
		WithSklRandLevelGen[uint64, int64](randomLevelV3),
	}
	switch mode {
	case linkedList:
		opts = append(opts, WithXConcSklDataNodeLinkedListMode[uint64, int64](
			func(i, j int64) int64 {
				// avoid calculation overflow
				if i == j {
					return 0
				} else if i > j {
					return 1
				}
				return -1
			}))
	case rbtree:
		opts = append(opts, WithXConcSklDataNodeRbtreeMode[uint64, int64](
			func(i, j int64) int64 {
				// avoid calculation overflow
				if i == j {
					return 0
				} else if i > j {
					return 1
				}
				return -1
			}, rmBySucc))
	}

	skl, err := NewXSkl[uint64, int64](
		XConcSkl,
		opts...,
	)
	require.NoError(t, err)

	ele, err := skl.PopHead()
	require.Nil(t, ele)
	require.True(t, errors.Is(err, ErrXSklIsEmpty))

	size := 10
	size2 := 10
	unorderedWeights := make([]int64, 0, size2)
	for i := 0; i < size2; i++ {
		unorderedWeights = append(unorderedWeights, int64(cryptoRandUint64()))
	}
	orderedWeights := make([]int64, 0, size2)
	orderedWeights = append(orderedWeights, unorderedWeights...)
	sort.Slice(orderedWeights, func(i, j int) bool {
		return orderedWeights[i] < orderedWeights[j]
	})

	var wg sync.WaitGroup
	wg.Add(size * size2)

	type answer struct {
		w  uint64
		id int64
	}
	expected := make([]*answer, 0, size*size2)

	for i := uint64(0); i < uint64(size); i++ {
		for j := uint64(0); j < uint64(size2); j++ {
			go func(_i, _j uint64) {
				w := (_i + 1) * 100
				time.Sleep(time.Duration(cryptoRandUint32()%5) * time.Millisecond)
				_ = skl.Insert(w, unorderedWeights[_j])
				wg.Done()
			}(i, j)
			expected = append(expected, &answer{w: (i + 1) * 100, id: orderedWeights[j]})
		}
	}
	wg.Wait()
	t.Logf("nodeLen: %d, indexCount: %d\n", skl.Len(), skl.IndexCount())

	skl.Foreach(func(idx int64, item SklIterationItem[uint64, int64]) bool {
		require.Equalf(t, expected[idx].w, item.Key(), "exp: %d; actual: %d\n", expected[idx].w, item.Key())
		require.Equalf(t, expected[idx].id, item.Val(), "exp: %d; actual: %d\n", expected[idx].id, item.Val())
		return true
	})

	wg.Add(size * size2)
	for i := uint64(0); i < uint64(size); i++ {
		for j := uint64(0); j < uint64(size2); j++ {
			go func(_i, _j uint64) {
				w := (_i + 1) * 100
				time.Sleep(time.Duration(cryptoRandUint32()%5) * time.Millisecond)
				if _, err := skl.RemoveFirst(w); err != nil {
					t.Logf("remove failed, key: %d, err: %v\n", w, err)
				}
				wg.Done()
			}(i, j)
		}
	}
	wg.Wait()
	require.Equal(t, int64(0), skl.Len())
	require.Equal(t, uint64(0), skl.IndexCount())
}

func TestXConcSkl_Duplicate_DataRace(t *testing.T) {
	type testcase struct {
		name       string
		typ        xNodeMode
		rbRmBySucc bool
	}
	testcases := []testcase{
		{
			name: "skl lock free mutex data race - linkedlist",
			typ:  linkedList,
		},
		{
			name: "skl lock free mutex data race - rbtree",
			typ:  rbtree,
		},
		{
			name:       "skl lock free mutex data race - rbtree (succ)",
			typ:        rbtree,
			rbRmBySucc: true,
		},
	}
	t.Parallel()
	for _, tc := range testcases {
		t.Run(tc.name, func(tt *testing.T) {
			xConcSklDuplicateDataRaceRunCore(tt, tc.typ, tc.rbRmBySucc)
		})
	}
}

func xConcSklPeekAndPopHeadRunCore(t *testing.T, mode xNodeMode) {
	skl := &xConcSkl[uint64, int64]{
		head:    newXConcSklHead[uint64, int64](),
		levels:  1,
		nodeLen: 0,
		vcmp: func(i, j int64) int64 {
			// avoid calculation overflow
			if i == j {
				return 0
			} else if i > j {
				return 1
			}
			return -1
		},
		rand:  randomLevelV3,
		flags: 0,
	}
	idGen, _ := id.MonotonicNonZeroID()
	skl.optVer = idGen
	if mode != unique {
		skl.flags = setBitsAs(skl.flags, xConcSklXNodeModeFlagBits, uint32(mode))
	}

	ele, err := skl.PopHead()
	require.Nil(t, ele)
	require.True(t, errors.Is(err, ErrXSklIsEmpty))

	size := 10
	size2 := 10
	unorderedWeights := make([]int64, 0, size2)
	for i := 0; i < size2; i++ {
		unorderedWeights = append(unorderedWeights, int64(cryptoRandUint64()))
	}
	orderedWeights := make([]int64, 0, size2)
	orderedWeights = append(orderedWeights, unorderedWeights...)
	sort.Slice(orderedWeights, func(i, j int) bool {
		return orderedWeights[i] < orderedWeights[j]
	})

	type answer struct {
		w  uint64
		id int64
	}
	expected := make([]*answer, 0, size*size2)

	for i := uint64(0); i < uint64(size); i++ {
		for j := uint64(0); j < uint64(size2); j++ {
			w := (i + 1) * 100
			_ = skl.Insert(w, orderedWeights[j])
			expected = append(expected, &answer{w: (i + 1) * 100, id: orderedWeights[j]})
		}
	}
	t.Logf("nodeLen: %d, indexCount: %d\n", skl.Len(), skl.IndexCount())

	if mode == unique {
		skl.Foreach(func(idx int64, item SklIterationItem[uint64, int64]) bool {
			require.Equalf(t, expected[idx*int64(size2)+9].w, item.Key(), "idx: %d; exp: %d; actual: %d\n", idx, expected[idx*int64(size2)+9].w, item.Key())
			require.Equalf(t, expected[idx*int64(size2)+9].id, item.Val(), "idx: %d; exp: %d; actual: %d\n", idx, expected[idx*int64(size2)+9].id, item.Val())
			return true
		})
	} else {
		skl.Foreach(func(idx int64, item SklIterationItem[uint64, int64]) bool {
			require.Equalf(t, expected[idx].w, item.Key(), "exp: %d; actual: %d\n", expected[idx].w, item.Key())
			require.Equalf(t, expected[idx].id, item.Val(), "exp: %d; actual: %d\n", expected[idx].id, item.Val())
			return true
		})
	}

	for i := int64(0); skl.Len() > 0; i++ {
		h1 := skl.PeekHead()
		require.NotNil(t, h1)
		h2, err := skl.PopHead()
		require.NoError(t, err)
		require.NotNil(t, h2)
		if mode == unique {
			require.Equal(t, expected[i*int64(size2)+9].w, h1.Key())
			require.Equal(t, expected[i*int64(size2)+9].id, h1.Val())
		} else {
			require.Equal(t, expected[i].w, h1.Key())
			require.Equal(t, expected[i].id, h1.Val())
		}
		require.Equal(t, h1.Key(), h2.Key())
		require.Equal(t, h1.Val(), h2.Val())
	}
}

func TestXConcSklPeekAndPopHead(t *testing.T) {
	type testcase struct {
		name string
		typ  xNodeMode
	}
	testcases := []testcase{
		{
			name: "skl lock free mutex data race - unique",
			typ:  unique,
		},
		{
			name: "skl lock free mutex data race - linkedlist",
			typ:  linkedList,
		},

		{
			name: "skl lock free mutex data race - rbtree",
			typ:  rbtree,
		},
		{
			name: "skl lock free mutex data race - rbtree (succ)",
			typ:  rbtree,
		},
	}
	t.Parallel()
	for _, tc := range testcases {
		t.Run(tc.name, func(tt *testing.T) {
			xConcSklPeekAndPopHeadRunCore(tt, tc.typ)
		})
	}
}

func BenchmarkXConcSklUnique_Random(b *testing.B) {
	testByBytes := []byte(`abc`)

	b.StopTimer()
	opts := make([]SklOption[int, []byte], 0, 2)
	opts = append(opts, WithXConcSklDataNodeUniqueMode[int, []byte]())
	skl, err := NewSkl[int, []byte](
		XConcSkl,
		opts...,
	)
	if err != nil {
		panic(err)
	}

	rngArr := make([]int, 0, b.N)
	for i := 0; i < b.N; i++ {
		rngArr = append(rngArr, randv2.Int())
	}

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		err := skl.Insert(rngArr[i], testByBytes)
		if err != nil {
			panic(err)
		}
	}
}

func BenchmarkXConcSklUnique_Serial(b *testing.B) {
	testByBytes := []byte(`abc`)

	b.StopTimer()
	opts := make([]SklOption[int, []byte], 0, 2)
	opts = append(opts, WithXConcSklDataNodeUniqueMode[int, []byte]())
	skl, err := NewSkl[int, []byte](
		XConcSkl,
		opts...,
	)
	if err != nil {
		panic(err)
	}

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		skl.Insert(i, testByBytes)
	}
}

func TestXConcSklUnique(t *testing.T) {
	testByBytes := []byte(`abc`)

	opts := make([]SklOption[int, []byte], 0, 2)
	opts = append(opts, WithXConcSklDataNodeUniqueMode[int, []byte]())
	skl, err := NewSkl[int, []byte](
		XConcSkl,
		opts...,
	)
	if err != nil {
		panic(err)
	}

	for i := 0; i < 3000000; i++ {
		skl.Insert(i, testByBytes)
	}
}

func BenchmarkXSklReadWrite(b *testing.B) {
	value := []byte(`abc`)
	for i := 0; i <= 10; i++ {
		b.Run(fmt.Sprintf("frac_%d", i), func(b *testing.B) {
			readFrac := float32(i) / 10.0
			opts := make([]SklOption[int, []byte], 0, 2)
			opts = append(opts, WithXConcSklDataNodeUniqueMode[int, []byte]())
			skl, err := NewSkl[int, []byte](
				XConcSkl,
				opts...,
			)
			if err != nil {
				panic(err)
			}
			b.ResetTimer()
			count := atomic.Int64{}
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					if randv2.Float32() < readFrac {
						v, err := skl.LoadFirst(randv2.Int())
						if err != nil && v != nil {
							count.Add(1)
						}
					} else {
						skl.Insert(randv2.Int(), value)
					}
				}
			})
		})
	}
}
