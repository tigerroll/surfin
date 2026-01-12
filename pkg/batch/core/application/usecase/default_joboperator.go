package usecase

import (
	"context"
	"fmt"
	"time"

	support "github.com/tigerroll/surfin/pkg/batch/core/config/support"
	model "github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	jobRepository "github.com/tigerroll/surfin/pkg/batch/core/domain/repository"
	exception "github.com/tigerroll/surfin/pkg/batch/support/util/exception"
	logger "github.com/tigerroll/surfin/pkg/batch/support/util/logger"
)

// DefaultJobOperator is the default implementation of the JobOperator interface.
// It manages batch metadata and orchestrates job execution using a JobRepository.
type DefaultJobOperator struct {
	jobRepository jobRepository.JobRepository
	jobFactory    *support.JobFactory
	jobLauncher   *SimpleJobLauncher // Concrete implementation of usecase.JobLauncher
	jobExplorer   JobExplorer        // usecase.JobExplorer
}

// Verify that DefaultJobOperator implements the JobOperator interface.
var _ JobOperator = (*DefaultJobOperator)(nil)

// NewDefaultJobOperator creates a new instance of DefaultJobOperator.
// It receives implementations of JobRepository and JobFactory.
func NewDefaultJobOperator(jobRepository jobRepository.JobRepository, jobFactory *support.JobFactory, jobExplorer JobExplorer) *DefaultJobOperator {
	return &DefaultJobOperator{
		jobRepository: jobRepository,
		jobFactory:    jobFactory,
		jobExplorer:   jobExplorer,
		// jobLauncher is expected to be set later via SetJobLauncher to avoid circular dependencies.
	}
}

// SetJobLauncher sets the reference to JobLauncher, typically done after construction to avoid circular dependencies.
func (o *DefaultJobOperator) SetJobLauncher(launcher *SimpleJobLauncher) {
	o.jobLauncher = launcher
}

// Restart restarts the specified JobExecution.
// This is an implementation of the JobOperator interface.
func (o *DefaultJobOperator) Restart(ctx context.Context, executionID string) (*model.JobExecution, error) {
	logger.Infof("JobOperator: Restart method called. Execution ID: %s", executionID)

	if o.jobLauncher == nil {
		return nil, exception.NewBatchErrorf("job_operator", "JobLauncher is not set. Cannot perform Restart operation.")
	}

	// 1. Load the previous JobExecution
	prevJobExecution, err := o.jobRepository.FindJobExecutionByID(ctx, executionID)
	if err != nil {
		return nil, exception.NewBatchError("job_operator", fmt.Sprintf("Restart processing error: Failed to load JobExecution (ID: %s)", executionID), err, false, false)
	}
	if prevJobExecution == nil {
		return nil, exception.NewBatchErrorf("job_operator", "Restart processing error: JobExecution (ID: %s) not found", executionID)
	}

	// 2. Check if it's in a restartable state
	if prevJobExecution.Status != model.BatchStatusFailed && prevJobExecution.Status != model.BatchStatusStopped && prevJobExecution.Status != model.BatchStatusAbandoned {
		return nil, exception.NewBatchErrorf("job_operator", "Restart processing error: JobExecution (ID: %s) is not in a restartable state (current status: %s)", executionID, prevJobExecution.Status)
	}
	logger.Infof("JobExecution (ID: %s) is in a restartable state (%s).", executionID, prevJobExecution.Status)

	// 3. Delegate restart processing by calling JobLauncher's Launch method.
	// Internally, Launch searches for the latest execution matching JobInstanceID and JobParameters, then performs the restart.
	// Launch creates and executes a JobExecution.
	newExecution, err := o.jobLauncher.Launch(ctx, prevJobExecution.JobName, prevJobExecution.Parameters)
	if err != nil {
		return nil, exception.NewBatchError("job_operator", fmt.Sprintf("Failed to restart job execution (ID: %s)", executionID), err, false, false)
	}

	logger.Infof("Restart of Job '%s' (Execution ID: %s) started. New execution ID: %s", prevJobExecution.JobName, executionID, newExecution.ID)
	return newExecution, nil
}

