package item_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"surfin/pkg/batch/core/config"
	"surfin/pkg/batch/core/domain/model"
	"surfin/pkg/batch/core/tx"
	"surfin/pkg/batch/engine/step/item"
	"surfin/pkg/batch/support/util/exception"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// --- Mocks ---

// MockItemReader は port.ItemReader のモック実装です。
type MockItemReader struct {
	mock.Mock
	readCount int
}

func (m *MockItemReader) Read(ctx context.Context) (any, error) {
	args := m.Called(ctx)
	return args.Get(0), args.Error(1)
}
func (m *MockItemReader) Open(ctx context.Context, ec model.ExecutionContext) error { return m.Called(ctx, ec).Error(0) }
func (m *MockItemReader) Close(ctx context.Context) error { return m.Called(ctx).Error(0) }
func (m *MockItemReader) SetExecutionContext(ctx context.Context, ec model.ExecutionContext) error {
	return m.Called(ctx, ec).Error(0)
}
func (m *MockItemReader) GetExecutionContext(ctx context.Context) (model.ExecutionContext, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return model.NewExecutionContext(), args.Error(1)
	}
	return args.Get(0).(model.ExecutionContext), args.Error(1)
}

// MockItemProcessor は port.ItemProcessor のモック実装です。
type MockItemProcessor struct {
	mock.Mock
}

func (m *MockItemProcessor) Process(ctx context.Context, item any) (any, error) {
	args := m.Called(ctx, item)
	return args.Get(0), args.Error(1)
}
func (m *MockItemProcessor) SetExecutionContext(ctx context.Context, ec model.ExecutionContext) error {
	return m.Called(ctx, ec).Error(0)
}
func (m *MockItemProcessor) GetExecutionContext(ctx context.Context) (model.ExecutionContext, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return model.NewExecutionContext(), args.Error(1)
	}
	return args.Get(0).(model.ExecutionContext), args.Error(1)
}

// MockItemWriter は port.ItemWriter のモック実装です。
type MockItemWriter struct {
	mock.Mock
}

func (m *MockItemWriter) Write(ctx context.Context, txAdapter tx.Tx, items []any) error {
	args := m.Called(ctx, txAdapter, items)
	return args.Error(0)
}
func (m *MockItemWriter) Open(ctx context.Context, ec model.ExecutionContext) error { return m.Called(ctx, ec).Error(0) }
func (m *MockItemWriter) Close(ctx context.Context) error { return m.Called(ctx).Error(0) }
func (m *MockItemWriter) SetExecutionContext(ctx context.Context, ec model.ExecutionContext) error {
	return m.Called(ctx, ec).Error(0)
}
func (m *MockItemWriter) GetExecutionContext(ctx context.Context) (model.ExecutionContext, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return model.NewExecutionContext(), args.Error(1)
	}
	return args.Get(0).(model.ExecutionContext), args.Error(1)
}

// MockJobRepository は repository.JobRepository のモック実装です。
type MockJobRepository struct {
	mock.Mock
}

