package config

import "go.uber.org/fx"

// NewLoggingConfigProvider extracts and provides *LoggingConfig from *Config.
func NewLoggingConfigProvider(cfg *Config) *LoggingConfig {
	return &cfg.Surfin.System.Logging
}

// Module provides configuration-related components to Fx.
var Module = fx.Options(
	fx.Provide(NewLoggingConfigProvider),
)
