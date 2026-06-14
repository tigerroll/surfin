package reader

import (
	coreAdapter "github.com/tigerroll/surfin/pkg/batch/core/adapter"
	core "github.com/tigerroll/surfin/pkg/batch/core/application/port"
	config "github.com/tigerroll/surfin/pkg/batch/core/config"
	jsl "github.com/tigerroll/surfin/pkg/batch/core/config/jsl"
	support "github.com/tigerroll/surfin/pkg/batch/core/config/support"
	"github.com/tigerroll/surfin/pkg/batch/support/util/logger"

	"go.uber.org/fx"
)

func NewHourlyForecastAPIReaderComponentBuilder() jsl.ComponentBuilder {
	return jsl.ComponentBuilder(func(
		cfg *config.Config,
		resolver core.ExpressionResolver,
		resourceProviders map[string]coreAdapter.ResourceProvider,
		properties map[string]interface{},
	) (interface{}, error) {
		return NewHourlyForecastAPIReader(cfg, resolver, resourceProviders, properties)
	})
}

func NewHourlyHistoryAPIReaderComponentBuilder() jsl.ComponentBuilder {
	return jsl.ComponentBuilder(func(
		cfg *config.Config,
		resolver core.ExpressionResolver,
		resourceProviders map[string]coreAdapter.ResourceProvider,
		properties map[string]interface{},
	) (interface{}, error) {
		return NewHourlyHistoryAPIReader(cfg, resolver, resourceProviders, properties)
	})
}

func RegisterHourlyForecastAPIReaderBuilder(jf *support.JobFactory, builder jsl.ComponentBuilder) {
	jf.RegisterComponentBuilder("weatherItemReader", builder)
	logger.Debugf("ComponentBuilder for HourlyForecastAPIReader registered with JobFactory. JSL ref: 'weatherItemReader'")
}

func RegisterHourlyHistoryAPIReaderBuilder(jf *support.JobFactory, builder jsl.ComponentBuilder) {
	jf.RegisterComponentBuilder("hourlyHistoryAPIReader", builder)
	logger.Debugf("ComponentBuilder for HourlyHistoryAPIReader registered with JobFactory. JSL ref: 'hourlyHistoryAPIReader'")
}

var Module = fx.Options(
	fx.Provide(
		fx.Annotate(NewHourlyForecastAPIReaderComponentBuilder, fx.ResultTags(`name:"weatherItemReader"`)),
		fx.Annotate(NewHourlyHistoryAPIReaderComponentBuilder, fx.ResultTags(`name:"hourlyHistoryAPIReader"`)),
	),
	fx.Invoke(
		fx.Annotate(RegisterHourlyForecastAPIReaderBuilder, fx.ParamTags(``, `name:"weatherItemReader"`)),
		fx.Annotate(RegisterHourlyHistoryAPIReaderBuilder, fx.ParamTags(``, `name:"hourlyHistoryAPIReader"`)),
	),
)
