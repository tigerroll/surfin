package metrics

import (
	"go.uber.org/fx"
)

// Module is an Fx module that provides metrics-related components.
var Module = fx.Options(
	// Provides Tracer abstraction (fallback)
	fx.Provide(fx.Annotate(
		NewNoOpTracer,
		fx.As(new(Tracer)),
		fx.ResultTags(`optional:"true"`),
	)),
)
