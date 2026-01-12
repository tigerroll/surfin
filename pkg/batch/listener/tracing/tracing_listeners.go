package tracing

import (
	"context"

	port "surfin/pkg/batch/core/application/port"
	model "surfin/pkg/batch/core/domain/model"
	"surfin/pkg/batch/core/metrics"
)

// TracingJobListener manages tracing spans for JobExecution.
type TracingJobListener struct {
	tracer metrics.Tracer
	// JobExecutionID -> Span End Function
	jobSpanEndFuncs map[string]func()
}

func NewTracingJobListener(tracer metrics.Tracer) port.JobExecutionListener {
	return &TracingJobListener{
		tracer: tracer,
		jobSpanEndFuncs: make(map[string]func()),
	}
}

func (l *TracingJobListener) BeforeJob(ctx context.Context, jobExecution *model.JobExecution) {
	// StartJobSpan returns a new context and an end function.
	_, endFunc := l.tracer.StartJobSpan(ctx, jobExecution)
	l.jobSpanEndFuncs[jobExecution.ID] = endFunc
}

func (l *TracingJobListener) AfterJob(ctx context.Context, jobExecution *model.JobExecution) {
	if endFunc, ok := l.jobSpanEndFuncs[jobExecution.ID]; ok {
		endFunc() // Ends the span.
		delete(l.jobSpanEndFuncs, jobExecution.ID)
	}
}

var _ port.JobExecutionListener = (*TracingJobListener)(nil)

// TracingStepListener manages tracing spans for StepExecution.
type TracingStepListener struct {
	tracer metrics.Tracer
	// StepExecutionID -> Span End Function
	stepSpanEndFuncs map[string]func()
}

func NewTracingStepListener(tracer metrics.Tracer) port.StepExecutionListener {
	return &TracingStepListener{
		tracer: tracer,
		stepSpanEndFuncs: make(map[string]func()),
	}
}

func (l *TracingStepListener) BeforeStep(ctx context.Context, stepExecution *model.StepExecution) {
	// StartStepSpan returns a new context and an end function.
	_, endFunc := l.tracer.StartStepSpan(ctx, stepExecution)
	l.stepSpanEndFuncs[stepExecution.ID] = endFunc
}

func (l *TracingStepListener) AfterStep(ctx context.Context, stepExecution *model.StepExecution) {
	if endFunc, ok := l.stepSpanEndFuncs[stepExecution.ID]; ok {
		endFunc() // Ends the span.
		delete(l.stepSpanEndFuncs, stepExecution.ID)
	}
}

var _ port.StepExecutionListener = (*TracingStepListener)(nil)
