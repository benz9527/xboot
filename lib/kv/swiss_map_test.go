package kv

import (
	"math/bits"
	"math/rand"
	randv2 "math/rand/v2"
	"testing"

	"github.com/benz9527/xboot/lib/infra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTrailingZeros16(t *testing.T) {
	bitset := uint16(0x0001)
	for i := 0; i < 16; i++ {
		tmp := bitset << i
		require.Equal(t, i, bits.TrailingZeros16(tmp))
	}
}

func TestNextIndexInSlot(t *testing.T) {
	bs := bitset(0x03)
	nextIndexInSlot(&bs)
	require.Equal(t, uint16(2), uint16(bs))

	bs = bitset(0x8F)
	nextIndexInSlot(&bs)
	require.Equal(t, uint16(0x8E), uint16(bs))

	bs = bitset(0x40)
	nextIndexInSlot(&bs)
	require.Equal(t, uint16(0), uint16(bs))
}

func TestModN(t *testing.T) {
	x := uint32(100)
	n := uint32(50)
	tmp := uint64(x) * uint64(n)
	require.NotEqual(t, uint32(n-1)&x, uint32(tmp>>32))
}

func genStrKeys(size, count int) (keys []string) {
	src := rand.New(rand.NewSource(int64(size * count)))
	letters := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	l := len(letters)
	r := make([]rune, size*count)
	for i := range r {
		r[i] = letters[src.Intn(l)]
	}
	keys = make([]string, count)
	for i := range keys {
		keys[i] = string(r[:size])
		r = r[size:]
	}
	return
}

func genUint64Keys(count int) (keys []uint64) {
	keys = make([]uint64, count)
	var x uint64
	for i := range keys {
		x += (randv2.Uint64() % 128) + 1
		keys[i] = x
	}
	return
}

func genFloat64Keys(count int) (keys []float64) {
	keys = make([]float64, count)
	var x float64
	for i := range keys {
		x += randv2.Float64() + 101.25
		keys[i] = x
	}
	return
}

func uniqueKeys[K infra.OrderedKey](keys []K) []K {
	s := make(map[K]struct{}, len(keys))
	for _, k := range keys {
		s[k] = struct{}{}
	}
	u := make([]K, 0, len(keys))
	for k := range s {
		u = append(u, k)
	}
	return u
}

func testSwissMapPutRunCore[K infra.OrderedKey](t *testing.T, keys []K) {
	m := NewSwissMap[K, int](uint32(len(keys)))
	assert.Equal(t, int64(0), m.Len())
	for i, key := range keys {
		m.Put(key, i)
	}
	assert.Equal(t, int64(len(keys)), m.Len())
	// overwrite
	for i, key := range keys {
		m.Put(key, -i)
	}
	assert.Equal(t, int64(len(keys)), m.Len())
	for i, key := range keys {
		act, ok := m.Get(key)
		assert.True(t, ok)
		assert.Equal(t, -i, act)
	}
	assert.Equal(t, int64(len(keys)), int64(m.resident))
}

func testSwissMapDeleteRunCore[K infra.OrderedKey](t *testing.T, keys []K) {
	m := NewSwissMap[K, int](uint32(len(keys)))
	assert.Equal(t, int64(0), m.Len())
	for i, key := range keys {
		m.Put(key, i)
	}
	assert.Equal(t, int64(len(keys)), m.Len())
	for _, key := range keys {
		m.Delete(key)
		_, ok := m.Get(key)
		assert.False(t, ok)
	}
	assert.Equal(t, int64(0), m.Len())
	// put keys back after deleting them
	for i, key := range keys {
		m.Put(key, i)
	}
	assert.Equal(t, int64(len(keys)), m.Len())
}

func testSwissMapClearRunCore[K infra.OrderedKey](t *testing.T, keys []K) {
	m := NewSwissMap[K, int](0)
	assert.Equal(t, int64(0), m.Len())
	for i, key := range keys {
		m.Put(key, i)
	}
	assert.Equal(t, int64(len(keys)), m.Len())
	m.Clear()
	assert.Equal(t, int64(0), m.Len())
	for _, key := range keys {
		_, ok := m.Get(key)
		assert.False(t, ok)
	}
	var calls int
	m.Foreach(func(i uint64, key K, val int) bool {
		calls++
		return true // continue
	})
	assert.Equal(t, 0, calls)

	var k K
	for _, g := range m.slots {
		for i := range g.keys {
			assert.Equal(t, k, g.keys[i])
			assert.Equal(t, 0, g.vals[i])
		}
	}
}

func testSwissMapForeachRunCore[K infra.OrderedKey](t *testing.T, keys []K) {
	m := NewSwissMap[K, int](uint32(len(keys)))
	for i, key := range keys {
		m.Put(key, i)
	}
	visited := make(map[K]uint, len(keys))
	m.Foreach(func(i uint64, k K, v int) bool {
		visited[k] = 0
		return true
	})
	if len(keys) == 0 {
		assert.Equal(t, len(visited), 0)
	} else {
		assert.Equal(t, len(visited), len(keys))
	}

	for _, k := range keys {
		visited[k] = 0
	}
	m.Foreach(func(i uint64, k K, v int) bool {
		visited[k]++
		return true
	})
	for _, c := range visited {
		assert.Equal(t, c, uint(1))
	}
	// mutate on iter
	m.Foreach(func(i uint64, k K, v int) bool {
		m.Put(k, -v)
		return true
	})
	for i, key := range keys {
		act, ok := m.Get(key)
		assert.True(t, ok)
		assert.Equal(t, -i, act)
	}
}

