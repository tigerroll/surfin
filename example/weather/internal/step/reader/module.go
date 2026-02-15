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

// NewHourlyForecastAPIReaderComponentBuilderParams defines the dependencies for NewHourlyForecastAPIReaderComponentBuilder.
type NewHourlyForecastAPIReaderComponentBuilderParams struct {
	fx.In
}

// NewHourlyForecastAPIReaderComponentBuilder creates a jsl.ComponentBuilder for the HourlyForecastAPIReader.
// This function is called by Fx as a provider.
//
// Returns:
//
//	A jsl.ComponentBuilder function that can construct a HourlyForecastAPIReader.
func NewHourlyForecastAPIReaderComponentBuilder() jsl.ComponentBuilder {
	// Returns the actual builder function that JobFactory calls to construct the component.
	return jsl.ComponentBuilder(func(
		cfg *config.Config,
		resolver core.ExpressionResolver,
		dbResolver coreAdapter.ResourceConnectionResolver,
		properties map[string]string,
	) (interface{}, error) {
		// Arguments unnecessary for this component are ignored.
		_ = dbResolver
		reader, err := NewHourlyForecastAPIReader(cfg, resolver, properties)
		if err != nil {
			return nil, err
		}
		return reader, nil
	})
}

// RegisterHourlyForecastAPIReaderBuilder is an Fx invoke function that registers the created ComponentBuilder with the JobFactory.
func RegisterHourlyForecastAPIReaderBuilder(
	jf *support.JobFactory,
	builder jsl.ComponentBuilder,
) {
	// Register the builder with the key "weatherItemReader" matching the 'ref' in JSL (job.yaml).
	jf.RegisterComponentBuilder("weatherItemReader", builder)
	logger.Debugf("ComponentBuilder for HourlyForecastAPIReader registered with JobFactory. JSL ref: 'weatherItemReader'")
}

// Module defines Fx options for the weatherReader component.
var Module = fx.Options(
	fx.Provide(fx.Annotate(
		NewHourlyForecastAPIReaderComponentBuilder,
		fx.ResultTags(`name:"weatherItemReader"`),
	)),
	fx.Invoke(fx.Annotate(
		RegisterHourlyForecastAPIReaderBuilder,
		fx.ParamTags(``, `name:"weatherItemReader"`),
	)),
)
