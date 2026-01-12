// Package inmemory provides an in-memory implementation of the JobRepository interface.
// It stores all job-related data in maps within memory, suitable for testing and
// scenarios where persistence is not required.
package inmemory

import (
	"sync"

	"surfin/pkg/batch/core/domain/model"
)

// InMemoryJobRepository is an in-memory implementation of the JobRepository interface.
// It holds all job-related data in in-memory maps.
type InMemoryJobRepository struct {
	jobInstances   map[string]*model.JobInstance
	jobExecutions  map[string]*model.JobExecution
	stepExecutions map[string]*model.StepExecution
	checkpointData map[string]*model.CheckpointData
	mu             sync.RWMutex // Mutex to protect concurrent access to maps.
}

// NewInMemoryJobRepository creates and initializes a new instance of InMemoryJobRepository.
func NewInMemoryJobRepository() *InMemoryJobRepository {
	return &InMemoryJobRepository{
		jobInstances:   make(map[string]*model.JobInstance),
		jobExecutions:  make(map[string]*model.JobExecution),
		stepExecutions: make(map[string]*model.StepExecution),
		checkpointData: make(map[string]*model.CheckpointData),
	}
}


// Close releases resources used by the repository.
// As an in-memory repository, it holds no external resources, so this method always returns nil.
func (r *InMemoryJobRepository) Close() error {
	return nil
}
