package metrics

import (
	"context"

	model "github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	"github.com/tigerroll/surfin/pkg/batch/core/metrics"
	"github.com/tigerroll/surfin/pkg/batch/support/util/exception"
	"github.com/tigerroll/surfin/pkg/batch/support/util/logger"
	"go.opentelemetry.io/otel/attribute"
)

// Package metrics provides concrete implementations of the MetricRecorder interface.
// These implementations typically log metric events or send them to specific monitoring systems.

// PrometheusMetricRecorder is a concrete implementation that records metrics to external systems like Prometheus.
// Currently, it only logs the metric events.
type PrometheusMetricRecorder struct{}

// NewPrometheusMetricRecorder creates and returns a new instance of PrometheusMetricRecorder.
func NewPrometheusMetricRecorder() metrics.MetricRecorder {
	logger.Infof("Metrics: Initializing Prometheus Metric Recorder.")
	return &PrometheusMetricRecorder{}
}

// RecordJobStart records the start of a JobExecution.
func (r *PrometheusMetricRecorder) RecordJobStart(ctx context.Context, execution *model.JobExecution) {
	logger.Debugf("Metrics: Job Start recorded. JobName: %s, ID: %s", execution.JobName, execution.ID)
}

// RecordJobEnd records the end of a JobExecution.
func (r *PrometheusMetricRecorder) RecordJobEnd(ctx context.Context, execution *model.JobExecution) {
	if execution.EndTime == nil {
		return
	}
	duration := execution.EndTime.Sub(execution.StartTime)
	logger.Infof("Metrics: Job End recorded. JobName: %s, Status: %s, Duration: %s", execution.JobName, execution.Status, duration.String())
}

// RecordStepStart records the start of a StepExecution.
func (r *PrometheusMetricRecorder) RecordStepStart(ctx context.Context, execution *model.StepExecution) {
	logger.Debugf("Metrics: Step Start recorded. StepName: %s, ID: %s", execution.StepName, execution.ID)
}

// RecordStepEnd records the end of a StepExecution.
func (r *PrometheusMetricRecorder) RecordStepEnd(ctx context.Context, execution *model.StepExecution) {
	if execution.EndTime == nil {
		return
	}
	duration := execution.EndTime.Sub(execution.StartTime)
	logger.Debugf("Metrics: Step End recorded. StepName: %s, Status: %s, Duration: %s", execution.StepName, execution.Status, duration.String())
}

// RecordItemRead records successful item reads.
func (r *PrometheusMetricRecorder) RecordItemRead(ctx context.Context, stepExecution *model.StepExecution, count int64) {
	logger.Debugf("Metrics: Item Read recorded. Step: %s, Count: %d", stepExecution.StepName, count)
}

// RecordItemProcess records successful item processing.
func (r *PrometheusMetricRecorder) RecordItemProcess(ctx context.Context, stepExecution *model.StepExecution, count int64) {
	logger.Debugf("Metrics: Item Process recorded. Step: %s, Count: %d", stepExecution.StepName, count)
}

// RecordItemWrite records successful item writes.
func (r *PrometheusMetricRecorder) RecordItemWrite(ctx context.Context, stepExecution *model.StepExecution, count int64) {
	logger.Debugf("Metrics: Item Write recorded. Step: %s, Count: %d", stepExecution.StepName, count)
}

// RecordItemSkip records item skips.
func (r *PrometheusMetricRecorder) RecordItemSkip(ctx context.Context, stepExecution *model.StepExecution, err error) {
	logger.Debugf("Metrics: Item Skip recorded. Step: %s, Reason: %s", stepExecution.StepName, exception.ExtractErrorMessage(err))
}

// RecordItemRetry records item retries.
func (r *PrometheusMetricRecorder) RecordItemRetry(ctx context.Context, stepExecution *model.StepExecution, err error) {
	logger.Debugf("Metrics: Item Retry recorded. Step: %s, Reason: %s", stepExecution.StepName, exception.ExtractErrorMessage(err))
}

// RecordChunkCommit records chunk commits.
func (r *PrometheusMetricRecorder) RecordChunkCommit(ctx context.Context, stepExecution *model.StepExecution, count int64) {
	logger.Debugf("Metrics: Chunk committed. Step: %s, Count: %d", stepExecution.StepName, count)
}

// RecordDuration records the execution time of a specific operation.
// This method logs the duration and any associated attributes.
func (r *PrometheusMetricRecorder) RecordDuration(ctx context.Context, name string, duration float64, attrs ...attribute.KeyValue) {
	// Convert OpenTelemetry attributes to a map[string]string for logging.
	tagsMap := make(map[string]string)
	for _, attr := range attrs {
		tagsMap[string(attr.Key)] = attr.Value.Emit() // Emit() returns the attribute value as a string.
	}
	logger.Debugf("Metrics: Duration recorded. Name: %s, Duration: %f, Tags: %+v", name, duration, tagsMap)
}

// RecordExecutionError records a general execution error.
func (r *PrometheusMetricRecorder) RecordExecutionError(ctx context.Context, err error) {
	logger.Errorf("Metrics: Execution error recorded: %v", err)
}

var _ metrics.MetricRecorder = (*PrometheusMetricRecorder)(nil)
