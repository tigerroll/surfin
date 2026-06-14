package job

import (
	support "github.com/tigerroll/surfin/pkg/batch/core/config/support"
	"go.uber.org/fx"
)

// RegisterHistoryJobBuilder registers the builder for the 'historyJob' with the JobFactory.
func RegisterHistoryJobBuilder(
	jf *support.JobFactory,
	builder support.JobBuilder,
) {
	jf.RegisterJobBuilder("historyJob", builder)
}

// RegisterForecastJobBuilder registers the builder for the 'forecastJob' with the JobFactory.
func RegisterForecastJobBuilder(
	jf *support.JobFactory,
	builder support.JobBuilder,
) {
	jf.RegisterJobBuilder("forecastJob", builder)
}

var Module = fx.Options(
	fx.Provide(
		fx.Annotate(
			NewHistoryJobBuilder,
			fx.ResultTags(`name:"historyJobBuilder"`),
		),
		fx.Annotate(
			NewForecastJobBuilder,
			fx.ResultTags(`name:"forecastJobBuilder"`),
		),
	),
	fx.Invoke(
		fx.Annotate(
			RegisterHistoryJobBuilder,
			fx.ParamTags(``, `name:"historyJobBuilder"`),
		),
		fx.Annotate(
			RegisterForecastJobBuilder,
			fx.ParamTags(``, `name:"forecastJobBuilder"`),
		),
	),
)
