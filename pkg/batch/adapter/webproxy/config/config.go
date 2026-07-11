package config

// WebProxyConfig defines the configuration for a web proxy connection.
type WebProxyConfig struct {
	Type         string `yaml:"type" json:"type"`                                       // Type of authentication (e.g., "HMAC", "OAUTH2", "APIKEY", "NONE", "TLS").
	Algorithm    string `yaml:"algorithm,omitempty" json:"algorithm,omitempty"`         // HMAC signing algorithm (e.g., "RSASSA-PSS").
	PrivateKey   string `yaml:"private_key,omitempty" json:"private_key,omitempty"`     // HMAC private key (can be sourced from environment variables).
	PublicKeyId  string `yaml:"public_key_id,omitempty" json:"public_key_id,omitempty"` // HMAC public key ID.
	Region       string `yaml:"region,omitempty" json:"region,omitempty"`               // HMAC region.
	GrantType    string `yaml:"grant_type,omitempty" json:"grant_type,omitempty"`       // OAuth2 grant type (e.g., "client_credentials").
	TokenUrl     string `yaml:"token_url,omitempty" json:"token_url,omitempty"`         // OAuth2 token URL.
	ClientId     string `yaml:"client_id,omitempty" json:"client_id,omitempty"`         // OAuth2 client ID.
	ClientSecret string `yaml:"client_secret,omitempty" json:"client_secret,omitempty"` // OAuth2 client secret.
	Key          string `yaml:"key,omitempty" json:"key,omitempty"`                     // API Key value.
	Prefix       string `yaml:"prefix,omitempty" json:"prefix,omitempty"`               // API Key prefix (e.g., "Bearer").
	APIKey       string `yaml:"api_key,omitempty" json:"api_key,omitempty"`             // API Key value (alternative).
	Placement    string `yaml:"placement,omitempty" json:"placement,omitempty"`         // API Key placement ("header", "query", or "auth_header").
	KeyName      string `yaml:"key_name,omitempty" json:"key_name,omitempty"`           // API Key header name or query parameter name.
	APIEndpoint  string `yaml:"api_endpoint,omitempty" json:"api_endpoint,omitempty"`   // Endpoint URL of the API to proxy.
	TLSKey       string `yaml:"tls_key,omitempty" json:"tls_key,omitempty"`             // TLS Key.
	TLSCrt       string `yaml:"tls_crt,omitempty" json:"tls_crt,omitempty"`             // TLS Certificate.
	TLSPem       string `yaml:"tls_pem,omitempty" json:"tls_pem,omitempty"`             // TLS PEM.
}