// JobInstance methods
func (m *MockJobRepository) SaveJobInstance(ctx context.Context, instance *model.JobInstance) error {
	return m.Called(ctx, instance).Error(0)
}
func (m *MockJobRepository) UpdateJobInstance(ctx context.Context, instance *model.JobInstance) error {
	return m.Called(ctx, instance).Error(0)
}
func (m *MockJobRepository) FindJobInstanceByID(ctx context.Context, id string) (*model.JobInstance, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.JobInstance), args.Error(1)
}
func (m *MockJobRepository) FindJobInstanceByJobNameAndParameters(ctx context.Context, jobName string, params model.JobParameters) (*model.JobInstance, error) {
	args := m.Called(ctx, jobName, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.JobInstance), args.Error(1)
}
func (m *MockJobRepository) FindJobInstancesByJobNameAndPartialParameters(ctx context.Context, jobName string, partialParams model.JobParameters) ([]*model.JobInstance, error) {
	args := m.Called(ctx, jobName, partialParams)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*model.JobInstance), args.Error(1)
}
func (m *MockJobRepository) GetJobInstanceCount(ctx context.Context, jobName string) (int, error) {
	args := m.Called(ctx, jobName)
	return args.Int(0), args.Error(1)
}
func (m *MockJobRepository) GetJobNames(ctx context.Context) ([]string, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

// JobExecution methods
func (m *MockJobRepository) SaveJobExecution(ctx context.Context, execution *model.JobExecution) error {
	return m.Called(ctx, execution).Error(0)
}
func (m *MockJobRepository) UpdateJobExecution(ctx context.Context, execution *model.JobExecution) error {
	return m.Called(ctx, execution).Error(0)
}
func (m *MockJobRepository) FindJobExecutionByID(ctx context.Context, id string) (*model.JobExecution, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.JobExecution), args.Error(1)
}
func (m *MockJobRepository) FindLatestRestartableJobExecution(ctx context.Context, jobInstanceID string) (*model.JobExecution, error) {
	args := m.Called(ctx, jobInstanceID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.JobExecution), args.Error(1)
}
func (m *MockJobRepository) FindJobExecutionsByJobInstance(ctx context.Context, instance *model.JobInstance) ([]*model.JobExecution, error) {
	args := m.Called(ctx, instance)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*model.JobExecution), args.Error(1)
}

// StepExecution methods
func (m *MockJobRepository) SaveStepExecution(ctx context.Context, stepExec *model.StepExecution) error {
	return m.Called(ctx, stepExec).Error(0)
}
func (m *MockJobRepository) UpdateStepExecution(ctx context.Context, stepExec *model.StepExecution) error {
	return m.Called(ctx, stepExec).Error(0)
}
func (m *MockJobRepository) FindStepExecutionByID(ctx context.Context, id string) (*model.StepExecution, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.StepExecution), args.Error(1)
}

// CheckpointDataRepository methods
func (m *MockJobRepository) FindCheckpointData(ctx context.Context, stepExecutionID string) (*model.CheckpointData, error) {
	args := m.Called(ctx, stepExecutionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.CheckpointData), args.Error(1)
}
func (m *MockJobRepository) SaveCheckpointData(ctx context.Context, cd *model.CheckpointData) error {
	return m.Called(ctx, cd).Error(0)
}

// Close method
func (m *MockJobRepository) Close() error {
	return m.Called().Error(0)
}

// MockTransactionManager は tx.TransactionManager のモック実装です。
type MockTransactionManager struct {
	mock.Mock
}

