package partition

import (
	"context"
	"fmt"
	core "surfin/pkg/batch/core/application/port"
	model "surfin/pkg/batch/core/domain/model"
	metrics "surfin/pkg/batch/core/metrics"
	logger "surfin/pkg/batch/support/util/logger"
)

// RemoteStepExecutor is an implementation of StepExecutor that delegates Step execution
// to a remote environment (e.g., Surfin Bird).
type RemoteStepExecutor struct {
	submitter core.RemoteJobSubmitter
	tracer    metrics.Tracer
}

// NewRemoteStepExecutor creates a new instance of RemoteStepExecutor.
func NewRemoteStepExecutor(submitter core.RemoteJobSubmitter, tracer metrics.Tracer) *RemoteStepExecutor {
	return &RemoteStepExecutor{
		submitter: submitter,
		tracer:    tracer,
	}
}

// ExecuteStep delegates the execution of the Step to the remote submitter and awaits completion.
func (e *RemoteStepExecutor) ExecuteStep(ctx context.Context, step core.Step, jobExecution *model.JobExecution, stepExecution *model.StepExecution) (*model.StepExecution, error) {
	logger.Infof("Remote Step Executor: Submitting StepExecution (ID: %s) to the remote execution engine.", stepExecution.ID)

	stepExecution.JobExecution = jobExecution

	// Submit the remote job
	e.tracer.RecordEvent(ctx, "remote_submission_start", map[string]interface{}{"step.name": stepExecution.StepName})
	remoteJobID, err := e.submitter.SubmitWorkerJob(ctx, jobExecution, stepExecution, step)
	if err != nil {
		e.tracer.RecordError(ctx, "remote_executor", err)
		return stepExecution, fmt.Errorf("failed to submit remote worker: %w", err)
	}
	e.tracer.RecordEvent(ctx, "remote_submission_success", map[string]interface{}{"remote.job.id": remoteJobID})

	logger.Debugf("Remote Step Executor: Successfully submitted remote job (ID: %s). Waiting for completion.", remoteJobID)

	// Await completion
	e.tracer.RecordEvent(ctx, "await_completion_start", map[string]interface{}{"remote.job.id": remoteJobID})
	if err := e.submitter.AwaitCompletion(ctx, remoteJobID, stepExecution); err != nil {
		e.tracer.RecordError(ctx, "remote_executor", err)
		return stepExecution, fmt.Errorf("an error occurred while waiting for remote worker completion: %w", err)
	}
	e.tracer.RecordEvent(ctx, "await_completion_end", map[string]interface{}{"status": stepExecution.Status.String()})

	logger.Infof("Remote Step Executor: Remote StepExecution (ID: %s) completed.", stepExecution.ID)

	return stepExecution, nil
}
