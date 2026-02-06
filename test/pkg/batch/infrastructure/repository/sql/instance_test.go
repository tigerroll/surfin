// Package sql_test provides unit tests for the SQL repository implementations,
// specifically for JobInstance operations.
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
	job "github.com/tigerroll/surfin/pkg/batch/core/domain/repository"
	sqlrepo "github.com/tigerroll/surfin/pkg/batch/infrastructure/repository/sql"
	"github.com/tigerroll/surfin/pkg/batch/support/util/exception"
	mocktx "github.com/tigerroll/surfin/pkg/batch/test"
	testutil "github.com/tigerroll/surfin/pkg/batch/test"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"testing"
)

// setupGormInstanceMock is a helper function to set up the GORM mock environment for JobInstance repository tests.
func setupGormInstanceMock(t *testing.T) (*gorm.DB, sqlmock.Sqlmock, dbadapter.DBConnection, job.JobRepository) {
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

func TestGORMJobRepository_SaveJobInstance(t *testing.T) {
	gormDB, mock, _, repo := setupGormInstanceMock(t)
	defer func() {
		mock.ExpectClose()
		sqlDB, _ := gormDB.DB()
		sqlDB.Close()
	}()

	ctx := context.Background()
	jobInstance := model.NewJobInstance("testJob", model.NewJobParameters())

	// Mock the transaction using MockTxManager.
	mockTx := new(mocktx.MockTx)
	// The query argument can be nil or an empty map, so use Anything.
	mockTx.On("ExecuteUpdate", testify_mock.Anything, testify_mock.Anything, "CREATE", "batch_job_instance", testify_mock.Anything).Return(int64(1), nil)

	// Create a context with the mocked transaction.
	txCtx := context.WithValue(ctx, "tx", mockTx)

	err := repo.SaveJobInstance(txCtx, jobInstance)
	assert.NoError(t, err)

	// The TxManager's Begin/Commit/Rollback methods are not called directly because the Tx is already in the context.
	mockTx.AssertExpectations(t)
}

func TestGORMJobRepository_UpdateJobInstance(t *testing.T) {
	gormDB, mock, _, repo := setupGormInstanceMock(t)
	defer func() {
		mock.ExpectClose()
		sqlDB, _ := gormDB.DB()
		sqlDB.Close()
	}()

	ctx := context.Background()
	jobInstance := model.NewJobInstance("testJob", model.NewJobParameters())
	jobInstance.ID = model.NewID()
	jobInstance.Version = 0

	// Mock the transaction using MockTxManager.
	mockTx := new(mocktx.MockTx)
	expectedQuery := map[string]interface{}{"version": 0}
	mockTx.On("ExecuteUpdate", testify_mock.Anything, testify_mock.Anything, "UPDATE", "batch_job_instance", expectedQuery).Return(int64(1), nil)

	// Create a context with the mocked transaction.
	txCtx := context.WithValue(ctx, "tx", mockTx)

	err := repo.UpdateJobInstance(txCtx, jobInstance)
	assert.NoError(t, err)
	assert.Equal(t, 1, jobInstance.Version) // Verify that the version is incremented.

	// The TxManager's Begin/Commit/Rollback methods are not called directly because the Tx is already in the context.
	mockTx.AssertExpectations(t)
}

func TestGORMJobRepository_UpdateJobInstance_OptimisticLocking(t *testing.T) {
	gormDB, mock, _, repo := setupGormInstanceMock(t)
	defer func() {
		mock.ExpectClose()
		sqlDB, _ := gormDB.DB()
		sqlDB.Close()
	}()

	ctx := context.Background()
	jobInstance := model.NewJobInstance("testJob", model.NewJobParameters())
	jobInstance.ID = model.NewID()
	jobInstance.Version = 0

	// Mock the transaction using MockTxManager.
	mockTx := new(mocktx.MockTx)
	expectedQuery := map[string]interface{}{"version": 0}
	mockTx.On("ExecuteUpdate", testify_mock.Anything, testify_mock.Anything, "UPDATE", "batch_job_instance", expectedQuery).Return(int64(0), nil)

	// Create a context with the mocked transaction.
	txCtx := context.WithValue(ctx, "tx", mockTx)

	err := repo.UpdateJobInstance(txCtx, jobInstance)
	assert.Error(t, err)
	assert.True(t, exception.IsOptimisticLockingFailure(err))
	assert.Equal(t, 0, jobInstance.Version) // Verify that the version is not incremented (rolled back).

	// The TxManager's Begin/Commit/Rollback methods are not called directly because the Tx is already in the context.
	mockTx.AssertExpectations(t)
}
