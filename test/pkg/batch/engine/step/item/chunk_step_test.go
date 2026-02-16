// Package item_test provides unit tests for the item processing steps,
// specifically the ChunkStep.
package item_test

import (
	"context"
	"database/sql"
	"errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	dbconfig "github.com/tigerroll/surfin/pkg/batch/adapter/database/config"
	coreadapter "github.com/tigerroll/surfin/pkg/batch/core/adapter"
	port "github.com/tigerroll/surfin/pkg/batch/core/application/port"
	"github.com/tigerroll/surfin/pkg/batch/core/config"
	"github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	"github.com/tigerroll/surfin/pkg/batch/core/tx"
	"github.com/tigerroll/surfin/pkg/batch/engine/step/item"
	"github.com/tigerroll/surfin/pkg/batch/support/util/exception"
	testutil "github.com/tigerroll/surfin/pkg/batch/test"
	"testing"
	"time"
)

// Dummy usage to prevent "imported and not used" error for the 'port' package.
var _ = port.ErrNoMoreItems

// --- Mocks ---

// MockItemReader is a mock implementation of port.ItemReader.
type MockItemReader struct {
	mock.Mock
	readCount int
}

func (m *MockItemReader) Read(ctx context.Context) (any, error) {
	args := m.Called(ctx)
	return args.Get(0), args.Error(1)
}
func (m *MockItemReader) Open(ctx context.Context, ec model.ExecutionContext) error {
	return m.Called(ctx, ec).Error(0)
}
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

// MockItemProcessor is a mock implementation of the port.ItemProcessor interface.
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

// MockItemWriter is a mock implementation of the port.ItemWriter interface.
type MockItemWriter struct {
	mock.Mock
}

func (m *MockItemWriter) Write(ctx context.Context, items []any) error {
	args := m.Called(ctx, items) // The mock now records only ctx and items
	return args.Error(0)
}
func (m *MockItemWriter) Open(ctx context.Context, ec model.ExecutionContext) error {
	return m.Called(ctx, ec).Error(0)
}
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

// GetTableName is a mock implementation of the GetTableName method from the port.ItemWriter interface.
func (m *MockItemWriter) GetTableName() string {
	args := m.Called()
	return args.String(0)
}

// GetTargetDBName is a mock implementation of the GetTargetDBName method from the port.ItemWriter interface.
func (m *MockItemWriter) GetTargetDBName() string {
	args := m.Called()
	return args.String(0)
}

// GetTargetResourceName is a mock implementation of the GetTargetResourceName method from the port.ItemWriter interface.
func (m *MockItemWriter) GetTargetResourceName() string {
	args := m.Called()
	return args.String(0)
}

// GetResourcePath is a mock implementation of the GetResourcePath method from the port.ItemWriter interface.
func (m *MockItemWriter) GetResourcePath() string {
	args := m.Called()
	return args.String(0)
}

// MockJobRepository is a mock implementation of the repository.JobRepository interface.
type MockJobRepository struct {
	mock.Mock
}

// JobInstance methods.
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
	if res, ok := args.Get(0).([]string); ok {
		return res, args.Error(1)
	}
	return nil, args.Error(1)
}

