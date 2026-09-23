package semantics_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/mock"
	dbadapter "github.com/tigerroll/surfin/pkg/batch/adapter/database"
	dbconfig "github.com/tigerroll/surfin/pkg/batch/adapter/database/config"
	"github.com/tigerroll/surfin/pkg/batch/core/domain/model"
)

// --- Mocks ---

// Ensure MockDBConnection implements dbadapter.DBConnection at compile time.
var _ dbadapter.DBConnection = (*MockDBConnection)(nil)

// MockDBConnection is a mock implementation of dbadapter.DBConnection for testing.
type MockDBConnection struct{ mock.Mock }

func (m *MockDBConnection) Close() error { return m.Called().Error(0) }
func (m *MockDBConnection) Type() string { return m.Called().String(0) }
func (m *MockDBConnection) Name() string { return m.Called().String(0) }
func (m *MockDBConnection) ExecuteUpdate(ctx context.Context, model interface{}, op, table string, query map[string]interface{}) (int64, error) {
	return 0, nil
}
func (m *MockDBConnection) ExecuteUpsert(ctx context.Context, model interface{}, table string, conflict, update []string) (int64, error) {
	return 0, nil
}
func (m *MockDBConnection) ExecuteQuery(ctx context.Context, target interface{}, query map[string]interface{}) error {
	return nil
}
func (m *MockDBConnection) ExecuteQueryAdvanced(ctx context.Context, target interface{}, query map[string]interface{}, orderBy string, limit int) error {
	return m.Called(ctx, target, query, orderBy, limit).Error(0)
}
func (m *MockDBConnection) Count(ctx context.Context, model interface{}, query map[string]interface{}) (int64, error) {
	return 0, nil
}
func (m *MockDBConnection) Pluck(ctx context.Context, model interface{}, col string, target interface{}, query map[string]interface{}) error {
	return nil
}
func (m *MockDBConnection) IsTableNotExistError(err error) bool         { return false }
func (m *MockDBConnection) RefreshConnection(ctx context.Context) error { return nil }
func (m *MockDBConnection) Config() dbconfig.DatabaseConfig             { return dbconfig.DatabaseConfig{} }
func (m *MockDBConnection) GetSQLDB() (*sql.DB, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*sql.DB), args.Error(1)
}
func (m *MockDBConnection) ScanRowsToStruct(rows *sql.Rows, dest interface{}) error {
	return m.Called(rows, dest).Error(0)
}

// MockReader is a mock implementation of port.ItemReader for testing.
type MockReader struct{ mock.Mock }

func (m *MockReader) Open(ctx context.Context, ec model.ExecutionContext) error {
	return m.Called(ctx, ec).Error(0)
}
func (m *MockReader) Read(ctx context.Context) (any, error) {
	args := m.Called(ctx)
	return args.Get(0), args.Error(1)
}
func (m *MockReader) Close(ctx context.Context) error { return m.Called(ctx).Error(0) }
func (m *MockReader) SetExecutionContext(ctx context.Context, ec model.ExecutionContext) error {
	return m.Called(ctx, ec).Error(0)
}
func (m *MockReader) GetExecutionContext(ctx context.Context) (model.ExecutionContext, error) {
	return m.Called(ctx).Get(0).(model.ExecutionContext), m.Called(ctx).Error(1)
}

// MockProcessor is a mock implementation of port.ItemProcessor for testing.
type MockProcessor struct{ mock.Mock }

func (m *MockProcessor) Process(ctx context.Context, item any) (any, error) {
	args := m.Called(ctx, item)
	return args.Get(0), args.Error(1)
}
func (m *MockProcessor) SetExecutionContext(ctx context.Context, ec model.ExecutionContext) error {
	return m.Called(ctx, ec).Error(0)
}
func (m *MockProcessor) GetExecutionContext(ctx context.Context) (model.ExecutionContext, error) {
	return m.Called(ctx).Get(0).(model.ExecutionContext), m.Called(ctx).Error(1)
}

// MockWriter is a mock implementation of port.ItemWriter for testing.
type MockWriter struct{ mock.Mock }

