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

// MockStepExecutor is a mock implementation of port.StepExecutor.
// It simulates the execution of a worker step and reflects the results in the provided StepExecution.
type MockStepExecutor struct {
	mock.Mock
}

// ExecuteStep simulates the execution of a Worker Step and updates the StepExecution state.
func (m *MockStepExecutor) ExecuteStep(ctx context.Context, step port.Step, jobExecution *model.JobExecution, stepExecution *model.StepExecution) (*model.StepExecution, error) {
	args := m.Called(ctx, step, jobExecution, stepExecution)

	execErr := args.Error(1)

	if execErr != nil {
		stepExecution.MarkAsFailed(execErr)
	} else {
		// On success, set ExitStatus from mock arguments.
		var exitStatus model.ExitStatus = model.ExitStatusCompleted
		if exitStatusStr, ok := args.Get(2).(string); ok {
			exitStatus = model.ExitStatus(exitStatusStr)
		}
		stepExecution.ExitStatus = exitStatus
		// Directly update status to avoid state transition errors.
		stepExecution.Status = model.BatchStatusCompleted
	}

	// Update ExecutionContext for aggregation verification.
	if ec, ok := args.Get(3).(model.ExecutionContext); ok {
		stepExecution.ExecutionContext = ec
	}

	// Update Read/Write counts for aggregation verification.
	if readCount, ok := args.Get(4).(int); ok {
		stepExecution.ReadCount = readCount
	}
	if writeCount, ok := args.Get(5).(int); ok {
		stepExecution.WriteCount = writeCount
	}

	return stepExecution, execErr
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
	mockExecutor := &MockStepExecutor{}

	// Worker Step (dummy)
	workerStep := &MockStep{IDValue: "workerStep"}

	// Partitioner (returns 3 partitions)
	mockPartitioner := &MockPartitioner{
		Partitions: map[string]model.ExecutionContext{
			"partition0": testutil.NewTestExecutionContext(map[string]interface{}{"start": 0}),
			"partition1": testutil.NewTestExecutionContext(map[string]interface{}{"start": 100}),
			"partition2": testutil.NewTestExecutionContext(map[string]interface{}{"start": 200}),
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

	// MockStepExecutor expectations (for Worker Step execution)

	// Worker 0: Success, Read=10, Write=5, EC: {p0.status: "OK", p0.count: 10, start: 0}
	mockExecutor.On("ExecuteStep", mock.Anything, workerStep, jobExecution, mock.AnythingOfType("*model.StepExecution")).Return(
		nil,
		nil,
		"COMPLETED",
		model.ExecutionContext{"p0.status": "OK", "p0.count": 10, "start": 0},
		10,
		5,
	).Once()

	// Worker 1: Failure (FAILED), Read=5, Write=0, EC: {p1.count: 5, p1.status: "ERROR", start: 100}
	worker1Error := errors.New("worker 1 failed")
	mockExecutor.On("ExecuteStep", mock.Anything, workerStep, jobExecution, mock.AnythingOfType("*model.StepExecution")).Return(
		nil,
		worker1Error,
		"FAILED",
		model.ExecutionContext{"p1.count": 5, "p1.status": "ERROR", "start": 100},
		5,
		0,
	).Once()

	// Worker 2: Success, Read=20, Write=15, EC: {p2.status: "OK", extra: "data", start: 200}
	mockExecutor.On("ExecuteStep", mock.Anything, workerStep, jobExecution, mock.AnythingOfType("*model.StepExecution")).Return(
		nil,
		nil,
		"COMPLETED",
		model.ExecutionContext{"p2.status": "OK", "extra": "data", "start": 200},
		20,
		15,
	).Once()

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
	mockExecutor.AssertExpectations(t)
	mockRepo.MockStepExecutionRepository.AssertExpectations(t)
}
