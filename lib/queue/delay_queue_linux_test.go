//go:build linux
// +build linux

package queue

import (
	"testing"

	"golang.org/x/sys/unix"
)

func TestUnixTimerResolution(t *testing.T) {
	res := unix.Timespec{}
	_ = unix.ClockGetres(unix.CLOCK_MONOTONIC, &res)
	t.Logf("Monotonic clock resolution is %d nanoseconds\n", res.Nsec)
}
