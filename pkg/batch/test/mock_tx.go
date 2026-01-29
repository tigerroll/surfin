package test

import (
	"context"
	"database/sql"

	"github.com/stretchr/testify/mock"
	tx "github.com/tigerroll/surfin/pkg/batch/core/tx"
)

// MockTx is a mock implementation of the tx.Tx interface.
// It provides mock methods for transaction-related operations,
// allowing for isolated testing of components that interact with transactions.
type MockTx struct {
	mock.Mock
}

// ExecuteUpdate mocks the ExecuteUpdate method of tx.TxExecutor.
// It records the call and returns the predefined values.
func (m *MockTx) ExecuteUpdate(ctx context.Context, model interface{}, operation string, tableName string, query map[string]interface{}) (rowsAffected int64, err error) {
	args := m.Called(ctx, model, operation, tableName, query)
	return args.Get(0).(int64), args.Error(1)
}

// ExecuteUpsert mocks the ExecuteUpsert method of tx.TxExecutor.
// It records the call and returns the predefined values.
func (m *MockTx) ExecuteUpsert(ctx context.Context, model interface{}, tableName string, conflictColumns []string, updateColumns []string) (rowsAffected int64, err error) {
	args := m.Called(ctx, model, tableName, conflictColumns, updateColumns)
	return args.Get(0).(int64), args.Error(1)
}

// IsTableNotExistError mocks the IsTableNotExistError method of tx.TxExecutor.
// It records the call and returns the predefined boolean value.
func (m *MockTx) IsTableNotExistError(err error) bool {
	args := m.Called(err)
	return args.Bool(0)
}

// Savepoint mocks the Savepoint method of tx.Tx.
// It records the call and returns the predefined error.
func (m *MockTx) Savepoint(name string) error {
	args := m.Called(name)
	return args.Error(0)
}

// RollbackToSavepoint mocks the RollbackToSavepoint method of tx.Tx.
// It records the call and returns the predefined error.
func (m *MockTx) RollbackToSavepoint(name string) error {
	args := m.Called(name)
	return args.Error(0)
}

// MockTxManager is a mock implementation of the tx.TransactionManager interface.
// It allows for mocking the lifecycle of transactions (Begin, Commit, Rollback).
type MockTxManager struct {
	mock.Mock
}

// Begin mocks the Begin method of tx.TransactionManager.
// It records the call and returns a mock Tx instance or an error.
func (m *MockTxManager) Begin(ctx context.Context, opts ...*sql.TxOptions) (tx.Tx, error) {
	args := m.Called(ctx, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(tx.Tx), args.Error(1)
}

// Commit mocks the Commit method of tx.TransactionManager.
// It records the call and returns the predefined error.
func (m *MockTxManager) Commit(t tx.Tx) error {
	args := m.Called(t)
	return args.Error(0)
}

// Rollback mocks the Rollback method of tx.TransactionManager.
// It records the call and returns the predefined error.
func (m *MockTxManager) Rollback(t tx.Tx) error {
	args := m.Called(t)
	return args.Error(0)
}

// Ensure that MockTx implements the tx.Tx interface.
var _ tx.Tx = (*MockTx)(nil)

// Ensure that MockTxManager implements the tx.TransactionManager interface.
var _ tx.TransactionManager = (*MockTxManager)(nil)
