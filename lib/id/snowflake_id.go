package id

import (
	"fmt"
	"strconv"
	"sync"
	"time"
)

// SnowFlake 64 bits
// 0-00000000_00000000_00000000_00000000_00000000_0-00000_00000-00000000_0000
// 1 bit, as symbol, it always set as 0
// 41 bits, the diff val between current ts and start ts
// The timestamp in high bits will be affected by clock skew (clock rollback).
// 5 bits, as datacenter id
// 5 bits, as machine id
// 12 bits, as internal sequence number, max value is 4096 (2 ^ 12)

const (
	classicSnowflakeStartEpoch      = int64(946659661000) // 2001-01-01 01:01:01 UTC+8
	classicSnowflakeTsDiffBits      = uint(41)
	classicSnowflakeDIDBits         = uint(5) // DataCenter ID
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

var (
	errSFInvalidDataCenterID = fmt.Errorf("data-center id invalid")
	errSFInvalidMachineID    = fmt.Errorf("machine id invalid")
)

// The now function is easily to be affected by clock skew. Then
// the global and unique id is unstable.
// Deprecated: Please use the SnowFlakeID(datacenterID, machineID int64, now func() time.Time) (UUIDGen, error)
func StandardSnowFlakeID(dataCenterID, machineID int64, now func() time.Time) (Gen, error) {
	if dataCenterID < 0 || dataCenterID > classicSnowflakeDIDMax {
		return nil, fmt.Errorf("dataCenterID: %d (max: %d), %w", dataCenterID, classicSnowflakeDIDMax, errSFInvalidDataCenterID)
	}

	if machineID < 0 || machineID > classicSnowflakeMIDMax {
		return nil, fmt.Errorf("machineID: %d (max: %d), %w", machineID, classicSnowflakeMIDMax, errSFInvalidMachineID)
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

		diff := now - classicSnowflakeStartEpoch
		if diff > classicSnowflakeTsDiffMax {
			return 0
		}
		lastTs = now
		id := (diff << classicSnowflakeTsDiffShiftLeft) |
			(dataCenterID << classicSnowflakeDIDShiftLeft) |
			(machineID << classicSnowflakeMIDShiftLeft) |
			sequence
		return uint64(id)
	}, nil
}

func SnowFlakeID(dataCenterID, machineID int64, now func() time.Time) (UUIDGen, error) {
	if dataCenterID < 0 || dataCenterID > classicSnowflakeDIDMax {
		return nil, errSFInvalidDataCenterID
	}

	if machineID < 0 || machineID > classicSnowflakeMIDMax {
		return nil, errSFInvalidMachineID
	}

	var lock = sync.Mutex{}
	lastTs, sequence := int64(0), int64(0)
	id := new(uuidDelegator)
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

		diff := now - classicSnowflakeStartEpoch
		if diff > classicSnowflakeTsDiffMax {
			return 0
		}
		lastTs = now
		id := (diff << classicSnowflakeTsDiffShiftLeft) |
			(dataCenterID << classicSnowflakeDIDShiftLeft) |
			(machineID << classicSnowflakeMIDShiftLeft) |
			sequence
		return uint64(id)
	}
	id.str = func() string {
		return strconv.FormatUint(id.number(), 10)
	}
	return id, nil
}
