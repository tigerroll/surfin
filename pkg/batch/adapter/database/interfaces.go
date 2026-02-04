package database

import (
	"context"
	"database/sql"

	dbconfig "github.com/tigerroll/surfin/pkg/batch/adapter/database/config"
	coreAdapter "github.com/tigerroll/surfin/pkg/batch/core/adapter"
)

// DBExecutor is an interface that defines common write and read operations for a database.
// It is intended to be embedded in both DBConnection and Tx (transaction).
type DBExecutor interface {
	// ExecuteUpdate performs write operations (INSERT, UPDATE, DELETE) within a transaction.
	ExecuteUpdate(ctx context.Context, model interface{}, operation string, tableName string, query map[string]interface{}) (rowsAffected int64, err error)

	// ExecuteUpsert performs an UPSERT operation (INSERT OR REPLACE / ON CONFLICT DO UPDATE) within a transaction.
	ExecuteUpsert(ctx context.Context, model interface{}, tableName string, conflictColumns []string, updateColumns []string) (rowsAffected int64, err error)

	// ExecuteQuery executes a read operation (SELECT) outside of a managed transaction.
	ExecuteQuery(ctx context.Context, target interface{}, query map[string]interface{}) error

	// ExecuteQueryAdvanced executes a read operation with optional sorting and limiting.
	ExecuteQueryAdvanced(ctx context.Context, target interface{}, query map[string]interface{}, orderBy string, limit int) error

	// Count counts the number of records matching the query.
	Count(ctx context.Context, model interface{}, query map[string]interface{}) (int64, error)

	// Pluck retrieves a list of values for a specific column.
	Pluck(ctx context.Context, model interface{}, column string, target interface{}, query map[string]interface{}) error
}

// DBConnection represents an abstraction of a database connection.
// It embeds coreAdapter.ResourceConnection for generic connection management
// and DBExecutor for database-specific operations.
type DBConnection interface {
	coreAdapter.ResourceConnection // Embeds Type(), Name(), Close()
	DBExecutor                     // Embeds ExecuteUpdate, ExecuteUpsert, ExecuteQuery, Count, Pluck

	// IsTableNotExistError checks if the given error indicates that a table does not exist.
	IsTableNotExistError(err error) bool
	// RefreshConnection forces the re-establishment of the database connection.
	RefreshConnection(ctx context.Context) error
	// Config returns the database configuration associated with this connection.
	Config() dbconfig.DatabaseConfig
	// GetSQLDB returns the underlying *sql.DB connection.
	GetSQLDB() (*sql.DB, error)
}

// DBConnectionResolver is an interface that resolves the required database connection instance based on the execution context.
// It embeds coreAdapter.ResourceConnectionResolver for generic resolution.
type DBConnectionResolver interface {
	coreAdapter.ResourceConnectionResolver // Embeds ResolveConnection, ResolveConnectionName

	// ResolveDBConnection resolves a database connection instance by name.
	// This method is responsible for ensuring that the returned connection is valid and re-established if necessary.
	ResolveDBConnection(ctx context.Context, name string) (DBConnection, error)

	// ResolveDBConnectionName resolves the name of the database connection based on the execution context.
	// This method allows dynamic selection of database connections (e.g., based on job parameters or step context).
	// jobExecution and stepExecution are passed as interface{} to avoid circular dependencies with model package.
	ResolveDBConnectionName(ctx context.Context, jobExecution interface{}, stepExecution interface{}, defaultName string) (string, error)
}

// DBProvider is an interface responsible for providing database connections based on configuration.
// It embeds coreAdapter.ResourceProvider for generic provider management.
type DBProvider interface {
	// GetConnection retrieves a database connection with the specified name.
	GetConnection(name string) (DBConnection, error)
	// CloseAll closes all connections managed by this provider.
	CloseAll() error
	// Type returns the type of resource handled by this provider (e.g., "database", "storage").
	Type() string
	// ForceReconnect forces the closure and re-establishment of an existing connection with the specified name.
	ForceReconnect(name string) (DBConnection, error)
}

// DBProviderGroup is an Fx tag used to group all DBProvider implementations.
const DBProviderGroup = `group:"db_providers"`
