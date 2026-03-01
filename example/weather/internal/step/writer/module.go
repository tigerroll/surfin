// Package writer provides the Fx module for the WeatherItemWriter component.
// It defines how the WeatherItemWriter is built and registered within the application's dependency graph.
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

// HourlyForecastDatabaseWriterComponentBuilderParams defines the dependencies for [NewHourlyForecastDatabaseWriterComponentBuilder].
//
// Parameters:
//
//	fx.In: Fx-injected parameters.
//	AllDBConnections: A map of all established database connections, keyed by their name.
//	DBResolver: The database connection resolver.
type HourlyForecastDatabaseWriterComponentBuilderParams struct {
	fx.In
	DBResolver database.DBConnectionResolver
}

// NewHourlyForecastDatabaseWriterComponentBuilder creates a [jsl.ComponentBuilder] for the hourlyForecastDatabaseWriter.
//
// It now receives its core dependencies via Fx.
//
// Returns:
//
//	A jsl.ComponentBuilder function that can construct a HourlyForecastDatabaseWriter.
func NewHourlyForecastDatabaseWriterComponentBuilder(p HourlyForecastDatabaseWriterComponentBuilderParams) jsl.ComponentBuilder {
	return jsl.ComponentBuilder(func(
		cfg *config.Config,
		resolver core.ExpressionResolver, // The expression resolver for dynamic property resolution.
		resourceProviders map[string]coreAdapter.ResourceProvider,
		properties map[string]interface{},
	) (interface{}, error) {
		writer, err := NewHourlyForecastDatabaseWriter(cfg, resolver, p.DBResolver, properties)
		if err != nil {
			return nil, err
		}
		return writer, nil
	})
}

// RegisterHourlyForecastDatabaseWriterBuilder registers the created jsl.ComponentBuilder with the support.JobFactory.
// This makes the "hourlyForecastDatabaseWriter" component available for use in JSL definitions.
//
// Parameters:
//
//	jf: The JobFactory instance to register the builder with.
//	builder: The jsl.ComponentBuilder for the HourlyForecastDatabaseWriter.
func RegisterHourlyForecastDatabaseWriterBuilder(
	jf *support.JobFactory,
	builder jsl.ComponentBuilder,
) {
	jf.RegisterComponentBuilder("weatherItemWriter", builder)
	logger.Debugf("ComponentBuilder for HourlyForecastDatabaseWriter registered with JobFactory. JSL ref: 'weatherItemWriter'")
}

// Module defines the Fx options for the weather writer component.
// It provides the component builder and registers it with the support.JobFactory.
var Module = fx.Options(
	fx.Provide(fx.Annotate(
		NewHourlyForecastDatabaseWriterComponentBuilder,
		fx.ResultTags(`name:"weatherItemWriter"`),
	)),
	fx.Invoke(fx.Annotate(
		RegisterHourlyForecastDatabaseWriterBuilder,
		fx.ParamTags(``, `name:"weatherItemWriter"`),
	)),
)
