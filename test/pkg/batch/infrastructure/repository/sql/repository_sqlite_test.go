// Package sql_test provides integration tests for the SQL repository implementations,
// specifically using an in-memory SQLite database for testing.
package sql_test

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	dbadapter "github.com/tigerroll/surfin/pkg/batch/adapter/database"
	dbconfig "github.com/tigerroll/surfin/pkg/batch/adapter/database/config"
	gormadapter "github.com/tigerroll/surfin/pkg/batch/adapter/database/gorm"
	_ "github.com/tigerroll/surfin/pkg/batch/adapter/database/gorm/sqlite"
	model "github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	"github.com/tigerroll/surfin/pkg/batch/core/domain/repository"
	tx "github.com/tigerroll/surfin/pkg/batch/core/tx"
	sqlrepo "github.com/tigerroll/surfin/pkg/batch/infrastructure/repository/sql"
	"github.com/tigerroll/surfin/pkg/batch/support/util/exception"
	test_util "github.com/tigerroll/surfin/pkg/batch/test"
	testmock "github.com/tigerroll/surfin/pkg/batch/test"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	sqlite_driver "gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"log"
	"os"
	"sync"
	"testing"
	"time"
)

const sqliteDBFile = "test_metadata_sqlite.db"

var (
	globalGormDB    *gorm.DB
	globalDBConn    dbadapter.DBConnection // The single database connection to be returned.
	globalTxManager tx.TransactionManager
	once            sync.Once
	testDBResolver  dbadapter.DBConnectionResolver // The test-specific DB connection resolver.
)

// GetConnectionStringForTest generates a database connection string for testing purposes.
// This function is a test helper and is not part of the main application logic.
func GetConnectionStringForTest(cfg dbconfig.DatabaseConfig) (string, error) {
	switch cfg.Type {
	case "postgres":
		// Example DSN: "host=pg_host port=5432 user=pg_user password=pg_password dbname=pg_db sslmode=require"
		return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
			cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Database, cfg.Sslmode), nil
	case "mysql":
		// Example DSN: "mysql_user:mysql_password@tcp(mysql_host:3306)/mysql_db?charset=utf8mb4&parseTime=True&loc=Local"
		// Handles cases with and without password.
		dsn := fmt.Sprintf("%s", cfg.User)
		if cfg.Password != "" {
			dsn += fmt.Sprintf(":%s", cfg.Password)
		}
		dsn += fmt.Sprintf("@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			cfg.Host, cfg.Port, cfg.Database)
		return dsn, nil
	case "sqlite":
		// Example DSN: "/path/to/sqlite.db" or ":memory:"
		return cfg.Database, nil
	default:
		return "", fmt.Errorf("unsupported database type for connection string generation: %s", cfg.Type)
	}
}

// setupSQLiteTestDB sets up the SQLite test environment.
// It shares a single in-memory DB connection across the entire test suite.
func setupSQLiteTestDB(t *testing.T) (repository.JobRepository, dbadapter.DBConnection, tx.TransactionManager) {
	once.Do(func() {
		// Delete existing DB file (relevant for file-based SQLite, not strictly needed for in-memory, but kept for safety).
		os.Remove(sqliteDBFile)

		// Explicitly import GORM's SQLite driver so its init() function is executed.
		// This allows GORM to recognize the SQLite database type and prevents "no dialector registered" errors.
		_ = sqlite_driver.Open("")

		cfg := dbconfig.DatabaseConfig{
			Type: "sqlite",
			// Use an in-memory database for testing.
			Database: ":memory:",
			Pool: dbconfig.PoolConfig{
				MaxOpenConns: 1,
				MaxIdleConns: 1,
			},
		}

		// 1. Establish the DBConnection.
		dialector, err := GetDialectorForTest(cfg)
		assert.NoError(t, err)

		// Configure GORM logger to be silent for tests.
		gormLogger := logger.New(
			log.New(os.Stdout, "\r\n", log.LstdFlags),
			logger.Config{
				SlowThreshold: time.Second,
				LogLevel:      logger.Silent,
				Colorful:      false,
			},
		)

		gormDB, err := gorm.Open(dialector, &gorm.Config{Logger: gormLogger})
		assert.NoError(t, err)

		globalGormDB = gormDB
		globalDBConn = gormadapter.NewGormDBAdapter(gormDB, cfg, "test_sqlite") // Create a GORM DB adapter.

		// Create a test resolver that always returns the globalDBConn.
		testDBResolver = testmock.NewTestSingleConnectionResolver(globalDBConn)

		// Create a TransactionManager using TransactionManagerFactory.
		// NewGormTransactionManagerFactory now accepts a DBConnectionResolver.
		txFactory := gormadapter.NewGormTransactionManagerFactory(testDBResolver)
		globalTxManager = txFactory.NewTransactionManager(globalDBConn)

		// 2. Perform schema migration by manually creating test tables.
		err = createTestTables(globalGormDB)
		assert.NoError(t, err, "Failed to create test tables manually")
	})

	// 3. Create the JobRepository using the shared connection and resolver.
	repo := sqlrepo.NewSQLJobRepository(testDBResolver, globalTxManager, "test_sqlite")

	return repo, globalDBConn, globalTxManager
}

