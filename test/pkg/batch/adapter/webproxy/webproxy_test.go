package webproxy_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/tigerroll/surfin/pkg/batch/adapter/webproxy"
	"github.com/tigerroll/surfin/pkg/batch/adapter/webproxy/config"
	coreAdapter "github.com/tigerroll/surfin/pkg/batch/core/adapter"
)

// MockSecretResolver is a mock implementation of secret.SecretResolver for testing purposes.
type MockSecretResolver struct{}

// Resolve returns the provided URI as the resolved secret value.
func (m *MockSecretResolver) Resolve(uri string) (any, error) {
	return uri, nil
}

// MockRoundTripper is a mock implementation of http.RoundTripper for testing HTTP requests.
type MockRoundTripper struct {
	mock.Mock
}

// RoundTrip executes the mocked HTTP request and returns the pre-configured response.
func (m *MockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	args := m.Called(req)
	return args.Get(0).(*http.Response), args.Error(1)
}

// TestNewWebProxyConnection verifies that a new WebProxyConnection can be initialized correctly.
func TestNewWebProxyConnection(t *testing.T) {
	cfg := config.WebProxyConfig{
		Type: "APIKEY",
		Key:  "test-key",
	}
	conn, err := webproxy.NewWebProxyConnection("test-conn", cfg, &MockSecretResolver{})
	assert.NoError(t, err)
	assert.NotNil(t, conn)

	assert.Equal(t, "webproxy", conn.Type())
	assert.Equal(t, "test-conn", conn.Name())
	assert.NotNil(t, conn.GetClient())
}

// TestWebProxyConnection_Close verifies that the connection can be closed without errors.
func TestWebProxyConnection_Close(t *testing.T) {
	cfg := config.WebProxyConfig{
		Type: "APIKEY",
		Key:  "test-key",
	}
	conn, err := webproxy.NewWebProxyConnection("test-conn", cfg, &MockSecretResolver{})
	assert.NoError(t, err)
	assert.NotNil(t, conn)

	err = conn.Close()
	assert.NoError(t, err)
}

// TestWebProxyProvider_GetConnection verifies that connections are correctly retrieved, cached, and handled when missing.
func TestWebProxyProvider_GetConnection(t *testing.T) {
	configs := map[string]config.WebProxyConfig{
		"proxy1": {Type: "APIKEY", Key: "key1"},
		"proxy2": {Type: "OAUTH2", ClientId: "id2"},
	}
	provider := webproxy.NewWebProxyProvider(configs, &MockSecretResolver{})

	// First retrieval
	conn1, err := provider.GetConnection("proxy1")
	assert.NoError(t, err)
	assert.NotNil(t, conn1)
	assert.Equal(t, "proxy1", conn1.Name())

	// Second retrieval (verify caching)
	conn1After, err := provider.GetConnection("proxy1")
	assert.NoError(t, err)
	assert.Same(t, conn1, conn1After)

	// Retrieve another connection
	conn2, err := provider.GetConnection("proxy2")
	assert.NoError(t, err)
	assert.NotNil(t, conn2)
	assert.Equal(t, "proxy2", conn2.Name())

	// Non-existent connection
	_, err = provider.GetConnection("non-existent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "configuration for connection 'non-existent' not found")
}

// TestWebProxyProvider_CloseAll verifies that all managed connections are closed and the cache is cleared.
func TestWebProxyProvider_CloseAll(t *testing.T) {
	configs := map[string]config.WebProxyConfig{
		"proxy1": {Type: "APIKEY", Key: "key1"},
	}
	provider := webproxy.NewWebProxyProvider(configs, &MockSecretResolver{})

	// Create a connection first.
	_, err := provider.GetConnection("proxy1")
	assert.NoError(t, err)

	err = provider.CloseAll()
	assert.NoError(t, err)

	// After CloseAll, attempting to get the connection again should create a new one (cache is cleared).
	conn1AfterClose, err := provider.GetConnection("proxy1")
	assert.NoError(t, err)
	assert.NotNil(t, conn1AfterClose)
}

// TestWebProxyProvider_TypeAndName verifies the Type and Name methods of the provider.
func TestWebProxyProvider_TypeAndName(t *testing.T) {
	provider := webproxy.NewWebProxyProvider(nil, &MockSecretResolver{})
	assert.Equal(t, "webproxy", provider.Type())
	assert.Equal(t, "webproxy", provider.Name())
}

// Verify that WebProxyProvider implements the coreAdapter.ResourceProvider interface.
var _ coreAdapter.ResourceProvider = (*webproxy.WebProxyProvider)(nil)
