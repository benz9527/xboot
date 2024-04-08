//go:build !windows
// +build !windows

package hrtime

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"golang.org/x/sys/unix"
)

func TestUnixTimerResolution(t *testing.T) {
	res := unix.Timespec{}
	_ = unix.ClockGetres(unix.CLOCK_MONOTONIC, &res)
	t.Logf("Monotonic clock resolution is %d nanoseconds\n", res.Nsec)
}

func TestNow(t *testing.T) {
	defaultTZOffset := DefaultTimezoneOffset()
	t1 := NowInDefaultTZ()
	t2 := NowIn(TzAsiaShanghaiOffset)
	assert.Equal(t, 0, int(t2.Sub(t1).Minutes()))
	_, tz1 := t1.Zone()
	_, tz2 := t2.Zone()
	assert.Equal(t, int(TzAsiaShanghaiOffset)-defaultTZOffset, tz2-tz1)
}

func TestMonotonicClock(t *testing.T) {
	startTs := time.Now().UnixNano()

	time.Sleep(1 * time.Second)

	elapsedMs := (time.Now().UnixNano() - startTs) / int64(time.Millisecond)
	t.Logf("elapsed ms: %v\n", elapsedMs)
}

func TestMonotonicClockByUnix(t *testing.T) {
	ts := unix.Timespec{}
	err := unix.ClockGettime(unix.CLOCK_MONOTONIC, &ts)
	assert.NoError(t, err)
	startTs := ts.Nano()

	time.Sleep(1 * time.Second)

	err = unix.ClockGettime(unix.CLOCK_MONOTONIC, &ts)
	assert.NoError(t, err)
	currentTs := ts.Nano()
	elapsedMs := (currentTs - startTs) / int64(time.Millisecond)
	t.Logf("elapsed ms: %v\n", elapsedMs)
}

func TestNonSysClockTime(t *testing.T) {
	ClockInit()
	t1 := NowInDefaultTZ()
	t.Logf("system clock current time: %v\n", t1)
	time.Sleep(200 * time.Millisecond)
	t2, t3 := GoMonotonicClock.NowInDefaultTZ(), UnixMonotonicClock.NowInDefaultTZ()
	t.Logf("go native non-sys clock current time: %v\n", t2)
	t.Logf("unix non-sys clock current time: %v\n", t3)
	assert.True(t, t2.UnixMilli()-t1.UnixMilli()-5 <= int64(200) && t2.UnixMilli()-t1.UnixMilli()+5 >= int64(200))
	assert.True(t, t3.UnixMilli()-t1.UnixMilli()-5 <= int64(200) && t3.UnixMilli()-t1.UnixMilli()+5 >= int64(200))
	time.Sleep(500 * time.Millisecond)
	t.Logf("go native sys clock current UTC time: %v; asia/shanghai time: %v\n",
		NowInUTC(), NowIn(TzAsiaShanghaiOffset))
	t.Logf("go native non-sys clock current UTC time: %v; asia/shanghai time: %v\n",
		GoMonotonicClock.NowInUTC(), GoMonotonicClock.NowIn(TzAsiaShanghaiOffset))
	t.Logf("unix non-sys clock current UTC time: %v; asia/shanghai time: %v\n",
		UnixMonotonicClock.NowInUTC(), UnixMonotonicClock.NowIn(TzAsiaShanghaiOffset))
	elapsedMs := int64(720)
	assert.GreaterOrEqual(t, elapsedMs, MonotonicElapsed().Milliseconds())
	assert.GreaterOrEqual(t, elapsedMs, GoMonotonicClock.MonotonicElapsed().Milliseconds())
	assert.GreaterOrEqual(t, elapsedMs, UnixMonotonicClock.MonotonicElapsed().Milliseconds())
}
