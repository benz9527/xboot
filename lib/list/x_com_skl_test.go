package list

import (
	"cmp"
	"github.com/stretchr/testify/require"
	"hash/fnv"
	"testing"

	"github.com/stretchr/testify/assert"
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
		randomLevelV2,
	)
	for _, o := range orders {
		_ = xsl.Insert(o.w, &xSkipListObject{id: o.id})
	}

	err := xsl.Insert(1, &xSkipListObject{id: "2"})
	require.NoError(t, err)
	xsl.Foreach(func(i int64, item SkipListIterationItem[int, *xSkipListObject]) bool {
		require.Equal(t, orders[i].w, item.Key())
		require.Equal(t, orders[i].id, item.Val().id)
		return true
	})
	assert.Equal(t, int64(len(orders)), xsl.Len())

	expectedFirstList := []element{
		{1, "3"},
		{2, "4"},
		{3, "9"},
		{4, "9"},
		{5, "7"},
		{6, "8"},
		{7, "8"},
	}
	for _, first := range expectedFirstList {
		ele, ok := xsl.LoadFirst(first.w)
		require.True(t, ok)
		assert.NotNil(t, ele)
		assert.Equal(t, first.w, ele.Key())
		assert.Equal(t, first.id, ele.Val().id)
	}

	var ele SkipListElement[int, *xSkipListObject]
	ele, err = xsl.RemoveFirst(4)
	require.NoError(t, err)
	assert.NotNil(t, ele)
	assert.Equal(t, 4, ele.Key())
	assert.Equal(t, "9", ele.Val().id)

	var eleList []SkipListElement[int, *xSkipListObject]
	eleList, err = xsl.RemoveAll(4)
	assert.NotNil(t, eleList)
	expectedRemoveList := []element{
		{4, "6"}, {4, "3"},
	}
	assert.Equal(t, len(expectedRemoveList), len(eleList))
	for i, e := range expectedRemoveList {
		assert.Equal(t, e.w, eleList[i].Key())
		assert.Equal(t, e.id, eleList[i].Val().id)
	}

	orders = []element{
		{1, "3"}, {1, "2"}, {1, "1"},
		{2, "4"}, {2, "2"},
		{3, "9"}, {3, "8"}, {3, "7"}, {3, "1"},
		{5, "7"}, {5, "6"}, {5, "2"},
		{6, "8"}, {6, "100"},
		{7, "8"}, {7, "7"}, {7, "2"}, {7, "1"},
	}
	xsl.Foreach(func(i int64, item SkipListIterationItem[int, *xSkipListObject]) bool {
		assert.Equal(t, orders[i].w, item.Key())
		assert.Equal(t, orders[i].id, item.Val().id)
		return true
	})
	assert.Equal(t, int32(len(orders)), xsl.Len())

	expectedFirstList = []element{
		{1, "3"},
		{2, "4"},
		{3, "9"},
		{5, "7"},
		{6, "8"},
		{7, "8"},
	}
	for _, first := range expectedFirstList {
		var ok bool
		ele, ok = xsl.LoadFirst(first.w)
		require.True(t, ok)
		assert.NotNil(t, ele)
		assert.Equal(t, first.w, ele.Key())
		assert.Equal(t, first.id, ele.Val().id)
	}

	expectedRemoveList = []element{
		{7, "2"}, {5, "6"}, {3, "8"},
	}
	for _, e := range expectedRemoveList {
		eleList, err = xsl.RemoveIfMatched(e.w, func(that *xSkipListObject) bool {
			return that.id == e.id
		})
		assert.NotNil(t, eleList)
		assert.Equal(t, 1, len(eleList))
		assert.Equal(t, e.w, eleList[0].Key())
		assert.Equal(t, e.id, eleList[0].Val().id)
	}

	orders = []element{
		{1, "3"}, {1, "2"}, {1, "1"},
		{2, "4"}, {2, "2"},
		{3, "9"}, {3, "7"}, {3, "1"},
		{5, "7"}, {5, "2"},
		{6, "8"}, {6, "100"},
		{7, "8"}, {7, "7"}, {7, "1"},
	}
	xsl.Foreach(func(i int64, item SkipListIterationItem[int, *xSkipListObject]) bool {
		assert.Equal(t, orders[i].w, item.Key())
		assert.Equal(t, orders[i].id, item.Val().id)
		return true
	})
	assert.Equal(t, int32(len(orders)), xsl.Len())

	expectedFindList := []element{
		{7, "7"}, {6, "100"}, {3, "7"},
	}
	for _, e := range expectedFindList {
		eleList, err = xsl.LoadIfMatched(e.w, func(obj *xSkipListObject) bool {
			return obj.id == e.id
		})
		require.NoError(t, err)
		assert.NotNil(t, eleList)
		assert.Equal(t, 1, len(eleList))
		assert.Equal(t, e.w, eleList[0].Key())
		assert.Equal(t, e.id, eleList[0].Val().id)
	}

	expectedFindList = []element{
		{7, "2"}, {5, "6"}, {3, "8"},
	}
	for _, e := range expectedFindList {
		eleList, err = xsl.LoadIfMatched(e.w, func(obj *xSkipListObject) bool {
			return obj.id == e.id
		})
		require.NoError(t, err)
		assert.Zero(t, len(eleList))
	}

	expectedFindList = []element{
		{3, "9"}, {3, "7"}, {3, "1"},
	}
	eleList, err = xsl.LoadAll(3)
	require.NoError(t, err)
	assert.NotZero(t, len(eleList))
	for i, e := range eleList {
		assert.Equal(t, expectedFindList[i].w, e.Key())
		assert.Equal(t, expectedFindList[i].id, e.Val().id)
	}

}

//
//func TestNewXSkipList_PopHead(t *testing.T) {
//	type element struct {
//		w  int
//		id string
//	}
//	orders := []element{
//		{1, "3"}, {1, "2"}, {1, "1"},
//		{2, "4"}, {2, "2"},
//		{3, "9"}, {3, "8"}, {3, "7"}, {3, "1"},
//		{4, "9"}, {4, "6"}, {4, "3"},
//		{5, "7"}, {5, "6"}, {5, "2"},
//		{6, "8"}, {6, "100"},
//		{7, "8"}, {7, "7"}, {7, "2"}, {7, "1"},
//	}
//	xsl := NewXSkipList[int, *xSkipListObject](func(i, j int) int {
//		return i - j
//	}, randomLevelV3)
//	for _, o := range orders {
//		xsl.Insert(o.w, &xSkipListObject{id: o.id})
//	}
//	for i := 0; i < nodeLen(orders); i++ {
//		e := xsl.PopHead()
//		assert.Equal(t, orders[i].w, e.Key())
//		assert.Equal(t, orders[i].id, e.Val().id)
//		restOrders := orders[i+1:]
//		internalIndex := 0
//		xsl.ForEach(func(idx int64, key int, val *xSkipListObject) {
//			assert.Equal(t, restOrders[internalIndex].w, key)
//			assert.Equal(t, restOrders[internalIndex].id, val.id)
//			internalIndex++
//		})
//	}
//}
