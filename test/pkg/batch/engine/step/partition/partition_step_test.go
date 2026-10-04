package partition_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	port "github.com/tigerroll/surfin/pkg/batch/core/application/port"
	model "github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	metrics "github.com/tigerroll/surfin/pkg/batch/core/metrics"
	partition "github.com/tigerroll/surfin/pkg/batch/engine/step/partition"
	testutil "github.com/tigerroll/surfin/pkg/batch/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// --- Mock Implementations ---

// MockStepExecutor is a mock implementation of port.StepExecutor for testing purposes.
type MockStepExecutor struct {
	// Results holds the execution results for each partition name.
	Results map[string]struct {
		Err        error
		Status     model.JobStatus
		EC         model.ExecutionContext
		ReadCount  int
		WriteCount int
	}
}

// ExecuteStep simulates the execution of a Worker Step and updates the StepExecution state.
func (m *MockStepExecutor) ExecuteStep(ctx context.Context, step port.Step, jobExecution *model.JobExecution, stepExecution *model.StepExecution) (*model.StepExecution, error) {
	// Retrieve the partition name from the ExecutionContext to determine the result.
	pName, _ := stepExecution.ExecutionContext.GetString("partition.name")
	res, ok := m.Results[pName]
	if !ok {
		// Default: Success.
		stepExecution.Status = model.BatchStatusCompleted
		stepExecution.ExitStatus = model.ExitStatusCompleted
		return stepExecution, nil
	}

	// 統計情報はエラーの有無に関わらず設定する
	stepExecution.ReadCount = res.ReadCount
	stepExecution.WriteCount = res.WriteCount
	stepExecution.ExecutionContext = res.EC

	// エラー発生時はエラーを返すのみ。状態遷移は PartitionStep に任せる
	if res.Err != nil {
		return stepExecution, res.Err
	}

	// Handle success cases.
	stepExecution.Status = res.Status
	stepExecution.ExitStatus = model.ExitStatus(res.Status.String())

	return stepExecution, nil
}

// MockJobInstanceRepository is a mock implementation of repository.JobInstance.
type MockJobInstanceRepository struct {
	mock.Mock
}

