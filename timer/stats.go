package timer

import (
	"context"
	"fmt"
	"strconv"
	"sync/atomic"

	"github.com/samber/lo"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/benz9527/xboot/lib/hrtime"
)

const (
	TimingWheelStatsName = "xboot/xtw"
)

type xTimingWheelsStats struct {
	ctx                 context.Context
	minTickMs           int64
	clock               hrtime.Clock
	jobExecutedCount    atomic.Int64
	jobHighLatencyCount atomic.Int64
	slotActiveCount     atomic.Int64
	jobAliveCounter     metric.Int64UpDownCounter
	jobTickAccuracy     metric.Float64ObservableGauge
	jobLatencies        metric.Int64Histogram
	jobExecuteDurations metric.Int64Histogram
	jobExecutedCounter  metric.Int64Counter
	jobCancelledCounter metric.Int64Counter
	slotCounter         metric.Int64Counter
	slotActiveCounter   metric.Int64ObservableUpDownCounter
}

func (stats *xTimingWheelsStats) RecordJobAliveCount(count int64) {
	if stats == nil {
		return
	}
	stats.jobAliveCounter.Add(stats.ctx, count)
}

func (stats *xTimingWheelsStats) UpdateSlotActiveCount(count int64) {
	if stats == nil {
		return
	}
	stats.slotActiveCount.Swap(count)
}

func (stats *xTimingWheelsStats) RecordSlotCount(count int64) {
	if stats == nil {
		return
	}
	stats.slotCounter.Add(stats.ctx, count)
}

func (stats *xTimingWheelsStats) IncreaseJobExecutedCount() {
	if stats == nil {
		return
	}
	stats.jobExecutedCounter.Add(stats.ctx, 1)
	stats.jobExecutedCount.Add(1)
}

func (stats *xTimingWheelsStats) IncreaseJobCancelledCount() {
	if stats == nil {
		return
	}
	stats.jobCancelledCounter.Add(stats.ctx, 1)
}

func (stats *xTimingWheelsStats) RecordJobLatency(latencyMs int64) {
	if stats == nil {
		return
	}
	as := attribute.NewSet(
		attribute.String("xtw.job.latency.ms", strconv.FormatInt(latencyMs, 10)),
	)
	stats.jobLatencies.Record(stats.ctx, 1, metric.WithAttributeSet(as))
	if latencyMs > stats.minTickMs || latencyMs < -stats.minTickMs {
		stats.jobHighLatencyCount.Add(1)
	}
}

func (stats *xTimingWheelsStats) RecordJobExecuteDuration(durationMs int64) {
	if stats == nil {
		return
	}
	as := attribute.NewSet(
		attribute.String("xtw.job.execute.duration.ms", strconv.FormatInt(durationMs, 10)),
	)
	stats.jobExecuteDurations.Record(stats.ctx, durationMs, metric.WithAttributeSet(as))
}

func newTimingWheelStats(ref *xTimingWheelsOption) *xTimingWheelsStats {
	meterName := fmt.Sprintf("%s/%s", TimingWheelStatsName, ref.getName())
	tickMs := ref.getBasicTickMilliseconds()
	stats := &xTimingWheelsStats{
		ctx:       context.Background(),
		minTickMs: tickMs,
		clock:     ref.getClock(),
		jobAliveCounter: lo.Must[metric.Int64UpDownCounter](otel.Meter(meterName).
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
		jobExecutedCounter: lo.Must[metric.Int64Counter](otel.Meter(meterName).
			Int64Counter(
				"xtw.job.executed.count",
				metric.WithDescription("The number of jobs executed by the timing wheel."),
			),
		),
		jobCancelledCounter: lo.Must[metric.Int64Counter](otel.Meter(meterName).
			Int64Counter(
				"xtw.job.cancelled.count",
				metric.WithDescription("The number of jobs cancelled by the timing wheel."),
			),
		),
		slotCounter: lo.Must[metric.Int64Counter](otel.Meter(meterName).
			Int64Counter(
				"xtw.slot.count",
				metric.WithDescription("The number of slots belongs to the timing wheel."),
			),
		),
	}
	stats.jobTickAccuracy = lo.Must[metric.Float64ObservableGauge](otel.Meter(meterName).
		Float64ObservableGauge(
			"xtw.job.tick.accuracy",
			metric.WithDescription(fmt.Sprintf("The accuracy of the timing wheel tick [-%d,%d] ms.", tickMs, tickMs)),
			metric.WithFloat64Callback(func(ctx context.Context, ob metric.Float64Observer) error {
				accuracy := 0.00
				if stats.jobExecutedCount.Load() > 0 {
					accuracy = float64(stats.jobExecutedCount.Load()-stats.jobHighLatencyCount.Load()) /
						float64(stats.jobExecutedCount.Load())
				}
				ob.Observe(accuracy)
				return nil
			}),
			metric.WithUnit("%"),
		),
	)
	stats.slotActiveCounter = lo.Must[metric.Int64ObservableUpDownCounter](otel.Meter(meterName).
		Int64ObservableUpDownCounter(
			"xtw.slot.active.count",
			metric.WithDescription("The number of slots in active (expired) belongs to the timing wheel."),
			metric.WithInt64Callback(func(ctx context.Context, ob metric.Int64Observer) error {
				slots := stats.slotActiveCount.Load()
				ob.Observe(slots)
				return nil
			}),
		),
	)
	return stats
}
