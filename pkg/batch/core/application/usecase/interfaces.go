package usecase

import (
	"context"

	model "surfin/pkg/batch/core/domain/model"
)

// JobOperator is an interface for performing operations on running jobs (e.g., restart, stop, abandon).
// It is equivalent to Spring Batch's JobOperator.
type JobOperator interface {
	// Restart restarts the specified JobExecution.
	// It returns a new JobExecution instance.
	Restart(ctx context.Context, executionID string) (*model.JobExecution, error)

	// Stop stops the specified JobExecution.
	// Stopping may occur asynchronously.
	Stop(ctx context.Context, executionID string) error

	// Abandon abandons the specified JobExecution.
	// An abandoned job cannot be restarted.
	Abandon(ctx context.Context, executionID string) error
}

// JobExplorer is an interface for querying batch metadata (JobInstance, JobExecution, StepExecution).
// It is equivalent to Spring Batch's JobExplorer.
type JobExplorer interface {
	// GetJobExecution retrieves a JobExecution by its ID.
	GetJobExecution(ctx context.Context, executionID string) (*model.JobExecution, error)

	// GetJobExecutions retrieves all JobExecutions associated with the specified JobInstance.
	GetJobExecutions(ctx context.Context, instanceID string) ([]*model.JobExecution, error)

	// GetLastJobExecution retrieves the latest JobExecution for a given JobInstance.
	GetLastJobExecution(ctx context.Context, instanceID string) (*model.JobExecution, error)

	// GetJobInstance retrieves a JobInstance by its ID.
	GetJobInstance(ctx context.Context, instanceID string) (*model.JobInstance, error)

	// GetJobInstances searches for JobInstances matching the specified job name and parameters.
	GetJobInstances(ctx context.Context, jobName string, params model.JobParameters) ([]*model.JobInstance, error)

	// GetJobNames retrieves all registered job names.
	GetJobNames(ctx context.Context) ([]string, error)

	// GetParameters retrieves the JobParameters for the specified JobExecution.
	GetParameters(ctx context.Context, executionID string) (model.JobParameters, error)
}
