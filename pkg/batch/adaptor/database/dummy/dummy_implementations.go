package dummy

import (
	"context"
	"database/sql"

	"github.com/tigerroll/surfin/pkg/batch/core/adaptor"
	dbconfig "github.com/tigerroll/surfin/pkg/batch/adaptor/database/config"
	"github.com/tigerroll/surfin/pkg/batch/core/tx"
	"github.com/tigerroll/surfin/pkg/batch/support/util/logger"
)

// DummyTx is a dummy implementation of the tx.Tx interface.
// It performs no actual operations.
type DummyTx struct{}

func (d *DummyTx) Commit(ctx context.Context) error   { return nil }
func (d *DummyTx) Rollback(ctx context.Context) error { return nil }
func (d *DummyTx) ExecuteUpdate(ctx context.Context, entity interface{}, operation string, tableName string, where map[string]interface{}) (int64, error) {
	return 0, nil
}
func (d *DummyTx) ExecuteUpsert(ctx context.Context, entities interface{}, tableName string, conflictColumns []string, updateColumns []string) (int64, error) {
	return 0, nil
}
func (d *DummyTx) IsTableNotExistError(err error) bool { return false }
func (d *DummyTx) Savepoint(name string) error           { return nil }
func (d *DummyTx) RollbackToSavepoint(name string) error { return nil }

// dummyTxManager is a dummy implementation of the tx.TransactionManager interface.
// It performs no actual operations.
type dummyTxManager struct{}

func (d *dummyTxManager) Begin(ctx context.Context, opts ...*sql.TxOptions) (tx.Tx, error) {
	return &DummyTx{}, nil
}
func (d *dummyTxManager) Commit(t tx.Tx) error   { return nil }
func (d *dummyTxManager) Rollback(t tx.Tx) error { return nil }

// DummyTxManagerFactory is a dummy implementation of the tx.TransactionManagerFactory interface.
// It always returns a dummy TransactionManager.
type DummyTxManagerFactory struct{}

func (d *DummyTxManagerFactory) NewTransactionManager(conn adaptor.DBConnection) tx.TransactionManager {
	return &dummyTxManager{}
}

// dummyDBConnection is a dummy implementation of the adaptor.DBConnection interface.
// It performs no actual operations.
type DummyDBConnection struct{}

func (d *DummyDBConnection) ExecuteUpdate(ctx context.Context, entity interface{}, operation string, tableName string, where map[string]interface{}) (int64, error) {
	return 0, nil
}
func (d *DummyDBConnection) ExecuteUpsert(ctx context.Context, entities interface{}, tableName string, conflictColumns []string, updateColumns []string) (int64, error) {
	return 0, nil
}
func (d *DummyDBConnection) ExecuteQuery(ctx context.Context, target interface{}, query map[string]interface{}) error {
	return nil
}
func (d *DummyDBConnection) ExecuteQueryAdvanced(ctx context.Context, target interface{}, query map[string]interface{}, orderBy string, limit int) error {
	return nil
}
func (d *DummyDBConnection) Count(ctx context.Context, tableName interface{}, where map[string]interface{}) (int64, error) {
	return 0, nil
}
func (d *DummyDBConnection) Pluck(ctx context.Context, model interface{}, column string, target interface{}, query map[string]interface{}) error {
	return nil
}
func (d *DummyDBConnection) Type() string                                { return "dummy" }
func (d *DummyDBConnection) Name() string                                { return "dummy" }
func (d *DummyDBConnection) Close() error                                { return nil }
func (d *DummyDBConnection) IsTableNotExistError(err error) bool         { return false }
func (d *DummyDBConnection) RefreshConnection(ctx context.Context) error { return nil }
func (d *DummyDBConnection) Config() dbconfig.DatabaseConfig             { return dbconfig.DatabaseConfig{} }
func (d *DummyDBConnection) GetSQLDB() (*sql.DB, error)                  { return nil, nil }

// dummyDBProvider is a dummy implementation of the adaptor.DBProvider interface.
// It always returns a dummy DBConnection.
type dummyDBProvider struct{}

func (d *dummyDBProvider) GetConnection(name string) (adaptor.DBConnection, error) {
	return &DummyDBConnection{}, nil
}
func (d *dummyDBProvider) ForceReconnect(name string) (adaptor.DBConnection, error) {
	return &DummyDBConnection{}, nil
}
func (d *dummyDBProvider) CloseAll() error { return nil }
func (d *dummyDBProvider) Type() string    { return "dummy" }

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
	return &DummyDBConnection{}, nil
}
