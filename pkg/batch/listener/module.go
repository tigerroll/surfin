package listener

import (
	"github.com/tigerroll/surfin/pkg/batch/listener/logging"
	"github.com/tigerroll/surfin/pkg/batch/listener/metrics"
	"github.com/tigerroll/surfin/pkg/batch/listener/tracing"
	"github.com/tigerroll/surfin/pkg/batch/listener/notification"

	"go.uber.org/fx"
)

// Module aggregates all listener modules of the batch framework.
var Module = fx.Options(
	logging.Module,
	metrics.Module,
	tracing.Module,
	notification.Module,
)
