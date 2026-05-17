package tracing

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"

	model "github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	"github.com/tigerroll/surfin/pkg/batch/core/metrics"
	"github.com/tigerroll/surfin/pkg/batch/support/util/logger"
)

// OpenTelemetryTracer is an implementation for integrating with distributed tracing systems like OpenTelemetry.
type OpenTelemetryTracer struct {
	tracer trace.Tracer
}

// NewOpenTelemetryTracer creates a new instance of OpenTelemetryTracer.
func NewOpenTelemetryTracer() metrics.Tracer {
	logger.Infof("Tracing: Initializing OpenTelemetry Tracer.")
	return &OpenTelemetryTracer{
		tracer: otel.Tracer("surfin-batch"),
	}
}

// StartJobSpan starts a trace span for job execution.
func (t *OpenTelemetryTracer) StartJobSpan(ctx context.Context, execution *model.JobExecution) (context.Context, func()) {
	ctx, span := t.tracer.Start(ctx, "job:"+execution.JobName)
	return ctx, func() {
		span.End()
	}
}

// StartStepSpan starts a trace span for step execution.
func (t *OpenTelemetryTracer) StartStepSpan(ctx context.Context, execution *model.StepExecution) (context.Context, func()) {
	ctx, span := t.tracer.Start(ctx, "step:"+execution.StepName)
	return ctx, func() {
		span.End()
	}
}

// RecordError records an error in the current Span.
func (t *OpenTelemetryTracer) RecordError(ctx context.Context, module string, err error) {
	span := trace.SpanFromContext(ctx)
	span.RecordError(err)
	logger.Errorf("Tracing: Error recorded in %s: %v", module, err)
}

// RecordEvent records an event in the current Span.
func (t *OpenTelemetryTracer) RecordEvent(ctx context.Context, name string, attributes map[string]interface{}) {
	logger.Debugf("Tracing: Event recorded: %s, Attributes: %+v", name, attributes)
}

var _ metrics.Tracer = (*OpenTelemetryTracer)(nil)