// GetDialectorForTest generates a GORM dialector for testing purposes.
// This function is a test helper and is not part of the main application logic.
func GetDialectorForTest(cfg dbconfig.DatabaseConfig) (gorm.Dialector, error) {
	switch cfg.Type {
	case "postgres":
		connStr, err := GetConnectionStringForTest(cfg)
		if err != nil {
			return nil, err
		}
		return postgres.Open(connStr), nil
	case "mysql":
		connStr, err := GetConnectionStringForTest(cfg)
		if err != nil {
			return nil, err
		}
		return mysql.Open(connStr), nil
	case "sqlite":
		connStr, err := GetConnectionStringForTest(cfg)
		if err != nil {
			return nil, err
		}
		return sqlite_driver.Open(connStr), nil
	default:
		return nil, fmt.Errorf("unsupported database type for dialector: %s", cfg.Type)
	}
}

// teardownSQLiteTestDB should be executed once at the end of the test suite,
// but since it's called with defer in each test, it currently does nothing as an in-memory DB is used.
func teardownSQLiteTestDB(t *testing.T, dbConn dbadapter.DBConnection) {
	// Cleanup is not needed as an in-memory DB is used, which is discarded after the test.
}

// TestSQLiteJobRepository_Lifecycle tests the basic lifecycle of JobRepository using SQLite.
func TestSQLiteJobRepository_Lifecycle(t *testing.T) {
	repo, dbConn, _ := setupSQLiteTestDB(t) // Setup the SQLite test database.
	defer teardownSQLiteTestDB(t, dbConn)   // Keep defer, but it's empty for in-memory DB.

	ctx := context.Background() // Declare variables beforehand to avoid `:=` in subsequent assignments.

	var txAdapter tx.Tx
	var txCtx context.Context
	var err error
	var foundInstance *model.JobInstance
	var foundExecution *model.JobExecution
	var foundStepExecution *model.StepExecution

	// 1. Save and Find JobInstance.
	instance := test_util.NewTestJobInstance("sqliteJob", test_util.NewTestJobParameters(map[string]interface{}{"key": "value"}))
	// Begin a new transaction.
	txAdapter, err = globalTxManager.Begin(ctx)
	assert.NoError(t, err)
	txCtx = context.WithValue(ctx, "tx", txAdapter)

	err = repo.SaveJobInstance(txCtx, instance)
	assert.NoError(t, err)
	err = globalTxManager.Commit(txAdapter)
	assert.NoError(t, err)

	foundInstance, err = repo.FindJobInstanceByID(ctx, instance.ID)
	assert.NoError(t, err)
	assert.NotNil(t, foundInstance)
	assert.Equal(t, instance.JobName, foundInstance.JobName)

	// 2. Save and Update JobExecution.
	execution := test_util.NewTestJobExecution(instance.ID, instance.JobName, instance.Parameters)
	txAdapter, err = globalTxManager.Begin(ctx)
	assert.NoError(t, err)
	txCtx = context.WithValue(ctx, "tx", txAdapter)

	err = repo.SaveJobExecution(txCtx, execution)
	assert.NoError(t, err)
	err = globalTxManager.Commit(txAdapter)
	assert.NoError(t, err)

	execution.MarkAsStarted()
	execution.ExecutionContext.Put("testKey", 123)
	txAdapter, err = globalTxManager.Begin(ctx)
	assert.NoError(t, err)
	txCtx = context.WithValue(ctx, "tx", txAdapter)
	err = repo.UpdateJobExecution(txCtx, execution)
	assert.NoError(t, err)
	err = globalTxManager.Commit(txAdapter)
	assert.NoError(t, err)

	foundExecution, err = repo.FindJobExecutionByID(ctx, execution.ID)
	assert.NoError(t, err)
	assert.NotNil(t, foundExecution)
	val, ok := foundExecution.ExecutionContext.GetInt("testKey")
	assert.True(t, ok)
	assert.Equal(t, 123, val)

	// 3. Save and Update StepExecution.
	stepExecution := test_util.NewTestStepExecution(execution, "step1")
	txAdapter, err = globalTxManager.Begin(ctx)
	assert.NoError(t, err)
	txCtx = context.WithValue(ctx, "tx", txAdapter)

	err = repo.SaveStepExecution(txCtx, stepExecution)
	assert.NoError(t, err)
	err = globalTxManager.Commit(txAdapter)
	assert.NoError(t, err)

	stepExecution.MarkAsStarted()
	stepExecution.ReadCount = 5
	stepExecution.MarkAsCompleted()
	txAdapter, err = globalTxManager.Begin(ctx)
	assert.NoError(t, err)
	txCtx = context.WithValue(ctx, "tx", txAdapter)
	err = repo.UpdateStepExecution(txCtx, stepExecution)
	assert.NoError(t, err)
	err = globalTxManager.Commit(txAdapter)
	assert.NoError(t, err)

	foundStepExecution, err = repo.FindStepExecutionByID(ctx, stepExecution.ID)
	assert.NoError(t, err)
	assert.NotNil(t, foundStepExecution)
	assert.Equal(t, 5, foundStepExecution.ReadCount)

	// 4. Cleanup (records remain in the in-memory DB for the duration of the suite, so delete them).
	// Execute outside the transaction.
	globalGormDB.Exec("DELETE FROM batch_checkpoint_data WHERE step_execution_id = ?", stepExecution.ID)
	globalGormDB.Exec("DELETE FROM batch_step_execution WHERE id = ?", stepExecution.ID)
	globalGormDB.Exec("DELETE FROM batch_job_execution WHERE id = ?", execution.ID)
	globalGormDB.Exec("DELETE FROM batch_job_instance WHERE id = ?", instance.ID)
}

