package kv

import (
	"math/bits"
	"math/rand"
	randv2 "math/rand/v2"
	"strconv"
	"testing"

	"github.com/benz9527/xboot/lib/infra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fast16WayHashMatchInNonAMD64(md *[16]int8, hash int8) uint16 {
	res := uint16(0)
	for i := 0; i < 16; i++ {
		if md[i] == hash {
			res |= 1 << uint(i)
		}
	}
	return res
}

func TestFast16WayHashMatchInNonAMD64(t *testing.T) {
	md := new(swissMapMetadata)
	hash := int8(0x51)
	for i := 0; i < 16; i++ {
		md[i] = empty
	}
	md[2] = hash
	md[9] = hash
	require.Equal(t, uint16(0x0204), fast16WayHashMatchInNonAMD64((*[slotSize]int8)(md), hash))
	require.Equal(t, uint16(0xFDFB), fast16WayHashMatchInNonAMD64((*[slotSize]int8)(md), empty))
}

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

func genStrKeys(strLen, count int) (keys []string) {
	src := rand.New(rand.NewSource(int64(strLen * count)))
	letters := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	l := len(letters)
	r := make([]rune, strLen*count)
	for i := range r {
		r[i] = letters[src.Intn(l)]
	}
	keys = make([]string, count)
	for i := range keys {
		keys[i] = string(r[:strLen])
		r = r[strLen:]
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
	m := newSwissMap[K, int](uint32(len(keys)))
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
	m := newSwissMap[K, K](uint32(len(keys)))
	assert.Equal(t, int64(0), m.Len())
	for _, key := range keys {
		m.Put(key, key)
	}
	assert.Equal(t, int64(len(keys)), m.Len())
	for _, key := range keys {
		val, err := m.Delete(key)
		require.NoError(t, err)
		require.Equal(t, key, val)
		_, ok := m.Get(key)
		assert.False(t, ok)
	}
	assert.Equal(t, int64(0), m.Len())
	// put keys back after deleting them
	for _, key := range keys {
		m.Put(key, key)
	}
	assert.Equal(t, int64(len(keys)), m.Len())
}

func testSwissMapClearRunCore[K infra.OrderedKey](t *testing.T, keys []K) {
	m := newSwissMap[K, int](0)
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
	m := newSwissMap[K, int](uint32(len(keys)))
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

func testSwissMapMigrateFromRunCore[K infra.OrderedKey](t *testing.T, keys []K) {
	m := newSwissMap[K, int](uint32(len(keys)))
	_m := make(map[K]int, len(keys))
	for i, key := range keys {
		_m[key] = i
	}

	err := m.MigrateFrom(_m)
	require.NoError(t, err)
	m.Foreach(func(i uint64, k K, v int) bool {
		val, ok := _m[k]
		require.True(t, ok)
		require.Equal(t, val, v)
		return true
	})
}

func testSwissMapRehashRunCore[K infra.OrderedKey](t *testing.T, keys []K) {
	n := uint32(len(keys))
	m := newSwissMap[K, int](n / 10)
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
		1 * maxAvgSlotLoad,
		2 * maxAvgSlotLoad,
		3 * maxAvgSlotLoad,
		4 * maxAvgSlotLoad,
		5 * maxAvgSlotLoad,
		10 * maxAvgSlotLoad,
		25 * maxAvgSlotLoad,
		50 * maxAvgSlotLoad,
		100 * maxAvgSlotLoad,
	}
	for _, c := range caps {
		m := newSwissMap[K, K](c)
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
	t.Run("put-get", func(tt *testing.T) {
		testSwissMapPutRunCore(tt, keys)
	})
	t.Run("put-delete-get-put", func(tt *testing.T) {
		testSwissMapDeleteRunCore(tt, keys)
	})
	t.Run("clear-foreach", func(tt *testing.T) {
		testSwissMapClearRunCore(tt, keys)
	})
	t.Run("put-foreach", func(tt *testing.T) {
		testSwissMapForeachRunCore(tt, keys)
	})
	t.Run("rehash", func(tt *testing.T) {
		testSwissMapRehashRunCore(tt, keys)
	})
	t.Run("migrate from go native map", func(tt *testing.T) {
		testSwissMapMigrateFromRunCore(tt, keys)
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

func fuzzStringSwissMap(t *testing.T, strKeyLen, count int, initMapCap uint32) {
	const limit = 1024 * 1024
	if count > limit || initMapCap > limit {
		t.Skip()
	}
	m := newSwissMap[string, int](initMapCap)
	if count == 0 {
		return
	}

	keys := genStrKeys(strKeyLen, count)
	standard := make(map[string]int, initMapCap)
	for i, k := range keys {
		m.Put(k, i)
		standard[k] = i
	}
	assert.Equal(t, int64(len(standard)), m.Len())

	for k, exp := range standard {
		act, ok := m.Get(k)
		assert.True(t, ok)
		assert.Equal(t, exp, act)
	}
	for _, k := range keys {
		_, ok := standard[k]
		assert.True(t, ok)
		_, exists := m.Get(k)
		assert.True(t, exists)
	}

	deletes := keys[:count/2]
	for _, k := range deletes {
		delete(standard, k)
		m.Delete(k)
	}
	assert.Equal(t, int64(len(standard)), m.Len())

	for _, k := range deletes {
		_, exists := m.Get(k)
		assert.False(t, exists)
	}
	for k, exp := range standard {
		act, ok := m.Get(k)
		assert.True(t, ok)
		assert.Equal(t, exp, act)
	}
}

func FuzzStringSwissMap(f *testing.F) {
	f.Add(1, 50, uint32(14))
	f.Add(2, 1, uint32(1))
	f.Add(2, 14, uint32(14))
	f.Add(2, 15, uint32(14))
	f.Add(2, 100, uint32(25))
	f.Add(2, 1000, uint32(25))
	f.Add(8, 1, uint32(0))
	f.Add(8, 1, uint32(1))
	f.Add(8, 14, uint32(14))
	f.Add(8, 15, uint32(14))
	f.Add(8, 100, uint32(25))
	f.Add(8, 1000, uint32(25))
	f.Add(16, 100_000, uint32(10_000))
	f.Fuzz(func(t *testing.T, strKeyLen, count int, initMapCap uint32) {
		fuzzStringSwissMap(t, strKeyLen, count, initMapCap)
	})
}

func TestMemFootprint(t *testing.T) {
	var samples []float64
	for n := 10; n <= 10_000; n += 10 {
		b1 := testing.Benchmark(func(b *testing.B) {
			// max load factor 7/8 => 14/16
			m := newSwissMap[int, int](uint32(n))
			require.NotNil(b, m)
		})
		b2 := testing.Benchmark(func(b *testing.B) {
			// max load factor 6.5/8
			m := make(map[int]int, n)
			require.NotNil(b, m)
		})
		x := float64(b1.MemBytes) / float64(b2.MemBytes)
		samples = append(samples, x)
	}
	t.Logf("mean size ratio: %.3f", func() float64 {
		var sum float64
		for _, x := range samples {
			sum += x
		}
		return sum / float64(len(samples))
	}())
}

func BenchmarkStringSwissMaps(b *testing.B) {
	const strKeyLen = 8
	sizes := []int{16, 128, 1024, 8192, 131072}
	for _, n := range sizes {
		b.Run("n="+strconv.Itoa(n), func(bb *testing.B) {
			keys := genStrKeys(strKeyLen, n)
			n := uint32(len(keys))
			mod := n - 1 // power of 2 fast modulus
			require.Equal(bb, 1, bits.OnesCount32(n))
			m := newSwissMap[string, string](n)
			for _, k := range keys {
				m.Put(k, k)
			}
			bb.ResetTimer()
			var ok bool
			for i := 0; i < b.N; i++ {
				_, ok = m.Get(keys[uint32(i)&mod])
			}
			assert.True(b, ok)
			bb.ReportAllocs()
		})
	}
}
