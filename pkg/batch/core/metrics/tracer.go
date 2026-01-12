package metrics

import (
	"context"

	model "surfin/pkg/batch/core/domain/model"
)

// Tracer is an abstract interface for distributed tracing.
// This interface provides functionality to integrate with tracing systems like OpenTelemetry,
// enabling visualization of job and step execution flows.
type Tracer interface {
	// StartJobSpan starts a Span for a JobExecution.
	//
	// ctx: The parent context.
	// execution: The JobExecution to be traced.
	//
	// Returns: A context with the new Span set, and a function to end the Span.
	//          It is recommended to call the returned function in a defer statement.
	StartJobSpan(ctx context.Context, execution *model.JobExecution) (context.Context, func())

	// StartStepSpan starts a Span for a StepExecution.
	//
	// ctx: The parent context (typically a context with a JobSpan).
	// execution: The StepExecution to be traced.
	//
	// Returns: A context with the new Span set, and a function to end the Span.
	//          It is recommended to call the returned function in a defer statement.
	StartStepSpan(ctx context.Context, execution *model.StepExecution) (context.Context, func())

	// RecordError records an error in the current Span.
	//
	// ctx: The context with the current Span.
	// module: The name of the module or component where the error occurred (e.g., "reader", "processor").
	// err: The error to record.
	RecordError(ctx context.Context, module string, err error)

	// RecordEvent records an event in the current Span.
	//
	// ctx: The context with the current Span.
	// name: The name of the event (e.g., "item_read", "db_query").
	// attributes: Additional attributes to associate with the event.
	//             Example: `map[string]interface{}{"item_id": "123", "status": "success"}`
	RecordEvent(ctx context.Context, name string, attributes map[string]interface{})
}
