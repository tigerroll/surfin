package writer

import (
	"go.uber.org/fx"

	core "surfin/pkg/batch/core/application/port"
	config "surfin/pkg/batch/core/config"
	jsl "surfin/pkg/batch/core/config/jsl"
	support "surfin/pkg/batch/core/config/support"
	job "surfin/pkg/batch/core/domain/repository"
	"surfin/pkg/batch/support/util/logger"

	"surfin/pkg/batch/core/adaptor"
	tx "surfin/pkg/batch/core/tx"
)

// WeatherWriterComponentBuilderParams defines the dependencies for NewWeatherWriterComponentBuilder.
type WeatherWriterComponentBuilderParams struct {
	fx.In
	AllDBConnections map[string]adaptor.DBConnection
	AllTxManagers    map[string]tx.TransactionManager
}

// NewWeatherWriterComponentBuilder creates a jsl.ComponentBuilder for the weatherItemWriter.
// It now receives its core dependencies (AllDBConnections, AllTxManagers) via Fx.
func NewWeatherWriterComponentBuilder(p WeatherWriterComponentBuilderParams) jsl.ComponentBuilder {
	// Returns the builder function with a standard signature that JobFactory calls to construct the component.
	return jsl.ComponentBuilder(func(
		cfg *config.Config,
		repo job.JobRepository,
		resolver core.ExpressionResolver,
		dbResolver core.DBConnectionResolver,
		properties map[string]string,
	) (interface{}, error) {
		// Arguments unnecessary for this component are ignored.
		_ = repo
		
		// Fxから注入された allDBConnections と allTxManagers を NewWeatherWriter に渡す
		writer, err := NewWeatherWriter(cfg, p.AllDBConnections, p.AllTxManagers, resolver, dbResolver, properties)
		if err != nil {
			return nil, err
		}
		return writer, nil
	})
}

// RegisterWeatherWriterBuilder registers the created ComponentBuilder with the JobFactory.
func RegisterWeatherWriterBuilder(
	jf *support.JobFactory,
	builder jsl.ComponentBuilder,
) {
	jf.RegisterComponentBuilder("weatherItemWriter", builder)
	logger.Debugf("ComponentBuilder for WeatherWriter registered with JobFactory. JSL ref: 'weatherItemWriter'")
}

// Module defines Fx options for the writer component.
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
