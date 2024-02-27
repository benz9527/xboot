//go:build linux
// +build linux

package timer

import (
	"github.com/benz9527/xboot/lib/hrtime"
)

const (
	SdkDefaultTime TimingWheelTimeSourceEnum = iota
	GoNativeClock
	UnixClock
)

func WithTimingWheelTimeSource(source TimingWheelTimeSourceEnum) TimingWheelsOption {
	return func(opt *xTimingWheelsOption) {
		switch source {
		case GoNativeClock:
			opt.clock = hrtime.GoMonotonicClock
		case UnixClock:
			opt.clock = hrtime.UnixMonotonicClock
		case SdkDefaultTime:
			fallthrough
		default:
			opt.clock = hrtime.SdkClock
		}
	}
}
