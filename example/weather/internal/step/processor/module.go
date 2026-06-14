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
func NewHourlyForecastTransformProcessorComponentBuilder() jsl.ComponentBuilder {
	return jsl.ComponentBuilder(func(
		cfg *config.Config,
		resolver core.ExpressionResolver,
		resourceProviders map[string]coreAdapter.ResourceProvider,
		properties map[string]interface{},
	) (interface{}, error) {
		processor, err := NewHourlyForecastTransformProcessor(cfg, resolver, properties)
		if err != nil {
			return nil, err
		}
		return processor, nil
	})
}

// NewHourlyHistoryTransformProcessorComponentBuilder creates a jsl.ComponentBuilder for the HourlyHistoryTransformProcessor.
func NewHourlyHistoryTransformProcessorComponentBuilder() jsl.ComponentBuilder {
	return jsl.ComponentBuilder(func(
		cfg *config.Config,
		resolver core.ExpressionResolver,
		resourceProviders map[string]coreAdapter.ResourceProvider,
		properties map[string]interface{},
	) (interface{}, error) {
		return NewHourlyHistoryTransformProcessor(cfg, resolver, properties)
	})
}

// RegisterHourlyForecastTransformProcessorBuilder registers the HourlyForecastTransformProcessor builder with the JobFactory.
func RegisterHourlyForecastTransformProcessorBuilder(jf *support.JobFactory, builder jsl.ComponentBuilder) {
	jf.RegisterComponentBuilder("weatherItemProcessor", builder)
	logger.Debugf("HourlyForecastTransformProcessor ComponentBuilder registered with JobFactory. JSL ref: 'weatherItemProcessor'")
}

// RegisterHourlyHistoryTransformProcessorBuilder registers the HourlyHistoryTransformProcessor builder with the JobFactory.
func RegisterHourlyHistoryTransformProcessorBuilder(jf *support.JobFactory, builder jsl.ComponentBuilder) {
	jf.RegisterComponentBuilder("hourlyHistoryTransformProcessor", builder)
	logger.Debugf("HourlyHistoryTransformProcessor ComponentBuilder registered with JobFactory. JSL ref: 'hourlyHistoryTransformProcessor'")
}

// Module provides the Fx module for registering weather-related item processors.
var Module = fx.Options(
	fx.Provide(
		fx.Annotate(NewHourlyForecastTransformProcessorComponentBuilder, fx.ResultTags(`name:"weatherItemProcessor"`)),
		fx.Annotate(NewHourlyHistoryTransformProcessorComponentBuilder, fx.ResultTags(`name:"hourlyHistoryTransformProcessor"`)),
	),
	fx.Invoke(
		fx.Annotate(RegisterHourlyForecastTransformProcessorBuilder, fx.ParamTags(``, `name:"weatherItemProcessor"`)),
		fx.Annotate(RegisterHourlyHistoryTransformProcessorBuilder, fx.ParamTags(``, `name:"hourlyHistoryTransformProcessor"`)),
	),
)
