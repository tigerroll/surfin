// Package sql_test provides unit tests for the SQL repository implementations,
// specifically for JobExecution operations.
package sql_test

import (
	"context"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	testify_mock "github.com/stretchr/testify/mock"
	dbadapter "github.com/tigerroll/surfin/pkg/batch/adapter/database"
	dbconfig "github.com/tigerroll/surfin/pkg/batch/adapter/database/config"
	gormadapter "github.com/tigerroll/surfin/pkg/batch/adapter/database/gorm"
	model "github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	repository "github.com/tigerroll/surfin/pkg/batch/core/domain/repository"
	sqlrepo "github.com/tigerroll/surfin/pkg/batch/infrastructure/repository/sql"
	"github.com/tigerroll/surfin/pkg/batch/support/util/exception"
	mocktx "github.com/tigerroll/surfin/pkg/batch/test"
	testutil "github.com/tigerroll/surfin/pkg/batch/test"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"testing"
	"time"
)

// setupGormJobMock is a helper function to set up the GORM mock environment for JobExecution repository tests.
func setupGormJobMock(t *testing.T) (*gorm.DB, sqlmock.Sqlmock, dbadapter.DBConnection, repository.JobRepository) {
	sqlDB, mock, err := sqlmock.New()
	assert.NoError(t, err)

	// Use mysql.New for GORM initialization, providing the mocked SQL DB.
	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      sqlDB,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})
	assert.NoError(t, err)

	cfg := dbconfig.DatabaseConfig{Type: "mock_db"}                // Define a mock database configuration.
	dbConn := gormadapter.NewGormDBAdapter(gormDB, cfg, "mock_db") // Create a GORM DB adapter with the mock DB.

	txManager := &mocktx.MockTxManager{}

	// Create a single connection resolver for testing.
	mockResolver := testutil.NewTestSingleConnectionResolver(dbConn)
	repo := sqlrepo.NewSQLJobRepository(mockResolver, txManager, "mock_db") // Initialize the SQL job repository.

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

	// Mock the transaction using MockTxManager.
	mockTx := new(mocktx.MockTx)

	// Expect an ExecuteUpdate("CREATE") call. The query argument can be nil or an empty map, so use Anything.
	mockTx.On("ExecuteUpdate", testify_mock.Anything, testify_mock.Anything, "CREATE", "batch_job_execution", testify_mock.Anything).Return(int64(1), nil)

	// Create a context with the mocked transaction, allowing JobRepository's getTxExecutor to detect it.
	txCtx := context.WithValue(ctx, "tx", mockTx)

	err := repo.SaveJobExecution(txCtx, jobExecution)
	assert.NoError(t, err)
	mockTx.AssertExpectations(t) // Verify that all mock expectations were met.
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
	jobExecution.ID = model.NewID() // Set an ID for the update operation.
	jobExecution.Version = 5        // Set the version for optimistic locking testing.
	jobExecution.MarkAsStarted()

	// Mock the transaction using MockTxManager.
	mockTx := new(mocktx.MockTx)
	expectedQuery := map[string]interface{}{"version": 5}
	mockTx.On("ExecuteUpdate", testify_mock.Anything, testify_mock.Anything, "UPDATE", "batch_job_execution", expectedQuery).Return(int64(1), nil)

	// Create a context with the mocked transaction.
	txCtx := context.WithValue(ctx, "tx", mockTx)

	err := repo.UpdateJobExecution(txCtx, jobExecution)
	assert.NoError(t, err)
	assert.Equal(t, 6, jobExecution.Version) // Verify that the version is incremented.
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
	jobExecution.ID = model.NewID() // Set an ID for the update operation.
	jobExecution.Version = 5        // Set the version for optimistic locking testing.
	jobExecution.MarkAsStarted()

	// Mock the transaction using MockTxManager.
	mockTx := new(mocktx.MockTx)
	expectedQuery := map[string]interface{}{"version": 5}
	mockTx.On("ExecuteUpdate", testify_mock.Anything, testify_mock.Anything, "UPDATE", "batch_job_execution", expectedQuery).Return(int64(0), nil) // Simulate 0 rows affected for optimistic locking failure.

	// Create a context with the mocked transaction.
	txCtx := context.WithValue(ctx, "tx", mockTx)

	err := repo.UpdateJobExecution(txCtx, jobExecution)
	assert.Error(t, err)
	assert.True(t, exception.IsOptimisticLockingFailure(err))
	assert.Equal(t, 5, jobExecution.Version) // Verify that the version is not incremented (rolled back).
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

	// GORM's First method internally uses ExecuteQueryAdvanced, expecting a SELECT statement.
	execRows := sqlmock.NewRows([]string{"id", "job_instance_id", "job_name", "status", "exit_status", "version", "start_time", "create_time", "last_updated", "execution_context", "parameters", "failures"}).
		AddRow(executionID, jobInstanceID, jobName, string(model.BatchStatusStarted), string(model.ExitStatusUnknown), 1, time.Now(), time.Now(), time.Now(), "{}", "{}", "[]")

	// Mock rows for StepExecution query (via FindStepExecutionsByJobExecutionID).
	// Note: `end_time` and `failures` columns are added, and `version` position is adjusted.
	stepRows := sqlmock.NewRows([]string{"id", "step_name", "job_execution_id", "start_time", "end_time", "status", "exit_status", "failures", "read_count", "write_count", "commit_count", "rollback_count", "filter_count", "skip_read_count", "skip_process_count", "skip_write_count", "execution_context", "last_updated", "version"}).
		// Values for `end_time` and `failures` are added, and `version` is moved to the end.
		AddRow("step-1", "stepA", executionID, time.Now(), time.Now(), string(model.BatchStatusCompleted), string(model.ExitStatusCompleted), "[]", 10, 10, 1, 0, 0, 0, 0, 0, "{}", time.Now(), 0)

	// 1. Mock the JobExecution query (via ExecuteQueryAdvanced).
	// FindJobExecutionByID uses an empty Order and Limit=1.
	mock.ExpectQuery("SELECT (.+) FROM `batch_job_execution` WHERE `batch_job_execution`.`id` = \\? LIMIT \\?").
		WithArgs(executionID, 1).WillReturnRows(execRows)

	// 2. Mock the StepExecution loading (via FindStepExecutionsByJobExecutionID).
	// FindStepExecutionsByJobExecutionID uses Order="start_time asc" and no Limit.
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
