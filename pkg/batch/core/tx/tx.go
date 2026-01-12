// Package tx provides an abstraction for transaction management in the Surfin Batch Framework.
// This ensures Atomicity, Consistency, Isolation, and Durability (ACID) of database operations,
// enabling unified transaction control across different database backends.
package tx

import (
	"context"
	"database/sql"

	"github.com/tigerroll/surfin/pkg/batch/core/adaptor"
)

// TxExecutor is an interface that defines common write operations executable within a transaction.
// This interface is intended to be embedded in both DBConnection and Tx,
// allowing data operations to be performed in the same way regardless of the presence of a transaction.
type TxExecutor interface {
	// ExecuteUpdate performs database write operations (INSERT, UPDATE, DELETE) on the specified model.
	// This operation is executed within the current transaction context.
	//
	// ctx: The context for the operation.
	// model: A Go struct or slice containing the data to be saved or updated in the database.
	// operation: A string indicating the type of operation to be performed (e.g., "CREATE", "UPDATE", "DELETE").
	// tableName: The name of the target database table.
	// query: A key-value map for specifying conditions in UPDATE or DELETE operations.
	//        Keys are column names, values are corresponding values. Multiple entries are combined with AND.
	// Returns: The number of affected rows and any error that occurred during the operation.
	ExecuteUpdate(ctx context.Context, model interface{}, operation string, tableName string, query map[string]interface{}) (rowsAffected int64, err error)
	
	// ExecuteUpsert performs an UPSERT operation (INSERT OR REPLACE / ON CONFLICT DO UPDATE) on the database.
	// This operation is executed within the current transaction context.
	//
	// ctx: The context for the operation.
	// model: A Go struct or slice containing the data to be inserted or updated in the database.
	// tableName: The name of the target database table.
	// conflictColumns: A list of column names used to detect conflicts. If the combination of these columns
	//                  duplicates an existing record, an UPSERT is triggered.
	// updateColumns: A list of column names to be updated if a conflict occurs. If this list is nil or empty,
	//                conflicts will be treated as DO NOTHING.
	// Returns: The number of affected rows and any error that occurred during the operation.
	ExecuteUpsert(ctx context.Context, model interface{}, tableName string, conflictColumns []string, updateColumns []string) (rowsAffected int64, err error)
}

// Tx represents an ongoing database transaction.
// Through this interface, data operations within a transaction and savepoint management are possible.
type Tx interface {
	TxExecutor // Embeds write operations executable within a transaction

	// Savepoint creates a new savepoint within the current transaction.
	// This allows rolling back a portion of the transaction to this savepoint later.
	// name: The unique name for the savepoint to be created.
	// Returns: An error that occurred during savepoint creation.
	Savepoint(name string) error
	
	// RollbackToSavepoint rolls back the transaction to the savepoint with the specified name.
	// This undoes changes made after the savepoint, but preserves changes made before it.
	// name: The name of the savepoint to roll back to.
	// Returns: An error that occurred during the rollback.
	RollbackToSavepoint(name string) error
}

// TransactionManager is an interface that manages the lifecycle of database transactions (begin, commit, rollback).
// This interface abstracts transaction propagation and isolation level control.
type TransactionManager interface {
	// Begin starts a new database transaction.
	// ctx: The context for the transaction.
	// opts: Optional arguments specifying transaction options (e.g., isolation level, read-only flag).
	// Returns: An instance of the started Tx interface and any error that occurred during transaction initiation.
	Begin(ctx context.Context, opts ...*sql.TxOptions) (Tx, error)
	// Commit commits the specified transaction, persisting all changes made within that transaction.
	// tx: The instance of the Tx interface to commit.
	// Returns: An error that occurred during the commit.
	Commit(tx Tx) error
	// Rollback rolls back the specified transaction, undoing all changes made within that transaction.
	// tx: The instance of the Tx interface to roll back.
	// Returns: An error that occurred during the rollback.
	Rollback(tx Tx) error
}

// TransactionManagerFactory is an abstract factory for creating TransactionManager instances from DBConnection.
// This allows for the generation of TransactionManagers that are independent of specific database connection types.
type TransactionManagerFactory interface {
	// NewTransactionManager creates a new TransactionManager based on the specified database connection.
	// conn: The database connection that the TransactionManager will manage.
	// Returns: A new TransactionManager instance.
	NewTransactionManager(conn adaptor.DBConnection) TransactionManager
}

// TransactionAdapter is an alias for the Tx interface.
// It is provided for compatibility maintenance as the framework evolves,
// allowing existing codebases to refer to the Tx interface.
//
// Deprecated: This type alias may be removed in the future.
// New code should use the `Tx` interface directly.
type TransactionAdapter = Tx
