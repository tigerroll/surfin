package sql_test

import (
	"context"
	"log"
	"os"
	"sync"
	"testing"
	"time"

	dbconfig "github.com/tigerroll/surfin/pkg/batch/adapter/database/config"      // Add this import
	sqlrepo "github.com/tigerroll/surfin/pkg/batch/infrastructure/repository/sql" // Add this import

	// Explicitly import GORM's SQLite driver so its init() function is executed.
	// This allows GORM to recognize the SQLite database type.
	sqlite_driver "gorm.io/driver/sqlite"

	// Import the Custom SQLite GORM provider so its init() function is executed.
	// This calls gormadapter.RegisterDialector to register the "sqlite" dialector factory.
	_ "github.com/tigerroll/surfin/pkg/batch/adapter/database/gorm/sqlite"

	gormadapter "github.com/tigerroll/surfin/pkg/batch/adapter/database/gorm"
	adapter "github.com/tigerroll/surfin/pkg/batch/core/adapter"
	port "github.com/tigerroll/surfin/pkg/batch/core/application/port"
	model "github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	"github.com/tigerroll/surfin/pkg/batch/core/domain/repository"
	tx "github.com/tigerroll/surfin/pkg/batch/core/tx"
	"github.com/tigerroll/surfin/pkg/batch/support/util/exception"
	test_util "github.com/tigerroll/surfin/pkg/batch/test"
	testmock "github.com/tigerroll/surfin/pkg/batch/test"

	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const sqliteDBFile = "test_metadata_sqlite.db"

var (
	globalGormDB    *gorm.DB
	globalDBConn    adapter.DBConnection // The single database connection to be returned.
	globalTxManager tx.TransactionManager
	once            sync.Once
	testDBResolver  port.DBConnectionResolver // The test-specific DB connection resolver.
)

// setupSQLiteTestDB sets up the SQLite test environment.
// It shares a single in-memory DB connection across the entire test suite.
func setupSQLiteTestDB(t *testing.T) (repository.JobRepository, adapter.DBConnection, tx.TransactionManager) {
	once.Do(func() {
		// Delete existing DB file (only for file-based, not needed for in-memory, but kept for safety)
		os.Remove(sqliteDBFile)

		// To ensure GORM's SQLite driver is loaded and its init() function runs, perform a dummy Open call. This prevents "no dialector registered" errors.
		_ = sqlite_driver.Open("")

		cfg := dbconfig.DatabaseConfig{ // Use correct DatabaseConfig
			Type: "sqlite", // Use dbconfig.DatabaseConfig
			// Use in-memory DB
			Database: ":memory:",
			Pool: dbconfig.PoolConfig{
				MaxOpenConns: 1,
				MaxIdleConns: 1,
			},
		}

		// 1. Establish DBConnection
		dialector, err := gormadapter.GetDialectorForTest(cfg) // Use gormadapter
		assert.NoError(t, err)

		// Set GORM Logger to Silent
		gormLogger := logger.New(
			log.New(os.Stdout, "\r\n", log.LstdFlags),
			logger.Config{
				SlowThreshold: time.Second, // Set GORM Logger to Silent
				LogLevel:      logger.Silent,
				Colorful:      false, // Explicitly set to Silent
			},
		)

		gormDB, err := gorm.Open(dialector, &gorm.Config{Logger: gormLogger})
		assert.NoError(t, err)

		globalGormDB = gormDB                                                   // Adjust to NewGormDBAdapter signature
		globalDBConn = gormadapter.NewGormDBAdapter(gormDB, cfg, "test_sqlite") // Use gormadapter

		// Create Test Resolver: Always returns globalDBConn
		testDBResolver = testmock.NewTestSingleConnectionResolver(globalDBConn)

		// Create TxManager using TransactionManagerFactory
		// NewGormTransactionManagerFactory was changed to accept DBConnectionResolver, so pass the Resolver here
		txFactory := gormadapter.NewGormTransactionManagerFactory(testDBResolver) // Use gormadapter
		globalTxManager = txFactory.NewTransactionManager(globalDBConn)

		// 2. Schema Migration (manual SQL execution)
		err = createTestTables(globalGormDB)
		assert.NoError(t, err, "Failed to create test tables manually")
	})

	// 3. Create JobRepository (using shared connection)
	// JobRepository was changed to accept DBResolver
	repo := sqlrepo.NewSQLJobRepository(testDBResolver, globalTxManager, "test_sqlite") // Use sqlrepo.NewSQLJobRepository

	return repo, globalDBConn, globalTxManager
}

// teardownSQLiteTestDB should be executed once at the end of the test suite,
// but since it's called with defer in each test, it does nothing here.
func teardownSQLiteTestDB(t *testing.T, dbConn adapter.DBConnection) {
	// Cleanup is not needed as in-memory DB is used
}

// TestSQLiteJobRepository_Lifecycle tests the basic lifecycle of JobRepository using SQLite.
func TestSQLiteJobRepository_Lifecycle(t *testing.T) {
	repo, dbConn, _ := setupSQLiteTestDB(t)
	defer teardownSQLiteTestDB(t, dbConn) // Keep defer, but it's empty

	ctx := context.Background() // Declare variables beforehand and limit := to initial declaration

	var txAdapter tx.Tx
	var txCtx context.Context
	var err error
	var foundInstance *model.JobInstance
	var foundExecution *model.JobExecution
	var foundStepExecution *model.StepExecution

	// 1. Save and Find JobInstance
	instance := test_util.NewTestJobInstance("sqliteJob", test_util.NewTestJobParameters(map[string]interface{}{"key": "value"}))
	// Begin transaction
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

	// 2. Save and Update JobExecution
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

	// 3. Save and Update StepExecution
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

	// 4. Cleanup (usually not needed for in-memory DB, but records remain, so delete)
	// Execute outside transaction // Execute outside transaction
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

	// Save JobInstance
	txAdapter, err := globalTxManager.Begin(ctx)
	assert.NoError(t, err)
	txCtx := context.WithValue(ctx, "tx", txAdapter)
	repo.SaveJobInstance(txCtx, instance)
	globalTxManager.Commit(txAdapter)

	// 1. Optimistic Locking Test for JobExecution
	exec1 := test_util.NewTestJobExecution(instance.ID, instance.JobName, instance.Parameters)

	txAdapter, err = globalTxManager.Begin(ctx)
	assert.NoError(t, err)
	txCtx = context.WithValue(ctx, "tx", txAdapter)
	repo.SaveJobExecution(txCtx, exec1) // Version 0
	globalTxManager.Commit(txAdapter)   // Get Version 0

	foundExec, _ := repo.FindJobExecutionByID(ctx, exec1.ID) // Get Version 0

	// Attempt to update with exec1 (Version 0) -> Success
	exec1.Version = 0
	exec1.MarkAsStarted()
	txAdapter, err = globalTxManager.Begin(ctx)
	assert.NoError(t, err)
	txCtx = context.WithValue(ctx, "tx", txAdapter)
	err = repo.UpdateJobExecution(txCtx, exec1) // Version 0 -> 1
	assert.NoError(t, err)
	globalTxManager.Commit(txAdapter)
	assert.Equal(t, 1, exec1.Version)

	// Attempt to update with foundExec (Version 0) -> Should fail
	foundExec.MarkAsStarted()
	foundExec.MarkAsCompleted()
	txAdapter, err = globalTxManager.Begin(ctx)
	assert.NoError(t, err)
	txCtx = context.WithValue(ctx, "tx", txAdapter)
	err = repo.UpdateJobExecution(txCtx, foundExec)
	assert.Error(t, err)
	assert.True(t, exception.IsOptimisticLockingFailure(err))
	globalTxManager.Rollback(txAdapter) // Rollback

	// Cleanup
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

	// Declare variables beforehand
	var txAdapter tx.Tx
	var txCtx context.Context
	var err error
	var foundData *model.CheckpointData
	var offset int
	var ok bool

	// Save StepExecution first
	txAdapter, err = txManager.Begin(ctx)
	assert.NoError(t, err)
	txCtx = context.WithValue(ctx, "tx", txAdapter)
	repo.SaveStepExecution(txCtx, stepExec)
	txManager.Commit(txAdapter)

	// 1. Save Checkpoint Data
	ec := test_util.NewTestExecutionContext(nil)
	ec.Put("offset", 50)
	dataToSave := &model.CheckpointData{StepExecutionID: stepExec.ID, ExecutionContext: ec}

	txAdapter, err = txManager.Begin(ctx)
	assert.NoError(t, err)
	txCtx = context.WithValue(ctx, "tx", txAdapter)
	err = repo.SaveCheckpointData(txCtx, dataToSave)
	assert.NoError(t, err)
	txManager.Commit(txAdapter)

	// 2. Find Checkpoint Data
	foundData, err = repo.FindCheckpointData(ctx, stepExec.ID)
	assert.NoError(t, err)
	assert.NotNil(t, foundData)
	offset, ok = foundData.ExecutionContext.GetInt("offset")
	assert.True(t, ok)
	assert.Equal(t, 50, offset)

	// 3. Test Update
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

	// 4. Cleanup
	globalGormDB.Exec("DELETE FROM batch_checkpoint_data WHERE step_execution_id = ?", stepExec.ID)
	globalGormDB.Exec("DELETE FROM batch_step_execution WHERE id = ?", stepExec.ID)
}

// createTestTables manually creates the tables required for SQLite tests.
// This is necessary to maintain the constraint of not adding GORM tags to schema.go, while eliminating the dependency on AutoMigrate.
func createTestTables(db *gorm.DB) error {
	// NOTE: For SQLite, JSON types are treated as TEXT.
	// Define primary keys, NOT NULL constraints, and indexes.

	// batch_job_instance
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
	// unique index for job_name and parameters_hash
	if err := db.Exec(`
		CREATE UNIQUE INDEX idx_job_instance_hash ON batch_job_instance (parameters_hash, job_name);
	`).Error; err != nil {
		return err
	}

	// batch_job_execution
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

	// batch_step_execution
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

	// batch_checkpoint_data
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
