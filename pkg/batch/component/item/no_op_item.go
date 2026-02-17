package item

import (
	"context"
	"io"

	port "github.com/tigerroll/surfin/pkg/batch/core/application/port"
	model "github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	"github.com/tigerroll/surfin/pkg/batch/support/util/logger"
)

// NoOpItemReader is an implementation of [port.ItemReader] that always returns [io.EOF].
type NoOpItemReader[O any] struct {
	ec model.ExecutionContext
}

// NewNoOpItemReader creates a new instance of [NoOpItemReader].
func NewNoOpItemReader[O any]() port.ItemReader[O] {
	return &NoOpItemReader[O]{
		ec: model.NewExecutionContext(),
	}
}

// Open initializes the reader with the given [model.ExecutionContext].
//
// Parameters:
//
//	ctx: The context for the operation.
//	ec: The ExecutionContext to initialize the reader with.
func (r *NoOpItemReader[O]) Open(ctx context.Context, ec model.ExecutionContext) error {
	logger.Debugf("NoOpItemReader: Open called.")
	r.ec = ec
	return nil
}

// Read always returns the zero value of type O and [io.EOF], indicating no more items.
//
// Parameters:
//
//	ctx: The context for the operation.
//
// Returns:
//
//	O: The zero value of type O.
//	error: Always returns [io.EOF].
func (r *NoOpItemReader[O]) Read(ctx context.Context) (O, error) {
	var zero O
	return zero, io.EOF
}

// Close releases any resources held by the reader.
//
// Parameters:
//
//	ctx: The context for the operation.
func (r *NoOpItemReader[O]) Close(ctx context.Context) error {
	logger.Debugf("NoOpItemReader: Close called.")
	return nil
}

// SetExecutionContext sets the [model.ExecutionContext] for the reader.
//
// Parameters:
//
//	ctx: The context for the operation.
//	ec: The ExecutionContext to set.
func (r *NoOpItemReader[O]) SetExecutionContext(ctx context.Context, ec model.ExecutionContext) error {
	r.ec = ec
	return nil
}

// GetExecutionContext retrieves the current [model.ExecutionContext] from the reader.
//
// Parameters:
//
//	ctx: The context for the operation.
//
// Returns:
//
//	model.ExecutionContext: The current ExecutionContext.
//	error: An error if retrieval fails.
func (r *NoOpItemReader[O]) GetExecutionContext(ctx context.Context) (model.ExecutionContext, error) {
	return r.ec, nil
}

// NoOpItemWriter is an implementation of [port.ItemWriter] that performs no operation.
type NoOpItemWriter[I any] struct {
	ec model.ExecutionContext
}

// NewNoOpItemWriter creates a new instance of [NoOpItemWriter].
func NewNoOpItemWriter[I any]() port.ItemWriter[I] {
	return &NoOpItemWriter[I]{
		ec: model.NewExecutionContext(),
	}
}

// Open initializes the writer with the given [model.ExecutionContext].
//
// Parameters:
//
//	ctx: The context for the operation.
//	ec: The ExecutionContext to initialize the writer with.
func (w *NoOpItemWriter[I]) Open(ctx context.Context, ec model.ExecutionContext) error {
	logger.Debugf("NoOpItemWriter: Open called.")
	w.ec = ec
	return nil
}

// Write performs no operation, effectively discarding the items.
//
// Parameters:
//
//	ctx: The context for the operation.
//	items: The items to be written.
func (w *NoOpItemWriter[I]) Write(ctx context.Context, items []I) error {
	logger.Debugf("NoOpItemWriter: Write called with %d items.", len(items))
	return nil
}

// Close releases any resources held by the writer.
//
// Parameters:
//
//	ctx: The context for the operation.
func (w *NoOpItemWriter[I]) Close(ctx context.Context) error {
	logger.Debugf("NoOpItemWriter: Close called.")
	return nil
}

// SetExecutionContext sets the [model.ExecutionContext] for the writer.
//
// Parameters:
//
//	ctx: The context for the operation.
//	ec: The ExecutionContext to set.
func (w *NoOpItemWriter[I]) SetExecutionContext(ctx context.Context, ec model.ExecutionContext) error {
	w.ec = ec
	return nil
}

// GetExecutionContext retrieves the current [model.ExecutionContext] from the writer.
//
// Parameters:
//
//	ctx: The context for the operation.
//
// Returns:
//
//	model.ExecutionContext: The current ExecutionContext.
//	error: An error if retrieval fails.
func (w *NoOpItemWriter[I]) GetExecutionContext(ctx context.Context) (model.ExecutionContext, error) {
	return w.ec, nil
}

// GetTargetResourceName returns the name of the target resource for this writer.
//
// Returns:
//
//	string: An empty string, as this writer does not write to a specific resource.
func (w *NoOpItemWriter[I]) GetTargetResourceName() string {
	return "" // This writer does not write to a specific resource, so an empty string is returned.
}

// GetResourcePath returns the path or identifier within the target resource for this writer.
//
// Returns:
//
//	string: An empty string, as this writer does not write to a specific path.
func (w *NoOpItemWriter[I]) GetResourcePath() string {
	return "" // This writer does not write to a specific path, so an empty string is returned.
}
