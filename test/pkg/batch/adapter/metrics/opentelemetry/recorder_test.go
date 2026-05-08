package opentelemetry_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata" // Requires metricdata.ResourceMetrics, etc.

	"github.com/tigerroll/surfin/pkg/batch/adapter/metrics/opentelemetry"
	"github.com/tigerroll/surfin/pkg/batch/core/domain/model"
)

// findMetric finds a metric with the given name within a collection of ResourceMetrics.
func findMetric(rm *metricdata.ResourceMetrics, name string) *metricdata.Metrics {
	for _, sm := range rm.ScopeMetrics { // Iterate through ScopeMetrics to find the target metric.
		for _, m := range sm.Metrics {
			if m.Name == name {
				return &m
			}
		}
	}
	return nil
}

// assertSumMetric asserts the value and attributes of a Sum metric.
func assertSumMetric(t *testing.T, rm *metricdata.ResourceMetrics, metricName string, expectedValue int64, expectedAttrs []attribute.KeyValue) {
	m := findMetric(rm, metricName)
	assert.NotNil(t, m, "Metric '%s' not found", metricName)
	if m == nil {
		return
	}

	assert.IsType(t, metricdata.Sum[int64]{}, m.Data, "Metric data for '%s' should be of type Sum[int64]", metricName)
	sumData := m.Data.(metricdata.Sum[int64])
	assert.Len(t, sumData.DataPoints, 1, "Expected one data point for '%s'", metricName)
	if len(sumData.DataPoints) == 0 {
		return
	}
	dp := sumData.DataPoints[0]
	assert.Equal(t, expectedValue, dp.Value, "Sum value mismatch for metric '%s'", metricName)
	assert.ElementsMatch(t, expectedAttrs, dp.Attributes.ToSlice(), "Attributes mismatch for metric '%s'", metricName)
}

// assertHistogramMetric asserts the count, sum, and attributes of a Histogram metric.
func assertHistogramMetric(t *testing.T, rm *metricdata.ResourceMetrics, metricName string, expectedCount int64, expectedSum float64, expectedAttrs []attribute.KeyValue) {
	m := findMetric(rm, metricName)
	assert.NotNil(t, m, "Metric '%s' not found", metricName)
	if m == nil {
		return
	}

	assert.IsType(t, metricdata.Histogram[float64]{}, m.Data, "Metric data for '%s' should be Histogram[float64]", metricName)
	histData := m.Data.(metricdata.Histogram[float64])
	assert.Len(t, histData.DataPoints, 1, "Expected one data point for '%s'", metricName)
	if len(histData.DataPoints) == 0 {
		return
	}
	dp := histData.DataPoints[0]
	assert.EqualValues(t, expectedCount, dp.Count, "Histogram count mismatch for metric '%s'", metricName)
	assert.InDelta(t, expectedSum, dp.Sum, 0.001, "Histogram sum mismatch for metric '%s'", metricName)
	assert.ElementsMatch(t, expectedAttrs, dp.Attributes.ToSlice(), "Attributes mismatch for metric '%s'", metricName)
}

// TestOtelMetricRecorder_NewOtelMetricRecorder tests the initialization of OtelMetricRecorder.
func TestOtelMetricRecorder_NewOtelMetricRecorder(t *testing.T) {
	// For OpenTelemetry SDK testing, use metric.NewManualReader() and metric.NewMeterProvider
	// instead of the deprecated metricdatatest.NewRecorder().
	reader := metric.NewManualReader()
	meterProvider := metric.NewMeterProvider(metric.WithReader(reader))
	meter := meterProvider.Meter("surfin.batch.metrics")

	recorder, err := opentelemetry.NewOtelMetricRecorder(meter)
	assert.NoError(t, err)
	assert.NotNil(t, recorder)
}

