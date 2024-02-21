//go:build windows
// +build windows

package hrtime

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNonSysClockTime(t *testing.T) {
	SetTimeResolutionTo1ms()
	defer func() {
		_ = ResetTimeResolutionFrom1ms()
	}()
	t1 := time.Now()
	t.Logf("system clock current time: %v\n", t1)
	time.Sleep(200 * time.Millisecond)
	t2, t3, t4 := NowInDefaultTZ(), NowIn(TzUtc8Offset), time.Now()
	t.Logf("windows non-sys clock current time in default tz: %v; in asia tz: %v\n", t2, t3)
	t.Logf("system clock current time: %v\n", t4)
	assert.True(t, t2.UnixMilli()-t1.UnixMilli()-1 <= int64(200) && t2.UnixMilli()-t1.UnixMilli()+1 >= int64(200))
	assert.True(t, t4.UnixMilli()-t1.UnixMilli()-1 <= int64(200) && t4.UnixMilli()-t1.UnixMilli()+1 >= int64(200))
	time.Sleep(500 * time.Millisecond)
	elapsedMs := int64(703)
	assert.GreaterOrEqual(t, elapsedMs, time.Since(t1).Milliseconds())
	assert.GreaterOrEqual(t, elapsedMs, MonotonicElapsed().Milliseconds())
}

func TestNonSysClockTimeWithoutSetResolution(t *testing.T) {
	t1 := time.Now()
	t.Logf("system clock current time: %v\n", t1)
	time.Sleep(200 * time.Millisecond)
	t2, t3, t4 := NowInDefaultTZ(), NowIn(TzUtc8Offset), time.Now()
	t.Logf("windows non-sys clock current time in default tz: %v; in asia tz: %v\n", t2, t3)
	t.Logf("system clock current time: %v\n", t4)
	assert.True(t, t2.UnixMilli()-t1.UnixMilli()-16 <= int64(200) && t2.UnixMilli()-t1.UnixMilli()+16 >= int64(200))
	assert.True(t, t4.UnixMilli()-t1.UnixMilli()-16 <= int64(200) && t4.UnixMilli()-t1.UnixMilli()+16 >= int64(200))
	time.Sleep(500 * time.Millisecond)
	elapsedMs := int64(730)
	e1, e2 := time.Since(t1).Milliseconds(), MonotonicElapsed().Milliseconds()
	t.Logf("e1: %v; e2: %v\n", e1, e2)
	assert.GreaterOrEqual(t, elapsedMs, e1)
	assert.GreaterOrEqual(t, elapsedMs, e2)
}
