// Package trace provides the Fx module and components for OpenTelemetry tracing integration.
// It configures and provides the OpenTelemetry TracerProvider and the application's Tracer
// based on the observability configuration.
package trace

import (
	sdk_trace "go.opentelemetry.io/otel/sdk/trace" // OpenTelemetry SDK for tracing.

	"github.com/tigerroll/surfin/pkg/batch/adapter/observability/config"
	traceconfig "github.com/tigerroll/surfin/pkg/batch/adapter/observability/opentelemetry/trace/config"
	"github.com/tigerroll/surfin/pkg/batch/core/metrics"
	"go.uber.org/fx"
)

// NewTraceExportersConfigFromObservabilityConfig extracts and filters trace exporter configurations
// from the overall ObservabilityConfig.
//
// It returns only those exporters that are enabled and have a 'Trace' specific configuration defined.
//
// Parameters:
//
//	obsConfig: The overall observability configuration.
//
// Returns:
//
//	traceconfig.TraceExportersConfig: A map of trace exporter configurations, or an empty map if no suitable exporters are found.
func NewTraceExportersConfigFromObservabilityConfig(obsConfig config.ObservabilityConfig) (traceconfig.TraceExportersConfig, error) {
	traceExporters := make(traceconfig.TraceExportersConfig)
	for name, commonCfg := range obsConfig {
		if commonCfg.Enabled && commonCfg.Trace != nil {
			// Only include exporters that are enabled and have a trace-specific configuration
			traceExporters[name] = commonCfg
		}
	}
	return traceExporters, nil
}

// Module is the Fx module for OpenTelemetry tracing.
// It provides the following to the Fx application context:
//   - traceconfig.TraceExportersConfig: Filtered trace exporter configurations extracted from the main observability config.
//   - *sdk_trace.TracerProvider: The OpenTelemetry TracerProvider instance, responsible for creating Tracers and managing spans.
//   - metrics.Tracer: The application's Tracer implementation (OtelTracer), which wraps the OpenTelemetry Tracer.
var Module = fx.Options(
	fx.Provide(NewTraceExportersConfigFromObservabilityConfig),
	fx.Provide(NewTracerProvider),
	fx.Provide(func(tracerProvider *sdk_trace.TracerProvider) (metrics.Tracer, error) {
		// Provides the application's Tracer implementation (OtelTracer) by wrapping
		// the OpenTelemetry Tracer obtained from the TracerProvider.
		return NewOtelTracer(tracerProvider.Tracer("surfin.batch"))
	}),
)
