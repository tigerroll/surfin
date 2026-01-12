package configbinder

import (
	"fmt"
	"reflect"

	"github.com/mitchellh/mapstructure"
)

// BindProperties takes a map of string properties (from JSL) and binds them to a target struct.
// The target struct should use `mapstructure` tags (which are compatible with `yaml` tags).
func BindProperties(props map[string]string, target interface{}) error {
	if len(props) == 0 {
		return nil
	}

	// mapstructure requires map[string]interface{} for binding,
	// but JSL properties are map[string]string.
	// We need to convert map[string]string to map[string]interface{} first.
	intermediateMap := make(map[string]interface{}, len(props))
	for k, v := range props {
		intermediateMap[k] = v
	}

	// Configure mapstructure decoder
	config := &mapstructure.DecoderConfig{
		Metadata: nil,
		Result:   target,
		// WeaklyTypedInput allows converting strings to numbers, bools, etc.
		WeaklyTypedInput: true,
		// TagName specifies which struct tag to use (yaml is common and works well here)
		TagName: "yaml",
	}

	decoder, err := mapstructure.NewDecoder(config)
	if err != nil {
		return fmt.Errorf("failed to create mapstructure decoder: %w", err)
	}

	if err := decoder.Decode(intermediateMap); err != nil {
		// Provide more context about the target type
		targetType := reflect.TypeOf(target)
		if targetType.Kind() == reflect.Ptr {
			targetType = targetType.Elem()
		}
		return fmt.Errorf("failed to bind properties to struct %s: %w", targetType.Name(), err)
	}

	return nil
}
