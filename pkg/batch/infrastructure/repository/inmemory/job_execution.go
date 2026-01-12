package inmemory

import (
	"context"
	"fmt"
	"sort"

	"github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	"github.com/tigerroll/surfin/pkg/batch/core/domain/repository"
)

// SaveJobExecution persists a new JobExecution.
// It returns an error if a JobExecution with the same ID already exists.
func (r *InMemoryJobRepository) SaveJobExecution(ctx context.Context, jobExecution *model.JobExecution) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.jobExecutions[jobExecution.ID]; exists {
		return fmt.Errorf("JobExecution with ID %s already exists", jobExecution.ID)
	}
	r.jobExecutions[jobExecution.ID] = jobExecution
	return nil
}

// UpdateJobExecution updates an existing JobExecution.
// It returns an error if the JobExecution with the given ID is not found.
func (r *InMemoryJobRepository) UpdateJobExecution(ctx context.Context, jobExecution *model.JobExecution) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.jobExecutions[jobExecution.ID]; !exists {
		return fmt.Errorf("JobExecution with ID %s not found for update", jobExecution.ID)
	}
	r.jobExecutions[jobExecution.ID] = jobExecution
	return nil
}

// FindJobExecutionByID finds a JobExecution by its ID.
// It also loads and associates all related StepExecutions with the JobExecution object.
// It returns an error if the JobExecution is not found.
func (r *InMemoryJobRepository) FindJobExecutionByID(ctx context.Context, id string) (*model.JobExecution, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	jobExecution, ok := r.jobExecutions[id]
	if !ok {
		return nil, repository.ErrJobExecutionNotFound
	}

	// Deep copy to prevent external modification of internal state
	clonedJobExecution := *jobExecution
	clonedJobExecution.StepExecutions = make([]*model.StepExecution, 0)

	// Associate StepExecutions
	for _, se := range r.stepExecutions {
		if se.JobExecutionID == clonedJobExecution.ID {
			clonedJobExecution.StepExecutions = append(clonedJobExecution.StepExecutions, se)
		}
	}
	// Sort StepExecutions by StartTime for consistency
	sort.Slice(clonedJobExecution.StepExecutions, func(i, j int) bool {
		return clonedJobExecution.StepExecutions[i].StartTime.Before(clonedJobExecution.StepExecutions[j].StartTime)
	})

	return &clonedJobExecution, nil
}

// FindLatestRestartableJobExecution finds the latest JobExecution for a given JobInstance that is restartable
func (r *InMemoryJobRepository) FindLatestRestartableJobExecution(ctx context.Context, jobInstanceID string) (*model.JobExecution, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var latestRestartable *model.JobExecution
	for _, je := range r.jobExecutions {
		if je.JobInstanceID == jobInstanceID && (je.Status == model.BatchStatusFailed || je.Status == model.BatchStatusStopped) {
			if latestRestartable == nil || je.CreateTime.After(latestRestartable.CreateTime) {
				latestRestartable = je
			}
		}
	}

	if latestRestartable == nil {
		return nil, repository.ErrJobExecutionNotFound
	}

	// Deep copy and associate StepExecutions
	clonedJobExecution := *latestRestartable
	clonedJobExecution.StepExecutions = make([]*model.StepExecution, 0)
	for _, se := range r.stepExecutions {
		if se.JobExecutionID == clonedJobExecution.ID {
			clonedJobExecution.StepExecutions = append(clonedJobExecution.StepExecutions, se)
		}
	}
	sort.Slice(clonedJobExecution.StepExecutions, func(i, j int) bool {
		return clonedJobExecution.StepExecutions[i].StartTime.Before(clonedJobExecution.StepExecutions[j].StartTime)
	})

	return &clonedJobExecution, nil
}

// FindJobExecutionsByJobInstance finds all JobExecutions associated with the specified JobInstance
func (r *InMemoryJobRepository) FindJobExecutionsByJobInstance(ctx context.Context, jobInstance *model.JobInstance) ([]*model.JobExecution, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var executions []*model.JobExecution
	for _, je := range r.jobExecutions {
		if je.JobInstanceID == jobInstance.ID {
			// Deep copy to prevent external modification of internal state
			clonedJe := *je
			clonedJe.StepExecutions = make([]*model.StepExecution, 0) // StepExecutions are not loaded by this method
			executions = append(executions, &clonedJe)
		}
	}

	// Sort by CreateTime in descending order (latest first)
	sort.Slice(executions, func(i, j int) bool {
		return executions[j].CreateTime.Before(executions[i].CreateTime)
	})

	return executions, nil
}