func (m *MockJobInstanceRepository) SaveJobInstance(ctx context.Context, instance *model.JobInstance) error {
	return m.Called(ctx, instance).Error(0)
}
func (m *MockJobInstanceRepository) UpdateJobInstance(ctx context.Context, instance *model.JobInstance) error {
	return m.Called(ctx, instance).Error(0)
}
func (m *MockJobInstanceRepository) FindJobInstanceByJobNameAndParameters(ctx context.Context, jobName string, params model.JobParameters) (*model.JobInstance, error) {
	args := m.Called(ctx, jobName, params)
	if res, ok := args.Get(0).(*model.JobInstance); ok {
		return res, args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *MockJobInstanceRepository) FindJobInstanceByID(ctx context.Context, instanceID string) (*model.JobInstance, error) {
	args := m.Called(ctx, instanceID)
	if res, ok := args.Get(0).(*model.JobInstance); ok {
		return res, args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *MockJobInstanceRepository) GetJobInstanceCount(ctx context.Context, jobName string) (int, error) {
	args := m.Called(ctx, jobName)
	return args.Int(0), args.Error(1)
}
func (m *MockJobInstanceRepository) GetJobNames(ctx context.Context) ([]string, error) {
	args := m.Called(ctx)
	if res, ok := args.Get(0).([]string); ok {
		return res, args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *MockJobInstanceRepository) FindJobInstancesByJobNameAndPartialParameters(ctx context.Context, jobName string, partialParams model.JobParameters) ([]*model.JobInstance, error) {
	args := m.Called(ctx, jobName, partialParams)
	if res, ok := args.Get(0).([]*model.JobInstance); ok {
		return res, args.Error(1)
	}
	return nil, args.Error(1)
}

// MockJobExecutionRepository is a mock implementation of repository.JobExecution.
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
func (m *MockJobExecutionRepository) FindLatestRestartableJobExecution(ctx context.Context, instanceID string) (*model.JobExecution, error) {
	args := m.Called(ctx, instanceID)
	if res, ok := args.Get(0).(*model.JobExecution); ok {
		return res, args.Error(1)
	}
	return nil, args.Error(1)
}

// MockStepExecutionRepository is a mock implementation of repository.StepExecution and repository.CheckpointDataRepository.
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
	if res, ok := args.Get(0).(*model.StepExecution); ok {
		return res, args.Error(1)
	}
	return nil, args.Error(1)
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
	if res, ok := args.Get(0).(*model.CheckpointData); ok {
		return res, args.Error(1)
	}
	return nil, args.Error(1)
}

// MockJobRepository is a composite mock implementation of the JobRepository interface.
type MockJobRepository struct {
	*MockJobInstanceRepository
	*MockJobExecutionRepository
	*MockStepExecutionRepository
}

// NewMockJobRepository creates a new instance of MockJobRepository.
func NewMockJobRepository() *MockJobRepository {
	return &MockJobRepository{
		MockJobInstanceRepository:   &MockJobInstanceRepository{},
		MockJobExecutionRepository:  &MockJobExecutionRepository{},
		MockStepExecutionRepository: &MockStepExecutionRepository{},
	}
}

// Close is a no-op implementation for the repository interface.
func (m *MockJobRepository) Close() error {
	return nil
}

// MockStep is a minimal implementation of the port.Step interface for testing purposes.
type MockStep struct {
	IDValue string
}

func (m *MockStep) ID() string       { return m.IDValue }
func (m *MockStep) StepName() string { return m.IDValue }
func (m *MockStep) Execute(ctx context.Context, jobExecution *model.JobExecution, stepExecution *model.StepExecution) error {
	return nil
}
func (m *MockStep) GetExecutionContextPromotion() *model.ExecutionContextPromotion { return nil }
func (m *MockStep) GetTransactionOptions() *sql.TxOptions                          { return nil }
func (m *MockStep) GetPropagation() string                                         { return "" }
func (m *MockStep) SetMetricRecorder(recorder metrics.MetricRecorder)              {}
func (m *MockStep) SetTracer(tracer metrics.Tracer)                                {}

// MockPartitioner is a mock implementation of port.Partitioner.
type MockPartitioner struct {
	mock.Mock
	Partitions map[string]model.ExecutionContext
}

// Partition returns the configured partitions or an error.
func (m *MockPartitioner) Partition(ctx context.Context, gridSize int) (map[string]model.ExecutionContext, error) {
	args := m.Called(ctx, gridSize)
	if m.Partitions != nil {
		return m.Partitions, args.Error(1)
	}
	if res, ok := args.Get(0).(map[string]model.ExecutionContext); ok {
		return res, args.Error(1)
	}
	return nil, args.Error(1)
}

// --- Test Cases ---

// TestPartitionStep_Aggregation verifies that the PartitionStep correctly aggregates
// results from multiple worker partitions, including statistics (ReadCount, WriteCount)
// and the ExecutionContext.
func TestPartitionStep_Aggregation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 1. Setup
	mockRepo := NewMockJobRepository()
	mockExecutor := &MockStepExecutor{
		Results: map[string]struct {
			Err        error
			Status     model.JobStatus
			EC         model.ExecutionContext
			ReadCount  int
			WriteCount int
		}{
			"partition0": {nil, model.BatchStatusCompleted, model.ExecutionContext{"p0.status": "OK", "p0.count": 10, "start": 0}, 10, 5},
			"partition1": {errors.New("worker 1 failed"), model.BatchStatusFailed, model.ExecutionContext{"p1.count": 5, "p1.status": "ERROR", "start": 100}, 5, 0},
			"partition2": {nil, model.BatchStatusCompleted, model.ExecutionContext{"p2.status": "OK", "extra": "data", "start": 200}, 20, 15},
		},
	}

	// Worker Step (dummy)
	workerStep := &MockStep{IDValue: "workerStep"}

	// Partitioner (returns 3 partitions)
	mockPartitioner := &MockPartitioner{
		Partitions: map[string]model.ExecutionContext{
			"partition0": testutil.NewTestExecutionContext(map[string]interface{}{"start": 0, "partition.name": "partition0"}),
			"partition1": testutil.NewTestExecutionContext(map[string]interface{}{"start": 100, "partition.name": "partition1"}),
			"partition2": testutil.NewTestExecutionContext(map[string]interface{}{"start": 200, "partition.name": "partition2"}),
		},
	}

	// Controller Step Execution
	jobExecution := testutil.NewTestJobExecution("jobInstID", "partitionJob", model.NewJobParameters())
	controllerExecution := testutil.NewTestStepExecution(jobExecution, "controllerStep")

	// Promotion settings (for EC aggregation verification)
	promotion := &model.ExecutionContextPromotion{
		Keys:         []string{"p1.count", "p2.status"},
		JobLevelKeys: map[string]string{"p0.status": "job.p0_status"},
	}

	// Create PartitionStep
	partitionStep := partition.NewPartitionStep(
		"controllerStep",
		mockPartitioner,
		workerStep,
		3, // gridSize
		1, // concurrency
		mockRepo,
		[]port.StepExecutionListener{},
		promotion,
		mockExecutor,
	)

	// 2. Set Mock Expectations

	// Partitioner.Partition expectations
	mockPartitioner.On("Partition", mock.Anything, 3).Return(nil, nil).Once()

	// JobRepository expectations (for Controller StepExecution updates)
	// Controller: STARTED (1 time), FAILED (1 time)
	mockRepo.MockStepExecutionRepository.On("UpdateStepExecution", mock.Anything, mock.AnythingOfType("*model.StepExecution")).Return(nil).Times(2)
	mockRepo.MockStepExecutionRepository.On("SaveStepExecution", mock.Anything, mock.AnythingOfType("*model.StepExecution")).Return(nil).Times(3)

	// 3. Execute
	err := partitionStep.Execute(ctx, jobExecution, controllerExecution)

	// 4. Assertions

	// 4.1. Verify execution results
	assert.Error(t, err) // Returns an error because Worker 1 failed
	assert.Contains(t, err.Error(), "one or more partitions failed")
	assert.Equal(t, model.BatchStatusFailed, controllerExecution.Status)
	assert.Equal(t, model.ExitStatusFailed, controllerExecution.ExitStatus)

	// 4.2. Verify aggregation of statistics
	assert.Equal(t, 35, controllerExecution.ReadCount, "ReadCount should be aggregated (10 + 5 + 20)")
	assert.Equal(t, 20, controllerExecution.WriteCount, "WriteCount should be aggregated (5 + 0 + 15)")

	// 4.3. Verify aggregation of ExecutionContext
	actualEC := controllerExecution.ExecutionContext
	startVal, startOk := actualEC["start"]
	delete(actualEC, "start") // Remove 'start' for deterministic comparison

	expectedEC := model.ExecutionContext{
		"p0.status": "OK",
		"p0.count":  10,
		"p1.count":  5,
		"p1.status": "ERROR",
		"p2.status": "OK",
		"extra":     "data",
	}

	assert.Equal(t, expectedEC, actualEC, "Controller EC should contain merged worker ECs (excluding 'start')")
	assert.True(t, startOk, "'start' key should be present in ExecutionContext")
	assert.Contains(t, []interface{}{0, 100, 200}, startVal, "'start' value should be one of the partition start values")

	// 4.4. Verify promotion to Job ExecutionContext
	p1Count, ok := jobExecution.ExecutionContext.GetNested("p1.count")
	assert.True(t, ok)
	assert.Equal(t, 5, p1Count)

	p2Status, ok := jobExecution.ExecutionContext.GetNested("p2.status")
	assert.True(t, ok)
	assert.Equal(t, "OK", p2Status)

	jobP0Status, ok := jobExecution.ExecutionContext.GetNested("job.p0_status")
	assert.True(t, ok)
	assert.Equal(t, "OK", jobP0Status)

	// 5. Verify mocks
	mockPartitioner.AssertExpectations(t)
	mockRepo.MockStepExecutionRepository.AssertExpectations(t)
}

// TestPartitionStep_PartialFailure verifies that if one partition fails, the controller step fails.
func TestPartitionStep_PartialFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mockRepo := NewMockJobRepository()
	mockExecutor := &MockStepExecutor{
		Results: map[string]struct {
			Err        error
			Status     model.JobStatus
			EC         model.ExecutionContext
			ReadCount  int
			WriteCount int
		}{
			"p0": {nil, model.BatchStatusCompleted, model.ExecutionContext{}, 1, 1},
			"p1": {errors.New("fail"), model.BatchStatusFailed, model.ExecutionContext{}, 0, 0},
		},
	}
	workerStep := &MockStep{IDValue: "workerStep"}
	mockPartitioner := &MockPartitioner{
		Partitions: map[string]model.ExecutionContext{
			"p0": testutil.NewTestExecutionContext(map[string]interface{}{"partition.name": "p0"}),
			"p1": testutil.NewTestExecutionContext(map[string]interface{}{"partition.name": "p1"}),
		},
	}
	jobExecution := testutil.NewTestJobExecution("jobInstID", "partitionJob", model.NewJobParameters())
	controllerExecution := testutil.NewTestStepExecution(jobExecution, "controllerStep")

	partitionStep := partition.NewPartitionStep(
		"controllerStep",
		mockPartitioner,
		workerStep,
		2,
		2,
		mockRepo,
		[]port.StepExecutionListener{},
		nil,
		mockExecutor,
	)

	mockPartitioner.On("Partition", mock.Anything, 2).Return(map[string]model.ExecutionContext{
		"p0": testutil.NewTestExecutionContext(map[string]interface{}{"partition.name": "p0"}),
		"p1": testutil.NewTestExecutionContext(map[string]interface{}{"partition.name": "p1"}),
	}, nil).Once()
	mockRepo.MockStepExecutionRepository.On("UpdateStepExecution", mock.Anything, mock.AnythingOfType("*model.StepExecution")).Return(nil).Maybe()
	mockRepo.MockStepExecutionRepository.On("SaveStepExecution", mock.Anything, mock.AnythingOfType("*model.StepExecution")).Return(nil).Times(2)

	err := partitionStep.Execute(ctx, jobExecution, controllerExecution)

	assert.Error(t, err)
	assert.Equal(t, model.BatchStatusFailed, controllerExecution.Status)
	mockRepo.MockStepExecutionRepository.AssertExpectations(t)
}

