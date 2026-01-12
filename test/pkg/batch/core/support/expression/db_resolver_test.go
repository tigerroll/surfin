package expression_test

import (
	"context"
	"errors"
	"testing"

	port "surfin/pkg/batch/core/application/port"
	model "surfin/pkg/batch/core/domain/model"
	expression "surfin/pkg/batch/core/support/expression"
	testutil "surfin/pkg/batch/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockExpressionResolver は port.ExpressionResolver のモック実装です。
type MockExpressionResolver struct {
	mock.Mock
}

func (m *MockExpressionResolver) Resolve(ctx context.Context, expression string, jobExecution *model.JobExecution, stepExecution *model.StepExecution) (string, error) {
	args := m.Called(ctx, expression, jobExecution, stepExecution)
	return args.String(0), args.Error(1)
}

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
		
		// Mock Resolver が JobParameters から値を解決することをシミュレート
		mockResolver.On("Resolve", ctx, expr, jobExecution, mock.Anything).Return("dynamic_workload", nil).Once()

		dbName, err := resolver.ResolveDBConnectionName(ctx, jobExecution, nil, expr)
		assert.NoError(t, err)
		assert.Equal(t, "dynamic_workload", dbName)
		mockResolver.AssertCalled(t, "Resolve", ctx, expr, jobExecution, mock.Anything)
	})

	// Case 4: Expression resolution fails (should fall back to original expression string if non-empty)
	t.Run("FailedResolutionFallback", func(t *testing.T) {
		expr := "#{jobParameters['missing_key']}"
		
		// Mock Resolver がエラーを返すことをシミュレート
		mockResolver.On("Resolve", ctx, expr, jobExecution, mock.Anything).Return("", errors.New("key not found")).Once()

		dbName, err := resolver.ResolveDBConnectionName(ctx, jobExecution, nil, expr)
		assert.NoError(t, err)
		// 解決に失敗したが、元の defaultName (expr) は空ではないため、そのまま返される
		assert.Equal(t, expr, dbName)
		mockResolver.AssertCalled(t, "Resolve", ctx, expr, jobExecution, mock.Anything)
	})
	
	// Case 5: Expression resolves to empty string (should fall back to "workload")
	t.Run("ResolvedToEmptyStringFallback", func(t *testing.T) {
		expr := "#{jobParameters['empty_key']}"
		
		// Mock Resolver が空文字列を返すことをシミュレート
		mockResolver.On("Resolve", ctx, expr, jobExecution, mock.Anything).Return("", nil).Once()

		dbName, err := resolver.ResolveDBConnectionName(ctx, jobExecution, nil, expr)
		assert.NoError(t, err)
		// 解決結果が空文字列の場合、"workload" にフォールバック
		assert.Equal(t, "workload", dbName)
		mockResolver.AssertCalled(t, "Resolve", ctx, expr, jobExecution, mock.Anything)
	})
}

// Verify interfaces
var _ port.ExpressionResolver = (*MockExpressionResolver)(nil)
