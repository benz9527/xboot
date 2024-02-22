package hrtime

import (
	"sync/atomic"
	"time"
)

type TimeZoneOffset int32

const (
	hourInMinutes                       = 3600
	TzUtc0Offset         TimeZoneOffset = 0
	TzUtc8Offset         TimeZoneOffset = 8 * hourInMinutes
	TzAsiaShanghaiOffset TimeZoneOffset = TzUtc8Offset
)

var (
	defaultTimezoneOffset int32
	appStartTime          time.Time
	zoneMap               = map[TimeZoneOffset]*time.Location{
		TzUtc0Offset:         time.UTC,
		TzAsiaShanghaiOffset: time.FixedZone("CST", int(TzAsiaShanghaiOffset)),
	}
)

func DefaultTimezoneOffset() int {
	return int(atomic.LoadInt32(&defaultTimezoneOffset))
}

func SetDefaultTimezoneOffset(tz TimeZoneOffset) {
	atomic.StoreInt32(&defaultTimezoneOffset, int32(tz))
}

// Reduce the location object allocation.
func loadTZLocation(offset TimeZoneOffset) *time.Location {
	loc, ok := zoneMap[offset]
	if !ok {
		return time.UTC
	}
	return loc
}

func MillisToTzTime(millis int64, tzOffset TimeZoneOffset) time.Time {
	return time.UnixMilli(millis).In(loadTZLocation(tzOffset))
}

func MillisToDefaultTzTime(millis int64) time.Time {
	return time.UnixMilli(millis).In(loadTZLocation(TimeZoneOffset(DefaultTimezoneOffset())))
}
