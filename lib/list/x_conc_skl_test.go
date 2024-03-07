package list

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func xConcSkipListSerialProcessingRunCore(t *testing.T, me mutexEnum) {
	skl := &xConcSkipList[uint64, *xSkipListObject]{
		head:  newXConcSkipListHead[uint64, *xSkipListObject](me),
		pool:  newXConcSkipListPool[uint64, *xSkipListObject](),
		idxHi: 1,
		len:   0,
		cmp: func(i, j uint64) int {
			res := int(i - j)
			return res
		},
		rand:  randomLevelV3,
		flags: flagBits{},
		id:    newMonotonicNonZeroID(),
	}
	skl.flags.setBitsAs(sklMutexType, uint32(me))

	size := 5
	for i := uint64(0); i < uint64(size); i++ {
		for j := uint64(0); j < 10; j++ {
			w := (i+1)*100 + j
			skl.Insert(w, &xSkipListObject{id: fmt.Sprintf("%d", w)})
		}
	}
	t.Logf("len: %d, indexes: %d\n", skl.Len(), skl.Indexes())

	skl.Foreach(func(idx int64, w uint64, o *xSkipListObject) {
		t.Logf("idx: %d, w: %v, o: %v\n", idx, w, o)
	})

	obj, ok := skl.Get(401)
	require.True(t, ok)
	require.Equal(t, "401", obj.id)

	for i := uint64(0); i < uint64(size); i++ {
		for j := uint64(0); j < 10; j++ {
			w := (i+1)*100 + j
			skl.RemoveFirst(w)
		}
	}
	t.Logf("len: %d, indexes: %d\n", skl.Len(), skl.Indexes())
}

