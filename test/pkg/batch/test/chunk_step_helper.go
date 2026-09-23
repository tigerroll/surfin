package test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	dbconfig "github.com/tigerroll/surfin/pkg/batch/adapter/database/config"
	coreadapter "github.com/tigerroll/surfin/pkg/batch/core/adapter"
	"github.com/tigerroll/surfin/pkg/batch/core/config"
	"github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	"github.com/tigerroll/surfin/pkg/batch/core/tx"
	"github.com/tigerroll/surfin/pkg/batch/engine/step/item"
	"go.opentelemetry.io/otel/attribute"
)

// --- Mocks ---

type MockTx struct {
	mock.Mock
}

func (m *MockTx) Commit() error {
	return m.Called().Error(0)
}

func (m *MockTx) Rollback() error {
	return m.Called().Error(0)
}

func (m *MockTx) Savepoint(name string) error {
	return m.Called(name).Error(0)
}

func (m *MockTx) RollbackToSavepoint(name string) error {
	return m.Called(name).Error(0)
}

func (m *MockTx) ExecuteUpdate(ctx context.Context, model interface{}, op, table string, query map[string]interface{}) (int64, error) {
	args := m.Called(ctx, model, op, table, query)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockTx) ExecuteUpsert(ctx context.Context, model interface{}, table string, conflict, update []string) (int64, error) {
	args := m.Called(ctx, model, table, conflict, update)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockTx) ExecuteQuery(ctx context.Context, target interface{}, query map[string]interface{}) error {
	return m.Called(ctx, target, query).Error(0)
}

func (m *MockTx) Count(ctx context.Context, model interface{}, query map[string]interface{}) (int64, error) {
	args := m.Called(ctx, model, query)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockTx) Pluck(ctx context.Context, model interface{}, col string, target interface{}, query map[string]interface{}) error {
	return m.Called(ctx, model, col, target, query).Error(0)
}

func (m *MockTx) IsTableNotExistError(err error) bool {
	return m.Called(err).Bool(0)
}

func (m *MockTx) Close() error {
	return m.Called().Error(0)
}

func (m *MockTx) Type() string {
	return m.Called().String(0)
}

func (m *MockTx) Name() string {
	return m.Called().String(0)
}

func (m *MockTx) RefreshConnection(ctx context.Context) error {
	return m.Called(ctx).Error(0)
}

func (m *MockTx) Config() dbconfig.DatabaseConfig {
	args := m.Called()
	return args.Get(0).(dbconfig.DatabaseConfig)
}

