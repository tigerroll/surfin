package test

import (
	"context"
	"database/sql"
	"testing"

	adapter "surfin/pkg/batch/core/adapter" // Changed from port to adapter
	config "surfin/pkg/batch/core/config"

	"gorm.io/gorm"
)

// --- Mock/Helper Structures ---

// MockProviderFactory is defined in pkg/batch/test/factory.go.

// MockDBConnection is a mock implementation of adapter.DBConnection for testing GORM repositories.
type MockDBConnection struct {
	DB *gorm.DB
}

// NewMockDBConnection creates a new MockDBConnection instance.
func NewMockDBConnection(db *gorm.DB) adapter.DBConnection {
	return &MockDBConnection{DB: db}
}

func (m *MockDBConnection) Close() error {
	return nil
}
func (m *MockDBConnection) Type() string {
	return "mock_db" // Returns a fixed value for mock.
}
func (m *MockDBConnection) RefreshConnection(ctx context.Context) error {
	return nil
}
func (m *MockDBConnection) Config() config.DatabaseConfig {
	return config.DatabaseConfig{}
}

func (m *MockDBConnection) Name() string {
	return "mock_name"
}

// GetSQLDB implements adapter.DBConnection.
func (m *MockDBConnection) GetSQLDB() (*sql.DB, error) {
	return nil, nil // Returns nil for testing purposes.
}

// ExecuteQuery implements adapter.DBConnection.
func (m *MockDBConnection) ExecuteQuery(ctx context.Context, target interface{}, query map[string]interface{}) error {
	return nil // Does nothing as it's a mock.
}

// ExecuteQueryAdvanced implements adapter.DBConnection.
func (m *MockDBConnection) ExecuteQueryAdvanced(ctx context.Context, target interface{}, query map[string]interface{}, orderBy string, limit int) error {
	return nil // Does nothing as it's a mock.
}

// Count implements adapter.DBConnection.
func (m *MockDBConnection) Count(ctx context.Context, model interface{}, query map[string]interface{}) (int64, error) {
	return 0, nil // Returns 0 as it's a mock.
}

// Pluck implements adapter.DBConnection.
func (m *MockDBConnection) Pluck(ctx context.Context, model interface{}, column string, target interface{}, query map[string]interface{}) error {
	return nil // Does nothing as it's a mock.
}

// ExecuteUpdate implements adapter.DBConnection.
func (m *MockDBConnection) ExecuteUpdate(ctx context.Context, model interface{}, operation string, tableName string, query map[string]interface{}) (rowsAffected int64, err error) {
	// Returns 1 assuming success, as it's a mock.
	if operation == "CREATE" || operation == "UPDATE" {
		return 1, nil
	}
	return 0, nil
}

// ExecuteUpsert implements adapter.DBConnection.
func (m *MockDBConnection) ExecuteUpsert(ctx context.Context, model interface{}, tableName string, conflictColumns []string, updateColumns []string) (rowsAffected int64, err error) {
	// Returns 1 assuming success, as it's a mock.
	return 1, nil
}

// MockProviderFactory is a dummy ProviderFactory for testing.
type MockProviderFactory struct{}

// GetProvider always returns nil.
func (m *MockProviderFactory) GetProvider(dbType string) (interface{}, error) {
	return nil, nil
}

// CreateTestDBConnection creates a DBConnection for testing.
func CreateTestDBConnection(t *testing.T, db *gorm.DB) adapter.DBConnection {
	t.Helper()
	// Uses MockDBConnection.
	return NewMockDBConnection(db)
}
