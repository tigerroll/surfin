package config

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"

	"github.com/tigerroll/surfin/pkg/batch/support/util/exception"
	"github.com/tigerroll/surfin/pkg/batch/support/util/logger"

	"go.uber.org/fx"
)

// Package config provides utilities for loading and managing application configuration
// from various sources, including YAML files and environment variables.

const moduleName = "config"

// ConfigParams defines the dependencies for NewConfigProvider.
type ConfigParams struct {
	fx.In
	EmbeddedConfig EmbeddedConfig // EmbeddedConfig contains the raw bytes of the configuration file.
	EnvFilePath    string `name:"envFilePath" optional:"true"` // EnvFilePath is the path to the .env file, if any.
}

// loadConfig loads configuration from a file and environment variables.
// This function is intended to be called only once during application startup.
//
// Parameters:
//   envFilePath: The path to the .env file.
//   embeddedConfig: The embedded configuration bytes.
// Returns:
//   A pointer to the loaded Config and an error if loading fails.
func loadConfig(envFilePath string, embeddedConfig EmbeddedConfig) (*Config, error) {
	if envFilePath != "" {
		if err := godotenv.Load(envFilePath); err != nil {
			logger.Warnf(".env file (%s) not found or could not be loaded: %v", envFilePath, err)
		}
	} else {
		if err := godotenv.Load(); err != nil {
			logger.Debugf(".env file not found or could not be loaded: %v", err)
		}
	}

	cfg := NewConfig()

	// 1. Load defaults from NewConfig()

	// 2. Load configuration from embedded YAML into a temporary Config struct.
	// This ensures that YAML values are correctly parsed into their respective types.
	var yamlConfig Config
	if err := yaml.Unmarshal(embeddedConfig, &yamlConfig); err != nil {
		return nil, exception.NewBatchError(moduleName, "failed to unmarshal embedded config", err, false, false)
	}

	// 3. Merge YAML configuration into the default configuration.
	mergeConfig(cfg, &yamlConfig)

	// 4. Override with environment variables
	if err := loadStructFromEnv(reflect.ValueOf(cfg).Elem(), ""); err != nil {
		return nil, exception.NewBatchError(moduleName, "failed to load config from environment variables", err, false, false)
	}
	return cfg, nil
}

// NewConfigProvider is an Fx provider that loads and provides *Config.
// It initializes the application configuration by loading defaults,
// merging from embedded YAML, and overriding with environment variables.
// It also sets the global logger level and validates exception classes.
//
// Parameters:
//   params: ConfigParams containing dependencies like embedded config and env file path.
// Returns:
//   A pointer to the initialized Config and an error if configuration loading or validation fails.
func NewConfigProvider(params ConfigParams) (*Config, error) {
	cfg, err := loadConfig(params.EnvFilePath, params.EmbeddedConfig)
	if err != nil {
		// If LoadConfig returns an error, return it as is.
		return nil, exception.NewBatchError(moduleName, "failed to load config from environment variables", err, false, false)
	}

	// Set global configuration
	GlobalConfig = cfg

	// Set log level
	logger.SetLogLevel(cfg.Surfin.System.Logging.Level)
	logger.Infof("Log level set to: %s", cfg.Surfin.System.Logging.Level)

	if err := validateExceptionClasses(cfg); err != nil {
		return nil, exception.NewBatchError(moduleName, "failed to validate configured exception classes", err, false, false)
	}

	return cfg, nil
}

// LoadConfig loads configuration from configuration files and environment variables.
// This function is expected to be called only once during application startup.
//
// Parameters:
//   envFilePath: The path to the .env file.
//   embeddedConfig: The embedded configuration bytes.
// Returns:
//   A pointer to the loaded Config and an error if loading fails.
func LoadConfig(envFilePath string, embeddedConfig EmbeddedConfig) (*Config, error) {
	return loadConfig(envFilePath, embeddedConfig)
}

