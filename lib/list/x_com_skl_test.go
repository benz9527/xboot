package list

import (
	"cmp"
	"errors"
	"hash/fnv"
	"testing"

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

type xSkipListObject struct {
	id string
}

func (o *xSkipListObject) Hash() uint64 {
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

func TestXSkipList_SimpleCRUD(t *testing.T) {
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
	xsl := newXComSkl[int, int](
		func(i, j int) int64 {
			if i == j {
				return 0
			} else if i < j {
				return -1
			}
			return 1
		},
		func(i, j int) int64 {
			if i == j {
				return 0
			} else if i > j {
				return -1
			}
			return 1
		},
		randomLevelV2,
	)

	_, err := xsl.RemoveAll(1)
	require.True(t, errors.Is(err, ErrXSklIsEmpty))

	for _, o := range orders {
		_ = xsl.Insert(o.w, o.id)
	}

	err = xsl.Insert(1, 2)
	require.NoError(t, err)
	xsl.Foreach(func(i int64, item SklIterationItem[int, int]) bool {
		require.Equal(t, orders[i].w, item.Key())
		require.Equal(t, orders[i].id, item.Val())
		t.Logf("key: %d, levels: %d\n", item.Key(), item.NodeLevel())
		return true
	})
	assert.Equal(t, int64(len(orders)), xsl.Len())

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
		ele, err := xsl.LoadFirst(first.w)
		require.NoError(t, err)
		assert.NotNil(t, ele)
		assert.Equal(t, first.w, ele.Key())
		assert.Equal(t, first.id, ele.Val())
	}

	var ele SklElement[int, int]
	ele, err = xsl.RemoveFirst(4)
	require.NoError(t, err)
	assert.NotNil(t, ele)
	assert.Equal(t, 4, ele.Key())
	assert.Equal(t, 29, ele.Val())

	var eleList []SklElement[int, int]
	eleList, err = xsl.RemoveAll(4)
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
	xsl.Foreach(func(i int64, item SklIterationItem[int, int]) bool {
		assert.Equal(t, orders[i].w, item.Key())
		assert.Equal(t, orders[i].id, item.Val())
		return true
	})
	assert.Equal(t, int64(len(orders)), xsl.Len())

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
		ele, err = xsl.LoadFirst(first.w)
		require.NoError(t, err)
		assert.NotNil(t, ele)
		assert.Equal(t, first.w, ele.Key())
		assert.Equal(t, first.id, ele.Val())
	}

	expectedRemoveList = []element{
		{16, 47}, {8, 35}, {128, 71},
	}
	for _, e := range expectedRemoveList {
		eleList, err = xsl.RemoveIfMatched(e.w, func(that int) bool {
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
	xsl.Foreach(func(i int64, item SklIterationItem[int, int]) bool {
		assert.Equal(t, orders[i].w, item.Key())
		assert.Equal(t, orders[i].id, item.Val())
		return true
	})
	assert.Equal(t, int64(len(orders)), xsl.Len())

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
		eleList, err = xsl.LoadIfMatched(e.w, func(obj int) bool {
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
		eleList, err = xsl.LoadIfMatched(e.w, func(obj int) bool {
			return obj == e.id
		})
		require.Error(t, err)
		assert.Zero(t, len(eleList))
	}

	expectedFindList = []element{
		{64, 69}, {64, 68}, {64, 67}, {64, 66}, {64, 65}, {64, 64}, {64, 63}, {64, 62}, {64, 61},
	}
	eleList, err = xsl.LoadAll(64)
	require.NoError(t, err)
	assert.NotZero(t, len(eleList))
	for i, e := range eleList {
		assert.Equal(t, expectedFindList[i].w, e.Key())
		assert.Equal(t, expectedFindList[i].id, e.Val())
	}
}

func TestNewXSkipList_PopHead(t *testing.T) {
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
	xsl := newXComSkl[int, *xSkipListObject](
		func(i, j int) int64 {
			if i == j {
				return 0
			} else if i < j {
				return -1
			}
			return 1
		},
		func(i, j *xSkipListObject) int64 {
			_i, _j := i.Hash(), j.Hash()
			if _i == _j {
				return 0
			} else if _i < _j {
				return -1
			}
			return 1
		},
		randomLevelV3,
	)

	for _, o := range orders {
		xsl.Insert(o.w, &xSkipListObject{id: o.id})
	}
	for i := 0; i < len(orders); i++ {
		ele, err := xsl.PopHead()
		require.NoError(t, err)
		assert.Equal(t, orders[i].w, ele.Key())
		assert.Equal(t, orders[i].id, ele.Val().id)
		restOrders := orders[i+1:]
		ii := 0
		xsl.Foreach(func(i int64, item SklIterationItem[int, *xSkipListObject]) bool {
			assert.Equal(t, restOrders[ii].w, item.Key())
			assert.Equal(t, restOrders[ii].id, item.Val().id)
			ii++
			return true
		})
	}
}