// TestPartitionStep_Cancellation verifies that worker cancellation
// results in a CANCELLED worker and a STOPPED controller.
func TestPartitionStep_Cancellation(t *testing.T) {
	// キャンセルされたコンテキストではなく、有効なコンテキストを使用する
	ctx := context.Background()

	mockRepo := NewMockJobRepository()
	mockExecutor := &MockStepExecutor{
		Results: map[string]struct {
			Err        error
			Status     model.JobStatus
			EC         model.ExecutionContext
			ReadCount  int
			WriteCount int
		}{
			"p0": {context.Canceled, model.BatchStatusCancelled, model.ExecutionContext{}, 0, 0},
		},
	}
	workerStep := &MockStep{IDValue: "workerStep"}
	mockPartitioner := &MockPartitioner{
		Partitions: map[string]model.ExecutionContext{
			"p0": testutil.NewTestExecutionContext(map[string]interface{}{"partition.name": "p0"}),
		},
	}
	jobExecution := testutil.NewTestJobExecution("jobInstID", "partitionJob", model.NewJobParameters())
	controllerExecution := testutil.NewTestStepExecution(jobExecution, "controllerStep")

	partitionStep := partition.NewPartitionStep(
		"controllerStep",
		mockPartitioner,
		workerStep,
		1,
		1,
		mockRepo,
		[]port.StepExecutionListener{},
		nil,
		mockExecutor,
	)

	// Expect UpdateStepExecution to be called twice (once for STARTED, once for final state)
	mockRepo.MockStepExecutionRepository.On("UpdateStepExecution", mock.Anything, mock.AnythingOfType("*model.StepExecution")).Return(nil).Times(2)
	mockRepo.MockStepExecutionRepository.On("SaveStepExecution", mock.Anything, mock.AnythingOfType("*model.StepExecution")).Return(nil).Once()

	// Configure Partitioner to return partitions
	mockPartitioner.On("Partition", mock.Anything, 1).Return(map[string]model.ExecutionContext{
		"p0": testutil.NewTestExecutionContext(map[string]interface{}{"partition.name": "p0"}),
	}, nil).Once()

	err := partitionStep.Execute(ctx, jobExecution, controllerExecution)

	assert.NoError(t, err) // Controller は STOPPED として正常終了する
	assert.Equal(t, model.BatchStatusStopped, controllerExecution.Status)
	assert.Equal(t, model.ExitStatusStopped, controllerExecution.ExitStatus)
	mockRepo.MockStepExecutionRepository.AssertExpectations(t)
}

