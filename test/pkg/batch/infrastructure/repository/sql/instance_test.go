package sql_test

import (
	"context"
	"testing"

	"surfin/pkg/batch/adaptor/database"
	adaptor "surfin/pkg/batch/core/adaptor"
	config "surfin/pkg/batch/core/config"
	model "surfin/pkg/batch/core/domain/model"
	job "surfin/pkg/batch/core/domain/repository"
	sqlRepo "surfin/pkg/batch/infrastructure/repository/sql"
	"surfin/pkg/batch/support/util/exception"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	testify_mock "github.com/stretchr/testify/mock" // 修正: エイリアスを使用
	"gorm.io/driver/mysql"                          // 追加
	"gorm.io/gorm"
)

func setupGormInstanceMock(t *testing.T) (*gorm.DB, sqlmock.Sqlmock, adaptor.DBConnection, job.JobRepository) {
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

func TestGORMJobRepository_SaveJobInstance(t *testing.T) {
	gormDB, mock, _, repo := setupGormInstanceMock(t)
	defer func() {
		mock.ExpectClose()
		sqlDB, _ := gormDB.DB()
		sqlDB.Close()
	}()

	ctx := context.Background()
	jobInstance := model.NewJobInstance("testJob", model.NewJobParameters())

	// MockTxManager を使用してトランザクションをモック
	mockTx := new(MockTx)
	// query引数は nil または空マップの可能性があるため Anything を使用
	mockTx.On("ExecuteUpdate", testify_mock.Anything, testify_mock.Anything, "CREATE", "batch_job_instance", testify_mock.Anything).Return(int64(1), nil)

	// トランザクションコンテキストを作成
	txCtx := context.WithValue(ctx, "tx", mockTx)

	err := repo.SaveJobInstance(txCtx, jobInstance)
	assert.NoError(t, err)

	// mockTxManager.AssertExpectations(t) // Txがコンテキストにあるため、TxManagerは呼び出されない
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

	// MockTxManager を使用してトランザクションをモック
	mockTx := new(MockTx)
	expectedQuery := map[string]interface{}{"version": 0}
	mockTx.On("ExecuteUpdate", testify_mock.Anything, testify_mock.Anything, "UPDATE", "batch_job_instance", expectedQuery).Return(int64(1), nil)

	// トランザクションコンテキストを作成
	txCtx := context.WithValue(ctx, "tx", mockTx)

	err := repo.UpdateJobInstance(txCtx, jobInstance)
	assert.NoError(t, err)
	assert.Equal(t, 1, jobInstance.Version) // バージョンがインクリメントされていることを確認

	// mockTxManager.AssertExpectations(t) // Txがコンテキストにあるため、TxManagerは呼び出されない
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

	// MockTxManager を使用してトランザクションをモック
	mockTx := new(MockTx)
	expectedQuery := map[string]interface{}{"version": 0}
	mockTx.On("ExecuteUpdate", testify_mock.Anything, testify_mock.Anything, "UPDATE", "batch_job_instance", expectedQuery).Return(int64(0), nil)

	// トランザクションコンテキストを作成
	txCtx := context.WithValue(ctx, "tx", mockTx)

	err := repo.UpdateJobInstance(txCtx, jobInstance)
	assert.Error(t, err)
	assert.True(t, exception.IsOptimisticLockingFailure(err))
	assert.Equal(t, 0, jobInstance.Version) // バージョンがロールバックされていることを確認

	// mockTxManager.AssertExpectations(t) // Txがコンテキストにあるため、TxManagerは呼び出されない
	mockTx.AssertExpectations(t)
}
