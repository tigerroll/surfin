package tasklet

import (
	"go.uber.org/fx"
)

// Module is the Fx module for the tasklet components.
var Module = fx.Options(
	// Register the ComponentBuilder for HourlyForecastExportTasklet
	fx.Provide(fx.Annotate(
		// Provide NewHourlyForecastExportTaskletBuilder directly, allowing Fx to resolve its dependencies.
		NewHourlyForecastExportTaskletBuilder,
		// fx.As(new(jsl.ComponentBuilder)), // This line is unnecessary and can be removed.
		fx.ResultTags(`name:"hourlyForecastExportTasklet"`), // Tag with the JSL reference name.
	)),
)
