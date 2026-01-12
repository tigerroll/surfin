package sql_test

import (
	"context"
	"log"
	"os"
	"sync"
	"testing"
	"time"

	adaptor "surfin/pkg/batch/core/adaptor"
	config "surfin/pkg/batch/core/config"
	model "surfin/pkg/batch/core/domain/model"
	"surfin/pkg/batch/core/domain/repository"
	adaptor_core "surfin/pkg/batch/core/adaptor"
	sql_repo "surfin/pkg/batch/infrastructure/repository/sql"
	"surfin/pkg/batch/adaptor/database"
	tx "surfin/pkg/batch/core/tx"
	"surfin/pkg/batch/support/util/exception"
	test_util "surfin/pkg/batch/test"

	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const sqliteDBFile = "test_metadata_sqlite.db"

var (
	globalGormDB    *gorm.DB
	globalDBConn    adaptor.DBConnection
	globalTxManager tx.TransactionManager
	once            sync.Once
	testDBResolver  adaptor_core.DBConnectionResolver
)

// setupSQLiteTestDB は SQLite のテスト環境をセットアップします。
// テストスイート全体で単一のインメモリDB接続を共有します。
func setupSQLiteTestDB(t *testing.T) (repository.JobRepository, adaptor.DBConnection, tx.TransactionManager) {
	once.Do(func() {
		// 既存のDBファイルを削除 (ファイルベースの場合のみ。インメモリでは不要だが、念のため)
		os.Remove(sqliteDBFile)

		cfg := config.DatabaseConfig{
			Type: "sqlite",
			// インメモリDBを使用
			Database: ":memory:",
			Pool: config.PoolConfig{
				MaxOpenConns: 1,
				MaxIdleConns: 1,
			},
		}

		// 1. DBConnection の確立
		dialector, err := database.GetDialectorForTest(cfg)
		assert.NoError(t, err)

		// GORM Logger を Silent に設定
		gormLogger := logger.New(
			log.New(os.Stdout, "\r\n", log.LstdFlags),
			logger.Config{
				SlowThreshold: time.Second,
				LogLevel:      logger.Silent, // 明示的に Silent に設定
				Colorful:      false,
			},
		)

		gormDB, err := gorm.Open(dialector, &gorm.Config{Logger: gormLogger})
		assert.NoError(t, err)

		globalGormDB = gormDB
		// NewGormDBAdapter のシグネチャに合わせる
		globalDBConn = database.NewGormDBAdapter(gormDB, cfg, "test_sqlite") 
		
		// Test Resolver を作成: 常に globalDBConn を返す
		testDBResolver = &testSingleConnectionResolver{conn: globalDBConn}

		// TransactionManagerFactory を使用して TxManager を作成する
		// NewGormTransactionManagerFactory は DBConnectionResolver を受け取るように変更されたため、ここでは Resolver を渡す
		txFactory := database.NewGormTransactionManagerFactory(testDBResolver)
		globalTxManager = txFactory.NewTransactionManager(globalDBConn)

		// 2. スキーマのマイグレーション (手動で SQL を実行)
		err = createTestTables(globalGormDB)
		assert.NoError(t, err, "Failed to create test tables manually")
	})

	// 3. JobRepository の作成 (共有接続を使用)
	// JobRepository は DBResolver を受け取るように変更
	repo := sql_repo.NewGORMJobRepository(testDBResolver, globalTxManager, "test_sqlite")

	return repo, globalDBConn, globalTxManager
}

// teardownSQLiteTestDB はテストスイートの最後に一度だけ実行されるべきですが、
// 現在のテスト構造では各テストの defer で呼ばれるため、ここでは何もしません。
func teardownSQLiteTestDB(t *testing.T, dbConn adaptor.DBConnection) {
	// インメモリDBを使用しているため、クリーンアップは不要
}

// TestSQLiteJobRepository_Lifecycle は SQLite を使用した JobRepository の基本的なライフサイクルをテストします。
func TestSQLiteJobRepository_Lifecycle(t *testing.T) {
	repo, dbConn, _ := setupSQLiteTestDB(t)
	defer teardownSQLiteTestDB(t, dbConn) // defer は残すが、中身は空

	ctx := context.Background()

	// 変数を事前に宣言し、:= の使用を最初の宣言に限定する
	var txAdapter tx.Tx
	var txCtx context.Context
	var err error
	var foundInstance *model.JobInstance
	var foundExecution *model.JobExecution
	var foundStepExecution *model.StepExecution

	// 1. JobInstance の保存と検索
	instance := test_util.NewTestJobInstance("sqliteJob", test_util.NewTestJobParameters(map[string]interface{}{"key": "value"}))
	// トランザクションを開始
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

	// 2. JobExecution の保存と更新
	execution := test_util.NewTestJobExecution(instance.ID, instance.JobName, instance.Parameters)
	txAdapter, err = globalTxManager.Begin(ctx)
	assert.NoError(t, err)
	txCtx = context.WithValue(ctx, "tx", txAdapter)
	
	err = repo.SaveJobExecution(txCtx, execution)
	assert.NoError(t, err)
	err = globalTxManager.Commit(txAdapter) // ここでコミットを追加
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

	// 3. StepExecution の保存と更新
	stepExecution := test_util.NewTestStepExecution(execution, "step1")
	txAdapter, err = globalTxManager.Begin(ctx)
	assert.NoError(t, err)
	txCtx = context.WithValue(ctx, "tx", txAdapter)
	
	err = repo.SaveStepExecution(txCtx, stepExecution)
	assert.NoError(t, err)
	err = globalTxManager.Commit(txAdapter) // ここでコミットを追加
	assert.NoError(t, err)

	stepExecution.MarkAsStarted() // 状態遷移修正
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

	// 4. クリーンアップ (インメモリDBのため、通常は不要だが、レコードは残るため削除)
	// トランザクション外で実行
	globalGormDB.Exec("DELETE FROM batch_checkpoint_data WHERE step_execution_id = ?", stepExecution.ID)
	globalGormDB.Exec("DELETE FROM batch_step_execution WHERE id = ?", stepExecution.ID)
	globalGormDB.Exec("DELETE FROM batch_job_execution WHERE id = ?", execution.ID)
	globalGormDB.Exec("DELETE FROM batch_job_instance WHERE id = ?", instance.ID)
}

// TestSQLiteJobRepository_OptimisticLocking は SQLite で楽観的ロックが機能するかテストします。
func TestSQLiteJobRepository_OptimisticLocking(t *testing.T) {
	repo, dbConn, _ := setupSQLiteTestDB(t)
	defer teardownSQLiteTestDB(t, dbConn)

	ctx := context.Background()
	instance := test_util.NewTestJobInstance("lockTestJob", test_util.NewTestJobParameters(nil))
	
	// JobInstanceの保存
	txAdapter, err := globalTxManager.Begin(ctx)
	assert.NoError(t, err)
	txCtx := context.WithValue(ctx, "tx", txAdapter)
	repo.SaveJobInstance(txCtx, instance)
	globalTxManager.Commit(txAdapter)

	// 1. JobExecution の楽観的ロックテスト
	exec1 := test_util.NewTestJobExecution(instance.ID, instance.JobName, instance.Parameters)
	
	txAdapter, err = globalTxManager.Begin(ctx)
	assert.NoError(t, err)
	txCtx = context.WithValue(ctx, "tx", txAdapter)
	repo.SaveJobExecution(txCtx, exec1) // Version 0
	globalTxManager.Commit(txAdapter)

	foundExec, _ := repo.FindJobExecutionByID(ctx, exec1.ID) // Version 0 を取得

	// exec1 (Version 0) で更新を試みる -> 成功
	exec1.Version = 0
	exec1.MarkAsStarted()
	txAdapter, err = globalTxManager.Begin(ctx)
	assert.NoError(t, err)
	txCtx = context.WithValue(ctx, "tx", txAdapter)
	err = repo.UpdateJobExecution(txCtx, exec1) // Version 0 -> 1
	assert.NoError(t, err)
	globalTxManager.Commit(txAdapter)
	assert.Equal(t, 1, exec1.Version)

	// foundExec (Version 0) で更新を試みる -> 失敗するはず
	foundExec.MarkAsStarted()
	foundExec.MarkAsCompleted()
	txAdapter, err = globalTxManager.Begin(ctx)
	assert.NoError(t, err)
	txCtx = context.WithValue(ctx, "tx", txAdapter)
	err = repo.UpdateJobExecution(txCtx, foundExec)
	assert.Error(t, err)
	assert.True(t, exception.IsOptimisticLockingFailure(err))
	globalTxManager.Rollback(txAdapter) // ロールバック

	// クリーンアップ
	globalGormDB.Exec("DELETE FROM batch_job_execution WHERE id = ?", exec1.ID)
	globalGormDB.Exec("DELETE FROM batch_job_instance WHERE id = ?", instance.ID)
}

// TestSQLiteJobRepository_CheckpointData は SQLite でチェックポイントデータが機能するかテストします。
func TestSQLiteJobRepository_CheckpointData(t *testing.T) {
	repo, dbConn, txManager := setupSQLiteTestDB(t)
	defer teardownSQLiteTestDB(t, dbConn)

	ctx := context.Background()
	instance := test_util.NewTestJobInstance("checkpointJob", test_util.NewTestJobParameters(nil))
	exec := test_util.NewTestJobExecution(instance.ID, instance.JobName, instance.Parameters)
	stepExec := test_util.NewTestStepExecution(exec, "checkpointStep")
	
	// 変数を事前に宣言
	var txAdapter tx.Tx
	var txCtx context.Context
	var err error
	var foundData *model.CheckpointData
	var offset int
	var ok bool

	// StepExecutionを先に保存
	txAdapter, err = txManager.Begin(ctx)
	assert.NoError(t, err)
	txCtx = context.WithValue(ctx, "tx", txAdapter)
	repo.SaveStepExecution(txCtx, stepExec)
	txManager.Commit(txAdapter)

	// 1. チェックポイントデータの保存
	ec := test_util.NewTestExecutionContext(nil)
	ec.Put("offset", 50)
	dataToSave := &model.CheckpointData{StepExecutionID: stepExec.ID, ExecutionContext: ec}
	
	txAdapter, err = txManager.Begin(ctx)
	assert.NoError(t, err)
	txCtx = context.WithValue(ctx, "tx", txAdapter)
	err = repo.SaveCheckpointData(txCtx, dataToSave)
	assert.NoError(t, err)
	txManager.Commit(txAdapter)

	// 2. チェックポイントデータの検索
	foundData, err = repo.FindCheckpointData(ctx, stepExec.ID)
	assert.NoError(t, err)
	assert.NotNil(t, foundData)
	offset, ok = foundData.ExecutionContext.GetInt("offset")
	assert.True(t, ok)
	assert.Equal(t, 50, offset)
	
	// 3. 更新のテスト
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

	// 4. クリーンアップ
	globalGormDB.Exec("DELETE FROM batch_checkpoint_data WHERE step_execution_id = ?", stepExec.ID)
	globalGormDB.Exec("DELETE FROM batch_step_execution WHERE id = ?", stepExec.ID)
}

// createTestTables は SQLite のテストに必要なテーブルを手動で作成します。
// これは、schema.go に GORM タグを付与しないという制約を維持しつつ、
// AutoMigrate への依存を排除するために必要です。
func createTestTables(db *gorm.DB) error {
	// NOTE: SQLite の場合、JSON 型は TEXT として扱われます。
	// 主キー、NOT NULL、インデックスを定義します。
	
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