// TestPartitionStep_MultipleWorkerFailures verifies that if multiple partitions fail, the controller step fails and errors are aggregated.
func TestPartitionStep_MultipleWorkerFailures(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mockRepo := NewMockJobRepository()
	mockExecutor := &MockStepExecutor{
		Results: map[string]struct {
			Err        error
			Status     model.JobStatus
			EC         model.ExecutionContext
			ReadCount  int
			WriteCount int
		}{
			"p0": {nil, model.BatchStatusCompleted, model.ExecutionContext{}, 1, 1},
			"p1": {errors.New("fail1"), model.BatchStatusFailed, model.ExecutionContext{}, 0, 0},
			"p2": {errors.New("fail2"), model.BatchStatusFailed, model.ExecutionContext{}, 0, 0},
		},
	}
	workerStep := &MockStep{IDValue: "workerStep"}
	mockPartitioner := &MockPartitioner{
		Partitions: map[string]model.ExecutionContext{
			"p0": testutil.NewTestExecutionContext(map[string]interface{}{"partition.name": "p0"}),
			"p1": testutil.NewTestExecutionContext(map[string]interface{}{"partition.name": "p1"}),
			"p2": testutil.NewTestExecutionContext(map[string]interface{}{"partition.name": "p2"}),
		},
	}
	jobExecution := testutil.NewTestJobExecution("jobInstID", "partitionJob", model.NewJobParameters())
	controllerExecution := testutil.NewTestStepExecution(jobExecution, "controllerStep")

	partitionStep := partition.NewPartitionStep(
		"controllerStep",
		mockPartitioner,
		workerStep,
		3,
		3,
		mockRepo,
		[]port.StepExecutionListener{},
		nil,
		mockExecutor,
	)

	mockPartitioner.On("Partition", mock.Anything, 3).Return(map[string]model.ExecutionContext{
		"p0": testutil.NewTestExecutionContext(map[string]interface{}{"partition.name": "p0"}),
		"p1": testutil.NewTestExecutionContext(map[string]interface{}{"partition.name": "p1"}),
		"p2": testutil.NewTestExecutionContext(map[string]interface{}{"partition.name": "p2"}),
	}, nil).Once()
	mockRepo.MockStepExecutionRepository.On("UpdateStepExecution", mock.Anything, mock.AnythingOfType("*model.StepExecution")).Return(nil).Maybe()
	mockRepo.MockStepExecutionRepository.On("SaveStepExecution", mock.Anything, mock.AnythingOfType("*model.StepExecution")).Return(nil).Times(3)

	err := partitionStep.Execute(ctx, jobExecution, controllerExecution)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "fail1")
	assert.Contains(t, err.Error(), "fail2")
	assert.Equal(t, model.BatchStatusFailed, controllerExecution.Status)
	mockRepo.MockStepExecutionRepository.AssertExpectations(t)
}

