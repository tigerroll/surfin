package trace

import (
	"context"
	"fmt"

	"github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	"github.com/tigerroll/surfin/pkg/batch/core/metrics"
	"github.com/tigerroll/surfin/pkg/batch/support/util/exception"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	api_trace "go.opentelemetry.io/otel/trace" // Alias for OpenTelemetry trace API.
)

// OtelTracer implements the metrics.Tracer interface using OpenTelemetry.
// It provides methods to create and manage tracing spans for job and step executions,
// and to record errors and events within these spans.
type OtelTracer struct {
	tracer api_trace.Tracer
}

// NewOtelTracer creates a new OtelTracer instance.
// It takes an OpenTelemetry Tracer as input.
//
// Parameters:
//
//	tracer: An OpenTelemetry Tracer instance. If nil, an error is returned.
//
// Returns:
//
//	metrics.Tracer: An implementation of the Tracer interface.
//	error: An error if the provided tracer is nil.
func NewOtelTracer(tracer api_trace.Tracer) (metrics.Tracer, error) {
	if tracer == nil {
		// For explicit nil, it's an error.
		return nil, exception.NewBatchError(moduleName, "OpenTelemetry Tracer cannot be nil", nil, false, false)
	}
	return &OtelTracer{tracer: tracer}, nil
}

// StartJobSpan starts a new tracing span for a JobExecution.
// It sets relevant attributes on the span, such as job ID and job name.
//
// Parameters:
//
//	ctx: The parent context.
//	execution: The JobExecution instance for which to start the span.
//
// Returns:
//
//	context.Context: A new context containing the created span.
//	func(): A function to call to end the span. It is recommended to call this function
//	        in a defer statement immediately after the call to StartJobSpan.
func (t *OtelTracer) StartJobSpan(ctx context.Context, execution *model.JobExecution) (context.Context, func()) {
	if t.tracer == nil { // Guard against nil tracer if it somehow becomes nil after initialization
		return ctx, func() {}
	}

	spanName := fmt.Sprintf("JobExecution/%s", execution.JobName)
	ctx, span := t.tracer.Start(ctx, spanName,
		api_trace.WithSpanKind(api_trace.SpanKindInternal),
		api_trace.WithAttributes(
			attribute.String("job.id", execution.ID),
			attribute.String("job.name", execution.JobName),
			attribute.String("job.status", string(execution.Status)),
		),
	)

	// Wrap span.End() in an anonymous function to match the func() signature.
	return ctx, func() {
		span.End()
	}
}

// StartStepSpan starts a new tracing span for a StepExecution.
// It sets relevant attributes on the span, such as step name and job execution ID.
//
// Parameters:
//
//	ctx: The parent context.
//	stepExecution: The StepExecution instance for which to start the span.
//
// Returns:
//
//	context.Context: A new context containing the created span.
//	func(): A function to call to end the span. It is recommended to call this function
//	        in a defer statement immediately after the call to StartStepSpan.
func (t *OtelTracer) StartStepSpan(ctx context.Context, stepExecution *model.StepExecution) (context.Context, func()) {
	if t.tracer == nil { // Guard against nil tracer
		return ctx, func() {}
	}

	spanName := fmt.Sprintf("StepExecution/%s", stepExecution.StepName)
	ctx, span := t.tracer.Start(ctx, spanName,
		api_trace.WithSpanKind(api_trace.SpanKindInternal),
		api_trace.WithAttributes(
			attribute.String("step.id", stepExecution.ID),
			attribute.String("step.name", stepExecution.StepName),
			attribute.String("job.execution.id", stepExecution.JobExecutionID),
			attribute.String("step.status", string(stepExecution.Status)),
		),
	)

	// Wrap span.End() in an anonymous function to match the func() signature.
	return ctx, func() {
		span.End()
	}
}

// RecordError records an error as an event in the current active span.
//
// Parameters:
//
//	ctx: The context containing the current span.
//	module: The name of the module or component where the error occurred (e.g., "reader", "processor").
//	err: The error to record. If nil, no action is taken.
func (t *OtelTracer) RecordError(ctx context.Context, module string, err error) {
	if t.tracer == nil || err == nil { // Guard against nil tracer or nil error
		return
	}
	span := api_trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.RecordError(err, api_trace.WithAttributes(attribute.String("error.module", module)))
		span.SetStatus(codes.Error, err.Error())
	}
}

// RecordEvent records an event in the current active span.
// This can be used to mark significant points in the execution flow.
//
// Parameters:
//
//	  ctx: The context containing the current span.
//	  name: The name of the event (e.g., "item_read", "db_query").
//	  attributes: Additional attributes to associate with the event.
//
//		Example: `map[string]interface{}{"item_id": "123", "status": "success"}`
func (t *OtelTracer) RecordEvent(ctx context.Context, name string, attributes map[string]interface{}) {
	if t.tracer == nil { // Guard against nil tracer
		return
	}
	span := api_trace.SpanFromContext(ctx)
	if span.IsRecording() {
		otelAttrs := make([]attribute.KeyValue, 0, len(attributes))
		for k, v := range attributes {
			// Convert interface{} to attribute.KeyValue. This is a simplified conversion.
			// For production, you might need more robust type handling.
			switch val := v.(type) {
			case string:
				otelAttrs = append(otelAttrs, attribute.String(k, val))
			case int:
				otelAttrs = append(otelAttrs, attribute.Int(k, val))
			case int64:
				otelAttrs = append(otelAttrs, attribute.Int64(k, val))
			case bool:
				otelAttrs = append(otelAttrs, attribute.Bool(k, val))
			case float64:
				otelAttrs = append(otelAttrs, attribute.Float64(k, val))
			default:
				// Fallback for unsupported types, convert to string
				otelAttrs = append(otelAttrs, attribute.String(k, fmt.Sprintf("%v", val)))
			}
		}
		span.AddEvent(name, api_trace.WithAttributes(otelAttrs...))
	}
}

// NewNoOpTracer returns a no-operation (no-op) implementation of metrics.Tracer.
// This tracer performs no actual tracing and is useful when tracing is disabled
// or not required.
func NewNoOpTracer() metrics.Tracer {
	// Return the NoOpTracer from the core metrics package.
	return metrics.NewNoOpTracer()
}

// Compile-time check to ensure OtelTracer implements metrics.Tracer
var _ metrics.Tracer = (*OtelTracer)(nil)
