package queue

import (
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"
)

func TestXRingBufferCursor(t *testing.T) {
	cursor := NewXRingBufferCursor()
	beginIs := time.Now()
	for i := 0; i < 100000000; i++ {
		x := cursor.Next()
		if x%10000000 == 0 {
			t.Logf("x=%d", x)
		}
	}
	diff := time.Since(beginIs)
	t.Logf("ts diff=%v", diff)
}

func TestXRingBufferCursorConcurrency(t *testing.T) {
	_, debugLogDisabled := os.LookupEnv("DISABLE_TEST_DEBUG_LOG")
	// lower than single goroutine test
	cursor := NewXRingBufferCursor()
	t.Logf("cursor size=%v", unsafe.Sizeof(*cursor.(*rbCursor)))
	beginIs := time.Now()
	wg := sync.WaitGroup{}
	wg.Add(10000)
	for i := 0; i < 10000; i++ {
		go func(idx int) {
			for j := 0; j < 10000; j++ {
				x := cursor.Next()
				if x%10000000 == 0 {
					if !debugLogDisabled {
						t.Logf("gid=%d, x=%d", idx, x)
					}
				}
			}
			wg.Done()
		}(i)
	}
	wg.Wait()
	diff := time.Since(beginIs)
	t.Logf("ts diff=%v", diff)
}

func TestXRingBufferCursorNoPaddingConcurrency(t *testing.T) {
	_, debugLogDisabled := os.LookupEnv("DISABLE_TEST_DEBUG_LOG")
	// Better than padding version
	var cursor uint64 // same address, meaningless for data race
	beginIs := time.Now()
	wg := sync.WaitGroup{}
	wg.Add(10000)
	for i := 0; i < 10000; i++ {
		go func(idx int) {
			for j := 0; j < 10000; j++ {
				x := atomic.AddUint64(&cursor, 1)
				if x%10000000 == 0 {
					if !debugLogDisabled {
						t.Logf("gid=%d, x=%d", idx, x)
					}
				}
			}
			wg.Done()
		}(i)
	}
	wg.Wait()
	diff := time.Since(beginIs)
	t.Logf("ts diff=%v", diff)
}

type noPaddingObj struct {
	a, b, c uint64
}

func (o *noPaddingObj) increase() {
	atomic.AddUint64(&o.a, 1)
	atomic.AddUint64(&o.b, 1)
	atomic.AddUint64(&o.c, 1)
}

type paddingObj struct {
	a uint64
	_ [8]uint64
	b uint64
	_ [8]uint64
	c uint64
	_ [8]uint64
}

func (o *paddingObj) increase() {
	atomic.AddUint64(&o.a, 1)
	atomic.AddUint64(&o.b, 1)
	atomic.AddUint64(&o.c, 1)
}

func BenchmarkNoPaddingObj(b *testing.B) {
	// Lower than padding version
	obj := &noPaddingObj{}
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			obj.increase()
		}
	})
}

func BenchmarkPaddingObj(b *testing.B) {
	obj := &paddingObj{}
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			obj.increase()
		}
	})
}

func TestFalseSharing(t *testing.T) {
	// In the same cache line
	volatileArray := [4]uint64{0, 0, 0, 0} // contiguous memory
	var wg sync.WaitGroup
	wg.Add(4)
	beginIs := time.Now()
	for i := 0; i < 4; i++ {
		// Concurrent write to the same cache line
		// Many cache misses, because of many RFO
		go func(idx int) {
			for j := 0; j < 100000000; j++ {
				atomic.AddUint64(&volatileArray[idx], 1)
			}
			wg.Done()
		}(i)
	}
	wg.Wait()
	diff := time.Since(beginIs)
	// ts diff=8.423525377s
	t.Logf("ts diff=%v", diff)
}

func TestNoFalseSharing(t *testing.T) {
	type padE struct {
		value uint64
		_     [cacheLinePadSize - unsafe.Sizeof(*new(uint64))]byte
	}

	// Each one in a different cache line
	volatileArray := [4]padE{{}, {}, {}, {}}
	var wg sync.WaitGroup
	wg.Add(4)
	beginIs := time.Now()
	for i := 0; i < 4; i++ {
		// No RFO data race
		go func(idx int) {
			for j := 0; j < 100000000; j++ {
				atomic.AddUint64(&volatileArray[idx].value, 1)
			}
			wg.Done()
		}(i)
	}
	wg.Wait()
	diff := time.Since(beginIs)
	// ts diff=890.219393ms
	t.Logf("ts diff=%v", diff)
}
