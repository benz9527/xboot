//go:build linux && !windows
// +build linux,!windows

package queue

import (
	"github.com/benz9527/xboot/lib/infra"
)

func (dq *ArrayDelayQueue[E]) PollToChan(nowFn func() int64, C infra.SendOnlyChannel[E]) {
	dq.exclusion.Lock()
	defer dq.exclusion.Unlock()

	dq.poll(nowFn, C)
}
