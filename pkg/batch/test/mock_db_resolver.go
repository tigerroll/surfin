package test

import (
	"context"
	"github.com/stretchr/testify/mock"
	adapter "github.com/tigerroll/surfin/pkg/batch/core/adapter" // Alias for clarity
	model "github.com/tigerroll/surfin/pkg/batch/core/domain/model"
)

// MockDBConnectionResolver is a mock implementation of the appport.DBConnectionResolver interface.
// It uses testify/mock to allow for flexible mocking of method calls.
type MockDBConnectionResolver struct {
	mock.Mock
}

// ResolveDBConnectionName mocks the ResolveDBConnectionName method.
// It records the call and returns the predefined values.
func (m *MockDBConnectionResolver) ResolveDBConnectionName(ctx context.Context, jobExecution *model.JobExecution, stepExecution *model.StepExecution, defaultName string) (string, error) {
	args := m.Called(ctx, jobExecution, stepExecution, defaultName)
	return args.String(0), args.Error(1)
}

// ResolveDBConnection mocks the ResolveDBConnection method.
// It records the call and returns the predefined values.
func (m *MockDBConnectionResolver) ResolveDBConnection(ctx context.Context, name string) (adapter.DBConnection, error) {
	args := m.Called(ctx, name)
	return args.Get(0).(adapter.DBConnection), args.Error(1)
}

// testSingleConnectionResolver is a concrete implementation of appport.DBConnectionResolver
// designed for tests that always return a single, predefined DBConnection.
type testSingleConnectionResolver struct {
	conn adapter.DBConnection // The single database connection to be returned.
}

// ResolveDBConnection implements the adapter.DBConnectionResolver interface.
// It always returns the pre-configured DBConnection.
func (r *testSingleConnectionResolver) ResolveDBConnection(ctx context.Context, name string) (adapter.DBConnection, error) {
	return r.conn, nil
}

// ResolveDBConnectionName implements the appport.DBConnectionResolver interface.
// For testing purposes, it simply returns the provided defaultName.
func (r *testSingleConnectionResolver) ResolveDBConnectionName(ctx context.Context, jobExecution *model.JobExecution, stepExecution *model.StepExecution, defaultName string) (string, error) {
	return defaultName, nil
}

// NewTestSingleConnectionResolver creates a new instance of testSingleConnectionResolver.
// This helper function is useful for tests that need a predictable DBConnectionResolver
// that always returns a specific connection.
//
// Parameters:
//
//	conn: The adapter.DBConnection instance that this resolver will always return.
//
// Returns:
//
//	adapter.DBConnectionResolver: A new test-specific DB connection resolver.
func NewTestSingleConnectionResolver(conn adapter.DBConnection) adapter.DBConnectionResolver {
	return &testSingleConnectionResolver{conn: conn}
}

// Ensure that testSingleConnectionResolver implements both appport.DBConnectionResolver
// and adapter.DBConnectionResolver interfaces.
var _ adapter.DBConnectionResolver = (*testSingleConnectionResolver)(nil)
