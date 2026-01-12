package inmemory

import (
	"context"
	"fmt"

	"github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	"github.com/tigerroll/surfin/pkg/batch/core/domain/repository"
)

// SaveStepExecution persists a new StepExecution.
// It returns an error if a StepExecution with the same ID already exists.
func (r *InMemoryJobRepository) SaveStepExecution(ctx context.Context, stepExecution *model.StepExecution) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.stepExecutions[stepExecution.ID]; exists {
		return fmt.Errorf("StepExecution with ID %s already exists", stepExecution.ID)
	}
	r.stepExecutions[stepExecution.ID] = stepExecution
	return nil
}

// UpdateStepExecution updates an existing StepExecution.
// It returns an error if the StepExecution with the given ID is not found.
func (r *InMemoryJobRepository) UpdateStepExecution(ctx context.Context, stepExecution *model.StepExecution) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.stepExecutions[stepExecution.ID]; !exists {
		return fmt.Errorf("StepExecution with ID %s not found for update", stepExecution.ID)
	}
	r.stepExecutions[stepExecution.ID] = stepExecution
	return nil
}

// FindStepExecutionByID finds a StepExecution by its ID.
// It returns an error if the StepExecution is not found.
func (r *InMemoryJobRepository) FindStepExecutionByID(ctx context.Context, id string) (*model.StepExecution, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stepExecution, ok := r.stepExecutions[id]
	if !ok {
		return nil, repository.ErrStepExecutionNotFound
	}
	// Deep copy to prevent external modification of internal state
	clonedStepExecution := *stepExecution
	return &clonedStepExecution, nil
}
