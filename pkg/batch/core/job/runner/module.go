package runner

import (
	port "surfin/pkg/batch/core/application/port"
	repository "surfin/pkg/batch/core/domain/repository"
	metrics "surfin/pkg/batch/core/metrics"
	"go.uber.org/fx"
)

// SimpleJobRunnerParams defines dependencies for SimpleJobRunner.
type SimpleJobRunnerParams struct {
	fx.In
	JobRepository repository.JobRepository
	StepExecutor  port.StepExecutor
	Tracer        metrics.Tracer
}

// NewJobRunner provides the concrete JobRunner implementation (SimpleJobRunner).
func NewJobRunner(p SimpleJobRunnerParams) port.JobRunner {
	return NewFlowJobRunner(p.JobRepository, p.StepExecutor, p.Tracer)
}

// Module provides the JobRunner implementation.
var Module = fx.Options(
	fx.Provide(fx.Annotate(
		NewJobRunner,
		fx.As(new(port.JobRunner)),
	)),
)
