package incrementer

import (
	"go.uber.org/fx"

	port "surfin/pkg/batch/core/application/port"
	config "surfin/pkg/batch/core/config"
	jsl "surfin/pkg/batch/core/config/jsl"
	support "surfin/pkg/batch/core/config/support"
	"surfin/pkg/batch/support/util/logger"
)

// NewRunIDIncrementerComponentBuilder provides a jsl.JobParametersIncrementerBuilder for RunIDIncrementer.
func NewRunIDIncrementerComponentBuilder() jsl.JobParametersIncrementerBuilder {
	return func(cfg *config.Config, properties map[string]string) (port.JobParametersIncrementer, error) {
		name, ok := properties["name"]
		if !ok || name == "" {
			name = "run.id" // Default value
			logger.Warnf("RunIDIncrementer: Property 'name' is not specified, using default value '%s'.", name)
		}
		return NewRunIDIncrementer(name), nil
	}
}

// RegisterRunIDIncrementerBuilder registers the RunIDIncrementer builder with the JobFactory.
func RegisterRunIDIncrementerBuilder(
	jf *support.JobFactory,
	builder jsl.JobParametersIncrementerBuilder,
) {
	jf.RegisterJobParametersIncrementerBuilder("runIdIncrementer", builder)
	logger.Debugf("Builder for RunIDIncrementer registered with JobFactory. JSL ref: 'runIdIncrementer'")
}

// NewTimestampIncrementerComponentBuilder provides a jsl.JobParametersIncrementerBuilder for TimestampIncrementer.
func NewTimestampIncrementerComponentBuilder() jsl.JobParametersIncrementerBuilder {
	return func(cfg *config.Config, properties map[string]string) (port.JobParametersIncrementer, error) {
		name, ok := properties["name"]
		if !ok || name == "" {
			name = "timestamp" // Default value
			logger.Warnf("TimestampIncrementer: Property 'name' is not specified, using default value '%s'.", name)
		}
		return NewTimestampIncrementer(name), nil
	}
}

// RegisterTimestampIncrementerBuilder registers the TimestampIncrementer builder with the JobFactory.
func RegisterTimestampIncrementerBuilder(
	jf *support.JobFactory,
	builder jsl.JobParametersIncrementerBuilder,
) {
	jf.RegisterJobParametersIncrementerBuilder("timestampIncrementer", builder)
	logger.Debugf("Builder for TimestampIncrementer registered with JobFactory. JSL ref: 'timestampIncrementer'")
}

// Module is the Fx module for the Incrementer package.
var Module = fx.Options(
	fx.Provide(
		fx.Annotate(
			NewRunIDIncrementerComponentBuilder,
			fx.ResultTags(`name:"runIdIncrementer"`),
		)),
	fx.Invoke(fx.Annotate(
		RegisterRunIDIncrementerBuilder,
		fx.ParamTags(``, `name:"runIdIncrementer"`),
	)),
	fx.Provide(fx.Annotate(
		NewTimestampIncrementerComponentBuilder,
		fx.ResultTags(`name:"timestampIncrementer"`),
	)),
	fx.Invoke(fx.Annotate(
		RegisterTimestampIncrementerBuilder,
		fx.ParamTags(``, `name:"timestampIncrementer"`),
	)),
)
