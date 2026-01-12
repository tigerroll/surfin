package processor

import (
	core "github.com/tigerroll/surfin/pkg/batch/core/application/port"
	config "github.com/tigerroll/surfin/pkg/batch/core/config"
	support "github.com/tigerroll/surfin/pkg/batch/core/config/support"
	jsl "github.com/tigerroll/surfin/pkg/batch/core/config/jsl"
	job "github.com/tigerroll/surfin/pkg/batch/core/domain/repository"
	"github.com/tigerroll/surfin/pkg/batch/support/util/logger"

	"go.uber.org/fx"
)

func NewWeatherProcessorComponentBuilder() jsl.ComponentBuilder {
	return jsl.ComponentBuilder(func(
		cfg *config.Config,
		repo job.JobRepository,
		resolver core.ExpressionResolver,
		dbResolver core.DBConnectionResolver,
		properties map[string]string,
	) (interface{}, error) {
		// Arguments unnecessary for this component are ignored.
		_ = repo
		_ = dbResolver
		processor, err := NewWeatherProcessor(cfg, resolver, properties)
		if err != nil {
			return nil, err
		}
		return processor, nil
	})
}

func RegisterWeatherProcessorBuilder(
	jf *support.JobFactory,
	builder jsl.ComponentBuilder,
) {
	jf.RegisterComponentBuilder("weatherItemProcessor", builder)
	logger.Debugf("WeatherProcessor ComponentBuilder registered with JobFactory. JSL ref: 'weatherItemProcessor'")
}

var Module = fx.Options(
	fx.Provide(fx.Annotate(
		NewWeatherProcessorComponentBuilder,
		fx.ResultTags(`name:"weatherItemProcessor"`),
	)),
	fx.Invoke(fx.Annotate(
		RegisterWeatherProcessorBuilder,
		fx.ParamTags(``, `name:"weatherItemProcessor"`),
	)),
)
