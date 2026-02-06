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

// NewWeatherProcessorComponentBuilder creates a jsl.ComponentBuilder for the WeatherProcessor.
// This function is called by Fx as a provider.
//
// Returns:
//
//	A jsl.ComponentBuilder function that can construct a WeatherProcessor.
func NewWeatherProcessorComponentBuilder() jsl.ComponentBuilder {
	return jsl.ComponentBuilder(func(
		cfg *config.Config,
		resolver core.ExpressionResolver,
		dbResolver coreAdapter.ResourceConnectionResolver,
		properties map[string]string,
	) (interface{}, error) {
		// Arguments unnecessary for this component are ignored.
		_ = dbResolver
		processor, err := NewWeatherProcessor(cfg, resolver, properties)
		if err != nil {
			return nil, err
		}
		return processor, nil
	})
}

func RegisterWeatherProcessorBuilder(
	jf *support.JobFactory,
	builder jsl.ComponentBuilder,
) {
	jf.RegisterComponentBuilder("weatherItemProcessor", builder)
	logger.Debugf("WeatherProcessor ComponentBuilder registered with JobFactory. JSL ref: 'weatherItemProcessor'")
}

var Module = fx.Options(
	fx.Provide(fx.Annotate(
		NewWeatherProcessorComponentBuilder,
		fx.ResultTags(`name:"weatherItemProcessor"`),
	)),
	fx.Invoke(fx.Annotate(
		RegisterWeatherProcessorBuilder,
		fx.ParamTags(``, `name:"weatherItemProcessor"`),
	)),
)
