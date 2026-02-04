package test

import (
	"context"
	"database/sql"
	"testing"

	dbadapter "github.com/tigerroll/surfin/pkg/batch/adapter/database"
	dbconfig "github.com/tigerroll/surfin/pkg/batch/adapter/database/config"

	"gorm.io/gorm"
)

// MockDBConnection is a mock implementation of the adapter.DBConnection interface.
// It is used for testing purposes, particularly when a real database connection
// is not required or when simulating specific database behaviors.
type MockDBConnection struct {
	DB *gorm.DB // The underlying GORM DB instance, primarily for internal mock logic.
}

// NewMockDBConnection creates a new instance of MockDBConnection.
//
// Parameters:
//
//	db: An optional *gorm.DB instance. In most mock scenarios, this can be nil.
//
// Returns:
//
//	adapter.DBConnection: A new mock database connection.
func NewMockDBConnection(db *gorm.DB) dbadapter.DBConnection {
	return &MockDBConnection{DB: db}
}

// Close simulates closing the database connection.
// It always returns nil, indicating success.
func (m *MockDBConnection) Close() error {
	return nil
}

// Type returns the simulated database type.
// It always returns "mock_db".
func (m *MockDBConnection) Type() string {
	return "mock_db"
}

// Name returns the simulated connection name.
// It always returns "mock_name".
func (m *MockDBConnection) Name() string {
	return "mock_name"
}

// RefreshConnection simulates refreshing the database connection.
// It always returns nil, indicating success.
func (m *MockDBConnection) RefreshConnection(ctx context.Context) error {
	return nil
}

// Config returns a dummy database configuration.
// It returns an empty dbconfig.DatabaseConfig struct.
func (m *MockDBConnection) Config() dbconfig.DatabaseConfig {
	return dbconfig.DatabaseConfig{}
}

// GetSQLDB returns a nil *sql.DB and nil error, as it's a mock.
func (m *MockDBConnection) GetSQLDB() (*sql.DB, error) {
	return nil, nil
}

// ExecuteQuery simulates a database query operation.
// It does nothing and always returns nil.
func (m *MockDBConnection) ExecuteQuery(ctx context.Context, target interface{}, query map[string]interface{}) error {
	return nil
}

// ExecuteQueryAdvanced simulates an advanced database query operation.
// It does nothing and always returns nil.
func (m *MockDBConnection) ExecuteQueryAdvanced(ctx context.Context, target interface{}, query map[string]interface{}, orderBy string, limit int) error {
	return nil
}

// Count simulates counting records in a database.
// It always returns 0 and nil error.
func (m *MockDBConnection) Count(ctx context.Context, model interface{}, query map[string]interface{}) (int64, error) {
	return 0, nil
}

// Pluck simulates plucking a column's values from a database.
// It does nothing and always returns nil.
func (m *MockDBConnection) Pluck(ctx context.Context, model interface{}, column string, target interface{}, query map[string]interface{}) error {
	return nil
}

// ExecuteUpdate simulates a database update operation (CREATE, UPDATE, DELETE).
// It returns 1 (simulating one row affected) and nil error for "CREATE" or "UPDATE" operations,
// and 0 and nil error otherwise.
func (m *MockDBConnection) ExecuteUpdate(ctx context.Context, model interface{}, operation string, tableName string, query map[string]interface{}) (rowsAffected int64, err error) {
	if operation == "CREATE" || operation == "UPDATE" {
		return 1, nil
	}
	return 0, nil
}

// ExecuteUpsert simulates a database upsert operation.
// It always returns 1 (simulating one row affected) and nil error.
func (m *MockDBConnection) ExecuteUpsert(ctx context.Context, model interface{}, tableName string, conflictColumns []string, updateColumns []string) (rowsAffected int64, err error) {
	return 1, nil
}

// IsTableNotExistError simulates checking if an error indicates a table does not exist.
// It always returns false.
func (m *MockDBConnection) IsTableNotExistError(err error) bool {
	return false
}

// MockProviderFactory is a dummy implementation of a provider factory for testing.
// It does not provide any actual functionality.
type MockProviderFactory struct{}

// GetProvider always returns nil for both the provider and error.
func (m *MockProviderFactory) GetProvider(dbType string) (interface{}, error) {
	return nil, nil
}

// CreateTestDBConnection creates a mock DBConnection for testing.
// This is a helper function to simplify test setups.
//
// Parameters:
//
//	t: The testing.T instance for test reporting.
//	db: An optional *gorm.DB instance to be wrapped by the mock.
//
// Returns:
//
//	adapter.DBConnection: A new mock database connection.
func CreateTestDBConnection(t *testing.T, db *gorm.DB) dbadapter.DBConnection {
	t.Helper()
	return NewMockDBConnection(db)
}