// TestOtelMetricRecorder_RecordJobStart tests recording job start metrics.
func TestOtelMetricRecorder_RecordJobStart(t *testing.T) {
	ctx := context.Background()

	reader := metric.NewManualReader()
	meterProvider := metric.NewMeterProvider(metric.WithReader(reader))
	meter := meterProvider.Meter("surfin.batch.metrics")

	recorder, err := opentelemetry.NewOtelMetricRecorder(meter)
	assert.NoError(t, err)

	jobExecution := &model.JobExecution{
		ID:        "job-123",
		JobName:   "test-job",
		StartTime: time.Now(),
	}

	recorder.RecordJobStart(ctx, jobExecution)

	var rm metricdata.ResourceMetrics // ResourceMetrics to collect data into.
	err = reader.Collect(ctx, &rm)    // Collect metrics into rm.
	assert.NoError(t, err)            // Assert no error during collection.

	// Use assertSumMetric helper function.
	expectedAttrs := []attribute.KeyValue{
		attribute.String("job.name", "test-job"),
		attribute.String("job.execution_id", "job-123"),
	}
	assertSumMetric(t, &rm, "surfin.batch.job.start.total", 1, expectedAttrs)
}

// TestOtelMetricRecorder_RecordJobEnd tests recording job end metrics.
func TestOtelMetricRecorder_RecordJobEnd(t *testing.T) {
	ctx := context.Background()

	reader := metric.NewManualReader()
	meterProvider := metric.NewMeterProvider(metric.WithReader(reader))
	meter := meterProvider.Meter("surfin.batch.metrics")

	recorder, err := opentelemetry.NewOtelMetricRecorder(meter)
	assert.NoError(t, err)

	endTime := time.Now()
	jobExecution := &model.JobExecution{
		ID:         "job-123",
		JobName:    "test-job",
		StartTime:  endTime.Add(-5 * time.Second),
		EndTime:    &endTime,
		Status:     model.BatchStatusCompleted,
		ExitStatus: model.ExitStatusCompleted,
	}

	recorder.RecordJobEnd(ctx, jobExecution)

	var rm metricdata.ResourceMetrics // ResourceMetrics to collect data into.
	err = reader.Collect(ctx, &rm)    // Collect metrics into rm.
	assert.NoError(t, err)            // Assert no error during collection.

	// Use assertSumMetric helper function for sum metric.
	expectedSumAttrs := []attribute.KeyValue{
		attribute.String("job.name", "test-job"),
		attribute.String("job.execution_id", "job-123"),
		attribute.String("job.status", string(model.BatchStatusCompleted)),
		attribute.String("job.exit_status", string(model.ExitStatusCompleted)),
	}
	assertSumMetric(t, &rm, "surfin.batch.job.end.total", 1, expectedSumAttrs)

	// Use assertHistogramMetric helper function for histogram metric.
	expectedHistAttrs := []attribute.KeyValue{
		attribute.String("metric.name", "job.duration"),
		attribute.String("job.name", "test-job"),
		attribute.String("job.execution_id", "job-123"),
		attribute.String("job.status", string(model.BatchStatusCompleted)),
		attribute.String("job.exit_status", string(model.ExitStatusCompleted)),
	}
	assertHistogramMetric(t, &rm, "surfin.batch.duration.seconds", 1, 5.0, expectedHistAttrs)
}

// TestOtelMetricRecorder_RecordStepStart tests recording step start metrics.
func TestOtelMetricRecorder_RecordStepStart(t *testing.T) {
	ctx := context.Background()
	reader := metric.NewManualReader()
	meterProvider := metric.NewMeterProvider(metric.WithReader(reader))
	meter := meterProvider.Meter("surfin.batch.metrics")
	recorder, err := opentelemetry.NewOtelMetricRecorder(meter)
	assert.NoError(t, err)

	jobExecution := &model.JobExecution{
		ID:        "job-123",
		JobName:   "test-job",
		StartTime: time.Now(),
	}
	stepExecution := &model.StepExecution{
		ID:             "step-456",
		StepName:       "test-step",
		JobExecution:   jobExecution,
		JobExecutionID: jobExecution.ID,
		StartTime:      time.Now(),
	}

	recorder.RecordStepStart(ctx, stepExecution)

	var rm metricdata.ResourceMetrics // ResourceMetrics to collect data into.
	err = reader.Collect(ctx, &rm)    // Collect metrics into rm.
	assert.NoError(t, err)            // Assert no error during collection.

	// Use assertSumMetric helper function.
	expectedAttrs := []attribute.KeyValue{
		attribute.String("job.name", "test-job"),
		attribute.String("job.execution_id", "job-123"),
		attribute.String("step.name", "test-step"),
		attribute.String("step.execution_id", "step-456"),
	}
	assertSumMetric(t, &rm, "surfin.batch.step.start.total", 1, expectedAttrs)
}