func testSwissMapRehashRunCore[K infra.OrderedKey](t *testing.T, keys []K) {
	n := uint32(len(keys))
	m := NewSwissMap[K, int](n / 10)
	for i, key := range keys {
		m.Put(key, i)
	}
	for i, key := range keys {
		act, ok := m.Get(key)
		assert.True(t, ok)
		assert.Equal(t, i, act)
	}
}

func testSwissMapCapacityRunCore[K infra.OrderedKey](t *testing.T, gen func(n int) []K) {
	caps := []uint32{
		1 * maxAvgGroupLoad,
		2 * maxAvgGroupLoad,
		3 * maxAvgGroupLoad,
		4 * maxAvgGroupLoad,
		5 * maxAvgGroupLoad,
		10 * maxAvgGroupLoad,
		25 * maxAvgGroupLoad,
		50 * maxAvgGroupLoad,
		100 * maxAvgGroupLoad,
	}
	for _, c := range caps {
		m := NewSwissMap[K, K](c)
		assert.Equal(t, int64(c), m.Cap())
		keys := gen(rand.Intn(int(c)))
		for _, k := range keys {
			m.Put(k, k)
		}
		assert.Equal(t, int64(int(c)-len(keys)), m.Cap())
		assert.Equal(t, int64(c), m.Len()+m.Cap())
	}
}

func testSwissMapRunCore[K infra.OrderedKey](t *testing.T, keys []K) {
	// sanity check
	require.Equal(t, len(keys), len(uniqueKeys(keys)), keys)
	t.Run("put-get", func(t *testing.T) {
		testSwissMapPutRunCore(t, keys)
	})
	t.Run("put-delete-get-put", func(t *testing.T) {
		testSwissMapDeleteRunCore(t, keys)
	})
	t.Run("clear-foreach", func(t *testing.T) {
		testSwissMapClearRunCore(t, keys)
	})
	t.Run("put-foreach", func(t *testing.T) {
		testSwissMapForeachRunCore(t, keys)
	})
	t.Run("rehash", func(t *testing.T) {
		testSwissMapRehashRunCore(t, keys)
	})
}

func TestSwissMap(t *testing.T) {
	t.Run("stringKeys=0", func(tt *testing.T) {
		testSwissMapRunCore[string](tt, genStrKeys(16, 0))
	})
	t.Run("stringKeys=100", func(tt *testing.T) {
		testSwissMapRunCore[string](tt, genStrKeys(16, 100))
	})
	t.Run("stringKeys=1000", func(tt *testing.T) {
		testSwissMapRunCore[string](tt, genStrKeys(16, 1000))
	})
	t.Run("stringKeys=10_000", func(tt *testing.T) {
		testSwissMapRunCore[string](tt, genStrKeys(16, 10_000))
	})
	t.Run("stringKeys=100_000", func(tt *testing.T) {
		testSwissMapRunCore[string](tt, genStrKeys(16, 100_000))
	})
	t.Run("stringKeys-cap", func(tt *testing.T) {
		testSwissMapCapacityRunCore(tt, func(n int) []string {
			return genStrKeys(16, n)
		})
	})

	t.Run("uint64Keys=0", func(tt *testing.T) {
		testSwissMapRunCore[uint64](tt, genUint64Keys(0))
	})
	t.Run("uint64Keys=100", func(tt *testing.T) {
		testSwissMapRunCore[uint64](tt, genUint64Keys(100))
	})
	t.Run("uint64Keys=1000", func(tt *testing.T) {
		testSwissMapRunCore[uint64](tt, genUint64Keys(1000))
	})
	t.Run("uint64Keys=10_000", func(tt *testing.T) {
		testSwissMapRunCore[uint64](tt, genUint64Keys(10_000))
	})
	t.Run("uint64Keys=100_000", func(tt *testing.T) {
		testSwissMapRunCore[uint64](tt, genUint64Keys(100_000))
	})
	t.Run("uint64Keys-cap", func(tt *testing.T) {
		testSwissMapCapacityRunCore(tt, genUint64Keys)
	})

	t.Run("float64Keys=0", func(tt *testing.T) {
		testSwissMapRunCore[float64](tt, genFloat64Keys(0))
	})
	t.Run("float64Keys=100", func(tt *testing.T) {
		testSwissMapRunCore[float64](tt, genFloat64Keys(100))
	})
	t.Run("float64Keys=1000", func(tt *testing.T) {
		testSwissMapRunCore[float64](tt, genFloat64Keys(1000))
	})
	t.Run("float64Keys=10_000", func(tt *testing.T) {
		testSwissMapRunCore[float64](tt, genFloat64Keys(10_000))
	})
	t.Run("float64Keys=100_000", func(tt *testing.T) {
		testSwissMapRunCore[float64](tt, genFloat64Keys(100_000))
	})
	t.Run("float64Keys-cap", func(tt *testing.T) {
		testSwissMapCapacityRunCore(tt, genFloat64Keys)
	})
}
