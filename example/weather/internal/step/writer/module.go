// Package writer provides the Fx module for the WeatherItemWriter component.
// It defines how the WeatherItemWriter is built and registered within the application's dependency graph.
package writer

import (
	"go.uber.org/fx"

	core "github.com/tigerroll/surfin/pkg/batch/core/application/port"  // Core application interfaces
	config "github.com/tigerroll/surfin/pkg/batch/core/config"          // Application configuration
	jsl "github.com/tigerroll/surfin/pkg/batch/core/config/jsl"         // Job Specification Language (JSL) definitions
	support "github.com/tigerroll/surfin/pkg/batch/core/config/support" // Support utilities for configuration
	"github.com/tigerroll/surfin/pkg/batch/support/util/logger"

	"github.com/tigerroll/surfin/pkg/batch/core/adapter" // Database adapter interfaces
)

// WeatherWriterComponentBuilderParams defines the dependencies for NewWeatherWriterComponentBuilder.
//
// Parameters:
//
//	fx.In: Fx-injected parameters.
//	AllDBConnections: A map of all established database connections, keyed by their name.
type WeatherWriterComponentBuilderParams struct {
	fx.In
	// AllDBConnections is a map of all established database connections,
	// provided by the main application module.
	AllDBConnections map[string]adapter.DBConnection
}

// NewWeatherWriterComponentBuilder creates a jsl.ComponentBuilder for the weatherItemWriter.
// It now receives its core dependencies (AllDBConnections) via Fx.
//
// Returns:
//
//	A jsl.ComponentBuilder function that can construct a WeatherItemWriter.
func NewWeatherWriterComponentBuilder(p WeatherWriterComponentBuilderParams) jsl.ComponentBuilder {
	// Returns the builder function with a standard signature that JobFactory calls to construct the component.
	return jsl.ComponentBuilder(func(
		cfg *config.Config,
		resolver core.ExpressionResolver,
		dbResolver core.DBConnectionResolver,
		properties map[string]string,
	) (interface{}, error) {

		// Pass allDBConnections injected from Fx to NewWeatherWriter.
		// The p.AllTxManagers dependency was removed as TxManager is now created on demand.
		writer, err := NewWeatherWriter(cfg, p.AllDBConnections, resolver, dbResolver, properties)
		if err != nil {
			return nil, err
		}
		return writer, nil
	})
}

// RegisterWeatherWriterBuilder registers the created ComponentBuilder with the JobFactory.
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
// It provides the component builder and registers it with the JobFactory.
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