// TestOtelMetricRecorder_RecordStepEnd tests recording step end metrics.
func TestOtelMetricRecorder_RecordStepEnd(t *testing.T) {
	ctx := context.Background()
	reader := metric.NewManualReader()
	meterProvider := metric.NewMeterProvider(metric.WithReader(reader))
	meter := meterProvider.Meter("surfin.batch.metrics")
	recorder, err := opentelemetry.NewOtelMetricRecorder(meter)
	assert.NoError(t, err)

	endTime := time.Now()
	jobExecution := &model.JobExecution{
		ID:        "job-123",
		JobName:   "test-job",
		StartTime: endTime.Add(-10 * time.Second),
	}
	stepExecution := &model.StepExecution{
		ID:             "step-456",
		StepName:       "test-step",
		JobExecution:   jobExecution,
		JobExecutionID: jobExecution.ID,
		StartTime:      endTime.Add(-5 * time.Second),
		EndTime:        &endTime,
		Status:         model.BatchStatusCompleted,
		ExitStatus:     model.ExitStatusCompleted,
	}

	recorder.RecordStepEnd(ctx, stepExecution)

	var rm metricdata.ResourceMetrics // ResourceMetrics to collect data into.
	err = reader.Collect(ctx, &rm)    // Collect metrics into rm.
	assert.NoError(t, err)            // Assert no error during collection.

	// Use assertSumMetric helper function for sum metric.
	expectedSumAttrs := []attribute.KeyValue{
		attribute.String("job.name", "test-job"),
		attribute.String("job.execution_id", "job-123"),
		attribute.String("step.name", "test-step"),
		attribute.String("step.execution_id", "step-456"),
		attribute.String("step.status", string(model.BatchStatusCompleted)),
		attribute.String("step.exit_status", string(model.ExitStatusCompleted)),
	}
	assertSumMetric(t, &rm, "surfin.batch.step.end.total", 1, expectedSumAttrs)

	// Use assertHistogramMetric helper function for histogram metric.
	expectedHistAttrs := []attribute.KeyValue{
		attribute.String("metric.name", "step.duration"),
		attribute.String("job.name", "test-job"),
		attribute.String("job.execution_id", "job-123"),
		attribute.String("step.name", "test-step"),
		attribute.String("step.execution_id", "step-456"),
		attribute.String("step.status", string(model.BatchStatusCompleted)),
		attribute.String("step.exit_status", string(model.ExitStatusCompleted)),
	}
	assertHistogramMetric(t, &rm, "surfin.batch.duration.seconds", 1, 5.0, expectedHistAttrs)
}

// TestOtelMetricRecorder_RecordItemRead tests recording item read metrics.
func TestOtelMetricRecorder_RecordItemRead(t *testing.T) {
	ctx := context.Background()
	reader := metric.NewManualReader()
	meterProvider := metric.NewMeterProvider(metric.WithReader(reader))
	meter := meterProvider.Meter("surfin.batch.metrics")
	recorder, err := opentelemetry.NewOtelMetricRecorder(meter)
	assert.NoError(t, err)

	jobExecution := &model.JobExecution{ID: "job-123", JobName: "test-job"}
	stepExecution := &model.StepExecution{ID: "step-456", StepName: "test-step", JobExecution: jobExecution, JobExecutionID: jobExecution.ID}

	recorder.RecordItemRead(ctx, stepExecution, 10)

	var rm metricdata.ResourceMetrics // ResourceMetrics to collect data into.
	err = reader.Collect(ctx, &rm)    // Collect metrics into rm.
	assert.NoError(t, err)            // Assert no error during collection.

	// Use assertSumMetric helper function.
	expectedAttrs := []attribute.KeyValue{
		attribute.String("job.name", "test-job"),
		attribute.String("job.execution_id", "job-123"),
		attribute.String("step.name", "test-step"),
		attribute.String("step.execution_id", "step-456"),
	}
	assertSumMetric(t, &rm, "surfin.batch.item.read.total", 10, expectedAttrs)
}