// validateExceptionClasses validates that configured exception class names exist in the registry.
func validateExceptionClasses(cfg *Config) error {
	// 1. Validate ItemRetry settings (RetryableExceptions)
	if cfg.Surfin.Batch.ItemRetry.RetryableExceptions != nil {
		if err := checkExceptionClasses(cfg.Surfin.Batch.ItemRetry.RetryableExceptions, "ItemRetry"); err != nil {
			return err
		}
	}

	// 2. Validate ItemSkip settings (SkippableExceptions)
	if cfg.Surfin.Batch.ItemSkip.SkippableExceptions != nil {
		if err := checkExceptionClasses(cfg.Surfin.Batch.ItemSkip.SkippableExceptions, "ItemSkip"); err != nil {
			return err
		}
	}

	// Skipping validation for NoRollback setting as it does not exist in config.go.

	return nil
}

// mergeConfig performs a deep merge from sourceConfig into destConfig.
// Values in sourceConfig will overwrite corresponding values in destConfig
// if they are not zero/empty values for their type.
//
// Parameters:
//   destConfig: The destination Config to merge into.
//   sourceConfig: The source Config to merge from.
func mergeConfig(destConfig, sourceConfig *Config) {
	// Merge SurfinConfig
	mergeSurfinConfig(&destConfig.Surfin, &sourceConfig.Surfin)
	// Add other top-level merges if needed
}

// mergeSurfinConfig merges source into dest.
//
// Parameters:
//   dest: The destination SurfinConfig to merge into.
//   source: The source SurfinConfig to merge from.
func mergeSurfinConfig(dest, source *SurfinConfig) {
	// Merge BatchConfig
	if source.Batch.PollingIntervalSeconds != 0 {
		dest.Batch.PollingIntervalSeconds = source.Batch.PollingIntervalSeconds
	}
	if source.Batch.APIEndpoint != "" {
		dest.Batch.APIEndpoint = source.Batch.APIEndpoint
	}
	if source.Batch.APIKey != "" {
		dest.Batch.APIKey = source.Batch.APIKey
	}
	if source.Batch.JobName != "" {
		dest.Batch.JobName = source.Batch.JobName
	}
	if source.Batch.ChunkSize != 0 {
		dest.Batch.ChunkSize = source.Batch.ChunkSize
	}
	if source.Batch.StepExecutorRef != "" {
		dest.Batch.StepExecutorRef = source.Batch.StepExecutorRef
	}
	if source.Batch.MetricsAsyncBufferSize != 0 {
		dest.Batch.MetricsAsyncBufferSize = source.Batch.MetricsAsyncBufferSize
	}
	// Merge nested structs like Retry, ItemRetry, ItemSkip
	mergeRetryConfig(&dest.Batch.Retry, &source.Batch.Retry)
	mergeItemRetryConfig(&dest.Batch.ItemRetry, &source.Batch.ItemRetry)
	mergeItemSkipConfig(&dest.Batch.ItemSkip, &source.Batch.ItemSkip)

	// Merge SystemConfig
	mergeSystemConfig(&dest.System, &source.System)

	// Merge InfrastructureConfig
	if source.Infrastructure.JobRepositoryDBRef != "" {
		dest.Infrastructure.JobRepositoryDBRef = source.Infrastructure.JobRepositoryDBRef
	}

	// Merge SecurityConfig
	if source.Security.MaskedParameterKeys != nil {
		dest.Security.MaskedParameterKeys = source.Security.MaskedParameterKeys
	}

	// Merge AdaptorConfigs (this is the critical part for database configs)
	if source.AdaptorConfigs != nil {
		if dest.AdaptorConfigs == nil {
			dest.AdaptorConfigs = make(map[string]interface{})
		}
		// Since source.AdaptorConfigs is a map[string]interface{}, iterate directly and merge.
		for key, value := range source.AdaptorConfigs {
			dest.AdaptorConfigs[key] = value
		}
	}
}

// Helper merge functions for nested structs (example, extend as needed)
// mergeRetryConfig merges source into dest.
//
// Parameters:
//   dest: The destination RetryConfig to merge into.
//   source: The source RetryConfig to merge from.
func mergeRetryConfig(dest, source *RetryConfig) {
	// Only overwrite if source value is not zero/empty
	if source.MaxAttempts != 0 { dest.MaxAttempts = source.MaxAttempts }
	if source.InitialInterval != 0 { dest.InitialInterval = source.InitialInterval }
	if source.MaxInterval != 0 { dest.MaxInterval = source.MaxInterval }
	if source.Factor != 0 { dest.Factor = source.Factor }
	if source.CircuitBreakerThreshold != 0 { dest.CircuitBreakerThreshold = source.CircuitBreakerThreshold }
	if source.CircuitBreakerResetInterval != 0 { dest.CircuitBreakerResetInterval = source.CircuitBreakerResetInterval }
}

