package inmemory

import (
	"context"
	"fmt"

	"surfin/pkg/batch/core/domain/model"
	"surfin/pkg/batch/core/domain/repository"
)

// SaveJobInstance persists a new JobInstance.
// It returns an error if a JobInstance with the same ID already exists.
func (r *InMemoryJobRepository) SaveJobInstance(ctx context.Context, jobInstance *model.JobInstance) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.jobInstances[jobInstance.ID]; exists {
		return fmt.Errorf("JobInstance with ID %s already exists", jobInstance.ID)
	}
	r.jobInstances[jobInstance.ID] = jobInstance
	return nil
}

// UpdateJobInstance updates an existing JobInstance.
// It returns an error if the JobInstance with the given ID is not found.
func (r *InMemoryJobRepository) UpdateJobInstance(ctx context.Context, jobInstance *model.JobInstance) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.jobInstances[jobInstance.ID]; !exists {
		return fmt.Errorf("JobInstance with ID %s not found for update", jobInstance.ID)
	}
	r.jobInstances[jobInstance.ID] = jobInstance
	return nil
}

// FindJobInstanceByID finds a JobInstance by its ID.
// It returns an error if the JobInstance is not found.
func (r *InMemoryJobRepository) FindJobInstanceByID(ctx context.Context, id string) (*model.JobInstance, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	jobInstance, ok := r.jobInstances[id]
	if !ok {
		return nil, repository.ErrJobInstanceNotFound
	}
	return jobInstance, nil
}

// FindJobInstanceByJobNameAndParameters finds a JobInstance by job name and exact parameters.
func (r *InMemoryJobRepository) FindJobInstanceByJobNameAndParameters(ctx context.Context, jobName string, params model.JobParameters) (*model.JobInstance, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, ji := range r.jobInstances {
		if ji.JobName == jobName && ji.Parameters.Equal(params) {
			return ji, nil
		}
	}
	return nil, repository.ErrJobInstanceNotFound
}

// FindJobInstancesByJobNameAndPartialParameters finds JobInstances matching job name and partial parameters.
func (r *InMemoryJobRepository) FindJobInstancesByJobNameAndPartialParameters(ctx context.Context, jobName string, partialParams model.JobParameters) ([]*model.JobInstance, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var matchingInstances []*model.JobInstance
	for _, ji := range r.jobInstances {
		if ji.JobName == jobName && ji.Parameters.Contains(partialParams) {
			matchingInstances = append(matchingInstances, ji)
		}
	}
	return matchingInstances, nil
}

// GetJobInstanceCount returns the count of JobInstances for a given job name.
func (r *InMemoryJobRepository) GetJobInstanceCount(ctx context.Context, jobName string) (int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	count := 0
	for _, ji := range r.jobInstances {
		if ji.JobName == jobName {
			count++
		}
	}
	return count, nil
}

// GetJobNames returns a list of all distinct job names.
func (r *InMemoryJobRepository) GetJobNames(ctx context.Context) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	uniqueNames := make(map[string]struct{})
	for _, ji := range r.jobInstances {
		uniqueNames[ji.JobName] = struct{}{}
	}

	var names []string
	for name := range uniqueNames {
		names = append(names, name)
	}
	return names, nil
}
