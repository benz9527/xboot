package id

import (
	"strconv"
	"sync/atomic"
	"time"

	"github.com/benz9527/xboot/lib/infra"
)

// SnowFlakeID Clock Rollback Issue.
// References:
// https://github.com/apache/incubator-seata/blob/2.x/common/src/main/java/org/apache/seata/common/util/IdWorker.java
// https://seata.apache.org/zh-cn/blog/seata-snowflake-explain/

// 0-00000_00000-00000000_00000000_00000000_00000000_00000000_0-00000000_0000
// 1 bit, as symbol, it always set as 0
// 5 bits, as datacenter id
// 5 bits, as machine id
// 41 bits, the diff val between current ts and start ts
// 12 bits, as internal sequence number, max value is 4096 (2 ^ 12)
//
// In this algorithm, it seems that the ID only has monotonic in single
// machine, which is not an global and unique ID. But it is enough for
// reducing the b+ tree page split in ORM database, such as MySQL innodb.
// b+ tree page split is unfriendly to the IO performance. We have to create
// new pages and copy the data to the new pages.
// In this algorithm, the ID may still contains b+ tree page split, but
// it will turn into stable after several split operations.
// It is convergent algorithm!

const (
	xSnowflakeStartEpoch        = int64(1712336461000) // 2024-04-06 01:01:01 UTC+8
	xSnowflakeDIDBits           = uint(5)              // DataCenter ID
	xSnowflakeMIDBits           = uint(5)              // Machine ID
	xSnowflakeTsDiffBits        = uint(41)
	xSnowflakeSequenceBits      = uint(12) // max 4096
	xSnowflakeTsDiffShiftLeft   = xSnowflakeSequenceBits
	xSnowflakeMIDShiftLeft      = xSnowflakeTsDiffShiftLeft + xSnowflakeTsDiffBits
	xSnowflakeDIDShiftLeft      = xSnowflakeMIDShiftLeft + xSnowflakeMIDBits
	xSnowflakeSequenceMax       = int64(-1 ^ (-1 << xSnowflakeSequenceBits))
	xSnowflakeMIDMax            = int64(-1 ^ (-1 << xSnowflakeMIDBits))
	xSnowflakeDIDMax            = xSnowflakeMIDMax
	xSnowflakeTsDiffMax         = int64(-1 ^ (-1 << xSnowflakeTsDiffBits))
	xSnowflakeWorkerIDMax       = int64(-1 ^ (-1 << (xSnowflakeMIDBits + xSnowflakeDIDBits)))
	xSnowflakeTsAndSequenceMask = int64(-1 ^ (-1 << (xSnowflakeTsDiffBits + xSnowflakeSequenceBits)))
)

const (
	errSFInvalidWorkerID = sfError("worker id invalid")
)

// The now function could use the relative timestamp generator implementation.
func xSnowFlakeID(dataCenterID, machineID int64, now func() time.Time) (UUIDGen, error) {
	if dataCenterID < 0 || dataCenterID > xSnowflakeDIDMax {
		return nil, infra.WrapErrorStackWithMessage(errSFInvalidDataCenterID, "dataCenterID: "+strconv.FormatInt(dataCenterID, 10)+
			" (max: "+strconv.FormatInt(xSnowflakeDIDMax, 10)+")")
	}

	if machineID < 0 || machineID > xSnowflakeMIDMax {
		return nil, infra.WrapErrorStackWithMessage(errSFInvalidMachineID, "machineID: "+strconv.FormatInt(machineID, 10)+
			" (max: "+strconv.FormatInt(xSnowflakeMIDMax, 10)+")")
	}

	tsAndSequence := now().UnixNano() / 1e6
	tsAndSequence <<= xSnowflakeTsDiffShiftLeft
	waitIfNecessary := func() {
		curTsAndSeq := atomic.LoadInt64(&tsAndSequence)
		cur := curTsAndSeq >> xSnowflakeTsDiffShiftLeft
		latest := now().UnixNano() / 1e6
		if latest <= cur { // clock skew
			time.Sleep(5 * time.Millisecond) // clock forward maybe blocked!!!
		}
	}

	id := new(uuidDelegator)
	id.number = func() uint64 {
		waitIfNecessary()
		tsAndSeq := atomic.AddInt64(&tsAndSequence, 1) & xSnowflakeTsAndSequenceMask
		id := tsAndSeq |
			(dataCenterID << xSnowflakeDIDShiftLeft) |
			(machineID << xSnowflakeMIDShiftLeft)
		return uint64(id)
	}
	id.str = func() string {
		return strconv.FormatUint(id.number(), 10)
	}
	return id, nil
}

func XSnowFlakeID(dataCenterID, machineID int64, now func() time.Time) (UUIDGen, error) {
	return xSnowFlakeID(dataCenterID, machineID, now)
}

func XSnowFlakeIDByWorkerID(workerID int64, now func() time.Time) (UUIDGen, error) {
	if workerID < 0 || workerID > xSnowflakeWorkerIDMax {
		return nil, infra.WrapErrorStackWithMessage(errSFInvalidWorkerID, "workerID: "+strconv.FormatInt(workerID, 10)+
			" (max: "+strconv.FormatInt(xSnowflakeWorkerIDMax, 10)+")")
	}
	idMask := int64(0x1f)
	mID := workerID & idMask
	dID := (workerID >> 5) & idMask
	return XSnowFlakeID(dID, mID, now)
}
