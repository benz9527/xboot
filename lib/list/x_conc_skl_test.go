package list

import (
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/benz9527/xboot/lib/id"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func xConcSkipListSerialProcessingRunCore(t *testing.T, me mutexImpl) {
	skl := &xConcSkl[uint64, *xSkipListObject]{
		head:    newXConcSklHead[uint64, *xSkipListObject](me, unique),
		pool:    newXConcSklPool[uint64, *xSkipListObject](),
		levels:  1,
		nodeLen: 0,
		kcmp: func(i, j uint64) int64 {
			res := int64(i - j)
			return res
		},
		vcmp: func(i, j *xSkipListObject) int64 {
			return int64(i.Hash() - j.Hash())
		},
		rand:  randomLevelV3,
		flags: flagBits{},
	}
	idGen, _ := id.MonotonicNonZeroID()
	skl.idGen = idGen
	skl.flags.setBitsAs(sklMutexImplBits, uint32(me))

	size := 5
	for i := uint64(0); i < uint64(size); i++ {
		for j := uint64(0); j < 10; j++ {
			w := (i+1)*100 + j
			_ = skl.Insert(w, &xSkipListObject{id: fmt.Sprintf("%d", w)})
		}
	}
	t.Logf("nodeLen: %d, indexCount: %d\n", skl.Len(), skl.IndexCount())

	skl.Foreach(func(idx int64, item SkipListIterationItem[uint64, *xSkipListObject]) bool {
		//t.Logf("idx: %d, key: %v, value: %v, levels: %d, count: %d\n", idx, item.Key(), item.Val(), item.NodeLevel(), item.NodeItemCount())
		return true
	})

	obj, ok := skl.LoadFirst(401)
	require.True(t, ok)
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

func TestXConcSkipList_SerialProcessing(t *testing.T) {
	type testcase struct {
		name string
		me   mutexImpl
	}
	testcases := []testcase{
		{
			name: "go native sync mutex",
			me:   goNativeMutex,
		}, {
			name: "skl lock free mutex",
			me:   xSklLockFree,
		},
	}
	t.Parallel()
	for _, tc := range testcases {
		t.Run(tc.name, func(tt *testing.T) {
			xConcSkipListSerialProcessingRunCore(tt, tc.me)
		})
	}
}

func xConcSkipListDataRaceRunCore(t *testing.T, mu mutexImpl) {
	skl := &xConcSkl[uint64, *xSkipListObject]{
		head:    newXConcSklHead[uint64, *xSkipListObject](mu, unique),
		pool:    newXConcSklPool[uint64, *xSkipListObject](),
		levels:  1,
		nodeLen: 0,
		kcmp: func(i, j uint64) int64 {
			// avoid calculation overflow
			if i == j {
				return 0
			} else if i > j {
				return 1
			}
			return -1
		},
		vcmp: func(i, j *xSkipListObject) int64 {
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
		flags: flagBits{},
	}
	idGen, _ := id.MonotonicNonZeroID()
	skl.idGen = idGen
	skl.flags.setBitsAs(sklMutexImplBits, uint32(mu))

	size := 5
	size2 := 10
	var wg sync.WaitGroup
	wg.Add(size * size2)
	for i := uint64(0); i < uint64(size); i++ {
		for j := uint64(0); j < uint64(size2); j++ {
			go func(idx uint64) {
				w := idx
				time.Sleep(time.Duration(cryptoRandUint32()%5) * time.Millisecond)
				_ = skl.Insert(w, &xSkipListObject{id: fmt.Sprintf("%d", w)})
				wg.Done()
			}((i+1)*100 + j)
		}
	}
	wg.Wait()
	t.Logf("nodeLen: %d, indexCount: %d\n", skl.Len(), skl.IndexCount())

	obj, ok := skl.LoadFirst(401)
	require.True(t, ok)
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

func TestXConcSkipList_DataRace(t *testing.T) {
	type testcase struct {
		name string
		mu   mutexImpl
	}
	testcases := []testcase{
		{
			name: "go native sync mutex data race -- unique",
			mu:   goNativeMutex,
		}, {
			name: "skl lock free mutex data race -- unique",
			mu:   xSklLockFree,
		},
	}
	t.Parallel()
	for _, tc := range testcases {
		t.Run(tc.name, func(tt *testing.T) {
			xConcSkipListDataRaceRunCore(tt, tc.mu)
		})
	}
}

func TestXConcSkipListDuplicate_SerialProcessing(t *testing.T) {
	skl := &xConcSkl[uint64, *xSkipListObject]{
		head:    newXConcSklHead[uint64, *xSkipListObject](goNativeMutex, linkedList),
		pool:    newXConcSklPool[uint64, *xSkipListObject](),
		levels:  1,
		nodeLen: 0,
		kcmp: func(i, j uint64) int64 {
			// avoid calculation overflow
			if i == j {
				return 0
			} else if i > j {
				return 1
			}
			return -1
		},
		vcmp: func(i, j *xSkipListObject) int64 {
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
		flags: flagBits{},
	}
	idGen, _ := id.MonotonicNonZeroID()
	skl.idGen = idGen
	skl.flags.setBitsAs(sklMutexImplBits, uint32(goNativeMutex))
	skl.flags.setBitsAs(sklXNodeModeBits, uint32(linkedList))

	skl.Insert(4, &xSkipListObject{id: fmt.Sprintf("%d", 9)})
	skl.Insert(4, &xSkipListObject{id: fmt.Sprintf("%d", 5)})
	skl.Insert(4, &xSkipListObject{id: fmt.Sprintf("%d", 8)})
	skl.Insert(4, &xSkipListObject{id: fmt.Sprintf("%d", 7)})
	skl.Insert(4, &xSkipListObject{id: fmt.Sprintf("%d", 1)})
	skl.Insert(4, &xSkipListObject{id: fmt.Sprintf("%d", 2)})
	skl.Insert(4, &xSkipListObject{id: fmt.Sprintf("%d", 4)})
	skl.Insert(4, &xSkipListObject{id: fmt.Sprintf("%d", 6)})
	skl.Insert(2, &xSkipListObject{id: fmt.Sprintf("%d", 9)})
	skl.Insert(2, &xSkipListObject{id: fmt.Sprintf("%d", 5)})
	skl.Insert(2, &xSkipListObject{id: fmt.Sprintf("%d", 8)})
	skl.Insert(2, &xSkipListObject{id: fmt.Sprintf("%d", 7)})
	skl.Insert(2, &xSkipListObject{id: fmt.Sprintf("%d", 1)})
	skl.Insert(2, &xSkipListObject{id: fmt.Sprintf("%d", 2)})
	skl.Insert(2, &xSkipListObject{id: fmt.Sprintf("%d", 4)})
	skl.Insert(2, &xSkipListObject{id: fmt.Sprintf("%d", 6)})
	skl.Insert(3, &xSkipListObject{id: fmt.Sprintf("%d", 100)})
	skl.Insert(3, &xSkipListObject{id: fmt.Sprintf("%d", 200)})
	skl.Insert(3, &xSkipListObject{id: fmt.Sprintf("%d", 2)})
	skl.Insert(1, &xSkipListObject{id: fmt.Sprintf("%d", 9)})
	skl.Insert(1, &xSkipListObject{id: fmt.Sprintf("%d", 200)})
	skl.Insert(1, &xSkipListObject{id: fmt.Sprintf("%d", 2)})

	t.Logf("nodeLen: %d, indexCount: %d\n", skl.Len(), skl.IndexCount())

	skl.Foreach(func(idx int64, item SkipListIterationItem[uint64, *xSkipListObject]) bool {
		t.Logf("idx: %d, key: %v, value: %v, levels: %d, count: %d\n", idx, item.Key(), item.Val(), item.NodeLevel(), item.NodeItemCount())
		return true
	})

	aux := make(xConcSklAux[uint64, *xSkipListObject], 2*xSkipListMaxLevel)
	foundResult := skl.rmTraverse(1, aux)
	assert.LessOrEqual(t, int32(0), foundResult)
	require.True(t, aux.loadPred(0).flags.isSet(nodeIsHeadFlagBit))
	require.Equal(t, uint64(1), aux.loadSucc(0).key)
	require.Equal(t, "9", (*aux.loadSucc(0).atomicLoadRoot().linkedListNext().vptr).id)

	foundResult = skl.rmTraverse(2, aux)
	assert.LessOrEqual(t, int32(0), foundResult)
	require.Equal(t, uint64(1), aux.loadPred(0).key)
	require.Equal(t, "9", (*aux.loadPred(0).atomicLoadRoot().linkedListNext().vptr).id)
	require.Equal(t, uint64(2), aux.loadSucc(0).key)
	require.Equal(t, "9", (*aux.loadSucc(0).atomicLoadRoot().linkedListNext().vptr).id)

	foundResult = skl.rmTraverse(3, aux)
	assert.LessOrEqual(t, int32(0), foundResult)
	require.Equal(t, uint64(2), aux.loadPred(0).key)
	require.Equal(t, "9", (*aux.loadPred(0).atomicLoadRoot().linkedListNext().vptr).id)
	require.Equal(t, uint64(3), aux.loadSucc(0).key)
	require.Equal(t, "2", (*aux.loadSucc(0).atomicLoadRoot().linkedListNext().vptr).id)

	foundResult = skl.rmTraverse(4, aux)
	assert.LessOrEqual(t, int32(0), foundResult)
	require.Equal(t, uint64(3), aux.loadPred(0).key)
	require.Equal(t, "2", (*aux.loadPred(0).atomicLoadRoot().linkedListNext().vptr).id)
	require.Equal(t, uint64(4), aux.loadSucc(0).key)
	require.Equal(t, "9", (*aux.loadSucc(0).atomicLoadRoot().linkedListNext().vptr).id)

	foundResult = skl.rmTraverse(100, aux)
	assert.Equal(t, int32(-1), foundResult)

	foundResult = skl.rmTraverse(0, aux)
	assert.Equal(t, int32(-1), foundResult)

}

func xConcSkipListDuplicateDataRaceRunCore(t *testing.T, mu mutexImpl, mode xNodeMode, rmBySucc bool) {
	skl := &xConcSkl[uint64, int64]{
		head:    newXConcSklHead[uint64, int64](mu, mode),
		pool:    newXConcSklPool[uint64, int64](),
		levels:  1,
		nodeLen: 0,
		kcmp: func(i, j uint64) int64 {
			// avoid calculation overflow
			if i == j {
				return 0
			} else if i > j {
				return 1
			}
			return -1
		},
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
		flags: flagBits{},
	}
	idGen, _ := id.MonotonicNonZeroID()
	skl.idGen = idGen
	skl.flags.setBitsAs(sklMutexImplBits, uint32(mu))
	skl.flags.setBitsAs(sklXNodeModeBits, uint32(mode))
	if mode == rbtree && rmBySucc {
		skl.flags.set(sklRbtreeRmReplaceFnFlagBit)
	}

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

	skl.Foreach(func(idx int64, item SkipListIterationItem[uint64, int64]) bool {
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

func TestXConcSkipListDuplicate_DataRace(t *testing.T) {
	type testcase struct {
		name       string
		mu         mutexImpl
		typ        xNodeMode
		rbRmBySucc bool
	}
	testcases := []testcase{
		{
			name: "go native sync mutex data race - linkedlist",
			mu:   goNativeMutex,
			typ:  linkedList,
		},
		{
			name: "skl lock free mutex data race - linkedlist",
			mu:   xSklLockFree,
			typ:  linkedList,
		},
		{
			name: "go native sync mutex data race - rbtree",
			mu:   goNativeMutex,
			typ:  rbtree,
		},
		{
			name: "skl lock free mutex data race - rbtree",
			mu:   xSklLockFree,
			typ:  rbtree,
		},
		{
			name:       "go native sync mutex data race - rbtree (succ)",
			mu:         goNativeMutex,
			typ:        rbtree,
			rbRmBySucc: true,
		},
		{
			name:       "skl lock free mutex data race - rbtree (succ)",
			mu:         xSklLockFree,
			typ:        rbtree,
			rbRmBySucc: true,
		},
	}
	t.Parallel()
	for _, tc := range testcases {
		t.Run(tc.name, func(tt *testing.T) {
			xConcSkipListDuplicateDataRaceRunCore(tt, tc.mu, tc.typ, tc.rbRmBySucc)
		})
	}
}

func xConcSklPeekAndPopHeadRunCore(t *testing.T, mu mutexImpl, mode xNodeMode) {
	skl := &xConcSkl[uint64, int64]{
		head:    newXConcSklHead[uint64, int64](mu, mode),
		pool:    newXConcSklPool[uint64, int64](),
		levels:  1,
		nodeLen: 0,
		kcmp: func(i, j uint64) int64 {
			// avoid calculation overflow
			if i == j {
				return 0
			} else if i > j {
				return 1
			}
			return -1
		},
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
		flags: flagBits{},
	}
	idGen, _ := id.MonotonicNonZeroID()
	skl.idGen = idGen
	if mode != unique {
		skl.flags.setBitsAs(sklMutexImplBits, uint32(mu))
		skl.flags.setBitsAs(sklXNodeModeBits, uint32(mode))
	}

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
		skl.Foreach(func(idx int64, item SkipListIterationItem[uint64, int64]) bool {
			require.Equalf(t, expected[idx*int64(size2)+9].w, item.Key(), "idx: %d; exp: %d; actual: %d\n", idx, expected[idx*int64(size2)+9].w, item.Key())
			require.Equalf(t, expected[idx*int64(size2)+9].id, item.Val(), "idx: %d; exp: %d; actual: %d\n", idx, expected[idx*int64(size2)+9].id, item.Val())
			return true
		})
	} else {
		skl.Foreach(func(idx int64, item SkipListIterationItem[uint64, int64]) bool {
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
		mu   mutexImpl
		typ  xNodeMode
	}
	testcases := []testcase{
		{
			name: "go native sync mutex data race - unique",
			mu:   goNativeMutex,
			typ:  unique,
		},
		{
			name: "skl lock free mutex data race - unique",
			mu:   xSklLockFree,
			typ:  unique,
		},
		{
			name: "go native sync mutex data race - linkedlist",
			mu:   goNativeMutex,
			typ:  linkedList,
		},
		{
			name: "skl lock free mutex data race - linkedlist",
			mu:   xSklLockFree,
			typ:  linkedList,
		},
		{
			name: "go native sync mutex data race - rbtree",
			mu:   goNativeMutex,
			typ:  rbtree,
		},
		{
			name: "skl lock free mutex data race - rbtree",
			mu:   xSklLockFree,
			typ:  rbtree,
		},
		{
			name: "go native sync mutex data race - rbtree (succ)",
			mu:   goNativeMutex,
			typ:  rbtree,
		},
		{
			name: "skl lock free mutex data race - rbtree (succ)",
			mu:   xSklLockFree,
			typ:  rbtree,
		},
	}
	t.Parallel()
	for _, tc := range testcases {
		t.Run(tc.name, func(tt *testing.T) {
			xConcSklPeekAndPopHeadRunCore(tt, tc.mu, tc.typ)
		})
	}
}