// TestSQLiteJobRepository_OptimisticLocking tests if optimistic locking works with SQLite.
func TestSQLiteJobRepository_OptimisticLocking(t *testing.T) {
	repo, dbConn, _ := setupSQLiteTestDB(t)
	defer teardownSQLiteTestDB(t, dbConn)

	ctx := context.Background()
	instance := test_util.NewTestJobInstance("lockTestJob", test_util.NewTestJobParameters(nil))

	// Save the JobInstance.
	txAdapter, err := globalTxManager.Begin(ctx)
	assert.NoError(t, err)
	txCtx := context.WithValue(ctx, "tx", txAdapter)
	repo.SaveJobInstance(txCtx, instance)
	globalTxManager.Commit(txAdapter)

	// 1. Optimistic Locking Test for JobExecution.
	exec1 := test_util.NewTestJobExecution(instance.ID, instance.JobName, instance.Parameters)

	txAdapter, err = globalTxManager.Begin(ctx)
	assert.NoError(t, err)
	txCtx = context.WithValue(ctx, "tx", txAdapter)
	repo.SaveJobExecution(txCtx, exec1) // Initial save sets version to 0.
	globalTxManager.Commit(txAdapter)   // Commit to persist version 0.

	foundExec, _ := repo.FindJobExecutionByID(ctx, exec1.ID) // Retrieve the execution, which will have version 0.

	// Attempt to update with exec1 (which has version 0) -> Should succeed and increment version.
	exec1.Version = 0
	exec1.MarkAsStarted()
	txAdapter, err = globalTxManager.Begin(ctx)
	assert.NoError(t, err)
	txCtx = context.WithValue(ctx, "tx", txAdapter)
	err = repo.UpdateJobExecution(txCtx, exec1) // Update from version 0 to 1.
	assert.NoError(t, err)
	globalTxManager.Commit(txAdapter)
	assert.Equal(t, 1, exec1.Version)

	// Attempt to update with foundExec (which still has version 0) -> Should fail due to optimistic locking.
	foundExec.MarkAsStarted()
	foundExec.MarkAsCompleted()
	txAdapter, err = globalTxManager.Begin(ctx)
	assert.NoError(t, err)
	txCtx = context.WithValue(ctx, "tx", txAdapter)
	err = repo.UpdateJobExecution(txCtx, foundExec)
	assert.Error(t, err)
	assert.True(t, exception.IsOptimisticLockingFailure(err))
	globalTxManager.Rollback(txAdapter) // Rollback the failed transaction.

	// Cleanup the created records.
	globalGormDB.Exec("DELETE FROM batch_job_execution WHERE id = ?", exec1.ID)
	globalGormDB.Exec("DELETE FROM batch_job_instance WHERE id = ?", instance.ID)
}

