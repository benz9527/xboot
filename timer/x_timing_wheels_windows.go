//go:build windows
// +build windows

package timer

import (
	"context"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/panjf2000/ants/v2"

	"github.com/benz9527/xboot/lib/hrtime"
	"github.com/benz9527/xboot/lib/ipc"
	"github.com/benz9527/xboot/lib/queue"
)

const (
	SdkDefaultTime TimingWheelTimeSourceEnum = iota
	WindowsClock
)

// NewTimingWheels creates a new timing wheel.
// Same as the kafka, Time.SYSTEM.hiResClockMs() is used.
func NewTimingWheels(ctx context.Context, opts ...TimingWheelsOption) TimingWheels {
	if ctx == nil {
		return nil
	}

	xtw := &xTimingWheels{
		ctx:          ctx,
		taskCounter:  &atomic.Int64{},
		slotCounter:  &atomic.Int64{},
		stopC:        make(chan struct{}),
		twEventC:     ipc.NewSafeClosableChannel[*timingWheelEvent](1024),
		expiredSlotC: ipc.NewSafeClosableChannel[TimingWheelSlot](128),
		tasksMap:     make(map[JobID]Task),
		isRunning:    &atomic.Bool{},
		twEventPool:  newTimingWheelEventsPool(),
		tw:           &timingWheel{},
	}
	xtw.isRunning.Store(false)
	for _, o := range opts {
		if o != nil {
			o(xtw)
		}
	}
	// Temporarily store the configuration
	tw := xtw.tw.(*timingWheel)
	if xtw.clock == nil {
		xtw.clock = hrtime.SdkClock
	}
	tw.startMs = xtw.clock.NowInDefaultTZ().UnixMilli()

	if tw.tickMs <= 0 {
		tw.tickMs = time.Millisecond.Milliseconds()
	}
	if tw.slotSize <= 0 {
		tw.slotSize = 20
	}
	xtw.dq = queue.NewArrayDelayQueue[TimingWheelSlot](ctx, 128)
	xtw.tw = newTimingWheel(
		ctx,
		tw.tickMs,
		tw.slotSize,
		tw.startMs,
		xtw.slotCounter,
		xtw.dq,
		xtw.clock,
	)
	if p, err := ants.NewPool(128, ants.WithPreAlloc(true)); err != nil {
		panic(err)
	} else {
		xtw.gPool = p
	}
	if xtw.name == "" {
		// FIXME UUID
		xtw.name = "default-" + strconv.FormatInt(xtw.GetStartMs(), 10)
	}
	if xtw.isStatsEnabled {
		xtw.stats = newTimingWheelStats(xtw)
	}
	xtw.schedule(ctx)
	return xtw
}

func WithTimingWheelTimeSource(source TimingWheelTimeSourceEnum) TimingWheelsOption {
	return func(xtw *xTimingWheels) {
		switch source {
		case WindowsClock:
			xtw.clock = hrtime.WindowsClock
		case SdkDefaultTime:
			fallthrough
		default:
			xtw.clock = hrtime.SdkClock
		}
	}
}
