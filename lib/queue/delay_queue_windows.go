//go:build windows
// +build windows

package queue

import (
	"github.com/benz9527/xboot/lib/hrtime"
	"github.com/benz9527/xboot/lib/infra"
)

func (dq *ArrayDelayQueue[E]) PollToChan(nowFn func() int64, C infra.SendOnlyChannel[E]) {
	// Note: The timer resolution is set to 1ms to improve the accuracy of the delay queue.
	// But below implementation is not a good solution.
	dq.exclusion.Lock()
	hrtime.SetTimeResolutionTo1ms()
	defer func() {
		_ = hrtime.ResetTimeResolutionFrom1ms()
		dq.exclusion.Unlock()
	}()

	dq.poll(nowFn, C)
}
