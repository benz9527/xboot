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

func TestXSkipList_Insert(t *testing.T) {
	xsl := NewXSkipList[int, *xSkipListObject](func(i, j int) int {
		return i - j
	})
	xsl.Insert(1, &xSkipListObject{id: "2"})
	xsl.Insert(1, &xSkipListObject{id: "1"})
	xsl.Insert(1, &xSkipListObject{id: "3"})
	xsl.Insert(1, &xSkipListObject{id: "2"})
	xsl.ForEach(func(idx int64, weight int, object *xSkipListObject) {
		t.Logf("idx: %d, weight: %d, obj id: %s\n", idx, weight, object.id)
	})
	t.Logf("xsl levels: %d\n", xsl.Level())
}

func BenchmarkRandomLevelV2(b *testing.B) {
	for i := 0; i < b.N; i++ {
		randomLevelV2(xSkipListMaxLevel, int32(i))
	}
	b.ReportAllocs()
}
