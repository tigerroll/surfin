// Package metrics provides functions to create and manage OpenTelemetry MeterProvider instances.
package metrics

import (
	"context"
	"fmt"
	"time"

	"github.com/tigerroll/surfin/pkg/batch/adapter/observability/config"
	metricsconfig "github.com/tigerroll/surfin/pkg/batch/adapter/observability/opentelemetry/metrics/config"
	"github.com/tigerroll/surfin/pkg/batch/support/util/exception"
	"github.com/tigerroll/surfin/pkg/batch/support/util/logger"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.uber.org/fx"
)

// moduleName is used for error reporting within this package.
const moduleName = "opentelemetry-metrics"

// NewMeterProvider creates and provides an OpenTelemetry MeterProvider.
//
// It initializes the MeterProvider based on the provided metrics exporter configurations.
// If no enabled metrics exporters are found, a no-op MeterProvider is returned.
// The function also registers an Fx lifecycle hook to ensure proper shutdown of the provider.
//
// Parameters:
//
//	lc: The Fx lifecycle object for registering shutdown hooks.
//	metricsExporters: A map of metrics exporter configurations.
//
// Returns:
//
//	An initialized *metric.MeterProvider and an error if initialization fails.
func NewMeterProvider(lc fx.Lifecycle, metricsExporters metricsconfig.MetricsExportersConfig) (*metric.MeterProvider, error) {
	var exporters []metric.Exporter

	res, err := resource.New(context.Background(),
		resource.WithFromEnv(),      // Load attributes from OTEL_RESOURCE_ATTRIBUTES and OTEL_SERVICE_NAME
		resource.WithTelemetrySDK(), // Auto-detect Telemetry SDK info
		resource.WithProcess(),      // Auto-detect process info
		resource.WithOS(),           // Auto-detect OS info
		resource.WithContainer(),    // Auto-detect container info
		resource.WithHost(),         // Auto-detect host info
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	var meterProviderOptions []metric.Option
	meterProviderOptions = append(meterProviderOptions, metric.WithResource(res))

	for name, exporterCfg := range metricsExporters {
		if !exporterCfg.Enabled {
			logger.Infof("OpenTelemetry metrics adapter is disabled: %s", name)
			continue
		}

		if exporterCfg.Type == "otlp" && exporterCfg.Metrics != nil {
			logger.Infof("Configuring OTLP metrics exporter: %s (protocol: %s)", name, exporterCfg.Metrics.Protocols)
			exporter, expErr := createOTLPExporter(exporterCfg.Metrics)
			if expErr != nil {
				return nil, exception.NewBatchError(moduleName, fmt.Sprintf("failed to create OTLP metrics exporter '%s'", name), expErr, false, false)
			}
			exporters = append(exporters, exporter)

			// Apply collection interval if specified
			if exporterCfg.Metrics.CollectionInterval != "" {
				interval, parseErr := time.ParseDuration(exporterCfg.Metrics.CollectionInterval)
				if parseErr != nil {
					logger.Warnf("Invalid collection_interval for exporter '%s': %v. Using default.", name, parseErr)
					meterProviderOptions = append(meterProviderOptions, metric.WithReader(metric.NewPeriodicReader(exporter)))
				} else {
					meterProviderOptions = append(meterProviderOptions, metric.WithReader(metric.NewPeriodicReader(exporter, metric.WithInterval(interval))))
				}
			} else {
				meterProviderOptions = append(meterProviderOptions, metric.WithReader(metric.NewPeriodicReader(exporter)))
			}

		} else {
			logger.Warnf("Unsupported metrics exporter type '%s' or missing metrics configuration for exporter '%s'", exporterCfg.Type, name)
		}
	}

	if len(exporters) == 0 {
		logger.Infof("No enabled OpenTelemetry metrics exporters found. Using NoopMeterProvider.")
		return metric.NewMeterProvider(), nil // No-op provider
	}

	mp := metric.NewMeterProvider(meterProviderOptions...)

	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			logger.Infof("Shutting down OpenTelemetry MeterProvider...")
			if err := mp.Shutdown(ctx); err != nil {
				return fmt.Errorf("failed to shut down MeterProvider: %w", err)
			}
			logger.Infof("OpenTelemetry MeterProvider shut down successfully.")
			return nil
		},
	})

	logger.Infof("OpenTelemetry metrics MeterProvider initialized successfully.")
	return mp, nil
}

// createOTLPExporter creates an OTLP metrics exporter based on the provided configuration.
// It supports gRPC and HTTP/protobuf protocols.
//
// Parameters:
//
//	cfg: The OTLP exporter configuration.
//
// Returns:
//
//	A metric.Exporter instance and an error if the exporter cannot be created
//	(e.g., due to unsupported protocol or invalid timeout).
func createOTLPExporter(cfg *config.OTLPExporterConfig) (metric.Exporter, error) {
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
		opts := []otlpmetricgrpc.Option{
			otlpmetricgrpc.WithEndpoint(endpoint),
			otlpmetricgrpc.WithTimeout(timeout),
			otlpmetricgrpc.WithHeaders(headers),
		}
		if cfg.Insecure {
			opts = append(opts, otlpmetricgrpc.WithInsecure())
		} else {
			// For secure gRPC, you would typically provide TLS credentials.
			// If no specific credentials are provided and Insecure is false,
			// the client will attempt a secure connection, which might fail
			// if the server's certificate is not trusted.
			// For demonstration, we'll use system default credentials if available,
			// or rely on the default secure behavior of the OTLP exporter.
			// If you need custom TLS, use otlpmetricgrpc.WithGRPCConnOptions(grpc.WithTransportCredentials(...))
			// For now, we assume default secure behavior is sufficient or WithInsecure handles it.
		}
		if cfg.Compression != "" {
			// OTLP gRPC exporter supports gzip compression.
			// The WithCompressor option takes a string (e.g., "gzip").
			opts = append(opts, otlpmetricgrpc.WithCompressor(cfg.Compression))
		}
		return otlpmetricgrpc.New(context.Background(), opts...)
	case "http/protobuf":
		opts := []otlpmetrichttp.Option{
			otlpmetrichttp.WithEndpoint(endpoint),
			otlpmetrichttp.WithTimeout(timeout),
			otlpmetrichttp.WithHeaders(headers),
		}
		if cfg.Insecure {
			opts = append(opts, otlpmetrichttp.WithInsecure())
		}
		if cfg.Compression == "gzip" {
			opts = append(opts, otlpmetrichttp.WithCompression(otlpmetrichttp.GzipCompression))
		} else if cfg.Compression != "" {
			return nil, fmt.Errorf("unsupported compression type for HTTP/protobuf metrics exporter: %s", cfg.Compression)
		}
		return otlpmetrichttp.New(context.Background(), opts...)
	default:
		return nil, fmt.Errorf("unsupported OTLP protocol for metrics: %s", cfg.Protocols)
	}
}
