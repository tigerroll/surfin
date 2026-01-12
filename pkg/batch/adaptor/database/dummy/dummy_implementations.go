package dummy

import (
	"context"
	"database/sql"

	"github.com/tigerroll/surfin/pkg/batch/core/adaptor"
	"github.com/tigerroll/surfin/pkg/batch/core/config"
	"github.com/tigerroll/surfin/pkg/batch/core/tx"
	"github.com/tigerroll/surfin/pkg/batch/support/util/logger"
)

// dummyTx is a dummy implementation of the tx.Tx interface.
// It performs no actual operations.
type dummyTx struct{}

func (d *dummyTx) Commit(ctx context.Context) error { return nil }
func (d *dummyTx) Rollback(ctx context.Context) error { return nil }
func (d *dummyTx) ExecuteUpdate(ctx context.Context, entity interface{}, operation string, tableName string, where map[string]interface{}) (int64, error) {
	return 0, nil
}
func (d *dummyTx) ExecuteUpsert(ctx context.Context, entities interface{}, tableName string, conflictColumns []string, updateColumns []string) (int64, error) {
	return 0, nil
}
func (d *dummyTx) Savepoint(name string) error { return nil }
func (d *dummyTx) RollbackToSavepoint(name string) error { return nil }

// dummyTxManager is a dummy implementation of the tx.TransactionManager interface.
// It performs no actual operations.
type dummyTxManager struct{}

func (d *dummyTxManager) Begin(ctx context.Context, opts ...*sql.TxOptions) (tx.Tx, error) {
	return &dummyTx{}, nil
}
func (d *dummyTxManager) Commit(t tx.Tx) error { return nil }
func (d *dummyTxManager) Rollback(t tx.Tx) error { return nil }

// dummyTxManagerFactory is a dummy implementation of the tx.TransactionManagerFactory interface.
// It always returns a dummy TransactionManager.
type dummyTxManagerFactory struct{}

func (d *dummyTxManagerFactory) NewTransactionManager(conn adaptor.DBConnection) tx.TransactionManager {
	return &dummyTxManager{}
}

// dummyDBConnection is a dummy implementation of the adaptor.DBConnection interface.
// It performs no actual operations.
type dummyDBConnection struct{}

func (d *dummyDBConnection) ExecuteUpdate(ctx context.Context, entity interface{}, operation string, tableName string, where map[string]interface{}) (int64, error) {
	return 0, nil
}
func (d *dummyDBConnection) ExecuteUpsert(ctx context.Context, entities interface{}, tableName string, conflictColumns []string, updateColumns []string) (int64, error) {
	return 0, nil
}
func (d *dummyDBConnection) ExecuteQuery(ctx context.Context, target interface{}, query map[string]interface{}) error {
	return nil
}
func (d *dummyDBConnection) ExecuteQueryAdvanced(ctx context.Context, target interface{}, query map[string]interface{}, orderBy string, limit int) error {
	return nil
}
func (d *dummyDBConnection) Count(ctx context.Context, tableName interface{}, where map[string]interface{}) (int64, error) {
	return 0, nil
}
func (d *dummyDBConnection) Pluck(ctx context.Context, model interface{}, column string, target interface{}, query map[string]interface{}) error {
	return nil
}
func (d *dummyDBConnection) Type() string { return "dummy" }
func (d *dummyDBConnection) Name() string { return "dummy" }
func (d *dummyDBConnection) Close() error { return nil }
func (d *dummyDBConnection) RefreshConnection(ctx context.Context) error { return nil }
func (d *dummyDBConnection) Config() config.DatabaseConfig { return config.DatabaseConfig{} }
func (d *dummyDBConnection) GetSQLDB() (*sql.DB, error) { return nil, nil }

// dummyDBProvider is a dummy implementation of the adaptor.DBProvider interface.
// It always returns a dummy DBConnection.
type dummyDBProvider struct{}

func (d *dummyDBProvider) GetConnection(name string) (adaptor.DBConnection, error) {
	return &dummyDBConnection{}, nil
}
func (d *dummyDBProvider) ForceReconnect(name string) (adaptor.DBConnection, error) {
	return &dummyDBConnection{}, nil
}
func (d *dummyDBProvider) CloseAll() error { return nil }
func (d *dummyDBProvider) Type() string { return "dummy" }

// DefaultDBConnectionResolver is a dummy implementation of the adaptor.DBConnectionResolver.
type DefaultDBConnectionResolver struct{}

// NewDefaultDBConnectionResolver creates a new DefaultDBConnectionResolver.
func NewDefaultDBConnectionResolver() *DefaultDBConnectionResolver {
	logger.Warnf("Running in DB-less mode. Providing dummy DB connection resolver.")
	return &DefaultDBConnectionResolver{}
}

// ResolveDBConnection resolves a database connection instance by name.
// It returns a dummy DBConnection.
func (r *DefaultDBConnectionResolver) ResolveDBConnection(ctx context.Context, name string) (adaptor.DBConnection, error) {
	logger.Warnf("Attempted to resolve DB connection '%s' in DB-less mode. Returning dummy connection.", name)
	return &dummyDBConnection{}, nil
}
