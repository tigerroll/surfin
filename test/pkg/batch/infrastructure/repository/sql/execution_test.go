package sql_test

import (
	"context"
	"testing"
	"time"

	dbconfig "github.com/tigerroll/surfin/pkg/batch/adapter/database/config"
	gormadapter "github.com/tigerroll/surfin/pkg/batch/adapter/database/gorm"
	adapter "github.com/tigerroll/surfin/pkg/batch/core/adapter"                  // Alias for clarity
	model "github.com/tigerroll/surfin/pkg/batch/core/domain/model"               // Add this import
	repository "github.com/tigerroll/surfin/pkg/batch/core/domain/repository"     // Alias for clarity
	sqlrepo "github.com/tigerroll/surfin/pkg/batch/infrastructure/repository/sql" // Alias for clarity
	"github.com/tigerroll/surfin/pkg/batch/support/util/exception"
	mocktx "github.com/tigerroll/surfin/pkg/batch/test"   // Alias for clarity
	testutil "github.com/tigerroll/surfin/pkg/batch/test" // Alias for clarity

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	testify_mock "github.com/stretchr/testify/mock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// setupGormJobMock is a helper for JobExecution repository tests.
func setupGormJobMock(t *testing.T) (*gorm.DB, sqlmock.Sqlmock, adapter.DBConnection, repository.JobRepository) {
	sqlDB, mock, err := sqlmock.New()
	assert.NoError(t, err)

	// Use mysql.New for GORM initialization (instead of MockDialector)
	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      sqlDB,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})
	assert.NoError(t, err)

	cfg := dbconfig.DatabaseConfig{Type: "mock_db"}                // Use correct DatabaseConfig
	dbConn := gormadapter.NewGormDBAdapter(gormDB, cfg, "mock_db") // Use gormadapter

	txManager := &mocktx.MockTxManager{}

	// Adapt to NewGORMJobRepository signature change
	mockResolver := testutil.NewTestSingleConnectionResolver(dbConn)
	repo := sqlrepo.NewSQLJobRepository(mockResolver, txManager, "mock_db") // Use sqlrepo.NewSQLJobRepository

	return gormDB, mock, dbConn, repo
}

func TestGORMJobRepository_SaveJobExecution(t *testing.T) {
	gormDB, mock, _, repo := setupGormJobMock(t)
	defer func() {
		mock.ExpectClose()
		sqlDB, _ := gormDB.DB()
		sqlDB.Close()
	}()

	ctx := context.Background()
	jobExecution := model.NewJobExecution(model.NewID(), "testJob", model.NewJobParameters())

	// Mock transaction using MockTxManager
	mockTx := new(mocktx.MockTx)

	// Expect ExecuteUpdate("CREATE") call (query argument can be nil or empty map, so use Anything)
	mockTx.On("ExecuteUpdate", testify_mock.Anything, testify_mock.Anything, "CREATE", "batch_job_execution", testify_mock.Anything).Return(int64(1), nil)

	// Create transaction context (so JobRepository's getTxExecutor can detect the Tx)
	txCtx := context.WithValue(ctx, "tx", mockTx)

	err := repo.SaveJobExecution(txCtx, jobExecution)
	assert.NoError(t, err)
	mockTx.AssertExpectations(t) // Verify mock expectations
}

func TestGORMJobRepository_UpdateJobExecution(t *testing.T) {
	gormDB, mock, _, repo := setupGormJobMock(t)
	defer func() {
		mock.ExpectClose()
		sqlDB, _ := gormDB.DB()
		sqlDB.Close()
	}()

	ctx := context.Background()
	jobExecution := model.NewJobExecution(model.NewID(), "testJob", model.NewJobParameters())
	jobExecution.ID = model.NewID() // Set ID for update
	jobExecution.Version = 5        // Set version for optimistic locking test
	jobExecution.MarkAsStarted()

	// Mock transaction using MockTxManager
	mockTx := new(mocktx.MockTx)
	expectedQuery := map[string]interface{}{"version": 5}
	mockTx.On("ExecuteUpdate", testify_mock.Anything, testify_mock.Anything, "UPDATE", "batch_job_execution", expectedQuery).Return(int64(1), nil)

	// Create transaction context
	txCtx := context.WithValue(ctx, "tx", mockTx)

	err := repo.UpdateJobExecution(txCtx, jobExecution)
	assert.NoError(t, err)
	assert.Equal(t, 6, jobExecution.Version) // Verify version is incremented
	mockTx.AssertExpectations(t)
}

