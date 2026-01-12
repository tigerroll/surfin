package gorm

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/tigerroll/surfin/pkg/batch/core/adaptor"
	config "github.com/tigerroll/surfin/pkg/batch/core/config"
	"github.com/tigerroll/surfin/pkg/batch/core/tx"
	"github.com/tigerroll/surfin/pkg/batch/support/util/logger"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	gorm_logger "gorm.io/gorm/logger"
)

// TableNamer represents a struct that has a TableName() string method.
type TableNamer interface {
	TableName() string
}

// applyTableName applies the table name to the GORM DB session if the model implements the TableNamer interface.
func applyTableName(db *gorm.DB, model interface{}) *gorm.DB {
	val := reflect.ValueOf(model)

	// Dereference the pointer
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	// 1. Check if the model itself implements TableNamer (for single entity)
	if namer, ok := model.(TableNamer); ok {
		return db.Table(namer.TableName())
	}

	// 2. For slices, check if the element type implements TableNamer.
	if val.Kind() == reflect.Slice || val.Kind() == reflect.Array {
		elemType := val.Type().Elem()

		// If the element is a pointer type, get its element type.
		if elemType.Kind() == reflect.Ptr {
			elemType = elemType.Elem()
		}

		// Check if the element type implements TableNamer.
		// Since TableName() is implemented with a value receiver, check using reflect.New(elemType).Interface().
		if reflect.New(elemType).Type().Implements(reflect.TypeOf((*TableNamer)(nil)).Elem()) {
			// If TableNamer is implemented, create a temporary instance to get the table name.
			if namer, ok := reflect.New(elemType).Interface().(TableNamer); ok {
				return db.Table(namer.TableName())
			}
		}
	}

	// 3. If unable to resolve, let GORM infer the table name from the model.
	return db.Model(model)
}

// NewGormLogger creates a gorm.Logger instance based on the configured log level.
func NewGormLogger(level string) gorm_logger.Interface {
	var gormLevel gorm_logger.LogLevel
	switch config.LogLevel(level) {
	case config.LogLevelSilent:
		gormLevel = gorm_logger.Silent
	case config.LogLevelError:
		gormLevel = gorm_logger.Error
	case config.LogLevelWarn:
		gormLevel = gorm_logger.Warn
	case config.LogLevelInfo:
		gormLevel = gorm_logger.Info
	default:
		// Default to Silent if not explicitly configured or unknown
		gormLevel = gorm_logger.Silent
	}

	// GormWriter is defined within this file to eliminate dependency on logger.go.
	writer := NewGormWriter()

	return gorm_logger.New(
		writer,
		gorm_logger.Config{
			SlowThreshold:             200 * time.Millisecond, // Default slow threshold
			LogLevel:                  gormLevel,
			IgnoreRecordNotFoundError: true,
			Colorful:                  false,
		},
	)
}

// GormWriter is an io.Writer that redirects GORM log output to surfin/pkg/batch/support/util/logger.
type GormWriter struct{}

// NewGormWriter creates a new instance of GormWriter.
func NewGormWriter() *GormWriter {
	return &GormWriter{}
}

// Write implements io.Writer.
func (w *GormWriter) Write(p []byte) (n int, err error) {
	msg := strings.TrimSpace(string(p))
	// GORM logs are typically in the format [<duration>ms] SELECT ..., so treat them as DEBUG.
	if strings.Contains(msg, "[") && strings.Contains(msg, "]") && (strings.Contains(msg, "SELECT") || strings.Contains(msg, "INSERT") || strings.Contains(msg, "UPDATE") || strings.Contains(msg, "DELETE")) {
		logger.Debugf("[GORM] %s", msg)
	} else {
		// Other GORM logs (connection info, warnings, etc.) are treated as INFO.
		logger.Infof("[GORM] %s", msg)
	}
	return len(p), nil
}

