package logger

import "go.uber.org/fx"

// Module is an Fx module that provides logging configuration and an fx.Logger adapter.
var Module = fx.Options(
	fx.WithLogger(NewFxLoggerAdapter),
)
