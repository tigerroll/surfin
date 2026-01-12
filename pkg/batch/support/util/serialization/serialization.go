// Package serialization provides utilities for serializing and deserializing various data structures used in the batch framework, such as JobParameters and ExecutionContext.
package serialization

import (
	"encoding/json"

	config "github.com/tigerroll/surfin/pkg/batch/core/config"
	"github.com/tigerroll/surfin/pkg/batch/support/util/exception"
	logger "github.com/tigerroll/surfin/pkg/batch/support/util/logger"
)

// GetMaskedJobParametersMap creates a copy of JobParameters and masks sensitive information based on configuration.
func GetMaskedJobParametersMap(params map[string]interface{}) map[string]interface{} {
	if params == nil || len(params) == 0 {
		return map[string]interface{}{}
	}

	// Create a masked copy
	maskedParams := make(map[string]interface{}, len(params))
	for k, v := range params {
		maskedParams[k] = v
	}

	maskedKeys := config.GetMaskedParameterKeys()
	for _, key := range maskedKeys {
		if _, ok := maskedParams[key]; ok {
			maskedParams[key] = "********" // Masking
		}
	}
	return maskedParams
}

// MarshalExecutionContext serializes an ExecutionContext map into a JSON byte slice.
func MarshalExecutionContext(ctx map[string]interface{}) ([]byte, error) {
	module := "serialization"

	if ctx == nil {
		logger.Debugf("ExecutionContext is nil. Returning empty JSON object.")
		return []byte("{}"), nil // Return empty JSON object if nil
	}
	data, err := json.Marshal(ctx)
	if err != nil {
		logger.Errorf("Failed to serialize ExecutionContext: %v", err)
		return nil, exception.NewBatchError(module, "Failed to serialize ExecutionContext", err, false, false)
	}
	return data, nil
}

// UnmarshalExecutionContext deserializes a JSON byte slice into an ExecutionContext map.
func UnmarshalExecutionContext(data []byte, ctx *map[string]interface{}) error {
	module := "serialization"

	if *ctx == nil {
		*ctx = make(map[string]interface{})
	} else {
		// Clear existing map
		for k := range *ctx {
			delete(*ctx, k)
		}
	}

	if len(data) == 0 || string(data) == "null" || string(data) == "{}" {
		logger.Debugf("ExecutionContext is nil or empty data. Created/cleared empty ExecutionContext.")
		return nil
	}

	err := json.Unmarshal(data, ctx)
	if err != nil {
		logger.Errorf("Failed to deserialize ExecutionContext: %v", err)
		return exception.NewBatchError(module, "Failed to deserialize ExecutionContext", err, false, false)
	}
	return nil
}

// MarshalJobParameters serializes a JobParameters map into a JSON byte slice, masking sensitive keys as configured.
func MarshalJobParameters(params map[string]interface{}) ([]byte, error) {
	module := "serialization"

	// Get a masked copy for persistence
	maskedParams := GetMaskedJobParametersMap(params)

	if len(maskedParams) == 0 {
		logger.Debugf("JobParameters.Params is nil. Returning empty JSON object.")
		return []byte("{}"), nil
	}

	data, err := json.Marshal(maskedParams)
	if err != nil {
		logger.Errorf("Failed to serialize JobParameters: %v", err)
		return nil, exception.NewBatchError(module, "Failed to serialize JobParameters", err, false, false)
	}
	return data, nil
}

// UnmarshalJobParameters deserializes a JSON byte slice into a JobParameters map.
func UnmarshalJobParameters(data []byte, params *map[string]interface{}) error {
	module := "serialization"

	if len(data) == 0 || string(data) == "null" {
		*params = make(map[string]interface{})
		logger.Debugf("JobParameters is nil or empty data. Created new empty JobParameters.")
		return nil
	}

	if *params == nil {
		*params = make(map[string]interface{})
		logger.Debugf("JobParameters map is nil. Created new map for deserialization.")
	} else {
		for k := range *params {
			delete(*params, k)
		}
		logger.Debugf("Cleared existing JobParameters map for deserialization.")
	}

	err := json.Unmarshal(data, params)
	if err != nil {
		logger.Errorf("Failed to deserialize JobParameters: %v", err)
		return exception.NewBatchError(module, "Failed to deserialize JobParameters", err, false, false)
	}
	return nil
}

// MarshalFailures serializes a slice of failure messages (strings) into a JSON byte slice.
func MarshalFailures(failures []string) ([]byte, error) {
	module := "serialization"

	if failures == nil {
		logger.Debugf("Failures is nil. Returning empty JSON array.")
		return []byte("[]"), nil
	}

	data, err := json.Marshal(failures)
	if err != nil {
		logger.Errorf("Failed to serialize Failures: %v", err)
		return nil, exception.NewBatchError(module, "Failed to serialize Failures", err, false, false)
	}
	return data, nil
}

// UnmarshalFailures deserializes a JSON byte slice into a slice of failure messages (strings).
func UnmarshalFailures(data []byte, msgs *[]string) error {
	module := "serialization"

	if len(data) == 0 || string(data) == "null" {
		*msgs = []string{}
		logger.Debugf("Failures is nil or empty data. Returning empty slice.")
		return nil
	}

	err := json.Unmarshal(data, msgs)
	if err != nil {
		logger.Errorf("Failed to deserialize Failures: %v", err)
		return exception.NewBatchError(module, "Failed to deserialize Failures", err, false, false)
	}

	return nil
}