type MockItemReader struct {
	mock.Mock
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
func (m *MockItemReader) Update(ctx context.Context, ec model.ExecutionContext) error {
	return m.Called(ctx, ec).Error(0)
}

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
func (m *MockItemProcessor) Update(ctx context.Context, ec model.ExecutionContext) error {
	return m.Called(ctx, ec).Error(0)
}

type MockItemWriter struct {
	mock.Mock
}

func (m *MockItemWriter) Write(ctx context.Context, items []any) error {
	args := m.Called(ctx, items)
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
func (m *MockItemWriter) Update(ctx context.Context, ec model.ExecutionContext) error {
	return m.Called(ctx, ec).Error(0)
}
func (m *MockItemWriter) GetTableName() string          { return m.Called().String(0) }
func (m *MockItemWriter) GetTargetDBName() string       { return m.Called().String(0) }
func (m *MockItemWriter) GetTargetResourceName() string { return m.Called().String(0) }
func (m *MockItemWriter) GetResourcePath() string       { return m.Called().String(0) }

type MockJobRepository struct {
	mock.Mock
	*MockJobExecutionRepository
	*MockStepExecutionRepository
}

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
func (m *MockJobRepository) Close() error { return m.Called().Error(0) }

// Delegated methods for JobExecutionRepository
func (m *MockJobRepository) SaveJobExecution(ctx context.Context, execution *model.JobExecution) error {
	return m.MockJobExecutionRepository.SaveJobExecution(ctx, execution)
}
func (m *MockJobRepository) UpdateJobExecution(ctx context.Context, execution *model.JobExecution) error {
	return m.MockJobExecutionRepository.UpdateJobExecution(ctx, execution)
}
func (m *MockJobRepository) FindJobExecutionByID(ctx context.Context, id string) (*model.JobExecution, error) {
	return m.MockJobExecutionRepository.FindJobExecutionByID(ctx, id)
}
func (m *MockJobRepository) FindJobExecutionsByJobInstance(ctx context.Context, instance *model.JobInstance) ([]*model.JobExecution, error) {
	return m.MockJobExecutionRepository.FindJobExecutionsByJobInstance(ctx, instance)
}
func (m *MockJobRepository) FindLatestRestartableJobExecution(ctx context.Context, instanceID string) (*model.JobExecution, error) {
	return m.MockJobExecutionRepository.FindLatestRestartableJobExecution(ctx, instanceID)
}

// Delegated methods for StepExecutionRepository
func (m *MockJobRepository) FindStepExecutionsByJobExecutionID(ctx context.Context, executionID string) ([]*model.StepExecution, error) {
	return m.MockStepExecutionRepository.FindStepExecutionsByJobExecutionID(ctx, executionID)
}

type MockJobExecutionRepository struct {
	mock.Mock
}

func (m *MockJobExecutionRepository) SaveJobExecution(ctx context.Context, execution *model.JobExecution) error {
	return m.Called(ctx, execution).Error(0)
}
func (m *MockJobExecutionRepository) UpdateJobExecution(ctx context.Context, execution *model.JobExecution) error {
	return m.Called(ctx, execution).Error(0)
}
func (m *MockJobExecutionRepository) FindJobExecutionByID(ctx context.Context, id string) (*model.JobExecution, error) {
	args := m.Called(ctx, id)
	if res, ok := args.Get(0).(*model.JobExecution); ok {
		return res, args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *MockJobExecutionRepository) FindJobExecutionsByJobInstance(ctx context.Context, instance *model.JobInstance) ([]*model.JobExecution, error) {
	args := m.Called(ctx, instance)
	if res, ok := args.Get(0).([]*model.JobExecution); ok {
		return res, args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *MockJobExecutionRepository) FindLatestRestartableJobExecution(ctx context.Context, jobInstanceID string) (*model.JobExecution, error) {
	args := m.Called(ctx, jobInstanceID)
	if res, ok := args.Get(0).(*model.JobExecution); ok {
		return res, args.Error(1)
	}
	return nil, args.Error(1)
}

type MockStepExecutionRepository struct {
	mock.Mock
}

func (m *MockStepExecutionRepository) SaveStepExecution(ctx context.Context, stepExec *model.StepExecution) error {
	return m.Called(ctx, stepExec).Error(0)
}
func (m *MockStepExecutionRepository) UpdateStepExecution(ctx context.Context, stepExec *model.StepExecution) error {
	return m.Called(ctx, stepExec).Error(0)
}
func (m *MockStepExecutionRepository) FindStepExecutionByID(ctx context.Context, id string) (*model.StepExecution, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.StepExecution), args.Error(1)
}
func (m *MockStepExecutionRepository) FindStepExecutionsByJobExecutionID(ctx context.Context, executionID string) ([]*model.StepExecution, error) {
	args := m.Called(ctx, executionID)
	if res, ok := args.Get(0).([]*model.StepExecution); ok {
		return res, args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *MockStepExecutionRepository) SaveCheckpointData(ctx context.Context, data *model.CheckpointData) error {
	return m.Called(ctx, data).Error(0)
}
func (m *MockStepExecutionRepository) FindCheckpointData(ctx context.Context, stepExecutionID string) (*model.CheckpointData, error) {
	args := m.Called(ctx, stepExecutionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.CheckpointData), args.Error(1)
}

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
func (m *MockMetricRecorder) RecordItemRead(ctx context.Context, stepExecution *model.StepExecution, count int64) {
	m.Called(ctx, stepExecution, count)
}
func (m *MockMetricRecorder) RecordItemProcess(ctx context.Context, stepExecution *model.StepExecution, count int64) {
	m.Called(ctx, stepExecution, count)
}
func (m *MockMetricRecorder) RecordItemWrite(ctx context.Context, stepExecution *model.StepExecution, count int64) {
	m.Called(ctx, stepExecution, count)
}
func (m *MockMetricRecorder) RecordItemSkip(ctx context.Context, stepExecution *model.StepExecution, err error) {
	m.Called(ctx, stepExecution, err)
}
func (m *MockMetricRecorder) RecordItemRetry(ctx context.Context, stepExecution *model.StepExecution, err error) {
	m.Called(ctx, stepExecution, err)
}
func (m *MockMetricRecorder) RecordChunkCommit(ctx context.Context, stepExecution *model.StepExecution, count int64) {
	m.Called(ctx, stepExecution, count)
}
func (m *MockMetricRecorder) RecordDuration(ctx context.Context, name string, duration float64, attrs ...attribute.KeyValue) {
	m.Called(ctx, name, duration, attrs)
}
func (m *MockMetricRecorder) RecordExecutionError(ctx context.Context, err error) {
	m.Called(ctx, err)
}

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

type MockDBConnection struct {
	mock.Mock
}

func (m *MockDBConnection) Close() error { return m.Called().Error(0) }
func (m *MockDBConnection) Type() string { return m.Called().String(0) }
func (m *MockDBConnection) Name() string { return m.Called().String(0) }
func (m *MockDBConnection) RefreshConnection(ctx context.Context) error {
	return m.Called(ctx).Error(0)
}
func (m *MockDBConnection) Config() dbconfig.DatabaseConfig {
	args := m.Called()
	return args.Get(0).(dbconfig.DatabaseConfig)
}
func (m *MockDBConnection) GetSQLDB() (*sql.DB, error) {
	args := m.Called()
	return args.Get(0).(*sql.DB), args.Error(1)
}
func (m *MockDBConnection) ExecuteQuery(ctx context.Context, target interface{}, query map[string]interface{}) error {
	return m.Called(ctx, target, query).Error(0)
}
func (m *MockDBConnection) ExecuteQueryAdvanced(ctx context.Context, target interface{}, query map[string]interface{}, orderBy string, limit int) error {
	return m.Called(ctx, target, query, orderBy, limit).Error(0)
}
func (m *MockDBConnection) Count(ctx context.Context, model interface{}, query map[string]interface{}) (int64, error) {
	args := m.Called(ctx, model, query)
	return args.Get(0).(int64), args.Error(1)
}
func (m *MockDBConnection) Pluck(ctx context.Context, model interface{}, column string, target interface{}, query map[string]interface{}) error {
	return m.Called(ctx, model, column, target, query).Error(0)
}
func (m *MockDBConnection) ExecuteUpdate(ctx context.Context, model interface{}, operation string, tableName string, query map[string]interface{}) (rowsAffected int64, err error) {
	args := m.Called(ctx, model, operation, tableName, query)
	return args.Get(0).(int64), args.Error(1)
}
func (m *MockDBConnection) ExecuteUpsert(ctx context.Context, model interface{}, tableName string, conflictColumns []string, updateColumns []string) (rowsAffected int64, err error) {
	args := m.Called(ctx, model, tableName, conflictColumns, updateColumns)
	return args.Get(0).(int64), args.Error(1)
}
func (m *MockDBConnection) IsTableNotExistError(err error) bool { return m.Called(err).Bool(0) }
func (m *MockDBConnection) ScanRowsToStruct(rows *sql.Rows, dest interface{}) error {
	return m.Called(rows, dest).Error(0)
}

type MockDBConnectionResolver struct {
	mock.Mock
}

func (m *MockDBConnectionResolver) ResolveConnectionName(ctx context.Context, jobExecution interface{}, stepExecution interface{}, defaultName string) (string, error) {
	args := m.Called(ctx, jobExecution, stepExecution, defaultName)
	return args.String(0), args.Error(1)
}
func (m *MockDBConnectionResolver) ResolveConnection(ctx context.Context, name string) (coreadapter.ResourceConnection, error) {
	args := m.Called(ctx, name)
	return args.Get(0).(coreadapter.ResourceConnection), args.Error(1)
}

func SetupChunkStep(t *testing.T) (*item.ChunkStep, *MockItemReader, *MockItemProcessor, *MockItemWriter, *MockJobRepository, *MockTransactionManager, *MockMetricRecorder, *MockTracer, *MockDBConnectionResolver, *MockDBConnection) {
	reader := new(MockItemReader)
	processor := new(MockItemProcessor)
	writer := new(MockItemWriter)
	repo := &MockJobRepository{
		MockJobExecutionRepository:  new(MockJobExecutionRepository),
		MockStepExecutionRepository: new(MockStepExecutionRepository),
	}
	txManager := new(MockTransactionManager)
	txManagerFactory := &MockTransactionManagerFactory{TxManager: txManager}
	metricRecorder := new(MockMetricRecorder)
	tracer := new(MockTracer)
	dbConn := new(MockDBConnection)

	dbConnResolver := new(MockDBConnectionResolver)
	dbConnResolver.On("ResolveConnectionName", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("mock_db", nil)
	dbConnResolver.On("ResolveConnection", mock.Anything, "mock_db").Return(dbConn, nil)

	retryConfig := &config.RetryConfig{MaxAttempts: 3}
	itemRetryConfig := config.ItemRetryConfig{MaxAttempts: 3, RetryableExceptions: []string{"TransientError"}}
	itemSkipConfig := config.ItemSkipConfig{SkipLimit: 5, SkippableExceptions: []string{"BatchError"}}

	step := item.NewJSLAdaptedStep(
		"testStep",
		reader,
		processor,
		writer,
		10,
		10,
		retryConfig,
		itemRetryConfig,
		itemSkipConfig,
		repo,
		nil, nil, nil, nil, nil, nil, nil,
		model.NewExecutionContextPromotion(),
		"SERIALIZABLE",
		"REQUIRED",
		txManagerFactory,
		metricRecorder,
		tracer,
		dbConnResolver,
	)
	assert.NotNil(t, step)

	return step, reader, processor, writer, repo, txManager, metricRecorder, tracer, dbConnResolver, dbConn
}

type MockTransactionManagerFactory struct {
	mock.Mock
	TxManager *MockTransactionManager
}

func (m *MockTransactionManagerFactory) NewTransactionManager(conn coreadapter.ResourceConnection) tx.TransactionManager {
	return m.TxManager
}
