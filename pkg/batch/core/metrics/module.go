package metrics

import (
	"go.uber.org/fx"
)

// Module is an Fx module that provides metrics-related components.
var Module = fx.Options(
	// By default, it provides NoOpMetricRecorder.
	// Actual metric implementations (e.g., PrometheusRecorder) are prioritized if provided by the infrastructure layer.
	// NoOpMetricRecorder remains as a fallback.
	fx.Provide(fx.Annotate(
		NewNoOpMetricRecorder,
		fx.As(new(MetricRecorder)),
		fx.ResultTags(`optional:"true"`),
	)),
	// Provides Tracer abstraction (fallback)
	fx.Provide(fx.Annotate(
		NewNoOpTracer,
		fx.As(new(Tracer)),
		fx.ResultTags(`optional:"true"`),
	)),
)
