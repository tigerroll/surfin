package factory

import (
	"go.uber.org/fx"
)

// Module provides StepFactory related components to Fx.
var Module = fx.Options(
	fx.Provide(NewDefaultStepFactory),
	fx.Provide(fx.Annotate(
		func(f *DefaultStepFactory) StepFactory {
			return f
		},
		fx.As(new(StepFactory)),
	)),
)
