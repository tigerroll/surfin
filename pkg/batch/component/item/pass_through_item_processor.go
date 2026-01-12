package item

import (
	"context"

	port "github.com/tigerroll/surfin/pkg/batch/core/application/port"
	model "github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	"github.com/tigerroll/surfin/pkg/batch/support/util/logger"
)

// PassThroughItemProcessor is an implementation of [port.ItemProcessor] that returns the input item as the output item as is.
type PassThroughItemProcessor[T any] struct {
	ec model.ExecutionContext
}

// NewPassThroughItemProcessor creates a new instance of [PassThroughItemProcessor].
func NewPassThroughItemProcessor[T any]() port.ItemProcessor[T, T] {
	return &PassThroughItemProcessor[T]{
		ec: model.NewExecutionContext(),
	}
}

// Process returns the input item as is.
func (p *PassThroughItemProcessor[T]) Process(ctx context.Context, item T) (T, error) {
	logger.Debugf("PassThroughItemProcessor: Processing item: %+v", item)
	return item, nil
}

// SetExecutionContext sets the [model.ExecutionContext] for the processor.
func (p *PassThroughItemProcessor[T]) SetExecutionContext(ctx context.Context, ec model.ExecutionContext) error {
	p.ec = ec
	return nil
}

// GetExecutionContext retrieves the current [model.ExecutionContext] from the processor.
func (p *PassThroughItemProcessor[T]) GetExecutionContext(ctx context.Context) (model.ExecutionContext, error) {
	return p.ec, nil
}
