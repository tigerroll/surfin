// Package writer provides the Fx module for the WeatherItemWriter component.
// It defines how the WeatherItemWriter is built and registered within the application's dependency graph.
package writer

import (
	"fmt"
	"go.uber.org/fx"

	"github.com/tigerroll/surfin/pkg/batch/adapter/database"
	core "github.com/tigerroll/surfin/pkg/batch/core/application/port"
	config "github.com/tigerroll/surfin/pkg/batch/core/config"
	jsl "github.com/tigerroll/surfin/pkg/batch/core/config/jsl"
	support "github.com/tigerroll/surfin/pkg/batch/core/config/support"
	"github.com/tigerroll/surfin/pkg/batch/support/util/logger"
)

import coreAdapter "github.com/tigerroll/surfin/pkg/batch/core/adapter"

// WeatherWriterComponentBuilderParams defines the dependencies for [NewWeatherWriterComponentBuilder].
//
// Parameters:
//
//	fx.In: Fx-injected parameters.
//	AllDBConnections: A map of all established database connections, keyed by their name. This is provided by the main application module.
type WeatherWriterComponentBuilderParams struct {
	fx.In
	AllDBConnections map[string]database.DBConnection
}

// NewWeatherWriterComponentBuilder creates a [jsl.ComponentBuilder] for the weatherItemWriter.
//
// It now receives its core dependencies (AllDBConnections) via Fx.
//
// Returns:
//
//	A jsl.ComponentBuilder function that can construct a WeatherItemWriter.
func NewWeatherWriterComponentBuilder(p WeatherWriterComponentBuilderParams) jsl.ComponentBuilder {
	// Returns the builder function with a standard signature that JobFactory calls to construct the component.
	return jsl.ComponentBuilder(func(
		cfg *config.Config,
		resolver core.ExpressionResolver, // The expression resolver for dynamic property resolution.
		dbResolver coreAdapter.ResourceConnectionResolver,
		properties map[string]string,
	) (interface{}, error) {
		// Type assert dbResolver to database.DBConnectionResolver.
		dbConnResolver, ok := dbResolver.(database.DBConnectionResolver)
		if !ok {
			return nil, fmt.Errorf("dbResolver is not of type database.DBConnectionResolver")
		}
		// Pass allDBConnections injected from Fx to NewWeatherWriter.
		// The p.AllTxManagers dependency was removed as TxManager is now created on demand. This comment is now redundant.
		writer, err := NewWeatherWriter(cfg, p.AllDBConnections, resolver, dbConnResolver, properties)
		if err != nil {
			return nil, err
		}
		return writer, nil
	})
}

// RegisterWeatherWriterBuilder registers the created jsl.ComponentBuilder with the support.JobFactory.
// This makes the "weatherItemWriter" component available for use in JSL definitions.
//
// Parameters:
//
//	jf: The JobFactory instance to register the builder with.
//	builder: The jsl.ComponentBuilder for the WeatherItemWriter.
func RegisterWeatherWriterBuilder(
	jf *support.JobFactory,
	builder jsl.ComponentBuilder,
) {
	jf.RegisterComponentBuilder("weatherItemWriter", builder)
	logger.Debugf("ComponentBuilder for WeatherWriter registered with JobFactory. JSL ref: 'weatherItemWriter'")
}

// Module defines the Fx options for the weather writer component.
// It provides the component builder and registers it with the support.JobFactory.
var Module = fx.Options(
	fx.Provide(fx.Annotate(
		NewWeatherWriterComponentBuilder,
		fx.ResultTags(`name:"weatherItemWriter"`),
	)),
	fx.Invoke(fx.Annotate(
		RegisterWeatherWriterBuilder,
		fx.ParamTags(``, `name:"weatherItemWriter"`),
	)),
)
