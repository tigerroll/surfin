package reader

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"

	"github.com/tigerroll/surfin/pkg/batch/core/application/port"
	"github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	"github.com/tigerroll/surfin/pkg/batch/support/util/exception"
	"github.com/tigerroll/surfin/pkg/batch/support/util/logger"
)

// SqlCursorReader is an ItemReader implementation that reads data from a database cursor.
// It is inspired by Spring Batch's JdbcCursorItemReader, providing efficient and restartable
// reading of large datasets.
type SqlCursorReader[T any] struct {
	db        *sql.DB                    // db is the database connection.
	name      string                     // name is the unique name of the reader, used for storing state in the ExecutionContext.
	baseQuery string                     // baseQuery is the SQL query without OFFSET/LIMIT clauses.
	baseArgs  []any                      // baseArgs are the arguments for the base query.
	mapper    func(*sql.Rows) (T, error) // mapper is a function that maps a *sql.Rows to a value of type T.
	rows      *sql.Rows                  // rows is the result set from the executed query.
	readCount int                        // readCount is the current number of items read, used for restartability.
	ec        model.ExecutionContext     // ec is the internal ExecutionContext for storing and retrieving state.
}

// NewSqlCursorReader creates a new instance of SqlCursorReader.
func NewSqlCursorReader[T any](db *sql.DB, name string, query string, args []any, mapper func(*sql.Rows) (T, error)) *SqlCursorReader[T] {
	return &SqlCursorReader[T]{
		db:        db,
		name:      name,
		baseQuery: query,
		baseArgs:  args,
		mapper:    mapper,
	}
}

// Open initializes the reader and executes the database query.
// It reads the previous read position from the ExecutionContext to resume processing.
func (r *SqlCursorReader[T]) Open(ctx context.Context, ec model.ExecutionContext) error {
	r.ec = ec // Store the provided ExecutionContext
	readCountKey := r.name + ".readCount"

	// Corrected: Capture both return values from GetInt
	startOffset, found := r.ec.GetInt(readCountKey)
	if found {
		r.readCount = startOffset
	} else {
		r.readCount = 0 // Initialize if not found
	}

	query := r.baseQuery
	args := r.baseArgs

	if r.readCount > 0 { // Use r.readCount for offset
		// Reconstruct the query by adding OFFSET.
		// Note: The syntax for LIMIT/OFFSET may vary depending on the database type.
		// A more generic approach might be needed in production code.
		query = fmt.Sprintf("%s OFFSET ?", r.baseQuery)
		args = append(args, r.readCount)
		logger.Infof("SqlCursorReader '%s': Resuming from offset %d. Query: %s", r.name, r.readCount, query)
	} else {
		logger.Infof("SqlCursorReader '%s': Starting new read. Query: %s", r.name, query)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return exception.NewBatchError("reader", fmt.Sprintf("Failed to execute query for SqlCursorReader '%s'", r.name), err, false, false)
	}
	r.rows = rows

	return nil
}

// Read reads the next data item.
// It returns io.EOF if no more data is available. The context can be used for cancellation.
func (r *SqlCursorReader[T]) Read(ctx context.Context) (T, error) {
	var item T
	if r.rows == nil {
		return item, exception.NewBatchError("reader", fmt.Sprintf("SqlCursorReader '%s': Reader not opened or already closed.", r.name), errors.New("reader not initialized"), false, false)
	}

	// Advance the cursor to the next row.
	// If Open() already called Next(), this will move to the second row on the first Read() call.
	// If Open() did not call Next(), this would move to the first row.
	// Given the current Open() implementation calls Next(), the first Read() will process the second row.
	// This might differ from typical ItemReader behavior where Read() processes the *current* row after Open().
	// However, for simplicity and consistency with how database cursors are often iterated in Go,
	// we proceed by always calling Next() here.
	if !r.rows.Next() {
		if err := r.rows.Err(); err != nil {
			return item, exception.NewBatchError("reader", fmt.Sprintf("Error during row iteration for SqlCursorReader '%s'", r.name), err, false, false)
		}
		// No more data.
		return item, io.EOF
	}

	// Map the data using the mapper function.
	mappedItem, err := r.mapper(r.rows)
	if err != nil {
		return item, exception.NewBatchError("reader", fmt.Sprintf("Failed to map row for SqlCursorReader '%s'", r.name), err, false, false)
	}

	// Increment read count and store in internal ExecutionContext
	r.readCount++
	r.ec.Put(r.name+".readCount", r.readCount) // Update the internal EC

	return mappedItem, nil
}

// Close releases the resources used (e.g., database cursor).
func (r *SqlCursorReader[T]) Close(ctx context.Context) error {
	if r.rows != nil {
		err := r.rows.Close()
		if err != nil {
			return exception.NewBatchError("reader", fmt.Sprintf("Failed to close rows for SqlCursorReader '%s'", r.name), err, false, false)
		}
		r.rows = nil // Set to nil after closing.
	}
	logger.Infof("SqlCursorReader '%s': Resources closed.", r.name)
	return nil
}

// GetExecutionContext returns the current ExecutionContext of the reader.
// This is used for restartability.
func (r *SqlCursorReader[T]) GetExecutionContext(ctx context.Context) (model.ExecutionContext, error) {
	if r.ec == nil {
		return model.NewExecutionContext(), nil // Return empty if not initialized
	}
	return r.ec, nil
}

// SetExecutionContext sets the ExecutionContext for the reader.
// This is used to restore state for restartability.
func (r *SqlCursorReader[T]) SetExecutionContext(ctx context.Context, ec model.ExecutionContext) error {
	r.ec = ec
	// When setting EC, we should also update the internal readCount
	readCountKey := r.name + ".readCount"
	if val, found := ec.GetInt(readCountKey); found {
		r.readCount = val
	} else {
		r.readCount = 0
	}
	return nil
}

// Verify that SqlCursorReader implements the port.ItemReader interface at compile time.
var _ port.ItemReader[any] = (*SqlCursorReader[any])(nil)
