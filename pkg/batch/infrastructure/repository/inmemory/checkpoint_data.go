package inmemory

import (
	"context"

	"github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	"github.com/tigerroll/surfin/pkg/batch/core/domain/repository"
)

// SaveCheckpointData persists checkpoint data.
// Due to the in-memory nature, it overwrites any existing data for the same key.
func (r *InMemoryJobRepository) SaveCheckpointData(ctx context.Context, data *model.CheckpointData) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := data.StepExecutionID
	// Deep copy to prevent external modification of internal state.
	clonedData := *data
	r.checkpointData[key] = &clonedData
	return nil
}

// FindCheckpointData finds checkpoint data based on the given StepExecutionID.
// It returns (nil, nil) if no checkpoint data is found for the specified key.
func (r *InMemoryJobRepository) FindCheckpointData(ctx context.Context, stepExecutionID string) (*model.CheckpointData, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	key := stepExecutionID
	data, ok := r.checkpointData[key]
	if !ok {
		return nil, repository.ErrCheckpointDataNotFound
	}
	// Deep copy to prevent external modification of internal state.
	clonedData := *data
	return &clonedData, nil
}
