package observability

import (
	"github.com/tigerroll/surfin/pkg/batch/adapter/observability/config"
	metricsOtel "github.com/tigerroll/surfin/pkg/batch/adapter/observability/opentelemetry/metrics"
	traceOtel "github.com/tigerroll/surfin/pkg/batch/adapter/observability/opentelemetry/trace"
	coreConfig "github.com/tigerroll/surfin/pkg/batch/core/config"
	"github.com/tigerroll/surfin/pkg/batch/support/util/exception"
	"go.uber.org/fx"
	"gopkg.in/yaml.v3"
)

// moduleName is the identifier for this package, used in error messages.
const moduleName = "observability"

// NewObservabilityConfigFromAppConfig extracts and decodes the observability configuration
// from the main application's core configuration.
//
// It expects the observability configuration to be located under `surfin.adapter.observability`
// within the `coreConfig.Config` structure. This function is designed to work with the generic
// `interface{}` type used for adapter configurations in `coreConfig.Config`, allowing specific
// adapter modules to extract and type-assert their relevant sections.
//
// Parameters:
//
//	cfg: The main application configuration.
//
// Returns:
//
//	The decoded ObservabilityConfig, or an empty config if the section is not found or
//	an error if decoding fails.
func NewObservabilityConfigFromAppConfig(cfg *coreConfig.Config) (config.ObservabilityConfig, error) {
	// The AdapterConfigs field is an interface{}, so we need to extract the specific
	// "observability" section and then unmarshal it into the ObservabilityConfig type.
	adapterConfigsMap, ok := cfg.Surfin.AdapterConfigs.(map[string]interface{})
	if !ok {
		// If AdapterConfigs is not a map[string]interface{}, it means there's no adapter config
		// or it's in an unexpected format. Return an empty config.
		return config.ObservabilityConfig{}, nil
	}

	observabilityConfigMap, ok := adapterConfigsMap[moduleName].(map[string]interface{})
	if !ok {
		// If there's no "observability" key under adapterConfigsMap, return an empty config.
		return config.ObservabilityConfig{}, nil
	}

	// Marshal the observability specific part back to YAML bytes to then unmarshal it
	// into the strongly typed ObservabilityConfig struct. This is a common pattern
	// when dealing with dynamic configuration structures in Go.
	observabilityBytes, err := yaml.Marshal(observabilityConfigMap)
	if err != nil {
		return nil, exception.NewBatchError(moduleName, "failed to marshal observability config", err, false, false)
	}

	var obsConfig config.ObservabilityConfig
	if err := yaml.Unmarshal(observabilityBytes, &obsConfig); err != nil {
		return nil, exception.NewBatchError(moduleName, "failed to unmarshal observability config", err, false, false)
	}

	return obsConfig, nil
}

// Module is the Fx module for observability components.
// It provides the ObservabilityConfig to the Fx application context
// and includes sub-modules for OpenTelemetry metrics and tracing.
var Module = fx.Options(
	fx.Provide(NewObservabilityConfigFromAppConfig),
	metricsOtel.Module, // Includes the OpenTelemetry metrics module.
	traceOtel.Module,   // Includes the OpenTelemetry trace module.
)
