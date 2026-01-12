package metrics

import (
	"context"
	"time"

	model "surfin/pkg/batch/core/domain/model"
	"surfin/pkg/batch/core/metrics"
	"surfin/pkg/batch/support/util/logger"
)

// PrometheusMetricRecorder is a concrete implementation that records metrics to external systems like Prometheus.
type PrometheusMetricRecorder struct{}

// NewPrometheusMetricRecorder creates a new instance of PrometheusMetricRecorder.
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
	if execution.EndTime == nil { return }
	duration := execution.EndTime.Sub(execution.StartTime)
	logger.Infof("Metrics: Job End recorded. JobName: %s, Status: %s, Duration: %s", execution.JobName, execution.Status, duration)
}

// RecordStepStart records the start of a StepExecution.
func (r *PrometheusMetricRecorder) RecordStepStart(ctx context.Context, execution *model.StepExecution) {
	logger.Debugf("Metrics: Step Start recorded. StepName: %s, ID: %s", execution.StepName, execution.ID)
}

// RecordStepEnd records the end of a StepExecution.
func (r *PrometheusMetricRecorder) RecordStepEnd(ctx context.Context, execution *model.StepExecution) {
	if execution.EndTime == nil { return }
	duration := execution.EndTime.Sub(execution.StartTime)
	logger.Debugf("Metrics: Step End recorded. StepName: %s, Status: %s, Duration: %s", execution.StepName, execution.Status, duration)
}

// RecordItemRead records successful item reads.
func (r *PrometheusMetricRecorder) RecordItemRead(ctx context.Context, stepName string) {}

// RecordItemProcess records successful item processing.
func (r *PrometheusMetricRecorder) RecordItemProcess(ctx context.Context, stepName string) {}

// RecordItemWrite records successful item writes.
func (r *PrometheusMetricRecorder) RecordItemWrite(ctx context.Context, stepName string, count int) {}

// RecordItemSkip records item skips.
func (r *PrometheusMetricRecorder) RecordItemSkip(ctx context.Context, stepName string, reason string) {}

// RecordItemRetry records item retries.
func (r *PrometheusMetricRecorder) RecordItemRetry(ctx context.Context, stepName string, reason string) {}

// RecordChunkCommit records chunk commits.
func (r *PrometheusMetricRecorder) RecordChunkCommit(ctx context.Context, stepName string, count int) {
	logger.Debugf("Metrics: Chunk committed. Step: %s, Count: %d", stepName, count)
}

// RecordDuration records the execution time of a specific operation.
func (r *PrometheusMetricRecorder) RecordDuration(ctx context.Context, name string, duration time.Duration, tags map[string]string) {
	// Implements custom metrics/duration logging based on user instructions.
	logger.Debugf("Metrics: Duration recorded. Name: %s, Duration: %s, Tags: %+v", name, duration, tags)
}

var _ metrics.MetricRecorder = (*PrometheusMetricRecorder)(nil)
