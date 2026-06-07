// Package config provides utilities for loading, merging, and managing application
// configuration from various sources, including embedded YAML files and environment variables.
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

// moduleName is used for error reporting within this package.
const moduleName = "config"

// ConfigParams defines the dependencies required by NewConfigProvider.
type ConfigParams struct {
	fx.In
	EmbeddedConfig EmbeddedConfig
	EnvFilePath    string `name:"envFilePath" optional:"true"`
}

// loadConfig loads and initializes the application configuration.
//
// The loading process follows these steps:
// 1. Loads the .env file (if specified).
// 2. Initializes the default configuration.
// 3. Expands environment variables within the embedded YAML configuration.
// 4. Unmarshals the expanded YAML into a temporary configuration struct.
// 5. Merges the YAML configuration into the default configuration.
// 6. Overrides values with environment variables.
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

	// 2. Expand environment variables in the embedded configuration.
	expander := NewOsEnvironmentExpander()
	expandedConfig, err := expander.Expand(embeddedConfig)
	if err != nil {
		return nil, exception.NewBatchError(moduleName, "failed to expand environment variables in embedded config", err, false, false)
	}

	// 3. Load configuration from embedded YAML into a temporary Config struct.
	var yamlConfig Config
	if err := yaml.Unmarshal(expandedConfig, &yamlConfig); err != nil {
		return nil, exception.NewBatchError(moduleName, "failed to unmarshal expanded embedded config", err, false, false)
	}

	// 4. Merge YAML configuration into the default configuration.
	mergeConfig(cfg, &yamlConfig)

	// 5. Override with environment variables
	if err := loadStructFromEnv(reflect.ValueOf(cfg).Elem(), ""); err != nil {
		return nil, exception.NewBatchError(moduleName, "failed to load config from environment variables", err, false, false)
	}
	return cfg, nil
}