// TestPartitionStep_MixedFailureAndCancellation verifies that if FAILED and CANCELLED are mixed,
// FAILED takes precedence and the controller step fails.
func TestPartitionStep_MixedFailureAndCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mockRepo := NewMockJobRepository()
	mockExecutor := &MockStepExecutor{
		Results: map[string]struct {
			Err        error
			Status     model.JobStatus
			EC         model.ExecutionContext
			ReadCount  int
			WriteCount int
		}{
			"p0": {nil, model.BatchStatusCompleted, model.ExecutionContext{}, 1, 1},
			"p1": {errors.New("fail"), model.BatchStatusFailed, model.ExecutionContext{}, 0, 0},
			"p2": {nil, model.BatchStatusCancelled, model.ExecutionContext{}, 0, 0},
		},
	}
	workerStep := &MockStep{IDValue: "workerStep"}
	mockPartitioner := &MockPartitioner{
		Partitions: map[string]model.ExecutionContext{
			"p0": testutil.NewTestExecutionContext(map[string]interface{}{"partition.name": "p0"}),
			"p1": testutil.NewTestExecutionContext(map[string]interface{}{"partition.name": "p1"}),
			"p2": testutil.NewTestExecutionContext(map[string]interface{}{"partition.name": "p2"}),
		},
	}
	jobExecution := testutil.NewTestJobExecution("jobInstID", "partitionJob", model.NewJobParameters())
	controllerExecution := testutil.NewTestStepExecution(jobExecution, "controllerStep")

	partitionStep := partition.NewPartitionStep(
		"controllerStep",
		mockPartitioner,
		workerStep,
		3,
		3,
		mockRepo,
		[]port.StepExecutionListener{},
		nil,
		mockExecutor,
	)

	mockPartitioner.On("Partition", mock.Anything, 3).Return(map[string]model.ExecutionContext{
		"p0": testutil.NewTestExecutionContext(map[string]interface{}{"partition.name": "p0"}),
		"p1": testutil.NewTestExecutionContext(map[string]interface{}{"partition.name": "p1"}),
		"p2": testutil.NewTestExecutionContext(map[string]interface{}{"partition.name": "p2"}),
	}, nil).Once()
	mockRepo.MockStepExecutionRepository.On("UpdateStepExecution", mock.Anything, mock.AnythingOfType("*model.StepExecution")).Return(nil).Maybe()
	mockRepo.MockStepExecutionRepository.On("SaveStepExecution", mock.Anything, mock.AnythingOfType("*model.StepExecution")).Return(nil).Times(3)

	err := partitionStep.Execute(ctx, jobExecution, controllerExecution)

	assert.Error(t, err)
	assert.Equal(t, model.BatchStatusFailed, controllerExecution.Status)
	assert.Equal(t, model.ExitStatusFailed, controllerExecution.ExitStatus)
	mockRepo.MockStepExecutionRepository.AssertExpectations(t)
}

