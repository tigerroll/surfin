package listener

import (
	"github.com/tigerroll/surfin/pkg/batch/listener/logging"
	"github.com/tigerroll/surfin/pkg/batch/listener/metrics"
	"github.com/tigerroll/surfin/pkg/batch/listener/notification"
	"github.com/tigerroll/surfin/pkg/batch/listener/tracing"

	"go.uber.org/fx"

	port "github.com/tigerroll/surfin/pkg/batch/core/application/port"
	config "github.com/tigerroll/surfin/pkg/batch/core/config"
	jsl "github.com/tigerroll/surfin/pkg/batch/core/config/jsl"
	support "github.com/tigerroll/surfin/pkg/batch/core/config/support"
	"github.com/tigerroll/surfin/pkg/batch/support/util/logger"
)

// Module aggregates all listener modules of the batch framework.
// It provides and registers various job and step execution listeners,
// including logging, metrics, tracing, and notification components.
var Module = fx.Options(
	logging.Module,
	metrics.Module,
	tracing.Module,
	notification.Module,
	// Provide and register JobCompletionSignaler
	fx.Provide(fx.Annotate(
		NewJobCompletionSignalerBuilder,
		fx.ResultTags(`name:"jobCompletionSignalerBuilder"`),
	)),
	fx.Invoke(fx.Annotate(
		RegisterJobCompletionSignalerBuilder,
		fx.ParamTags(``, `name:"jobCompletionSignalerBuilder"`),
	)),
)

// NewJobCompletionSignalerBuilder provides a builder for JobCompletionSignaler.
// This builder receives the JobDoneChan from Fx, which is used to signal job completion.
//
// Parameters:
//   jobDoneChan: A channel of type `chan struct{}` that is closed upon job completion.
//
// Returns:
//   A `jsl.JobExecutionListenerBuilder` function that can create a `JobCompletionSignaler` instance.
func NewJobCompletionSignalerBuilder(jobDoneChan chan struct{}) jsl.JobExecutionListenerBuilder {
	return func(cfg *config.Config, properties map[string]string) (port.JobExecutionListener, error) {
		return NewJobCompletionSignaler(jobDoneChan), nil
	}
}

// RegisterJobCompletionSignalerBuilder registers the JobCompletionSignaler builder with the JobFactory.
// This allows the JobFactory to instantiate `JobCompletionSignaler` when referenced in JSL.
//
// Parameters:
//   jf: The `support.JobFactory` instance to register the builder with.
//   builder: The `jsl.JobExecutionListenerBuilder` for `JobCompletionSignaler`.
func RegisterJobCompletionSignalerBuilder(
	jf *support.JobFactory,
	builder jsl.JobExecutionListenerBuilder,
) {
	// "jobCompletionSignaler" is the key used in JSL to reference this listener.
	jf.RegisterJobListenerBuilder("jobCompletionSignaler", builder)
	logger.Debugf("Builder for JobCompletionSignaler registered with JobFactory. JSL ref: 'jobCompletionSignaler'")
}
