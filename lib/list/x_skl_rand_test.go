package list

import (
	"math"
	"math/bits"
	"math/rand"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

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

func TestConcRandomLevelV3(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			loop := 1000
			for j := 0; j < loop; j++ {
				t.Logf("conc randv3 id: %d; rand: %d\n", id, concRandomLevelV3(xSkipListMaxLevel, int32(j)))
			}
			wg.Done()
		}(i)
	}
	wg.Wait()
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
