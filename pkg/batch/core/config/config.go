package config

// Package config provides structures and utilities for managing application configuration.

// EmbeddedConfig holds the content of the configuration file, typically passed from main.go.
// This is used when loading configuration from an embedded source (e.g., a compiled binary).
type EmbeddedConfig []byte

// LogLevel defines the logging level for the application.
// It is used to control the verbosity of log output.
type LogLevel string

const (
	LogLevelTrace  LogLevel = "TRACE"
	LogLevelDebug  LogLevel = "DEBUG"
	LogLevelInfo   LogLevel = "INFO"
	LogLevelWarn   LogLevel = "WARN"
	LogLevelError  LogLevel = "ERROR"
	LogLevelFatal  LogLevel = "FATAL"
	LogLevelSilent LogLevel = "SILENT"
)

// RetryConfig holds configuration for general retry mechanisms.
type RetryConfig struct {
	MaxAttempts                 int     `yaml:"max_attempts"`                   // MaxAttempts is the maximum number of retry attempts.
	InitialInterval             int     `yaml:"initial_interval"`               // InitialInterval is the initial backoff interval in milliseconds.
	MaxInterval                 int     `yaml:"max_interval"`                   // MaxInterval is the maximum backoff interval in milliseconds.
	Factor                      float64 `yaml:"factor"`                         // Factor is the factor by which the interval increases (e.g., 2.0 for exponential backoff).
	CircuitBreakerThreshold     int     `yaml:"circuit_breaker_threshold"`      // CircuitBreakerThreshold is the number of consecutive failures to open the circuit.
	CircuitBreakerResetInterval int     `yaml:"circuit_breaker_reset_interval"` // CircuitBreakerResetInterval is the time in milliseconds before attempting to close the circuit.
}

// ItemRetryConfig holds item-level retry configuration.
type ItemRetryConfig struct {
	MaxAttempts         int      `yaml:"max_attempts"`         // MaxAttempts is the maximum number of retry attempts for an item.
	InitialInterval     int      `yaml:"initial_interval"`     // InitialInterval is the initial backoff interval in milliseconds for an item.
	RetryableExceptions []string `yaml:"retryable_exceptions"` // RetryableExceptions is a list of retryable exception names (string).
}

// ItemSkipConfig holds item-level skip configuration.
type ItemSkipConfig struct {
	SkipLimit           int      `yaml:"skip_limit"`           // SkipLimit is the maximum number of items to skip.
	SkippableExceptions []string `yaml:"skippable_exceptions"` // SkippableExceptions is a list of skippable exception names (string).
}

// SecurityConfig holds security-related settings.
type SecurityConfig struct {
	// MaskedParameterKeys is a list of keys in JobParameters whose values should be masked in logs.
	MaskedParameterKeys []string `yaml:"masked_parameter_keys"`
}

// BatchConfig holds configuration specific to the batch processing engine.
type BatchConfig struct {
	// PollingIntervalSeconds is the interval for polling job status.
	PollingIntervalSeconds int `yaml:"polling_interval_seconds"`
	// APIEndpoint is a general API endpoint (e.g., for external services).
	APIEndpoint string `yaml:"api_endpoint"`
	// APIKey is a general API key.
	APIKey string `yaml:"api_key"`
	// JobName is the default job name if not specified elsewhere.
	JobName string `yaml:"job_name"`
	// Retry is the general retry configuration.
	Retry RetryConfig `yaml:"retry"`
	// ChunkSize is the default chunk size for chunk-oriented steps.
	ChunkSize int `yaml:"chunk_size"`
	// ItemRetry is the item-level retry configuration.
	ItemRetry ItemRetryConfig `yaml:"item_retry"`
	// ItemSkip is the item-level skip configuration.
	ItemSkip ItemSkipConfig `yaml:"item_skip"`
	// StepExecutorRef is the reference name for the Step Executor (e.g., "simpleStepExecutor", "remoteStepExecutor").
	StepExecutorRef string `yaml:"step_executor_ref"`
	// MetricsAsyncBufferSize is the buffer size for asynchronous metric recording.
	MetricsAsyncBufferSize int `yaml:"metrics_async_buffer_size"`
}

