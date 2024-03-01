package list

import (
	"hash/fnv"
	"math"
	"math/bits"
	"math/rand"
	"sync"
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

func randomLevelV1(maxLevel int, currentElements int32) int32 {
	// Call function maxLevels to get total?
	// maxLevel => n, 2^n -1, there will be 2^n-1 elements in the skip list
	var total uint64
	if maxLevel == xSkipListMaxLevel {
		total = uint64(math.MaxUint32)
	} else {
		total = uint64(1)<<maxLevel - 1
	}
	// goland math random (math.Float64()) contains global mutex lock
	// Ref
	// https://cs.opensource.google/go/go/+/refs/tags/go1.21.5:src/math/rand/rand.go
	// https://cs.opensource.google/go/go/+/refs/tags/go1.21.5:src/math/bits/bits.go
	// 1. Avoid to use global mutex lock
	// 2. Avoid to generate random number each time
	rest := rand.Uint64() & total
	// Bits right shift equal to manipulate a high-level bit
	// Calculate the minimum bits of the random number
	tmp := bits.Len64(rest) // Lookup table.
	level := maxLevel - tmp + 1
	// Avoid the value of randomly generated level deviates
	//   far from the number of elements within the skip-list.
	// level should be greater than but approximate to log(currentElements)
	for level > 1 && uint64(1)<<(level-1) > uint64(currentElements) {
		level--
	}
	return int32(level)
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

func TestRandomLevel(t *testing.T) {
	loop := 10
	for i := 0; i < loop; i++ {
		t.Log(randomLevel(xSkipListMaxLevel, int32(i)))
	}
}

func TestRandomLevelV1(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			loop := 1000
			for j := 0; j < loop; j++ {
				t.Logf("randv1 id: %d; rand: %d\n", id, randomLevelV1(xSkipListMaxLevel, int32(j)))
			}
			wg.Done()
		}(i)
	}
	wg.Wait()
}

func TestRandomLevelV2(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			loop := 1000
			for j := 0; j < loop; j++ {
				t.Logf("randv2 id: %d; rand: %d\n", id, randomLevelV2(xSkipListMaxLevel, int32(j)))
			}
			wg.Done()
		}(i)
	}
	wg.Wait()
}

func TestRandomLevelV3(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			loop := 1000
			for j := 0; j < loop; j++ {
				t.Logf("randv3 id: %d; rand: %d\n", id, randomLevelV3(xSkipListMaxLevel, int32(j)))
			}
			wg.Done()
		}(i)
	}
	wg.Wait()
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
	}, randomLevelV2)
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

	expectedFindList := []element{
		{7, "7"}, {6, "100"}, {3, "7"},
	}
	for _, e := range expectedFindList {
		eleList = xsl.FindIfMatch(e.w, func(obj *xSkipListObject) bool {
			return obj.id == e.id
		})
		assert.NotNil(t, eleList)
		assert.Equal(t, 1, len(eleList))
		assert.Equal(t, e.w, eleList[0].Weight())
		assert.Equal(t, e.id, eleList[0].Object().id)
	}

	expectedFindList = []element{
		{7, "2"}, {5, "6"}, {3, "8"},
	}
	for _, e := range expectedFindList {
		eleList = xsl.FindIfMatch(e.w, func(obj *xSkipListObject) bool {
			return obj.id == e.id
		})
		assert.Zero(t, len(eleList))
	}

	expectedFindList = []element{
		{3, "9"}, {3, "7"}, {3, "1"},
	}
	eleList = xsl.FindAll(3)
	assert.NotZero(t, len(eleList))
	for i, e := range eleList {
		assert.Equal(t, expectedFindList[i].w, e.Weight())
		assert.Equal(t, expectedFindList[i].id, e.Object().id)
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
	xsl := NewXSkipList[int, *xSkipListObject](func(i, j int) int {
		return i - j
	}, randomLevelV3)
	for _, o := range orders {
		xsl.Insert(o.w, &xSkipListObject{id: o.id})
	}
	for i := 0; i < len(orders); i++ {
		e := xsl.PopHead()
		assert.Equal(t, orders[i].w, e.Weight())
		assert.Equal(t, orders[i].id, e.Object().id)
		restOrders := orders[i+1:]
		internalIndex := 0
		xsl.ForEach(func(idx int64, weight int, object *xSkipListObject) {
			assert.Equal(t, restOrders[internalIndex].w, weight)
			assert.Equal(t, restOrders[internalIndex].id, object.id)
			internalIndex++
		})
	}
}

func BenchmarkRandomLevel(b *testing.B) {
	for i := 0; i < b.N; i++ {
		randomLevel(xSkipListMaxLevel, int32(i))
	}
	b.ReportAllocs()
}

func BenchmarkRandomLevelV2(b *testing.B) {
	for i := 0; i < b.N; i++ {
		randomLevelV2(xSkipListMaxLevel, int32(i))
	}
	b.ReportAllocs()
}
