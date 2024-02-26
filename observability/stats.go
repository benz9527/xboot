package observability

import (
	"context"
	"runtime"
	"strings"
	"sync"

	"github.com/samber/lo"
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
				"app.core.goroutines",
				metric.WithDescription(`The application goroutines' info.`),
				metric.WithInt64Callback(func(ctx context.Context, ob metric.Int64Observer) error {
					gNum := runtime.NumGoroutine()
					ob.Observe(int64(gNum))
					return nil
				}),
			),
			),
			processes: lo.Must[metric.Int64ObservableUpDownCounter](otel.Meter(
				name,
				metric.WithInstrumentationVersion(otelruntime.Version()),
			).Int64ObservableUpDownCounter(
				"app.core.processes",
				metric.WithDescription(`The application processes' info.`),
				metric.WithInt64Callback(func(ctx context.Context, ob metric.Int64Observer) error {
					procs := runtime.GOMAXPROCS(0)
					ob.Observe(int64(procs))
					return nil
				}),
			),
			),
		}
		_ = otelruntime.Start()
		stats.waitForShutdown()
	})
}