// LoggingConfig holds logging configuration.
type LoggingConfig struct {
	// Level is the logging level (e.g., "INFO", "DEBUG", "TRACE").
	Level string `yaml:"level"`
}

// SystemConfig holds system-wide settings.
type SystemConfig struct {
	// Timezone is the application timezone (e.g., "UTC", "Asia/Tokyo").
	Timezone string `yaml:"timezone"`
	// Logging is the logging configuration.
	Logging LoggingConfig `yaml:"logging"`
}

// InfrastructureConfig holds logical dependency settings for infrastructure components.
type InfrastructureConfig struct {
	// JobRepositoryDBRef is the name of the DBConnection used by JobRepository (e.g., "metadata").
	JobRepositoryDBRef string `yaml:"job_repository_db_ref"`
}

// SurfinConfig holds all configuration under the "surfin" top-level key.
type SurfinConfig struct {
	// Batch contains batch processing specific configurations.
	Batch BatchConfig `yaml:"batch"`
	// System contains system-wide configurations.
	System SystemConfig `yaml:"system"`
	// Infrastructure contains infrastructure-related configurations.
	Infrastructure InfrastructureConfig `yaml:"infrastructure"`
	// Security contains security-related configurations.
	Security SecurityConfig `yaml:"security"`
	// AdaptorConfigs holds configurations for various adaptors, typically database connections.
	AdaptorConfigs map[string]interface{} `yaml:"database"`
}

// Config is the root structure for the entire application configuration.
type Config struct {
	// Surfin contains the top-level configuration for the Surfin Batch Framework.
	Surfin SurfinConfig `yaml:"surfin"`
	// EmbeddedConfig holds configuration loaded from an embedded source, not from YAML.
	EmbeddedConfig EmbeddedConfig `yaml:"-"`
}

// GlobalConfig is a pointer to the configuration instance shared across the application.
// It is expected to be set via fx.Supply or fx.Provide.
var GlobalConfig *Config

// GetMaskedParameterKeys retrieves the list of keys to be masked from the global configuration.
//
// Returns:
//
//	A slice of strings representing the keys whose values should be masked.
func GetMaskedParameterKeys() []string {
	if GlobalConfig == nil {
		return []string{}
	}
	return GlobalConfig.Surfin.Security.MaskedParameterKeys
}

// NewConfig returns a new instance of Config with default values.
//
// Returns:
//
//	A pointer to a new Config instance initialized with default settings.
func NewConfig() *Config {
	cfg := &Config{
		Surfin: SurfinConfig{
			System: SystemConfig{
				Timezone: "UTC", // Default value set to UTC
				Logging:  LoggingConfig{Level: "INFO"},
			},
			Batch: BatchConfig{
				JobName:                "",                   // Default Job name is empty. Expected to be set by the application or loaded from JSL.
				ChunkSize:              10,                   // Default chunk size.
				StepExecutorRef:        "simpleStepExecutor", // Default is local execution.
				MetricsAsyncBufferSize: 100,                  // Default buffer size for asynchronous metrics.
				ItemRetry: ItemRetryConfig{ // Default item retry configuration.
					MaxAttempts:     3,
					InitialInterval: 1000, // Default value (e.g., 1000ms).
					RetryableExceptions: []string{ // Default retryable exceptions.
						"*surfin/pkg/batch/support/util/exception.TemporaryNetworkError",
						"net.OpError",
						"context.DeadlineExceeded",
						"context.Canceled",
					},
				},
				ItemSkip: ItemSkipConfig{ // Default item skip configuration.
					SkipLimit: 0, // Default is no skipping.
					SkippableExceptions: []string{ // Default skippable exceptions.
						"*surfin/pkg/batch/support/util/exception.DataConversionError",
						"json.UnmarshalTypeError",
					},
				},
			},
			Infrastructure: InfrastructureConfig{ // Default values.
				JobRepositoryDBRef: "metadata",
			},
			Security: SecurityConfig{ // Default values.
				MaskedParameterKeys: []string{"password", "api_key", "secret"},
			},
		},
	}

	// Initialize AdaptorConfigs as an empty map, to be populated by YAML or by mergeConfig.
	cfg.Surfin.AdaptorConfigs = map[string]interface{}{}
	return cfg
}
