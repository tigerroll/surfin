package metrics

import (
	"go.uber.org/fx"
	metrics "surfin/pkg/batch/core/metrics"
)

// Module is an Fx module that provides PrometheusRecorder and OpenTelemetryTracer.
var Module = fx.Options(
	// Provide PrometheusRecorder as a core.MetricRecorder interface.
	fx.Provide(fx.Annotate(
		NewPrometheusRecorder,
		fx.As(new(metrics.MetricRecorder)),
	)),
	// Provide OpenTelemetryTracer as a core.Tracer interface.
	fx.Provide(fx.Annotate(
		NewOpenTelemetryTracer,
		fx.As(new(metrics.Tracer)),
	)),
)
