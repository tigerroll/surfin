package runner

import (
	"context"
	"time"

	port "github.com/tigerroll/surfin/pkg/batch/core/application/port"
	model "github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	repository "github.com/tigerroll/surfin/pkg/batch/core/domain/repository"
	metrics "github.com/tigerroll/surfin/pkg/batch/core/metrics"
	logger "github.com/tigerroll/surfin/pkg/batch/support/util/logger"
)

// SimpleJobRunner is an implementation of port.JobRunner that executes the flow by calling the Job's Run method.
type SimpleJobRunner struct {
	jobRepository repository.JobRepository
	stepExecutor  port.StepExecutor // Currently unused, but may be needed in the future
	tracer        metrics.Tracer
}

// NewFlowJobRunner creates an instance of SimpleJobRunner.
// Although named FlowJobRunner, it is essentially a SimpleJobRunner.
func NewFlowJobRunner(
	repo repository.JobRepository,
	executor port.StepExecutor,
	tracer metrics.Tracer,
) port.JobRunner {
	return &SimpleJobRunner{
		jobRepository: repo,
		stepExecutor:  executor,
		tracer:        tracer,
	}
}

// Run executes the Job's flow.
func (r *SimpleJobRunner) Run(ctx context.Context, jobInstance port.Job, jobExecution *model.JobExecution, flowDef *model.FlowDefinition) { // flowDef is currently unused but kept for future extensibility.
	// Update JobExecution status to STARTED
	if jobExecution.Status == model.BatchStatusStarting || jobExecution.Status == model.BatchStatusRestarting {
		jobExecution.MarkAsStarted()
		if err := r.jobRepository.UpdateJobExecution(ctx, jobExecution); err != nil {
			logger.Errorf("JobRunner: Failed to update JobExecution (ID: %s) status to STARTED: %v", jobExecution.ID, err)
			// Continue processing as a fatal error
		}
	}

	// Execute the Job's Run method
	err := jobInstance.Run(ctx, jobExecution, jobExecution.Parameters)

	// Post-execution processing
	if err != nil {
		// MarkAsFailed/Stopped should have already been called inside JobExecution.Run, but check just in case
		if jobExecution.Status.IsFinished() {
			logger.Warnf("JobRunner: Job execution finished with error, but status already set to %s.", jobExecution.Status)
		} else {
			// Mark as FAILED for unexpected errors
			jobExecution.MarkAsFailed(err)
		}
	} else if !jobExecution.Status.IsFinished() {
		// If Run finishes without error but status is not completed (should not happen normally)
		jobExecution.MarkAsCompleted()
	}

	// Final persistence of JobExecution
	if updateErr := r.jobRepository.UpdateJobExecution(ctx, jobExecution); updateErr != nil {
		logger.Errorf("JobRunner: Failed to update final JobExecution (ID: %s) state: %v", jobExecution.ID, updateErr)
		// Do not add persistence errors to JobExecution Failures (as it's a metadata DB issue)
	}

	// If JobExecution EndTime is not set, set it
	if jobExecution.EndTime == nil {
		now := time.Now()
		jobExecution.EndTime = &now
	}
}

var _ port.JobRunner = (*SimpleJobRunner)(nil)
