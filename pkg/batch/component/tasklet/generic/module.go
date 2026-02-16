// Package generic provides Fx modules for generic tasklet components.
package generic

import ( // Import for adapter.DBConnectionResolver
	coreAdapter "github.com/tigerroll/surfin/pkg/batch/core/adapter"
	port "github.com/tigerroll/surfin/pkg/batch/core/application/port"
	config "github.com/tigerroll/surfin/pkg/batch/core/config"
	jsl "github.com/tigerroll/surfin/pkg/batch/core/config/jsl"
	support "github.com/tigerroll/surfin/pkg/batch/core/config/support"
	"github.com/tigerroll/surfin/pkg/batch/support/util/logger"
	"go.uber.org/fx"
)

// NewExecutionContextWriterTaskletComponentBuilderParams defines the dependencies for NewExecutionContextWriterTaskletComponentBuilder.
type NewExecutionContextWriterTaskletComponentBuilderParams struct {
	fx.In
}

// NewExecutionContextWriterTaskletComponentBuilder creates a `jsl.ComponentBuilder` for `ExecutionContextWriterTasklet`.
//
// This builder function is responsible for instantiating `ExecutionContextWriterTasklet`
// with its required properties.
//
// Returns:
//
//	A `jsl.ComponentBuilder` function that can create `ExecutionContextWriterTasklet` instances.
func NewExecutionContextWriterTaskletComponentBuilder() jsl.ComponentBuilder {
	return jsl.ComponentBuilder(func(
		cfg *config.Config,
		resolver port.ExpressionResolver,
		resourceProviders map[string]coreAdapter.ResourceProvider,
		properties map[string]string,
	) (interface{}, error) {
		// Unused arguments are ignored for this component.
		_ = cfg
		_ = resolver
		_ = resourceProviders // This tasklet does not interact with resource providers directly.
		return NewExecutionContextWriterTasklet("executionContextWriterTasklet", properties), nil
	})
}

// RegisterExecutionContextWriterTaskletBuilder registers the `ExecutionContextWriterTasklet` component builder with the `JobFactory`.
//
// This allows the framework to locate and use the `ExecutionContextWriterTasklet` when it's referenced in JSL (Job Specification Language) files.
// The component is registered under the name "executionContextWriterTasklet".
//
// Parameters:
//
//	jf: The JobFactory instance.
//	builder: The `ExecutionContextWriterTasklet` component builder.
func RegisterExecutionContextWriterTaskletBuilder(jf *support.JobFactory, builder jsl.ComponentBuilder) {
	jf.RegisterComponentBuilder("executionContextWriterTasklet", builder)
	logger.Debugf("Component 'executionContextWriterTasklet' was registered with JobFactory.")
}

// NewRandomFailTaskletComponentBuilderParams defines the dependencies for NewRandomFailTaskletComponentBuilder.
type NewRandomFailTaskletComponentBuilderParams struct {
	fx.In
}

// NewRandomFailTaskletComponentBuilder creates a `jsl.ComponentBuilder` for `RandomFailTasklet`.
//
// This builder function is responsible for instantiating `RandomFailTasklet`
// with its required properties.
//
// Returns:
//
//	A `jsl.ComponentBuilder` function that can create `RandomFailTasklet` instances.
func NewRandomFailTaskletComponentBuilder() jsl.ComponentBuilder {
	return jsl.ComponentBuilder(func(
		cfg *config.Config,
		resolver port.ExpressionResolver,
		resourceProviders map[string]coreAdapter.ResourceProvider,
		properties map[string]string,
	) (interface{}, error) {
		// Unused arguments are ignored for this component.
		_ = cfg
		_ = resolver
		_ = resourceProviders // This tasklet does not interact with resource providers directly.
		return NewRandomFailTasklet("randomFailTasklet", properties), nil
	})
}

// RegisterRandomFailTaskletBuilder registers the `RandomFailTasklet` component builder with the `JobFactory`.
//
// This allows the framework to locate and use the `RandomFailTasklet` when it's referenced in JSL (Job Specification Language) files.
// The component is registered under the name "randomFailTasklet".
//
// Parameters:
//
//	jf: The JobFactory instance.
//	builder: The `RandomFailTasklet` component builder.
func RegisterRandomFailTaskletBuilder(jf *support.JobFactory, builder jsl.ComponentBuilder) {
	jf.RegisterComponentBuilder("randomFailTasklet", builder)
	logger.Debugf("Component 'randomFailTasklet' was registered with JobFactory.")
}

// Module provides `ComponentBuilders` for generic Tasklets and registers them with the `JobFactory`.
var Module = fx.Options(
	fx.Provide(fx.Annotate(
		NewExecutionContextWriterTaskletComponentBuilder,
		fx.ResultTags(`name:"executionContextWriterTasklet"`),
	)),
	fx.Invoke(fx.Annotate(
		RegisterExecutionContextWriterTaskletBuilder,
		fx.ParamTags(``, `name:"executionContextWriterTasklet"`),
	)),
	fx.Provide(fx.Annotate(
		NewRandomFailTaskletComponentBuilder,
		fx.ResultTags(`name:"randomFailTasklet"`),
	)),
	fx.Invoke(fx.Annotate(
		RegisterRandomFailTaskletBuilder,
		fx.ParamTags(``, `name:"randomFailTasklet"`),
	)),
)
