package test

import (
	"context"
	port "surfin/pkg/batch/core/application/port"
	model "surfin/pkg/batch/core/domain/model"
)

// MockDBConnectionResolver is a mock implementation of port.DBConnectionResolver.
type MockDBConnectionResolver struct {
	port.DBConnectionResolver // Enforces the use of the port package.
	ResolveFunc func(ctx context.Context, jobExecution *model.JobExecution, stepExecution *model.StepExecution, defaultName string) (string, error)
}

// ResolveDBConnectionName executes the mock function.
func (m *MockDBConnectionResolver) ResolveDBConnectionName(ctx context.Context, jobExecution *model.JobExecution, stepExecution *model.StepExecution, defaultName string) (string, error) {
	if m.ResolveFunc != nil {
		return m.ResolveFunc(ctx, jobExecution, stepExecution, defaultName)
	}
	return defaultName, nil // Default behavior.
}
