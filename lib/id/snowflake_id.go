package id

import (
	"fmt"
	"strconv"
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
	classicSnowflakeStartTs         = int64(946659661000) // 2001-01-01 01:01:01 UTC+8
	classicSnowflakeTsDiffBits      = uint(41)
	classicSnowflakeDIDBits         = uint(5) // Datacenter ID
	classicSnowflakeMIDBits         = uint(5) // Machine ID
	classicSnowflakeSequenceBits    = uint(12)
	classicSnowflakeMIDShiftLeft    = classicSnowflakeSequenceBits
	classicSnowflakeDIDShiftLeft    = classicSnowflakeMIDShiftLeft + classicSnowflakeMIDBits
	classicSnowflakeTsDiffShiftLeft = classicSnowflakeDIDShiftLeft + classicSnowflakeDIDBits // 22 bits
	classicSnowflakeSequenceMax     = int64(-1 ^ (-1 << classicSnowflakeSequenceBits))
	classicSnowflakeMIDMax          = int64(-1 ^ (-1 << classicSnowflakeMIDBits))
	classicSnowflakeDIDMax          = classicSnowflakeMIDMax
	classicSnowflakeTsDiffMax       = int64(-1 ^ (-1 << classicSnowflakeTsDiffBits))
)

func StandardSnowFlakeID(datacenterID, machineID int64, now func() time.Time) (Gen, error) {
	if datacenterID < 0 || datacenterID > classicSnowflakeDIDMax {
		return nil, fmt.Errorf("datacenter id invalid")
	}

	if machineID < 0 || machineID > classicSnowflakeMIDMax {
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
			sequence = (sequence + 1) & classicSnowflakeSequenceMax
			if sequence == 0 {
				for now <= lastTs {
					now = time.Now().UnixNano() / 1e6
				}
			}
		}

		diff := now - classicSnowflakeStartTs
		if diff > classicSnowflakeTsDiffMax {
			return 0
		}
		lastTs = now
		id := ((diff) << classicSnowflakeTsDiffShiftLeft) |
			(datacenterID << classicSnowflakeDIDShiftLeft) |
			(machineID << classicSnowflakeMIDShiftLeft) |
			sequence
		return uint64(id)
	}, nil
}

func SnowFlakeID(datacenterID, machineID int64, now func() time.Time) (Generator, error) {
	if datacenterID < 0 || datacenterID > classicSnowflakeDIDMax {
		return nil, fmt.Errorf("datacenter id invalid")
	}

	if machineID < 0 || machineID > classicSnowflakeMIDMax {
		return nil, fmt.Errorf("machine id invalid")
	}

	var lock = sync.Mutex{}
	lastTs, sequence := int64(0), int64(0)
	id := new(defaultID)
	id.number = func() uint64 {
		lock.Lock()
		defer lock.Unlock()

		now := now().UnixNano() / 1e6
		if now != lastTs {
			sequence = int64(0)
		} else {
			sequence = (sequence + 1) & classicSnowflakeSequenceMax
			if sequence == 0 {
				for now <= lastTs {
					now = time.Now().UnixNano() / 1e6
				}
			}
		}

		diff := now - classicSnowflakeStartTs
		if diff > classicSnowflakeTsDiffMax {
			return 0
		}
		lastTs = now
		id := ((diff) << classicSnowflakeTsDiffShiftLeft) |
			(datacenterID << classicSnowflakeDIDShiftLeft) |
			(machineID << classicSnowflakeMIDShiftLeft) |
			sequence
		return uint64(id)
	}
	id.str = func() string {
		return strconv.FormatUint(id.number(), 10)
	}
	return id, nil
}
