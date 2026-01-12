package metrics

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	model "surfin/pkg/batch/core/domain/model"
	port "surfin/pkg/batch/core/application/port"
	metrics "surfin/pkg/batch/core/metrics"
	logger "surfin/pkg/batch/support/util/logger"
)

// PrometheusRecorder is a Prometheus implementation of the metrics.MetricRecorder interface.
type PrometheusRecorder struct {
	registry *prometheus.Registry

	// Job Metrics
	jobDurationSeconds *prometheus.HistogramVec
	jobStatusCounter   *prometheus.CounterVec

	// Step Metrics
	stepDurationSeconds *prometheus.HistogramVec
	stepStatusCounter   *prometheus.CounterVec
	stepReadCount       *prometheus.CounterVec
	stepWriteCount      *prometheus.CounterVec
	stepFilterCount     *prometheus.CounterVec
	stepCommitCount     *prometheus.CounterVec
	stepRollbackCount   *prometheus.CounterVec

	// Item Metrics
	itemSkipCounter  *prometheus.CounterVec
	itemRetryCounter *prometheus.CounterVec
}

// NewPrometheusRecorder creates a new instance of PrometheusRecorder.
func NewPrometheusRecorder() metrics.MetricRecorder {
	registry := prometheus.NewRegistry()

	// Register Go standard metrics and process/OS metrics.
	registry.MustRegister(collectors.NewGoCollector())
	registry.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))

	r := &PrometheusRecorder{
		registry: registry,
		jobDurationSeconds: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "batch_job_duration_seconds",
			Help:    "Duration of batch job executions.",
			Buckets: prometheus.DefBuckets,
		}, []string{"job_name", "status", "exit_status"}),
		jobStatusCounter: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "batch_job_status_total",
			Help: "Total number of batch job executions by status.",
		}, []string{"job_name", "status"}),
		stepDurationSeconds: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "batch_step_duration_seconds",
			Help:    "Duration of batch step executions.",
			Buckets: prometheus.DefBuckets,
		}, []string{"job_name", "step_name", "status", "exit_status"}),
		stepStatusCounter: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "batch_step_status_total",
			Help: "Total number of batch step executions by status.",
		}, []string{"job_name", "step_name", "status"}),
		stepReadCount: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "batch_step_read_total",
			Help: "Total items read by step.",
		}, []string{"job_name", "step_name"}),
		stepWriteCount: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "batch_step_write_total",
			Help: "Total items written by step.",
		}, []string{"job_name", "step_name"}),
		stepFilterCount: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "batch_step_filter_total",
			Help: "Total items filtered by step.",
		}, []string{"job_name", "step_name"}),
		stepCommitCount: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "batch_step_commit_total",
			Help: "Total chunk commits by step.",
		}, []string{"job_name", "step_name"}),
		stepRollbackCount: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "batch_step_rollback_total",
			Help: "Total chunk rollbacks by step.",
		}, []string{"job_name", "step_name"}),
		itemSkipCounter: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "batch_item_skip_total",
			Help: "Total items skipped by step and reason.",
		}, []string{"job_name", "step_name", "reason", "type"}), // type: read, process, write
		itemRetryCounter: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "batch_item_retry_total",
			Help: "Total item retries by step and reason.",
		}, []string{"job_name", "step_name", "reason", "type"}), // type: read, process, write
	}

	// Register all metrics with the registry.
	registry.MustRegister(r.jobDurationSeconds)
	registry.MustRegister(r.jobStatusCounter)
	registry.MustRegister(r.stepDurationSeconds)
	registry.MustRegister(r.stepStatusCounter)
	registry.MustRegister(r.stepReadCount)
	registry.MustRegister(r.stepWriteCount)
	registry.MustRegister(r.stepFilterCount)
	registry.MustRegister(r.stepCommitCount)
	registry.MustRegister(r.stepRollbackCount)
	registry.MustRegister(r.itemSkipCounter)
	registry.MustRegister(r.itemRetryCounter)

	return r
}

// GetRegistry returns the Prometheus registry.
func (r *PrometheusRecorder) GetRegistry() *prometheus.Registry {
	return r.registry
}

// RecordJobStart records the start of a JobExecution.
func (r *PrometheusRecorder) RecordJobStart(ctx context.Context, execution *model.JobExecution) {
	r.jobStatusCounter.WithLabelValues(execution.JobName, execution.Status.String()).Inc()
	logger.Debugf("Metrics: Job '%s' started.", execution.JobName)
}

// RecordJobEnd records the end of a JobExecution.
func (r *PrometheusRecorder) RecordJobEnd(ctx context.Context, execution *model.JobExecution) {
	if execution.EndTime == nil {
		return
	}
	duration := execution.EndTime.Sub(execution.StartTime).Seconds()

	r.jobDurationSeconds.WithLabelValues(
		execution.JobName,
		execution.Status.String(),
		execution.ExitStatus.String(),
	).Observe(duration)

	logger.Debugf("Metrics: Job '%s' ended. Duration: %.3fs", execution.JobName, duration)
}

