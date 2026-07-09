package secret

import "fmt"

// Secret defines the interface for types that can hold sensitive data
// and be populated from a resolved secret value.
type Secret interface {
	SetValue(val any) error
}

// SecretString holds a sensitive string value.
// It implements the fmt.Stringer interface to mask the value when logged.
type SecretString string

// String returns a masked representation of the secret string.
func (s SecretString) String() string {
	return "********"
}

// SetValue sets the value from any type, handling string and []byte.
func (s *SecretString) SetValue(val any) error {
	switch v := val.(type) {
	case string:
		*s = SecretString(v)
	case []byte:
		*s = SecretString(string(v))
	default:
		return fmt.Errorf("unsupported type for SecretString: %T", val)
	}
	return nil
}

// SecretData holds sensitive binary data.
// It implements the fmt.Stringer interface to mask the data when logged.
type SecretData []byte

// String returns a masked representation of the secret data.
func (s SecretData) String() string {
	return "********"
}

// SetValue sets the value from any type, handling string and []byte.
func (s *SecretData) SetValue(val any) error {
	switch v := val.(type) {
	case string:
		*s = SecretData([]byte(v))
	case []byte:
		*s = SecretData(v)
	default:
		return fmt.Errorf("unsupported type for SecretData: %T", val)
	}
	return nil
}
