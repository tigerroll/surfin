package bootstrap

import (
	"context"
	"github.com/tigerroll/surfin/pkg/batch/core/config"
	jsl "github.com/tigerroll/surfin/pkg/batch/core/config/jsl"
	"github.com/tigerroll/surfin/pkg/batch/support/util/logger"

	"go.uber.org/fx"
)

// BatchInitializer is responsible for initializing batch components, such as loading JSL definitions.
type BatchInitializer struct {
	jslBytes jsl.JSLDefinitionBytes
}

// NewBatchInitializer creates a new instance of BatchInitializer.
func NewBatchInitializer(jslBytes jsl.JSLDefinitionBytes) *BatchInitializer {
	return &BatchInitializer{
		jslBytes: jslBytes,
	}
}

// GetJSLDefinitionBytes returns the loaded JSL definition byte slice.
func (i *BatchInitializer) GetJSLDefinitionBytes() jsl.JSLDefinitionBytes {
	return i.jslBytes
}

// ApplyLoggingConfigHook applies the logging level based on the configuration. (Corresponds to A_LOG_1)
func ApplyLoggingConfigHook(cfg *config.Config) {
	if cfg.Surfin.System.Logging.Level != "" {
		logger.SetLogLevel(cfg.Surfin.System.Logging.Level)
		logger.Infof("Log level set to: %s", cfg.Surfin.System.Logging.Level)
	}
}

// LoadJSLDefinitionsHook registers an Fx lifecycle hook to load JSL definitions.
// Defined as a named function for use with fx.Invoke.
func LoadJSLDefinitionsHook(lc fx.Lifecycle, initializer *BatchInitializer) {
	lc.Append(fx.Hook{
		OnStart: onStartLoadJSLDefinitions(initializer),
	})
}

// onStartLoadJSLDefinitions is a helper function for the OnStart hook that begins loading JSL definitions.
func onStartLoadJSLDefinitions(initializer *BatchInitializer) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		logger.Infof("Loading JSL definitions.")
		return jsl.LoadJSLDefinitionFromBytes(initializer.GetJSLDefinitionBytes())
	}
}
