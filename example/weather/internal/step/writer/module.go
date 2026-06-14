package writer

import (
	"go.uber.org/fx"

	"github.com/tigerroll/surfin/pkg/batch/adapter/database"
	core "github.com/tigerroll/surfin/pkg/batch/core/application/port"
	config "github.com/tigerroll/surfin/pkg/batch/core/config"
	jsl "github.com/tigerroll/surfin/pkg/batch/core/config/jsl"
	support "github.com/tigerroll/surfin/pkg/batch/core/config/support"
	"github.com/tigerroll/surfin/pkg/batch/support/util/logger"
)

import coreAdapter "github.com/tigerroll/surfin/pkg/batch/core/adapter"

type HourlyForecastDatabaseWriterComponentBuilderParams struct {
	fx.In
	DBResolver database.DBConnectionResolver
}

func NewHourlyForecastDatabaseWriterComponentBuilder(p HourlyForecastDatabaseWriterComponentBuilderParams) jsl.ComponentBuilder {
	return jsl.ComponentBuilder(func(
		cfg *config.Config,
		resolver core.ExpressionResolver,
		resourceProviders map[string]coreAdapter.ResourceProvider,
		properties map[string]interface{},
	) (interface{}, error) {
		return NewHourlyForecastDatabaseWriter(cfg, resolver, p.DBResolver, properties)
	})
}

func NewHourlyHistoryDatabaseWriterComponentBuilder(p HourlyForecastDatabaseWriterComponentBuilderParams) jsl.ComponentBuilder {
	return jsl.ComponentBuilder(func(
		cfg *config.Config,
		resolver core.ExpressionResolver,
		resourceProviders map[string]coreAdapter.ResourceProvider,
		properties map[string]interface{},
	) (interface{}, error) {
		return NewHourlyHistoryDatabaseWriter(cfg, resolver, p.DBResolver, properties)
	})
}

func RegisterHourlyForecastDatabaseWriterBuilder(jf *support.JobFactory, builder jsl.ComponentBuilder) {
	jf.RegisterComponentBuilder("weatherItemWriter", builder)
	logger.Debugf("ComponentBuilder for HourlyForecastDatabaseWriter registered with JobFactory. JSL ref: 'weatherItemWriter'")
}

func RegisterHourlyHistoryDatabaseWriterBuilder(jf *support.JobFactory, builder jsl.ComponentBuilder) {
	jf.RegisterComponentBuilder("hourlyHistoryDatabaseWriter", builder)
	logger.Debugf("ComponentBuilder for HourlyHistoryDatabaseWriter registered with JobFactory. JSL ref: 'hourlyHistoryDatabaseWriter'")
}

var Module = fx.Options(
	fx.Provide(
		fx.Annotate(NewHourlyForecastDatabaseWriterComponentBuilder, fx.ResultTags(`name:"weatherItemWriter"`)),
		fx.Annotate(NewHourlyHistoryDatabaseWriterComponentBuilder, fx.ResultTags(`name:"hourlyHistoryDatabaseWriter"`)),
	),
	fx.Invoke(
		fx.Annotate(RegisterHourlyForecastDatabaseWriterBuilder, fx.ParamTags(``, `name:"weatherItemWriter"`)),
		fx.Annotate(RegisterHourlyHistoryDatabaseWriterBuilder, fx.ParamTags(``, `name:"hourlyHistoryDatabaseWriter"`)),
	),
)