// TestOtelMetricRecorder_RecordItemProcess tests recording item process metrics.
func TestOtelMetricRecorder_RecordItemProcess(t *testing.T) {
	ctx := context.Background()
	reader := metric.NewManualReader()
	meterProvider := metric.NewMeterProvider(metric.WithReader(reader))
	meter := meterProvider.Meter("surfin.batch.metrics")
	recorder, err := opentelemetry.NewOtelMetricRecorder(meter)
	assert.NoError(t, err)

	jobExecution := &model.JobExecution{ID: "job-123", JobName: "test-job"}
	stepExecution := &model.StepExecution{ID: "step-456", StepName: "test-step", JobExecution: jobExecution, JobExecutionID: jobExecution.ID}

	recorder.RecordItemProcess(ctx, stepExecution, 5)

	var rm metricdata.ResourceMetrics // ResourceMetrics to collect data into.
	err = reader.Collect(ctx, &rm)    // Collect metrics into rm.
	assert.NoError(t, err)            // Assert no error during collection.

	// Use assertSumMetric helper function.
	expectedAttrs := []attribute.KeyValue{
		attribute.String("job.name", "test-job"),
		attribute.String("job.execution_id", "job-123"),
		attribute.String("step.name", "test-step"),
		attribute.String("step.execution_id", "step-456"),
	}
	assertSumMetric(t, &rm, "surfin.batch.item.process.total", 5, expectedAttrs)
}

// TestOtelMetricRecorder_RecordItemWrite tests recording item write metrics.
func TestOtelMetricRecorder_RecordItemWrite(t *testing.T) {
	ctx := context.Background()
	reader := metric.NewManualReader()
	meterProvider := metric.NewMeterProvider(metric.WithReader(reader))
	meter := meterProvider.Meter("surfin.batch.metrics")
	recorder, err := opentelemetry.NewOtelMetricRecorder(meter)
	assert.NoError(t, err)

	jobExecution := &model.JobExecution{ID: "job-123", JobName: "test-job"}
	stepExecution := &model.StepExecution{ID: "step-456", StepName: "test-step", JobExecution: jobExecution, JobExecutionID: jobExecution.ID}

	recorder.RecordItemWrite(ctx, stepExecution, 8)

	var rm metricdata.ResourceMetrics // ResourceMetrics to collect data into.
	err = reader.Collect(ctx, &rm)    // Collect metrics into rm.
	assert.NoError(t, err)            // Assert no error during collection.

	// Use assertSumMetric helper function.
	expectedAttrs := []attribute.KeyValue{
		attribute.String("job.name", "test-job"),
		attribute.String("job.execution_id", "job-123"),
		attribute.String("step.name", "test-step"),
		attribute.String("step.execution_id", "step-456"),
	}
	assertSumMetric(t, &rm, "surfin.batch.item.write.total", 8, expectedAttrs)
}

// TestOtelMetricRecorder_RecordItemSkip tests recording item skip metrics.
func TestOtelMetricRecorder_RecordItemSkip(t *testing.T) {
	ctx := context.Background()
	reader := metric.NewManualReader()
	meterProvider := metric.NewMeterProvider(metric.WithReader(reader))
	meter := meterProvider.Meter("surfin.batch.metrics")
	recorder, err := opentelemetry.NewOtelMetricRecorder(meter)
	assert.NoError(t, err)

	jobExecution := &model.JobExecution{ID: "job-123", JobName: "test-job"}
	stepExecution := &model.StepExecution{ID: "step-456", StepName: "test-step", JobExecution: jobExecution, JobExecutionID: jobExecution.ID}
	skipErr := fmt.Errorf("test skip error")

	recorder.RecordItemSkip(ctx, stepExecution, skipErr)

	var rm metricdata.ResourceMetrics // ResourceMetrics to collect data into.
	err = reader.Collect(ctx, &rm)    // Collect metrics into rm.
	assert.NoError(t, err)            // Assert no error during collection.

	// Use assertSumMetric helper function.
	expectedAttrs := []attribute.KeyValue{
		attribute.String("job.name", "test-job"),
		attribute.String("job.execution_id", "job-123"),
		attribute.String("step.name", "test-step"),
		attribute.String("step.execution_id", "step-456"),
		attribute.String("skip.reason", "test skip error"),
	}
	assertSumMetric(t, &rm, "surfin.batch.item.skip.total", 1, expectedAttrs)
}

