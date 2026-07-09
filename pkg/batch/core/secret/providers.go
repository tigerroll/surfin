package secret

import (
	"fmt"
	"os"
	"strings"
)

// FileSecretProvider handles the "file://" URI scheme by reading secrets from the filesystem.
type FileSecretProvider struct{}

// Supports returns true if the provider can handle the URI.
func (p *FileSecretProvider) Supports(uri string) bool {
	return strings.HasPrefix(uri, "file://")
}

// Resolve reads the secret from the file path specified in the URI.
// Always returns []byte for file://.
func (p *FileSecretProvider) Resolve(uri string) (any, error) {
	path := strings.TrimPrefix(uri, "file://")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read secret file: %w", err)
	}
	return data, nil
}

// EnvSecretProvider handles the "env://" URI scheme by reading secrets from environment variables.
type EnvSecretProvider struct{}

// Supports returns true if the URI starts with "env://".
func (p *EnvSecretProvider) Supports(uri string) bool {
	return strings.HasPrefix(uri, "env://")
}

// Resolve retrieves the secret from the environment variable specified in the URI.
// Always returns string for env://.
func (p *EnvSecretProvider) Resolve(uri string) (any, error) {
	key := strings.TrimPrefix(uri, "env://")
	val := os.Getenv(key)
	if val == "" {
		return nil, fmt.Errorf("environment variable not found: %s", key)
	}
	return val, nil
}
