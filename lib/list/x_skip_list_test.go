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
	return hash.Sum64()
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

	xsl := NewXSkipList[int, *xSkipListObject](func(i, j int) int {
		return i - j
	})
	xsl.Insert(1, &xSkipListObject{id: "2"})
	xsl.Insert(1, &xSkipListObject{id: "1"})
	xsl.Insert(1, &xSkipListObject{id: "3"})
	xsl.Insert(1, &xSkipListObject{id: "2"})
	expectedOrder := []element{{1, "3"}, {1, "2"}, {1, "1"}}
	xsl.ForEach(func(idx int64, weight int, object *xSkipListObject) {
		assert.Equal(t, expectedOrder[idx].w, weight)
		assert.Equal(t, expectedOrder[idx].id, object.id)
	})
	assert.Equal(t, int32(3), xsl.Len())

	xsl.RemoveFirst(1, func(obj *xSkipListObject) int {
		if obj.id == "2" {
			return 0
		}
		return -1
	})
	xsl.ForEach(func(idx int64, weight int, object *xSkipListObject) {
		t.Logf("idx: %d, weight: %d, obj id: %s\n", idx, weight, object.id)
	})
	t.Logf("xsl levels: %d\n", xsl.Level())

	e := xsl.FindFirst(1, func(obj *xSkipListObject) int {
		if obj.id == "2" {
			return 0
		}
		return -1
	})
	assert.Nil(t, e)
	expectedOrder = []element{{1, "3"}, {1, "1"}}
	xsl.ForEach(func(idx int64, weight int, object *xSkipListObject) {
		assert.Equal(t, expectedOrder[idx].w, weight)
		assert.Equal(t, expectedOrder[idx].id, object.id)
	})
	assert.Equal(t, int32(2), xsl.Len())

	e = xsl.FindFirst(1, func(obj *xSkipListObject) int {
		if obj.id == "1" {
			return 0
		}
		return -1
	})
	assert.NotNil(t, e)
	assert.Equal(t, "1", e.Object().id)

	xsl.Insert(2, &xSkipListObject{id: "2"})
	xsl.Insert(3, &xSkipListObject{id: "1"})
	xsl.Insert(4, &xSkipListObject{id: "3"})
	xsl.Insert(2, &xSkipListObject{id: "4"})
	xsl.Insert(5, &xSkipListObject{id: "1"})
	xsl.Insert(6, &xSkipListObject{id: "8"})
	expectedOrder = []element{
		{1, "3"}, {1, "1"},
		{2, "4"}, {2, "2"},
		{3, "1"},
		{4, "3"},
		{5, "1"},
		{6, "8"},
	}
	xsl.ForEach(func(idx int64, weight int, object *xSkipListObject) {
		assert.Equal(t, expectedOrder[idx].w, weight)
		assert.Equal(t, expectedOrder[idx].id, object.id)
	})
	assert.Equal(t, int32(len(expectedOrder)), xsl.Len())

	e = xsl.RemoveFirst(2, func(obj *xSkipListObject) int {
		if obj.id == "2" {
			return 0
		}
		return -1
	})
	assert.Equal(t, "2", e.Object().id)

	expectedOrder = []element{
		{1, "3"}, {1, "1"},
		{2, "4"},
		{3, "1"},
		{4, "3"},
		{5, "1"},
		{6, "8"},
	}
	xsl.ForEach(func(idx int64, weight int, object *xSkipListObject) {
		assert.Equal(t, expectedOrder[idx].w, weight)
		assert.Equal(t, expectedOrder[idx].id, object.id)
	})
	assert.Equal(t, int32(len(expectedOrder)), xsl.Len())
}

func BenchmarkRandomLevelV2(b *testing.B) {
	for i := 0; i < b.N; i++ {
		randomLevelV2(xSkipListMaxLevel, int32(i))
	}
	b.ReportAllocs()
}
