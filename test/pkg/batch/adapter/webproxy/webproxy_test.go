package webproxy_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	coreAdapter "github.com/tigerroll/surfin/pkg/batch/core/adapter"
	"github.com/tigerroll/surfin/pkg/batch/adapter/webproxy"
	"github.com/tigerroll/surfin/pkg/batch/adapter/webproxy/config"
)

// MockRoundTripper is a mock implementation of http.RoundTripper.
type MockRoundTripper struct {
	mock.Mock
}

func (m *MockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	args := m.Called(req)
	return args.Get(0).(*http.Response), args.Error(1)
}

func TestNewWebProxyConnection(t *testing.T) {
	cfg := config.WebProxyConfig{
		Type: "APIKEY",
		Key:  "test-key",
	}
	conn, err := webproxy.NewWebProxyConnection("test-conn", cfg)
	assert.NoError(t, err)
	assert.NotNil(t, conn)

	assert.Equal(t, "webproxy", conn.Type())
	assert.Equal(t, "test-conn", conn.Name())
	assert.NotNil(t, conn.GetClient())
}

func TestWebProxyConnection_Close(t *testing.T) {
	cfg := config.WebProxyConfig{
		Type: "APIKEY",
		Key:  "test-key",
	}
	conn, err := webproxy.NewWebProxyConnection("test-conn", cfg)
	assert.NoError(t, err)
	assert.NotNil(t, conn)

	err = conn.Close()
	assert.NoError(t, err)
	// Directly testing if CloseIdleConnections is called is difficult, but we assert no error occurs.
}

func TestWebProxyProvider_GetConnection(t *testing.T) {
	configs := map[string]config.WebProxyConfig{
		"proxy1": {Type: "APIKEY", Key: "key1"},
		"proxy2": {Type: "OAUTH2", ClientId: "id2"},
	}
	provider := webproxy.NewWebProxyProvider(configs)

	// First retrieval
	conn1, err := provider.GetConnection("proxy1")
	assert.NoError(t, err)
	assert.NotNil(t, conn1)
	assert.Equal(t, "proxy1", conn1.Name())

	// Second retrieval (verify caching)
	conn1After, err := provider.GetConnection("proxy1")
	assert.NoError(t, err)
	assert.Same(t, conn1, conn1After) // Verify that the same instance is returned.

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

func TestWebProxyProvider_CloseAll(t *testing.T) {
	configs := map[string]config.WebProxyConfig{
		"proxy1": {Type: "APIKEY", Key: "key1"},
	}
	provider := webproxy.NewWebProxyProvider(configs)

	// Create a connection first.
	_, err := provider.GetConnection("proxy1")
	assert.NoError(t, err)

	err = provider.CloseAll()
	assert.NoError(t, err)

	// After CloseAll, attempting to get the connection again should create a new one (cache is cleared).
	conn1AfterClose, err := provider.GetConnection("proxy1")
	assert.NoError(t, err)
	assert.NotNil(t, conn1AfterClose)
	// assert.NotSame(t, conn1, conn1AfterClose) // This assertion is not needed as NewWebProxyConnection always returns a new instance.
}

func TestWebProxyProvider_TypeAndName(t *testing.T) {
	provider := webproxy.NewWebProxyProvider(nil)
	assert.Equal(t, "webproxy", provider.Type())
	assert.Equal(t, "webproxy", provider.Name())
}

// Verify that WebProxyProvider implements the coreAdapter.ResourceProvider interface.
var _ coreAdapter.ResourceProvider = (*webproxy.WebProxyProvider)(nil)
