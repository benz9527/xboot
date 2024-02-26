package observability

// https://opentelemetry.io/docs/languages/go/exporters/

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/sdk/metric"
)

// Serves for test/dev environment.
func newConsoleMetricsExporter(interval, timeout time.Duration, opts ...stdoutmetric.Option) (func(ctx context.Context) error, error) {
	exporter, err := stdoutmetric.New(opts...)
	if err != nil {
		return nil, err
	}
	mp := metric.NewMeterProvider(metric.WithReader(metric.NewPeriodicReader(
		exporter,
		metric.WithInterval(interval),
		metric.WithTimeout(timeout),
	)))
	callback := mp.Shutdown
	otel.SetMeterProvider(mp)
	return callback, nil
}

// Serves for the product environment and fetch stats metrics by HTTP.
func newPrometheusMetricsExporter() (func(ctx context.Context) error, error) {
	exporter, err := prometheus.New()
	if err != nil {
		return nil, err
	}
	mp := metric.NewMeterProvider(metric.WithReader(exporter))
	callback := mp.Shutdown
	otel.SetMeterProvider(mp)
	return callback, nil
}
