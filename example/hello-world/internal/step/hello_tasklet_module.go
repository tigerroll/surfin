// Package step provides the Fx module for the HelloWorldTasklet component,
// registering its builder with the JobFactory.
package step

import (
	"go.uber.org/fx"

	coreAdapter "github.com/tigerroll/surfin/pkg/batch/core/adapter"
	core "github.com/tigerroll/surfin/pkg/batch/core/application/port"
	config "github.com/tigerroll/surfin/pkg/batch/core/config"
	jsl "github.com/tigerroll/surfin/pkg/batch/core/config/jsl"
	support "github.com/tigerroll/surfin/pkg/batch/core/config/support"
	logger "github.com/tigerroll/surfin/pkg/batch/support/util/logger"
)

// NewHelloWorldTaskletComponentBuilder creates and returns a jsl.ComponentBuilder for the HelloWorldTasklet.
// This builder is responsible for instantiating the HelloWorldTasklet, binding its properties
// from the Job Specification Language (JSL) configuration.
//
// The returned ComponentBuilder is a function that takes:
//   - cfg: The application's core configuration (unused by this specific tasklet).
//   - resolver: An expression resolver (unused by this specific tasklet).
//   - resourceProviders: A map of resource providers (unused by this specific tasklet).
//   - properties: A map of string properties defined in the JSL for this tasklet.
//
// It returns the instantiated HelloWorldTasklet or an error if property binding fails.
func NewHelloWorldTaskletComponentBuilder() jsl.ComponentBuilder {
	return jsl.ComponentBuilder(func(
		cfg *config.Config,
		resolver core.ExpressionResolver,
		resourceProviders map[string]coreAdapter.ResourceProvider,
		properties map[string]string,
	) (interface{}, error) {
		// These arguments are part of the generic ComponentBuilder signature but are not
		// utilized by the simple HelloWorldTasklet.
		_ = cfg
		_ = resolver
		_ = resourceProviders

		tasklet, err := NewHelloWorldTasklet(properties)
		if err != nil {
			return nil, err
		}
		return tasklet, nil
	})
}

// RegisterHelloWorldTaskletBuilder registers the provided ComponentBuilder with the JobFactory.
// This makes the HelloWorldTasklet available for use within batch jobs defined in JSL.
//
// The registration key "helloWorldTasklet" must match the 'ref' attribute used in JSL
// (e.g., in job.yaml) to reference this specific tasklet.
//
// Parameters:
//   - jf: The JobFactory instance to register the builder with.
//   - builder: The jsl.ComponentBuilder for HelloWorldTasklet.
func RegisterHelloWorldTaskletBuilder(
	jf *support.JobFactory,
	builder jsl.ComponentBuilder,
) {
	jf.RegisterComponentBuilder("helloWorldTasklet", builder)
	logger.Debugf("ComponentBuilder for HelloWorldTasklet registered with JobFactory. JSL ref: 'helloWorldTasklet'")
}

// Module defines the Fx options for the HelloWorldTasklet component.
// It provides the NewHelloWorldTaskletComponentBuilder and registers it
// with the JobFactory, making the HelloWorldTasklet available for dependency injection
// and JSL configuration.
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
