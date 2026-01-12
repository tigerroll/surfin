// Package item provides various item-related components for batch processing,
// including readers, processors, and writers.
package item

import (
	"context"
	"reflect"

	port "github.com/tigerroll/surfin/pkg/batch/core/application/port"
	model "github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	tx "github.com/tigerroll/surfin/pkg/batch/core/tx"
	logger "github.com/tigerroll/surfin/pkg/batch/support/util/logger"
	config "github.com/tigerroll/surfin/pkg/batch/core/config"
	jsl "github.com/tigerroll/surfin/pkg/batch/core/config/jsl"
	support "github.com/tigerroll/surfin/pkg/batch/core/config/support"
	repository "github.com/tigerroll/surfin/pkg/batch/core/domain/repository"
)

// ExecutionContextItemWriter is an ItemWriter that stores the number of items written to the ExecutionContext.
// It is primarily used for testing and debugging.
type ExecutionContextItemWriter[I any] struct {
	ec model.ExecutionContext
	key string // Key to store the count in the ExecutionContext.
}

// NewExecutionContextItemWriter creates a new instance of ExecutionContextItemWriter.
func NewExecutionContextItemWriter[I any](key string) port.ItemWriter[I] {
	if key == "" {
		key = "writer.write_count"
	}
	return &ExecutionContextItemWriter[I]{
		ec: model.NewExecutionContext(),
		key: key,
	}
}

// Open opens resources.
func (w *ExecutionContextItemWriter[I]) Open(ctx context.Context, ec model.ExecutionContext) error {
	logger.Debugf("ExecutionContextItemWriter: Open called.")
	w.ec = ec
	return nil
}

// Write stores the number of items in the ExecutionContext.
func (w *ExecutionContextItemWriter[I]) Write(ctx context.Context, tx tx.Tx, items []I) error {
	logger.Debugf("ExecutionContextItemWriter: Writing %d items to ExecutionContext key '%s'.", len(items), w.key)

	// Retrieve existing count.
	currentCount, ok := w.ec.GetInt(w.key)
	if !ok {
		currentCount = 0
	}
	
	newCount := currentCount + len(items)
	w.ec.Put(w.key, newCount)
	
	logger.Debugf("ExecutionContextItemWriter: Updated count to %d.", newCount)
	
	// Log item contents for debugging.
	for i, item := range items {
		logger.Debugf("Item %d: Type=%s, Value=%+v", i, reflect.TypeOf(item), item)
	}
	
	return nil
}

// Close closes resources.
func (w *ExecutionContextItemWriter[I]) Close(ctx context.Context) error {
	logger.Debugf("ExecutionContextItemWriter: Close called.")
	return nil
}

// SetExecutionContext sets the ExecutionContext.
func (w *ExecutionContextItemWriter[I]) SetExecutionContext(ctx context.Context, ec model.ExecutionContext) error {
	w.ec = ec
	return nil
}

// GetExecutionContext retrieves the ExecutionContext.
func (w *ExecutionContextItemWriter[I]) GetExecutionContext(ctx context.Context) (model.ExecutionContext, error) {
	return w.ec, nil
}

// ComponentBuilder for ExecutionContextItemWriter (used by JobFactory)
func NewExecutionContextItemWriterBuilder() jsl.ComponentBuilder {
	return func(
		cfg *config.Config,
		repo repository.JobRepository,
		resolver port.ExpressionResolver,
		dbResolver port.DBConnectionResolver,
		properties map[string]string,
	) (interface{}, error) {
		// Arguments unnecessary for this component are ignored.
		_ = cfg
		_ = repo
		_ = resolver
		_ = dbResolver

		key, ok := properties["key"]
		if !ok {
			key = "writer.write_count"
		}
		// Return as ItemWriter[any]
		writer := NewExecutionContextItemWriter[any](key)
		return writer, nil
	}
}

// RegisterExecutionContextItemWriterBuilder registers the builder with the JobFactory.
func RegisterExecutionContextItemWriterBuilder(jf *support.JobFactory, builder jsl.ComponentBuilder) {
	jf.RegisterComponentBuilder("executionContextItemWriter", builder)
	logger.Debugf("Component 'executionContextItemWriter' was registered with JobFactory.")
}
