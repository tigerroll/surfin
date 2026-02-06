package dummy

import (
	"context"
	"database/sql"
	"fmt"

	dbconfig "github.com/tigerroll/surfin/pkg/batch/adapter/database/config"
	"github.com/tigerroll/surfin/pkg/batch/support/util/logger"

	"github.com/tigerroll/surfin/pkg/batch/adapter/database"
	coreAdapter "github.com/tigerroll/surfin/pkg/batch/core/adapter"
	"github.com/tigerroll/surfin/pkg/batch/core/tx"
)

// dummyDBConnection is a dummy implementation of the database.DBConnection interface.
// It performs no actual database operations, suitable for DB-less mode or testing.
type dummyDBConnection struct{}

// ExecuteUpdate is a dummy implementation of DBExecutor.ExecuteUpdate.
func (d *dummyDBConnection) ExecuteUpdate(ctx context.Context, model interface{}, operation string, tableName string, query map[string]interface{}) (int64, error) {
	logger.Debugf("Dummy DBConnection: ExecuteUpdate called, doing nothing.")
	return 0, nil
}

// ExecuteUpsert is a dummy implementation of DBExecutor.ExecuteUpsert.
func (d *dummyDBConnection) ExecuteUpsert(ctx context.Context, model interface{}, tableName string, conflictColumns []string, updateColumns []string) (int64, error) {
	logger.Debugf("Dummy DBConnection: ExecuteUpsert called, doing nothing. Table: %s", tableName)
	return 0, nil
}

// ExecuteQuery is a dummy implementation of DBExecutor.ExecuteQuery.
func (d *dummyDBConnection) ExecuteQuery(ctx context.Context, target interface{}, query map[string]interface{}) error {
	logger.Debugf("Dummy DBConnection: ExecuteQuery called, doing nothing. Query: %v", query)
	return nil
}

// Count is a dummy implementation of DBExecutor.Count.
func (d *dummyDBConnection) Count(ctx context.Context, model interface{}, query map[string]interface{}) (int64, error) {
	logger.Debugf("Dummy DBConnection: Count called, doing nothing. Query: %v", query)
	return 0, nil
}

// ExecuteQueryAdvanced is a dummy implementation of DBExecutor.ExecuteQueryAdvanced.
func (d *dummyDBConnection) ExecuteQueryAdvanced(ctx context.Context, target interface{}, query map[string]interface{}, orderBy string, limit int) error {
	logger.Debugf("Dummy DBConnection: ExecuteQueryAdvanced called, doing nothing. Query: %v, OrderBy: %s, Limit: %d", query, orderBy, limit)
	return nil
}

// Pluck is a dummy implementation of DBExecutor.Pluck.
func (d *dummyDBConnection) Pluck(ctx context.Context, model interface{}, column string, target interface{}, query map[string]interface{}) error {
	logger.Debugf("Dummy DBConnection: Pluck called, doing nothing. Field: %s, Value: %v, Query: %v", column, target, query)
	return nil
}

// RefreshConnection is a dummy implementation of DBConnection.RefreshConnection.
func (d *dummyDBConnection) RefreshConnection(ctx context.Context) error {
	logger.Debugf("Dummy DBConnection: RefreshConnection called, doing nothing.")
	return nil
}

// Type returns the type of the dummy database connection.
func (d *dummyDBConnection) Type() string { return "dummy" }

// Name returns the name of the dummy database connection.
func (d *dummyDBConnection) Name() string { return "dummy" }

// Close closes the dummy database connection (no-op).
func (d *dummyDBConnection) Close() error { return nil }

// IsTableNotExistError checks if the given error indicates that a table does not exist (always returns false for dummy).
func (d *dummyDBConnection) IsTableNotExistError(err error) bool { return false }

// Config returns the dummy database configuration.
func (d *dummyDBConnection) Config() dbconfig.DatabaseConfig { return dbconfig.DatabaseConfig{} }

// GetSQLDB returns an error as a dummyDBConnection does not have an underlying *sql.DB.
func (d *dummyDBConnection) GetSQLDB() (*sql.DB, error) {
	return nil, fmt.Errorf("dummyDBConnection does not have an underlying *sql.DB")
}

// dummyDBProvider is a dummy implementation of the database.DBProvider interface.
// It always returns a dummy DBConnection instance.
type dummyDBProvider struct{}

// GetConnection returns a dummy DBConnection.
func (d *dummyDBProvider) GetConnection(name string) (database.DBConnection, error) {
	logger.Debugf("Dummy DBProvider: GetConnection called for '%s'.", name)
	return &dummyDBConnection{}, nil
}

// ForceReconnect returns a new dummy DBConnection, simulating re-establishment.
func (d *dummyDBProvider) ForceReconnect(name string) (database.DBConnection, error) {
	logger.Debugf("Dummy DBProvider: ForceReconnect called for '%s'.", name)
	return &dummyDBConnection{}, nil
}

