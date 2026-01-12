package repository

import (
	"context"
	"errors"

	model "github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	"github.com/tigerroll/surfin/pkg/batch/support/util/exception"
)

// ErrJobExecutionNotFound is the error returned when a JobExecution is not found.
var ErrJobExecutionNotFound = errors.New("job execution not found")

func init() {
	// Register the error type in the registry upon framework startup
	exception.RegisterErrorType("ErrJobExecutionNotFound", ErrJobExecutionNotFound)
}

type JobExecution interface {
	// SaveJobExecution persists a new JobExecution
	SaveJobExecution(ctx context.Context, jobExecution *model.JobExecution) error

	// UpdateJobExecution updates the state of an existing JobExecution
	UpdateJobExecution(ctx context.Context, jobExecution *model.JobExecution) error

	// FindJobExecutionByID finds a JobExecution by its ID
	// It is expected to load associated StepExecutions as well
	FindJobExecutionByID(ctx context.Context, executionID string) (*model.JobExecution, error)

	// FindLatestRestartableJobExecution finds the latest JobExecution for a given JobInstance that is restartable
	FindLatestRestartableJobExecution(ctx context.Context, jobInstanceID string) (*model.JobExecution, error)

	// FindJobExecutionsByJobInstance finds all JobExecutions associated with the specified JobInstance
	FindJobExecutionsByJobInstance(ctx context.Context, jobInstance *model.JobInstance) ([]*model.JobExecution, error)
}
