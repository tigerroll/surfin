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

// PoolConfig holds database connection pool settings.
type PoolConfig struct { // PoolConfig defines connection pool settings for databases.
	MaxOpenConns           int `yaml:"max_open_conns"`
	MaxIdleConns           int `yaml:"max_idle_conns"`
	ConnMaxLifetimeMinutes int `yaml:"conn_max_lifetime_minutes"`
}

// MigrationConfig holds database migration settings.
// NOTE: This struct is removed based on the refactoring plan (Phase 1.2).
// DatabaseConfig holds database connection settings.
type DatabaseConfig struct {
	Type      string     `yaml:"type"`                 // Database type (e.g., "postgres", "mysql", "sqlite").
	Host      string     `yaml:"host"`                 // Database host address.
	Port      int        `yaml:"port"`                 // Database port number.
	Database  string     `yaml:"database"`             // Database name.
	User      string     `yaml:"user"`                 // Database user.
	Password  string     `yaml:"password"`             // Database password.
	Schema    string     `yaml:"schema,omitempty"`     // Schema name for PostgreSQL/Redshift.
	Sslmode   string     `yaml:"sslmode"`              // SSL mode for the connection.
	ProjectID string     `yaml:"project_id,omitempty"` // Project ID for Google Cloud databases.
	DatasetID string     `yaml:"dataset_id,omitempty"` // Dataset ID for Google Cloud databases.
	TableID   string     `yaml:"table_id,omitempty"`   // Table ID for Google Cloud databases.
	Pool      PoolConfig `yaml:"pool"`                 // Connection pool settings.
	// Migration MigrationConfig `yaml:"migration"` // Removed based on refactoring plan
}

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
type SurfinConfig struct {
	Datasources    map[string]DatabaseConfig `yaml:"datasources"`
	Batch          BatchConfig               `yaml:"batch"`
	System         SystemConfig              `yaml:"system"`
	Infrastructure InfrastructureConfig      `yaml:"infrastructure"` // Infrastructure configuration.
	Security       SecurityConfig            `yaml:"security"`       // Security configuration.
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
			Datasources: map[string]DatabaseConfig{ // Default values for the Datasources map.
				"metadata": { // Default configuration for the framework metadata database.
					Type:     "postgres",
					Host:     "localhost",
					Port:     5432,
					Database: "batch_metadata",
					User:     "batch_user",
					Password: "batch_password",
					Sslmode:  "disable",
					Pool: PoolConfig{
						MaxOpenConns:           10,
						MaxIdleConns:           5,
						ConnMaxLifetimeMinutes: 5,
					},
				},
				"workload": { // Default configuration for the application data database.
					Type:     "postgres",
					Host:     "localhost",
					Port:     5433, // Another port or host
					Database: "app_data",
					User:     "app_user",
					Password: "app_password",
					Sslmode:  "disable",
					Pool: PoolConfig{
						MaxOpenConns:           10,
						MaxIdleConns:           5,
						ConnMaxLifetimeMinutes: 5,
					},
					// Migration config removed
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
