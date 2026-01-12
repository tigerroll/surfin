package repository

import (
	"errors"
	"context"
	model "surfin/pkg/batch/core/domain/model"
)

// CheckpointDataRepository defines operations for persisting and retrieving checkpoint data.
type CheckpointDataRepository interface {
	// SaveCheckpointData persists or updates the ExecutionContext associated with the specified StepExecutionID.
	SaveCheckpointData(ctx context.Context, data *model.CheckpointData) error

	// FindCheckpointData retrieves the ExecutionContext associated with the specified StepExecutionID.
	FindCheckpointData(ctx context.Context, stepExecutionID string) (*model.CheckpointData, error)
}

// ErrCheckpointDataNotFound is returned when checkpoint data is not found.
var ErrCheckpointDataNotFound = errors.New("checkpoint data not found")

// JobRepository is the interface for persisting and managing batch execution metadata, similar to Spring Batch's JobRepository.
// It embeds multiple smaller repository interfaces to separate concerns.
type JobRepository interface {
	JobInstance   // Embeds the JobInstance interface (definition in job_instance.go)
	JobExecution  // Embeds the JobExecution interface (definition in job_execution.go)
	StepExecution // Embeds the StepExecution interface (definition in step_execution.go)
	CheckpointDataRepository // Embeds checkpoint data operations

	// Close releases resources (such as database connections) used by the repository.
	Close() error
}