// Stop stops the specified JobExecution.
// This is an implementation of the JobOperator interface.
func (o *DefaultJobOperator) Stop(ctx context.Context, executionID string) error {
	logger.Infof("JobOperator: Stop method called. Execution ID: %s", executionID)

	if o.jobLauncher == nil {
		return exception.NewBatchErrorf("job_operator", "JobLauncher is not set. Cannot perform Stop operation.")
	}

	// 1. Load JobExecution
	jobExecution, err := o.jobRepository.FindJobExecutionByID(ctx, executionID)
	if err != nil {
		return exception.NewBatchError("job_operator", fmt.Sprintf("Stop processing error: Failed to load JobExecution (ID: %s)", executionID), err, false, false)
	}
	if jobExecution == nil {
		return exception.NewBatchErrorf("job_operator", "Stop processing error: JobExecution (ID: %s) not found", executionID)
	}

	// 2. Do not stop if already in a finished state
	if jobExecution.Status.IsFinished() {
		logger.Warnf("JobExecution (ID: %s) cannot be stopped as it is already in a finished state (%s).", executionID, jobExecution.Status)
		return exception.NewBatchErrorf("job_operator", "Stop processing error: JobExecution (ID: %s) is already in a finished state (%s)", executionID, jobExecution.Status)
	}

	// 3. Update JobExecution status to STOPPING
	if err := jobExecution.TransitionTo(model.BatchStatusStopping); err != nil {
		logger.Warnf("Failed to update JobExecution (ID: %s) status to STOPPING: %v", executionID, err)
		// Force set and continue processing
		jobExecution.Status = model.BatchStatusStopping
		jobExecution.LastUpdated = time.Now() // Update timestamp even if forced
	}
	if err := o.jobRepository.UpdateJobExecution(ctx, jobExecution); err != nil {
		logger.Errorf("Stop processing error: Failed to update JobExecution (ID: %s) status to STOPPING: %v", executionID, err)
		return exception.NewBatchError("job_operator", fmt.Sprintf("Stop processing error: Failed to update JobExecution (ID: %s) status", executionID), err, false, false)
	}
	logger.Infof("Updated JobExecution (ID: %s) status to STOPPING.", executionID)

	// 4. Call CancelFunc registered with JobLauncher
	cancelFunc, ok := o.jobLauncher.GetCancelFunc(executionID)
	if !ok {
		logger.Warnf("No CancelFunc found for JobExecution (ID: %s). The job may have already finished or is not registered with JobLauncher.", executionID)
		// Although a stop signal cannot be sent, the DB status has become STOPPING. While this might not always be an error, it is treated as an error here.
		return exception.NewBatchErrorf("job_operator", "Stop processing error: CancelFunc for JobExecution (ID: %s) not found", executionID)
	}
	cancelFunc() // Cancel Context to interrupt job execution

	logger.Infof("Sent stop signal for JobExecution (ID: %s).", executionID)
	return nil
}

// Abandon abandons the specified JobExecution.
// This is a stub implementation of the JobOperator interface.
func (o *DefaultJobOperator) Abandon(ctx context.Context, executionID string) error {
	logger.Infof("JobOperator: Abandon method called. Execution ID: %s", executionID)

	// 1. Load JobExecution
	jobExecution, err := o.jobRepository.FindJobExecutionByID(ctx, executionID)
	if err != nil {
		return exception.NewBatchError("job_operator", fmt.Sprintf("Abandon processing error: Failed to load JobExecution (ID: %s)", executionID), err, false, false)
	}
	if jobExecution == nil {
		return exception.NewBatchErrorf("job_operator", "Abandon processing error: JobExecution (ID: %s) not found", executionID)
	}

	// 2. Running or stopping jobs cannot be abandoned (JSR-352 compliant)
	if jobExecution.Status == model.BatchStatusStarted || jobExecution.Status == model.BatchStatusStopping || jobExecution.Status == model.BatchStatusRestarting {
		logger.Warnf("JobExecution (ID: %s) cannot be abandoned as it is running/stopping/restarting (current status: %s).", executionID, jobExecution.Status)
		return exception.NewBatchErrorf("job_operator", "Abandon processing error: JobExecution (ID: %s) cannot be abandoned as it is running/stopping/restarting (%s)", executionID, jobExecution.Status)
	}

	// Do nothing if already ABANDONED
	if jobExecution.Status == model.BatchStatusAbandoned {
		logger.Infof("JobExecution (ID: %s) is already in ABANDONED status.", executionID)
		return nil
	}

	// If already in a finished state but not ABANDONED (COMPLETED, FAILED, STOPPED, STOPPING_FAILED), it cannot be abandoned.
	if jobExecution.Status.IsFinished() {
		logger.Warnf("JobExecution (ID: %s) cannot be abandoned as it is already in a finished state (%s).", executionID, jobExecution.Status)
		return exception.NewBatchErrorf("job_operator", "Abandon processing error: JobExecution (ID: %s) is already in a finished state (%s)", executionID, jobExecution.Status)
	}

	// 3. Update JobExecution status to ABANDONED
	jobExecution.MarkAsAbandoned()
	logger.Infof("Updated JobExecution (ID: %s) status to ABANDONED.", executionID)

	// 4. Persist JobExecution
	err = o.jobRepository.UpdateJobExecution(ctx, jobExecution)
	if err != nil {
		return exception.NewBatchError("job_operator", fmt.Sprintf("Abandon processing error: Failed to update JobExecution (ID: %s) status", executionID), err, false, false)
	}

	// If a CancelFunc is registered with JobLauncher, unregister it.
	if o.jobLauncher != nil {
		o.jobLauncher.UnregisterCancelFunc(executionID)
	}

	logger.Infof("Successfully abandoned JobExecution (ID: %s).", executionID)
	return nil
}

