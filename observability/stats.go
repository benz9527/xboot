package observability

import (
	"context"
	"go.opentelemetry.io/otel/attribute"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/samber/lo"
	"github.com/shirou/gopsutil/v3/process"
	otelruntime "go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

var (
	once sync.Once
)

type appStats struct {
	ctx              context.Context
	shutdownCallback func(ctx context.Context) error
	goroutines       metric.Int64ObservableUpDownCounter
	processes        metric.Int64ObservableUpDownCounter
	memories         metric.Int64ObservableUpDownCounter
	cpuUsages        metric.Float64ObservableGauge
	cpuTimes         metric.Float64ObservableGauge
	moreRuntimeInfo  bool
}

func (stats *appStats) waitForShutdown() {
	if stats == nil || stats.shutdownCallback == nil {
		return
	}
	go func() {
		select {
		case <-stats.ctx.Done():
			_ = stats.shutdownCallback(context.Background())
		}
	}()
}

func InitAppStats(ctx context.Context, name string) {
	once.Do(func() {
		builder := &strings.Builder{}
		builder.WriteString("xboot/app")
		if len(strings.TrimSpace(name)) > 0 {
			builder.Write([]byte("/"))
			builder.WriteString(name)
		} else {
			builder.Write([]byte("/"))
			builder.WriteString("default")
		}
		name = builder.String()
		stats := &appStats{
			ctx: ctx,
			goroutines: lo.Must[metric.Int64ObservableUpDownCounter](otel.Meter(
				name,
				metric.WithInstrumentationVersion(otelruntime.Version()),
			).Int64ObservableUpDownCounter(
				"app.runtime.goroutines",
				metric.WithDescription(`The application runtime goroutines' info.`),
				metric.WithInt64Callback(func(ctx context.Context, ob metric.Int64Observer) error {
					gNum := runtime.NumGoroutine()
					ob.Observe(int64(gNum))
					return nil
				}),
			)),
			processes: lo.Must[metric.Int64ObservableUpDownCounter](otel.Meter(
				name,
				metric.WithInstrumentationVersion(otelruntime.Version()),
			).Int64ObservableUpDownCounter(
				"app.runtime.processes",
				metric.WithDescription(`The application runtime processes' info.`),
				metric.WithInt64Callback(func(ctx context.Context, ob metric.Int64Observer) error {
					procs := runtime.GOMAXPROCS(0)
					ob.Observe(int64(procs))
					return nil
				}),
			)),
		}
		_, stats.moreRuntimeInfo = os.LookupEnv("AppMoreRuntimeInfo")
		if stats.moreRuntimeInfo {
			pid := os.Getpid()
			proc, err := process.NewProcess(int32(pid))
			if err != nil {
				panic(err)
			}
			stats.memories = lo.Must[metric.Int64ObservableUpDownCounter](otel.Meter(
				name,
				metric.WithInstrumentationVersion(otelruntime.Version()),
			).Int64ObservableUpDownCounter(
				"app.runtime.mem.allocated",
				metric.WithDescription(`The application runtime allocated memory's info.`),
				metric.WithUnit("KB"),
				metric.WithInt64Callback(func(ctx context.Context, ob metric.Int64Observer) error {
					memStats := &runtime.MemStats{}
					runtime.ReadMemStats(memStats)
					ob.Observe(int64(memStats.Alloc >> 10))
					return nil
				}),
			))
			stats.cpuUsages = lo.Must[metric.Float64ObservableGauge](otel.Meter(
				name,
				metric.WithInstrumentationVersion(otelruntime.Version()),
			).Float64ObservableGauge(
				"app.runtime.cpu.usage",
				metric.WithDescription(`The application main process runtime CPU usage percent info.`),
				metric.WithUnit("%"),
				metric.WithFloat64Callback(func(ctx context.Context, ob metric.Float64Observer) error {
					percent, err := proc.CPUPercent()
					if err != nil {
						return err
					}
					times, err := proc.Times()
					if err != nil {
						panic(err)
					}
					set := attribute.NewSet(attribute.Float64("milliseconds", (times.User+times.System)/float64(time.Millisecond)))
					ob.Observe(percent, metric.WithAttributeSet(set))
					return nil
				}),
			))
			stats.cpuTimes = lo.Must[metric.Float64ObservableGauge](otel.Meter(
				name,
				metric.WithInstrumentationVersion(otelruntime.Version()),
			).Float64ObservableGauge(
				"app.runtime.cpu.times",
				metric.WithDescription(`The application main process runtime CPU times info.`),
				metric.WithUnit("ms"),
				metric.WithFloat64Callback(func(ctx context.Context, ob metric.Float64Observer) error {
					times, err := proc.Times()
					if err != nil {
						panic(err)
					}
					cpuTimes := (times.User + times.System) / float64(time.Millisecond)
					ob.Observe(cpuTimes)
					return nil
				}),
			))
		}
		_ = otelruntime.Start()
		stats.waitForShutdown()
	})
}
