package repository

import (
	"context"
	"errors"
	model "surfin/pkg/batch/core/domain/model"
	"surfin/pkg/batch/support/util/exception"
)

// JobInstance defines operations for persisting and retrieving job instance metadata.
type JobInstance interface {
	// SaveJobInstance persists a new JobInstance.
	SaveJobInstance(ctx context.Context, instance *model.JobInstance) error

	// UpdateJobInstance updates the state of an existing JobInstance.
	UpdateJobInstance(ctx context.Context, instance *model.JobInstance) error

	// FindJobInstanceByID finds a JobInstance by its ID.
	FindJobInstanceByID(ctx context.Context, id string) (*model.JobInstance, error)

	// FindJobInstanceByJobNameAndParameters finds a JobInstance by job name and exact parameters.
	FindJobInstanceByJobNameAndParameters(ctx context.Context, jobName string, params model.JobParameters) (*model.JobInstance, error)

	// FindJobInstancesByJobNameAndPartialParameters finds JobInstances matching job name and partial parameters.
	FindJobInstancesByJobNameAndPartialParameters(ctx context.Context, jobName string, partialParams model.JobParameters) ([]*model.JobInstance, error)

	// GetJobInstanceCount returns the count of JobInstances for a given job name.
	GetJobInstanceCount(ctx context.Context, jobName string) (int, error)

	// GetJobNames returns a list of all distinct job names.
	GetJobNames(ctx context.Context) ([]string, error)
}

// ErrJobInstanceNotFound is returned when JobInstance is not found.
var ErrJobInstanceNotFound = errors.New("job instance not found")

func init() {
	// Register the error type in the registry upon framework startup.
	exception.RegisterErrorType("ErrJobInstanceNotFound", ErrJobInstanceNotFound)
}
