// Package trace provides functions to create and manage OpenTelemetry TracerProvider instances.
package trace

import (
	"context"
	"fmt"
	"time"

	"github.com/tigerroll/surfin/pkg/batch/adapter/observability/config"
	traceconfig "github.com/tigerroll/surfin/pkg/batch/adapter/observability/opentelemetry/trace/config"
	"github.com/tigerroll/surfin/pkg/batch/support/util/exception"
	"github.com/tigerroll/surfin/pkg/batch/support/util/logger"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.uber.org/fx"
)

// moduleName is the identifier for this package, used in error messages.
const moduleName = "opentelemetry-trace"

// NewTracerProvider creates and provides an OpenTelemetry TracerProvider.
//
// It initializes the TracerProvider based on the provided trace exporter configurations.
// If no enabled trace exporters are found, a no-op TracerProvider is returned.
// The function also registers an Fx lifecycle hook to ensure proper shutdown of the provider.
//
// Parameters:
//
//	lc: The Fx lifecycle object for registering shutdown hooks.
//	traceExporters: A map of trace exporter configurations.
//
// Returns:
//
//	An initialized *trace.TracerProvider and an error if initialization fails.
func NewTracerProvider(lc fx.Lifecycle, traceExporters traceconfig.TraceExportersConfig) (*trace.TracerProvider, error) {
	var (
		exporters []trace.SpanExporter
		err       error
	)

	for name, exporterCfg := range traceExporters {
		if !exporterCfg.Enabled {
			logger.Infof("OpenTelemetry trace adapter is disabled: %s", name)
			continue
		}

		if exporterCfg.Type == "otlp" && exporterCfg.Trace != nil {
			logger.Infof("Configuring OTLP trace exporter: %s (protocol: %s)", name, exporterCfg.Trace.Protocols)
			exporter, expErr := createOTLPExporter(exporterCfg.Trace)
			if expErr != nil {
				return nil, exception.NewBatchError(moduleName, fmt.Sprintf("failed to create OTLP trace exporter '%s'", name), expErr, false, false)
			}
			exporters = append(exporters, exporter)
		} else {
			logger.Warnf("Unsupported trace exporter type '%s' or missing trace configuration for exporter '%s'", exporterCfg.Type, name)
		}
	}

	if len(exporters) == 0 {
		logger.Infof("No enabled OpenTelemetry trace exporters found. Using NoopTracerProvider.")
		return trace.NewTracerProvider(), nil // No-op provider
	}

	var spanProcessors []trace.SpanProcessor
	for _, exp := range exporters {
		spanProcessors = append(spanProcessors, trace.NewBatchSpanProcessor(exp))
	}

	res, err := resource.New(context.Background(),
		resource.WithAttributes(
			semconv.ServiceName("surfin-batch"),
			semconv.ServiceVersion("1.0.0"), // TODO: Make this configurable or derive from build info
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	tp := trace.NewTracerProvider(
		trace.WithResource(res),
		trace.WithSampler(trace.AlwaysSample()), // Always sample for now, can be configured later
	)

	for _, sp := range spanProcessors {
		tp.RegisterSpanProcessor(sp)
	}

	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			logger.Infof("Shutting down OpenTelemetry TracerProvider...")
			if err := tp.Shutdown(ctx); err != nil {
				return fmt.Errorf("failed to shut down TracerProvider: %w", err)
			}
			logger.Infof("OpenTelemetry TracerProvider shut down successfully.")
			return nil
		},
	})

	logger.Infof("OpenTelemetry trace TracerProvider initialized successfully.")
	return tp, nil
}

// createOTLPExporter creates an OpenTelemetry SpanExporter for OTLP based on the provided configuration.
// It supports gRPC and HTTP/protobuf protocols.
//
// Parameters:
//
//	cfg: The OTLP exporter configuration.
//
// Returns:
//
//	A trace.SpanExporter instance and an error if the exporter cannot be created
//	(e.g., due to unsupported protocol or invalid timeout).
func createOTLPExporter(cfg *config.OTLPExporterConfig) (trace.SpanExporter, error) {
	endpoint := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	timeout, err := time.ParseDuration(cfg.Timeout)
	if err != nil {
		return nil, fmt.Errorf("invalid timeout duration: %w", err)
	}

	headers := make(map[string]string)
	for k, v := range cfg.Headers {
		headers[k] = v
	}

	switch cfg.Protocols {
	case "grpc":
		opts := []otlptracegrpc.Option{
			otlptracegrpc.WithEndpoint(endpoint),
			otlptracegrpc.WithTimeout(timeout),
			otlptracegrpc.WithHeaders(headers),
		}
		if cfg.Insecure {
			opts = append(opts, otlptracegrpc.WithInsecure())
		}
		if cfg.Compression != "" {
			// OTLP gRPC exporter supports gzip compression.
			// The WithCompressor option takes a string (e.g., "gzip").
			opts = append(opts, otlptracegrpc.WithCompressor(cfg.Compression))
		}
		return otlptracegrpc.New(context.Background(), opts...)
	case "http/protobuf":
		opts := []otlptracehttp.Option{
			otlptracehttp.WithEndpoint(endpoint),
			otlptracehttp.WithTimeout(timeout),
			otlptracehttp.WithHeaders(headers),
		}
		if cfg.Insecure {
			opts = append(opts, otlptracehttp.WithInsecure())
		}
		// OTLP HTTP exporter supports gzip compression via otlptracehttp.WithCompression.
		// Ensure the OpenTelemetry SDK is up-to-date to use this.
		if cfg.Compression == "gzip" {
			opts = append(opts, otlptracehttp.WithCompression(otlptracehttp.GzipCompression))
		} else if cfg.Compression != "" {
			return nil, fmt.Errorf("unsupported compression type for HTTP/protobuf trace exporter: %s", cfg.Compression)
		}
		return otlptracehttp.New(context.Background(), opts...)
	default:
		return nil, fmt.Errorf("unsupported OTLP protocol: %s", cfg.Protocols)
	}
}
