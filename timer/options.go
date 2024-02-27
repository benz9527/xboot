package timer

import (
	"fmt"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/sdk/metric"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/benz9527/xboot/lib/hrtime"
	"github.com/benz9527/xboot/lib/id"
)

const (
	defaultMinEventBufferSize          = 1024
	defaultMinWorkerPoolSize           = 128
	defaultMinSlotIncrementSize        = 4
	defaultMinTickAccuracyMilliseconds = 1
)

type xTimingWheelsOption struct {
	name         string
	basicTickMs  int64
	slotIncrSize int64
	idGenerator  id.Gen
	stats        *xTimingWheelsStats
	clock        hrtime.Clock
	bufferSize   int
	workPoolSize int
}

func (opt *xTimingWheelsOption) getBasicTickMilliseconds() int64 {
	if opt.basicTickMs < defaultMinTickAccuracyMilliseconds {
		return defaultMinTickAccuracyMilliseconds
	}
	return opt.basicTickMs
}

func (opt *xTimingWheelsOption) getEventBufferSize() int {
	if opt.bufferSize < defaultMinEventBufferSize {
		return defaultMinEventBufferSize
	}
	return opt.bufferSize
}

func (opt *xTimingWheelsOption) getSlotIncrementSize() int64 {
	if opt.slotIncrSize < defaultMinSlotIncrementSize {
		return defaultMinSlotIncrementSize
	}
	return opt.slotIncrSize
}

func (opt *xTimingWheelsOption) getWorkerPoolSize() int {
	if opt.workPoolSize < defaultMinWorkerPoolSize {
		return defaultMinWorkerPoolSize
	}
	return opt.workPoolSize
}

func (opt *xTimingWheelsOption) getExpiredSlotBufferSize() int {
	return int(opt.getSlotIncrementSize() + 8)
}

func (opt *xTimingWheelsOption) getClock() hrtime.Clock {
	if opt.clock == nil {
		return hrtime.SdkClock
	}
	return opt.clock
}

func (opt *xTimingWheelsOption) getIDGenerator() id.Gen {
	if opt.idGenerator == nil {
		return func() uint64 {
			return uint64(opt.getClock().NowInDefaultTZ().UnixNano())
		}
	}
	return opt.idGenerator
}

func (opt *xTimingWheelsOption) getStats() *xTimingWheelsStats {
	return opt.stats
}

func (opt *xTimingWheelsOption) defaultDelayQueueCapacity() int {
	return 128
}

func (opt *xTimingWheelsOption) getName() string {
	if opt.name == "" {
		return fmt.Sprintf("xtw-%s-%d", runtime.GOOS, opt.getIDGenerator()())
	}
	return opt.name
}

type TimingWheelsOption func(option *xTimingWheelsOption)

func WithTimingWheelsTickMs(basicTickMs time.Duration) TimingWheelsOption {
	return func(opt *xTimingWheelsOption) {
		if basicTickMs.Milliseconds() < defaultMinTickAccuracyMilliseconds {
			panic(fmt.Sprintf("timing-wheels' tick accuracy must be greater than or equals to %dms", defaultMinTickAccuracyMilliseconds))
		}
		opt.basicTickMs = basicTickMs.Milliseconds()
	}
}

func WithTimingWheelsSlotSize(slotSize int64) TimingWheelsOption {
	return func(opt *xTimingWheelsOption) {
		if slotSize < defaultMinSlotIncrementSize {
			panic(fmt.Sprintf("timing-wheels' slot size must be greater than or equals to %d", defaultMinSlotIncrementSize))
		}
		opt.slotIncrSize = slotSize
	}
}

func WithTimingWheelsName(name string) TimingWheelsOption {
	return func(opt *xTimingWheelsOption) {
		if len(strings.TrimSpace(name)) <= 0 {
			panic("timing-wheels' name must not be empty or blank")
		}
		opt.name = name
	}
}

func WithTimingWheelsSnowflakeID(datacenterID, machineID int64) TimingWheelsOption {
	return func(opt *xTimingWheelsOption) {
		idGenerator, err := id.StandardSnowFlakeID(datacenterID, machineID, func() time.Time {
			if opt.clock == nil {
				panic("timing-wheels' clock must be not nil")
			}
			return opt.clock.NowInDefaultTZ()
		})
		if err != nil {
			panic(err)
		}
		opt.idGenerator = idGenerator
	}
}

func WithTimingWheelsStats() TimingWheelsOption {
	return func(opt *xTimingWheelsOption) {
		opt.stats = newTimingWheelStats(opt)
	}
}

func WithTimingWheelsWorkerPoolSize(poolSize int) TimingWheelsOption {
	return func(opt *xTimingWheelsOption) {
		if poolSize < defaultMinWorkerPoolSize {
			panic(fmt.Sprintf("timing-wheels' work pool size must be greater than or equals to %d", defaultMinWorkerPoolSize))
		}
		opt.workPoolSize = poolSize
	}
}

func WithTimingWheelsEventBufferSize(size int) TimingWheelsOption {
	return func(opt *xTimingWheelsOption) {
		if size < defaultMinEventBufferSize {
			panic(fmt.Sprintf("timing-wheels' event buffer size must be greater than or equals to %d", defaultMinEventBufferSize))
		}
		opt.bufferSize = size
	}
}

func withTimingWheelsStatsInit(interval int64) TimingWheelsOption {
	return func(opt *xTimingWheelsOption) {
		exp, err := stdoutmetric.New(
			stdoutmetric.WithWriter(os.Stdout),
		)
		if err != nil {
			panic(err)
		}
		mp := metric.NewMeterProvider(metric.WithReader(metric.NewPeriodicReader(exp, metric.WithInterval(time.Duration(interval)*time.Second))))
		otel.SetMeterProvider(mp)
	}
}
