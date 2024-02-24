//go:build linux
// +build linux

package timer

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestXTimingWheels_ScheduleFunc_goNativeClock_1MsInfinite(t *testing.T) {
	ctx, cancel := context.WithTimeoutCause(context.Background(), 5*time.Second, errors.New("timeout"))
	defer cancel()
	tw := NewTimingWheels(
		ctx,
		withTimingWheelStatsInit(5),
		WithTimingWheelStats(),
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

func TestXTimingWheels_ScheduleFunc_goNativeClock_2MsInfinite(t *testing.T) {
	ctx, cancel := context.WithTimeoutCause(context.Background(), 5*time.Second, errors.New("timeout"))
	defer cancel()
	tw := NewTimingWheels(
		ctx,
		WithTimingWheelTickMs(2*time.Millisecond),
		WithTimingWheelSlotSize(20),
		WithTimingWheelTimeSource(GoNativeClock),
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

func TestXTimingWheels_ScheduleFunc_unixClock_1MsInfinite(t *testing.T) {
	ctx, cancel := context.WithTimeoutCause(context.Background(), 5*time.Second, errors.New("timeout"))
	defer cancel()
	tw := NewTimingWheels(
		ctx,
		WithTimingWheelTimeSource(UnixClock),
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

func TestXTimingWheels_ScheduleFunc_unixClock_2MsInfinite(t *testing.T) {
	ctx, cancel := context.WithTimeoutCause(context.Background(), 5*time.Second, errors.New("timeout"))
	defer cancel()
	tw := NewTimingWheels(
		ctx,
		WithTimingWheelTickMs(2*time.Millisecond),
		WithTimingWheelSlotSize(20),
		WithTimingWheelTimeSource(UnixClock),
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
