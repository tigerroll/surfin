package writer

import (
	"context"
	"fmt"

	"github.com/tigerroll/surfin/pkg/batch/core/application/port"
	"github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	"github.com/tigerroll/surfin/pkg/batch/core/tx"
	"github.com/tigerroll/surfin/pkg/batch/support/util/exception"
	"github.com/tigerroll/surfin/pkg/batch/support/util/logger"
)

// Package writer provides implementations for various item writers used in batch processing,
// facilitating the persistence of data to external systems.

// SqlBulkWriter is an implementation of [port.ItemWriter] that performs bulk writes to a database.
// It processes items in chunks and participates in an external transaction provided via the context.
// This writer leverages the [tx.Tx.ExecuteUpsert] method for efficient database operations,
// abstracting away database-specific SQL syntax for UPSERT/INSERT ON CONFLICT.
type SqlBulkWriter[T any] struct {
	name                 string                 // name is the unique name of the writer instance, used for logging and resource identification.
	bulkSize             int                    // bulkSize is the maximum number of items to process in a single database operation (chunk size).
	tableName            string                 // tableName is the name of the target database table where items will be written.
	conflictColumns      []string               // conflictColumns are the columns used for conflict resolution during UPSERT operations (e.g., primary keys).
	updateColumns        []string               // updateColumns are the columns to update if a conflict occurs during an UPSERT operation (empty for DO NOTHING).
	stepExecutionContext model.ExecutionContext // stepExecutionContext holds a reference to the Step's ExecutionContext, primarily for state management by the framework.
}

// NewSqlBulkWriter creates a new instance of [SqlBulkWriter].
//
// Parameters:
//
//	name: A unique name for this writer instance.
//	bulkSize: The maximum number of items to process in a single database operation.
//	tableName: The name of the target database table.
//	conflictColumns: The columns used for conflict resolution (e.g., primary keys for UPSERT).
//	updateColumns: The columns to update on conflict (empty for DO NOTHING).
//
// Returns:
//
//	A new [SqlBulkWriter] instance.
func NewSqlBulkWriter[T any](name string, bulkSize int, tableName string, conflictColumns []string, updateColumns []string) *SqlBulkWriter[T] {
	return &SqlBulkWriter[T]{
		name:            name,
		bulkSize:        bulkSize,
		tableName:       tableName,
		conflictColumns: conflictColumns,
		updateColumns:   updateColumns,
	}
}

// Verify that [SqlBulkWriter] implements the [port.ItemWriter] interface at compile time.
var _ port.ItemWriter[any] = (*SqlBulkWriter[any])(nil)

// Open initializes the writer.
// As [tx.Tx] interface handles internal statement management, no specific initialization
// or prepared statement creation is required within this method.
// The provided [model.ExecutionContext] is stored for later retrieval.
//
// Parameters:
//
//	ctx: The context for the operation.
//	ec: The [model.ExecutionContext] associated with the current step.
//
// Returns:
//
//	An error if initialization fails, otherwise nil.
func (w *SqlBulkWriter[T]) Open(ctx context.Context, ec model.ExecutionContext) error {
	logger.Infof("SqlBulkWriter '%s': Opened.", w.name)
	w.stepExecutionContext = ec // Store the ExecutionContext for potential use by the framework.
	return nil
}

// Write writes the given items to the database in chunks.
// This method expects a [tx.Tx] instance to be present in the provided [context.Context],
// as it participates in an external transaction managed by the framework.
//
// Parameters:
//
//	ctx: The context for the operation, expected to contain a [tx.Tx] instance.
//	items: A slice of items to be written to the database.
//
// Returns:
//
//	An error if any part of the write operation fails, otherwise nil.
func (w *SqlBulkWriter[T]) Write(ctx context.Context, items []T) error {
	if len(items) == 0 {
		return nil // Do nothing if there are no items to write.
	}

	// Retrieve the transaction from the context.
	currentTx, ok := tx.TxFromContext(ctx)
	if !ok {
		return exception.NewBatchError("writer", "transaction not found in context for SqlBulkWriter", nil, false, false)
	}

	// Divide the items slice into chunks based on bulkSize.
	for i := 0; i < len(items); i += w.bulkSize {
		end := i + w.bulkSize
		if end > len(items) {
			end = len(items)
		}
		chunk := items[i:end]

		// Perform bulk UPSERT using the transaction from context.
		// The Tx.ExecuteUpsert method handles its own statement preparation and execution.
		_, err := currentTx.ExecuteUpsert(ctx, chunk, w.tableName, w.conflictColumns, w.updateColumns)
		if err != nil {
			return exception.NewBatchError("writer", fmt.Sprintf("Failed to bulk upsert data for SqlBulkWriter '%s' (chunk start index %d)", w.name, i), err, false, false)
		}

		logger.Debugf("SqlBulkWriter '%s': Wrote %d items in chunk (start index %d).", w.name, len(chunk), i)
	}

	logger.Infof("SqlBulkWriter '%s': Successfully wrote all %d items.", w.name, len(items))
	return nil
}

// Close releases any resources held by the writer.
// As [tx.Tx] interface manages its own resources, no specific resource closing
// (e.g., prepared statements) is required within this method.
//
// Parameters:
//
//	ctx: The context for the operation.
//
// Returns:
//
//	An error if resource release fails, otherwise nil.
func (w *SqlBulkWriter[T]) Close(ctx context.Context) error {
	logger.Infof("SqlBulkWriter '%s': Closed.", w.name)
	return nil
}

// SetExecutionContext sets the [model.ExecutionContext] for the writer.
// [SqlBulkWriter] does not manage its own restartable state; it simply stores
// the provided context for potential use by the framework.
//
// Parameters:
//
//	ctx: The context for the operation.
//	ec: The [model.ExecutionContext] to be set.
//
// Returns:
//
//	An error if the context is cancelled, otherwise nil.
func (w *SqlBulkWriter[T]) SetExecutionContext(ctx context.Context, ec model.ExecutionContext) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	w.stepExecutionContext = ec
	return nil
}

// GetExecutionContext retrieves the current [model.ExecutionContext] from the writer.
// [SqlBulkWriter] does not manage its own restartable state; it returns the
// [model.ExecutionContext] that was provided during the [Open] or [SetExecutionContext] call.
//
// Parameters:
//
//	ctx: The context for the operation.
//
// Returns:
//
//	The current [model.ExecutionContext] and an error if the context is cancelled.
func (w *SqlBulkWriter[T]) GetExecutionContext(ctx context.Context) (model.ExecutionContext, error) {
	return w.stepExecutionContext, nil
}

// GetTargetResourceName returns the unique name of the target resource for this writer.
// This typically corresponds to the 'name' field provided during construction.
//
// Returns:
//
//	The name of the target resource.
func (w *SqlBulkWriter[T]) GetTargetResourceName() string {
	return w.name // Using the writer's name as the resource name
}

// GetResourcePath returns the path or identifier within the target resource for this writer.
// For [SqlBulkWriter], this is typically the target database table name.
//
// Returns:
//
//	The path or identifier within the target resource.
func (w *SqlBulkWriter[T]) GetResourcePath() string {
	return w.tableName // Using the table name as the resource path
}