func TestGORMJobRepository_UpdateJobExecution_OptimisticLocking(t *testing.T) {
	gormDB, mock, _, repo := setupGormJobMock(t)
	defer func() {
		mock.ExpectClose()
		sqlDB, _ := gormDB.DB()
		sqlDB.Close()
	}()

	ctx := context.Background()
	jobExecution := model.NewJobExecution(model.NewID(), "testJob", model.NewJobParameters())
	jobExecution.ID = model.NewID() // Set ID for update
	jobExecution.Version = 5        // Set version for optimistic locking test
	jobExecution.MarkAsStarted()

	// Mock transaction using MockTxManager
	mockTx := new(mocktx.MockTx)
	expectedQuery := map[string]interface{}{"version": 5}
	mockTx.On("ExecuteUpdate", testify_mock.Anything, testify_mock.Anything, "UPDATE", "batch_job_execution", expectedQuery).Return(int64(0), nil) // Return 0 rows affected

	// Create transaction context
	txCtx := context.WithValue(ctx, "tx", mockTx)

	err := repo.UpdateJobExecution(txCtx, jobExecution)
	assert.Error(t, err)
	assert.True(t, exception.IsOptimisticLockingFailure(err))
	assert.Equal(t, 5, jobExecution.Version) // Verify version is rolled back
	mockTx.AssertExpectations(t)
}

func TestGORMJobRepository_FindJobExecutionByID(t *testing.T) {
	gormDB, mock, _, repo := setupGormJobMock(t)
	defer func() {
		mock.ExpectClose()
		sqlDB, _ := gormDB.DB()
		sqlDB.Close()
	}()

	ctx := context.Background()
	executionID := "exec-1"
	jobInstanceID := "instance-1"
	jobName := "testJob"

	// GORM First expects a SELECT statement (internal operation of ExecuteQueryAdvanced)
	execRows := sqlmock.NewRows([]string{"id", "job_instance_id", "job_name", "status", "exit_status", "version", "start_time", "create_time", "last_updated", "execution_context", "parameters", "failures"}).
		AddRow(executionID, jobInstanceID, jobName, string(model.BatchStatusStarted), string(model.ExitStatusUnknown), 1, time.Now(), time.Now(), time.Now(), "{}", "{}", "[]")

	// StepExecution query (via FindStepExecutionsByJobExecutionID)
	// Note: Added `end_time` and `failures` columns, and adjusted `version` position.
	stepRows := sqlmock.NewRows([]string{"id", "step_name", "job_execution_id", "start_time", "end_time", "status", "exit_status", "failures", "read_count", "write_count", "commit_count", "rollback_count", "filter_count", "skip_read_count", "skip_process_count", "skip_write_count", "execution_context", "last_updated", "version"}).
		// Note: Added values for `end_time` and `failures`, and moved `version` to the end.
		AddRow("step-1", "stepA", executionID, time.Now(), time.Now(), string(model.BatchStatusCompleted), string(model.ExitStatusCompleted), "[]", 10, 10, 1, 0, 0, 0, 0, 0, "{}", time.Now(), 0)

	// 1. JobExecution query (via ExecuteQueryAdvanced)
	// FindJobExecutionByID uses Order="" Limit=1.
	mock.ExpectQuery("SELECT (.+) FROM `batch_job_execution` WHERE `batch_job_execution`.`id` = \\? LIMIT \\?").
		WithArgs(executionID, 1).WillReturnRows(execRows)

	// 2. StepExecution loading (via FindStepExecutionsByJobExecutionID)
	// FindStepExecutionsByJobExecutionID uses Order="start_time asc" Limit=0.
	mock.ExpectQuery("SELECT (.+) FROM `batch_step_execution` WHERE `batch_step_execution`.`job_execution_id` = \\? ORDER BY start_time asc").
		WithArgs(executionID).
		WillReturnRows(stepRows)

	foundExecution, err := repo.FindJobExecutionByID(ctx, executionID)
	assert.NoError(t, err)
	assert.NotNil(t, foundExecution)
	assert.Equal(t, executionID, foundExecution.ID)
	assert.Len(t, foundExecution.StepExecutions, 1)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}
