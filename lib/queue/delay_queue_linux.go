//go:build linux
// +build linux

package queue

import (
	"github.com/benz9527/xboot/lib/infra"
)

func (dq *ArrayDelayQueue[E]) PollToChan(nowFn func() int64, C infra.SendOnlyChannel[E]) {
	dq.exclusion.Lock()
	defer dq.exclusion.Unlock()

	dq.poll(nowFn, C)
}
