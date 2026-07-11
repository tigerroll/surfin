package webproxy

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/tigerroll/surfin/pkg/batch/adapter/webproxy/config"
	coreAdapter "github.com/tigerroll/surfin/pkg/batch/core/adapter"
	"github.com/tigerroll/surfin/pkg/batch/core/secret"
)

// ContextKeySignaturePayload is the type used for context keys related to signature payloads.
type ContextKeySignaturePayload string

const (
	// SignaturePayloadContextKey is the key for storing the string to be signed in the context.
	SignaturePayloadContextKey ContextKeySignaturePayload = "signaturePayload"
	// SignatureHeaderContextKey is the key for storing the signed header in the context.
	SignatureHeaderContextKey ContextKeySignaturePayload = "signatureHeader"
)

// WebProxyConnection represents a connection to a Web Proxy, managing authentication and TLS configuration.
type WebProxyConnection struct {
	name   string
	cfg    config.WebProxyConfig
	client *http.Client

	// HMAC-related fields
	privateKey *rsa.PrivateKey

	// TLS-related fields
	tlsCert []byte
	tlsKey  []byte
	tlsPem  []byte
}

// NewWebProxyConnection initializes a new WebProxyConnection with the provided configuration and secret resolver.
// It resolves secrets for authentication and TLS, and parses private keys if required.
func NewWebProxyConnection(name string, cfg config.WebProxyConfig, resolver secret.SecretResolver) (*WebProxyConnection, error) {
	// Helper to resolve string-based secrets
	resolve := func(val string) (string, error) {
		if strings.HasPrefix(val, "env://") || strings.HasPrefix(val, "file://") {
			resolved, err := resolver.Resolve(val)
			if err != nil {
				return "", err
			}
			if str, ok := resolved.(string); ok {
				return str, nil
			}
			if b, ok := resolved.([]byte); ok {
				return string(b), nil
			}
			return "", fmt.Errorf("unsupported secret type")
		}
		return val, nil
	}

	// Helper to resolve byte-based secrets
	resolveToBytes := func(val string) ([]byte, error) {
		if val == "" {
			return nil, nil
		}
		if strings.HasPrefix(val, "env://") || strings.HasPrefix(val, "file://") {
			resolved, err := resolver.Resolve(val)
			if err != nil {
				return nil, err
			}
			if b, ok := resolved.([]byte); ok {
				return b, nil
			}
			if s, ok := resolved.(string); ok {
				return []byte(s), nil
			}
			return nil, fmt.Errorf("unsupported secret type")
		}
		return []byte(val), nil
	}

	// Resolve configuration fields
	var err error
	cfg.PrivateKey, err = resolve(cfg.PrivateKey)
	if err != nil {
		return nil, err
	}
	cfg.ClientSecret, err = resolve(cfg.ClientSecret)
	if err != nil {
		return nil, err
	}
	cfg.Key, err = resolve(cfg.Key)
	if err != nil {
		return nil, err
	}

	// Resolve TLS fields
	tlsKey, err := resolveToBytes(cfg.TLSKey)
	if err != nil {
		return nil, err
	}
	tlsCrt, err := resolveToBytes(cfg.TLSCrt)
	if err != nil {
		return nil, err
	}
	tlsPem, err := resolveToBytes(cfg.TLSPem)
	if err != nil {
		return nil, err
	}

	var privateKey *rsa.PrivateKey
	if cfg.Type == "HMAC" && cfg.PrivateKey != "" {
		block, _ := pem.Decode([]byte(cfg.PrivateKey))
		if block == nil {
			return nil, fmt.Errorf("webproxy: failed to decode PEM block for private key for HMAC connection '%s'", name)
		} else {
			parsedKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
			if err != nil {
				parsedKey, err = x509.ParsePKCS1PrivateKey(block.Bytes)
			}
			if err == nil {
				if rsaKey, ok := parsedKey.(*rsa.PrivateKey); ok {
					privateKey = rsaKey
				} else {
					return nil, fmt.Errorf("webproxy: private key for HMAC connection '%s' is not an RSA private key", name)
				}
			} else {
				return nil, fmt.Errorf("webproxy: failed to parse private key for HMAC connection '%s': %w", name, err)
			}
		}
	}

	if cfg.Type == "APIKEY" {
		if cfg.Key == "" {
			return nil, fmt.Errorf("webproxy: APIKEY type for connection '%s' requires 'key' to be configured", name)
		}
		if (cfg.Placement == "query" || cfg.Placement == "header") && cfg.KeyName == "" {
			return nil, fmt.Errorf("webproxy: APIKEY authentication with placement '%s' for connection '%s' requires 'key_name'", cfg.Placement, name)
		}
	}

	transport := &WebProxyRoundTripper{
		next:           http.DefaultTransport,
		cfg:            cfg,
		hmacPrivateKey: privateKey,
	}
	return &WebProxyConnection{
		name:    name,
		cfg:     cfg,
		tlsKey:  tlsKey,
		tlsCert: tlsCrt,
		tlsPem:  tlsPem,
		client: &http.Client{
			Transport: transport,
		},
		privateKey: privateKey,
	}, nil
}

