package metricsconfig


import (
	"github.com/tigerroll/surfin/pkg/batch/adapter/observability/config"
)

// MetricsExportersConfig defines the configuration for all metrics exporters.
// The key is the name of the exporter (e.g., "otlp_exporter").
// Each exporter configuration is a CommonExporterConfig, which can contain
// specific OTLP settings for metrics.
type MetricsExportersConfig map[string]config.CommonExporterConfig
