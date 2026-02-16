// Package item provides Fx modules for various item-related components,
// including No-Op readers/writers, pass-through processors, and execution context writers.
package item

import (
	"go.uber.org/fx"

	coreAdapter "github.com/tigerroll/surfin/pkg/batch/core/adapter"
	port "github.com/tigerroll/surfin/pkg/batch/core/application/port"
	config "github.com/tigerroll/surfin/pkg/batch/core/config"
	jsl "github.com/tigerroll/surfin/pkg/batch/core/config/jsl"
	support "github.com/tigerroll/surfin/pkg/batch/core/config/support"
	"github.com/tigerroll/surfin/pkg/batch/support/util/logger"
)

// NewNoOpItemReaderComponentBuilder creates a `jsl.ComponentBuilder` for a No-Operation (No-Op) `ItemReader`.
//
// This builder function is designed to instantiate `NoOpItemReader` which always returns `port.ErrNoMoreItems`,
// effectively acting as a reader that has no items to read. It does not require any specific properties.
//
// Returns:
//
//	A `jsl.ComponentBuilder` function that can create `NoOpItemReader` instances.
func NewNoOpItemReaderComponentBuilder() jsl.ComponentBuilder {
	return func(
		_ *config.Config, // Not used by NoOpItemReader
		_ port.ExpressionResolver, // Not used by NoOpItemReader
		resourceProviders map[string]coreAdapter.ResourceProvider,
		_ map[string]string, // Not used by NoOpItemReader
	) (interface{}, error) {
		_ = resourceProviders // This reader does not use resource providers.
		return NewNoOpItemReader[any](), nil
	}
}

// NewPassThroughItemProcessorComponentBuilder creates a `jsl.ComponentBuilder` for a Pass-Through `ItemProcessor`.
//
// This builder function is designed to instantiate `PassThroughItemProcessor` which simply returns
// the input item as is, without any modification. It does not require any specific properties.
//
// Returns:
//
//	A `jsl.ComponentBuilder` function that can create `PassThroughItemProcessor` instances.
func NewPassThroughItemProcessorComponentBuilder() jsl.ComponentBuilder {
	return func(
		_ *config.Config, // Not used by PassThroughItemProcessor
		_ port.ExpressionResolver, // Not used by PassThroughItemProcessor
		resourceProviders map[string]coreAdapter.ResourceProvider,
		_ map[string]string, // Not used by PassThroughItemProcessor
	) (interface{}, error) {
		_ = resourceProviders // This processor does not use resource providers.
		return NewPassThroughItemProcessor[any](), nil
	}
}

// NewNoOpItemWriterComponentBuilder creates a `jsl.ComponentBuilder` for a No-Operation (No-Op) `ItemWriter`.
//
// This builder function is designed to instantiate `NoOpItemWriter` which performs no actual writing
// operations. It is useful for testing or scenarios where writing is not required.
// It does not require any specific properties.
//
// Returns:
//
//	A `jsl.ComponentBuilder` function that can create `NoOpItemWriter` instances.
func NewNoOpItemWriterComponentBuilder() jsl.ComponentBuilder {
	return func(
		_ *config.Config, // Not used by NoOpItemWriter
		_ port.ExpressionResolver, // Not used by NoOpItemReader
		resourceProviders map[string]coreAdapter.ResourceProvider,
		_ map[string]string, // Not used by NoOpItemWriter
	) (interface{}, error) {
		_ = resourceProviders // This writer does not use resource providers.
		return NewNoOpItemWriter[any](), nil
	}
}

// NewExecutionContextItemWriterComponentBuilder creates a `jsl.ComponentBuilder` for `ExecutionContextItemWriter`.
//
// This builder function is responsible for instantiating `ExecutionContextItemWriter`
// with its required properties, typically the `key` for storing the item count.
//
// Returns:
//
//	A `jsl.ComponentBuilder` function that can create `ExecutionContextItemWriter` instances.
func NewExecutionContextItemWriterComponentBuilder() jsl.ComponentBuilder {
	return func(
		cfg *config.Config,
		resolver port.ExpressionResolver,
		resourceProviders map[string]coreAdapter.ResourceProvider,
		properties map[string]string,
	) (interface{}, error) {
		// Arguments unnecessary for this component are ignored.
		_ = cfg               // Not used by ExecutionContextItemWriter
		_ = resolver          // Not used by ExecutionContextItemWriter
		_ = resourceProviders // Not used by ExecutionContextItemWriter

		key, ok := properties["key"]
		if !ok || key == "" {
			key = "writer_context"
		}
		return NewExecutionContextItemWriter[any](key), nil
	}
}

// genericItemBuilders is a struct used by Fx to inject all generic item component builders.
// Each field is annotated with `name` to specify the unique name under which the builder is provided.
type genericItemBuilders struct {
	fx.In
	NoOpReaderBuilder          jsl.ComponentBuilder `name:"noOpItemReader"`
	PassThroughProcBuilder     jsl.ComponentBuilder `name:"passThroughItemProcessor"`
	NoOpWriterBuilder          jsl.ComponentBuilder `name:"noOpItemWriter"`
	ExecutionContextItemWriter jsl.ComponentBuilder `name:"executionContextItemWriter"` // Builder for ExecutionContextItemWriter
}

// RegisterGenericItemBuilders registers all generic item component builders with the `JobFactory`.
//
// This function is invoked by the Fx framework to make these builders available for JSL parsing.
// It maps the named component builders to their respective JSL reference names.
//
// Registered components:
//   - "noOpItemReader"
//   - "passThroughItemProcessor"
//   - "noOpItemWriter"
//   - "executionContextItemWriter"
//
// Parameters:
//
//	jf: The `JobFactory` instance to register builders with.
//	builders: A struct containing all generic item component builders injected by Fx.
func RegisterGenericItemBuilders(jf *support.JobFactory, builders genericItemBuilders) {
	jf.RegisterComponentBuilder("noOpItemReader", builders.NoOpReaderBuilder)
	jf.RegisterComponentBuilder("passThroughItemProcessor", builders.PassThroughProcBuilder)
	jf.RegisterComponentBuilder("noOpItemWriter", builders.NoOpWriterBuilder)
	jf.RegisterComponentBuilder("executionContextItemWriter", builders.ExecutionContextItemWriter)
	logger.Debugf("Generic item components (noOpItemReader, passThroughItemProcessor, noOpItemWriter, executionContextItemWriter) were registered with JobFactory.")
}

// Module provides `ComponentBuilders` for generic item-related components and registers them with the `JobFactory`.
//
// This Fx module encapsulates the dependency injection setup for `NoOpItemReader`, `PassThroughItemProcessor`,
// `NoOpItemWriter`, and `ExecutionContextItemWriter`.
var Module = fx.Options(
	fx.Provide(fx.Annotate(
		NewNoOpItemReaderComponentBuilder,
		fx.ResultTags(`name:"noOpItemReader"`),
	)),
	fx.Provide(fx.Annotate(
		NewPassThroughItemProcessorComponentBuilder,
		fx.ResultTags(`name:"passThroughItemProcessor"`),
	)),
	fx.Provide(fx.Annotate(
		NewNoOpItemWriterComponentBuilder,
		fx.ResultTags(`name:"noOpItemWriter"`),
	)),
	fx.Provide(fx.Annotate(
		NewExecutionContextItemWriterComponentBuilder,
		fx.ResultTags(`name:"executionContextItemWriter"`),
	)),
	fx.Invoke(RegisterGenericItemBuilders),
)