// GetJobExecution retrieves a JobExecution by its ID.
// This is an implementation of the JobOperator interface.
func (o *DefaultJobOperator) GetJobExecution(ctx context.Context, executionID string) (*model.JobExecution, error) {
	logger.Infof("JobOperator: GetJobExecution method called. Execution ID: %s", executionID)
	return o.jobExplorer.GetJobExecution(ctx, executionID)
}

// GetJobExecutions retrieves all JobExecutions associated with the specified JobInstance.
// This is an implementation of the JobOperator interface.
func (o *DefaultJobOperator) GetJobExecutions(ctx context.Context, instanceID string) ([]*model.JobExecution, error) {
	logger.Infof("JobOperator: GetJobExecutions method called. Instance ID: %s", instanceID)
	return o.jobExplorer.GetJobExecutions(ctx, instanceID)
}

// GetLastJobExecution retrieves the latest JobExecution for a given JobInstance.
// This is an implementation of the JobOperator interface.
func (o *DefaultJobOperator) GetLastJobExecution(ctx context.Context, instanceID string) (*model.JobExecution, error) {
	logger.Infof("JobOperator: GetLastJobExecution method called. Instance ID: %s", instanceID)
	return o.jobExplorer.GetLastJobExecution(ctx, instanceID)
}

// GetJobInstance retrieves a JobInstance by its ID.
// This is an implementation of the JobOperator interface.
func (o *DefaultJobOperator) GetJobInstance(ctx context.Context, instanceID string) (*model.JobInstance, error) {
	logger.Infof("JobOperator: GetJobInstance method called. Instance ID: %s", instanceID)
	return o.jobExplorer.GetJobInstance(ctx, instanceID)
}

// GetJobInstances searches for JobInstances matching the specified job name and parameters.
// This is an implementation of the JobOperator interface.
// Returns a list as JSR352 may return multiple instances.
func (o *DefaultJobOperator) GetJobInstances(ctx context.Context, jobName string, params model.JobParameters) ([]*model.JobInstance, error) {
	logger.Infof("JobOperator: GetJobInstances method called. Job Name: %s, Parameters: %+v", jobName, params)
	return o.jobExplorer.GetJobInstances(ctx, jobName, params)
}

// GetJobNames retrieves all registered job names.
// This is an implementation of the JobOperator interface.
func (o *DefaultJobOperator) GetJobNames(ctx context.Context) ([]string, error) {
	logger.Infof("JobOperator: GetJobNames method called.")
	return o.jobExplorer.GetJobNames(ctx)
}

// GetParameters retrieves the JobParameters for the specified JobExecution.
// This is an implementation of the JobOperator interface.
func (o *DefaultJobOperator) GetParameters(ctx context.Context, executionID string) (model.JobParameters, error) {
	logger.Infof("JobOperator: GetParameters method called. Execution ID: %s", executionID)
	return o.jobExplorer.GetParameters(ctx, executionID)
}