// TestPartitionStep_WorkerTimeout verifies that if a worker times out, the controller step fails.
func TestPartitionStep_WorkerTimeout(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mockRepo := NewMockJobRepository()
	mockExecutor := &MockStepExecutor{
		Results: map[string]struct {
			Err        error
			Status     model.JobStatus
			EC         model.ExecutionContext
			ReadCount  int
			WriteCount int
		}{
			"p0": {context.DeadlineExceeded, model.BatchStatusFailed, model.ExecutionContext{}, 0, 0},
		},
	}
	workerStep := &MockStep{IDValue: "workerStep"}
	mockPartitioner := &MockPartitioner{
		Partitions: map[string]model.ExecutionContext{
			"p0": testutil.NewTestExecutionContext(map[string]interface{}{"partition.name": "p0"}),
		},
	}
	jobExecution := testutil.NewTestJobExecution("jobInstID", "partitionJob", model.NewJobParameters())
	controllerExecution := testutil.NewTestStepExecution(jobExecution, "controllerStep")

	partitionStep := partition.NewPartitionStep(
		"controllerStep",
		mockPartitioner,
		workerStep,
		1,
		1,
		mockRepo,
		[]port.StepExecutionListener{},
		nil,
		mockExecutor,
	)

	mockPartitioner.On("Partition", mock.Anything, 1).Return(map[string]model.ExecutionContext{
		"p0": testutil.NewTestExecutionContext(map[string]interface{}{"partition.name": "p0"}),
	}, nil).Once()
	mockRepo.MockStepExecutionRepository.On("UpdateStepExecution", mock.Anything, mock.AnythingOfType("*model.StepExecution")).Return(nil).Maybe()
	mockRepo.MockStepExecutionRepository.On("SaveStepExecution", mock.Anything, mock.AnythingOfType("*model.StepExecution")).Return(nil).Once()

	err := partitionStep.Execute(ctx, jobExecution, controllerExecution)

	assert.Error(t, err)
	assert.True(t, errors.Is(err, context.DeadlineExceeded))
	assert.Equal(t, model.BatchStatusFailed, controllerExecution.Status)
	mockRepo.MockStepExecutionRepository.AssertExpectations(t)
}

