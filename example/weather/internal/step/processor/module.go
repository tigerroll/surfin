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
// This function is called by Fx as a provider.
//
// Returns:
//
//	A jsl.ComponentBuilder function that can construct a HourlyForecastTransformProcessor.
func NewHourlyForecastTransformProcessorComponentBuilder() jsl.ComponentBuilder {
	return jsl.ComponentBuilder(func(
		cfg *config.Config,
		resolver core.ExpressionResolver,
		dbResolver coreAdapter.ResourceConnectionResolver,
		properties map[string]string,
	) (interface{}, error) {
		// Arguments unnecessary for this component are ignored.
		_ = dbResolver
		processor, err := NewHourlyForecastTransformProcessor(cfg, resolver, properties)
		if err != nil {
			return nil, err
		}
		return processor, nil
	})
}

func RegisterHourlyForecastTransformProcessorBuilder(
	jf *support.JobFactory,
	builder jsl.ComponentBuilder,
) {
	// Register the builder with the key "weatherItemProcessor" matching the 'ref' in JSL (job.yaml).
	jf.RegisterComponentBuilder("weatherItemProcessor", builder)
	logger.Debugf("HourlyForecastTransformProcessor ComponentBuilder registered with JobFactory. JSL ref: 'weatherItemProcessor'")
}

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