// NewConfigProvider initializes and provides the application *Config.
// It loads defaults, merges embedded YAML, overrides with environment variables,
// sets the global logger level, and validates exception classes.
func NewConfigProvider(params ConfigParams) (*Config, error) {
	cfg, err := loadConfig(params.EnvFilePath, params.EmbeddedConfig)
	if err != nil {
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

// LoadConfig wraps loadConfig for external use during application startup.
func LoadConfig(envFilePath string, embeddedConfig EmbeddedConfig) (*Config, error) {
	return loadConfig(envFilePath, embeddedConfig)
}

// validateExceptionClasses verifies that all exception class names referenced in the configuration
// are present in the exception registry.
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

	return nil
}

// mergeConfig performs a deep merge of sourceConfig into destConfig.
// Values in sourceConfig overwrite corresponding values in destConfig if they are non-zero.
func mergeConfig(destConfig, sourceConfig *Config) {
	// Merge SurfinConfig
	mergeSurfinConfig(&destConfig.Surfin, &sourceConfig.Surfin)
}

// mergeSurfinConfig merges the source SurfinConfig into the destination.
func mergeSurfinConfig(dest, source *SurfinConfig) {
	// Merge BatchConfig
	if source.Batch.PollingIntervalSeconds != 0 {
		dest.Batch.PollingIntervalSeconds = source.Batch.PollingIntervalSeconds
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

	// Merge AdapterConfigs
	if source.AdapterConfigs != nil {
		if dest.AdapterConfigs == nil {
			dest.AdapterConfigs = make(map[string]any)
		}

		for key, value := range source.AdapterConfigs {
			dest.AdapterConfigs[key] = value
		}
	}
}

// mergeRetryConfig merges the source RetryConfig into the destination.
func mergeRetryConfig(dest, source *RetryConfig) {
	if source.MaxAttempts != 0 {
		dest.MaxAttempts = source.MaxAttempts
	}
	if source.InitialInterval != 0 {
		dest.InitialInterval = source.InitialInterval
	}
	if source.MaxInterval != 0 {
		dest.MaxInterval = source.MaxInterval
	}
	if source.Factor != 0 {
		dest.Factor = source.Factor
	}
	if source.CircuitBreakerThreshold != 0 {
		dest.CircuitBreakerThreshold = source.CircuitBreakerThreshold
	}
	if source.CircuitBreakerResetInterval != 0 {
		dest.CircuitBreakerResetInterval = source.CircuitBreakerResetInterval
	}
}

// mergeItemRetryConfig merges the source ItemRetryConfig into the destination.
func mergeItemRetryConfig(dest, source *ItemRetryConfig) {
	if source.MaxAttempts != 0 {
		dest.MaxAttempts = source.MaxAttempts
	}
	if source.InitialInterval != 0 {
		dest.InitialInterval = source.InitialInterval
	}
	if source.RetryableExceptions != nil {
		dest.RetryableExceptions = source.RetryableExceptions
	}
}

// mergeItemSkipConfig merges the source ItemSkipConfig into the destination.
func mergeItemSkipConfig(dest, source *ItemSkipConfig) {
	if source.SkipLimit != 0 {
		dest.SkipLimit = source.SkipLimit
	}
	if source.SkippableExceptions != nil {
		dest.SkippableExceptions = source.SkippableExceptions
	}
}

// mergeSystemConfig merges the source SystemConfig into the destination.
func mergeSystemConfig(dest, source *SystemConfig) {
	if source.Timezone != "" {
		dest.Timezone = source.Timezone
	}
	if source.Logging.Level != "" {
		dest.Logging.Level = source.Logging.Level
	}
}

// checkExceptionClasses validates that all exception class names in the provided list
// are registered in the exception registry.
func checkExceptionClasses(classNames []string, configType string) error {
	for _, name := range classNames {
		if !exception.IsErrorTypeRegistered(name) {
			return fmt.Errorf("%s configuration references unknown exception class: '%s'. Ensure it is registered.", configType, name)
		}
	}
	return nil
}

// loadStructFromEnv recursively populates a struct from environment variables.
// It uses the "yaml" tag to map struct fields to environment variable names.
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
		if !exists && field.Kind() != reflect.Map {
			continue
		}

		if field.Kind() == reflect.Map && field.Type().Key().Kind() == reflect.String && field.Type().Elem().Kind() == reflect.Struct {
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

// loadMapOfStructsFromEnv populates a map[string]struct{} field from environment variables.
// It infers map keys and struct field names from the environment variable naming convention
// (e.g., PREFIX_KEY_FIELD).
func loadMapOfStructsFromEnv(mapField reflect.Value, prefix string) error {
	if mapField.IsNil() {
		mapField.Set(reflect.MakeMap(mapField.Type()))
	}

	elemType := mapField.Type().Elem()

	for _, env := range os.Environ() {
		if !strings.HasPrefix(env, prefix) {
			continue
		}

		keyPartWithValue := strings.TrimPrefix(env, prefix)
		parts := strings.SplitN(keyPartWithValue, "=", 2)
		if len(parts) != 2 {
			continue
		}
		keyAndField := parts[0]
		envValue := parts[1]

		keyAndFieldParts := strings.Split(keyAndField, "_")
		if len(keyAndFieldParts) < 2 {
			continue
		}
		mapKey := strings.ToLower(keyAndFieldParts[0])
		structFieldName := strings.Join(keyAndFieldParts[1:], "_")

		structVal := mapField.MapIndex(reflect.ValueOf(mapKey))
		if !structVal.IsValid() {
			structVal = reflect.New(elemType).Elem()
		}

		if err := setStructFieldFromEnv(structVal, structFieldName, envValue); err != nil {
			return err
		}
		mapField.SetMapIndex(reflect.ValueOf(mapKey), structVal)
	}
	return nil
}

// setStructFieldFromEnv sets a specific struct field value from an environment variable.
// It matches the fieldName (case-insensitive) against the struct field's "yaml" tag.
func setStructFieldFromEnv(structVal reflect.Value, fieldName string, value string) error {
	typ := structVal.Type()
	for i := 0; i < typ.NumField(); i++ {
		field := structVal.Field(i)
		fieldType := typ.Field(i)
		yamlTag := fieldType.Tag.Get("yaml")
		if yamlTag == "" || yamlTag == "-" {
			continue
		}

		if strings.EqualFold(yamlTag, fieldName) {
			return setField(field, value)
		}
	}
	return nil
}

// setField converts and sets the value of a reflect.Value field based on its kind.
// Supported types: string, int, float64, and bool.
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