// TestOtelMetricRecorder_RecordItemRetry tests recording item retry metrics.
func TestOtelMetricRecorder_RecordItemRetry(t *testing.T) {
	ctx := context.Background()
	reader := metric.NewManualReader()
	meterProvider := metric.NewMeterProvider(metric.WithReader(reader))
	meter := meterProvider.Meter("surfin.batch.metrics")
	recorder, err := opentelemetry.NewOtelMetricRecorder(meter)
	assert.NoError(t, err)

	jobExecution := &model.JobExecution{ID: "job-123", JobName: "test-job"}
	stepExecution := &model.StepExecution{ID: "step-456", StepName: "test-step", JobExecution: jobExecution, JobExecutionID: jobExecution.ID}
	retryErr := fmt.Errorf("test retry error")

	recorder.RecordItemRetry(ctx, stepExecution, retryErr)

	var rm metricdata.ResourceMetrics // ResourceMetrics to collect data into.
	err = reader.Collect(ctx, &rm)    // Collect metrics into rm.
	assert.NoError(t, err)            // Assert no error during collection.

	// Use assertSumMetric helper function.
	expectedAttrs := []attribute.KeyValue{
		attribute.String("job.name", "test-job"),
		attribute.String("job.execution_id", "job-123"),
		attribute.String("step.name", "test-step"),
		attribute.String("step.execution_id", "step-456"),
		attribute.String("retry.reason", "test retry error"),
	}
	assertSumMetric(t, &rm, "surfin.batch.item.retry.total", 1, expectedAttrs)
}

// TestOtelMetricRecorder_RecordChunkCommit tests recording chunk commit metrics.
func TestOtelMetricRecorder_RecordChunkCommit(t *testing.T) {
	ctx := context.Background()
	reader := metric.NewManualReader()
	meterProvider := metric.NewMeterProvider(metric.WithReader(reader))
	meter := meterProvider.Meter("surfin.batch.metrics")
	recorder, err := opentelemetry.NewOtelMetricRecorder(meter)
	assert.NoError(t, err)

	jobExecution := &model.JobExecution{ID: "job-123", JobName: "test-job"}
	stepExecution := &model.StepExecution{ID: "step-456", StepName: "test-step", JobExecution: jobExecution, JobExecutionID: jobExecution.ID}

	recorder.RecordChunkCommit(ctx, stepExecution, 1)

	var rm metricdata.ResourceMetrics // ResourceMetrics to collect data into.
	err = reader.Collect(ctx, &rm)    // Collect metrics into rm.
	assert.NoError(t, err)            // Assert no error during collection.

	// Use assertSumMetric helper function.
	expectedAttrs := []attribute.KeyValue{
		attribute.String("job.name", "test-job"),
		attribute.String("job.execution_id", "job-123"),
		attribute.String("step.name", "test-step"),
		attribute.String("step.execution_id", "step-456"),
	}
	assertSumMetric(t, &rm, "surfin.batch.chunk.commit.total", 1, expectedAttrs)
}

// TestOtelMetricRecorder_RecordDuration tests recording duration metrics.
func TestOtelMetricRecorder_RecordDuration(t *testing.T) {
	ctx := context.Background()
	reader := metric.NewManualReader()
	meterProvider := metric.NewMeterProvider(metric.WithReader(reader))
	meter := meterProvider.Meter("surfin.batch.metrics")
	recorder, err := opentelemetry.NewOtelMetricRecorder(meter)
	assert.NoError(t, err)

	recorder.RecordDuration(ctx, "test.operation.duration", 1.23, attribute.String("key", "value"))

	var rm metricdata.ResourceMetrics // ResourceMetrics to collect data into.
	err = reader.Collect(ctx, &rm)    // Collect metrics into rm.
	assert.NoError(t, err)            // Assert no error during collection.

	// Use assertHistogramMetric helper function.
	expectedAttrs := []attribute.KeyValue{
		attribute.String("metric.name", "test.operation.duration"),
		attribute.String("key", "value"),
	}
	assertHistogramMetric(t, &rm, "surfin.batch.duration.seconds", 1, 1.23, expectedAttrs)
}
