package usecase

import (
	"context"
	"errors"
	"fmt"
	"sync"

	port "github.com/tigerroll/surfin/pkg/batch/core/application/port"
	support "github.com/tigerroll/surfin/pkg/batch/core/config/support"
	model "github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	repository "github.com/tigerroll/surfin/pkg/batch/core/domain/repository"
	exception "github.com/tigerroll/surfin/pkg/batch/support/util/exception"
	logger "github.com/tigerroll/surfin/pkg/batch/support/util/logger"
)

// JobLauncher is an interface for launching a Job with JobParameters.
// It is equivalent to Spring Batch's JobLauncher.
type JobLauncher interface {
	// Launch starts the specified Job with JobParameters.
	// It returns the launched JobExecution instance.
	// The error returned here indicates an error in the launch process itself, not an error in the job's execution.
	Launch(ctx context.Context, jobName string, params model.JobParameters) (*model.JobExecution, error)
}

// SimpleJobLauncher implements JobLauncher for local execution.
type SimpleJobLauncher struct {
	jobRepository repository.JobRepository
	jobFactory    *support.JobFactory
	jobRunner     port.JobRunner // FIX: Added jobRunner.
	// activeJobCancellations holds the cancel functions for running jobs.
	activeJobCancellations map[string]context.CancelFunc // FIX: Added field.
	mu                     sync.Mutex                    // FIX: Added field.
}

// NewSimpleJobLauncher creates a new SimpleJobLauncher.
func NewSimpleJobLauncher(
	repo repository.JobRepository,
	factory *support.JobFactory,
	runner port.JobRunner, // FIX: Added runner.
) *SimpleJobLauncher {
	return &SimpleJobLauncher{
		jobRepository:          repo,
		jobFactory:             factory,
		jobRunner:              runner,
		activeJobCancellations: make(map[string]context.CancelFunc), // FIX: Added field.
	}
}

// RegisterCancelFunc registers the cancel function for a running job execution.
func (l *SimpleJobLauncher) RegisterCancelFunc(executionID string, cancelFunc context.CancelFunc) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.activeJobCancellations[executionID] = cancelFunc
	logger.Debugf("Registered CancelFunc for JobExecution (ID: %s).", executionID)
}

// UnregisterCancelFunc unregisters the cancel function for a running job execution.
func (l *SimpleJobLauncher) UnregisterCancelFunc(executionID string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, ok := l.activeJobCancellations[executionID]; ok { // FIX: Changed cancelFunc to _.
		// cancelFunc() // Commented out as JobRunner calls it upon completion.
		delete(l.activeJobCancellations, executionID)
		logger.Debugf("Unregistered CancelFunc for JobExecution (ID: %s).", executionID)
	}
}

// GetCancelFunc retrieves the cancel function for the specified JobExecution ID.
func (l *SimpleJobLauncher) GetCancelFunc(executionID string) (context.CancelFunc, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	cancelFunc, ok := l.activeJobCancellations[executionID]
	return cancelFunc, ok
}

