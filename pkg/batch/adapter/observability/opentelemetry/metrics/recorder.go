package metrics

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	"github.com/tigerroll/surfin/pkg/batch/core/metrics"
	"github.com/tigerroll/surfin/pkg/batch/support/util/exception"
)

// OtelMetricRecorder implements metrics.MetricRecorder using OpenTelemetry.
type OtelMetricRecorder struct {
	meter metric.Meter

	// Counters
	jobStartCounter    metric.Int64Counter
	jobEndCounter      metric.Int64Counter
	stepStartCounter   metric.Int64Counter
	stepEndCounter     metric.Int64Counter
	itemReadCounter    metric.Int64Counter
	itemProcessCounter metric.Int64Counter
	itemWriteCounter   metric.Int64Counter
	itemSkipCounter    metric.Int64Counter
	itemRetryCounter   metric.Int64Counter
	chunkCommitCounter metric.Int64Counter

	// Histograms
	durationHistogram metric.Float64Histogram
}

// NewOtelMetricRecorder creates a new instance of OtelMetricRecorder.
// It initializes all necessary OpenTelemetry metric instruments (counters and histograms)
// using the provided `metric.Meter`.
//
// Parameters:
//
//	meter: The OpenTelemetry `metric.Meter` used to create instruments.
//
// Returns:
//
//	metrics.MetricRecorder: An initialized `OtelMetricRecorder` instance.
//	error: An error if any metric instrument fails to be created.
func NewOtelMetricRecorder(meter metric.Meter) (metrics.MetricRecorder, error) {

	var err error
	r := &OtelMetricRecorder{
		meter: meter,
	}

	// Initialize Counters
	r.jobStartCounter, err = meter.Int64Counter(
		"surfin.batch.job.start.total",
		metric.WithDescription("Total number of job starts"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create jobStartCounter: %w", err)
	}
	r.jobEndCounter, err = meter.Int64Counter(
		"surfin.batch.job.end.total",
		metric.WithDescription("Total number of job ends"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create jobEndCounter: %w", err)
	}
	r.stepStartCounter, err = meter.Int64Counter(
		"surfin.batch.step.start.total",
		metric.WithDescription("Total number of step starts"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create stepStartCounter: %w", err)
	}
	r.stepEndCounter, err = meter.Int64Counter(
		"surfin.batch.step.end.total",
		metric.WithDescription("Total number of step ends"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create stepEndCounter: %w", err)
	}
	r.itemReadCounter, err = meter.Int64Counter(
		"surfin.batch.item.read.total",
		metric.WithDescription("Total number of items read"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create itemReadCounter: %w", err)
	}
	r.itemProcessCounter, err = meter.Int64Counter(
		"surfin.batch.item.process.total",
		metric.WithDescription("Total number of items processed"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create itemProcessCounter: %w", err)
	}
	r.itemWriteCounter, err = meter.Int64Counter(
		"surfin.batch.item.write.total",
		metric.WithDescription("Total number of items written"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create itemWriteCounter: %w", err)
	}
	r.itemSkipCounter, err = meter.Int64Counter(
		"surfin.batch.item.skip.total",
		metric.WithDescription("Total number of items skipped"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create itemSkipCounter: %w", err)
	}
	r.itemRetryCounter, err = meter.Int64Counter(
		"surfin.batch.item.retry.total",
		metric.WithDescription("Total number of items retried"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create itemRetryCounter: %w", err)
	}
	r.chunkCommitCounter, err = meter.Int64Counter(
		"surfin.batch.chunk.commit.total",
		metric.WithDescription("Total number of chunks committed"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create chunkCommitCounter: %w", err)
	}

	// Initialize Histograms
	r.durationHistogram, err = meter.Float64Histogram(
		"surfin.batch.duration.seconds",
		metric.WithDescription("Duration of various batch operations in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create durationHistogram: %w", err)
	}

	return r, nil
}

// RecordJobStart records the initiation of a JobExecution.
// It increments a counter for job starts and attaches relevant job attributes.
//
// Parameters:
//
//	ctx: The context for the operation.
//	jobExecution: The JobExecution instance that has started.
func (r *OtelMetricRecorder) RecordJobStart(ctx context.Context, jobExecution *model.JobExecution) {
	attrs := []attribute.KeyValue{
		attribute.String("job.name", jobExecution.JobName),
		attribute.String("job.execution_id", jobExecution.ID), // Use JobExecutionID instead of JobInstanceID.
	}
	r.jobStartCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// RecordJobEnd records the completion (success or failure) of a JobExecution.
// It increments a counter for job ends and records the job's total duration.
//
// Parameters:
//
//	ctx: The context for the operation.
//	jobExecution: The JobExecution instance that has completed.
func (r *OtelMetricRecorder) RecordJobEnd(ctx context.Context, jobExecution *model.JobExecution) {
	attrs := []attribute.KeyValue{
		attribute.String("job.name", jobExecution.JobName),
		attribute.String("job.execution_id", jobExecution.ID), // Use JobExecutionID instead of JobInstanceID.
		attribute.String("job.status", string(jobExecution.Status)),
		attribute.String("job.exit_status", string(jobExecution.ExitStatus)), // Add ExitStatus attribute.
	}
	r.jobEndCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
	// Record job duration if EndTime is available and StartTime is not zero.
	if jobExecution.EndTime != nil && !jobExecution.StartTime.IsZero() {
		r.RecordDuration(ctx, "job.duration", jobExecution.EndTime.Sub(jobExecution.StartTime).Seconds(), attrs...)
	}
}

// RecordStepStart records the initiation of a StepExecution.
// It increments a counter for step starts and attaches relevant step attributes.
//
// Parameters:
//
//	ctx: The context for the operation.
//	stepExecution: The StepExecution instance that has started.
func (r *OtelMetricRecorder) RecordStepStart(ctx context.Context, stepExecution *model.StepExecution) {
	attrs := []attribute.KeyValue{
		attribute.String("job.name", stepExecution.JobExecution.JobName), // Retrieve JobName from JobExecution.
		attribute.String("job.execution_id", stepExecution.JobExecutionID),
		attribute.String("step.name", stepExecution.StepName),
		attribute.String("step.execution_id", stepExecution.ID), // ID is a string.
	}
	r.stepStartCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// RecordStepEnd records the completion (success or failure) of a StepExecution.
// It increments a counter for step ends and records the step's total duration.
//
// Parameters:
//
//	ctx: The context for the operation.
//	stepExecution: The StepExecution instance that has completed.
func (r *OtelMetricRecorder) RecordStepEnd(ctx context.Context, stepExecution *model.StepExecution) {
	attrs := []attribute.KeyValue{
		attribute.String("job.name", stepExecution.JobExecution.JobName), // Retrieve JobName from JobExecution.
		attribute.String("job.execution_id", stepExecution.JobExecutionID),
		attribute.String("step.name", stepExecution.StepName),
		attribute.String("step.execution_id", stepExecution.ID), // ID is a string.
		attribute.String("step.status", string(stepExecution.Status)),
		attribute.String("step.exit_status", string(stepExecution.ExitStatus)), // Add ExitStatus attribute.
	}
	r.stepEndCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
	// Record step duration if EndTime is available and StartTime is not zero.
	if stepExecution.EndTime != nil && !stepExecution.StartTime.IsZero() {
		r.RecordDuration(ctx, "step.duration", stepExecution.EndTime.Sub(stepExecution.StartTime).Seconds(), attrs...)
	}
}

// RecordItemRead records the number of items successfully read during a step.
// It increments a counter for items read and attaches relevant step attributes.
//
// Parameters:
//
//	ctx: The context for the operation.
//	stepExecution: The StepExecution instance where the items were read.
//	count: The number of items read.
func (r *OtelMetricRecorder) RecordItemRead(ctx context.Context, stepExecution *model.StepExecution, count int64) {
	attrs := []attribute.KeyValue{
		attribute.String("job.name", stepExecution.JobExecution.JobName),
		attribute.String("job.execution_id", stepExecution.JobExecutionID),
		attribute.String("step.name", stepExecution.StepName),
		attribute.String("step.execution_id", stepExecution.ID),
	}
	r.itemReadCounter.Add(ctx, count, metric.WithAttributes(attrs...))
}

// RecordItemProcess records the number of items successfully processed during a step.
// It increments a counter for items processed and attaches relevant step attributes.
//
// Parameters:
//
//	ctx: The context for the operation.
//	stepExecution: The StepExecution instance where the items were processed.
//	count: The number of items processed.
func (r *OtelMetricRecorder) RecordItemProcess(ctx context.Context, stepExecution *model.StepExecution, count int64) {
	attrs := []attribute.KeyValue{
		attribute.String("job.name", stepExecution.JobExecution.JobName),
		attribute.String("job.execution_id", stepExecution.JobExecutionID),
		attribute.String("step.name", stepExecution.StepName),
		attribute.String("step.execution_id", stepExecution.ID),
	}
	r.itemProcessCounter.Add(ctx, count, metric.WithAttributes(attrs...))
}

// RecordItemWrite records the number of items successfully written during a step.
// It increments a counter for items written and attaches relevant step attributes.
//
// Parameters:
//
//	ctx: The context for the operation.
//	stepExecution: The StepExecution instance where the items were written.
//	count: The number of items written.
func (r *OtelMetricRecorder) RecordItemWrite(ctx context.Context, stepExecution *model.StepExecution, count int64) {
	attrs := []attribute.KeyValue{
		attribute.String("job.name", stepExecution.JobExecution.JobName),
		attribute.String("job.execution_id", stepExecution.JobExecutionID),
		attribute.String("step.name", stepExecution.StepName),
		attribute.String("step.execution_id", stepExecution.ID),
	}
	r.itemWriteCounter.Add(ctx, count, metric.WithAttributes(attrs...))
}

// RecordItemSkip records that an item was skipped due to an error.
// It increments a counter for skipped items and attaches relevant step and error attributes.
//
// Parameters:
//
//	ctx: The context for the operation.
//	stepExecution: The StepExecution instance where the item was skipped.
//	err: The error that caused the item to be skipped.
func (r *OtelMetricRecorder) RecordItemSkip(ctx context.Context, stepExecution *model.StepExecution, err error) {
	attrs := []attribute.KeyValue{
		attribute.String("job.name", stepExecution.JobExecution.JobName),
		attribute.String("job.execution_id", stepExecution.JobExecutionID),
		attribute.String("step.name", stepExecution.StepName),
		attribute.String("step.execution_id", stepExecution.ID),
		attribute.String("skip.reason", exception.ExtractErrorMessage(err)), // Extract and attach the error message.
	}
	r.itemSkipCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// RecordItemRetry records that an item was retried after an error.
// It increments a counter for retried items and attaches relevant step and error attributes.
//
// Parameters:
//
//	ctx: The context for the operation.
//	stepExecution: The StepExecution instance where the item was retried.
//	err: The error that caused the item to be retried.
func (r *OtelMetricRecorder) RecordItemRetry(ctx context.Context, stepExecution *model.StepExecution, err error) {
	attrs := []attribute.KeyValue{
		attribute.String("job.name", stepExecution.JobExecution.JobName),
		attribute.String("job.execution_id", stepExecution.JobExecutionID),
		attribute.String("step.name", stepExecution.StepName),
		attribute.String("step.execution_id", stepExecution.ID),
		attribute.String("retry.reason", exception.ExtractErrorMessage(err)), // Extract and attach the error message.
	}
	r.itemRetryCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// RecordChunkCommit records the number of items committed in a chunk.
// It increments a counter for chunk commits and attaches relevant step attributes.
//
// Parameters:
//
//	ctx: The context for the operation.
//	stepExecution: The StepExecution instance where the chunk was committed.
//	count: The number of items committed in the chunk.
func (r *OtelMetricRecorder) RecordChunkCommit(ctx context.Context, stepExecution *model.StepExecution, count int64) {
	attrs := []attribute.KeyValue{
		attribute.String("job.name", stepExecution.JobExecution.JobName),
		attribute.String("job.execution_id", stepExecution.JobExecutionID),
		attribute.String("step.name", stepExecution.StepName),
		attribute.String("step.execution_id", stepExecution.ID),
	}
	r.chunkCommitCounter.Add(ctx, count, metric.WithAttributes(attrs...))
}

// RecordDuration records a custom duration metric.
// It records the given duration value to a histogram and attaches provided attributes,
// along with a `metric.name` attribute to identify the specific duration being recorded.
//
// Parameters:
//
//	ctx: The context for the operation.
//	name: The logical name of the duration (e.g., "api_call_duration", "db_query_time").
//	duration: The duration value to record, typically in seconds.
//	attrs: Optional OpenTelemetry attributes to associate with this specific duration recording.
func (r *OtelMetricRecorder) RecordDuration(ctx context.Context, name string, duration float64, attrs ...attribute.KeyValue) {
	// Add a generic "metric.name" attribute to distinguish different durations
	// recorded by the same histogram instrument.
	allAttrs := make([]attribute.KeyValue, len(attrs)+1)
	allAttrs[0] = attribute.String("metric.name", name)
	copy(allAttrs[1:], attrs)

	r.durationHistogram.Record(ctx, duration, metric.WithAttributes(allAttrs...))
}
