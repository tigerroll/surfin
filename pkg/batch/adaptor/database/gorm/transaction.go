package gorm

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/tigerroll/surfin/pkg/batch/core/adaptor"
	tx "github.com/tigerroll/surfin/pkg/batch/core/tx"
)

// GormTxAdapter implements tx.Tx and is used by GormTransactionManager.
type GormTxAdapter struct {
	db *gorm.DB
}

// ExecuteUpdate implements tx.TxExecutor.
// This logic is similar to GormDBAdapter.ExecuteUpdate but operates on the transaction's *gorm.DB.
func (t *GormTxAdapter) ExecuteUpdate(ctx context.Context, model interface{}, operation string, tableName string, query map[string]interface{}) (rowsAffected int64, err error) {
	db := t.db.WithContext(ctx)

	// SkipDefaultTransaction is not needed as the DB within the transaction is used.

	var result *gorm.DB

	// Apply table name if specified.
	if tableName != "" {
		db = db.Table(tableName)
	}

	switch operation {
	case "CREATE":
		// For CREATE operations, 'model' must be a pointer to an entity or a slice of entities.
		result = db.Create(model)

	case "UPDATE":
		// For UPDATE operations, 'model' must be a pointer to an entity with fields to be updated.
		// Use db.Model(model) to apply primary key and additional query conditions.
		result = db.Model(model).Where(query).Updates(model)

	case "DELETE":
		// For DELETE operations, explicitly specify the table name.
		if query != nil {
			db = db.Where(query)
		}

		// For DELETE operations, 'model' must be a pointer to the entity to be deleted.
		result = db.Delete(model)

	default:
		return 0, fmt.Errorf("unsupported update operation: %s", operation)
	}

	if result.Error != nil {
		return 0, result.Error
	}
	return result.RowsAffected, nil
}

// ExecuteUpsert implements tx.TxExecutor.
// This logic is similar to GormDBAdapter.ExecuteUpsert but operates on the transaction's *gorm.DB.
func (t *GormTxAdapter) ExecuteUpsert(ctx context.Context, model interface{}, tableName string, conflictColumns []string, updateColumns []string) (rowsAffected int64, err error) {
	db := t.db.WithContext(ctx)

	// SkipDefaultTransaction is not needed as the DB within the transaction is used.

	var columns []clause.Column

	if tableName != "" {
		db = db.Table(tableName)
	}

	for _, col := range conflictColumns {
		columns = append(columns, clause.Column{Name: col})
	}

	onConflict := clause.OnConflict{
		Columns: columns,
	}

	if len(updateColumns) > 0 {
		// DO UPDATE
		onConflict.DoUpdates = clause.AssignmentColumns(updateColumns)
	} else {
		// DO NOTHING
		onConflict.DoNothing = true
	}

	result := db.Clauses(onConflict).Create(model)

	if result.Error != nil {
		return 0, result.Error
	}
	return result.RowsAffected, nil
}

// Savepoint implements tx.Tx.
func (t *GormTxAdapter) Savepoint(name string) error {
	return t.db.SavePoint(name).Error
}

// RollbackToSavepoint implements tx.Tx.
func (t *GormTxAdapter) RollbackToSavepoint(name string) error {
	return t.db.RollbackTo(name).Error
}

// IsTableNotExistError implements tx.TxExecutor.
func (t *GormTxAdapter) IsTableNotExistError(err error) bool {
	if err == nil {
		return false
	}
	// Note: GormTxAdapter does not directly hold dbType, but it operates on a GORM DB instance.
	// For simplicity, we'll use a common check or assume the underlying DB type.
	// A more robust solution might involve passing the dbType from the manager or connection.
	errMsg := err.Error()
	// These checks cover common SQL errors for table not found across different DBs.
	return (strings.Contains(errMsg, "relation \"") && strings.Contains(errMsg, "\" does not exist")) || // PostgreSQL
		(strings.Contains(errMsg, "Error 1146") && strings.Contains(errMsg, "doesn't exist")) || // MySQL
		strings.Contains(errMsg, "no such table:") // SQLite
}

// GormTransactionManager implements tx.TransactionManager
type GormTransactionManager struct {
	dbResolver adaptor.DBConnectionResolver
	dbName     string
}

// NewGormTransactionManager creates a new GormTransactionManager.
// func NewGormTransactionManager(dbConn adaptor.DBConnection) tx.TransactionManager {
// 	return &GormTransactionManager{
// 		dbConn: dbConn,
// 	} // This is commented out because it's replaced by the factory pattern.
// }

func (m *GormTransactionManager) Begin(ctx context.Context, opts ...*sql.TxOptions) (tx.Tx, error) {
	// 1. Retrieve the latest DBConnection using DBConnectionResolver.
	conn, err := m.dbResolver.ResolveDBConnection(ctx, m.dbName)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve DB connection '%s' for transaction: %w", m.dbName, err)
	}
	// 2. Get the GORM DB from the DBConnection.
	// This depends on internal implementation but is acceptable only within the adaptor layer.
	adapter, ok := conn.(*GormDBAdapter)
	if !ok {
		return nil, fmt.Errorf("internal error: DBConnection implementation is not *GormDBAdapter")
	}
	gormDB := adapter.GetGormDB().WithContext(ctx)

	var txOpts *sql.TxOptions
	if len(opts) > 0 && opts[0] != nil {
		txOpts = opts[0]
	}

	// Start GORM transaction
	gormTx := gormDB.Begin(txOpts)
	if gormTx.Error != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", gormTx.Error)
	}

	return &GormTxAdapter{db: gormTx}, nil
}

func (m *GormTransactionManager) Commit(t tx.Tx) error {
	// Cast to GormTxAdapter to get the GORM DB.
	gormTxAdapter, ok := t.(*GormTxAdapter)
	if !ok {
		return fmt.Errorf("invalid transaction type: expected *GormTxAdapter")
	}
	return gormTxAdapter.db.Commit().Error
}

func (m *GormTransactionManager) Rollback(t tx.Tx) error {
	gormTxAdapter, ok := t.(*GormTxAdapter) // Cast to GormTxAdapter to get the GORM DB.
	if !ok {
		return fmt.Errorf("invalid transaction type: expected *GormTxAdapter")
	}
	return gormTxAdapter.db.Rollback().Error
}
