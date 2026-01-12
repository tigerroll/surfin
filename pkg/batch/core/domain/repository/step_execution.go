package repository

import (
	"context"
	"errors"

	model "github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	"github.com/tigerroll/surfin/pkg/batch/support/util/exception"
)

// ErrStepExecutionNotFound is the error returned when StepExecution is not found.
var ErrStepExecutionNotFound = errors.New("step execution not found")

func init() {
	// Register the error type in the registry upon framework startup.
	exception.RegisterErrorType("ErrStepExecutionNotFound", ErrStepExecutionNotFound)
}

type StepExecution interface {
	// SaveStepExecution persists a new StepExecution.
	SaveStepExecution(ctx context.Context, stepExecution *model.StepExecution) error

	// UpdateStepExecution updates the state of an existing StepExecution.
	UpdateStepExecution(ctx context.Context, stepExecution *model.StepExecution) error

	// FindStepExecutionByID finds a StepExecution by its ID.
	FindStepExecutionByID(ctx context.Context, executionID string) (*model.StepExecution, error)
}
