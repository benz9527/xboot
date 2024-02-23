//go:build windows
// +build windows

package timer

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/metric"

	"github.com/benz9527/xboot/lib/hrtime"
)

func testAfterFunc(t *testing.T) (percent float64) {
	ctx, cancel := context.WithTimeoutCause(context.Background(), 2100*time.Millisecond, errors.New("timeout"))
	defer cancel()
	tw := NewTimingWheels(
		ctx,
		WithTimingWheelTimeSource(SdkDefaultTime),
		WithTimingWheelSnowflakeID(0, 0),
		withTimingWheelStatsInit(2),
		WithTimingWheelStats(),
	)
	defer func() {
		mp, ok := otel.GetMeterProvider().(*metric.MeterProvider)
		if ok && mp != nil {
			_ = mp.Shutdown(ctx)
		}
	}()

	delays := []time.Duration{
		time.Millisecond,
		2 * time.Millisecond,
		5 * time.Millisecond,
		10 * time.Millisecond,
		15 * time.Millisecond,
		18 * time.Millisecond,
		20 * time.Millisecond,
		21 * time.Millisecond,
		22 * time.Millisecond,
		23 * time.Millisecond,
		50 * time.Millisecond,
		51 * time.Millisecond,
		100 * time.Millisecond,
		200 * time.Millisecond,
		500 * time.Millisecond,
		time.Second,
	}

	expectedExecCount := int64(len(delays))
	actualExecCounter := atomic.Int64{}
	startTs := time.Now().UTC().UnixMilli()
	for i := 0; i < len(delays); i++ {
		_, err := tw.AfterFunc(delays[i], func(ctx context.Context, md JobMetadata) {
			actualExecCounter.Add(1)
			t.Logf("exec diff: %v; delay: %v\n", time.Now().UTC().UnixMilli()-startTs, delays[i])
		})
		assert.NoError(t, err)
	}
	<-ctx.Done()
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, expectedExecCount, actualExecCounter.Load())
	return percent
}

func TestXTimingWheels_AfterFunc(t *testing.T) {
	loops := 1
	percents := 0.0
	for i := 0; i < loops; i++ {
		t.Logf("loop %d\n", i)
		percents += testAfterFunc(t)
	}
	t.Logf("average percent: %f\n", percents/float64(loops))
}

func TestXTimingWheels_ScheduleFunc_windowsClock_1MsInfinite(t *testing.T) {
	defer func() {
		_ = hrtime.ResetTimeResolutionFrom1ms()
	}()
	ctx, cancel := context.WithTimeoutCause(context.Background(), 5*time.Second, errors.New("timeout"))
	defer cancel()
	tw := NewTimingWheels(
		ctx,
		WithTimingWheelTimeSource(WindowsClock),
		WithTimingWheelStats(),
		withTimingWheelStatsInit(5),
	)

	delays := []time.Duration{
		time.Millisecond,
	}
	schedFn := func() Scheduler {
		return NewInfiniteScheduler(delays...)
	}
	assert.NotNil(t, schedFn())
	loop := 20
	tasks := make([]Task, loop)
	for i := range loop {
		var err error
		tasks[i], err = tw.ScheduleFunc(schedFn, func(ctx context.Context, md JobMetadata) {})
		assert.NoError(t, err)
		time.Sleep(2 * time.Millisecond)
	}

	<-ctx.Done()
	time.Sleep(100 * time.Millisecond)
}

func TestXTimingWheels_ScheduleFunc_windowsClock_2MsInfinite(t *testing.T) {
	ctx, cancel := context.WithTimeoutCause(context.Background(), 5*time.Second, errors.New("timeout"))
	defer cancel()
	tw := NewTimingWheels(
		ctx,
		WithTimingWheelTickMs(2*time.Millisecond),
		WithTimingWheelSlotSize(20),
		WithTimingWheelTimeSource(WindowsClock),
		withTimingWheelStatsInit(5),
		WithTimingWheelStats(),
	)

	delays := []time.Duration{
		2 * time.Millisecond,
	}
	schedFn := func() Scheduler {
		return NewInfiniteScheduler(delays...)
	}
	assert.NotNil(t, schedFn())
	loop := 20
	tasks := make([]Task, loop)
	for i := range loop {
		var err error
		tasks[i], err = tw.ScheduleFunc(schedFn, func(ctx context.Context, md JobMetadata) {})
		assert.NoError(t, err)
		time.Sleep(2 * time.Millisecond)
	}

	<-ctx.Done()
	time.Sleep(100 * time.Millisecond)
}
