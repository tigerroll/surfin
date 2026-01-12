package job

import (
	support "surfin/pkg/batch/core/config/support"
	"go.uber.org/fx"
)

// RegisterWeatherJobBuilder registers the builder for the 'weatherJob' with the JobFactory.
func RegisterWeatherJobBuilder(
	jf *support.JobFactory,
	builder support.JobBuilder,
) {
	// The key "weatherJob" must match the job ID in the JSL file (job.yaml).
	// This allows the JobFactory to find this builder when CreateJob("weatherJob") is called.
	jf.RegisterJobBuilder("weatherJob", builder)
}

var Module = fx.Options(
	fx.Provide(fx.Annotate(
		NewWeatherJobBuilder,
		fx.ResultTags(`name:"weatherJobBuilder"`),
	)),
	fx.Invoke(fx.Annotate(
		RegisterWeatherJobBuilder,
		fx.ParamTags(``, `name:"weatherJobBuilder"`),
	)),
)
