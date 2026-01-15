package config

// EmbeddedConfig holds the content of the configuration file, typically passed from main.go.
// This is used when loading configuration from an embedded source.
type EmbeddedConfig []byte

// LogLevel defines the logging level for the application.
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


// The ConnectionString() and GetDialector() methods have been removed.

// RetryConfig holds configuration for general retry mechanisms.
type RetryConfig struct {
	MaxAttempts                 int     `yaml:"max_attempts"`                   // Maximum number of retry attempts.
	InitialInterval             int     `yaml:"initial_interval"`               // Initial backoff interval in milliseconds.
	MaxInterval                 int     `yaml:"max_interval"`                   // Maximum backoff interval in milliseconds.
	Factor                      float64 `yaml:"factor"`                         // Factor by which the interval increases.
	CircuitBreakerThreshold     int     `yaml:"circuit_breaker_threshold"`      // Number of consecutive failures to open the circuit.
	CircuitBreakerResetInterval int     `yaml:"circuit_breaker_reset_interval"` // Time in milliseconds before attempting to close the circuit.
}

// ItemRetryConfig holds item-level retry configuration.
type ItemRetryConfig struct {
	MaxAttempts         int      `yaml:"max_attempts"`         // Maximum number of retry attempts for an item.
	InitialInterval     int      `yaml:"initial_interval"`     // Initial backoff interval in milliseconds for an item.
	RetryableExceptions []string `yaml:"retryable_exceptions"` // List of retryable exception names (string).
}

// ItemSkipConfig holds item-level skip configuration.
type ItemSkipConfig struct {
	SkipLimit           int      `yaml:"skip_limit"`           // Maximum number of items to skip.
	SkippableExceptions []string `yaml:"skippable_exceptions"` // List of skippable exception names (string).
}

// SecurityConfig holds security-related settings.
type SecurityConfig struct {
	MaskedParameterKeys []string `yaml:"masked_parameter_keys"` // Keys in JobParameters whose values should be masked in logs.
}

type BatchConfig struct {
	PollingIntervalSeconds int             `yaml:"polling_interval_seconds"`  // Interval for polling job status.
	APIEndpoint            string          `yaml:"api_endpoint"`              // General API endpoint (e.g., for external services).
	APIKey                 string          `yaml:"api_key"`                   // General API key.
	JobName                string          `yaml:"job_name"`                  // Default job name if not specified elsewhere.
	Retry                  RetryConfig     `yaml:"retry"`                     // General retry configuration.
	ChunkSize              int             `yaml:"chunk_size"`                // Default chunk size for chunk-oriented steps.
	ItemRetry              ItemRetryConfig `yaml:"item_retry"`                // Item-level retry configuration.
	ItemSkip               ItemSkipConfig  `yaml:"item_skip"`                 // Item-level skip configuration.
	StepExecutorRef        string          `yaml:"step_executor_ref"`         // Reference name for the Step Executor (e.g., "simpleStepExecutor", "remoteStepExecutor").
	MetricsAsyncBufferSize int             `yaml:"metrics_async_buffer_size"` // Buffer size for asynchronous metric recording.
}

// LoggingConfig holds logging configuration.
type LoggingConfig struct {
	Level string `yaml:"level"` // Logging level (e.g., "INFO", "DEBUG", "TRACE").
}

// SystemConfig holds system-wide settings.
type SystemConfig struct {
	Timezone string        `yaml:"timezone"` // Application timezone (e.g., "UTC", "Asia/Tokyo").
	Logging  LoggingConfig `yaml:"logging"`  // Logging configuration.
}

// InfrastructureConfig holds logical dependency settings for infrastructure components.
type InfrastructureConfig struct {
	JobRepositoryDBRef string `yaml:"job_repository_db_ref"` // Name of the DBConnection used by JobRepository (e.g., "metadata").
}

// SurfinConfig holds all configuration under the "surfin" top-level key.
// It now uses a generic AdaptorConfigs map to hold configurations for various adaptors.
type SurfinConfig struct {
	// AdaptorConfigs holds configurations for various adaptors (e.g., database, storage).
	// The structure is: adaptor_type -> adaptor_specific_config_map.
	// Example:
	// adaptor_configs:
	//   database:
	//     datasources:
	//       metadata:
	//         type: postgres
	//         host: localhost
	//         ...
	//   storage:
	//     datasources:
	//       default_gcs:
	//         type: gcs
	//         bucket_name: my-bucket
	//         ...
	AdaptorConfigs map[string]map[string]interface{} `yaml:"adaptor_configs"`
	Batch          BatchConfig                       `yaml:"batch"`
	System         SystemConfig                      `yaml:"system"`
	Infrastructure InfrastructureConfig              `yaml:"infrastructure"` // Infrastructure configuration.
	Security       SecurityConfig                    `yaml:"security"`       // Security configuration.
}

type Config struct {
	Surfin         SurfinConfig   `yaml:"surfin"` // Top-level configuration for the Surfin Batch Framework.
	EmbeddedConfig EmbeddedConfig `yaml:"-"`      // Not loaded from YAML.
}

// GlobalConfig is a pointer to the configuration instance shared across the application.
// It is expected to be set via fx.Supply or fx.Provide.
var GlobalConfig *Config

// GetMaskedParameterKeys retrieves the list of keys to be masked from the global configuration.
func GetMaskedParameterKeys() []string {
	if GlobalConfig == nil {
		return []string{}
	}
	return GlobalConfig.Surfin.Security.MaskedParameterKeys
}

// NewConfig returns a new instance of Config with default values.
func NewConfig() *Config {
	return &Config{
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
				ItemRetry: ItemRetryConfig{
					MaxAttempts:         3,
					InitialInterval:     1000,
					RetryableExceptions: []string{"*surfin/pkg/batch/support/util/exception.TemporaryNetworkError", "net.OpError", "context.DeadlineExceeded", "context.Canceled"},
				},
				ItemSkip: ItemSkipConfig{
					SkipLimit:           0,
					SkippableExceptions: []string{"*surfin/pkg/batch/support/util/exception.DataConversionError", "json.UnmarshalTypeError"},
				},
			},
			AdaptorConfigs: map[string]map[string]interface{}{ // Initialize AdaptorConfigs
				"database": { // Database adaptor configurations
					"datasources": map[string]interface{}{
						"metadata": map[string]interface{}{
							"type":     "postgres",
							"host":     "localhost",
							"port":     5432,
							"database": "batch_metadata",
							"user":     "batch_user",
							"password": "batch_password",
							"sslmode":  "disable",
							"pool": map[string]interface{}{
								"max_open_conns":            10,
								"max_idle_conns":            5,
								"conn_max_lifetime_minutes": 5,
							},
						},
						"workload": map[string]interface{}{
							"type":     "postgres",
							"host":     "localhost",
							"port":     5433,
							"database": "app_data",
							"user":     "app_user",
							"password": "app_password",
							"sslmode":  "disable",
							"pool": map[string]interface{}{
								"max_open_conns":            10,
								"max_idle_conns":            5,
								"conn_max_lifetime_minutes": 5,
							},
						},
					},
				},
				"storage": { // ストレージアダプターの設定
					"datasources": map[string]interface{}{
						"default_gcs": map[string]interface{}{
							"type":            "gcs",
							"bucket_name":     "your-default-gcs-bucket",
							"credentials_file": "", // Empty string means use default GCP authentication.
							"hive_partition": map[string]interface{}{
								"enable_hive_partitioning":     false,
								"hive_partition_prefix_format": "",
							},
						},
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
}