// RecordStepStart records the start of a StepExecution.
func (r *PrometheusRecorder) RecordStepStart(ctx context.Context, execution *model.StepExecution) {
	// JobExecution is included in StepExecution, so JobName can be obtained.
	jobName := execution.JobExecution.JobName
	r.stepStatusCounter.WithLabelValues(jobName, execution.StepName, execution.Status.String()).Inc()
	logger.Debugf("Metrics: Step '%s' started.", execution.StepName)
}

// RecordStepEnd records the end of a StepExecution.
func (r *PrometheusRecorder) RecordStepEnd(ctx context.Context, execution *model.StepExecution) {
	if execution.EndTime == nil {
		return
	}
	duration := execution.EndTime.Sub(execution.StartTime).Seconds()

	jobName := execution.JobExecution.JobName
	stepName := execution.StepName

	r.stepDurationSeconds.WithLabelValues(
		jobName,
		stepName,
		execution.Status.String(),
		execution.ExitStatus.String(),
	).Observe(duration)

	// Add the final aggregated value of StepExecution to the counter (only the final value is recorded here as it's incremented within ChunkStep.Execute).
	// Note: Prometheus Counters should be monotonically increasing, so use Add().
	// StepExecution's ReadCount, etc., are total values over the entire lifecycle of the StepExecution, so
	// using Add() here might lead to double counting if JobExecution is executed multiple times.
	// It is common to record only Duration and Status for Job/Step completion metrics, and Read/Write/Skip/Retry at the item or chunk level.
	// However, here we follow the design of recording the final values of StepExecution.

	// Since Status is already recorded in RecordStepStart, only the final values of Read/Write/Commit/Rollback are recorded here.
	// Since it's already incremented in ChunkStep.Execute, there's no need to record the final difference here.
	// Instead of recording final statistics in StepEnd, trust the counters incremented within ChunkStep.

	logger.Debugf("Metrics: Step '%s' ended. Duration: %.3fs", execution.StepName, duration)
}

// RecordItemRead records successful item reads.
func (r *PrometheusRecorder) RecordItemRead(ctx context.Context, stepName string) {
	if se := port.GetStepExecutionFromContext(ctx); se != nil {
		r.stepReadCount.WithLabelValues(se.JobExecution.JobName, stepName).Inc()
	}
}

// RecordItemProcess records successful item processing.
func (r *PrometheusRecorder) RecordItemProcess(ctx context.Context, stepName string) {
	// Successful item processing occurs between Read/Write, so a separate counter is usually not needed.
	// If necessary, increment stepProcessCount here.
}

// RecordItemWrite records successful item writes.
func (r *PrometheusRecorder) RecordItemWrite(ctx context.Context, stepName string, count int) {
	if se := port.GetStepExecutionFromContext(ctx); se != nil {
		r.stepWriteCount.WithLabelValues(se.JobExecution.JobName, stepName).Add(float64(count))
	}
}

// RecordItemSkip records item skips.
func (r *PrometheusRecorder) RecordItemSkip(ctx context.Context, stepName string, reason string) {
	skipType := "unknown"
	if reason == "read" || reason == "process" || reason == "write" {
		skipType = reason
	}

	if se := port.GetStepExecutionFromContext(ctx); se != nil {
		r.itemSkipCounter.WithLabelValues(se.JobExecution.JobName, stepName, reason, skipType).Inc()
	}
}

// RecordItemRetry records item retries.
func (r *PrometheusRecorder) RecordItemRetry(ctx context.Context, stepName string, reason string) {
	retryType := "unknown"
	if reason == "read" || reason == "process" || reason == "write" {
		retryType = reason
	}

	if se := port.GetStepExecutionFromContext(ctx); se != nil {
		r.itemRetryCounter.WithLabelValues(se.JobExecution.JobName, stepName, reason, retryType).Inc()
	}
}

// RecordChunkCommit records chunk commits.
func (r *PrometheusRecorder) RecordChunkCommit(ctx context.Context, stepName string, count int) {
	if se := port.GetStepExecutionFromContext(ctx); se != nil {
		r.stepCommitCount.WithLabelValues(se.JobExecution.JobName, stepName).Inc()
	}
}

// RecordDuration records the execution time of a specific operation.
func (r *PrometheusRecorder) RecordDuration(ctx context.Context, name string, duration time.Duration, tags map[string]string) {
	// Generic Duration recording can be implemented using Prometheus Histograms, but
	// since this is specialized for Job/Step Duration, a generic implementation is skipped.
	// If necessary, define a new HistogramVec.
}

var _ metrics.MetricRecorder = (*PrometheusRecorder)(nil)
