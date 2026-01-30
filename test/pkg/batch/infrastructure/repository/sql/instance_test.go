package sql_test

import (
	"context"
	"testing"

	dbconfig "github.com/tigerroll/surfin/pkg/batch/adapter/database/config"
	gormadapter "github.com/tigerroll/surfin/pkg/batch/adapter/database/gorm"
	adapter "github.com/tigerroll/surfin/pkg/batch/core/adapter"
	model "github.com/tigerroll/surfin/pkg/batch/core/domain/model" // Add this import
	job "github.com/tigerroll/surfin/pkg/batch/core/domain/repository"
	sqlrepo "github.com/tigerroll/surfin/pkg/batch/infrastructure/repository/sql" // Add this import
	"github.com/tigerroll/surfin/pkg/batch/support/util/exception"
	mocktx "github.com/tigerroll/surfin/pkg/batch/test"   // Alias for clarity
	testutil "github.com/tigerroll/surfin/pkg/batch/test" // Alias for clarity

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	testify_mock "github.com/stretchr/testify/mock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// setupGormInstanceMock is a helper for JobInstance repository tests.
func setupGormInstanceMock(t *testing.T) (*gorm.DB, sqlmock.Sqlmock, adapter.DBConnection, job.JobRepository) {
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

func TestGORMJobRepository_SaveJobInstance(t *testing.T) {
	gormDB, mock, _, repo := setupGormInstanceMock(t)
	defer func() {
		mock.ExpectClose()
		sqlDB, _ := gormDB.DB()
		sqlDB.Close()
	}()

	ctx := context.Background()
	jobInstance := model.NewJobInstance("testJob", model.NewJobParameters())

	// Mock transaction using MockTxManager
	mockTx := new(mocktx.MockTx)
	// Using Anything as query argument can be nil or empty map
	mockTx.On("ExecuteUpdate", testify_mock.Anything, testify_mock.Anything, "CREATE", "batch_job_instance", testify_mock.Anything).Return(int64(1), nil)

	// Create transaction context
	txCtx := context.WithValue(ctx, "tx", mockTx)

	err := repo.SaveJobInstance(txCtx, jobInstance)
	assert.NoError(t, err)

	// TxManager is not called because Tx is in context
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

	// Mock transaction using MockTxManager
	mockTx := new(mocktx.MockTx)
	expectedQuery := map[string]interface{}{"version": 0}
	mockTx.On("ExecuteUpdate", testify_mock.Anything, testify_mock.Anything, "UPDATE", "batch_job_instance", expectedQuery).Return(int64(1), nil)

	// Create transaction context
	txCtx := context.WithValue(ctx, "tx", mockTx)

	err := repo.UpdateJobInstance(txCtx, jobInstance)
	assert.NoError(t, err)
	assert.Equal(t, 1, jobInstance.Version) // Verify version is incremented

	// TxManager is not called because Tx is in context
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

	// Mock transaction using MockTxManager
	mockTx := new(mocktx.MockTx)
	expectedQuery := map[string]interface{}{"version": 0}
	mockTx.On("ExecuteUpdate", testify_mock.Anything, testify_mock.Anything, "UPDATE", "batch_job_instance", expectedQuery).Return(int64(0), nil)

	// Create transaction context
	txCtx := context.WithValue(ctx, "tx", mockTx)

	err := repo.UpdateJobInstance(txCtx, jobInstance)
	assert.Error(t, err)
	assert.True(t, exception.IsOptimisticLockingFailure(err))
	assert.Equal(t, 0, jobInstance.Version) // Verify version is rolled back

	// TxManager is not called because Tx is in context
	mockTx.AssertExpectations(t)
}
