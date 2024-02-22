package hrtime

import "time"

type Clock interface {
	NowIn(offset TimeZoneOffset) time.Time
	NowInDefaultTZ() time.Time
	NowInUTC() time.Time
	MonotonicElapsed() time.Duration
	Since(time.Time) time.Duration
}
