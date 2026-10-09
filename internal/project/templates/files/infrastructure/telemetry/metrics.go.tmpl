package telemetry

import (
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/prometheus"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

// NewMeter initialises a Prometheus-backed MeterProvider and sets it as the
// global provider. The returned provider must be shut down on application exit.
func NewMeter(cfg Config) (*sdkmetric.MeterProvider, error) {
	if !cfg.Enabled {
		mp := sdkmetric.NewMeterProvider()
		otel.SetMeterProvider(mp)
		return mp, nil
	}

	exporter, err := prometheus.New()
	if err != nil {
		return nil, fmt.Errorf("telemetry: create prometheus exporter: %w", err)
	}

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(exporter),
	)
	otel.SetMeterProvider(mp)

	return mp, nil
}
