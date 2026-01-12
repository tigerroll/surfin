package listener

import (
	"surfin/pkg/batch/listener/logging"
	"surfin/pkg/batch/listener/metrics"
	"surfin/pkg/batch/listener/tracing"
	"surfin/pkg/batch/listener/notification"

	"go.uber.org/fx"
)

// Module aggregates all listener modules of the batch framework.
var Module = fx.Options(
	logging.Module,
	metrics.Module,
	tracing.Module,
	notification.Module,
)
