package writer

import (
	"go.uber.org/fx"

	core "github.com/tigerroll/surfin/pkg/batch/core/application/port"
	config "github.com/tigerroll/surfin/pkg/batch/core/config"
	jsl "github.com/tigerroll/surfin/pkg/batch/core/config/jsl"
	support "github.com/tigerroll/surfin/pkg/batch/core/config/support"
	job "github.com/tigerroll/surfin/pkg/batch/core/domain/repository"
	"github.com/tigerroll/surfin/pkg/batch/support/util/logger"

	"github.com/tigerroll/surfin/pkg/batch/core/adaptor"
	tx "github.com/tigerroll/surfin/pkg/batch/core/tx"
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

		// Pass allDBConnections and allTxManagers injected from Fx to NewWeatherWriter.
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
