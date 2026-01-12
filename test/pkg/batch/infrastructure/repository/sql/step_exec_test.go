package sql_test

import (
	"context"
	"testing"

	"surfin/pkg/batch/adaptor/database"
	adaptor "surfin/pkg/batch/core/adaptor"
	config "surfin/pkg/batch/core/config"
	model "surfin/pkg/batch/core/domain/model"
	repository "surfin/pkg/batch/core/domain/repository"
	sqlRepo "surfin/pkg/batch/infrastructure/repository/sql"
	"surfin/pkg/batch/support/util/exception"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	testify_mock "github.com/stretchr/testify/mock" // 修正: エイリアスを使用
	"gorm.io/driver/mysql"                          // 追加
	"gorm.io/gorm"
)

// setupGormStepMock is a setup helper for StepExecution repository tests.
func setupGormStepMock(t *testing.T) (*gorm.DB, sqlmock.Sqlmock, adaptor.DBConnection, repository.JobRepository) {
	sqlDB, mock, err := sqlmock.New()
	assert.NoError(t, err)

	// GORMの初期化に mysql.New を使用 (MockDialectorの代わりに)
	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      sqlDB,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})
	assert.NoError(t, err)

	cfg := config.DatabaseConfig{Type: "mock_db"}
	dbConn := database.NewGormDBAdapter(gormDB, cfg, "mock_db")

	txManager := &MockTxManager{}

	// NewGORMJobRepository のシグネチャ変更に対応
	mockResolver := &testSingleConnectionResolver{conn: dbConn}
	repo := sqlRepo.NewGORMJobRepository(mockResolver, txManager, "mock_db")

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

	// MockTxManager を使用してトランザクションをモック
	mockTx := new(MockTx)
	mockTx.On("ExecuteUpdate", testify_mock.Anything, testify_mock.Anything, "CREATE", "batch_step_execution", testify_mock.Anything).Return(int64(1), nil)

	// トランザクションコンテキストを作成
	txCtx := context.WithValue(ctx, "tx", mockTx)

	err := repo.SaveStepExecution(txCtx, stepExecution)
	assert.NoError(t, err)

	// mockTxManager.AssertExpectations(t) // Txがコンテキストにあるため、TxManagerは呼び出されない
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

	// MockTxManager を使用してトランザクションをモック
	mockTx := new(MockTx)
	expectedQuery := map[string]interface{}{"version": 0}
	mockTx.On("ExecuteUpdate", testify_mock.Anything, testify_mock.Anything, "UPDATE", "batch_step_execution", expectedQuery).Return(int64(1), nil)

	// トランザクションコンテキストを作成
	txCtx := context.WithValue(ctx, "tx", mockTx)

	err := repo.UpdateStepExecution(txCtx, stepExecution)
	assert.NoError(t, err)
	assert.Equal(t, 1, stepExecution.Version) // バージョンがインクリメントされていることを確認

	// mockTxManager.AssertExpectations(t) // Txがコンテキストにあるため、TxManagerは呼び出されない
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

	// MockTxManager を使用してトランザクションをモック
	mockTx := new(MockTx)
	expectedQuery := map[string]interface{}{"version": 0}
	mockTx.On("ExecuteUpdate", testify_mock.Anything, testify_mock.Anything, "UPDATE", "batch_step_execution", expectedQuery).Return(int64(0), nil)

	// トランザクションコンテキストを作成
	txCtx := context.WithValue(ctx, "tx", mockTx)

	err := repo.UpdateStepExecution(txCtx, stepExecution)
	assert.Error(t, err)
	assert.True(t, exception.IsOptimisticLockingFailure(err))
	assert.Equal(t, 0, stepExecution.Version) // バージョンがロールバックされていることを確認

	// mockTxManager.AssertExpectations(t) // Txがコンテキストにあるため、TxManagerは呼び出されない
	mockTx.AssertExpectations(t)
}
