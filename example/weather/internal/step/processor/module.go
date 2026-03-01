package processor

import (
	coreAdapter "github.com/tigerroll/surfin/pkg/batch/core/adapter"
	core "github.com/tigerroll/surfin/pkg/batch/core/application/port"
	config "github.com/tigerroll/surfin/pkg/batch/core/config"
	jsl "github.com/tigerroll/surfin/pkg/batch/core/config/jsl"
	support "github.com/tigerroll/surfin/pkg/batch/core/config/support"
	"github.com/tigerroll/surfin/pkg/batch/support/util/logger"

	"go.uber.org/fx"
)

// NewHourlyForecastTransformProcessorComponentBuilder creates a jsl.ComponentBuilder for the HourlyForecastTransformProcessor.
// This builder is used by the framework to construct an instance of the processor.
func NewHourlyForecastTransformProcessorComponentBuilder() jsl.ComponentBuilder {
	return jsl.ComponentBuilder(func(
		cfg *config.Config,
		resolver core.ExpressionResolver,
		resourceProviders map[string]coreAdapter.ResourceProvider,
		properties map[string]interface{},
	) (interface{}, error) {
		// resourceProviders are ignored as this processor does not use them.
		_ = resourceProviders
		processor, err := NewHourlyForecastTransformProcessor(cfg, resolver, properties)
		if err != nil {
			return nil, err
		}
		return processor, nil
	})
}

// RegisterHourlyForecastTransformProcessorBuilder registers the HourlyForecastTransformProcessor's ComponentBuilder with the JobFactory.
// This makes the "weatherItemProcessor" component available for use in JSL definitions.
//
// Parameters:
//
//	jf: The JobFactory instance to register the builder with.
//	builder: The jsl.ComponentBuilder for the HourlyForecastTransformProcessor.
func RegisterHourlyForecastTransformProcessorBuilder(
	jf *support.JobFactory,
	builder jsl.ComponentBuilder,
) {
	jf.RegisterComponentBuilder("weatherItemProcessor", builder)
	logger.Debugf("HourlyForecastTransformProcessor ComponentBuilder registered with JobFactory. JSL ref: 'weatherItemProcessor'")
}

// Module defines Fx options for the weather processor component.
// It provides the component builder and registers it with the support.JobFactory.
var Module = fx.Options(
	fx.Provide(fx.Annotate(
		NewHourlyForecastTransformProcessorComponentBuilder,
		fx.ResultTags(`name:"weatherItemProcessor"`),
	)),
	fx.Invoke(fx.Annotate(
		RegisterHourlyForecastTransformProcessorBuilder,
		fx.ParamTags(``, `name:"weatherItemProcessor"`),
	)),
)
