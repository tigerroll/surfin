// Package config provides core configuration structures and utilities for the batch framework.
// This module defines Fx providers for configuration-related components.
package config

import "go.uber.org/fx"

// NewLoggingConfigProvider extracts and provides *LoggingConfig from *Config.
// This allows other Fx components to depend only on the logging configuration.
//
// Parameters:
//
//	cfg: The main application configuration.
//
// Returns:
//
//	A pointer to the LoggingConfig.
func NewLoggingConfigProvider(cfg *Config) *LoggingConfig {
	return &cfg.Surfin.System.Logging
}

// Module provides configuration-related components to Fx.
// It includes providers for logging configuration and the EnvironmentExpander.
var Module = fx.Options(
	fx.Provide(NewLoggingConfigProvider),
	// Provides an instance of EnvironmentExpander (specifically OsEnvironmentExpander)
	// as the EnvironmentExpander interface, making it available for dependency injection.
	fx.Provide(func() EnvironmentExpander {
		return NewOsEnvironmentExpander()
	}),
)
