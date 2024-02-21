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
)

func DefaultTimezoneOffset() int {
	return int(atomic.LoadInt32(&defaultTimezoneOffset))
}

func SetDefaultTimezoneOffset(tz TimeZoneOffset) {
	atomic.StoreInt32(&defaultTimezoneOffset, int32(tz))
}
