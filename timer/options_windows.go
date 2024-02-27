//go:build windows
// +build windows

package timer

import (
	"github.com/benz9527/xboot/lib/hrtime"
)

const (
	SdkDefaultTime TimingWheelTimeSourceEnum = iota
	WindowsClock
)

func WithTimingWheelTimeSource(source TimingWheelTimeSourceEnum) TimingWheelsOption {
	return func(opt *xTimingWheelsOption) {
		switch source {
		case WindowsClock:
			opt.clock = hrtime.WindowsClock
		case SdkDefaultTime:
			fallthrough
		default:
			opt.clock = hrtime.SdkClock
		}
	}
}
