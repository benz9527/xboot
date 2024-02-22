//go:build windows
// +build windows

package timer

import (
	"context"
	"time"
)

func jobStatsWrapper(stats *timingWheelStats, invoke Job) Job {
	if stats == nil {
		return invoke
	}
	return func(ctx context.Context, metadata JobMetadata) {
		var beginTime time.Time
		beginTime = stats.clock.NowInDefaultTZ()

		defer func() {
			stats.IncreaseJobExecutedCount()
			stats.RecordJobExecuteDuration(stats.clock.Since(beginTime).Milliseconds())
		}()
		stats.RecordJobLatency(beginTime.UnixMilli() - metadata.GetExpiredMs())
		invoke(ctx, metadata)
	}
}
