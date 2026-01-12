package item

import (
	"context"
	"io"
 
	port "surfin/pkg/batch/core/application/port"
	model "surfin/pkg/batch/core/domain/model"
	tx "surfin/pkg/batch/core/tx"
	"surfin/pkg/batch/support/util/logger"
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
func (r *NoOpItemReader[O]) Open(ctx context.Context, ec model.ExecutionContext) error {
	logger.Debugf("NoOpItemReader: Open called.")
	r.ec = ec
	return nil
}

// Read always returns the zero value of type O and [io.EOF], indicating no more items.
func (r *NoOpItemReader[O]) Read(ctx context.Context) (O, error) {
	var zero O
	return zero, io.EOF
}

// Close releases any resources held by the reader.
func (r *NoOpItemReader[O]) Close(ctx context.Context) error {
	logger.Debugf("NoOpItemReader: Close called.")
	return nil
}

// SetExecutionContext sets the [model.ExecutionContext] for the reader.
func (r *NoOpItemReader[O]) SetExecutionContext(ctx context.Context, ec model.ExecutionContext) error {
	r.ec = ec
	return nil
}

// GetExecutionContext retrieves the current [model.ExecutionContext] from the reader.
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
func (w *NoOpItemWriter[I]) Open(ctx context.Context, ec model.ExecutionContext) error {
	logger.Debugf("NoOpItemWriter: Open called.")
	w.ec = ec
	return nil
}

// Write performs no operation, effectively discarding the items.
func (w *NoOpItemWriter[I]) Write(ctx context.Context, tx tx.Tx, items []I) error {
	logger.Debugf("NoOpItemWriter: Write called with %d items.", len(items))
	return nil
}

// Close releases any resources held by the writer.
func (w *NoOpItemWriter[I]) Close(ctx context.Context) error {
	logger.Debugf("NoOpItemWriter: Close called.")
	return nil
}

// SetExecutionContext sets the [model.ExecutionContext] for the writer.
func (w *NoOpItemWriter[I]) SetExecutionContext(ctx context.Context, ec model.ExecutionContext) error {
	w.ec = ec
	return nil
}

// GetExecutionContext retrieves the current [model.ExecutionContext] from the writer.
func (w *NoOpItemWriter[I]) GetExecutionContext(ctx context.Context) (model.ExecutionContext, error) {
	return w.ec, nil
}