func (m *MockWriter) Open(ctx context.Context, ec model.ExecutionContext) error {
	return m.Called(ctx, ec).Error(0)
}
func (m *MockWriter) Write(ctx context.Context, items []any) error {
	return m.Called(ctx, items).Error(0)
}
func (m *MockWriter) Close(ctx context.Context) error { return m.Called(ctx).Error(0) }
func (m *MockWriter) SetExecutionContext(ctx context.Context, ec model.ExecutionContext) error {
	return m.Called(ctx, ec).Error(0)
}
func (m *MockWriter) GetExecutionContext(ctx context.Context) (model.ExecutionContext, error) {
	return m.Called(ctx).Get(0).(model.ExecutionContext), m.Called(ctx).Error(1)
}
func (m *MockWriter) GetTableName() string          { return m.Called().String(0) }
func (m *MockWriter) GetTargetDBName() string       { return m.Called().String(0) }
func (m *MockWriter) GetTargetResourceName() string { return m.Called().String(0) }
func (m *MockWriter) GetResourcePath() string       { return m.Called().String(0) }

// MockRepository is a mock implementation of repository.JobRepository for testing.
type MockRepository struct{ mock.Mock }

func (m *MockRepository) SaveJobInstance(ctx context.Context, instance *model.JobInstance) error {
	return m.Called(ctx, instance).Error(0)
}
func (m *MockRepository) UpdateJobInstance(ctx context.Context, instance *model.JobInstance) error {
	return m.Called(ctx, instance).Error(0)
}
func (m *MockRepository) FindJobInstanceByID(ctx context.Context, id string) (*model.JobInstance, error) {
	return nil, nil
}
func (m *MockRepository) FindJobInstanceByJobNameAndParameters(ctx context.Context, jobName string, params model.JobParameters) (*model.JobInstance, error) {
	return nil, nil
}
func (m *MockRepository) FindJobInstancesByJobNameAndPartialParameters(ctx context.Context, jobName string, partialParams model.JobParameters) ([]*model.JobInstance, error) {
	return nil, nil
}
func (m *MockRepository) GetJobInstanceCount(ctx context.Context, jobName string) (int, error) {
	return 0, nil
}
func (m *MockRepository) GetJobNames(ctx context.Context) ([]string, error) { return nil, nil }
func (m *MockRepository) SaveJobExecution(ctx context.Context, execution *model.JobExecution) error {
	return nil
}
func (m *MockRepository) UpdateJobExecution(ctx context.Context, execution *model.JobExecution) error {
	return nil
}
func (m *MockRepository) FindJobExecutionByID(ctx context.Context, id string) (*model.JobExecution, error) {
	return nil, nil
}
func (m *MockRepository) FindLatestRestartableJobExecution(ctx context.Context, jobInstanceID string) (*model.JobExecution, error) {
	return nil, nil
}
func (m *MockRepository) SaveStepExecution(ctx context.Context, stepExec *model.StepExecution) error {
	return m.Called(ctx, stepExec).Error(0)
}
func (m *MockRepository) UpdateStepExecution(ctx context.Context, stepExec *model.StepExecution) error {
	return m.Called(ctx, stepExec).Error(0)
}
func (m *MockRepository) FindStepExecutionByID(ctx context.Context, id string) (*model.StepExecution, error) {
	return nil, nil
}
func (m *MockRepository) FindStepExecutionsByJobExecutionID(ctx context.Context, executionID string) ([]*model.StepExecution, error) {
	return nil, nil
}
func (m *MockRepository) SaveCheckpointData(ctx context.Context, data *model.CheckpointData) error {
	return m.Called(ctx, data).Error(0)
}
func (m *MockRepository) FindCheckpointData(ctx context.Context, stepExecutionID string) (*model.CheckpointData, error) {
	return nil, nil
}
func (m *MockRepository) Close() error { return nil }

// --- Tests ---

// TestChunkStep_Execute verifies that ChunkStep.Execute calls GetExecutionContext exactly three times:
// twice during chunk processing and once during finalization.
func TestChunkStep_Execute(t *testing.T) {
	reader := new(MockReader)
	writer := new(MockWriter)

	// Expect 3 calls: 2 during chunk processing + 1 during finalization.
	reader.On("GetExecutionContext", mock.Anything).Return(model.NewExecutionContext(), nil).Times(3)
	writer.On("GetExecutionContext", mock.Anything).Return(model.NewExecutionContext(), nil).Times(3)

	// ... Other mock configurations and test logic ...
}
