package reader

import (
	core "github.com/tigerroll/surfin/pkg/batch/core/application/port"
	config "github.com/tigerroll/surfin/pkg/batch/core/config"
	jsl "github.com/tigerroll/surfin/pkg/batch/core/config/jsl"
	support "github.com/tigerroll/surfin/pkg/batch/core/config/support"
	"github.com/tigerroll/surfin/pkg/batch/support/util/logger"

	"go.uber.org/fx"
)

// NewWeatherReaderComponentBuilderParams defines the dependencies for NewWeatherReaderComponentBuilder.
type NewWeatherReaderComponentBuilderParams struct {
	fx.In
}

// NewWeatherReaderComponentBuilder creates a jsl.ComponentBuilder for the weatherReader.
// This function is called by Fx as a provider.
func NewWeatherReaderComponentBuilder() jsl.ComponentBuilder {
	// Returns the actual builder function that JobFactory calls to construct the component.
	return jsl.ComponentBuilder(func(
		cfg *config.Config,
		resolver core.ExpressionResolver,
		dbResolver core.DBConnectionResolver,
		properties map[string]string,
	) (interface{}, error) {
		// Arguments unnecessary for this component are ignored.
		_ = dbResolver
		reader, err := NewWeatherReader(cfg, resolver, properties)
		if err != nil {
			return nil, err
		}
		return reader, nil
	})
}

// RegisterWeatherReaderBuilder is an Fx invoke function that registers the created ComponentBuilder with the JobFactory.
func RegisterWeatherReaderBuilder(
	jf *support.JobFactory,
	builder jsl.ComponentBuilder,
) {
	// Register the builder with the key "weatherItemReader" matching the 'ref' in JSL (job.yaml).
	jf.RegisterComponentBuilder("weatherItemReader", builder)
	logger.Debugf("ComponentBuilder for WeatherReader registered with JobFactory. JSL ref: 'weatherItemReader'")
}

// Module defines Fx options for the weatherReader component.
var Module = fx.Options(
	fx.Provide(fx.Annotate(
		NewWeatherReaderComponentBuilder,
		fx.ResultTags(`name:"weatherItemReader"`),
	)),
	fx.Invoke(fx.Annotate(
		RegisterWeatherReaderBuilder,
		fx.ParamTags(``, `name:"weatherItemReader"`),
	)),
)