// TLSCert returns the resolved TLS certificate data.
func (c *WebProxyConnection) TLSCert() []byte { return c.tlsCert }

// TLSKey returns the resolved TLS key data.
func (c *WebProxyConnection) TLSKey() []byte { return c.tlsKey }

// TLSPem returns the resolved TLS PEM data.
func (c *WebProxyConnection) TLSPem() []byte { return c.tlsPem }

// Type returns the resource type, which is "webproxy".
func (c *WebProxyConnection) Type() string {
	return "webproxy"
}

// Name returns the connection name.
func (c *WebProxyConnection) Name() string {
	return c.name
}

// Close closes idle connections associated with the HTTP client.
func (c *WebProxyConnection) Close() error {
	if c.client != nil && c.client.Transport != nil {
		if tr, ok := c.client.Transport.(*http.Transport); ok {
			tr.CloseIdleConnections()
		}
	}
	return nil
}

// GetClient returns the underlying http.Client.
func (c *WebProxyConnection) GetClient() *http.Client {
	return c.client
}

// WebProxyRoundTripper implements http.RoundTripper to inject authentication headers into requests.
type WebProxyRoundTripper struct {
	next http.RoundTripper
	cfg  config.WebProxyConfig

	// OAuth2-related fields
	oauth2Token     string
	oauth2ExpiresAt time.Time
	oauth2Mutex     sync.Mutex

	// HMAC-related fields
	hmacPrivateKey *rsa.PrivateKey
}

// RoundTrip processes the HTTP request, applies authentication, and executes the request.
func (t *WebProxyRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())

	if t.cfg.APIEndpoint != "" {
		parsedURL, err := url.Parse(t.cfg.APIEndpoint)
		if err != nil {
			return nil, fmt.Errorf("webproxy: invalid APIEndpoint configured: %w", err)
		}
		newPath := parsedURL.Path
		if !strings.HasSuffix(newPath, "/") && !strings.HasPrefix(req.URL.Path, "/") {
			newPath += "/"
		}
		newPath += req.URL.Path

		req.URL.Scheme = parsedURL.Scheme
		req.URL.Host = parsedURL.Host
		req.URL.Path = newPath
		req.URL.RawPath = newPath
	}

	switch t.cfg.Type {
	case "APIKEY":
		authValue := t.cfg.Key
		if t.cfg.Prefix != "" {
			authValue = fmt.Sprintf("%s %s", t.cfg.Prefix, authValue)
		}

		switch t.cfg.Placement {
		case "header":
			if t.cfg.KeyName != "" {
				req.Header.Set(t.cfg.KeyName, authValue)
			} else {
				return nil, fmt.Errorf("webproxy: APIKEY authentication with placement 'header' requires 'key_name'")
			}
		case "query":
			if t.cfg.KeyName != "" {
				q := req.URL.Query()
				q.Add(t.cfg.KeyName, authValue)
				req.URL.RawQuery = q.Encode()
			} else {
				return nil, fmt.Errorf("webproxy: APIKEY authentication with placement 'query' requires 'key_name'")
			}
		case "auth_header":
			req.Header.Set("Authorization", authValue)
		default:
			return nil, fmt.Errorf("webproxy: unsupported APIKEY placement type: %s", t.cfg.Placement)
		}
	case "OAUTH2":
		if t.oauth2Token == "" || time.Now().After(t.oauth2ExpiresAt.Add(-5*time.Minute)) {
			err := t.refreshOAuth2Token(req.Context())
			if err != nil {
				return nil, fmt.Errorf("webproxy: failed to refresh OAuth2 token: %w", err)
			}
		}
		if t.oauth2Token != "" {
			req.Header.Set("Authorization", "Bearer "+t.oauth2Token)
		} else {
			return nil, fmt.Errorf("webproxy: OAuth2 token is empty after refresh attempt")
		}
	case "HMAC":
		if t.hmacPrivateKey == nil {
			return nil, fmt.Errorf("webproxy: HMAC authentication requires a valid private key configured")
		}

		signaturePayload, ok := req.Context().Value(SignaturePayloadContextKey).(string)
		if !ok || signaturePayload == "" {
			return nil, fmt.Errorf("webproxy: HMAC authentication requires signature payload in context with key '%s'", SignaturePayloadContextKey)
		}

		var signed []byte
		var err error
		switch t.cfg.Algorithm {
		case "RSASSA-PSS":
			hashed := Sha256Sum([]byte(signaturePayload))
			signed, err = rsa.SignPSS(rand.Reader, t.hmacPrivateKey, CryptoSHA256(), hashed[:], nil)
			if err != nil {
				return nil, fmt.Errorf("webproxy: failed to sign with RSASSA-PSS: %w", err)
			}
		default:
			return nil, fmt.Errorf("webproxy: unsupported HMAC algorithm: %s", t.cfg.Algorithm)
		}

		authHeader := fmt.Sprintf("AMZ-PAY-RSASSA-PSS-V2 SignedHeaders=x-amz-pay-date;x-amz-pay-id;x-amz-pay-region, Signature=%s",
			url.QueryEscape(string(signed)))

		req.Header.Set("Authorization", authHeader)

		if t.cfg.PublicKeyId != "" {
			req.Header.Set("x-amz-pay-id", t.cfg.PublicKeyId)
		}
		if t.cfg.Region != "" {
			req.Header.Set("x-amz-pay-region", t.cfg.Region)
		}

	case "NONE":
		return t.next.RoundTrip(req)
	case "TLS":
		return t.next.RoundTrip(req)
	}

	return t.next.RoundTrip(req)
}

