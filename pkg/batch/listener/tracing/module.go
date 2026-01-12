package tracing

import (
	port "github.com/tigerroll/surfin/pkg/batch/core/application/port"
	config "github.com/tigerroll/surfin/pkg/batch/core/config"
	"github.com/tigerroll/surfin/pkg/batch/core/config/jsl"
	"github.com/tigerroll/surfin/pkg/batch/core/config/support"
	"github.com/tigerroll/surfin/pkg/batch/core/metrics"
	"github.com/tigerroll/surfin/pkg/batch/support/util/logger"
	"go.uber.org/fx"
)

// NewTracingJobListenerBuilder creates a ComponentBuilder for TracingJobListener.
func NewTracingJobListenerBuilder(tracer metrics.Tracer) jsl.JobExecutionListenerBuilder {
	return func(
		_ *config.Config,
		_ map[string]string,
	) (port.JobExecutionListener, error) {
		// Assumes NewTracingJobListener is defined in tracing_listeners.go
		return NewTracingJobListener(tracer), nil
	}
}

// NewTracingStepListenerBuilder creates a ComponentBuilder for TracingStepListener.
func NewTracingStepListenerBuilder(tracer metrics.Tracer) jsl.StepExecutionListenerBuilder {
	return func(
		_ *config.Config,
		_ map[string]string,
	) (port.StepExecutionListener, error) {
		// Assumes NewTracingStepListener is defined in tracing_listeners.go
		return NewTracingStepListener(tracer), nil
	}
}

// AllTracingListenerBuilders is a struct to receive all tracing listener builders from Fx.
type AllTracingListenerBuilders struct {
	fx.In
	JobListenerBuilder  jsl.JobExecutionListenerBuilder  `name:"tracingJobListener"`
	StepListenerBuilder jsl.StepExecutionListenerBuilder `name:"tracingStepListener"`
}

// RegisterAllTracingListeners registers all tracing listener builders with the JobFactory.
func RegisterAllTracingListeners(jf *support.JobFactory, builders AllTracingListenerBuilders) {
	jf.RegisterJobListenerBuilder("tracingJobListener", builders.JobListenerBuilder)
	jf.RegisterStepExecutionListenerBuilder("tracingStepListener", builders.StepListenerBuilder)
	logger.Debugf("All tracing listeners registered with JobFactory.")
}

// Module provides tracing-related components.
var Module = fx.Options(
	// 1. Providing a concrete implementation of Tracer is delegated to the infrastructure layer (pkg/batch/infrastructure/metrics/module.go).

	// 2. Provides listener builders.
	fx.Provide(fx.Annotate(NewTracingJobListenerBuilder, fx.ResultTags(`name:"tracingJobListener"`))),
	fx.Provide(fx.Annotate(NewTracingStepListenerBuilder, fx.ResultTags(`name:"tracingStepListener"`))),

	// 3. Registers listeners with JobFactory.
	fx.Invoke(RegisterAllTracingListeners),
)
