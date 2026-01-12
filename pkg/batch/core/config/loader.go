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

const moduleName = "config"

// ConfigParams defines the dependencies for NewConfigProvider.
type ConfigParams struct {
	fx.In
	EmbeddedConfig EmbeddedConfig
	EnvFilePath    string `name:"envFilePath" optional:"true"`
}

// loadConfig loads configuration from a file and environment variables.
// This function is intended to be called only once during application startup.
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

	// 1. Load defaults from embedded configuration
	if err := yaml.Unmarshal(embeddedConfig, cfg); err != nil {
		return nil, exception.NewBatchError(moduleName, "failed to unmarshal embedded config", err, false, false)
	}

	// 2. Override with environment variables
	if err := loadStructFromEnv(reflect.ValueOf(cfg).Elem(), ""); err != nil {
		return nil, exception.NewBatchError(moduleName, "failed to load config from environment variables", err, false, false)
	}
	return cfg, nil
}

// NewConfigProvider is an Fx provider that loads and provides *Config.
func NewConfigProvider(params ConfigParams) (*Config, error) {
	// FIX: Changed to call LoadConfig function
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

	// T_ERR_SAFE_1: Validate exception class names
	if err := validateExceptionClasses(cfg); err != nil {
		return nil, exception.NewBatchError(moduleName, "failed to validate configured exception classes", err, false, false)
	}

	return cfg, nil
}

// LoadConfig loads configuration from configuration files and environment variables.
// This function is expected to be called only once during application startup.
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

func checkExceptionClasses(classNames []string, configType string) error {
	for _, name := range classNames {
		if !exception.IsErrorTypeRegistered(name) {
			return fmt.Errorf("%s configuration references unknown exception class: '%s'. Ensure it is registered.", configType, name)
		}
	}
	return nil
}

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
// Example: Databases map[string]DatabaseConfig
func loadMapOfStructsFromEnv(mapField reflect.Value, prefix string) error {
	if mapField.IsNil() {
		mapField.Set(reflect.MakeMap(mapField.Type()))
	}

	elemType := mapField.Type().Elem() // Type of DatabaseConfig

	// Infer map keys from environment variables and load each element
	// Example: DATABASES_JOBDB_HOST -> key="JOBDB", field="HOST"
	for _, env := range os.Environ() {
		if !strings.HasPrefix(env, prefix) {
			continue
		}
		
		// Extract key and field name from environment variable name
		// Example: DATABASES_JOBDB_HOST=localhost -> keyPart="JOBDB_HOST", value="localhost"
		keyPartWithValue := strings.TrimPrefix(env, prefix)
		parts := strings.SplitN(keyPartWithValue, "=", 2)
		if len(parts) != 2 {
			continue
		}
		keyAndField := parts[0]
		envValue := parts[1]

		// Separate map key and struct field name from keyAndField
		// Example: JOBDB_HOST -> mapKey="metadata", structFieldName="Host"
		keyAndFieldParts := strings.Split(keyAndField, "_")
		if len(keyAndFieldParts) < 2 {
			continue
		}
		mapKey := strings.ToLower(keyAndFieldParts[0]) // e.g., "jobdb"
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
