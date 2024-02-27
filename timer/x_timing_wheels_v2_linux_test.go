//go:build linux
// +build linux

package timer

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/benz9527/xboot/lib/hrtime"
	"github.com/benz9527/xboot/observability"
)

func TestXTimingWheelsV2_ScheduleFunc_goNativeClock_1MsInfinite(t *testing.T) {
	hrtime.ClockInit()
	observability.InitAppStats(context.Background(), "goNative1msInfinite")
	ctx, cancel := context.WithTimeoutCause(context.Background(), 5*time.Second, errors.New("timeout"))
	defer cancel()
	tw := NewXTimingWheelsV2(
		ctx,
		withTimingWheelsStatsInit(5),
		WithTimingWheelsStats(),
		WithTimingWheelTimeSource(GoNativeClock),
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

func TestXTimingWheelsV2_ScheduleFunc_goNativeClock_2MsInfinite(t *testing.T) {
	hrtime.ClockInit()
	observability.InitAppStats(context.Background(), "goNative2msInfinite")
	ctx, cancel := context.WithTimeoutCause(context.Background(), 5*time.Second, errors.New("timeout"))
	defer cancel()
	tw := NewXTimingWheelsV2(
		ctx,
		WithTimingWheelsTickMs(2*time.Millisecond),
		WithTimingWheelsSlotSize(20),
		WithTimingWheelTimeSource(GoNativeClock),
		withTimingWheelsStatsInit(5),
		WithTimingWheelsStats(),
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

func TestXTimingWheelsV2_ScheduleFunc_unixClock_1MsInfinite(t *testing.T) {
	hrtime.ClockInit()
	observability.InitAppStats(context.Background(), "unix1msInfinite")
	ctx, cancel := context.WithTimeoutCause(context.Background(), 5*time.Second, errors.New("timeout"))
	defer cancel()
	tw := NewXTimingWheelsV2(
		ctx,
		WithTimingWheelTimeSource(UnixClock),
		WithTimingWheelsStats(),
		withTimingWheelsStatsInit(5),
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

func TestXTimingWheelsV2_ScheduleFunc_unixClock_2MsInfinite(t *testing.T) {
	hrtime.ClockInit()
	observability.InitAppStats(context.Background(), "unix2msInfinite")
	ctx, cancel := context.WithTimeoutCause(context.Background(), 5*time.Second, errors.New("timeout"))
	defer cancel()
	tw := NewXTimingWheelsV2(
		ctx,
		WithTimingWheelsTickMs(2*time.Millisecond),
		WithTimingWheelsSlotSize(20),
		WithTimingWheelTimeSource(UnixClock),
		withTimingWheelsStatsInit(5),
		WithTimingWheelsStats(),
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
