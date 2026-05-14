package config

// OTLPExporterConfig defines the configuration for an OTLP exporter.
// This struct is used for both trace and metrics OTLP configurations.
type OTLPExporterConfig struct {
	Host               string            `yaml:"host"`                          // Host address of the OTLP endpoint.
	Port               int               `yaml:"port"`                          // Port number of the OTLP endpoint.
	Protocols          string            `yaml:"protocols"`                     // Protocol to use (e.g., "grpc", "http/protobuf").
	Headers            map[string]string `yaml:"headers,omitempty"`             // Additional headers for the export operation.
	Timeout            string            `yaml:"timeout"`                       // Timeout for the export operation (e.g., "5s").
	Compression        string            `yaml:"compression,omitempty"`         // Compression method (e.g., "gzip", "snappy").
	Insecure           bool              `yaml:"insecure,omitempty"`            // Whether to use an insecure connection (e.g., HTTP instead of HTTPS).
	CollectionInterval string            `yaml:"collection_interval,omitempty"` // Interval at which metrics are collected (e.g., "60s").
	ItemSamplingRate   float64           `yaml:"item_sampling_rate,omitempty"`  // Sampling rate for item-level metrics (0.0-1.0).
}

// CommonExporterConfig defines the common configuration for any observability exporter.
// It includes general settings like type and enabled status, and optional
// nested configurations for trace and metrics specific OTLP settings.
type CommonExporterConfig struct {
	Type    string              `yaml:"type"`              // Type of the exporter (e.g., "otlp", "prometheus").
	Enabled bool                `yaml:"enabled"`           // Whether this exporter is enabled.
	Trace   *OTLPExporterConfig `yaml:"trace,omitempty"`   // OTLP configuration specific to traces.
	Metrics *OTLPExporterConfig `yaml:"metrics,omitempty"` // OTLP configuration specific to metrics.
}

// ObservabilityConfig is a map of named observability exporter configurations.
// The key is the name of the exporter (e.g., "otlp_exporter").
type ObservabilityConfig map[string]CommonExporterConfig
