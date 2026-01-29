package sql_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"surfin/pkg/batch/adapter/database"
	adapter "surfin/pkg/batch/core/adapter"
	config "surfin/pkg/batch/core/config"
	model "surfin/pkg/batch/core/domain/model"
	repository "surfin/pkg/batch/core/domain/repository"
	tx "surfin/pkg/batch/core/tx"
	sqlRepo "surfin/pkg/batch/infrastructure/repository/sql"
	"surfin/pkg/batch/support/util/exception"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	testify_mock "github.com/stretchr/testify/mock" // 修正: エイリアスを使用
	"gorm.io/driver/mysql"                          // 追加
	"gorm.io/gorm"
)

// MockTx implements tx.Tx for testing.
type MockTx struct {
	testify_mock.Mock // 修正
}

func (m *MockTx) ExecuteUpdate(ctx context.Context, model interface{}, operation string, tableName string, query map[string]interface{}) (rowsAffected int64, err error) {
	args := m.Called(ctx, model, operation, tableName, query)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockTx) ExecuteUpsert(ctx context.Context, model interface{}, tableName string, conflictColumns []string, updateColumns []string) (rowsAffected int64, err error) {
	args := m.Called(ctx, model, tableName, conflictColumns, updateColumns)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockTx) Savepoint(name string) error {
	args := m.Called(name)
	return args.Error(0)
}

func (m *MockTx) RollbackToSavepoint(name string) error {
	args := m.Called(name)
	return args.Error(0)
}

// MockTxManager は tx.TransactionManager のモック実装
type MockTxManager struct {
	testify_mock.Mock // 修正
}

