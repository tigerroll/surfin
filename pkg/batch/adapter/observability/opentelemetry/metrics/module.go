package metrics

import (
	"github.com/tigerroll/surfin/pkg/batch/adapter/observability/config"
	metricsconfig "github.com/tigerroll/surfin/pkg/batch/adapter/observability/opentelemetry/metrics/config"
	coreMetrics "github.com/tigerroll/surfin/pkg/batch/core/metrics"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.uber.org/fx"
)

// NewMetricsExportersConfigFromObservabilityConfig extracts and filters metrics exporter configurations
// from the overall ObservabilityConfig.
//
// It returns only those exporters that are enabled and have a 'Metrics' specific configuration defined.
//
// Parameters:
//
//	obsConfig: The overall observability configuration.
//
// Returns:
//
//	A map of metrics exporter configurations, or an empty map if no suitable exporters are found.
func NewMetricsExportersConfigFromObservabilityConfig(obsConfig config.ObservabilityConfig) (metricsconfig.MetricsExportersConfig, error) {
	metricsExporters := make(metricsconfig.MetricsExportersConfig)
	for name, commonCfg := range obsConfig {
		if commonCfg.Enabled && commonCfg.Metrics != nil {
			// Only include exporters that are enabled and have a metrics-specific configuration
			metricsExporters[name] = commonCfg
		}
	}
	return metricsExporters, nil
}

// Module is the Fx module for OpenTelemetry metrics.
// It provides the following to the Fx application context:
//   - metricsconfig.MetricsExportersConfig: Filtered metrics exporter configurations.
//   - *metric.MeterProvider: The OpenTelemetry MeterProvider instance.
//   - coreMetrics.MetricRecorder: An implementation of the MetricRecorder interface
//     that uses the initialized OpenTelemetry MeterProvider.
var Module = fx.Options(
	fx.Provide(NewMetricsExportersConfigFromObservabilityConfig),
	fx.Provide(NewMeterProvider),
	fx.Provide(func(meterProvider *metric.MeterProvider) (coreMetrics.MetricRecorder, error) {
		// Retrieves a Meter from the MeterProvider and passes it to NewOtelMetricRecorder.
		return NewOtelMetricRecorder(meterProvider.Meter("surfin.batch"))
	}),
)
