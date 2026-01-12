package runner

import (
	port "surfin/pkg/batch/core/application/port"
	repository "surfin/pkg/batch/core/domain/repository"
	metrics "surfin/pkg/batch/core/metrics"
	"go.uber.org/fx"
)

// FlowJobRunnerParams defines dependencies for FlowJobRunner.
type FlowJobRunnerParams struct {
	fx.In
	JobRepository repository.JobRepository
	StepExecutor  port.StepExecutor
	Tracer        metrics.Tracer
}

// NewJobRunner provides the concrete JobRunner implementation (FlowJobRunner).
func NewJobRunner(p FlowJobRunnerParams) port.JobRunner {
	return NewFlowJobRunner(p.JobRepository, p.StepExecutor, p.Tracer)
}

// Module provides the JobRunner implementation.
var Module = fx.Options(
	fx.Provide(fx.Annotate(
		NewJobRunner,
		fx.As(new(port.JobRunner)),
	)),
)
