package configuration_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"surfin/pkg/batch/adapter/database"
	config "surfin/pkg/batch/core/config"
)

func TestNewConfig_Defaults(t *testing.T) {
	cfg := config.NewConfig()

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

func TestDatabaseConfig_ConnectionString(t *testing.T) {
	// PostgreSQL
	cfg := config.DatabaseConfig{
		Type:     "postgres",
		Host:     "pg_host",
		Port:     5432,
		Database: "pg_db",
		User:     "pg_user",
		Password: "pg_password",
		Sslmode:  "require",
	}
	connStr, err := database.GetConnectionStringForTest(cfg)
	assert.NoError(t, err)
	expected := "host=pg_host port=5432 user=pg_user password=pg_password dbname=pg_db sslmode=require"
	assert.Equal(t, expected, connStr)

	// MySQL
	cfg = config.DatabaseConfig{
		Type:     "mysql",
		Host:     "mysql_host",
		Port:     3306,
		Database: "mysql_db",
		User:     "mysql_user",
		Password: "mysql_password",
	}
	connStr, err = database.GetConnectionStringForTest(cfg)
	assert.NoError(t, err)
	expected = "mysql_user:mysql_password@tcp(mysql_host:3306)/mysql_db?charset=utf8mb4&parseTime=True&loc=Local"
	assert.Equal(t, expected, connStr)

	// MySQL (No Password)
	cfg = config.DatabaseConfig{
		Type:     "mysql",
		Host:     "mysql_host",
		Port:     3306,
		Database: "mysql_db",
		User:     "mysql_user",
		Password: "",
	}
	connStr, err = database.GetConnectionStringForTest(cfg)
	assert.NoError(t, err)
	expected = "mysql_user@tcp(mysql_host:3306)/mysql_db?charset=utf8mb4&parseTime=True&loc=Local"
	assert.Equal(t, expected, connStr)

	// SQLite
	cfg = config.DatabaseConfig{
		Type:     "sqlite",
		Database: "/path/to/sqlite.db",
	}
	connStr, err = database.GetConnectionStringForTest(cfg)
	assert.NoError(t, err)
	expected = "/path/to/sqlite.db"
	assert.Equal(t, expected, connStr)
}

func TestGetMaskedParameterKeys(t *testing.T) {
	// 既存の GlobalConfig を保存
	originalConfig := config.GlobalConfig
	defer func() {
		config.GlobalConfig = originalConfig
	}()

	// 1. GlobalConfig が nil の場合
	config.GlobalConfig = nil
	keys := config.GetMaskedParameterKeys()
	if len(keys) != 0 {
		t.Errorf("Expected 0 keys when GlobalConfig is nil, got %d", len(keys))
	}

	// 2. GlobalConfig が設定されている場合
	cfg := config.NewConfig()
	cfg.Surfin.Security.MaskedParameterKeys = []string{"token", "secret"}
	config.GlobalConfig = cfg

	keys = config.GetMaskedParameterKeys()
	expected := []string{"token", "secret"}
	if len(keys) != len(expected) || keys[0] != expected[0] || keys[1] != expected[1] {
		t.Errorf("Expected %v, got %v", expected, keys)
	}
}
