//go:build windows
// +build windows

package hrtime

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestHighResolutionTime(t *testing.T) {
	t.Run("use default windows tr around 15.6ms", func(tt *testing.T) {
		ClockInit()
		_ = ResetTimeResolutionFrom1ms()
		t1 := time.Now()
		tt.Logf("system clock current time: %v\n", t1)
		time.Sleep(200 * time.Millisecond)
		t2, t3, t4 := NowInDefaultTZ(), NowIn(TzUtc8Offset), time.Now()
		tt.Logf("windows non-sys clock current time in default tz: %v; in asia tz: %v\n", t2, t3)
		tt.Logf("system clock current time: %v\n", t4)
		elapsedMs := MonotonicElapsed().Milliseconds()
		tt.Logf("elapsed ms: %d\n", elapsedMs)
		assert.True(tt, t2.UnixMilli()-t1.UnixMilli()-1 <= elapsedMs && t2.UnixMilli()-t1.UnixMilli()+1 >= elapsedMs)
		assert.True(tt, t4.UnixMilli()-t1.UnixMilli()-1 <= elapsedMs && t4.UnixMilli()-t1.UnixMilli()+1 >= elapsedMs)
		time.Sleep(500 * time.Millisecond)
		elapsedMs = int64(750)
		e1, e2 := time.Since(t1).Milliseconds(), MonotonicElapsed().Milliseconds()
		tt.Logf("e1: %v; e2: %v\n", e1, e2)
		assert.GreaterOrEqual(tt, elapsedMs, e1)
		assert.GreaterOrEqual(tt, elapsedMs, e2)
	})
	t.Run("set windows tr to 1ms by ClockInit", func(tt *testing.T) {
		ClockInit()
		defer func() {
			_ = ResetTimeResolutionFrom1ms()
		}()
		t1 := time.Now()
		tt.Logf("system clock current time: %v\n", t1)
		time.Sleep(200 * time.Millisecond)
		t2, t3, t4 := NowInDefaultTZ(), NowIn(TzUtc8Offset), time.Now()
		tt.Logf("windows non-sys clock current time in default tz: %v; in asia tz: %v\n", t2, t3)
		tt.Logf("system clock current time: %v\n", t4)
		elapsedMs := MonotonicElapsed().Milliseconds()
		tt.Logf("elapsed ms: %d\n", elapsedMs)
		assert.True(tt, t2.UnixMilli()-t1.UnixMilli()-1 <= elapsedMs && t2.UnixMilli()-t1.UnixMilli()+1 >= elapsedMs)
		assert.True(tt, t4.UnixMilli()-t1.UnixMilli()-1 <= elapsedMs && t4.UnixMilli()-t1.UnixMilli()+1 >= elapsedMs)
		time.Sleep(500 * time.Millisecond)
		windowsElapsedMs, defaultElapsed := time.Since(t1).Milliseconds(), MonotonicElapsed().Milliseconds()
		assert.GreaterOrEqual(tt, windowsElapsedMs, defaultElapsed)
	})
}
