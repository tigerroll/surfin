package configbinder

import (
	"fmt"

	"github.com/mitchellh/mapstructure"
)

// BindProperties binds a map of properties to a target struct using mapstructure.
// It uses the "yaml" tag for binding and allows weakly typed input (e.g., string to int conversion).
//
// Parameters:
//
//	properties: The map of properties to bind.
//	target: The target struct to bind the properties to.
//
// Returns:
//
//	An error if binding fails.
func BindProperties(properties map[string]interface{}, target interface{}) error {
	decoderConfig := &mapstructure.DecoderConfig{
		Metadata:         nil,
		Result:           target,
		TagName:          "yaml", // Use "yaml" tag for binding.
		WeaklyTypedInput: true,   // Allow converting strings to numeric types.
	}

	decoder, err := mapstructure.NewDecoder(decoderConfig)
	if err != nil {
		return fmt.Errorf("failed to create mapstructure decoder: %w", err)
	}

	if err := decoder.Decode(properties); err != nil {
		return fmt.Errorf("failed to decode properties: %w", err)
	}

	return nil
}