func (m *MockTransactionManager) Begin(ctx context.Context, opts ...*sql.TxOptions) (tx.Tx, error) {
	args := m.Called(ctx, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(tx.Tx), args.Error(1)
}
func (m *MockTransactionManager) Commit(adapter tx.Tx) error {
	return m.Called(adapter).Error(0)
}
func (m *MockTransactionManager) Rollback(adapter tx.Tx) error {
	return m.Called(adapter).Error(0)
}

// MockTransactionAdapter は tx.Tx のモック実装です。
type MockTransactionAdapter struct {
	mock.Mock
}

func (m *MockTransactionAdapter) ExecuteUpdate(ctx context.Context, model interface{}, operation string, tableName string, query map[string]interface{}) (rowsAffected int64, err error) {
	args := m.Called(ctx, model, operation, tableName, query)
	return args.Get(0).(int64), args.Error(1)
}
func (m *MockTransactionAdapter) ExecuteUpsert(ctx context.Context, model interface{}, tableName string, conflictColumns []string, updateColumns []string) (rowsAffected int64, err error) {
	args := m.Called(ctx, model, tableName, conflictColumns, updateColumns)
	return args.Get(0).(int64), args.Error(1)
}
func (m *MockTransactionAdapter) Savepoint(name string) error {
	return m.Called(name).Error(0)
}
func (m *MockTransactionAdapter) RollbackToSavepoint(name string) error {
	return m.Called(name).Error(0)
}

// MockMetricRecorder は metrics.MetricRecorder のモック実装です。
type MockMetricRecorder struct {
	mock.Mock
}

func (m *MockMetricRecorder) RecordJobStart(ctx context.Context, execution *model.JobExecution) { m.Called(ctx, execution) }
func (m *MockMetricRecorder) RecordJobEnd(ctx context.Context, execution *model.JobExecution) { m.Called(ctx, execution) }
func (m *MockMetricRecorder) RecordStepStart(ctx context.Context, execution *model.StepExecution) { m.Called(ctx, execution) }
func (m *MockMetricRecorder) RecordStepEnd(ctx context.Context, execution *model.StepExecution) { m.Called(ctx, execution) }
func (m *MockMetricRecorder) RecordItemRead(ctx context.Context, stepID string) { m.Called(ctx, stepID) }
func (m *MockMetricRecorder) RecordItemProcess(ctx context.Context, stepID string) { m.Called(ctx, stepID) }
func (m *MockMetricRecorder) RecordItemWrite(ctx context.Context, stepID string, count int) { m.Called(ctx, stepID, count) }
func (m *MockMetricRecorder) RecordItemSkip(ctx context.Context, stepID string, reason string) { m.Called(ctx, stepID, reason) }
func (m *MockMetricRecorder) RecordItemRetry(ctx context.Context, stepID string, reason string) { m.Called(ctx, stepID, reason) }
func (m *MockMetricRecorder) RecordChunkCommit(ctx context.Context, stepID string, count int) { m.Called(ctx, stepID, count) }
func (m *MockMetricRecorder) RecordDuration(ctx context.Context, name string, duration time.Duration, tags map[string]string) { m.Called(ctx, name, duration, tags) }

// MockTracer は metrics.Tracer のモック実装です。
type MockTracer struct {
	mock.Mock
}

func (m *MockTracer) RecordError(ctx context.Context, stepID string, err error) { m.Called(ctx, stepID, err) }
func (m *MockTracer) StartJobSpan(ctx context.Context, execution *model.JobExecution) (context.Context, func()) {
	args := m.Called(ctx, execution)
	if args.Get(1) == nil {
		return ctx, func() {}
	}
	return args.Get(0).(context.Context), args.Get(1).(func())
}
func (m *MockTracer) StartStepSpan(ctx context.Context, execution *model.StepExecution) (context.Context, func()) {
	args := m.Called(ctx, execution)
	if args.Get(1) == nil {
		return ctx, func() {}
	}
	return args.Get(0).(context.Context), args.Get(1).(func())
}
func (m *MockTracer) RecordEvent(ctx context.Context, name string, attributes map[string]interface{}) { m.Called(ctx, name, attributes) }

// --- Test Setup (修正) ---

func setupChunkStep(t *testing.T) (*item.ChunkStep, *MockItemReader, *MockItemProcessor, *MockItemWriter, *MockJobRepository, *MockTransactionManager, *MockMetricRecorder, *MockTracer) {
	reader := new(MockItemReader)
	processor := new(MockItemProcessor)
	writer := new(MockItemWriter)
	repo := new(MockJobRepository)
	txManager := new(MockTransactionManager)
	metricRecorder := new(MockMetricRecorder)
	tracer := new(MockTracer)

	// Default configurations for policies
	retryConfig := &config.RetryConfig{MaxAttempts: 3}
	itemRetryConfig := config.ItemRetryConfig{MaxAttempts: 3, RetryableExceptions: []string{"TransientError"}}
	// isSkippable=true な BatchError をスキップ可能とするために、"BatchError" を型名で指定
	itemSkipConfig := config.ItemSkipConfig{SkipLimit: 5, SkippableExceptions: []string{"BatchError"}}

	step := item.NewJSLAdaptedStep(
		"testStep",
		reader,
		processor,
		writer,
		10, // chunkSize
		10, // commitInterval
		retryConfig,
		itemRetryConfig,
		itemSkipConfig,
		repo,
		nil, nil, nil, nil, nil, nil, nil, // Listeners
		model.NewExecutionContextPromotion(),
		"SERIALIZABLE",
		"REQUIRED",
		txManager,
		metricRecorder,
		tracer,
	)
	
	// Ensure the step is correctly initialized
	assert.NotNil(t, step)

	return step, reader, processor, writer, repo, txManager, metricRecorder, tracer
}

// --- Tests ---

// TestChunkStep_ChunkSplittingOnSkippableWriteFailure は、チャンク書き込み失敗時にチャンク分割処理が正しく行われることを検証します。
// 成功アイテムはコミットされ、スキップ可能エラーアイテムはスキップされます。
func TestChunkStep_ChunkSplittingOnSkippableWriteFailure(t *testing.T) {
	step, _, _, writer, repo, txManager, recorder, tracer := setupChunkStep(t)
	ctx := context.Background()
	
	// Initial Step Execution setup
	jobExec := model.NewJobExecution(model.NewID(), "testJob", model.NewJobParameters())
	se := model.NewStepExecution(model.NewID(), jobExec, "testStep")
	se.WriteCount = 10 // 既に10件の書き込みがあったと仮定
	
	originalItems := []any{"item1", "item2", "item3"}
	
	// Define the skippable error
	skippableErr := exception.NewBatchError("testStep", "DB constraint violation", errors.New("unique constraint failed"), true, false) // S=true, R=false
	// Note: exception.AddExceptionName は存在しないため、ここではエラーメッセージまたは型名でマッチさせる
	
	// Mock Transaction Adapters for chunk splitting (one per item)
	txAdapter1 := new(MockTransactionAdapter)
	txAdapter2 := new(MockTransactionAdapter)
	txAdapter3 := new(MockTransactionAdapter)
	
	// Mock JobRepository calls for StepExecution updates (Save/Update)
	repo.On("UpdateStepExecution", mock.Anything, mock.AnythingOfType("*model.StepExecution")).Return(nil).Maybe()
	
	// Mock Tracer Span End function (ダミー)
	mockSpanEnd := func() {}
	tracer.On("StartJobSpan", mock.Anything, mock.Anything).Return(ctx, mockSpanEnd).Maybe()
	tracer.On("StartStepSpan", mock.Anything, mock.Anything).Return(ctx, mockSpanEnd).Maybe()
	
	// 1. Transaction Manager Mocks: Begin/Commit/Rollback for chunk splitting
	
	// Item 1: Success
	txManager.On("Begin", mock.Anything, mock.Anything).Return(txAdapter1, nil).Once()
	txManager.On("Commit", txAdapter1).Return(nil).Once()
	
	// Item 2: Failure (Skippable)
	txManager.On("Begin", mock.Anything, mock.Anything).Return(txAdapter2, nil).Once()
	txManager.On("Rollback", txAdapter2).Return(nil).Once()
	
	// Item 3: Success
	txManager.On("Begin", mock.Anything, mock.Anything).Return(txAdapter3, nil).Once()
	txManager.On("Commit", txAdapter3).Return(nil).Once()
	
	// 2. Item Writer Mocks: Write calls during chunk splitting
	
	// Item 1 write: Success
	writer.On("Write", mock.Anything, txAdapter1, []any{"item1"}).Return(nil).Once()
	
	// Item 2 write: Failure (Skippable)
	writer.On("Write", mock.Anything, txAdapter2, []any{"item2"}).Return(skippableErr).Once()
	
	// Item 3 write: Success
	writer.On("Write", mock.Anything, txAdapter3, []any{"item3"}).Return(nil).Once()
	
	// 3. Listener/Metric Mocks
	// Item 2 Skip notification
	tracer.On("RecordError", mock.Anything, "testStep", mock.AnythingOfType("*exception.BatchError")).Once() // エラー型を明示
	recorder.On("RecordItemSkip", mock.Anything, "testStep", "write").Once()
	
	// Item 1 and Item 3 success
	recorder.On("RecordItemWrite", mock.Anything, "testStep", 1).Twice() 

	// --- Execution ---
	
	// HandleSkippableWriteFailure は public に変更されている前提
	remainingItems, fatalErr := step.HandleSkippableWriteFailure(ctx, originalItems, se)
	
	// --- Assertions ---
	
	assert.Nil(t, fatalErr, "Chunk splitting should not result in a fatal error")
	assert.Empty(t, remainingItems, "All items should have been processed (committed or skipped)")
	
	// Item 1 (Success) + Item 3 (Success) = 2 new writes
	assert.Equal(t, 12, se.WriteCount, "Write count should be updated (10 initial + 2 successful commits)")
	
	// Item 2 (Skipped) = 1 skip
	assert.Equal(t, 1, se.SkipWriteCount, "Skip write count should be 1")
	
	// Verify mocks
	txManager.AssertExpectations(t)
	writer.AssertExpectations(t)
	recorder.AssertExpectations(t)
	tracer.AssertExpectations(t)
}

// TestChunkStep_ChunkSplitting_FatalError は、チャンク分割中に致命的なエラーが発生した場合に、処理が中断されることを検証します。
func TestChunkStep_ChunkSplitting_FatalError(t *testing.T) {
	step, _, _, writer, repo, txManager, recorder, tracer := setupChunkStep(t)
	ctx := context.Background()
	
	jobExec := model.NewJobExecution(model.NewID(), "testJob", model.NewJobParameters())
	se := model.NewStepExecution(model.NewID(), jobExec, "testStep")
	originalItems := []any{"item1", "item2"}
	
	// スキップ可能ではない致命的なエラー
	fatalWriteErr := errors.New("database connection lost")
	
	// Mock Transaction Adapters
	txAdapter1 := new(MockTransactionAdapter)
	txAdapter2 := new(MockTransactionAdapter)
	
	// Mock JobRepository calls for StepExecution updates (Save/Update)
	repo.On("UpdateStepExecution", mock.Anything, mock.AnythingOfType("*model.StepExecution")).Return(nil).Maybe()
	
	// Mock Tracer Span End function (ダミー)
	mockSpanEnd := func() {}
	tracer.On("StartJobSpan", mock.Anything, mock.Anything).Return(ctx, mockSpanEnd).Maybe()
	tracer.On("StartStepSpan", mock.Anything, mock.Anything).Return(ctx, mockSpanEnd).Maybe()
	
	// Item 1: Success
	txManager.On("Begin", mock.Anything, mock.Anything).Return(txAdapter1, nil).Once()
	writer.On("Write", mock.Anything, txAdapter1, []any{"item1"}).Return(nil).Once()
	txManager.On("Commit", txAdapter1).Return(nil).Once()
	recorder.On("RecordItemWrite", mock.Anything, "testStep", 1).Once()
	
	// Item 2: Failure (Fatal error during write)
	txManager.On("Begin", mock.Anything, mock.Anything).Return(txAdapter2, nil).Once()
	writer.On("Write", mock.Anything, txAdapter2, []any{"item2"}).Return(fatalWriteErr).Once()
	
	// Fatal errorが発生した場合、そのアイテムのトランザクションはロールバックされる
	txManager.On("Rollback", txAdapter2).Return(nil).Once()
	
	// --- Execution ---
	
	remainingItems, resultErr := step.HandleSkippableWriteFailure(ctx, originalItems, se)
	
	// --- Assertions ---
	
	assert.NotNil(t, resultErr, "Fatal error should be returned")
	assert.True(t, exception.IsBatchError(resultErr), "Error should be wrapped as BatchError")
	assert.Contains(t, resultErr.Error(), "Item write failed during chunk splitting (Fatal or limit reached)", "Error message should indicate fatal failure")
	
	// Item 1はコミットされた
	assert.Equal(t, 1, se.WriteCount, "Write count should reflect committed items before fatal error")
	assert.Equal(t, 0, se.SkipWriteCount, "No skips occurred")
	assert.Empty(t, remainingItems, "処理が中断されたため、残りのアイテムは空であるべき")
	
	// Verify mocks
	txManager.AssertExpectations(t)
	writer.AssertExpectations(t)
	recorder.AssertExpectations(t)
	tracer.AssertExpectations(t)
}