// CloseAll performs no operation for dummy connections.
func (d *dummyDBProvider) CloseAll() error {
	logger.Debugf("Dummy DBProvider: CloseAll called.")
	return nil
}

// Type returns the type of the dummy database provider.
func (d *dummyDBProvider) Type() string { return "dummy" }

// NewDummyDBConnection returns a new dummy DBConnection instance.
func NewDummyDBConnection() database.DBConnection {
	return &dummyDBConnection{}
}

// NewDummyDBProvider returns a new dummy DBProvider instance.
func NewDummyDBProvider() database.DBProvider {
	return &dummyDBProvider{}
}

// DummyTx is a dummy implementation of the tx.Tx interface.
// It performs no actual operations.
type DummyTx struct{}

// ExecuteUpdate is a dummy implementation of TxExecutor.ExecuteUpdate.
func (d *DummyTx) ExecuteUpdate(ctx context.Context, model interface{}, operation string, tableName string, query map[string]interface{}) (int64, error) {
	return 0, nil
}

// ExecuteUpsert is a dummy implementation of TxExecutor.ExecuteUpsert.
func (d *DummyTx) ExecuteUpsert(ctx context.Context, model interface{}, tableName string, conflictColumns []string, updateColumns []string) (int64, error) {
	return 0, nil
}

// IsTableNotExistError is a dummy implementation of TxExecutor.IsTableNotExistError.
func (d *DummyTx) IsTableNotExistError(err error) bool { return false }

// Savepoint is a dummy implementation of Tx.Savepoint.
func (d *DummyTx) Savepoint(name string) error { return nil }

// RollbackToSavepoint is a dummy implementation of Tx.RollbackToSavepoint.
func (d *DummyTx) RollbackToSavepoint(name string) error { return nil }

// dummyTxManager is a dummy implementation of the tx.TransactionManager interface.
// It performs no actual operations.
type dummyTxManager struct{}

// Begin is a dummy implementation of TransactionManager.Begin.
func (d *dummyTxManager) Begin(ctx context.Context, opts ...*sql.TxOptions) (tx.Tx, error) {
	return &DummyTx{}, nil
}

// Commit is a dummy implementation of TransactionManager.Commit.
func (d *dummyTxManager) Commit(t tx.Tx) error { return nil }

// Rollback is a dummy implementation of TransactionManager.Rollback.
func (d *dummyTxManager) Rollback(t tx.Tx) error { return nil }

// DummyTxManagerFactory is a dummy implementation of the tx.TransactionManagerFactory interface.
// It always returns a dummy TransactionManager.
type DummyTxManagerFactory struct{}

// NewTransactionManager creates a new dummy TransactionManager.
func (d *DummyTxManagerFactory) NewTransactionManager(conn coreAdapter.ResourceConnection) tx.TransactionManager {
	return &dummyTxManager{}
}

// DefaultDBConnectionResolver is a dummy implementation of database.DBConnectionResolver.
type DefaultDBConnectionResolver struct{}

// NewDefaultDBConnectionResolver creates a new DefaultDBConnectionResolver.
func NewDefaultDBConnectionResolver() *DefaultDBConnectionResolver {
	logger.Warnf("Running in DB-less mode. Providing dummy DB connection resolver.")
	return &DefaultDBConnectionResolver{}
}

// ResolveDBConnection resolves a database connection instance by name, returning a dummy connection.
func (r *DefaultDBConnectionResolver) ResolveDBConnection(ctx context.Context, name string) (database.DBConnection, error) {
	logger.Warnf("Attempted to resolve DB connection '%s' in DB-less mode. Returning dummy connection.", name)
	return &dummyDBConnection{}, nil
}

// ResolveConnection resolves a resource connection instance by name, returning a dummy connection.
func (r *DefaultDBConnectionResolver) ResolveConnection(ctx context.Context, name string) (coreAdapter.ResourceConnection, error) {
	return r.ResolveDBConnection(ctx, name)
}

// ResolveConnectionName resolves the name of the resource connection, returning the default name.
func (r *DefaultDBConnectionResolver) ResolveConnectionName(ctx context.Context, jobExecution interface{}, stepExecution interface{}, defaultName string) (string, error) {
	return defaultName, nil
}

// ResolveDBConnectionName resolves the name of the database connection, returning the default name.
func (r *DefaultDBConnectionResolver) ResolveDBConnectionName(ctx context.Context, jobExecution interface{}, stepExecution interface{}, defaultName string) (string, error) {
	return defaultName, nil
}

// Ensure that dummy implementations satisfy their respective interfaces.
var _ database.DBConnection = (*dummyDBConnection)(nil)
var _ database.DBProvider = (*dummyDBProvider)(nil)
var _ tx.Tx = (*DummyTx)(nil)
var _ tx.TransactionManager = (*dummyTxManager)(nil)
var _ tx.TransactionManagerFactory = (*DummyTxManagerFactory)(nil)
var _ database.DBConnectionResolver = (*DefaultDBConnectionResolver)(nil)
