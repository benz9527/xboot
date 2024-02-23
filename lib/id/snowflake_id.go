package id

import (
	"fmt"
	"sync"
	"time"
)

// SnowFlake 64 bits
// 0-00000000 00000000 00000000 00000000 00000000 0-00000000 00-00000000 0000
// 1 bit, as symbol, it always set as 0
// 41 bits, the diff val between current ts and start ts
// 5 bits, as datacenter id
// 5 bits, as machine id
// 12 bits, as internal sequence number, max value is 4096 (2 ^ 12)

const (
	SF_START_TS                 = int64(946659661000) // 2001-01-01 01:01:01 UTC+8
	SF_TS_DIFF_BITS             = uint(41)
	SF_DATACENTER_ID_BITS       = uint(5)
	SF_MACHINE_ID_BITS          = uint(5)
	SF_SEQUENCE_BITS            = uint(12)
	SF_MACHINE_ID_SHIFT_LEFT    = SF_SEQUENCE_BITS
	SF_DATACENTER_ID_SHIFT_LEFT = SF_MACHINE_ID_SHIFT_LEFT + SF_MACHINE_ID_BITS
	SF_TS_DIFF_SHIFT_LEFT       = SF_DATACENTER_ID_SHIFT_LEFT + SF_DATACENTER_ID_BITS // 22 bits

	SF_SEQUENCE_MAX      = int64(-1 ^ (-1 << SF_SEQUENCE_BITS))
	SF_MACHINE_ID_MAX    = int64(-1 ^ (-1 << SF_MACHINE_ID_BITS))
	SF_DATACENTER_ID_MAX = SF_MACHINE_ID_MAX
	SF_TS_DIFF_MAX       = int64(-1 ^ (-1 << SF_TS_DIFF_BITS))
)

type Gen func() uint64

func StandardSnowFlakeID(datacenterID, machineID int64, now func() time.Time) (Gen, error) {
	if datacenterID < 0 || datacenterID > SF_DATACENTER_ID_MAX {
		return nil, fmt.Errorf("datacenter id invalid")
	}

	if machineID < 0 || machineID > SF_MACHINE_ID_MAX {
		return nil, fmt.Errorf("machine id invalid")
	}

	var lock = sync.Mutex{}
	lastTs, sequence := int64(0), int64(0)
	return func() uint64 {
		lock.Lock()
		defer lock.Unlock()

		now := now().UnixNano() / 1e6
		if now != lastTs {
			sequence = int64(0)
		} else {
			sequence = (sequence + 1) & SF_SEQUENCE_MAX
			if sequence == 0 {
				for now <= lastTs {
					now = time.Now().UnixNano() / 1e6
				}
			}
		}

		diff := now - SF_START_TS
		if diff > SF_TS_DIFF_MAX {
			return 0
		}
		lastTs = now
		id := ((diff) << SF_TS_DIFF_SHIFT_LEFT) |
			(datacenterID << SF_DATACENTER_ID_SHIFT_LEFT) |
			(machineID << SF_MACHINE_ID_SHIFT_LEFT) |
			sequence
		return uint64(id)
	}, nil
}
