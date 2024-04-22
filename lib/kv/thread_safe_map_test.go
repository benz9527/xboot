package kv

import (
	"fmt"
	"math/bits"
	randv2 "math/rand/v2"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestThreadSafeMap_SimpleCRUD(t *testing.T) {
	keys := genStrKeys(8, 10000)
	vals := make([]int, 0, len(keys))
	m := make(map[string]int, len(keys))
	_m := NewThreadSafeMap[string, int](WithThreadSafeMapInitCap[string, int](10_000),
		WithThreadSafeMapCloseableItemCheck[string, int](),
	)
	for i, key := range keys {
		m[key] = i
		vals = append(vals, i)
	}
	_m.Replace(m)

	_keys := _m.ListKeys()
	require.Equal(t, len(keys), len(_keys))
	require.ElementsMatch(t, keys, _keys)

	_vals := _m.ListValues()
	require.ElementsMatch(t, vals, _vals)

	i := 1001
	res, exists := _m.Get(keys[i])
	require.True(t, exists)
	require.Equal(t, i, res)

	res, err := _m.Delete(keys[i])
	require.NoError(t, err)
	require.Equal(t, i, res)

	err = _m.AddOrUpdate(keys[i], i)
	require.NoError(t, err)

	_keys = _m.ListKeys()
	require.Equal(t, len(keys), len(_keys))
	require.ElementsMatch(t, keys, _keys)

	_vals = _m.ListValues()
	require.ElementsMatch(t, vals, _vals)

	err = _m.Purge()
	require.NoError(t, err)
}

func BenchmarkStringThreadSafeMap(b *testing.B) {
	const strKeyLen = 8
	sizes := []int{16, 128, 1024, 8192, 131072, 1 << 20}
	for _, n := range sizes {
		b.Run("ThreadSafeMap n="+strconv.Itoa(n), func(bb *testing.B) {
			keys := genStrKeys(strKeyLen, n)
			n := uint32(len(keys))
			mod := n - 1 // power of 2 fast modulus
			require.Equal(bb, 1, bits.OnesCount32(n))
			m := NewThreadSafeMap[string, string](WithThreadSafeMapInitCap[string, string](n))
			bb.ResetTimer()
			for _, k := range keys {
				_ = m.AddOrUpdate(k, k)
			}
			var ok bool
			for i := 0; i < b.N; i++ {
				_, ok = m.Get(keys[uint32(i)&mod])
				require.True(b, ok)
			}
			bb.ReportAllocs()
		})
		b.Run("SyncMap n="+strconv.Itoa(n), func(bb *testing.B) {
			keys := genStrKeys(strKeyLen, n)
			n := uint32(len(keys))
			mod := n - 1 // power of 2 fast modulus
			require.Equal(bb, 1, bits.OnesCount32(n))
			m := sync.Map{}
			bb.ResetTimer()
			for _, k := range keys {
				m.Store(k, k)
			}
			var ok bool
			for i := 0; i < b.N; i++ {
				_, ok = m.Load(keys[uint32(i)&mod])
				require.True(b, ok)
			}
			bb.ReportAllocs()
		})
	}
}

func BenchmarkThreadSafeMapReadWrite(b *testing.B) {
	value := []byte(`abc`)
	for i := 0; i <= 10; i++ {
		b.Run(fmt.Sprintf("ThreadSafeMap frac_%d", i), func(bb *testing.B) {
			readFrac := float32(i) / 10.0
			tsm := NewThreadSafeMap[int, []byte](
				WithThreadSafeMapInitCap[int, []byte](4096),
			)
			bb.ResetTimer()
			count := atomic.Int32{}
			bb.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					if randv2.Float32() < readFrac {
						v, exists := tsm.Get(randv2.Int())
						if exists && v != nil {
							count.Add(1)
						}
					} else {
						err := tsm.AddOrUpdate(randv2.Int(), value)
						require.NoError(bb, err)
					}
				}
			})
		})
		b.Run(fmt.Sprintf("SyncMap frac_%d", i), func(bb *testing.B) {
			readFrac := float32(i) / 10.0
			tsm := sync.Map{}
			bb.ResetTimer()
			count := atomic.Int32{}
			bb.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					if randv2.Float32() < readFrac {
						v, exists := tsm.Load(randv2.Int())
						if exists && v != nil {
							count.Add(1)
						}
					} else {
						tsm.Store(randv2.Int(), value)
					}
				}
			})
		})
	}
}

func BenchmarkStringThreadSafeMap_DataRace(b *testing.B) {
	const strKeyLen = 8
	sizes := []int{16, 128, 512, 1024, 8192, 131072, 1 << 19}
	for _, n := range sizes {
		b.Run("ThreadSafeMap n="+strconv.Itoa(n), func(bb *testing.B) {
			keys := genStrKeys(strKeyLen, n)
			n := uint32(len(keys))
			mod := n - 1 // power of 2 fast modulus
			require.Equal(bb, 1, bits.OnesCount32(n))
			m := NewThreadSafeMap[string, string](WithThreadSafeMapInitCap[string, string](n))
			channels := make([]chan string, 4)
			for i := 0; i < len(channels); i++ {
				channels[i] = make(chan string)
			}
			wg := sync.WaitGroup{}
			wg.Add(4)
			bb.ResetTimer()
			go func() {
				for k := range channels[0] {
					_ = m.AddOrUpdate(k, k)
				}
				wg.Done()
			}()
			go func() {
				for k := range channels[1] {
					_ = m.AddOrUpdate(k, k)
				}
				wg.Done()
			}()
			go func() {
				for k := range channels[2] {
					_ = m.AddOrUpdate(k, k)
				}
				wg.Done()
			}()
			go func() {
				for k := range channels[3] {
					_ = m.AddOrUpdate(k, k)
				}
				wg.Done()
			}()
			for i, k := range keys {
				channels[i%4] <- k
			}
			for i := 0; i < 4; i++ {
				close(channels[i])
			}
			wg.Wait()
			var ok bool
			for i := 0; i < b.N; i++ {
				_, ok = m.Get(keys[uint32(i)&mod])
				require.True(b, ok)
			}
			bb.ReportAllocs()
		})
		b.Run("syncMap n="+strconv.Itoa(n), func(bb *testing.B) {
			keys := genStrKeys(strKeyLen, n)
			n := uint32(len(keys))
			mod := n - 1 // power of 2 fast modulus
			require.Equal(bb, 1, bits.OnesCount32(n))
			m := sync.Map{}
			channels := make([]chan string, 4)
			for i := 0; i < len(channels); i++ {
				channels[i] = make(chan string)
			}
			wg := sync.WaitGroup{}
			wg.Add(4)
			bb.ResetTimer()
			go func() {
				for k := range channels[0] {
					m.Store(k, k)
				}
				wg.Done()
			}()
			go func() {
				for k := range channels[1] {
					m.Store(k, k)
				}
				wg.Done()
			}()
			go func() {
				for k := range channels[2] {
					m.Store(k, k)
				}
				wg.Done()
			}()
			go func() {
				for k := range channels[3] {
					m.Store(k, k)
				}
				wg.Done()
			}()
			for i, k := range keys {
				channels[i%4] <- k
			}
			for i := 0; i < 4; i++ {
				close(channels[i])
			}
			wg.Wait()
			var ok bool
			for i := 0; i < b.N; i++ {
				_, ok = m.Load(keys[uint32(i)&mod])
				require.True(b, ok)
			}
			bb.ReportAllocs()
		})
	}
}
