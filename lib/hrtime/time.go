//go:build !windows
// +build !windows

package hrtime

// Central Standard Time (CST)
// https://www.timeanddate.com/time/zones/cst
// Coordinated Universal Time (UTC)
// Greenwich Mean Time (GMT)

import (
	"sync/atomic"
	"time"

	"github.com/samber/lo"
	"golang.org/x/sys/unix"
)

func NowIn(offset TimeZoneOffset) time.Time {
	zone := time.FixedZone("CST", int(offset))
	return time.Now().In(zone)
}

func NowInDefaultTZ() time.Time {
	return NowIn(TimeZoneOffset(atomic.LoadInt32(&defaultTimezoneOffset)))
}

func NowInUTC() time.Time {
	return NowIn(TzUtc0Offset)
}

func MonotonicElapsed() time.Duration {
	return time.Since(appStartTime)
}

var (
	GoMonotonicClock     NonSysClockTime = &goNonSysClockTime{}
	goMonotonicStartTs   int64
	UnixMonotonicClock   NonSysClockTime = &unixNonSysClockTime{}
	unixMonotonicStartTs int64
)

func init() {
	defaultTimezoneOffset = int32(TzUtc0Offset)
	zone := time.FixedZone("CST", int(defaultTimezoneOffset))
	appStartTime = time.Now().In(zone)

	goMonotonicStartTs = appStartTime.UnixNano()

	ts := unix.Timespec{}
	lo.Must0(unix.ClockGettime(unix.CLOCK_MONOTONIC, &ts))
	unixMonotonicStartTs = ts.Nano()
}

type NonSysClockTime interface {
	NowIn(offset TimeZoneOffset) time.Time
	NowInDefaultTZ() time.Time
	NowInUTC() time.Time
	MonotonicElapsed() time.Duration
}

type goNonSysClockTime struct{}

func (g *goNonSysClockTime) now() time.Time {
	nano := appStartTime.UnixNano() + g.MonotonicElapsed().Nanoseconds()
	return time.UnixMilli(time.Duration(nano).Milliseconds())
}

func (g *goNonSysClockTime) NowIn(offset TimeZoneOffset) time.Time {
	zone := time.FixedZone("CST", int(offset))
	return g.now().In(zone)
}

func (g *goNonSysClockTime) NowInDefaultTZ() time.Time {
	return g.NowIn(TimeZoneOffset(atomic.LoadInt32(&defaultTimezoneOffset)))
}

func (g *goNonSysClockTime) NowInUTC() time.Time {
	return g.NowIn(TzUtc0Offset)
}

func (g *goNonSysClockTime) MonotonicElapsed() time.Duration {
	return time.Duration(time.Now().UnixNano() - goMonotonicStartTs)
}

type unixNonSysClockTime struct{}

func (u *unixNonSysClockTime) now() time.Time {
	nano := appStartTime.UnixNano() + u.MonotonicElapsed().Nanoseconds()
	return time.UnixMilli(time.Duration(nano).Milliseconds())
}

func (u *unixNonSysClockTime) NowIn(offset TimeZoneOffset) time.Time {
	zone := time.FixedZone("CST", int(offset))
	return u.now().In(zone)
}

func (u *unixNonSysClockTime) NowInDefaultTZ() time.Time {
	return u.NowIn(TimeZoneOffset(atomic.LoadInt32(&defaultTimezoneOffset)))
}

func (u *unixNonSysClockTime) NowInUTC() time.Time {
	return u.NowIn(TzUtc0Offset)
}

func (u *unixNonSysClockTime) MonotonicElapsed() time.Duration {
	ts := unix.Timespec{}
	lo.Must0(unix.ClockGettime(unix.CLOCK_MONOTONIC, &ts))
	currentTs := ts.Nano()
	return time.Duration(currentTs - unixMonotonicStartTs)
}
