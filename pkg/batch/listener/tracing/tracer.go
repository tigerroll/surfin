package tracing

import (
	"context"

	model "surfin/pkg/batch/core/domain/model"
	"surfin/pkg/batch/core/metrics"
	"surfin/pkg/batch/support/util/logger"
)

// OpenTelemetryTracer is an implementation for integrating with distributed tracing systems like OpenTelemetry.
// Currently, it only performs logging.
type OpenTelemetryTracer struct{}

// NewOpenTelemetryTracer creates a new instance of OpenTelemetryTracer.
func NewOpenTelemetryTracer() metrics.Tracer {
	logger.Infof("Tracing: Initializing OpenTelemetry Tracer.")
	return &OpenTelemetryTracer{}
}

// StartJobSpan starts a trace span for job execution.
func (t *OpenTelemetryTracer) StartJobSpan(ctx context.Context, execution *model.JobExecution) (context.Context, func()) {
	logger.Debugf("Tracing: Starting Job Span for %s (ID: %s)", execution.JobName, execution.ID)
	// In a real implementation, an OpenTelemetry Span would be started here and stored in a new Context.
	return ctx, func() {
		logger.Debugf("Tracing: Ending Job Span for %s (Status: %s)", execution.JobName, execution.Status)
	}
}

// StartStepSpan starts a trace span for step execution.
func (t *OpenTelemetryTracer) StartStepSpan(ctx context.Context, execution *model.StepExecution) (context.Context, func()) {
	logger.Debugf("Tracing: Starting Step Span for %s (ID: %s)", execution.StepName, execution.ID)
	// In a real implementation, an OpenTelemetry Span would be started here and stored in a new Context.
	return ctx, func() {
		logger.Debugf("Tracing: Ending Step Span for %s (Status: %s)", execution.StepName, execution.Status)
	}
}

// RecordError records an error in the current Span.
func (t *OpenTelemetryTracer) RecordError(ctx context.Context, module string, err error) {
	logger.Errorf("Tracing: Error recorded in %s: %v", module, err)
}

// RecordEvent records an event in the current Span.
func (t *OpenTelemetryTracer) RecordEvent(ctx context.Context, name string, attributes map[string]interface{}) {
	logger.Debugf("Tracing: Event recorded: %s, Attributes: %+v", name, attributes)
}

var _ metrics.Tracer = (*OpenTelemetryTracer)(nil)
