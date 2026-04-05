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

	coreAdapter "github.com/tigerroll/surfin/pkg/batch/core/adapter"
	"github.com/tigerroll/surfin/pkg/batch/adapter/webproxy/config"
)

// ContextKeySignaturePayload is the key used to store the payload for HMAC signing in context.Context.
type ContextKeySignaturePayload string

const (
	// SignaturePayloadContextKey is the key for storing the string to be signed in the context.
	SignaturePayloadContextKey ContextKeySignaturePayload = "signaturePayload"
	// SignatureHeaderContextKey is the key for storing the signed header in the context.
	// This can be used when HMAC signature applies to a custom header other than the Authorization header.
	SignatureHeaderContextKey ContextKeySignaturePayload = "signatureHeader"
)

// WebProxyConnection represents a connection to a Web Proxy.
type WebProxyConnection struct {
	name   string
	cfg    config.WebProxyConfig
	client *http.Client // The actual HTTP client.
	// HMAC-related fields
	privateKey *rsa.PrivateKey // Parsed private key.
}

// NewWebProxyConnection creates a new WebProxyConnection.
//
// Parameters:
//   name: The name of the connection.
//   cfg: The WebProxyConfig for this connection.
//
// Returns:
//   A new WebProxyConnection instance.
//   An error if private key parsing or configuration validation fails.
func NewWebProxyConnection(name string, cfg config.WebProxyConfig) (*WebProxyConnection, error) {
	var privateKey *rsa.PrivateKey
	if cfg.Type == "HMAC" && cfg.PrivateKey != "" {
		// Parse the private key.
		block, _ := pem.Decode([]byte(cfg.PrivateKey))
		if block == nil {
			return nil, fmt.Errorf("webproxy: failed to decode PEM block for private key for HMAC connection '%s'", name)
		} else {
			parsedKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
			if err != nil {
				// If PKCS8 parsing fails, try PKCS1 format.
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

	// Validate APIKEY type configuration.
	if cfg.Type == "APIKEY" {
		if cfg.Key == "" {
			return nil, fmt.Errorf("webproxy: APIKEY type for connection '%s' requires 'key' to be configured (check environment variable)", name)
		}
		if (cfg.Placement == "query" || cfg.Placement == "header") && cfg.KeyName == "" {
			return nil, fmt.Errorf("webproxy: APIKEY authentication with placement '%s' for connection '%s' requires 'key_name'", cfg.Placement, name)
		}
	}

	transport := &WebProxyRoundTripper{
		next: http.DefaultTransport, // Wrap the default transport.
		cfg:  cfg,
		// Pass the parsed private key to WebProxyRoundTripper.
		hmacPrivateKey: privateKey,
	}
	return &WebProxyConnection{
		name: name,
		cfg:  cfg,
		client: &http.Client{
			Transport: transport,
		},
		privateKey: privateKey, // Also keep it in WebProxyConnection itself.
	}, nil
}

// Type returns the type of the resource.
//
// Returns:
//   The string "webproxy".
func (c *WebProxyConnection) Type() string {
	return "webproxy"
}

// Name returns the connection name.
//
// Returns:
//   The name of the connection.
func (c *WebProxyConnection) Name() string {
	return c.name
}

// Close closes the connection.
//
// Returns:
//   An error if closing idle connections fails.
func (c *WebProxyConnection) Close() error {
	// Close idle connections of the HTTP client.
	if c.client != nil && c.client.Transport != nil {
		if tr, ok := c.client.Transport.(*http.Transport); ok {
			tr.CloseIdleConnections()
		}
	}
	return nil
}

// GetClient returns the http.Client associated with this connection.
//
// Returns:
//   The *http.Client instance.
func (c *WebProxyConnection) GetClient() *http.Client {
	return c.client
}

// WebProxyRoundTripper implements the http.RoundTripper interface and
// is responsible for adding authentication information to requests.
type WebProxyRoundTripper struct {
	next http.RoundTripper
	cfg  config.WebProxyConfig

	// OAuth2-related fields
	oauth2Token    string
	oauth2ExpiresAt time.Time
	oauth2Mutex    sync.Mutex // Mutex to prevent race conditions during token acquisition.

	// HMAC-related fields
	hmacPrivateKey *rsa.PrivateKey // Parsed private key.
}

// RoundTrip processes the request and adds authentication information.
//
// Parameters:
//   req: The HTTP request to process.
//
// Returns:
//   The HTTP response.
//   An error if authentication fails or the request cannot be sent.
func (t *WebProxyRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// Copy the request to avoid modifying the original.
	// This is an http.RoundTripper best practice.
	req = req.Clone(req.Context())

	// If APIEndpoint is configured, rewrite the request URL.
	if t.cfg.APIEndpoint != "" {
		parsedURL, err := url.Parse(t.cfg.APIEndpoint)
		if err != nil {
			return nil, fmt.Errorf("webproxy: invalid APIEndpoint configured: %w", err)
		}
		// Rewrite the Scheme, Host, and Path of the request URL.
		// req.URL.Path is combined with parsedURL.Path.
		// Example: parsedURL.Path = "/v1", req.URL.Path = "/forecast" -> newPath = "/v1/forecast"
		newPath := parsedURL.Path
		if !strings.HasSuffix(newPath, "/") && !strings.HasPrefix(req.URL.Path, "/") {
			newPath += "/"
		}
		newPath += req.URL.Path // Combine with the original path.

		req.URL.Scheme = parsedURL.Scheme
		req.URL.Host = parsedURL.Host
		req.URL.Path = newPath
		req.URL.RawPath = newPath // Also update RawPath.
	}

	switch t.cfg.Type {
	case "APIKEY":
		switch t.cfg.Placement {
		case "header":
			if t.cfg.KeyName != "" {
				req.Header.Set(t.cfg.KeyName, t.cfg.Key)
			} else {
				return nil, fmt.Errorf("webproxy: APIKEY authentication with placement 'header' requires 'key_name'")
			}
		case "query":
			if t.cfg.KeyName != "" {
				q := req.URL.Query()
				q.Add(t.cfg.KeyName, t.cfg.Key)
				req.URL.RawQuery = q.Encode()
			} else {
				return nil, fmt.Errorf("webproxy: APIKEY authentication with placement 'query' requires 'key_name'")
			}
		case "auth_header":
			// Set the key directly in the Authorization header. KeyName is not used.
			req.Header.Set("Authorization", t.cfg.Key)
		default:
			return nil, fmt.Errorf("webproxy: unsupported APIKEY placement type: %s", t.cfg.Placement)
		}
	case "OAUTH2":
		// Acquire/refresh token if expired or not yet obtained.
		if t.oauth2Token == "" || time.Now().After(t.oauth2ExpiresAt.Add(-5*time.Minute)) { // Try to refresh 5 minutes before expiry.
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

		// Get the string to be signed from the context.
		signaturePayload, ok := req.Context().Value(SignaturePayloadContextKey).(string)
		if !ok || signaturePayload == "" {
			return nil, fmt.Errorf("webproxy: HMAC authentication requires signature payload in context with key '%s'", SignaturePayloadContextKey)
		}

		// Select signing algorithm and generate signature.
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

		// Construct the Authorization header.
		// Although the design document states injection into the Authorization header, the specific format depends on the external API.
		// Here, we construct the Authorization header referencing the Amazon Pay v2 example.
		// It's possible to add flexibility, e.g., by getting the header name from the context, if needed.
		authHeader := fmt.Sprintf("AMZ-PAY-RSASSA-PSS-V2 SignedHeaders=x-amz-pay-date;x-amz-pay-id;x-amz-pay-region, Signature=%s",
			url.QueryEscape(string(signed))) // URL-encode the signature.
		// In Amazon Pay v2 examples, the signature itself is often Base64 encoded, but simplified here.
		// Adjustment according to actual API specifications may be necessary.

		// Inject into the Authorization header as specified in the design document.
		req.Header.Set("Authorization", authHeader)

		// Other HMAC-related headers (e.g., x-amz-pay-date, x-amz-pay-id, x-amz-pay-region) are
		// expected to be set by the ItemReader.
		// If necessary, these header values can be included in WebProxyConfig or
		// logic to retrieve them from Context can be added.
		if t.cfg.PublicKeyId != "" {
			req.Header.Set("x-amz-pay-id", t.cfg.PublicKeyId)
		}
		if t.cfg.Region != "" {
			req.Header.Set("x-amz-pay-region", t.cfg.Region)
		}
		// x-amz-pay-date is typically a timestamp at the time of request submission, so it should be set by the ItemReader.
		// Setting it here might cause a mismatch with the signature payload calculated by the ItemReader.

	case "NONE":
		// For "NONE" type, no authentication logic is applied; the request is passed directly to the next RoundTripper.
		// This means the WebProxy simply acts as a transparent pass-through.
		return t.next.RoundTrip(req)
	default:
		// Unknown authentication type, or no authentication required.
		// However, this path should typically not be reached as unknown/unsupported types are skipped in module.go.
	}

	return t.next.RoundTrip(req)
}

// refreshOAuth2Token acquires or refreshes an OAuth2 access token.
// It safely handles concurrent calls from multiple goroutines.
//
// Parameters:
//   ctx: The context for the operation.
//
// Returns:
//   An error if token acquisition or refresh fails.
func (t *WebProxyRoundTripper) refreshOAuth2Token(ctx context.Context) error {
	t.oauth2Mutex.Lock()
	defer t.oauth2Mutex.Unlock()

	// Re-check token validity after acquiring the lock (double-checked locking).
	if t.oauth2Token != "" && time.Now().Before(t.oauth2ExpiresAt.Add(-5*time.Minute)) {
		return nil // Another goroutine has already refreshed.
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

	// Use a new client with the default Transport for token acquisition to avoid
	// using its own RoundTripper, which would cause an infinite loop.
	client := &http.Client{Transport: t.next} // Use t.next to avoid infinite loop.
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
		ExpiresIn   int    `json:"expires_in"` // seconds
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResponse); err != nil {
		return fmt.Errorf("webproxy: failed to decode token response: %w", err)
	}

	if tokenResponse.AccessToken == "" {
		return fmt.Errorf("webproxy: access_token not found in response")
	}

	t.oauth2Token = tokenResponse.AccessToken
	// Expiry is current time + ExpiresIn seconds.
	t.oauth2ExpiresAt = time.Now().Add(time.Duration(tokenResponse.ExpiresIn) * time.Second)

	return nil
}

// WebProxyProvider provides WebProxyConnection instances.
type WebProxyProvider struct {
	name        string
	configs     map[string]config.WebProxyConfig
	connections map[string]*WebProxyConnection
	mu          sync.RWMutex // Protects access to the connections map.
}

// NewWebProxyProvider creates a new WebProxyProvider.
// This function accepts a map of WebProxyConfig parsed by the Fx module.
//
// Parameters:
//   configs: A map of WebProxyConfig instances, keyed by connection name.
//
// Returns:
//   A new WebProxyProvider instance.
func NewWebProxyProvider(configs map[string]config.WebProxyConfig) *WebProxyProvider {
	return &WebProxyProvider{
		name:        "webproxy", // Fixed name for the provider.
		configs:     configs,
		connections: make(map[string]*WebProxyConnection),
	}
}

// GetConnection retrieves a WebProxyConnection with the specified name.
//
// Parameters:
//   name: The name of the connection to retrieve.
//
// Returns:
//   The ResourceConnection instance.
//   An error if the configuration for the connection is not found or connection creation fails.
func (p *WebProxyProvider) GetConnection(name string) (coreAdapter.ResourceConnection, error) {
	p.mu.RLock()
	conn, ok := p.connections[name]
	p.mu.RUnlock()
	if ok {
		return conn, nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Double-checked locking.
	conn, ok = p.connections[name]
	if ok {
		return conn, nil
	}

	cfg, cfgOk := p.configs[name]
	if !cfgOk {
		return nil, fmt.Errorf("webproxy: configuration for connection '%s' not found", name)
	}

	newConn, err := NewWebProxyConnection(name, cfg)
	if err != nil {
		return nil, err // Propagate the error from NewWebProxyConnection.
	}

	p.connections[name] = newConn
	return newConn, nil
}

// CloseAll closes all connections managed by this provider.
//
// Returns:
//   An error if any connection fails to close.
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

// Type returns the type of the provider.
//
// Returns:
//   The string "webproxy".
func (p *WebProxyProvider) Type() string {
	return "webproxy"
}

// Name returns the name of the provider.
//
// Returns:
//   The name of the provider.
func (p *WebProxyProvider) Name() string {
	return p.name
}

// Sha256Sum hashes a byte slice with SHA256.
//
// Parameters:
//   data: The byte slice to hash.
//
// Returns:
//   The 32-byte SHA256 hash.
func Sha256Sum(data []byte) [32]byte {
	return sha256.Sum256(data)
}

// CryptoSHA256 returns crypto.SHA256.
//
// Returns:
//   The crypto.Hash value for SHA256.
func CryptoSHA256() crypto.Hash {
	return crypto.SHA256
}

// Verify that WebProxyProvider implements coreAdapter.ResourceProvider interface.
var _ coreAdapter.ResourceProvider = (*WebProxyProvider)(nil)