// TestSQLiteJobRepository_CheckpointData tests if checkpoint data works with SQLite.
func TestSQLiteJobRepository_CheckpointData(t *testing.T) {
	repo, dbConn, txManager := setupSQLiteTestDB(t)
	defer teardownSQLiteTestDB(t, dbConn)

	ctx := context.Background()
	instance := test_util.NewTestJobInstance("checkpointJob", test_util.NewTestJobParameters(nil))
	exec := test_util.NewTestJobExecution(instance.ID, instance.JobName, instance.Parameters)
	stepExec := test_util.NewTestStepExecution(exec, "checkpointStep")

	// Declare variables beforehand to avoid `:=` in subsequent assignments.
	var txAdapter tx.Tx
	var txCtx context.Context
	var err error
	var foundData *model.CheckpointData
	var offset int
	var ok bool

	// Save StepExecution first, as CheckpointData references it.
	txAdapter, err = txManager.Begin(ctx)
	assert.NoError(t, err)
	txCtx = context.WithValue(ctx, "tx", txAdapter)
	repo.SaveStepExecution(txCtx, stepExec)
	txManager.Commit(txAdapter)

	// 1. Save Checkpoint Data.
	ec := test_util.NewTestExecutionContext(nil)
	ec.Put("offset", 50)
	dataToSave := &model.CheckpointData{StepExecutionID: stepExec.ID, ExecutionContext: ec}

	txAdapter, err = txManager.Begin(ctx)
	assert.NoError(t, err)
	txCtx = context.WithValue(ctx, "tx", txAdapter)
	err = repo.SaveCheckpointData(txCtx, dataToSave)
	assert.NoError(t, err)
	txManager.Commit(txAdapter)

	// 2. Find Checkpoint Data.
	foundData, err = repo.FindCheckpointData(ctx, stepExec.ID)
	assert.NoError(t, err)
	assert.NotNil(t, foundData)
	offset, ok = foundData.ExecutionContext.GetInt("offset")
	assert.True(t, ok)
	assert.Equal(t, 50, offset)

	// 3. Test Update.
	ec.Put("offset", 100)
	dataToSave.ExecutionContext = ec

	txAdapter, err = txManager.Begin(ctx)
	assert.NoError(t, err)
	txCtx = context.WithValue(ctx, "tx", txAdapter)
	err = repo.SaveCheckpointData(txCtx, dataToSave)
	assert.NoError(t, err)
	txManager.Commit(txAdapter)

	foundData, err = repo.FindCheckpointData(ctx, stepExec.ID)
	assert.NoError(t, err)
	offset, ok = foundData.ExecutionContext.GetInt("offset")
	assert.True(t, ok)
	assert.Equal(t, 100, offset)

	// 4. Cleanup the created records.
	globalGormDB.Exec("DELETE FROM batch_checkpoint_data WHERE step_execution_id = ?", stepExec.ID)
	globalGormDB.Exec("DELETE FROM batch_step_execution WHERE id = ?", stepExec.ID)
}

