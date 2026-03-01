// Package item provides various item-related components for batch processing,
// including readers, processors, and writers.
package item

import (
	"context"
	"reflect"

	coreAdapter "github.com/tigerroll/surfin/pkg/batch/core/adapter"
	port "github.com/tigerroll/surfin/pkg/batch/core/application/port"
	config "github.com/tigerroll/surfin/pkg/batch/core/config"
	jsl "github.com/tigerroll/surfin/pkg/batch/core/config/jsl"
	support "github.com/tigerroll/surfin/pkg/batch/core/config/support"
	model "github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	logger "github.com/tigerroll/surfin/pkg/batch/support/util/logger"
)

// ExecutionContextItemWriter is an ItemWriter that stores the number of items written to the ExecutionContext.
// It is primarily used for testing and debugging, allowing the count of processed items to be
// easily inspected in the `ExecutionContext`.
type ExecutionContextItemWriter[I any] struct {
	// ec holds the current ExecutionContext, where the item count will be stored.
	ec model.ExecutionContext
	// key is the string key under which the item count will be stored in the ExecutionContext.
	key string
}

// NewExecutionContextItemWriter creates a new instance of ExecutionContextItemWriter.
//
// Parameters:
//
//	key: The key under which to store the item count in the ExecutionContext.
//	     If an empty string is provided, it defaults to "writer.write_count".
//
// Returns:
//
//	A new `port.ItemWriter` instance configured to write to the ExecutionContext.
func NewExecutionContextItemWriter[I any](key string) port.ItemWriter[I] {
	if key == "" {
		key = "writer.write_count"
	}
	return &ExecutionContextItemWriter[I]{
		ec:  model.NewExecutionContext(),
		key: key,
	}
}

// Open initializes the writer with the provided `ExecutionContext`.
//
// Parameters:
//
//	ctx: The context for the operation.
//	ec: The ExecutionContext to initialize the writer with.
func (w *ExecutionContextItemWriter[I]) Open(ctx context.Context, ec model.ExecutionContext) error {
	logger.Debugf("ExecutionContextItemWriter: Open called.")
	w.ec = ec
	return nil
}

// Write increments the count stored under the configured key in the `ExecutionContext`
// by the number of items provided in the current chunk.
//
// Parameters:
//
//	ctx: The context for the operation.
//	items: The items to be written.
func (w *ExecutionContextItemWriter[I]) Write(ctx context.Context, items []I) error {
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

// Close releases any resources held by the writer.
// For `ExecutionContextItemWriter`, there are no external resources to close, so this method does nothing.
//
// Parameters:
//
//	ctx: The context for the operation.
func (w *ExecutionContextItemWriter[I]) Close(ctx context.Context) error {
	logger.Debugf("ExecutionContextItemWriter: Close called.")
	return nil
}

// SetExecutionContext updates the internal `ExecutionContext` used by the writer.
//
// Parameters:
//
//	ctx: The context for the operation.
//	ec: The ExecutionContext to set.
func (w *ExecutionContextItemWriter[I]) SetExecutionContext(ctx context.Context, ec model.ExecutionContext) error {
	w.ec = ec
	return nil
}

// GetExecutionContext returns the current `ExecutionContext` held by the writer.
//
// Parameters:
//
//	ctx: The context for the operation.
//
// Returns:
//
//	model.ExecutionContext: The current ExecutionContext.
//	error: An error if retrieval fails.
func (w *ExecutionContextItemWriter[I]) GetExecutionContext(ctx context.Context) (model.ExecutionContext, error) {
	return w.ec, nil
}

// GetTargetResourceName returns the name of the target resource for this writer.
// Since `ExecutionContextItemWriter` does not interact with a specific resource, this method returns an empty string.
//
// Returns:
//
//	string: An empty string, as this writer does not write to a specific resource.
func (w *ExecutionContextItemWriter[I]) GetTargetResourceName() string {
	return "" // This writer does not write to a specific resource, so an empty string is returned.
}

// GetResourcePath returns the path or identifier within the target resource for this writer.
// Since `ExecutionContextItemWriter` does not interact with a specific resource, this method returns an empty string.
//
// Returns:
//
//	string: An empty string, as this writer does not write to a specific path.
func (w *ExecutionContextItemWriter[I]) GetResourcePath() string {
	return "" // This writer does not write to a specific path, so an empty string is returned.
}

// NewExecutionContextItemWriterBuilder creates a `jsl.ComponentBuilder` for `ExecutionContextItemWriter`.
//
// This builder function is responsible for instantiating `ExecutionContextItemWriter`
// with its required properties, typically the `key` for storing the count.
//
// Returns:
//
//	A `jsl.ComponentBuilder` function that can create `ExecutionContextItemWriter` instances.
func NewExecutionContextItemWriterBuilder() jsl.ComponentBuilder {
	return func(
		cfg *config.Config,
		resolver port.ExpressionResolver,
		resourceProviders map[string]coreAdapter.ResourceProvider,
		properties map[string]interface{},
	) (interface{}, error) {
		// Unused arguments are ignored for this component.
		_ = cfg
		_ = resolver
		_ = resourceProviders // This writer does not interact with a database directly.

		keyVal, ok := properties["key"]
		var key string
		if ok {
			if s, isString := keyVal.(string); isString {
				key = s
			} else {
				// Handle non-string type if necessary, or default
				logger.Warnf("ExecutionContextItemWriterBuilder: 'key' property is not a string, using default.")
				key = "writer.write_count"
			}
		} else {
			key = "writer.write_count"
		}
		// Return as ItemWriter[any]
		writer := NewExecutionContextItemWriter[any](key)
		return writer, nil
	}
}

// RegisterExecutionContextItemWriterBuilder registers the `ExecutionContextItemWriter` component builder with the `JobFactory`.
//
// This allows the framework to locate and use the `ExecutionContextItemWriter` when it's referenced in JSL (Job Specification Language) files.
// The component is registered under the name "executionContextItemWriter".
//
// Parameters:
//
//	jf: The JobFactory instance.
//	builder: The `ExecutionContextItemWriter` component builder.
func RegisterExecutionContextItemWriterBuilder(jf *support.JobFactory, builder jsl.ComponentBuilder) {
	jf.RegisterComponentBuilder("executionContextItemWriter", builder)
	logger.Debugf("Component 'executionContextItemWriter' was registered with JobFactory.")
}
