// Package expression_test provides tests for the database connection expression resolver.
package expression_test

import (
	"context"
	"errors"
	"testing"

	port "github.com/tigerroll/surfin/pkg/batch/core/application/port"
	model "github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	expression "github.com/tigerroll/surfin/pkg/batch/core/support/expression"
	testutil "github.com/tigerroll/surfin/pkg/batch/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockExpressionResolver is a mock implementation of the port.ExpressionResolver interface.
type MockExpressionResolver struct {
	mock.Mock
}

// Resolve mocks the resolution of an expression string.
// Parameters:
//   ctx: The context for the operation.
//   expression: The expression string to resolve.
//   jobExecution: The current JobExecution.
//   stepExecution: The current StepExecution.
// Returns:
//   The resolved string and an error if resolution fails.
func (m *MockExpressionResolver) Resolve(ctx context.Context, expression string, jobExecution *model.JobExecution, stepExecution *model.StepExecution) (string, error) {
	args := m.Called(ctx, expression, jobExecution, stepExecution)
	return args.String(0), args.Error(1)
}

// TestDefaultDBConnectionResolver_ResolveDBConnectionName verifies the behavior of
// ResolveDBConnectionName method in DefaultDBConnectionResolver. It covers scenarios
// such as static names, fallback to default ("workload") for empty or resolved-to-empty expressions,
// successful expression resolution, and handling of resolution failures.
func TestDefaultDBConnectionResolver_ResolveDBConnectionName(t *testing.T) {
	ctx := context.Background()
	mockResolver := new(MockExpressionResolver)
	resolver := expression.NewDefaultDBConnectionResolver(mockResolver)

	jobExecution := testutil.NewTestJobExecution("inst1", "testJob", testutil.NewTestJobParameters(map[string]interface{}{
		"target_db": "dynamic_workload",
		"empty_key": "",
	}))

	// Case 1: Default name is static (no expression)
	t.Run("StaticName", func(t *testing.T) {
		dbName, err := resolver.ResolveDBConnectionName(ctx, jobExecution, nil, "static_db")
		assert.NoError(t, err)
		assert.Equal(t, "static_db", dbName)
		mockResolver.AssertNotCalled(t, "Resolve")
	})

	// Case 2: Default name is empty (should fall back to "workload")
	t.Run("EmptyNameFallback", func(t *testing.T) {
		// Reset mock calls for this run
		mockResolver.ExpectedCalls = nil

		dbName, err := resolver.ResolveDBConnectionName(ctx, jobExecution, nil, "")
		assert.NoError(t, err)
		assert.Equal(t, "workload", dbName)
		mockResolver.AssertNotCalled(t, "Resolve")
	})

	// Case 3: Default name contains a resolvable expression (JobParameters)
	t.Run("ResolvableExpression", func(t *testing.T) {
		expr := "#{jobParameters['target_db']}"

		// Simulate Mock Resolver resolving the value from JobParameters
		mockResolver.On("Resolve", ctx, expr, jobExecution, mock.Anything).Return("dynamic_workload", nil).Once()

		dbName, err := resolver.ResolveDBConnectionName(ctx, jobExecution, nil, expr)
		assert.NoError(t, err)
		assert.Equal(t, "dynamic_workload", dbName)
		mockResolver.AssertCalled(t, "Resolve", ctx, expr, jobExecution, mock.Anything)
	})

	// Case 4: Expression resolution fails (should fall back to original expression string if non-empty)
	t.Run("FailedResolutionFallback", func(t *testing.T) {
		expr := "#{jobParameters['missing_key']}"

		// Simulate Mock Resolver returning an error
		mockResolver.On("Resolve", ctx, expr, jobExecution, mock.Anything).Return("", errors.New("key not found")).Once()

		dbName, err := resolver.ResolveDBConnectionName(ctx, jobExecution, nil, expr)
		assert.NoError(t, err)
		// If resolution fails but the original defaultName (expr) is not empty, it should be returned as is.
		assert.Equal(t, expr, dbName)
		mockResolver.AssertCalled(t, "Resolve", ctx, expr, jobExecution, mock.Anything)
	})

	// Case 5: Expression resolves to empty string (should fall back to "workload")
	t.Run("ResolvedToEmptyStringFallback", func(t *testing.T) {
		expr := "#{jobParameters['empty_key']}"

		// Simulate Mock Resolver returning an empty string
		mockResolver.On("Resolve", ctx, expr, jobExecution, mock.Anything).Return("", nil).Once()

		dbName, err := resolver.ResolveDBConnectionName(ctx, jobExecution, nil, expr)
		assert.NoError(t, err)
		// If the resolved result is an empty string, it should fall back to "workload".
		assert.Equal(t, "workload", dbName)
		mockResolver.AssertCalled(t, "Resolve", ctx, expr, jobExecution, mock.Anything)
	})
}

// Verify interfaces
var _ port.ExpressionResolver = (*MockExpressionResolver)(nil)
