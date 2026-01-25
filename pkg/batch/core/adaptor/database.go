// Package adaptor provides abstractions for database connections and providers in the Surfin Batch Framework.
// This allows unified access to different database systems (e.g., PostgreSQL, MySQL, SQLite) through a consistent interface.
package adaptor

import (
	"context"
	"database/sql"

	config "github.com/tigerroll/surfin/pkg/batch/core/config"
)

// DBExecutor is an interface that defines common write operations for a database.
// It is intended to be embedded in both DBConnection and Tx (transaction).
//
// This interface is designed to allow data operations to be executed in the same way,
// regardless of whether a transaction is active. For example, if a transaction is active,
// it will be executed within that transaction; otherwise, it will be executed in auto-commit mode.
//
// Methods:
//   - ExecuteUpdate: Executes write operations such as INSERT, UPDATE, DELETE.
//   - ExecuteUpsert: Executes UPSERT (INSERT OR REPLACE / ON CONFLICT DO UPDATE) operations.
type DBExecutor interface {
	// ExecuteUpdate performs write operations (INSERT, UPDATE, DELETE) within a transaction.
	// model: The target model struct or slice.
	// operation: The type of operation to execute (e.g., "CREATE", "UPDATE", "DELETE").
	// tableName: The name of the table to operate on.
	// query: Query conditions (for UPDATE/DELETE, a map of key-value pairs, combined with AND).
	// Returns: The number of affected rows and an error.
	ExecuteUpdate(ctx context.Context, model interface{}, operation string, tableName string, query map[string]interface{}) (rowsAffected int64, err error)

	// ExecuteUpsert performs an UPSERT operation (INSERT OR REPLACE / ON CONFLICT DO UPDATE) within a transaction.
	// model: The target model struct or slice.
	// tableName: The name of the table to operate on.
	// conflictColumns: List of column names used to detect conflicts.
	// updateColumns: List of column names to update on conflict (DO NOTHING if nil or empty).
	// Returns: The number of affected rows and an error.
	ExecuteUpsert(ctx context.Context, model interface{}, tableName string, conflictColumns []string, updateColumns []string) (rowsAffected int64, err error)
}

// DBConnection represents an abstraction of a database connection.
// It provides database operations, connection management, and access to configuration.
//
// This interface enables interaction with databases in a way that is independent of
// specific database implementations (e.g., PostgreSQL, MySQL), enhancing the framework's portability.
//
// Methods:
//   - Type: Returns the type of the database.
//   - Name: Returns the connection name.
//   - Close: Closes the connection.
type DBConnection interface {
	DBExecutor // Embeds ExecuteUpdate, ExecuteUpsert

	// Type returns the type of the database (e.g., "mysql", "postgres").
	Type() string
	// Name returns the connection name (e.g., "metadata", "workload").
	Name() string
	// Close closes the database connection.
	Close() error
	// IsTableNotExistError checks if the given error indicates that a table does not exist.
	IsTableNotExistError(err error) bool
	// RefreshConnection forces the re-establishment of the database connection.
	// This is used, for example, when reflecting schema changes after migration.
	RefreshConnection(ctx context.Context) error
	// Config returns the database configuration associated with this connection.
	// Returns: config.DatabaseConfig struct.
	Config() config.DatabaseConfig

	// GetSQLDB returns the underlying *sql.DB connection.
	// This exposes low-level dependencies but is necessary for migration tools and raw SQL access.
	// Returns: *sql.DB instance and an error.
	GetSQLDB() (*sql.DB, error)

	// ExecuteQuery executes a read operation (SELECT) outside of a managed transaction.
	// target: A pointer to the struct or slice to store the results.
	// query: Query conditions (key-value map, combined with AND).
	// Returns: An error.
	ExecuteQuery(ctx context.Context, target interface{}, query map[string]interface{}) error

	// ExecuteQueryAdvanced executes a read operation with optional sorting and limiting.
	// This method is extended to allow specifying sort order and a limit on the number of records fetched,
	// to meet the needs of more complex SELECT queries.
	//
	// ctx: The context for the operation.
	// target: A pointer to the Go struct or slice to store the query results.
	//         For example, types like `[]MyStruct` or `*MyStruct` are expected.
	// query: A key-value map defining the WHERE clause conditions.
	//        Keys are column names, values are corresponding condition values. Multiple entries are combined with AND.
	//        Example: `{"status": "active", "user_id": 123}`
	// orderBy: A string specifying the sort order of the results.
	//          Example: `"created_at DESC", "name ASC"`. Use comma-separated for multiple columns.
	// limit: The maximum number of records to retrieve. If 0, all records are retrieved without limit.
	//
	// Returns:
	//   - error: An error that occurred during query execution. nil on success.
	//
	ExecuteQueryAdvanced(ctx context.Context, target interface{}, query map[string]interface{}, orderBy string, limit int) error

	// Count counts the number of records matching the query.
	// model: The target model struct.
	// query: Query conditions for counting.
	// Returns: The count of records and an error.
	Count(ctx context.Context, model interface{}, query map[string]interface{}) (int64, error)

	// Pluck retrieves a list of values for a specific column.
	// This method extracts values of a specific column based on the given model and query conditions,
	// and stores them into the target slice.
	//
	// ctx: The context for the operation.
	// model: The target model struct. Data will be retrieved from the table of this model.
	// column: The name of the column to extract.
	// target: A pointer to the slice to store the extracted values.
	//         Example: `&[]string{}`, `&[]int64{}` are expected.
	// query: A key-value map defining the WHERE clause conditions.
	//        Keys are column names, values are corresponding condition values. Multiple entries are combined with AND.
	//
	// Returns:
	//   - error: An error that occurred during the operation. nil on success.
	//
	Pluck(ctx context.Context, model interface{}, column string, target interface{}, query map[string]interface{}) error
}

// DBConnectionResolver is an interface that resolves the required database connection instance based on the execution context.
type DBConnectionResolver interface {
	// ResolveDBConnection resolves a database connection instance by name.
	// This method is responsible for ensuring that the returned connection is valid and re-established if necessary.
	//
	// ctx: The context for the operation.
	// name: The name of the database connection to resolve (e.g., "metadata", "workload").
	//
	// Returns:
	//   - DBConnection: The resolved database connection instance.
	//   - error: An error that occurred during connection resolution or re-establishment.
	//
	ResolveDBConnection(ctx context.Context, name string) (DBConnection, error)
}

// DBProvider is an interface responsible for providing database connections based on configuration.
// Concrete implementations corresponding to different database types (e.g., PostgreSQL, MySQL) are provided.
//
// This interface abstracts the lifecycle management of database connections (getting, forcing reconnect, closing all connections),
// allowing connections to be handled independently of the database type.
//
// Methods:
//   - GetConnection: Retrieves a database connection with the specified name.
//   - ForceReconnect: Forces the closure and re-establishment of an existing connection.
//   - CloseAll: Closes all connections managed by this provider.
//   - Type: Returns the type of database handled by this provider.
type DBProvider interface {
	// GetConnection retrieves a database connection with the specified name.
	// name: The name of the database connection to retrieve.
	GetConnection(name string) (DBConnection, error)
	// ForceReconnect forces the closure and re-establishment of an existing connection with the specified name.
	// name: The name of the database connection to reconnect.
	ForceReconnect(name string) (DBConnection, error)
	// CloseAll closes all database connections managed by this provider.
	CloseAll() error
	// Type returns the type of database handled by this provider (e.g., "postgres", "mysql").
	Type() string
}

// DBProviderGroup is an Fx tag used to group all DBProvider implementations.
const DBProviderGroup = "db_providers"