// mergeItemRetryConfig merges source into dest.
//
// Parameters:
//   dest: The destination ItemRetryConfig to merge into.
//   source: The source ItemRetryConfig to merge from.
func mergeItemRetryConfig(dest, source *ItemRetryConfig) {
	if source.MaxAttempts != 0 { dest.MaxAttempts = source.MaxAttempts }
	if source.InitialInterval != 0 { dest.InitialInterval = source.InitialInterval }
	if source.RetryableExceptions != nil { dest.RetryableExceptions = source.RetryableExceptions }
}

// mergeItemSkipConfig merges source into dest.
//
// Parameters:
//   dest: The destination ItemSkipConfig to merge into.
//   source: The source ItemSkipConfig to merge from.
func mergeItemSkipConfig(dest, source *ItemSkipConfig) {
	if source.SkipLimit != 0 { dest.SkipLimit = source.SkipLimit }
	if source.SkippableExceptions != nil { dest.SkippableExceptions = source.SkippableExceptions }
}

// mergeSystemConfig merges source into dest.
//
// Parameters:
//   dest: The destination SystemConfig to merge into.
//   source: The source SystemConfig to merge from.
func mergeSystemConfig(dest, source *SystemConfig) {
	if source.Timezone != "" { dest.Timezone = source.Timezone }
	if source.Logging.Level != "" { dest.Logging.Level = source.Logging.Level }
}

// checkExceptionClasses validates that all exception class names in the provided list
// are registered in the exception registry.
//
// Parameters:
//   classNames: A slice of strings representing exception class names.
//   configType: A string indicating the configuration type (e.g., "ItemRetry", "ItemSkip") for error messages.
func checkExceptionClasses(classNames []string, configType string) error {
	for _, name := range classNames {
		if !exception.IsErrorTypeRegistered(name) {
			return fmt.Errorf("%s configuration references unknown exception class: '%s'. Ensure it is registered.", configType, name)
		}
	}
	return nil
}

// loadStructFromEnv recursively loads configuration values into a struct from environment variables.
// It uses the "yaml" tag to determine the environment variable name.
//
// Parameters:
//   val: The reflect.Value of the struct to populate.
//   prefix: The prefix for environment variable names (e.g., "SURFIN_BATCH_").
// Returns: An error if any field cannot be set.
func loadStructFromEnv(val reflect.Value, prefix string) error {
	typ := val.Type()
	for i := 0; i < typ.NumField(); i++ {
		field := val.Field(i)
		fieldType := typ.Field(i)
		yamlTag := fieldType.Tag.Get("yaml")
		if yamlTag == "" || yamlTag == "-" {
			continue
		}
		envVarName := strings.ToUpper(prefix + yamlTag)

		if field.Kind() == reflect.Struct {
			if err := loadStructFromEnv(field, envVarName+"_"); err != nil {
				return err
			}
			continue
		}

		envValue, exists := os.LookupEnv(envVarName)
		if !exists && field.Kind() != reflect.Map { // If it's a map type, continue to process nested environment variables.
			continue
		}

		if field.Kind() == reflect.Map && field.Type().Key().Kind() == reflect.String && field.Type().Elem().Kind() == reflect.Struct {
			// For map[string]struct{}, process nested environment variables
			// Example: DATABASES_JOBDB_HOST, DATABASES_APP_HOST
			if err := loadMapOfStructsFromEnv(field, envVarName+"_"); err != nil {
				return err
			}
			continue
		}

		if err := setField(field, envValue); err != nil {
			return fmt.Errorf("failed to set field '%s' from env var '%s': %w", fieldType.Name, envVarName, err)
		}
	}
	return nil
}

