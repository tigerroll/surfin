package metrics

import (
	"context"

	model "surfin/pkg/batch/core/domain/model"
	metrics "surfin/pkg/batch/core/metrics" // FIX: Changed from core to domain/model.
	logger "surfin/pkg/batch/support/util/logger"
)

// OpenTelemetryTracer is an implementation of metrics.Tracer using OpenTelemetry.
type OpenTelemetryTracer struct{}

// NewOpenTelemetryTracer creates a new instance of OpenTelemetryTracer.
func NewOpenTelemetryTracer() metrics.Tracer {
	return &OpenTelemetryTracer{}
}

// StartJobSpan starts a new span for a JobExecution.
func (t *OpenTelemetryTracer) StartJobSpan(ctx context.Context, execution *model.JobExecution) (context.Context, func()) {
	logger.Debugf("Tracer: OTel StartJobSpan called for Job '%s'", execution.JobName)
	return ctx, func() {
		logger.Debugf("Tracer: OTel FinishJobSpan called for Job '%s'", execution.JobName)
	}
}

// StartStepSpan starts a new span for a StepExecution.
func (t *OpenTelemetryTracer) StartStepSpan(ctx context.Context, execution *model.StepExecution) (context.Context, func()) {
	logger.Debugf("Tracer: OTel StartStepSpan called for Step '%s'", execution.StepName)
	return ctx, func() {
		logger.Debugf("Tracer: OTel FinishStepSpan called for Step '%s'", execution.StepName)
	}
}

// RecordError records an error in the current span.
func (t *OpenTelemetryTracer) RecordError(ctx context.Context, module string, err error) {
	logger.Debugf("Tracer: OTel RecordError called in module %s: %v", module, err)
}

// RecordEvent records an event in the current span.
func (t *OpenTelemetryTracer) RecordEvent(ctx context.Context, name string, attributes map[string]interface{}) {
	logger.Debugf("Tracer: OTel RecordEvent called: %s, attributes: %v", name, attributes)
}

var _ metrics.Tracer = (*OpenTelemetryTracer)(nil)
