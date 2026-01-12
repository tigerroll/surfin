package metrics

import (
	"context"
	"time"

	model "surfin/pkg/batch/core/domain/model"
)

// Span represents a single operation or unit of work in distributed tracing.
// This interface provides basic methods for managing the lifecycle of a span.
type Span interface {
	// End sets the end time of the current span and finishes the span.
	// Once a span is ended, its data is ready to be exported to the tracing system.
	End()
}

// MetricRecorder is an abstract interface for recording metrics related to batch execution.
// Issue I.1: Abstraction of metric collection
//
// This interface provides a standardized way to record metrics for job, step, item-level events,
// and chunk processing.
// This facilitates integration with different metrics backends (e.g., Prometheus, OpenTelemetry Metrics).
type MetricRecorder interface {
	// RecordJobStart records the start of a JobExecution.
	//
	// ctx: The context for the operation.
	// execution: Details of the started JobExecution.
	RecordJobStart(ctx context.Context, execution *model.JobExecution)

	// RecordJobEnd records the end of a JobExecution.
	//
	// ctx: The context for the operation.
	// execution: Details of the ended JobExecution.
	RecordJobEnd(ctx context.Context, execution *model.JobExecution)

	// RecordStepStart records the start of a StepExecution.
	//
	// ctx: The context for the operation.
	// execution: Details of the started StepExecution.
	RecordStepStart(ctx context.Context, execution *model.StepExecution)

	// RecordStepEnd records the end of a StepExecution.
	//
	// ctx: The context for the operation.
	// execution: Details of the ended StepExecution.
	RecordStepEnd(ctx context.Context, execution *model.StepExecution)

	// RecordItemRead records the successful reading of an item.
	//
	// ctx: The context for the operation.
	// stepName: The name of the step where the item was read.
	RecordItemRead(ctx context.Context, stepName string)

	// RecordItemProcess records the successful processing of an item.
	//
	// ctx: The context for the operation.
	// stepName: The name of the step where the item was processed.
	RecordItemProcess(ctx context.Context, stepName string)

	// RecordItemWrite records the successful writing of items.
	//
	// ctx: The context for the operation.
	// stepName: The name of the step where the items were written.
	// count: The number of items written.
	RecordItemWrite(ctx context.Context, stepName string, count int)

	// RecordItemSkip records the skipping of an item.
	//
	// ctx: The context for the operation.
	// stepName: The name of the step where the item was skipped.
	// reason: A string indicating the reason for skipping (e.g., error type).
	RecordItemSkip(ctx context.Context, stepName string, reason string)

	// RecordItemRetry records the retry of an item.
	//
	// ctx: The context for the operation.
	// stepName: The name of the step where the item was retried.
	// reason: A string indicating the reason for the retry (e.g., error type).
	RecordItemRetry(ctx context.Context, stepName string, reason string)

	// RecordChunkCommit records the commitment of a chunk.
	//
	// ctx: The context for the operation.
	// stepName: The name of the step where the chunk was committed.
	// count: The number of items committed.
	RecordChunkCommit(ctx context.Context, stepName string, count int)

	// RecordDuration records the execution time of a specific operation.
	//
	// ctx: The context for the operation.
	// name: The name of the duration to record (e.g., "api_call_duration", "db_query_time").
	// duration: The length of the duration to record.
	// tags: A map of additional tags or attributes to associate with the duration.
	//       Example: `{"api_name": "OpenMeteo", "status": "success"}`
	RecordDuration(ctx context.Context, name string, duration time.Duration, tags map[string]string)
}