// Printf implements gormLogger.Writer interface.
func (w *GormWriter) Printf(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	if strings.Contains(msg, "[") && strings.Contains(msg, "]") && (strings.Contains(msg, "SELECT") || strings.Contains(msg, "INSERT") || strings.Contains(msg, "UPDATE") || strings.Contains(msg, "DELETE")) {
		logger.Debugf("[GORM] %s", strings.TrimSpace(msg))
	} else {
		logger.Infof("[GORM] %s", strings.TrimSpace(msg))
	}
}

// GormDBAdapter implements adaptor.DBConnection
type GormDBAdapter struct {
	db     *gorm.DB
	sqlDB  *sql.DB
	cfg    config.DatabaseConfig
	dbType string
	name   string
}

// NewGormDBAdapter creates a new GormDBAdapter.
func NewGormDBAdapter(db *gorm.DB, cfg config.DatabaseConfig, name string) adaptor.DBConnection {
	sqlDB, err := db.DB()
	if err != nil {
		logger.Fatalf("Failed to get underlying *sql.DB: %v", err)
	}

	return &GormDBAdapter{
		db:     db,
		sqlDB:  sqlDB,
		cfg:    cfg,
		dbType: cfg.Type,
		name:   name,
	}
}

// GetGormDB returns the underlying *gorm.DB instance.
// NOTE: This method is intended for internal use within the 'gorm' adaptor package only.
func (a *GormDBAdapter) GetGormDB() *gorm.DB {
	return a.db
}

// GormDB() was removed from adaptor.DBConnection, so it is not implemented here.
// However, access to the internal *gorm.DB is done through helper functions within the same package.

func (a *GormDBAdapter) Close() error {
	if a.sqlDB != nil {
		logger.Infof("Closing database connection '%s'...", a.name)
		return a.sqlDB.Close()
	}
	return nil
}

func (a *GormDBAdapter) Type() string {
	return a.dbType
}

func (a *GormDBAdapter) Name() string {
	return a.name
}

// RefreshConnection implements adaptor.DBConnection.
func (a *GormDBAdapter) RefreshConnection(ctx context.Context) error {
	if a.sqlDB == nil {
		return fmt.Errorf("database connection is not initialized")
	}
	// Re-ping the connection pool to ensure validity
	return a.sqlDB.PingContext(ctx)
}

// Config implements adaptor.DBConnection.
func (a *GormDBAdapter) Config() config.DatabaseConfig {
	return a.cfg
}

// GetSQLDB implements adaptor.DBConnection.
func (a *GormDBAdapter) GetSQLDB() (*sql.DB, error) {
	if a.sqlDB == nil {
		return nil, fmt.Errorf("underlying sql.DB is nil")
	}
	return a.sqlDB, nil
}

// ExecuteQuery implements adaptor.DBConnection.
// This method executes a read operation using GORM's Find method.
func (a *GormDBAdapter) ExecuteQuery(ctx context.Context, target interface{}, query map[string]interface{}) error {
	db := a.db.WithContext(ctx)

	db = applyTableName(db, target) // Fix: Check TableNamer and apply table name.

	// Execute the query using GORM's Find method.
	// If query is a map, it is used as a Where clause.
	result := db.Where(query).Find(target)

	if result.Error != nil {
		return result.Error
	}

	// If no record is found, GORM does not return an error, so the caller needs to handle it.
	// However, Find() does not return ErrRecordNotFound for slices, so only error checking is performed here.
	return nil
}

// ExecuteQueryAdvanced implements adaptor.DBConnection.
func (a *GormDBAdapter) ExecuteQueryAdvanced(ctx context.Context, target interface{}, query map[string]interface{}, orderBy string, limit int) error {
	db := a.db.WithContext(ctx)

	db = applyTableName(db, target) // Fix: Check TableNamer and apply table name.

	if query != nil {
		db = db.Where(query)
	}

	if orderBy != "" {
		db = db.Order(orderBy)
	}

	if limit > 0 {
		db = db.Limit(limit)
	}

	result := db.Find(target)

	if result.Error != nil {
		return result.Error
	}
	return nil
}

