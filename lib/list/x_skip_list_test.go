package list

import (
	"hash/fnv"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

type xSkipListObject struct {
	id string
}

func (o *xSkipListObject) Hash() uint64 {
	hash := fnv.New64()
	_, _ = hash.Write([]byte(o.id))
	val := hash.Sum64()
	return val
}

func TestStringHash_FNV(t *testing.T) {
	s1, s2 := "1", "2"
	hash := fnv.New64()
	_, _ = hash.Write([]byte(s1))
	assert.Equal(t, uint64(12638153115695167470), hash.Sum64())
	hash = fnv.New64()
	_, _ = hash.Write([]byte(s2))
	assert.Equal(t, uint64(12638153115695167469), hash.Sum64())
}

func TestMaxLevel(t *testing.T) {
	levels := maxLevels(math.MaxInt32, 0.25) // 30
	assert.GreaterOrEqual(t, 32, levels)

	levels = maxLevels(int64(1), 0.25) // 0
	assert.GreaterOrEqual(t, 0, levels)

	levels = maxLevels(int64(2), 0.25) // 1
	assert.GreaterOrEqual(t, 1, levels)
}

func TestRandomLevelV2(t *testing.T) {
	loop := 10
	for i := 0; i < loop; i++ {
		t.Log(randomLevelV2(xSkipListMaxLevel, int32(i)))
	}
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
	xsl := NewXSkipList[int, *xSkipListObject](func(i, j int) int {
		return i - j
	})
	for _, o := range orders {
		xsl.Insert(o.w, &xSkipListObject{id: o.id})
	}

	_, ok := xsl.Insert(1, &xSkipListObject{id: "2"})
	assert.False(t, ok)
	xsl.ForEach(func(idx int64, weight int, object *xSkipListObject) {
		assert.Equal(t, orders[idx].w, weight)
		assert.Equal(t, orders[idx].id, object.id)
	})
	assert.Equal(t, int32(len(orders)), xsl.Len())

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
		ele := xsl.FindFirst(first.w)
		assert.NotNil(t, ele)
		assert.Equal(t, first.w, ele.Weight())
		assert.Equal(t, first.id, ele.Object().id)
	}

	ele := xsl.RemoveFirst(4)
	assert.NotNil(t, ele)
	assert.Equal(t, 4, ele.Weight())
	assert.Equal(t, "9", ele.Object().id)

	eleList := xsl.RemoveAll(4)
	assert.NotNil(t, eleList)
	expectedRemoveList := []element{
		{4, "6"}, {4, "3"},
	}
	assert.Equal(t, len(expectedRemoveList), len(eleList))
	for i, e := range expectedRemoveList {
		assert.Equal(t, e.w, eleList[i].Weight())
		assert.Equal(t, e.id, eleList[i].Object().id)
	}

	orders = []element{
		{1, "3"}, {1, "2"}, {1, "1"},
		{2, "4"}, {2, "2"},
		{3, "9"}, {3, "8"}, {3, "7"}, {3, "1"},
		{5, "7"}, {5, "6"}, {5, "2"},
		{6, "8"}, {6, "100"},
		{7, "8"}, {7, "7"}, {7, "2"}, {7, "1"},
	}
	xsl.ForEach(func(idx int64, weight int, object *xSkipListObject) {
		assert.Equal(t, orders[idx].w, weight)
		assert.Equal(t, orders[idx].id, object.id)
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
		ele := xsl.FindFirst(first.w)
		assert.NotNil(t, ele)
		assert.Equal(t, first.w, ele.Weight())
		assert.Equal(t, first.id, ele.Object().id)
	}

	expectedRemoveList = []element{
		{7, "2"}, {5, "6"}, {3, "8"},
	}
	for _, e := range expectedRemoveList {
		eleList = xsl.RemoveIfMatch(e.w, func(obj *xSkipListObject) bool {
			return obj.id == e.id
		})
		assert.NotNil(t, eleList)
		assert.Equal(t, 1, len(eleList))
		assert.Equal(t, e.w, eleList[0].Weight())
		assert.Equal(t, e.id, eleList[0].Object().id)
	}

	orders = []element{
		{1, "3"}, {1, "2"}, {1, "1"},
		{2, "4"}, {2, "2"},
		{3, "9"}, {3, "7"}, {3, "1"},
		{5, "7"}, {5, "2"},
		{6, "8"}, {6, "100"},
		{7, "8"}, {7, "7"}, {7, "1"},
	}
	xsl.ForEach(func(idx int64, weight int, object *xSkipListObject) {
		assert.Equal(t, orders[idx].w, weight)
		assert.Equal(t, orders[idx].id, object.id)
	})
	assert.Equal(t, int32(len(orders)), xsl.Len())
}

//func TestNewXSkipList_PopHead(t *testing.T) {
//	type element struct {
//		w  int
//		id string
//	}
//	orders := []element{
//		{1, "3"}, {1, "2"}, {1, "1"},
//		{2, "4"}, {2, "2"},
//		{3, "1"},
//		{4, "3"},
//		{5, "1"},
//		{6, "8"}, {6, "100"},
//	}
//	xsl := NewXSkipList[int, *xSkipListObject](func(i, j int) int {
//		return i - j
//	})
//	for _, o := range orders {
//		xsl.Insert(o.w, &xSkipListObject{id: o.id})
//	}
//	for i := 0; i < len(orders); i++ {
//		e := xsl.PopHead()
//		assert.Equal(t, orders[i].w, e.Weight())
//		assert.Equal(t, orders[i].id, e.Object().id)
//		//restOrders := orders[i+1:]
//		//internalIndex := 0
//		t.Logf("loop %d\n", i)
//		xsl.ForEach(func(idx int64, weight int, object *xSkipListObject) {
//			//assert.Equal(t, restOrders[internalIndex].w, weight)
//			//assert.Equal(t, restOrders[internalIndex].id, object.id)
//			//internalIndex++
//			//t.Logf("idx: %d, weight: %d, id: %s\n", idx, weight, object.id)
//		})
//	}
//}

func BenchmarkRandomLevelV2(b *testing.B) {
	for i := 0; i < b.N; i++ {
		randomLevelV2(xSkipListMaxLevel, int32(i))
	}
	b.ReportAllocs()
}
