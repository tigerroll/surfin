package opentelemetry

import (
	"context"
	"fmt"

	"github.com/mitchellh/mapstructure"
	"go.uber.org/fx"

	metricsConfig "github.com/tigerroll/surfin/pkg/batch/adapter/metrics/config"
	coreConfig "github.com/tigerroll/surfin/pkg/batch/core/config"
)

// Package opentelemetry provides an OpenTelemetry-based metrics adapter for the batch system.
// It integrates with the Fx dependency injection framework to configure and provide
// metric recording capabilities, allowing the application to export metrics to
// OpenTelemetry-compatible systems.

// NewMetricsConfigFromAppConfig extracts and decodes the metrics configuration from the application's global configuration.
// This function specifically looks for the "metrics" section within the "adapter_configs"
// of the `SurfinConfig` structure.
//
// Parameters:
//
//	cfg: The main application configuration (`*coreConfig.Config`).
//
// Returns:
//
//	metricsConfig.MetricsConfig: A map containing the parsed metrics adapter configurations.
//	error: An error if the configuration cannot be properly decoded or is malformed.
func NewMetricsConfigFromAppConfig(cfg *coreConfig.Config) (metricsConfig.MetricsConfig, error) {
	metricsCfg := make(metricsConfig.MetricsConfig)

	// Type assert AdapterConfigs to map[string]interface{} as it's defined as interface{} in coreConfig.
	adapterConfigs, ok := cfg.Surfin.AdapterConfigs.(map[string]interface{})
	if !ok {
		// If AdapterConfigs is not a map or is nil, it means no adapter configurations are present.
		// Return an empty metrics configuration without error.
		return metricsCfg, nil
	}

	// Look for the "metrics" section within the adapter configurations.
	metricsSection, found := adapterConfigs["metrics"]
	if !found {
		// If the "metrics" section is not found, return an empty metrics configuration.
		return metricsCfg, nil
	}

	// Ensure the metrics section is a map of string to interface{}, which is expected for adapter configurations.
	metricsMap, ok := metricsSection.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("metrics: 'metrics' section in adapter config is not a map[string]interface{}")
	}

	for name, rawAdapterCfg := range metricsMap {
		var adapterConfig metricsConfig.MetricsAdapterConfig
		decoderConfig := &mapstructure.DecoderConfig{
			Result:           &adapterConfig, // Target struct for decoding.
			WeaklyTypedInput: true,           // Allow for flexible type conversions (e.g., int to string).
			TagName:          "yaml",         // Use YAML tags for struct field mapping.
		}

		// Create a new decoder for each adapter configuration.
		decoder, err := mapstructure.NewDecoder(decoderConfig)
		if err != nil {
			return nil, fmt.Errorf("metrics: failed to create decoder for metrics config '%s': %w", name, err)
		}
		if err := decoder.Decode(rawAdapterCfg); err != nil {
			return nil, fmt.Errorf("metrics: failed to decode metrics config for '%s': %w", name, err)
		}
		metricsCfg[name] = adapterConfig
	}

	return metricsCfg, nil
}

// Module is the Fx module for the OpenTelemetry metrics adapter.
// It provides the necessary components for metrics collection and export,
// including the metrics configuration, the MetricRecorder implementation,
// and a lifecycle hook for graceful shutdown of the MeterProvider.
var Module = fx.Options(
	// Provide the metrics configuration extracted from the application config.
	fx.Provide(NewMetricsConfigFromAppConfig),
	// Provide the OpenTelemetry-based MetricRecorder and its MeterProviderHolder.
	fx.Provide(NewMetricRecorderProvider),

	// Register an Fx lifecycle hook to gracefully shut down the OpenTelemetry MeterProvider
	// when the application stops. This ensures all buffered metrics are exported.
	fx.Invoke(func(lifecycle fx.Lifecycle, holder *MeterProviderHolder) {
		lifecycle.Append(fx.Hook{
			OnStop: func(ctx context.Context) error {
				return ShutdownMeterProvider(ctx, holder)
			},
		})
	}),
)
