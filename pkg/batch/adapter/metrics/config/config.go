package config

// Package config defines the configuration structures for various metrics adapters.
// These configurations are typically loaded from YAML files.

// OTLPExporterConfig holds configuration for OpenTelemetry Protocol (OTLP) exporters.
type OTLPExporterConfig struct {
	Host        string            `yaml:"host"`                  // Host address of the OTLP endpoint.
	Port        int               `yaml:"port"`                  // Port number of the OTLP endpoint.
	Protocols   string            `yaml:"protocols"`             // Protocol to use (e.g., "grpc", "http").
	Headers     map[string]string `yaml:"headers,omitempty"`     // Additional headers for the export request.
	Timeout     string            `yaml:"timeout"`               // Timeout for the export operation (e.g., "5s").
	Compression string            `yaml:"compression,omitempty"` // Compression method (e.g., "gzip", "snappy", "none").
	Insecure    bool              `yaml:"insecure,omitempty"`    // Whether to use an insecure connection (e.g., disable TLS).
}

// PrometheusPushgatewayConfig holds configuration for Prometheus Pushgateway.
// Note: This is outside the direct scope of OpenTelemetry SDK implementation.
type PrometheusPushgatewayConfig struct {
	Host              string `yaml:"host"`                          // Host address of the Prometheus Pushgateway.
	Port              int    `yaml:"port"`                          // Port number of the Prometheus Pushgateway.
	JobName           string `yaml:"job_name"`                      // Job name to associate with the pushed metrics.
	InstanceName      string `yaml:"instance_name,omitempty"`       // Optional instance name to associate with the pushed metrics.
	BasicAuthUsername string `yaml:"basic_auth_username,omitempty"` // Optional username for basic authentication.
	BasicAuthPassword string `yaml:"basic_auth_password,omitempty"` // Optional password for basic authentication.
	Timeout           string `yaml:"timeout"`                       // Timeout for the push operation (e.g., "5s").
}

// MetricsAdapterConfig holds the configuration for a single OpenTelemetry metrics adapter.
type MetricsAdapterConfig struct {
	Type       string                      `yaml:"type"`                 // Type of the exporter (e.g., "otlp", "prometheus_pushgateway").
	Enabled    bool                        `yaml:"enabled"`              // Whether this adapter is enabled.
	OTLP       OTLPExporterConfig          `yaml:"otlp,omitempty"`       // OTLP exporter specific configuration.
	Prometheus PrometheusPushgatewayConfig `yaml:"prometheus,omitempty"` // Prometheus Pushgateway specific configuration.
}

// MetricsConfig holds the overall metrics configuration, mapping names to adapter configs.
type MetricsConfig map[string]MetricsAdapterConfig
