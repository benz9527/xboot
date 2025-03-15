//go:build darwin
// +build darwin

package hrtime

import (
	"testing"

	"golang.org/x/sys/unix"
)

func TestDarwinUnixTimerResolution(t *testing.T) {
	res := unix.Timespec{}
	_ = unix.ClockGettime(unix.CLOCK_MONOTONIC_RAW, &res)
	t.Logf("Monotonic clock resolution is %d nanoseconds\n", res.Nsec)
}