func TestXConcSkipList_SerialProcessing(t *testing.T) {
	type testcase struct {
		name string
		me   mutexEnum
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

func xConcSkipListDataRaceRunCore(t *testing.T, me mutexEnum) {
	h := newXConcSkipListNode[uint64, *xSkipListObject](0, nil, xSkipListMaxLevel, me)
	h.flags.atomicSet(nodeFullyLinked)
	skl := &xConcSkipList[uint64, *xSkipListObject]{
		head:  h,
		pool:  newXConcSkipListPool[uint64, *xSkipListObject](),
		idxHi: 1,
		len:   0,
		cmp: func(i, j uint64) int {
			res := int(i - j)
			return res
		},
		rand:  randomLevelV3,
		id:    newMonotonicNonZeroID(),
		flags: flagBits{},
	}
	skl.flags.setBitsAs(sklMutexType, uint32(me))

	size := 5
	size2 := 10
	var wg sync.WaitGroup
	wg.Add(size * size2)
	for i := uint64(0); i < uint64(size); i++ {
		for j := uint64(0); j < uint64(size2); j++ {
			go func(idx uint64) {
				w := idx
				time.Sleep(time.Duration(cryptoRandUint32()%5) * time.Millisecond)
				skl.Insert(w, &xSkipListObject{id: fmt.Sprintf("%d", w)})
				wg.Done()
			}((i+1)*100 + j)
		}
	}
	wg.Wait()
	t.Logf("len: %d, indexes: %d\n", skl.Len(), skl.Indexes())

	skl.Foreach(func(idx int64, w uint64, o *xSkipListObject) {
		t.Logf("idx: %d, w: %v, o: %v\n", idx, w, o)
	})

	obj, ok := skl.Get(401)
	require.True(t, ok)
	require.Equal(t, "401", obj.id)

	wg.Add(size * size2 * 2)
	for i := uint64(0); i < uint64(size); i++ {
		for j := uint64(0); j < uint64(size2); j++ {
			go func(idx uint64) {
				w := idx
				time.Sleep(time.Duration(cryptoRandUint32()%5) * time.Millisecond)
				skl.RemoveFirst(w)
				wg.Done()
			}((i+1)*100 + j)
			go func(idx uint64) {
				w := idx
				skl.Insert(w, &xSkipListObject{id: fmt.Sprintf("%d", w)})
				wg.Done()
			}((i+1)*666 + j)
		}
	}
	wg.Wait()
	t.Logf("len: %d, indexes: %d\n", skl.Len(), skl.Indexes())
	skl.Foreach(func(idx int64, w uint64, o *xSkipListObject) {
		t.Logf("idx: %d, w: %v, o: %v\n", idx, w, o)
	})
}

func TestXConcSkipList_DataRace(t *testing.T) {
	type testcase struct {
		name string
		me   mutexEnum
	}
	testcases := []testcase{
		{
			name: "go native sync mutex data race",
			me:   goNativeMutex,
		}, {
			name: "skl lock free mutex data race",
			me:   xSklLockFree,
		},
	}
	t.Parallel()
	for _, tc := range testcases {
		t.Run(tc.name, func(tt *testing.T) {
			xConcSkipListDataRaceRunCore(tt, tc.me)
		})
	}
}

func TestXConcSkipListDuplicate_SerialProcessing(t *testing.T) {
	skl := &xConcSkipList[uint64, *xSkipListObject]{
		head:  newXConcSkipListHead[uint64, *xSkipListObject](goNativeMutex),
		pool:  newXConcSkipListPool[uint64, *xSkipListObject](),
		idxHi: 1,
		len:   0,
		cmp: func(i, j uint64) int {
			res := int(i - j)
			return res
		},
		rand:  randomLevelV3,
		flags: flagBits{},
		id:    newMonotonicNonZeroID(),
	}
	skl.flags.setBitsAs(sklMutexType, uint32(goNativeMutex))
	skl.flags.atomicSet(sklDuplicate)

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

	t.Logf("len: %d, indexes: %d\n", skl.Len(), skl.Indexes())

	skl.Foreach(func(idx int64, w uint64, o *xSkipListObject) {
		t.Logf("idx: %d, w: %v, o: %v\n", idx, w, o)
	})

	aux := make(xConcSkipListAuxiliary[uint64, *xSkipListObject], 2*xSkipListMaxLevel)
	foundResult := skl.rmTraverse1(1, aux)
	assert.Less(t, int32(0), foundResult)
	require.True(t, aux.loadPred(0).flags.isSet(nodeHeadMarked))
	require.Equal(t, uint64(1), aux.loadSucc(0).weight)
	require.Equal(t, "9", (*aux.loadSucc(0).object).id)
	ele, ok := skl.remove1(1)
	assert.True(t, ok)
	assert.NotNil(t, ele)
	require.Equal(t, uint64(1), ele.Weight())
	require.Equal(t, "9", ele.Object().id)

	foundResult = skl.rmTraverse1(2, aux)
	assert.Less(t, int32(0), foundResult)
	require.Equal(t, uint64(1), aux.loadPred(0).weight)
	require.Equal(t, "200", (*aux.loadPred(0).object).id)
	require.Equal(t, uint64(2), aux.loadSucc(0).weight)
	require.Equal(t, "9", (*aux.loadSucc(0).object).id)

	foundResult = skl.rmTraverse1(3, aux)
	assert.Less(t, int32(0), foundResult)
	require.Equal(t, uint64(2), aux.loadPred(0).weight)
	require.Equal(t, "1", (*aux.loadPred(0).object).id)
	require.Equal(t, uint64(3), aux.loadSucc(0).weight)
	require.Equal(t, "2", (*aux.loadSucc(0).object).id)

	foundResult = skl.rmTraverse1(4, aux)
	assert.Less(t, int32(0), foundResult)
	require.Equal(t, uint64(3), aux.loadPred(0).weight)
	require.Equal(t, "100", (*aux.loadPred(0).object).id)
	require.Equal(t, uint64(4), aux.loadSucc(0).weight)
	require.Equal(t, "9", (*aux.loadSucc(0).object).id)

	foundResult = skl.rmTraverse1(100, aux)
	assert.Equal(t, int32(-1), foundResult)

	foundResult = skl.rmTraverse1(0, aux)
	assert.Equal(t, int32(-1), foundResult)

	skl.Foreach(func(idx int64, w uint64, o *xSkipListObject) {
		t.Logf("idx: %d, w: %v, o: %v\n", idx, w, o)
	})
}

func xConcSkipListDuplicateDataRaceRunCore(t *testing.T, me mutexEnum) {
	h := newXConcSkipListNode[uint64, *xSkipListObject](0, nil, xSkipListMaxLevel, me)
	h.flags.atomicSet(nodeFullyLinked)
	skl := &xConcSkipList[uint64, *xSkipListObject]{
		head:  h,
		pool:  newXConcSkipListPool[uint64, *xSkipListObject](),
		idxHi: 1,
		len:   0,
		cmp: func(i, j uint64) int {
			res := int(i - j)
			return res
		},
		rand:  randomLevelV3,
		id:    newMonotonicNonZeroID(),
		flags: flagBits{},
	}
	skl.flags.setBitsAs(sklMutexType, uint32(me))
	skl.flags.atomicSet(sklDuplicate)

	size := 5
	unorderedWeights := []uint64{9, 5, 8, 7, 1, 200, 2, 4, 6, 100}
	size2 := len(unorderedWeights)
	var wg sync.WaitGroup
	wg.Add(size * size2)

	type answer struct {
		w  uint64
		id string
	}
	expected := make([]*answer, 0, size*size2)
	orderedWeights := []uint64{9, 8, 7, 6, 5, 4, 2, 1, 200, 100}

	for i := uint64(0); i < uint64(size); i++ {
		for j := uint64(0); j < uint64(size2); j++ {
			go func(_i, _j uint64) {
				w := (_i + 1) * 100
				time.Sleep(time.Duration(cryptoRandUint32()%5) * time.Millisecond)
				skl.Insert(w, &xSkipListObject{id: fmt.Sprintf("%d", unorderedWeights[_j])})
				wg.Done()
			}(i, j)
			expected = append(expected, &answer{w: (i + 1) * 100, id: fmt.Sprintf("%d", orderedWeights[j])})
		}
	}
	wg.Wait()
	t.Logf("len: %d, indexes: %d\n", skl.Len(), skl.Indexes())

	skl.Foreach(func(idx int64, w uint64, o *xSkipListObject) {
		require.Equal(t, expected[idx].w, w)
		require.Equal(t, expected[idx].id, o.id)
	})

	//for _, e := range expected {
	//	skl.FindAll()
	//}
}

func TestXConcSkipListDuplicate_DataRace(t *testing.T) {
	type testcase struct {
		name string
		me   mutexEnum
	}
	testcases := []testcase{
		{
			name: "go native sync mutex data race",
			me:   goNativeMutex,
		}, {
			name: "skl lock free mutex data race",
			me:   xSklLockFree,
		},
	}
	t.Parallel()
	for _, tc := range testcases {
		t.Run(tc.name, func(tt *testing.T) {
			xConcSkipListDuplicateDataRaceRunCore(tt, tc.me)
		})
	}
}
