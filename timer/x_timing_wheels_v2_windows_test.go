//go:build windows
// +build windows

package timer

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/benz9527/xboot/lib/hrtime"
	"github.com/benz9527/xboot/observability"
	"github.com/stretchr/testify/assert"
	"go.uber.org/automaxprocs/maxprocs"
)

func TestXTimingWheelsV2_ScheduleFunc_windowsClock_1MsInfinite(t *testing.T) {
	_, _ = maxprocs.Set(maxprocs.Min(4), maxprocs.Logger(t.Logf))
	hrtime.ClockInit()
	observability.InitAppStats(context.Background(), "window1msInfinite")
	defer func() {
		_ = hrtime.ResetTimeResolutionFrom1ms()
	}()
	ctx, cancel := context.WithTimeoutCause(context.Background(), 5*time.Second, errors.New("timeout"))
	defer cancel()
	tw := NewXTimingWheelsV2(
		ctx,
		WithTimingWheelTimeSource(WindowsClock),
		WithTimingWheelsStats(),
		withTimingWheelsDebugStatsInit(5),
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
