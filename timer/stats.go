package timer

import (
	"context"
	"fmt"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/samber/lo"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const (
	TimingWheelStatsName = "xboot/xtw"
)

type timingWheelStats struct {
	minTickMs             int64
	jobExecutedCounter    atomic.Int64
	jobHighLatencyCounter atomic.Int64
	jobCount              metric.Int64UpDownCounter
	jobTickAccuracy       metric.Float64ObservableGauge
	jobLatencies          metric.Int64Histogram
	jobExecuteDurations   metric.Int64Histogram
	jobExecutedCount      metric.Int64Counter
	jobCancelledCount     metric.Int64Counter
}

func (stats *timingWheelStats) RecordJobCount(count int64) {
	if stats == nil {
		return
	}
	stats.jobCount.Add(context.Background(), count)
}

func (stats *timingWheelStats) IncreaseJobExecutedCount() {
	if stats == nil {
		return
	}
	stats.jobExecutedCount.Add(context.Background(), 1)
	stats.jobExecutedCounter.Add(1)
}

func (stats *timingWheelStats) IncreaseJobCancelledCount() {
	if stats == nil {
		return
	}
	stats.jobCancelledCount.Add(context.Background(), 1)
}

func (stats *timingWheelStats) RecordJobLatency(latencyMs int64) {
	if stats == nil {
		return
	}
	as := attribute.NewSet(
		attribute.String("xtw.job.latency.ms", strconv.FormatInt(latencyMs, 10)),
	)
	stats.jobLatencies.Record(context.Background(), 1, metric.WithAttributeSet(as))
	if latencyMs > stats.minTickMs || latencyMs < -stats.minTickMs {
		stats.jobHighLatencyCounter.Add(1)
	}
}

func (stats *timingWheelStats) RecordJobExecuteDuration(durationMs int64) {
	if stats == nil {
		return
	}
	as := attribute.NewSet(
		attribute.String("xtw.job.execute.duration.ms", strconv.FormatInt(durationMs, 10)),
	)
	stats.jobExecuteDurations.Record(context.Background(), durationMs, metric.WithAttributeSet(as))
}

func WithTimingWheelStats() TimingWheelsOption {
	return func(xtw *xTimingWheels) {
		xtw.isStatsEnabled = true
	}
}

func newTimingWheelStats(ref *xTimingWheels) *timingWheelStats {
	meterName := fmt.Sprintf("%s/%s", TimingWheelStatsName, ref.name)
	tickMs := ref.GetTickMs()
	stats := &timingWheelStats{
		minTickMs: tickMs,
		jobCount: lo.Must[metric.Int64UpDownCounter](otel.Meter(meterName).
			Int64UpDownCounter(
				"xtw.job.count",
				metric.WithDescription("The number of jobs in the timing wheel."),
			),
		),
		jobLatencies: lo.Must[metric.Int64Histogram](otel.Meter(meterName).
			Int64Histogram(
				"xtw.job.latency",
				metric.WithDescription("The latency of the timing wheel job. In milliseconds."),
				metric.WithUnit("ms"),
			),
		),
		jobExecuteDurations: lo.Must[metric.Int64Histogram](otel.Meter(meterName).
			Int64Histogram(
				"xtw.job.execute.duration",
				metric.WithDescription("The duration of the timing wheel job execution. In milliseconds."),
				metric.WithUnit("ms"),
			),
		),
		jobExecutedCount: lo.Must[metric.Int64Counter](otel.Meter(meterName).
			Int64Counter(
				"xtw.job.executed.count",
				metric.WithDescription("The number of jobs executed by the timing wheel."),
			),
		),
		jobCancelledCount: lo.Must[metric.Int64Counter](otel.Meter(meterName).
			Int64Counter(
				"xtw.job.cancelled.count",
				metric.WithDescription("The number of jobs cancelled by the timing wheel."),
			),
		),
	}
	stats.jobTickAccuracy = lo.Must[metric.Float64ObservableGauge](otel.Meter(meterName).
		Float64ObservableGauge(
			"xtw.job.tick.accuracy",
			metric.WithDescription(fmt.Sprintf("The accuracy of the timing wheel tick [-%d,%d] ms.", tickMs, tickMs)),
			metric.WithFloat64Callback(func(ctx context.Context, ob metric.Float64Observer) error {
				accuracy := 1.00
				if stats.jobExecutedCounter.Load() > 0 {
					accuracy = float64(stats.jobExecutedCounter.Load()-stats.jobHighLatencyCounter.Load()) /
						float64(stats.jobExecutedCounter.Load())
				}
				ob.Observe(accuracy)
				return nil
			}),
			metric.WithUnit("%"),
		),
	)
	return stats
}

func jobStatsWrapper(stats *timingWheelStats, invoke Job) Job {
	if stats == nil {
		return invoke
	}
	return func(ctx context.Context, metadata JobMetadata) {
		beginTs := time.Now().UTC()
		defer func() {
			stats.IncreaseJobExecutedCount()
			stats.RecordJobExecuteDuration(time.Since(beginTs).Milliseconds())
		}()
		stats.RecordJobLatency(beginTs.UnixMilli() - metadata.GetExpiredMs())
		invoke(ctx, metadata)
	}
}
