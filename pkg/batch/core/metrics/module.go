package metrics

import (
	"go.uber.org/fx"
)

// Module is an Fx module that provides metrics-related components.
// Note: metrics.Tracer is now provided by the observability module.
var Module = fx.Options()
