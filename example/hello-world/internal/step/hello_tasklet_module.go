// Package step provides the Fx module for the HelloWorldTasklet component.
// It registers the tasklet builder with the JobFactory.
package step

import (
	"go.uber.org/fx"

	adapter "github.com/tigerroll/surfin/pkg/batch/core/adapter" // Add this import
	core "github.com/tigerroll/surfin/pkg/batch/core/application/port"
	config "github.com/tigerroll/surfin/pkg/batch/core/config" // Import config package
	jsl "github.com/tigerroll/surfin/pkg/batch/core/config/jsl"
	support "github.com/tigerroll/surfin/pkg/batch/core/config/support"
	logger "github.com/tigerroll/surfin/pkg/batch/support/util/logger"
)

// NewHelloWorldTaskletComponentBuilder creates a jsl.ComponentBuilder for HelloWorldTasklet.
// This builder function is responsible for instantiating HelloWorldTasklet
// with its required properties.
func NewHelloWorldTaskletComponentBuilder() jsl.ComponentBuilder {
	return jsl.ComponentBuilder(func(
		cfg *config.Config,
		resolver core.ExpressionResolver, // Keep this as core.ExpressionResolver
		dbResolver adapter.ResourceConnectionResolver, // Change to adapter.ResourceConnectionResolver
		properties map[string]string,
	) (interface{}, error) {
		// Unused arguments are ignored for this component.
		_ = cfg
		_ = resolver
		_ = dbResolver

		tasklet, err := NewHelloWorldTasklet(properties)
		if err != nil {
			return nil, err
		}
		return tasklet, nil
	})
}

// RegisterHelloWorldTaskletBuilder registers the created ComponentBuilder with the JobFactory.
//
// This allows the framework to locate and use the HelloWorldTasklet
// when it's referenced in JSL (Job Specification Language) files.
func RegisterHelloWorldTaskletBuilder(
	jf *support.JobFactory,
	builder jsl.ComponentBuilder,
) { // Register the ComponentBuilder with the JobFactory.
	// The key "helloWorldTasklet" must match the 'ref' attribute in the JSL (e.g., job.yaml).
	jf.RegisterComponentBuilder("helloWorldTasklet", builder)
	logger.Debugf("ComponentBuilder for HelloWorldTasklet registered with JobFactory. JSL ref: 'helloWorldTasklet'")
}

// Module defines the Fx options for the HelloWorldTasklet component.
// It provides the component builder and registers it with the JobFactory.
var Module = fx.Options(
	fx.Provide(fx.Annotate(
		NewHelloWorldTaskletComponentBuilder,
		fx.ResultTags(`name:"helloWorldTasklet"`),
	)),
	fx.Invoke(fx.Annotate(
		RegisterHelloWorldTaskletBuilder,
		fx.ParamTags(``, `name:"helloWorldTasklet"`),
	)),
)
