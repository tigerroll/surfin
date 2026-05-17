package metrics

import (
	"context"

	"go.opentelemetry.io/otel/attribute"

	model "github.com/tigerroll/surfin/pkg/batch/core/domain/model"
)

// Package metrics provides interfaces and no-operation implementations for recording batch job metrics.
// It defines how metric data related to job and step executions should be collected.
type MetricRecorder interface {
	// RecordJobStart records the start of a JobExecution.
	//
	// Parameters:
	//   ctx: The context for the operation.
	//   execution: The JobExecution instance that has started.
	RecordJobStart(ctx context.Context, execution *model.JobExecution)

	// RecordJobEnd records the end of a JobExecution.
	//
	// Parameters:
	//   ctx: The context for the operation.
	//   execution: The JobExecution instance that has ended.
	RecordJobEnd(ctx context.Context, execution *model.JobExecution)

	// RecordStepStart records the start of a StepExecution.
	//
	// Parameters:
	//   ctx: The context for the operation.
	//   execution: The StepExecution instance that has started.
	RecordStepStart(ctx context.Context, execution *model.StepExecution)

	// RecordStepEnd records the end of a StepExecution.
	//
	// Parameters:
	//   ctx: The context for the operation.
	//   execution: The StepExecution instance that has ended.
	RecordStepEnd(ctx context.Context, execution *model.StepExecution)

	// RecordItemRead records the successful reading of an item.
	//
	// Parameters:
	//   ctx: The context for the operation.
	//   stepExecution: The StepExecution instance where the item was read.
	//   count: The number of items read.
	RecordItemRead(ctx context.Context, stepExecution *model.StepExecution, count int64)

	// RecordItemProcess records the successful processing of an item.
	//
	// Parameters:
	//   ctx: The context for the operation.
	//   stepExecution: The StepExecution instance where the item was processed.
	//   count: The number of items processed.
	RecordItemProcess(ctx context.Context, stepExecution *model.StepExecution, count int64)

	// RecordItemWrite records the successful writing of items.
	//
	// Parameters:
	//   ctx: The context for the operation.
	//   stepExecution: The StepExecution instance where the items were written.
	//   count: The number of items written.
	RecordItemWrite(ctx context.Context, stepExecution *model.StepExecution, count int64)

	// RecordItemSkip records the skipping of an item.
	//
	// Parameters:
	//   ctx: The context for the operation.
	//   stepExecution: The StepExecution instance where the item was skipped.
	//   err: The error that caused the item to be skipped.
	RecordItemSkip(ctx context.Context, stepExecution *model.StepExecution, err error)

	// RecordItemRetry records the retry of an item.
	//
	// Parameters:
	//   ctx: The context for the operation.
	//   stepExecution: The StepExecution instance where the item was retried.
	//   err: The error that caused the item to be retried.
	RecordItemRetry(ctx context.Context, stepExecution *model.StepExecution, err error)

	// RecordChunkCommit records the commitment of a chunk.
	//
	// Parameters:
	//   ctx: The context for the operation.
	//   stepExecution: The StepExecution instance where the chunk was committed.
	//   count: The number of items committed.
	RecordChunkCommit(ctx context.Context, stepExecution *model.StepExecution, count int64)

	// RecordDuration records the execution time of a specific operation.
	//
	// Parameters:
	//   ctx: The context for the operation.
	//   name: The name of the duration metric (e.g., "api_call_duration", "db_query_time").
	//   duration: The duration value to record, typically in seconds.
	//   attrs: Optional OpenTelemetry attributes to associate with the duration.
	//          Example: `attribute.String("api_name", "OpenMeteo"), attribute.String("status", "success")`
	RecordDuration(ctx context.Context, name string, duration float64, attrs ...attribute.KeyValue)

	// RecordExecutionError records a general execution error.
	//
	// Parameters:
	//   ctx: The context for the operation.
	//   err: The error that occurred.
	RecordExecutionError(ctx context.Context, err error)
}

// NoOpMetricRecorder is a no-operation implementation of MetricRecorder.
// It performs no action when its methods are called, effectively disabling metric recording.
type NoOpMetricRecorder struct{}

// NewNoOpMetricRecorder creates a new instance of NoOpMetricRecorder.
//
// Returns:
//
//	A new instance of MetricRecorder that performs no operations.
func NewNoOpMetricRecorder() MetricRecorder {
	return &NoOpMetricRecorder{}
}

// RecordJobStart implements the MetricRecorder interface for NoOpMetricRecorder.
// It performs no operation.
func (r *NoOpMetricRecorder) RecordJobStart(ctx context.Context, execution *model.JobExecution) {}

// RecordJobEnd implements the MetricRecorder interface for NoOpMetricRecorder.
// It performs no operation.
func (r *NoOpMetricRecorder) RecordJobEnd(ctx context.Context, execution *model.JobExecution) {}

// RecordStepStart implements the MetricRecorder interface for NoOpMetricRecorder.
// It performs no operation.
func (r *NoOpMetricRecorder) RecordStepStart(ctx context.Context, execution *model.StepExecution) {}

// RecordStepEnd implements the MetricRecorder interface for NoOpMetricRecorder.
// It performs no operation.
func (r *NoOpMetricRecorder) RecordStepEnd(ctx context.Context, execution *model.StepExecution) {}

// RecordItemRead implements the MetricRecorder interface for NoOpMetricRecorder.
// It performs no operation.
func (r *NoOpMetricRecorder) RecordItemRead(ctx context.Context, stepExecution *model.StepExecution, count int64) {
}

// RecordItemProcess implements the MetricRecorder interface for NoOpMetricRecorder.
// It performs no operation.
func (r *NoOpMetricRecorder) RecordItemProcess(ctx context.Context, stepExecution *model.StepExecution, count int64) {
}

// RecordItemWrite implements the MetricRecorder interface for NoOpMetricRecorder.
// It performs no operation.
func (r *NoOpMetricRecorder) RecordItemWrite(ctx context.Context, stepExecution *model.StepExecution, count int64) {
}

// RecordItemSkip implements the MetricRecorder interface for NoOpMetricRecorder.
// It performs no operation.
func (r *NoOpMetricRecorder) RecordItemSkip(ctx context.Context, stepExecution *model.StepExecution, err error) {
}

// RecordItemRetry implements the MetricRecorder interface for NoOpMetricRecorder.
// It performs no operation.
func (r *NoOpMetricRecorder) RecordItemRetry(ctx context.Context, stepExecution *model.StepExecution, err error) {
}

// RecordChunkCommit implements the MetricRecorder interface for NoOpMetricRecorder.
// It performs no operation.
func (r *NoOpMetricRecorder) RecordChunkCommit(ctx context.Context, stepExecution *model.StepExecution, count int64) {
}

// RecordDuration implements the MetricRecorder interface for NoOpMetricRecorder.
// It performs no operation.
func (r *NoOpMetricRecorder) RecordDuration(ctx context.Context, name string, duration float64, attrs ...attribute.KeyValue) {
}

// RecordExecutionError implements the MetricRecorder interface for NoOpMetricRecorder.
// It performs no operation.
func (r *NoOpMetricRecorder) RecordExecutionError(ctx context.Context, err error) {}

var _ MetricRecorder = (*NoOpMetricRecorder)(nil) // Compile-time check to ensure NoOpMetricRecorder implements MetricRecorder.
