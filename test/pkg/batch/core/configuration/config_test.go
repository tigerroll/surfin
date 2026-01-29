package configuration_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	dbconfig "github.com/tigerroll/surfin/pkg/batch/adapter/database/config"  // Correct path for DatabaseConfig
	gormadapter "github.com/tigerroll/surfin/pkg/batch/adapter/database/gorm" // Correct path for GetConnectionStringForTest
	coreconfig "github.com/tigerroll/surfin/pkg/batch/core/config"            // Path for core configuration
)

// TestNewConfig_Defaults verifies that the NewConfig function correctly initializes
// the application configuration with expected default values.
func TestNewConfig_Defaults(t *testing.T) {
	cfg := coreconfig.NewConfig()

	if cfg.Surfin.System.Timezone != "UTC" {
		t.Errorf("Expected default Timezone 'UTC', got %s", cfg.Surfin.System.Timezone)
	}
	if cfg.Surfin.System.Logging.Level != "INFO" {
		t.Errorf("Expected default Logging Level 'INFO', got %s", cfg.Surfin.System.Logging.Level)
	}
	if cfg.Surfin.Batch.ChunkSize != 10 {
		t.Errorf("Expected default ChunkSize 10, got %d", cfg.Surfin.Batch.ChunkSize)
	}
	if cfg.Surfin.Batch.StepExecutorRef != "simpleStepExecutor" {
		t.Errorf("Expected default StepExecutorRef 'simpleStepExecutor', got %s", cfg.Surfin.Batch.StepExecutorRef)
	}
	if len(cfg.Surfin.Security.MaskedParameterKeys) == 0 {
		t.Errorf("Expected default MaskedParameterKeys to be set")
	}
	if cfg.Surfin.Infrastructure.JobRepositoryDBRef != "metadata" {
		t.Errorf("Expected default JobRepositoryDBRef 'metadata', got %s", cfg.Surfin.Infrastructure.JobRepositoryDBRef)
	}
}

// TestDatabaseConfig_ConnectionString verifies that GetConnectionStringForTest
// correctly generates connection strings for different database types (PostgreSQL, MySQL, SQLite).
func TestDatabaseConfig_ConnectionString(t *testing.T) {
	// PostgreSQL
	cfg := dbconfig.DatabaseConfig{ // Use dbconfig.DatabaseConfig
		Type:     "postgres",
		Host:     "pg_host",
		Port:     5432,
		Database: "pg_db",
		User:     "pg_user",
		Password: "pg_password",
		Sslmode:  "require",
	}
	connStr, err := gormadapter.GetConnectionStringForTest(cfg) // Use gormadapter.GetConnectionStringForTest
	assert.NoError(t, err)
	expected := "host=pg_host port=5432 user=pg_user password=pg_password dbname=pg_db sslmode=require"
	assert.Equal(t, expected, connStr)

	// MySQL
	cfg = dbconfig.DatabaseConfig{
		Type:     "mysql",
		Host:     "mysql_host",
		Port:     3306,
		Database: "mysql_db",
		User:     "mysql_user",
		Password: "mysql_password",
	}
	connStr, err = gormadapter.GetConnectionStringForTest(cfg) // Use gormadapter.GetConnectionStringForTest
	assert.NoError(t, err)
	expected = "mysql_user:mysql_password@tcp(mysql_host:3306)/mysql_db?charset=utf8mb4&parseTime=True&loc=Local"
	assert.Equal(t, expected, connStr)

	// MySQL (No Password)
	cfg = dbconfig.DatabaseConfig{
		Type:     "mysql",
		Host:     "mysql_host",
		Port:     3306,
		Database: "mysql_db",
		User:     "mysql_user",
		Password: "",
	}
	connStr, err = gormadapter.GetConnectionStringForTest(cfg) // Use gormadapter.GetConnectionStringForTest
	assert.NoError(t, err)
	expected = "mysql_user@tcp(mysql_host:3306)/mysql_db?charset=utf8mb4&parseTime=True&loc=Local"
	assert.Equal(t, expected, connStr)

	// SQLite
	cfg = dbconfig.DatabaseConfig{
		Type:     "sqlite",
		Database: "/path/to/sqlite.db",
	}
	connStr, err = gormadapter.GetConnectionStringForTest(cfg) // Use gormadapter.GetConnectionStringForTest
	assert.NoError(t, err)
	expected = "/path/to/sqlite.db"
	assert.Equal(t, expected, connStr)
}

// setupMaskingConfig is a helper function that temporarily sets the global configuration
// for masked parameter keys and returns a cleanup function to restore the original state.
func setupMaskingConfig(keys []string) func() {
	originalConfig := coreconfig.GlobalConfig
	cfg := coreconfig.NewConfig()
	cfg.Surfin.Security.MaskedParameterKeys = keys
	coreconfig.GlobalConfig = cfg

	return func() {
		coreconfig.GlobalConfig = originalConfig // Use coreconfig
	}
}

// TestGetMaskedParameterKeys verifies that the GetMaskedParameterKeys function
// correctly retrieves the list of keys to be masked from the global configuration.
// It tests scenarios where GlobalConfig is nil and where it is properly set.
func TestGetMaskedParameterKeys(t *testing.T) {
	defer setupMaskingConfig([]string{"token", "secret"})()

	// 1. When GlobalConfig is nil
	coreconfig.GlobalConfig = nil               // Use coreconfig
	keys := coreconfig.GetMaskedParameterKeys() // Use coreconfig
	if len(keys) != 0 {
		t.Errorf("Expected 0 keys when GlobalConfig is nil, got %d", len(keys))
	}

	// 2. When GlobalConfig is set
	cfg := coreconfig.NewConfig() // Use coreconfig
	cfg.Surfin.Security.MaskedParameterKeys = []string{"token", "secret"}
	coreconfig.GlobalConfig = cfg // Use coreconfig

	keys = coreconfig.GetMaskedParameterKeys() // Use coreconfig
	expected := []string{"token", "secret"}
	if len(keys) != len(expected) || keys[0] != expected[0] || keys[1] != expected[1] {
		t.Errorf("Expected %v, got %v", expected, keys)
	}
}
