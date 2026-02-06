// Package sql_test provides unit tests for the SQL repository implementations,
// specifically for StepExecution operations.
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
)

// setupGormStepMock is a helper function to set up the GORM mock environment for StepExecution repository tests.
func setupGormStepMock(t *testing.T) (*gorm.DB, sqlmock.Sqlmock, dbadapter.DBConnection, repository.JobRepository) {
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

func TestGORMJobRepository_SaveStepExecution(t *testing.T) {
	gormDB, mock, _, repo := setupGormStepMock(t)
	defer func() {
		mock.ExpectClose()
		sqlDB, _ := gormDB.DB()
		sqlDB.Close()
	}()

	ctx := context.Background()
	jobExecution := model.NewJobExecution(model.NewID(), "testJob", model.NewJobParameters())
	stepExecution := model.NewStepExecution(model.NewID(), jobExecution, "testStep")

	// Mock the transaction using MockTxManager.
	mockTx := new(mocktx.MockTx)
	mockTx.On("ExecuteUpdate", testify_mock.Anything, testify_mock.Anything, "CREATE", "batch_step_execution", testify_mock.Anything).Return(int64(1), nil)

	// Create a context with the mocked transaction.
	txCtx := context.WithValue(ctx, "tx", mockTx)

	err := repo.SaveStepExecution(txCtx, stepExecution)
	assert.NoError(t, err)

	// The TxManager's Begin/Commit/Rollback methods are not called directly because the Tx is already in the context.
	mockTx.AssertExpectations(t)
}

func TestGORMJobRepository_UpdateStepExecution(t *testing.T) {
	gormDB, mock, _, repo := setupGormStepMock(t)
	defer func() {
		mock.ExpectClose()
		sqlDB, _ := gormDB.DB()
		sqlDB.Close()
	}()

	ctx := context.Background()
	jobExecution := model.NewJobExecution(model.NewID(), "testJob", model.NewJobParameters())
	stepExecution := model.NewStepExecution(model.NewID(), jobExecution, "testStep")
	stepExecution.ID = model.NewID()
	stepExecution.Version = 0
	stepExecution.MarkAsStarted()

	// Mock the transaction using MockTxManager.
	mockTx := new(mocktx.MockTx)
	expectedQuery := map[string]interface{}{"version": 0}
	mockTx.On("ExecuteUpdate", testify_mock.Anything, testify_mock.Anything, "UPDATE", "batch_step_execution", expectedQuery).Return(int64(1), nil)

	// Create a context with the mocked transaction.
	txCtx := context.WithValue(ctx, "tx", mockTx)

	err := repo.UpdateStepExecution(txCtx, stepExecution)
	assert.NoError(t, err)
	assert.Equal(t, 1, stepExecution.Version) // Verify that the version is incremented.

	// The TxManager's Begin/Commit/Rollback methods are not called directly because the Tx is already in the context.
	mockTx.AssertExpectations(t)
}

func TestGORMJobRepository_UpdateStepExecution_OptimisticLocking(t *testing.T) {
	gormDB, mock, _, repo := setupGormStepMock(t)
	defer func() {
		mock.ExpectClose()
		sqlDB, _ := gormDB.DB()
		sqlDB.Close()
	}()

	ctx := context.Background()
	jobExecution := model.NewJobExecution(model.NewID(), "testJob", model.NewJobParameters())
	stepExecution := model.NewStepExecution(model.NewID(), jobExecution, "testStep")
	stepExecution.ID = model.NewID()
	stepExecution.Version = 0
	stepExecution.MarkAsStarted()

	// Mock the transaction using MockTxManager.
	mockTx := new(mocktx.MockTx)
	expectedQuery := map[string]interface{}{"version": 0}
	mockTx.On("ExecuteUpdate", testify_mock.Anything, testify_mock.Anything, "UPDATE", "batch_step_execution", expectedQuery).Return(int64(0), nil)

	// Create a context with the mocked transaction.
	txCtx := context.WithValue(ctx, "tx", mockTx)

	err := repo.UpdateStepExecution(txCtx, stepExecution)
	assert.Error(t, err)
	assert.True(t, exception.IsOptimisticLockingFailure(err))
	assert.Equal(t, 0, stepExecution.Version) // Verify that the version is not incremented (rolled back).

	// The TxManager's Begin/Commit/Rollback methods are not called directly because the Tx is already in the context.
	mockTx.AssertExpectations(t)
}
