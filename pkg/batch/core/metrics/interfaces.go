package metrics

import (
	"context"

	"github.com/tigerroll/surfin/pkg/batch/core/domain/model"
)

// Tracer defines the interface for distributed tracing within the batch framework.
// Implementations of this interface are responsible for creating and managing
// tracing spans for various batch operations, and for recording errors and events.
type Tracer interface {
	// StartJobSpan starts a new tracing span for a JobExecution.
	// It returns a new context containing the created span and a function to end the span.
	StartJobSpan(ctx context.Context, execution *model.JobExecution) (context.Context, func())

	// StartStepSpan starts a new tracing span for a StepExecution.
	// It returns a new context containing the created span and a function to end the span.
	StartStepSpan(ctx context.Context, stepExecution *model.StepExecution) (context.Context, func())

	// RecordError records an error as an event in the current active span.
	RecordError(ctx context.Context, module string, err error)

	// RecordEvent records an event in the current active span.
	RecordEvent(ctx context.Context, name string, attributes map[string]interface{})
}

// NoOpTracer is a no-operation (no-op) implementation of the Tracer interface.
// It performs no actual tracing and is useful when tracing is disabled or not required.

// NoOpTracer is a no-operation (no-op) implementation of the Tracer interface.
// It performs no actual tracing and is useful when tracing is disabled or not required.
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
func (t *NoOpTracer) StartStepSpan(ctx context.Context, stepExecution *model.StepExecution) (context.Context, func()) {
	return ctx, func() {}
}

// RecordError is a no-op implementation that does nothing.
func (t *NoOpTracer) RecordError(ctx context.Context, module string, err error) {}

// RecordEvent is a no-op implementation that does nothing.
func (t *NoOpTracer) RecordEvent(ctx context.Context, name string, attributes map[string]interface{}) {
}

var _ Tracer = (*NoOpTracer)(nil) // Compile-time check to ensure NoOpTracer implements Tracer