// Launch launches a job execution.
func (l *SimpleJobLauncher) Launch(ctx context.Context, jobName string, jobParameters model.JobParameters) (*model.JobExecution, error) {
	const op = "SimpleJobLauncher.Launch"
	logger.Infof("Launching Job '%s' using JobLauncher. Parameters: %s", jobName, jobParameters.String())

	var jobExecution *model.JobExecution
	var jobInstance *model.JobInstance

	// 1. Retrieve Job Definition
	jobInstanceDef, err := l.jobFactory.CreateJob(jobName)
	if err != nil {
		return nil, exception.NewBatchError(op, fmt.Sprintf("Failed to create job definition for '%s'", jobName), err, false, false)
	}

	// Validate JobParameters
	if err := jobInstanceDef.ValidateParameters(jobParameters); err != nil {
		logger.Errorf("Job '%s': JobParameters validation failed: %v", jobName, err)
		return nil, exception.NewBatchError("job_launcher", "JobParameters validation error", err, false, false)
	}

	// 2. Find/Create JobInstance

	// T_RESTART_1: Retrieve JobParametersIncrementer
	incrementer := l.jobFactory.GetJobParametersIncrementer(jobName)

	// Search for existing JobInstance
	existingInstance, err := l.jobRepository.FindJobInstanceByJobNameAndParameters(ctx, jobName, jobParameters)

	if err != nil && !errors.Is(err, repository.ErrJobInstanceNotFound) {
		return nil, exception.NewBatchError(op, "Failed to search for existing JobInstance", err, false, false)
	}

	if existingInstance != nil {
		// If an existing JobInstance is found

		// T_RESTART_1: Find the latest restartable JobExecution
		latestRestartableExecution, restartErr := l.jobRepository.FindLatestRestartableJobExecution(ctx, existingInstance.ID)

		// Error during JobExecution search
		if restartErr != nil && !errors.Is(restartErr, repository.ErrJobExecutionNotFound) {
			logger.Errorf("Failed to search for restartable JobExecution for JobInstance (ID: %s): %v", existingInstance.ID, restartErr)
			return nil, exception.NewBatchError("job_launcher", "Launch processing error: Failed to search for restartable JobExecution", restartErr, false, false)
		}

		if latestRestartableExecution != nil && latestRestartableExecution.Status.IsFinished() {
			// --- Restart Path ---

			// Update the status of the existing JobExecution to ABANDONED
			latestRestartableExecution.MarkAsAbandoned()
			if err := l.jobRepository.UpdateJobExecution(ctx, latestRestartableExecution); err != nil {
				logger.Warnf("Failed to update existing JobExecution (ID: %s) to ABANDONED: %v", latestRestartableExecution.ID, err)
			}
			logger.Infof("Updated status of existing JobExecution (ID: %s) to ABANDONED.", latestRestartableExecution.ID)

			// Create a new JobExecution (new ID, existing parameters, copied EC)
			newExecution := model.NewJobExecution(existingInstance.ID, jobName, latestRestartableExecution.Parameters)
			newExecution.ExecutionContext = latestRestartableExecution.ExecutionContext.Copy()
			newExecution.RestartCount = latestRestartableExecution.RestartCount + 1
			newExecution.CurrentStepName = latestRestartableExecution.CurrentStepName
			newExecution.Status = model.BatchStatusRestarting

			// Copy StepExecutions
			for _, prevStepExecution := range latestRestartableExecution.StepExecutions {
				newStepExecution := prevStepExecution.CopyForRestart(newExecution.ID)
				newExecution.AddStepExecution(newStepExecution)
			}

			jobExecution = newExecution
			jobInstance = existingInstance

			logger.Infof("Created restart JobExecution (ID: %s). Restart Count: %d", jobExecution.ID, jobExecution.RestartCount)

		} else if latestRestartableExecution != nil && !latestRestartableExecution.Status.IsFinished() {
			// If a running JobExecution is found
			err := exception.NewBatchErrorf("job_launcher", "A running JobExecution (ID: %s, Status: %s) already exists for JobInstance (ID: %s). Concurrent restart is not allowed.", latestRestartableExecution.ID, latestRestartableExecution.Status, existingInstance.ID)
			logger.Errorf("%v", err)
			return nil, err
		} else {
			// If an existing JobInstance exists but no restartable JobExecution (new execution)
			jobExecution = model.NewJobExecution(existingInstance.ID, jobName, existingInstance.Parameters)
			jobInstance = existingInstance
			logger.Infof("Creating new JobExecution for existing JobInstance (ID: %s).", existingInstance.ID)
		}

	} else {
		// --- New Execution Path ---

		// 1. Apply JobParametersIncrementer
		if incrementer != nil {
			jobParameters = incrementer.GetNext(jobParameters)
			logger.Infof("Generated new JobParameters using JobParametersIncrementer: %s", jobParameters.String())
		}

		// 2. Create a new JobInstance
		jobInstance = model.NewJobInstance(jobName, jobParameters)
		if err := l.jobRepository.SaveJobInstance(ctx, jobInstance); err != nil {
			return nil, exception.NewBatchError(op, fmt.Sprintf("Failed to save new JobInstance for '%s'", jobName), err, false, false)
		}
		logger.Infof("Created and saved new JobInstance (ID: %s, JobName: %s).", jobInstance.ID, jobName)

		// 3. Create a new JobExecution
		jobExecution = model.NewJobExecution(jobInstance.ID, jobName, jobInstance.Parameters)
	}

	// 3. Initial persistence of JobExecution and start of asynchronous execution

	// Create Context with CancelFunc and set it in JobExecution
	jobCtx, cancel := context.WithCancel(ctx)
	jobExecution.CancelFunc = cancel
	l.RegisterCancelFunc(jobExecution.ID, cancel)

	logger.Infof("Starting launch process for Job '%s' (Execution ID: %s, Job Instance ID: %s).", jobName, jobExecution.ID, jobInstance.ID)

	// Save JobExecution to JobRepository (Initial Save)
	err = l.jobRepository.SaveJobExecution(jobCtx, jobExecution)
	if err != nil {
		l.UnregisterCancelFunc(jobExecution.ID)
		logger.Errorf("Failed to persist JobExecution (ID: %s) initially: %v", jobExecution.ID, err)
		return jobExecution, exception.NewBatchError("job_launcher", "Launch processing error: Failed to save JobExecution initially", err, false, false)
	}
	logger.Debugf("Initially saved JobExecution (ID: %s) to JobRepository (Status: %s).", jobExecution.ID, jobExecution.Status)

	// 4. Start JobRunner asynchronously
	go func() {
		// The JobRunner updates the JobExecution status to STARTED and begins execution.
		l.jobRunner.Run(jobCtx, jobInstanceDef, jobExecution, jobInstanceDef.GetFlow())
	}()

	// Launch returns the JobExecution and exits.
	return jobExecution, nil
}
