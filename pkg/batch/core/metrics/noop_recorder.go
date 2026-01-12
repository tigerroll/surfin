package metrics

import (
	"context"
	"time"

	model "surfin/pkg/batch/core/domain/model"
)

// NoOpMetricRecorder is an implementation of MetricRecorder that does nothing.
// It is used when metrics are disabled or during testing.
type NoOpMetricRecorder struct{}

// NewNoOpMetricRecorder creates a new instance of NoOpMetricRecorder.
func NewNoOpMetricRecorder() MetricRecorder {
	return &NoOpMetricRecorder{}
}

// RecordJobStart does nothing.
func (r *NoOpMetricRecorder) RecordJobStart(ctx context.Context, execution *model.JobExecution) {}
// RecordJobEnd does nothing.
func (r *NoOpMetricRecorder) RecordJobEnd(ctx context.Context, execution *model.JobExecution)   {}
// RecordStepStart does nothing.
func (r *NoOpMetricRecorder) RecordStepStart(ctx context.Context, execution *model.StepExecution) {}
// RecordStepEnd does nothing.
func (r *NoOpMetricRecorder) RecordStepEnd(ctx context.Context, execution *model.StepExecution)   {}
// RecordItemRead does nothing.
func (r *NoOpMetricRecorder) RecordItemRead(ctx context.Context, stepName string)                {}
// RecordItemProcess does nothing.
func (r *NoOpMetricRecorder) RecordItemProcess(ctx context.Context, stepName string)             {}
// RecordItemWrite does nothing.
func (r *NoOpMetricRecorder) RecordItemWrite(ctx context.Context, stepName string, count int)    {}
// RecordItemSkip does nothing.
func (r *NoOpMetricRecorder) RecordItemSkip(ctx context.Context, stepName string, reason string) {}
// RecordItemRetry does nothing.
func (r *NoOpMetricRecorder) RecordItemRetry(ctx context.Context, stepName string, reason string) {}
// RecordChunkCommit does nothing.
func (r *NoOpMetricRecorder) RecordChunkCommit(ctx context.Context, stepName string, count int)  {}
// RecordDuration does nothing.
func (r *NoOpMetricRecorder) RecordDuration(ctx context.Context, name string, duration time.Duration, tags map[string]string) {
}

var _ MetricRecorder = (*NoOpMetricRecorder)(nil)

// --- NoOpTracer ---

// NoOpTracer is an implementation of Tracer that does nothing.
type NoOpTracer struct{}

// NewNoOpTracer creates a new instance of NoOpTracer.
func NewNoOpTracer() Tracer {
	return &NoOpTracer{}
}

// StartJobSpan starts a Span for a JobExecution.
func (t *NoOpTracer) StartJobSpan(ctx context.Context, execution *model.JobExecution) (context.Context, func()) {
	return ctx, func() {}
}

// StartStepSpan starts a Span for a StepExecution.
func (t *NoOpTracer) StartStepSpan(ctx context.Context, execution *model.StepExecution) (context.Context, func()) {
	return ctx, func() {}
}

// RecordError records an error in the current Span.
func (t *NoOpTracer) RecordError(ctx context.Context, module string, err error) {}

// RecordEvent records an event in the current Span.
func (t *NoOpTracer) RecordEvent(ctx context.Context, name string, attributes map[string]interface{}) {}

var _ Tracer = (*NoOpTracer)(nil)