// TestPartitionStep_RepositoryUpdateFailure verifies that if the repository fails to update the controller step, the step fails.
func TestPartitionStep_RepositoryUpdateFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mockRepo := NewMockJobRepository()
	mockExecutor := &MockStepExecutor{
		Results: map[string]struct {
			Err        error
			Status     model.JobStatus
			EC         model.ExecutionContext
			ReadCount  int
			WriteCount int
		}{
			"p0": {nil, model.BatchStatusCompleted, model.ExecutionContext{}, 1, 1},
		},
	}
	workerStep := &MockStep{IDValue: "workerStep"}
	mockPartitioner := &MockPartitioner{
		Partitions: map[string]model.ExecutionContext{
			"p0": testutil.NewTestExecutionContext(map[string]interface{}{"partition.name": "p0"}),
		},
	}
	jobExecution := testutil.NewTestJobExecution("jobInstID", "partitionJob", model.NewJobParameters())
	controllerExecution := testutil.NewTestStepExecution(jobExecution, "controllerStep")

	partitionStep := partition.NewPartitionStep(
		"controllerStep",
		mockPartitioner,
		workerStep,
		1,
		1,
		mockRepo,
		[]port.StepExecutionListener{},
		nil,
		mockExecutor,
	)

	mockPartitioner.On("Partition", mock.Anything, 1).Return(map[string]model.ExecutionContext{
		"p0": testutil.NewTestExecutionContext(map[string]interface{}{"partition.name": "p0"}),
	}, nil).Once()

	// 1. The first call (update to STARTED) should succeed.
	mockRepo.MockStepExecutionRepository.On("UpdateStepExecution", mock.Anything, mock.AnythingOfType("*model.StepExecution")).Return(nil).Once()

	// 2. Worker saving should succeed.
	mockRepo.MockStepExecutionRepository.On("SaveStepExecution", mock.Anything, mock.AnythingOfType("*model.StepExecution")).Return(nil).Once()

	// 3. The final call (update to final state) should return an error.
	mockRepo.MockStepExecutionRepository.On("UpdateStepExecution", mock.Anything, mock.AnythingOfType("*model.StepExecution")).Return(errors.New("db error")).Once()

	err := partitionStep.Execute(ctx, jobExecution, controllerExecution)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "db error")
	mockRepo.MockStepExecutionRepository.AssertExpectations(t)
}
