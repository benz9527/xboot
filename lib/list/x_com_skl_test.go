package list

import (
	"cmp"
	"errors"
	"hash/fnv"
	"sort"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type HashObject interface {
	comparable
	Hash() uint64
}

type emptyHashObject struct{}

func (o *emptyHashObject) Hash() uint64 { return 0 }

type SkipListWeight interface {
	cmp.Ordered
	// ~uint8 == byte
}

type xSklObject struct {
	id string
}

func (o *xSklObject) Hash() uint64 {
	if o == nil {
		return 0
	}
	hash := fnv.New64()
	_, _ = hash.Write([]byte(o.id))
	val := hash.Sum64()
	return val
}

func TestStringHash_FNV(t *testing.T) {
	s1, s2 := "1", "2"
	hash := fnv.New64()
	_, _ = hash.Write([]byte(s1))
	v1 := hash.Sum64()
	assert.Equal(t, uint64(12638153115695167470), v1)
	hash = fnv.New64()
	_, _ = hash.Write([]byte(s2))
	v2 := hash.Sum64()
	assert.Equal(t, uint64(12638153115695167469), v2)
	res := int64(v2 - v1)
	assert.Equal(t, res, int64(-1))

	s1, s2 = "100", "200"
	hash = fnv.New64()
	_, _ = hash.Write([]byte(s1))
	v1 = hash.Sum64()
	hash = fnv.New64()
	_, _ = hash.Write([]byte(s2))
	v2 = hash.Sum64()
	assert.Greater(t, v1, v2)
}

func TestXComSkl_SimpleCRUD(t *testing.T) {
	type element struct {
		w  int
		id int
	}
	orders := []element{
		{1, 9}, {1, 8}, {1, 7}, {1, 6}, {1, 5}, {1, 4}, {1, 3}, {1, 2}, {1, 1},
		{2, 19}, {2, 18}, {2, 17}, {2, 16}, {2, 15}, {2, 14}, {2, 13}, {2, 12}, {2, 11},
		{4, 29}, {4, 28}, {4, 27}, {4, 26}, {4, 25}, {4, 24}, {4, 23}, {4, 22}, {4, 21},
		{8, 39}, {8, 38}, {8, 37}, {8, 36}, {8, 35}, {8, 34}, {8, 33}, {8, 32}, {8, 31},
		{16, 49}, {16, 48}, {16, 47}, {16, 46}, {16, 45}, {16, 44}, {16, 43}, {16, 42}, {16, 41},
		{32, 59}, {32, 58}, {32, 57}, {32, 56}, {32, 55}, {32, 54}, {32, 53}, {32, 52}, {32, 51},
		{64, 69}, {64, 68}, {64, 67}, {64, 66}, {64, 65}, {64, 64}, {64, 63}, {64, 62}, {64, 61},
		{128, 79}, {128, 78}, {128, 77}, {128, 76}, {128, 75}, {128, 74}, {128, 73}, {128, 72}, {128, 71},
	}

	skl := NewXSkl[int, int](
		XComSkl,
		func(i, j int) int64 {
			if i == j {
				return 0
			} else if i < j {
				return -1
			}
			return 1
		},
		WithXComSklValComparator[int, int](
			func(i, j int) int64 {
				if i == j {
					return 0
				} else if i > j {
					return -1
				}
				return 1
			}),
		WithSklRandLevelGen[int, int](randomLevelV2),
	)

	_, err := skl.RemoveAll(1)
	require.True(t, errors.Is(err, ErrXSklIsEmpty))

	for _, o := range orders {
		_ = skl.Insert(o.w, o.id)
	}

	err = skl.Insert(1, 2)
	require.NoError(t, err)
	skl.Foreach(func(i int64, item SklIterationItem[int, int]) bool {
		require.Equal(t, orders[i].w, item.Key())
		require.Equal(t, orders[i].id, item.Val())
		t.Logf("key: %d, levels: %d\n", item.Key(), item.NodeLevel())
		return true
	})
	assert.Equal(t, int64(len(orders)), skl.Len())

	expectedFirstList := []element{
		{1, 9},
		{2, 19},
		{4, 29},
		{8, 39},
		{16, 49},
		{32, 59},
		{64, 69},
		{128, 79},
	}
	for _, first := range expectedFirstList {
		ele, err := skl.LoadFirst(first.w)
		require.NoError(t, err)
		assert.NotNil(t, ele)
		assert.Equal(t, first.w, ele.Key())
		assert.Equal(t, first.id, ele.Val())
	}

	var ele SklElement[int, int]
	ele, err = skl.RemoveFirst(4)
	require.NoError(t, err)
	assert.NotNil(t, ele)
	assert.Equal(t, 4, ele.Key())
	assert.Equal(t, 29, ele.Val())

	var eleList []SklElement[int, int]
	eleList, err = skl.RemoveAll(4)
	assert.NotNil(t, eleList)
	expectedRemoveList := []element{
		{4, 28}, {4, 27}, {4, 26}, {4, 25}, {4, 24}, {4, 23}, {4, 22}, {4, 21},
	}
	assert.Equal(t, len(expectedRemoveList), len(eleList))
	for i, e := range expectedRemoveList {
		assert.Equal(t, e.w, eleList[i].Key())
		assert.Equal(t, e.id, eleList[i].Val())
	}

	orders = []element{
		{1, 9}, {1, 8}, {1, 7}, {1, 6}, {1, 5}, {1, 4}, {1, 3}, {1, 2}, {1, 1},
		{2, 19}, {2, 18}, {2, 17}, {2, 16}, {2, 15}, {2, 14}, {2, 13}, {2, 12}, {2, 11},
		{8, 39}, {8, 38}, {8, 37}, {8, 36}, {8, 35}, {8, 34}, {8, 33}, {8, 32}, {8, 31},
		{16, 49}, {16, 48}, {16, 47}, {16, 46}, {16, 45}, {16, 44}, {16, 43}, {16, 42}, {16, 41},
		{32, 59}, {32, 58}, {32, 57}, {32, 56}, {32, 55}, {32, 54}, {32, 53}, {32, 52}, {32, 51},
		{64, 69}, {64, 68}, {64, 67}, {64, 66}, {64, 65}, {64, 64}, {64, 63}, {64, 62}, {64, 61},
		{128, 79}, {128, 78}, {128, 77}, {128, 76}, {128, 75}, {128, 74}, {128, 73}, {128, 72}, {128, 71},
	}
	skl.Foreach(func(i int64, item SklIterationItem[int, int]) bool {
		assert.Equal(t, orders[i].w, item.Key())
		assert.Equal(t, orders[i].id, item.Val())
		return true
	})
	assert.Equal(t, int64(len(orders)), skl.Len())

	expectedFirstList = []element{
		{1, 9},
		{2, 19},
		{8, 39},
		{16, 49},
		{32, 59},
		{64, 69},
		{128, 79},
	}
	for _, first := range expectedFirstList {
		var err error
		ele, err = skl.LoadFirst(first.w)
		require.NoError(t, err)
		assert.NotNil(t, ele)
		assert.Equal(t, first.w, ele.Key())
		assert.Equal(t, first.id, ele.Val())
	}

	expectedRemoveList = []element{
		{16, 47}, {8, 35}, {128, 71},
	}
	for _, e := range expectedRemoveList {
		eleList, err = skl.RemoveIfMatched(e.w, func(that int) bool {
			return that == e.id
		})
		assert.NotNil(t, eleList)
		assert.Equal(t, 1, len(eleList))
		assert.Equal(t, e.w, eleList[0].Key())
		assert.Equal(t, e.id, eleList[0].Val())
	}

	orders = []element{
		{1, 9}, {1, 8}, {1, 7}, {1, 6}, {1, 5}, {1, 4}, {1, 3}, {1, 2}, {1, 1},
		{2, 19}, {2, 18}, {2, 17}, {2, 16}, {2, 15}, {2, 14}, {2, 13}, {2, 12}, {2, 11},
		{8, 39}, {8, 38}, {8, 37}, {8, 36}, {8, 34}, {8, 33}, {8, 32}, {8, 31},
		{16, 49}, {16, 48}, {16, 46}, {16, 45}, {16, 44}, {16, 43}, {16, 42}, {16, 41},
		{32, 59}, {32, 58}, {32, 57}, {32, 56}, {32, 55}, {32, 54}, {32, 53}, {32, 52}, {32, 51},
		{64, 69}, {64, 68}, {64, 67}, {64, 66}, {64, 65}, {64, 64}, {64, 63}, {64, 62}, {64, 61},
		{128, 79}, {128, 78}, {128, 77}, {128, 76}, {128, 75}, {128, 74}, {128, 73}, {128, 72},
	}
	skl.Foreach(func(i int64, item SklIterationItem[int, int]) bool {
		assert.Equal(t, orders[i].w, item.Key())
		assert.Equal(t, orders[i].id, item.Val())
		return true
	})
	assert.Equal(t, int64(len(orders)), skl.Len())

	expectedFindList := []element{
		{1, 9},
		{2, 19},
		{8, 39},
		{16, 49},
		{32, 59},
		{64, 69},
		{128, 79},
	}
	for _, e := range expectedFindList {
		eleList, err = skl.LoadIfMatched(e.w, func(obj int) bool {
			return obj == e.id
		})
		require.NoError(t, err)
		assert.NotNil(t, eleList)
		assert.Equal(t, 1, len(eleList))
		assert.Equal(t, e.w, eleList[0].Key())
		assert.Equal(t, e.id, eleList[0].Val())
	}

	expectedFindList = []element{
		{4, 20},
		{100, 100},
		{129, 77},
	}
	for _, e := range expectedFindList {
		eleList, err = skl.LoadIfMatched(e.w, func(obj int) bool {
			return obj == e.id
		})
		require.Error(t, err)
		assert.Zero(t, len(eleList))
	}

	expectedFindList = []element{
		{64, 69}, {64, 68}, {64, 67}, {64, 66}, {64, 65}, {64, 64}, {64, 63}, {64, 62}, {64, 61},
	}
	eleList, err = skl.LoadAll(64)
	require.NoError(t, err)
	assert.NotZero(t, len(eleList))
	for i, e := range eleList {
		assert.Equal(t, expectedFindList[i].w, e.Key())
		assert.Equal(t, expectedFindList[i].id, e.Val())
	}
}

func TestXComSkl_PopHead(t *testing.T) {
	type element struct {
		w  int
		id string
	}

	count := 1000
	elements := make([]element, 0, count)
	for i := 0; i < count; i++ {
		w := int(cryptoRandUint32())
		elements = append(elements, element{w: w, id: strconv.Itoa(w)})
	}

	skl := NewSkl[int, *xSklObject](
		XComSkl,
		func(i, j int) int64 {
			if i == j {
				return 0
			} else if i < j {
				return -1
			}
			return 1
		},
		WithSklRandLevelGen[int, *xSklObject](randomLevelV2),
	)

	for _, o := range elements {
		skl.Insert(o.w, &xSklObject{id: o.id})
	}

	sort.Slice(elements, func(i, j int) bool {
		return elements[i].w < elements[j].w
	})

	for i := 0; i < len(elements); i++ {
		ele, err := skl.PopHead()
		require.NoError(t, err)
		assert.Equal(t, elements[i].w, ele.Key())
		assert.Equal(t, elements[i].id, ele.Val().id)
		restOrders := elements[i+1:]
		ii := 0
		skl.Foreach(func(i int64, item SklIterationItem[int, *xSklObject]) bool {
			assert.Equal(t, restOrders[ii].w, item.Key())
			assert.Equal(t, restOrders[ii].id, item.Val().id)
			ii++
			return true
		})
	}
}

func TestXComSkl_Duplicate_PopHead(t *testing.T) {
	type element struct {
		w  int
		id string
	}
	orders := []element{
		{1, "3"}, {1, "2"}, {1, "1"},
		{2, "4"}, {2, "2"},
		{3, "9"}, {3, "8"}, {3, "7"}, {3, "1"},
		{4, "9"}, {4, "6"}, {4, "3"},
		{5, "7"}, {5, "6"}, {5, "2"},
		{6, "8"}, {6, "100"},
		{7, "8"}, {7, "7"}, {7, "2"}, {7, "1"},
	}

	skl := NewXSkl[int, *xSklObject](
		XComSkl,
		func(i, j int) int64 {
			if i == j {
				return 0
			} else if i < j {
				return -1
			}
			return 1
		},
		WithXComSklValComparator[int, *xSklObject](
			func(i, j *xSklObject) int64 {
				_i, _j := i.Hash(), j.Hash()
				if _i == _j {
					return 0
				} else if _i < _j {
					return -1
				}
				return 1
			}),
		WithSklRandLevelGen[int, *xSklObject](randomLevelV3),
	)

	for _, o := range orders {
		skl.Insert(o.w, &xSklObject{id: o.id})
	}
	for i := 0; i < len(orders); i++ {
		ele, err := skl.PopHead()
		require.NoError(t, err)
		assert.Equal(t, orders[i].w, ele.Key())
		assert.Equal(t, orders[i].id, ele.Val().id)
		restOrders := orders[i+1:]
		ii := 0
		skl.Foreach(func(i int64, item SklIterationItem[int, *xSklObject]) bool {
			assert.Equal(t, restOrders[ii].w, item.Key())
			assert.Equal(t, restOrders[ii].id, item.Val().id)
			ii++
			return true
		})
	}
}

func TestXComSklDuplicateDataRace(t *testing.T) {
	opts := []SklOption[uint64, int64]{
		WithSklRandLevelGen[uint64, int64](randomLevelV3),
		WithXComSklEnableConc[uint64, int64](),
		WithXComSklValComparator[uint64, int64](
			func(i, j int64) int64 {
				// avoid calculation overflow
				if i == j {
					return 0
				} else if i > j {
					return 1
				}
				return -1
			},
		),
	}
	skl := NewXSkl[uint64, int64](
		XComSkl,
		func(i, j uint64) int64 {
			// avoid calculation overflow
			if i == j {
				return 0
			} else if i > j {
				return 1
			}
			return -1
		},
		opts...,
	)

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