func (m *MockTxManager) Begin(ctx context.Context, opts ...*sql.TxOptions) (tx.Tx, error) {
	args := m.Called(ctx, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(tx.Tx), args.Error(1)
}

func (m *MockTxManager) Commit(t tx.Tx) error {
	args := m.Called(t)
	return args.Error(0)
}

func (m *MockTxManager) Rollback(t tx.Tx) error {
	args := m.Called(t)
	return args.Error(0)
}

// testSingleConnectionResolver はテスト用の DBConnectionResolver 実装
type testSingleConnectionResolver struct {
	conn adapter.DBConnection
}

func (r *testSingleConnectionResolver) ResolveDBConnection(ctx context.Context, name string) (adapter.DBConnection, error) {
	return r.conn, nil
}

// setupGormJobMock は JobExecution repository テスト用のヘルパーです。
func setupGormJobMock(t *testing.T) (*gorm.DB, sqlmock.Sqlmock, adapter.DBConnection, repository.JobRepository) {
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

func TestGORMJobRepository_SaveJobExecution(t *testing.T) {
	gormDB, mock, _, repo := setupGormJobMock(t)
	defer func() {
		mock.ExpectClose()
		sqlDB, _ := gormDB.DB()
		sqlDB.Close()
	}()

	ctx := context.Background()
	jobExecution := model.NewJobExecution(model.NewID(), "testJob", model.NewJobParameters())

	// MockTxManager を使用してトランザクションをモック
	mockTx := new(MockTx)

	// 期待される ExecuteUpdate("CREATE") の呼び出し (query引数は nil または空マップの可能性があるため Anything を使用)
	mockTx.On("ExecuteUpdate", testify_mock.Anything, testify_mock.Anything, "CREATE", "batch_job_execution", testify_mock.Anything).Return(int64(1), nil)

	// トランザクションコンテキストを作成 (JobRepositoryのgetTxExecutorがTxを検出できるように)
	txCtx := context.WithValue(ctx, "tx", mockTx)

	err := repo.SaveJobExecution(txCtx, jobExecution)
	assert.NoError(t, err)

	// mockTxManager.AssertExpectations(t) // Txがコンテキストにあるため、TxManagerは呼び出されない
	mockTx.AssertExpectations(t)
	// DB操作はTxモックに委譲されているため、sqlmockの期待は不要だが、クリーンアップのために残す
	// assert.NoError(t, mock.ExpectationsWereMet())
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
	jobExecution.ID = model.NewID()
	jobExecution.Version = 5 // 楽観的ロックテストのため、バージョンを5に設定
	jobExecution.MarkAsStarted()

	// MockTxManager を使用してトランザクションをモック
	mockTx := new(MockTx)
	// ExecuteUpdate の呼び出しをモック
	expectedQuery := map[string]interface{}{"version": 5}
	mockTx.On("ExecuteUpdate", testify_mock.Anything, testify_mock.Anything, "UPDATE", "batch_job_execution", expectedQuery).Return(int64(1), nil)

	// トランザクションコンテキストを作成
	txCtx := context.WithValue(ctx, "tx", mockTx)

	err := repo.UpdateJobExecution(txCtx, jobExecution)
	assert.NoError(t, err)
	assert.Equal(t, 6, jobExecution.Version) // バージョンがインクリメントされていることを確認

	// mockTxManager.AssertExpectations(t)
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
	jobExecution.ID = model.NewID()
	jobExecution.Version = 5 // 楽観的ロックテストのため、バージョンを5に設定
	jobExecution.MarkAsStarted()

	// MockTxManager を使用してトランザクションをモック
	mockTx := new(MockTx)
	expectedQuery := map[string]interface{}{"version": 5}
	// RowsAffected: 0 を返す
	mockTx.On("ExecuteUpdate", testify_mock.Anything, testify_mock.Anything, "UPDATE", "batch_job_execution", expectedQuery).Return(int64(0), nil)

	// トランザクションコンテキストを作成
	txCtx := context.WithValue(ctx, "tx", mockTx)

	err := repo.UpdateJobExecution(txCtx, jobExecution)
	assert.Error(t, err)
	assert.True(t, exception.IsOptimisticLockingFailure(err))
	assert.Equal(t, 5, jobExecution.Version) // バージョンがロールバックされていることを確認

	// mockTxManager.AssertExpectations(t)
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

	// GORM First expects a SELECT statement (ExecuteQueryAdvancedの内部動作)
	execRows := sqlmock.NewRows([]string{"id", "job_instance_id", "job_name", "status", "exit_status", "version", "start_time", "create_time", "last_updated", "execution_context", "parameters", "failures"}).
		AddRow(executionID, jobInstanceID, jobName, string(model.BatchStatusStarted), string(model.ExitStatusUnknown), 1, time.Now(), time.Now(), time.Now(), "{}", "{}", "[]")

	// StepExecution のクエリ (FindStepExecutionsByJobExecutionID 経由)
	stepRows := sqlmock.NewRows([]string{"id", "step_name", "job_execution_id", "start_time", "version", "status", "exit_status", "read_count", "write_count", "commit_count", "rollback_count", "filter_count", "skip_read_count", "skip_process_count", "skip_write_count", "execution_context", "last_updated"}).
		AddRow("step-1", "stepA", executionID, time.Now(), 0, string(model.BatchStatusCompleted), string(model.ExitStatusCompleted), 10, 10, 1, 0, 0, 0, 0, 0, "{}", time.Now())

	// 1. JobExecution のクエリ (ExecuteQueryAdvanced経由)
	// FindJobExecutionByID は Order="" Limit=1 を使用する。
	// 修正: LIMIT ? に対応し、引数に 1 を追加
	mock.ExpectQuery("SELECT (.+) FROM `batch_job_execution` WHERE `batch_job_execution`.`id` = \\? LIMIT \\?").
		WithArgs(executionID, 1).
		WillReturnRows(execRows)

	// 2. StepExecution のロード (FindStepExecutionsByJobExecutionID 経由)
	// FindStepExecutionsByJobExecutionID は Order="start_time asc" Limit=0 を使用する。
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
