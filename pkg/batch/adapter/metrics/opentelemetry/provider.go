package opentelemetry

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"google.golang.org/grpc/credentials" // Required for gRPC TLS credentials.

	metricsConfig "github.com/tigerroll/surfin/pkg/batch/adapter/metrics/config" // Import for metrics adapter configuration types.
	"github.com/tigerroll/surfin/pkg/batch/core/metrics"
	"github.com/tigerroll/surfin/pkg/batch/support/util/logger"
)

// MeterProviderHolder is a simple struct that holds the OpenTelemetry MeterProvider instance.
// This holder is used to manage the MeterProvider's lifecycle, ensuring it can be
// properly shut down when the application exits, allowing for graceful flushing of metrics.
type MeterProviderHolder struct {
	Provider *metric.MeterProvider
}

// NewMetricRecorderProvider initializes the OpenTelemetry MeterProvider and returns an
// OpenTelemetry-backed MetricRecorder implementation.
// It configures various OpenTelemetry exporters (e.g., OTLP) based on the provided metrics configuration.
//
// Parameters:
//
//	cfg: The metrics configuration, which specifies the types and settings for metric exporters.
//	     The type of this parameter is `metricsConfig.MetricsConfig`.
//
// Returns:
//
//	metrics.MetricRecorder: An instance of `OtelMetricRecorder` that records metrics using OpenTelemetry.
//	*MeterProviderHolder: A pointer to `MeterProviderHolder` containing the initialized `MeterProvider`.
//	error: An error if the MeterProvider or any exporter fails to initialize.
func NewMetricRecorderProvider(cfg metricsConfig.MetricsConfig) (metrics.MetricRecorder, *MeterProviderHolder, error) {
	var (
		exporters []metric.Exporter
		err       error
	)

	for name, adapterCfg := range cfg { // Iterate through each named metrics adapter configuration.
		// metricsConfig.MetricsConfig is a map[string]MetricsAdapterConfig, so direct iteration is possible.
		if !adapterCfg.Enabled {
			logger.Infof("OpenTelemetry metrics adapter is disabled: %s", name)
			continue
		}

		switch adapterCfg.Type {
		case "otlp":
			logger.Infof("Configuring OTLP exporter: %s (protocol: %s)", name, adapterCfg.OTLP.Protocols)
			// Create an OTLP exporter (gRPC or HTTP) based on the OTLP configuration.
			exporter, expErr := createOTLPExporter(adapterCfg.OTLP)
			if expErr != nil {
				return nil, nil, fmt.Errorf("failed to create OTLP exporter %s: %w", name, expErr)
			}
			exporters = append(exporters, exporter)
		case "prometheus_pushgateway":
			// Prometheus Pushgateway is out of scope for direct OpenTelemetry SDK export.
			// It's typically handled via an OpenTelemetry Collector, which can scrape metrics and push them.
			logger.Warnf("Prometheus Pushgateway configuration found for %s but direct export is not supported by OpenTelemetry SDK. Please use OpenTelemetry Collector.", name)
		default:
			logger.Warnf("Unsupported metrics adapter type for %s: %s", name, adapterCfg.Type)
		}
	}

	if len(exporters) == 0 {
		// If no enabled exporters are found, return a No-Op MetricRecorder to prevent errors.
		logger.Infof("No enabled OpenTelemetry metrics exporters found. Using NoopMetricRecorder.")
		return metrics.NewNoOpMetricRecorder(), &MeterProviderHolder{Provider: nil}, nil
	}

	// Instead of using metric.NewMultiExporter, create multiple PeriodicReaders, one for each exporter,
	// and pass them to the MeterProvider. This allows for different export intervals/timeouts per exporter if needed.
	var readerOptions []metric.Option
	for _, exp := range exporters {
		// Configure the PeriodicReader to push metrics at a regular interval.
		// TODO: Make the interval configurable via adapterCfg.
		readerOptions = append(readerOptions, metric.WithReader(metric.NewPeriodicReader(exp, metric.WithInterval(5*time.Second))))
	}

	// Configure the resource for the MeterProvider.
	// This resource provides common attributes (e.g., service name, version) that will be attached to all exported metrics.
	res, err := resource.New(context.Background(),
		resource.WithAttributes(
			semconv.ServiceName("surfin-batch"),
			semconv.ServiceVersion("1.0.0"), // TODO: Make this configurable or derive from build info
		),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create the MeterProvider.
	// Combine the resource option with the reader options and pass them to the MeterProvider.
	meterProvider := metric.NewMeterProvider(
		append([]metric.Option{metric.WithResource(res)}, readerOptions...)...,
	)

	// Create the OtelMetricRecorder.
	recorder, err := NewOtelMetricRecorder(meterProvider.Meter("surfin.batch.metrics"))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create OtelMetricRecorder: %w", err)
	}

	logger.Infof("OpenTelemetry metrics recorder initialized successfully.")
	return recorder, &MeterProviderHolder{Provider: meterProvider}, nil
}