// refreshOAuth2Token acquires or refreshes an OAuth2 access token.
// It uses a mutex to ensure thread safety during token acquisition.
func (t *WebProxyRoundTripper) refreshOAuth2Token(ctx context.Context) error {
	t.oauth2Mutex.Lock()
	defer t.oauth2Mutex.Unlock()

	if t.oauth2Token != "" && time.Now().Before(t.oauth2ExpiresAt.Add(-5*time.Minute)) {
		return nil
	}

	if t.cfg.GrantType != "client_credentials" {
		return fmt.Errorf("webproxy: unsupported OAuth2 grant type: %s", t.cfg.GrantType)
	}
	if t.cfg.TokenUrl == "" || t.cfg.ClientId == "" || t.cfg.ClientSecret == "" {
		return fmt.Errorf("webproxy: OAuth2 client_credentials requires token_url, client_id, and client_secret")
	}

	data := url.Values{}
	data.Set("grant_type", "client_credentials")
	data.Set("client_id", t.cfg.ClientId)
	data.Set("client_secret", t.cfg.ClientSecret)

	tokenReq, err := http.NewRequestWithContext(ctx, "POST", t.cfg.TokenUrl, bytes.NewBufferString(data.Encode()))
	if err != nil {
		return fmt.Errorf("webproxy: failed to create token request: %w", err)
	}
	tokenReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Transport: t.next}
	resp, err := client.Do(tokenReq)
	if err != nil {
		return fmt.Errorf("webproxy: failed to send token request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("webproxy: token request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var tokenResponse struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResponse); err != nil {
		return fmt.Errorf("webproxy: failed to decode token response: %w", err)
	}

	if tokenResponse.AccessToken == "" {
		return fmt.Errorf("webproxy: access_token not found in response")
	}

	t.oauth2Token = tokenResponse.AccessToken
	t.oauth2ExpiresAt = time.Now().Add(time.Duration(tokenResponse.ExpiresIn) * time.Second)

	return nil
}

// WebProxyProvider manages and provides WebProxyConnection instances.
type WebProxyProvider struct {
	name        string
	configs     map[string]config.WebProxyConfig
	connections map[string]*WebProxyConnection
	mu          sync.RWMutex
	resolver    secret.SecretResolver
}

// NewWebProxyProvider creates a new WebProxyProvider with the given configurations and secret resolver.
func NewWebProxyProvider(configs map[string]config.WebProxyConfig, resolver secret.SecretResolver) *WebProxyProvider {
	return &WebProxyProvider{
		name:        "webproxy",
		configs:     configs,
		connections: make(map[string]*WebProxyConnection),
		resolver:    resolver,
	}
}

// GetConnection retrieves a WebProxyConnection by name, creating it if it does not exist.
func (p *WebProxyProvider) GetConnection(name string) (coreAdapter.ResourceConnection, error) {
	p.mu.RLock()
	conn, ok := p.connections[name]
	p.mu.RUnlock()
	if ok {
		return conn, nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	conn, ok = p.connections[name]
	if ok {
		return conn, nil
	}

	cfg, cfgOk := p.configs[name]
	if !cfgOk {
		return nil, fmt.Errorf("webproxy: configuration for connection '%s' not found", name)
	}

	newConn, err := NewWebProxyConnection(name, cfg, p.resolver)
	if err != nil {
		return nil, err
	}

	p.connections[name] = newConn
	return newConn, nil
}

// CloseAll closes all connections managed by the provider.
func (p *WebProxyProvider) CloseAll() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	var errs []error
	for name, conn := range p.connections {
		if err := conn.Close(); err != nil {
			errs = append(errs, fmt.Errorf("webproxy: failed to close connection '%s': %w", name, err))
		}
		delete(p.connections, name)
	}
	if len(errs) > 0 {
		return fmt.Errorf("webproxy: multiple errors occurred while closing connections: %v", errs)
	}
	return nil
}

// Type returns the provider type, which is "webproxy".
func (p *WebProxyProvider) Type() string {
	return "webproxy"
}

// Name returns the provider name.
func (p *WebProxyProvider) Name() string {
	return p.name
}

// Sha256Sum computes the SHA256 hash of the given data.
func Sha256Sum(data []byte) [32]byte {
	return sha256.Sum256(data)
}

// CryptoSHA256 returns the crypto.Hash value for SHA256.
func CryptoSHA256() crypto.Hash {
	return crypto.SHA256
}

var _ coreAdapter.ResourceProvider = (*WebProxyProvider)(nil)