// loadMapOfStructsFromEnv loads fields of type map[string]struct{} from environment variables.
// It infers map keys and struct field names from environment variable names.
//
// Example: For a field `Databases map[string]DatabaseConfig` in the config struct,
// an environment variable `DATABASES_JOBDB_HOST=localhost` would set the `Host` field
// of the `DatabaseConfig` instance associated with the key "jobdb".
//
// Parameters:
//   mapField: The reflect.Value of the map field (e.g., `cfg.Surfin.AdaptorConfigs`).
//   prefix: The environment variable prefix for this map (e.g., "SURFIN_DATABASE_").
func loadMapOfStructsFromEnv(mapField reflect.Value, prefix string) error {
	if mapField.IsNil() {
		mapField.Set(reflect.MakeMap(mapField.Type()))
	}

	elemType := mapField.Type().Elem() // Type of DatabaseConfig

	// Infer map keys from environment variables and load each element
	// Example: DATABASES_JOBDB_HOST -> mapKey="jobdb", structFieldName="HOST"
	for _, env := range os.Environ() {
		if !strings.HasPrefix(env, prefix) {
			continue
		}

		// Extract key and field name from environment variable name
		// Example: DATABASES_JOBDB_HOST=localhost -> keyAndField="JOBDB_HOST", envValue="localhost"
		keyPartWithValue := strings.TrimPrefix(env, prefix)
		parts := strings.SplitN(keyPartWithValue, "=", 2)
		if len(parts) != 2 {
			continue
		}
		keyAndField := parts[0] // e.g., "JOBDB_HOST"
		envValue := parts[1]

		// Separate map key and struct field name from keyAndField
		// Example: JOBDB_HOST -> mapKey="metadata", structFieldName="Host"
		keyAndFieldParts := strings.Split(keyAndField, "_")
		if len(keyAndFieldParts) < 2 {
			continue
		}
		mapKey := strings.ToLower(keyAndFieldParts[0])             // e.g., "jobdb"
		structFieldName := strings.Join(keyAndFieldParts[1:], "_") // e.g., "HOST"

		// Get or create an instance of the struct
		structVal := mapField.MapIndex(reflect.ValueOf(mapKey))
		if !structVal.IsValid() {
			structVal = reflect.New(elemType).Elem() // Create a new instance if not found
		}

		// Set the value of the struct field
		if err := setStructFieldFromEnv(structVal, structFieldName, envValue); err != nil {
			return err
		}
		mapField.SetMapIndex(reflect.ValueOf(mapKey), structVal)
	}
	return nil
}

// setStructFieldFromEnv sets the value of a specific struct field from an environment variable.
// It iterates through the struct's fields, matching the `fieldName` (case-insensitively)
// against the field's `yaml` tag.
//
// Parameters:
//   structVal: The reflect.Value of the struct instance.
//   fieldName: The name of the field to set (derived from the environment variable).
//   value: The string value to set.
// Returns: An error if the field cannot be set due to type conversion issues.
func setStructFieldFromEnv(structVal reflect.Value, fieldName string, value string) error {
	typ := structVal.Type()
	for i := 0; i < typ.NumField(); i++ {
		field := structVal.Field(i)
		fieldType := typ.Field(i)
		yamlTag := fieldType.Tag.Get("yaml")
		if yamlTag == "" || yamlTag == "-" {
			continue
		}

		// Check if YAML tag and environment variable name match
		if strings.EqualFold(yamlTag, fieldName) { // (case-insensitive comparison)
			return setField(field, value)
		}
	}
	return nil // Return nil if field not found (not an error)
}

// setField sets the value of a reflect.Value field based on its kind.
// It handles string, int, float, and bool types.
//
// Parameters:
//   field: The reflect.Value of the field to set.
//   value: The string value to convert and set.
func setField(field reflect.Value, value string) error {
	if !field.CanSet() {
		return nil
	}
	switch field.Kind() {
	case reflect.String:
		field.SetString(value)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		intValue, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return err
		}
		field.SetInt(intValue)
	case reflect.Float64, reflect.Float32:
		floatValue, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return err
		}
		field.SetFloat(floatValue)
	case reflect.Bool:
		boolValue, err := strconv.ParseBool(value)
		if err != nil {
			return err
		}
		field.SetBool(boolValue)
	}
	return nil
}
