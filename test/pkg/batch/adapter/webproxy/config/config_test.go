package config_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tigerroll/surfin/pkg/batch/adapter/webproxy/config"
	"gopkg.in/yaml.v3" // Import yaml package
)

func TestWebProxyConfig_APIKey(t *testing.T) {
	yamlStr := `
type: "APIKEY"
key: "test-api-key"
placement: "header"
key_name: "X-API-Key"
`
	var cfg config.WebProxyConfig
	// Test via map[string]interface{} instead of direct YAML unmarshaling.
	// This aligns with how Config is processed in module.go.
	var raw map[string]interface{}
	err := yaml.Unmarshal([]byte(yamlStr), &raw) // Parse YAML string into a map.
	assert.NoError(t, err)

	jsonBytes, err := json.Marshal(raw)
	assert.NoError(t, err)
	err = json.Unmarshal(jsonBytes, &cfg)
	assert.NoError(t, err)

	assert.Equal(t, "APIKEY", cfg.Type)
	assert.Equal(t, "test-api-key", cfg.Key)
	assert.Equal(t, "header", cfg.Placement)
	assert.Equal(t, "X-API-Key", cfg.KeyName)
}

func TestWebProxyConfig_OAuth2(t *testing.T) {
	yamlStr := `
type: "OAUTH2"
grant_type: "client_credentials"
client_id: "test-client-id"
client_secret: "test-client-secret"
token_url: "https://example.com/oauth/token"
`
	var cfg config.WebProxyConfig
	var raw map[string]interface{}
	err := yaml.Unmarshal([]byte(yamlStr), &raw)
	assert.NoError(t, err)

	jsonBytes, err := json.Marshal(raw)
	assert.NoError(t, err)
	err = json.Unmarshal(jsonBytes, &cfg)
	assert.NoError(t, err)

	assert.Equal(t, "OAUTH2", cfg.Type)
	assert.Equal(t, "client_credentials", cfg.GrantType)
	assert.Equal(t, "test-client-id", cfg.ClientId)
	assert.Equal(t, "test-client-secret", cfg.ClientSecret)
	assert.Equal(t, "https://example.com/oauth/token", cfg.TokenUrl)
}

func TestWebProxyConfig_HMAC(t *testing.T) {
	yamlStr := `
type: "HMAC"
algorithm: "RSASSA-PSS"
private_key: "---BEGIN PRIVATE KEY---..."
public_key_id: "key-123"
region: "us-east-1"
`
	var cfg config.WebProxyConfig
	var raw map[string]interface{}
	err := yaml.Unmarshal([]byte(yamlStr), &raw)
	assert.NoError(t, err)

	jsonBytes, err := json.Marshal(raw)
	assert.NoError(t, err)
	err = json.Unmarshal(jsonBytes, &cfg)
	assert.NoError(t, err)

	assert.Equal(t, "HMAC", cfg.Type)
	assert.Equal(t, "RSASSA-PSS", cfg.Algorithm)
	assert.Equal(t, "---BEGIN PRIVATE KEY---...", cfg.PrivateKey)
	assert.Equal(t, "key-123", cfg.PublicKeyId)
	assert.Equal(t, "us-east-1", cfg.Region)
}