// createTestTables manually creates the tables required for SQLite tests.
// This is necessary to maintain the constraint of not adding GORM tags to schema.go, while eliminating the dependency on AutoMigrate.
func createTestTables(db *gorm.DB) error {
	// Note: For SQLite, JSON types are typically stored as TEXT.
	// Primary keys, NOT NULL constraints, and indexes are defined here.

	// Create batch_job_instance table.
	if err := db.Exec(`
		CREATE TABLE batch_job_instance (
			id VARCHAR(36) PRIMARY KEY,
			job_name VARCHAR(255) NOT NULL,
			parameters TEXT,
			create_time DATETIME NOT NULL,
			version INTEGER NOT NULL,
			parameters_hash VARCHAR(64) NOT NULL
		);
	`).Error; err != nil {
		return err
	}
	// Create a unique index for job_name and parameters_hash.
	if err := db.Exec(`
		CREATE UNIQUE INDEX idx_job_instance_hash ON batch_job_instance (parameters_hash, job_name);
	`).Error; err != nil {
		return err
	}

	// Create batch_job_execution table.
	if err := db.Exec(`
		CREATE TABLE batch_job_execution (
			id VARCHAR(36) PRIMARY KEY,
			job_instance_id VARCHAR(36),
			job_name VARCHAR(255) NOT NULL,
			parameters TEXT,
			start_time DATETIME NOT NULL,
			end_time DATETIME,
			status VARCHAR(20) NOT NULL,
			exit_status VARCHAR(20) NOT NULL,
			exit_code INTEGER,
			failures TEXT,
			version INTEGER NOT NULL,
			create_time DATETIME NOT NULL,
			last_updated DATETIME NOT NULL,
			execution_context TEXT,
			current_step_name VARCHAR(255) NOT NULL DEFAULT '',
			restart_count INTEGER NOT NULL DEFAULT 0,
			FOREIGN KEY(job_instance_id) REFERENCES batch_job_instance(id)
		);
	`).Error; err != nil {
		return err
	}
	if err := db.Exec(`
		CREATE INDEX idx_job_execution_instance_id ON batch_job_execution (job_instance_id);
	`).Error; err != nil {
		return err
	}

	// Create batch_step_execution table.
	if err := db.Exec(`
		CREATE TABLE batch_step_execution (
			id VARCHAR(36) PRIMARY KEY,
			step_name VARCHAR(255) NOT NULL,
			job_execution_id VARCHAR(36),
			start_time DATETIME NOT NULL,
			end_time DATETIME,
			status VARCHAR(20) NOT NULL,
			exit_status VARCHAR(20) NOT NULL,
			failures TEXT,
			read_count INTEGER NOT NULL,
			write_count INTEGER NOT NULL,
			commit_count INTEGER NOT NULL,
			rollback_count INTEGER NOT NULL,
			filter_count INTEGER NOT NULL,
			skip_read_count INTEGER NOT NULL,
			skip_process_count INTEGER NOT NULL,
			skip_write_count INTEGER NOT NULL,
			execution_context TEXT,
			last_updated DATETIME NOT NULL,
			version INTEGER NOT NULL,
			FOREIGN KEY(job_execution_id) REFERENCES batch_job_execution(id)
		);
	`).Error; err != nil {
		return err
	}
	if err := db.Exec(`
		CREATE INDEX idx_step_execution_job_execution_id ON batch_step_execution (job_execution_id);
	`).Error; err != nil {
		return err
	}

	// Create batch_checkpoint_data table.
	if err := db.Exec(`
		CREATE TABLE batch_checkpoint_data (
			step_execution_id VARCHAR(36) PRIMARY KEY,
			execution_context TEXT NOT NULL,
			last_updated DATETIME NOT NULL,
			FOREIGN KEY(step_execution_id) REFERENCES batch_step_execution(id)
		);
	`).Error; err != nil {
		return err
	}

	return nil
}