// createOTLPExporter creates and configures an OpenTelemetry Protocol (OTLP) exporter.
// It supports both gRPC and HTTP protocols based on the provided configuration.
//
// Returns a `metric.Exporter` instance or an error if configuration is invalid.
func createOTLPExporter(cfg metricsConfig.OTLPExporterConfig) (metric.Exporter, error) {
	endpoint := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	timeout, err := time.ParseDuration(cfg.Timeout)
	if err != nil {
		return nil, fmt.Errorf("invalid timeout duration '%s': %w", cfg.Timeout, err)
	}

	headers := make(map[string]string)
	for k, v := range cfg.Headers {
		headers[k] = v
	}

	switch cfg.Protocols {
	// Configure gRPC exporter.
	case "grpc":
		opts := []otlpmetricgrpc.Option{
			otlpmetricgrpc.WithEndpoint(endpoint),
			otlpmetricgrpc.WithTimeout(timeout),
			otlpmetricgrpc.WithHeaders(headers),
		}
		if cfg.Insecure {
			opts = append(opts, otlpmetricgrpc.WithInsecure())
		} else {
			opts = append(opts, otlpmetricgrpc.WithTLSCredentials(credentials.NewClientTLSFromCert(nil, "")))
		}
		if cfg.Compression != "" && cfg.Compression != "none" {
			opts = append(opts, otlpmetricgrpc.WithCompressor(cfg.Compression))
		}
		return otlpmetricgrpc.New(context.Background(), opts...)
	// Configure HTTP exporter.
	case "http":
		opts := []otlpmetrichttp.Option{
			otlpmetrichttp.WithEndpoint(endpoint),
			otlpmetrichttp.WithTimeout(timeout),
			otlpmetrichttp.WithHeaders(headers),
		}
		if cfg.Insecure {
			opts = append(opts, otlpmetrichttp.WithInsecure())
		}
		switch cfg.Compression {
		case "gzip":
			opts = append(opts, otlpmetrichttp.WithCompression(otlpmetrichttp.GzipCompression)) // Enable gzip compression for HTTP.
		case "none", "":
			// No compression or default
		default:
			// Log a warning for unsupported compression, but don't fail.
			logger.Warnf("Unsupported HTTP compression type for OTLP exporter: %s. Skipping compression.", cfg.Compression)
		}
		return otlpmetrichttp.New(context.Background(), opts...)
	default:
		return nil, fmt.Errorf("unsupported OTLP protocol: %s", cfg.Protocols)
	}
}

// ShutdownMeterProvider gracefully shuts down the OpenTelemetry MeterProvider.
// This function should be called during application shutdown to ensure all
// buffered metrics are flushed and resources are released.
//
// Parameters:
//
//	ctx: The context for the shutdown operation, allowing for cancellation or timeouts.
//	holder: The `MeterProviderHolder` containing the `MeterProvider` to be shut down.
func ShutdownMeterProvider(ctx context.Context, holder *MeterProviderHolder) error {
	if holder != nil && holder.Provider != nil {
		logger.Infof("Shutting down OpenTelemetry MeterProvider...")
		if err := holder.Provider.Shutdown(ctx); err != nil {
			return fmt.Errorf("failed to shut down MeterProvider: %w", err)
		}
		logger.Infof("OpenTelemetry MeterProvider shut down successfully.")
	}
	return nil
}