// Count implements adaptor.DBConnection.
func (a *GormDBAdapter) Count(ctx context.Context, model interface{}, query map[string]interface{}) (int64, error) {
	db := a.db.WithContext(ctx)

	db = applyTableName(db, model) // Fix: Check TableNamer and apply table name.

	if query != nil {
		db = db.Where(query)
	}
	var count int64
	if err := db.Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// Pluck implements adaptor.DBConnection.
func (a *GormDBAdapter) Pluck(ctx context.Context, model interface{}, column string, target interface{}, query map[string]interface{}) error {
	db := a.db.WithContext(ctx)

	db = applyTableName(db, model) // Fix: Check TableNamer and apply table name.

	if query != nil {
		db = db.Where(query)
	}

	db = db.Distinct() // Apply Distinct before Pluck.
	if err := db.Pluck(column, target).Error; err != nil {
		return err
	}
	return nil
}

// ExecuteUpdate implements adaptor.DBExecutor.
// This method executes a write operation (CREATE, UPDATE, DELETE) using GORM.
func (a *GormDBAdapter) ExecuteUpdate(ctx context.Context, model interface{}, operation string, tableName string, query map[string]interface{}) (rowsAffected int64, err error) {
	db := a.db.WithContext(ctx)

	// NOTE: Skip GORM's default transaction.
	db = db.Session(&gorm.Session{SkipDefaultTransaction: true})

	var result *gorm.DB

	// Apply table name if specified (prioritize instructions from the repository layer).
	if tableName != "" {
		db = db.Table(tableName)
	}

	switch operation {
	case "CREATE":
		// For CREATE operations, 'model' must be a pointer to an entity or a slice of entities.
		// db.Create() uses the table if db.Table() is called, otherwise it resolves from the model.
		result = db.Create(model)

	case "UPDATE":
		// For UPDATE operations, 'model' must be a pointer to an entity with fields to be updated.

		// Using db.Model(model) automatically uses the model's primary key as a WHERE clause condition.
		// If db.Table() is called, the operation is performed on that table.
		db = db.Model(model)
		result = db.Where(query).Updates(model)

	case "DELETE":
		// For DELETE operations, 'model' must be a pointer to the entity to be deleted.

		// If db.Table() is called, the operation is performed on that table.
		if query != nil {
			db = db.Where(query)
		}

		result = db.Delete(model)

	default:
		return 0, fmt.Errorf("unsupported update operation: %s", operation)
	}

	if result.Error != nil {
		return 0, result.Error
	}
	return result.RowsAffected, nil
}

// ExecuteUpsert implements adaptor.DBExecutor.
func (a *GormDBAdapter) ExecuteUpsert(ctx context.Context, model interface{}, tableName string, conflictColumns []string, updateColumns []string) (rowsAffected int64, err error) {
	db := a.db.WithContext(ctx)

	// NOTE: Skip GORM's default transaction.
	db = db.Session(&gorm.Session{SkipDefaultTransaction: true})

	var columns []clause.Column

	// Apply table name if specified (prioritize instructions from the repository layer).
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

// NewGormTransactionManager creates a GORM-based TransactionManager.
// Since it is called via TransactionManagerFactory from outside, change the argument to a concrete adapter type.
// func NewGormTransactionManager(conn *GormDBAdapter) tx.TransactionManager {
// 	return &GormTransactionManager{dbConn: conn}
// }

// GormTransactionManagerFactory is the GORM implementation of tx.TransactionManagerFactory.
type GormTransactionManagerFactory struct {
	dbResolver adaptor.DBConnectionResolver // Add
}

// NewGormTransactionManagerFactory creates an instance of GormTransactionManagerFactory.
func NewGormTransactionManagerFactory(dbResolver adaptor.DBConnectionResolver) tx.TransactionManagerFactory {
	return &GormTransactionManagerFactory{dbResolver: dbResolver}
}

// NewTransactionManager creates a GormTransactionManager from GormDBAdapter.
func (f *GormTransactionManagerFactory) NewTransactionManager(dbConn adaptor.DBConnection) tx.TransactionManager {
	// TxManager is changed to depend on DBConnectionResolver and DBName.
	return &GormTransactionManager{
		dbResolver: f.dbResolver,
		dbName:     dbConn.Name(),
	}
}
