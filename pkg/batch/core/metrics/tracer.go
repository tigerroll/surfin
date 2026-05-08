package metrics

// Package metrics defines interfaces for distributed tracing within the batch processing system.
// It provides an abstraction layer to integrate with various tracing backends.

import (
	"context"

	model "github.com/tigerroll/surfin/pkg/batch/core/domain/model"
)

// Tracer is an abstract interface for distributed tracing.
// This interface provides functionality to integrate with tracing systems like OpenTelemetry,
// enabling visualization of job and step execution flows.
type Tracer interface {
	// StartJobSpan starts a new tracing span for a JobExecution.
	//
	// ctx: The parent context.
	// execution: The JobExecution instance to be traced.
	//
	// Returns: A context with the new Span set, and a function to end the Span.
	//          It is recommended to call the returned function in a defer statement.
	StartJobSpan(ctx context.Context, execution *model.JobExecution) (context.Context, func())

	// StartStepSpan starts a new tracing span for a StepExecution.
	//
	// ctx: The parent context (typically a context with a JobSpan).
	// execution: The StepExecution to be traced.
	//
	// Returns: A context with the new Span set, and a function to end the Span.
	//          It is recommended to call the returned function in a defer statement.
	StartStepSpan(ctx context.Context, execution *model.StepExecution) (context.Context, func())

	// RecordError records an error as an event in the current active span.
	//
	// ctx: The context with the current Span.
	// module: The name of the module or component where the error occurred (e.g., "reader", "processor").
	// err: The error to record.
	RecordError(ctx context.Context, module string, err error)

	// RecordEvent records an event in the current Span.
	// This can be used to mark significant points in the execution flow.
	// ctx: The context with the current Span.
	// name: The name of the event (e.g., "item_read", "db_query").
	// attributes: Additional attributes to associate with the event.
	//             Example: `map[string]interface{}{"item_id": "123", "status": "success"}`
	RecordEvent(ctx context.Context, name string, attributes map[string]interface{})
}

// NoOpTracer is a no-op implementation of the Tracer interface.
// It is used when tracing is disabled or as a fallback.
type NoOpTracer struct{}

// NewNoOpTracer creates and returns a new instance of NoOpTracer.
func NewNoOpTracer() Tracer {
	return &NoOpTracer{}
}

// StartJobSpan is a no-op implementation that returns the original context and an empty function.
func (t *NoOpTracer) StartJobSpan(ctx context.Context, execution *model.JobExecution) (context.Context, func()) {
	return ctx, func() {}
}

// StartStepSpan is a no-op implementation that returns the original context and an empty function.
func (t *NoOpTracer) StartStepSpan(ctx context.Context, execution *model.StepExecution) (context.Context, func()) {
	return ctx, func() {}
}

// RecordError is a no-op implementation that does nothing.
func (t *NoOpTracer) RecordError(ctx context.Context, module string, err error) {}

// RecordEvent is a no-op implementation that does nothing.
func (t *NoOpTracer) RecordEvent(ctx context.Context, name string, attributes map[string]interface{}) {
}

var _ Tracer = (*NoOpTracer)(nil) // Compile-time check
