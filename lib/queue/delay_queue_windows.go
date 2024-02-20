//go:build windows
// +build windows

package queue

import (
	"golang.org/x/sys/windows"

	"github.com/benz9527/xboot/lib/ipc"
)

func setTimeResolutionTo1ms() {
	if err := windows.TimeBeginPeriod(1); err != nil {
		panic(err)
	}
}

func resetTimeResolution() error {
	return windows.TimeEndPeriod(1)
}

func (dq *ArrayDelayQueue[E]) PollToChan(nowFn func() int64, C ipc.SendOnlyChannel[E]) {
	// Note: The timer resolution is set to 1ms to improve the accuracy of the delay queue.
	// But below implementation is not a good solution.
	dq.exclusion.Lock()
	setTimeResolutionTo1ms()
	defer func() {
		resetTimeResolution()
		dq.exclusion.Unlock()
	}()

	dq.poll(nowFn, C)
}
