package usecase

import (
	"go.uber.org/fx"
)

// Module is the Fx module for JobLauncher, JobOperator, and JobExplorer.
var Module = fx.Options(
	// Provide JobExplorer
	fx.Provide(fx.Annotate(
		NewSimpleJobExplorer,
		fx.As(new(JobExplorer)),
	)),
	// Provide JobOperator
	fx.Provide(fx.Annotate(
		NewDefaultJobOperator,
		fx.As(new(JobOperator)),
	)),
	
	// Provide JobLauncher (uses constructor defined in simple_joblauncher.go)
	fx.Provide(NewSimpleJobLauncher),
	fx.Provide(fx.Annotate(
		func(launcher *SimpleJobLauncher) JobLauncher { return launcher },
		fx.As(new(JobLauncher)),
	)),
	
	// Invoke hook to set JobLauncher in DefaultJobOperator
	fx.Invoke(func(operator JobOperator, launcher *SimpleJobLauncher) {
		// Downcast to concrete type DefaultJobOperator and call SetJobLauncher
		if defaultOperator, ok := operator.(*DefaultJobOperator); ok {
			defaultOperator.SetJobLauncher(launcher)
		}
	}),
)
