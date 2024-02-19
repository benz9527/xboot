package observability

// https://opentelemetry.io/docs/languages/go/exporters/

import (
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/sdk/metric"
)

func NewConsoleMetricsExporter(opts ...stdoutmetric.Option) (metric.Exporter, error) {
	return stdoutmetric.New(opts...)
}
