package timer

import (
	"fmt"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/sdk/metric"
	"log/slog"
	"math"
	"os"
	"runtime"
	"strings"
	"sync/atomic"
	"time"

	"github.com/benz9527/xboot/lib/hrtime"
	"github.com/benz9527/xboot/lib/id"
)

const (
	defaultMinEventBufferSize          = 1024
	defaultMinWorkerPoolSize           = 128
	defaultMinSlotIncrementSize        = 10
	defaultMinIntervalMilliseconds     = 20 // lt 20ms will overflow
	defaultMinTickAccuracyMilliseconds = 1
)

type xTimingWheelsOption struct {
	name            string
	basicTickMs     int64
	slotIncrSize    int64
	idGenerator     id.Gen
	stats           *xTimingWheelsStats
	clock           hrtime.Clock
	bufferSize      int
	workPoolSize    int
	_isValueChecked *atomic.Bool
}

func (opt *xTimingWheelsOption) getBasicTickMilliseconds() int64 {
	if opt._isValueChecked == nil || !opt._isValueChecked.Load() {
		panic("value unchecked")
	}
	if opt.basicTickMs < defaultMinTickAccuracyMilliseconds {
		return defaultMinTickAccuracyMilliseconds
	}
	return opt.basicTickMs
}

func (opt *xTimingWheelsOption) getEventBufferSize() int {
	if opt._isValueChecked == nil || !opt._isValueChecked.Load() {
		panic("value unchecked")
	}
	if opt.bufferSize < defaultMinEventBufferSize {
		return defaultMinEventBufferSize
	}
	return opt.bufferSize
}

func (opt *xTimingWheelsOption) getSlotIncrementSize() int64 {
	if opt._isValueChecked == nil || !opt._isValueChecked.Load() {
		panic("value unchecked")
	}
	if opt.slotIncrSize < defaultMinSlotIncrementSize {
		return defaultMinSlotIncrementSize
	}
	return opt.slotIncrSize
}

func (opt *xTimingWheelsOption) getWorkerPoolSize() int {
	if opt._isValueChecked == nil || !opt._isValueChecked.Load() {
		panic("value unchecked")
	}
	if opt.workPoolSize < defaultMinWorkerPoolSize {
		return defaultMinWorkerPoolSize
	}
	return opt.workPoolSize
}

func (opt *xTimingWheelsOption) getExpiredSlotBufferSize() int {
	if opt._isValueChecked == nil || !opt._isValueChecked.Load() {
		panic("value unchecked")
	}
	return int(opt.getSlotIncrementSize() + 8)
}

func (opt *xTimingWheelsOption) getClock() hrtime.Clock {
	if opt._isValueChecked == nil || !opt._isValueChecked.Load() {
		panic("value unchecked")
	}
	if opt.clock == nil {
		return hrtime.SdkClock
	}
	return opt.clock
}

func (opt *xTimingWheelsOption) getIDGenerator() id.Gen {
	if opt._isValueChecked == nil || !opt._isValueChecked.Load() {
		panic("value unchecked")
	}
	if opt.idGenerator == nil {
		gen, err := id.StandardSnowFlakeID(0, 0, func() time.Time {
			return opt.getClock().NowInDefaultTZ()
		})
		if err != nil {
			panic(err)
		}
		opt.idGenerator = gen
	}
	return opt.idGenerator
}

func (opt *xTimingWheelsOption) getStats() *xTimingWheelsStats {
	if opt._isValueChecked == nil || !opt._isValueChecked.Load() {
		panic("value unchecked")
	}
	return opt.stats
}

func (opt *xTimingWheelsOption) defaultDelayQueueCapacity() int {
	if opt._isValueChecked == nil || !opt._isValueChecked.Load() {
		panic("value unchecked")
	}
	return 128
}

func (opt *xTimingWheelsOption) getName() string {
	if opt._isValueChecked == nil || !opt._isValueChecked.Load() {
		panic("value unchecked")
	}
	if opt.name == "" {
		return fmt.Sprintf("xtw-%s-%d", runtime.GOOS, opt.getIDGenerator()())
	}
	return opt.name
}

func (opt *xTimingWheelsOption) Validate() {
	opt._isValueChecked = &atomic.Bool{}
	if opt.basicTickMs < 1 {
		opt.basicTickMs = defaultMinTickAccuracyMilliseconds
		slog.Warn("adjust the tick accuracy", "from", 0, "to", opt.basicTickMs)
	}
	if opt.basicTickMs > 0 && opt.slotIncrSize > 0 &&
		opt.basicTickMs*opt.slotIncrSize < defaultMinIntervalMilliseconds {
		from := opt.slotIncrSize
		opt.slotIncrSize = int64(math.Ceil(float64(defaultMinIntervalMilliseconds) / float64(opt.basicTickMs)))
		slog.Warn("adjust the slot increment size", "from", from, "to", opt.slotIncrSize)
	}
	if opt.basicTickMs >= 1 && opt.slotIncrSize < 1 {
		opt.slotIncrSize = int64(math.Ceil(float64(defaultMinIntervalMilliseconds) / float64(opt.basicTickMs)))
		slog.Warn("adjust the slot increment size", "from", 0, "to", opt.slotIncrSize)
	}
	opt._isValueChecked.Store(true)
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
		if slotSize < 1 {
			panic("empty slot increment size")
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