// JobExecution methods.
func (m *MockJobRepository) SaveJobExecution(ctx context.Context, execution *model.JobExecution) error {
	return m.Called(ctx, execution).Error(0)
}
func (m *MockJobRepository) UpdateJobExecution(ctx context.Context, execution *model.JobExecution) error {
	return m.Called(ctx, execution).Error(0)
}
func (m *MockJobRepository) FindJobExecutionByID(ctx context.Context, id string) (*model.JobExecution, error) {
	args := m.Called(ctx, id)
	if res, ok := args.Get(0).(*model.JobExecution); ok {
		return res, args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *MockJobRepository) FindLatestRestartableJobExecution(ctx context.Context, jobInstanceID string) (*model.JobExecution, error) {
	args := m.Called(ctx, jobInstanceID)
	if res, ok := args.Get(0).(*model.JobExecution); ok {
		return res, args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *MockJobRepository) FindJobExecutionsByJobInstance(ctx context.Context, instance *model.JobInstance) ([]*model.JobExecution, error) {
	args := m.Called(ctx, instance)
	if res, ok := args.Get(0).([]*model.JobExecution); ok {
		return res, args.Error(1)
	}
	return nil, args.Error(1)
}

// StepExecution methods.
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

// CheckpointDataRepository methods.
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

// Close method.
func (m *MockJobRepository) Close() error {
	return m.Called().Error(0)
}

// MockTransactionManager is a mock implementation of the tx.TransactionManager interface.
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

// MockMetricRecorder is a mock implementation of the metrics.MetricRecorder interface.
type MockMetricRecorder struct {
	mock.Mock
}

func (m *MockMetricRecorder) RecordJobStart(ctx context.Context, execution *model.JobExecution) {
	m.Called(ctx, execution)
}
func (m *MockMetricRecorder) RecordJobEnd(ctx context.Context, execution *model.JobExecution) {
	m.Called(ctx, execution)
}
func (m *MockMetricRecorder) RecordStepStart(ctx context.Context, execution *model.StepExecution) {
	m.Called(ctx, execution)
}
func (m *MockMetricRecorder) RecordStepEnd(ctx context.Context, execution *model.StepExecution) {
	m.Called(ctx, execution)
}
func (m *MockMetricRecorder) RecordItemRead(ctx context.Context, stepID string) {
	m.Called(ctx, stepID)
}
func (m *MockMetricRecorder) RecordItemProcess(ctx context.Context, stepID string) {
	m.Called(ctx, stepID)
}
func (m *MockMetricRecorder) RecordItemWrite(ctx context.Context, stepID string, count int) {
	m.Called(ctx, stepID, count)
}
func (m *MockMetricRecorder) RecordItemSkip(ctx context.Context, stepID string, reason string) {
	m.Called(ctx, stepID, reason)
}
func (m *MockMetricRecorder) RecordItemRetry(ctx context.Context, stepID string, reason string) {
	m.Called(ctx, stepID, reason)
}
func (m *MockMetricRecorder) RecordChunkCommit(ctx context.Context, stepID string, count int) {
	m.Called(ctx, stepID, count)
}
func (m *MockMetricRecorder) RecordDuration(ctx context.Context, name string, duration time.Duration, tags map[string]string) {
	m.Called(ctx, name, duration, tags)
}

// MockTracer is a mock implementation of metrics.Tracer.
type MockTracer struct {
	mock.Mock
}

func (m *MockTracer) RecordError(ctx context.Context, stepID string, err error) {
	m.Called(ctx, stepID, err)
}
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
func (m *MockTracer) RecordEvent(ctx context.Context, name string, attributes map[string]interface{}) {
	m.Called(ctx, name, attributes)
}

// MockTransactionManagerFactory is a mock implementation of tx.TransactionManagerFactory.
type MockTransactionManagerFactory struct {
	mock.Mock
	TxManager *MockTransactionManager
}

func (m *MockTransactionManagerFactory) NewTransactionManager(conn coreadapter.ResourceConnection) tx.TransactionManager {
	m.Called(conn)
	return m.TxManager // MockTxManager implements the tx.TransactionManager interface, so it can be returned directly.
}

// MockDBConnection is a mock implementation of adapter.DBConnection.
type MockDBConnection struct {
	mock.Mock
}

func (m *MockDBConnection) Close() error { return m.Called().Error(0) }
func (m *MockDBConnection) Type() string { return m.Called().String(0) }
func (m *MockDBConnection) Name() string { return m.Called().String(0) }
func (m *MockDBConnection) RefreshConnection(ctx context.Context) error {
	return m.Called(ctx).Error(0)
}
func (m *MockDBConnection) Config() dbconfig.DatabaseConfig { // Changed config.DatabaseConfig to dbconfig.DatabaseConfig
	args := m.Called()
	return args.Get(0).(dbconfig.DatabaseConfig)
}
func (m *MockDBConnection) GetSQLDB() (*sql.DB, error) {
	args := m.Called() // Added
	return args.Get(0).(*sql.DB), args.Error(1)
}
func (m *MockDBConnection) ExecuteQuery(ctx context.Context, target interface{}, query map[string]interface{}) error {
	return m.Called(ctx, target, query).Error(0) // Added
}
func (m *MockDBConnection) ExecuteQueryAdvanced(ctx context.Context, target interface{}, query map[string]interface{}, orderBy string, limit int) error {
	return m.Called(ctx, target, query, orderBy, limit).Error(0) // Added
}
func (m *MockDBConnection) Count(ctx context.Context, model interface{}, query map[string]interface{}) (int64, error) {
	args := m.Called(ctx, model, query) // Added
	return args.Get(0).(int64), args.Error(1)
}
func (m *MockDBConnection) Pluck(ctx context.Context, model interface{}, column string, target interface{}, query map[string]interface{}) error {
	return m.Called(ctx, model, column, target, query).Error(0) // Added
}
func (m *MockDBConnection) ExecuteUpdate(ctx context.Context, model interface{}, operation string, tableName string, query map[string]interface{}) (rowsAffected int64, err error) {
	args := m.Called(ctx, model, operation, tableName, query) // Added
	return args.Get(0).(int64), args.Error(1)
}
func (m *MockDBConnection) ExecuteUpsert(ctx context.Context, model interface{}, tableName string, conflictColumns []string, updateColumns []string) (rowsAffected int64, err error) {
	args := m.Called(ctx, model, tableName, conflictColumns, updateColumns) // Added
	return args.Get(0).(int64), args.Error(1)
}
func (m *MockDBConnection) IsTableNotExistError(err error) bool { return m.Called(err).Bool(0) } // Added

// --- Test Setup ---

func setupChunkStep(t *testing.T) (*item.ChunkStep, *MockItemReader, *MockItemProcessor, *MockItemWriter, *MockJobRepository, *MockTransactionManager, *MockMetricRecorder, *MockTracer, *testutil.MockDBConnectionResolver) {
	reader := new(MockItemReader)
	processor := new(MockItemProcessor)
	writer := new(MockItemWriter)
	repo := new(MockJobRepository)
	txManager := new(MockTransactionManager)
	txManagerFactory := &MockTransactionManagerFactory{TxManager: txManager} // Create TxManagerFactory
	metricRecorder := new(MockMetricRecorder)
	tracer := new(MockTracer)
	dbConn := new(MockDBConnection) // Create MockDBConnection (implements adapter.DBConnection)

	// Create MockDBConnectionResolver instance and mock ResolveDBConnectionName and ResolveDBConnection
	dbConnResolver := new(testutil.MockDBConnectionResolver)
	// Use mock.Anything for arguments of ResolveDBConnectionName as they are now interface{}.
	dbConnResolver.On("ResolveDBConnectionName", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("mock_db", nil)
	dbConnResolver.On("ResolveDBConnection", mock.Anything, "mock_db").Return(dbConn, nil)

	// Default configurations for policies
	retryConfig := &config.RetryConfig{MaxAttempts: 3}
	itemRetryConfig := config.ItemRetryConfig{MaxAttempts: 3, RetryableExceptions: []string{"TransientError"}}
	// To allow skippable BatchError (isSkippable=true), specify "BatchError" by type name
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
		txManagerFactory, // Pass tx.TransactionManagerFactory
		metricRecorder,
		tracer,
		dbConnResolver, // Pass port.DBConnectionResolver
	)
	// Ensure the step is correctly initialized
	assert.NotNil(t, step)

	return step, reader, processor, writer, repo, txManager, metricRecorder, tracer, dbConnResolver
}

// --- Tests ---

// TestChunkStep_ChunkSplittingOnSkippableWriteFailure verifies that chunk splitting is correctly performed on chunk write failure.
// Successful items are committed, and skippable error items are skipped.
func TestChunkStep_ChunkSplittingOnSkippableWriteFailure(t *testing.T) {
	step, _, _, writer, repo, txManager, recorder, tracer, _ := setupChunkStep(t) // txManager is MockTransactionManager
	ctx := context.Background()

	// Initial Step Execution setup
	jobExec := model.NewJobExecution(model.NewID(), "testJob", model.NewJobParameters())
	se := model.NewStepExecution(model.NewID(), jobExec, "testStep")
	se.WriteCount = 10 // Assume 10 items were already written

	originalItems := []any{"item1", "item2", "item3"}

	// Define the skippable error
	skippableErr := exception.NewBatchError("testStep", "DB constraint violation", errors.New("unique constraint failed"), true, false) // S=true, R=false
	// Note: exception.AddExceptionName does not exist, so match by error message or type name here.

	// Mock Transaction Adapters for chunk splitting (one per item) - Use testutil.MockTx
	txAdapter1 := new(testutil.MockTx)
	txAdapter2 := new(testutil.MockTx)
	txAdapter3 := new(testutil.MockTx)

	// Mock JobRepository calls for StepExecution updates (Save/Update)
	repo.On("UpdateStepExecution", mock.Anything, mock.AnythingOfType("*model.StepExecution")).Return(nil).Maybe()

	// Mock Tracer Span End function (dummy)
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
	writer.On("Write", mock.Anything, []any{"item1"}).Return(nil).Once() // Updated mock expectation

	// Item 2 write: Failure (Skippable)
	writer.On("Write", mock.Anything, []any{"item2"}).Return(skippableErr).Once() // Updated mock expectation

	// Item 3 write: Success
	writer.On("Write", mock.Anything, []any{"item3"}).Return(nil).Once() // Updated mock expectation

	// 3. Listener/Metric Mocks
	// Item 2 Skip notification
	tracer.On("RecordError", mock.Anything, "testStep", mock.AnythingOfType("*exception.BatchError")).Once() // Explicit error type
	recorder.On("RecordItemSkip", mock.Anything, "testStep", "write").Once()

	// Item 1 and Item 3 success
	recorder.On("RecordItemWrite", mock.Anything, "testStep", 1).Twice()

	// --- Execution ---

	// Assuming HandleSkippableWriteFailure is public
	remainingItems, fatalErr := step.HandleSkippableWriteFailure(ctx, originalItems, se, txManager) // Pass txManager as argument

	// --- Assertions ---

	assert.Nil(t, fatalErr, "Chunk splitting should not result in a fatal error")
	assert.Empty(t, remainingItems, "All items should have been processed (committed or skipped)")

	// Item 1 (Success) + Item 3 (Success) = 2 new writes (se.WriteCount is already 10, so total 12)
	assert.Equal(t, 12, se.WriteCount, "Write count should be updated (10 initial + 2 successful commits)")

	// Item 2 (Skipped) = 1 skip
	assert.Equal(t, 1, se.SkipWriteCount, "Skip write count should be 1")

	// Verify mocks
	txManager.AssertExpectations(t)
	writer.AssertExpectations(t)
	recorder.AssertExpectations(t)
	tracer.AssertExpectations(t)
}

// TestChunkStep_ChunkSplitting_FatalError verifies that processing is interrupted if a fatal error occurs during chunk splitting.
func TestChunkStep_ChunkSplitting_FatalError(t *testing.T) {
	step, _, _, writer, repo, txManager, recorder, tracer, _ := setupChunkStep(t) // txManager is MockTransactionManager
	ctx := context.Background()

	jobExec := model.NewJobExecution(model.NewID(), "testJob", model.NewJobParameters())
	se := model.NewStepExecution(model.NewID(), jobExec, "testStep")
	originalItems := []any{"item1", "item2"}

	// Non-skippable fatal error
	fatalWriteErr := errors.New("database connection lost")

	// Mock Transaction Adapters
	txAdapter1 := new(testutil.MockTx)
	txAdapter2 := new(testutil.MockTx)

	// Mock JobRepository calls for StepExecution updates (Save/Update)
	repo.On("UpdateStepExecution", mock.Anything, mock.AnythingOfType("*model.StepExecution")).Return(nil).Maybe()

	// Mock Tracer Span End function (dummy)
	mockSpanEnd := func() {}
	tracer.On("StartJobSpan", mock.Anything, mock.Anything).Return(ctx, mockSpanEnd).Maybe()
	tracer.On("StartStepSpan", mock.Anything, mock.Anything).Return(ctx, mockSpanEnd).Maybe()

	// Item 1: Success
	txManager.On("Begin", mock.Anything, mock.Anything).Return(txAdapter1, nil).Once()
	writer.On("Write", mock.Anything, []any{"item1"}).Return(nil).Once() // Updated mock expectation
	txManager.On("Commit", txAdapter1).Return(nil).Once()
	recorder.On("RecordItemWrite", mock.Anything, "testStep", 1).Once()

	// Item 2: Failure (Fatal error during write)
	txManager.On("Begin", mock.Anything, mock.Anything).Return(txAdapter2, nil).Once()
	writer.On("Write", mock.Anything, []any{"item2"}).Return(fatalWriteErr).Once() // Updated mock expectation

	// If a fatal error occurs, the transaction for that item is rolled back.
	txManager.On("Rollback", txAdapter2).Return(nil).Once()

	// --- Execution ---

	remainingItems, resultErr := step.HandleSkippableWriteFailure(ctx, originalItems, se, txManager) // Pass txManager as argument

	// --- Assertions ---

	assert.NotNil(t, resultErr, "Fatal error should be returned")
	assert.True(t, exception.IsBatchError(resultErr), "Error should be wrapped as BatchError")
	assert.Contains(t, resultErr.Error(), "Item write failed during chunk splitting (Fatal or limit reached)", "Error message should indicate fatal failure")

	// Item 1 was committed
	assert.Equal(t, 1, se.WriteCount, "Write count should reflect committed items before fatal error")
	assert.Equal(t, 0, se.SkipWriteCount, "No skips occurred")
	assert.Empty(t, remainingItems, "Remaining items should be empty because processing was interrupted")

	// Verify mocks
	txManager.AssertExpectations(t)
	writer.AssertExpectations(t)
	recorder.AssertExpectations(t)
	tracer.AssertExpectations(t)
}
