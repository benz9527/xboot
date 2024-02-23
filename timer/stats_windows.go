//go:build windows
// +build windows

package timer

import (
	"context"
)

func jobStatsWrapper(stats *timingWheelStats, invoke Job) Job {
	if stats == nil {
		return invoke
	}
	return func(ctx context.Context, metadata JobMetadata) {
		var beginTime = stats.clock.NowInDefaultTZ()
		defer func() {
			stats.IncreaseJobExecutedCount()
			stats.RecordJobExecuteDuration(stats.clock.Since(beginTime).Milliseconds())
		}()
		stats.RecordJobLatency(beginTime.UnixMilli() - metadata.GetExpiredMs())
		invoke(ctx, metadata)
	}
}
