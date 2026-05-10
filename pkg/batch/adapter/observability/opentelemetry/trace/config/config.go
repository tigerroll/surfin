package traceconfig


import (
	"github.com/tigerroll/surfin/pkg/batch/adapter/observability/config"
)

// TraceExportersConfig defines the configuration for all trace exporters.
// The key is the name of the exporter (e.g., "otlp_exporter").
// Each exporter configuration is a CommonExporterConfig, which can contain
// specific OTLP settings for tracing.
type TraceExportersConfig map[string]config.CommonExporterConfig
