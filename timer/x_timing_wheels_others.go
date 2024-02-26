//go:build !windows && !linux
// +build !windows,!linux

package timer

import (
	"context"
	"fmt"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/panjf2000/ants/v2"

	"github.com/benz9527/xboot/lib/hrtime"
	"github.com/benz9527/xboot/lib/infra"
	"github.com/benz9527/xboot/lib/queue"
)

const (
	SdkDefaultTime TimingWheelTimeSourceEnum = iota
	GoNativeClock
)

// NewTimingWheels creates a new timing wheel.
// Same as the kafka, Time.SYSTEM.hiResClockMs() is used.
func NewTimingWheels(ctx context.Context, opts ...TimingWheelsOption) TimingWheels {
	if ctx == nil {
		return nil
	}

	xtw := &xTimingWheels{
		ctx:          ctx,
		stopC:        make(chan struct{}),
		twEventC:     infra.NewSafeClosableChannel[*timingWheelEvent](1024),
		expiredSlotC: infra.NewSafeClosableChannel[TimingWheelSlot](128),
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
	if xtw.idGenerator == nil {
		xtw.idGenerator = func() uint64 {
			return uint64(xtw.clock.NowInDefaultTZ().UnixNano())
		}
	}

	if tw.tickMs <= 0 {
		tw.tickMs = time.Millisecond.Milliseconds()
	}
	if tw.slotSize <= 0 {
		tw.slotSize = 20
	}
	if xtw.isStatsEnabled {
		xtw.stats = newTimingWheelStats(xtw)
	}
	xtw.dq = queue.NewArrayDelayQueue[TimingWheelSlot](ctx, 128)
	xtw.tw = newTimingWheel(
		ctx,
		tw.tickMs,
		tw.slotSize,
		tw.startMs,
		xtw.stats,
		xtw.dq,
		xtw.clock,
	)
	if p, err := ants.NewPool(128, ants.WithPreAlloc(true)); err != nil {
		panic(err)
	} else {
		xtw.gPool = p
	}
	if xtw.name == "" {
		xtw.name = fmt.Sprintf("xtw-%s-%d", runtime.GOOS, xtw.idGenerator())
	}
	xtw.schedule(ctx)
	return xtw
}

func WithTimingWheelTimeSource(source TimingWheelTimeSourceEnum) TimingWheelsOption {
	return func(xtw *xTimingWheels) {
		switch source {
		case GoNativeClock:
			xtw.clock = hrtime.GoMonotonicClock
		case SdkDefaultTime:
			fallthrough
		default:
			xtw.clock = hrtime.SdkClock
		}
	}
}
