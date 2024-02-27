//go:build !linux && !windows
// +build !linux,!windows

package timer

import (
	"github.com/benz9527/xboot/lib/hrtime"
)

const (
	SdkDefaultTime TimingWheelTimeSourceEnum = iota
	GoNativeClock
)

func WithTimingWheelTimeSource(source TimingWheelTimeSourceEnum) TimingWheelsOption {
	return func(opt *xTimingWheelsOption) {
		switch source {
		case GoNativeClock:
			opt.clock = hrtime.GoMonotonicClock
		case SdkDefaultTime:
			fallthrough
		default:
			opt.clock = hrtime.SdkClock
		}
	}
}
