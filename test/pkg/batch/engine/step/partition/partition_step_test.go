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

// --- Mock Implementations (copied/adjusted from flow_control_test.go) ---

// MockStepExecutor implements port.StepExecutor
type MockStepExecutor struct {
	mock.Mock
}

// ExecuteStep simulates the execution of a Worker Step and reflects the results in StepExecution.
// Returns: (*model.StepExecution, error)
func (m *MockStepExecutor) ExecuteStep(ctx context.Context, step port.Step, jobExecution *model.JobExecution, stepExecution *model.StepExecution) (*model.StepExecution, error) {
	// Ignore the first three arguments (ctx, step, jobExecution) and use the fourth argument (stepExecution) to call the mock.
	args := m.Called(ctx, step, jobExecution, stepExecution)

	// Reflect execution results in StepExecution
	execErr := args.Error(1)

	if execErr != nil {
		stepExecution.MarkAsFailed(execErr)
	} else {
		// On success, get ExitStatus from mock arguments and set it in StepExecution
		var exitStatus model.ExitStatus = model.ExitStatusCompleted
		if exitStatusStr, ok := args.Get(2).(string); ok {
			exitStatus = model.ExitStatus(exitStatusStr)
		}
		stepExecution.ExitStatus = exitStatus
		stepExecution.MarkAsCompleted()
	}

	// Update ExecutionContext (for aggregation verification)
	if ec, ok := args.Get(3).(model.ExecutionContext); ok {
		stepExecution.ExecutionContext = ec
	}

	// Update Read/Write Count (for aggregation verification)
	if readCount, ok := args.Get(4).(int); ok {
		stepExecution.ReadCount = readCount
	}
	if writeCount, ok := args.Get(5).(int); ok {
		stepExecution.WriteCount = writeCount
	}

	return stepExecution, execErr
}

// MockJobInstanceRepository implements repository.JobInstance
type MockJobInstanceRepository struct {
	mock.Mock
}

func (m *MockJobInstanceRepository) SaveJobInstance(ctx context.Context, instance *model.JobInstance) error {
	args := m.Called(ctx, instance)
	return args.Error(0)
}
func (m *MockJobInstanceRepository) UpdateJobInstance(ctx context.Context, instance *model.JobInstance) error {
	args := m.Called(ctx, instance)
	return args.Error(0)
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

// MockJobExecutionRepository implements repository.JobExecution
type MockJobExecutionRepository struct {
	mock.Mock
}

func (m *MockJobExecutionRepository) SaveJobExecution(ctx context.Context, execution *model.JobExecution) error {
	args := m.Called(ctx, execution)
	return args.Error(0)
}
func (m *MockJobExecutionRepository) UpdateJobExecution(ctx context.Context, execution *model.JobExecution) error {
	args := m.Called(ctx, execution)
	return args.Error(0)
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

// MockStepExecutionRepository implements repository.StepExecution and repository.CheckpointDataRepository
type MockStepExecutionRepository struct {
	mock.Mock
}

func (m *MockStepExecutionRepository) SaveStepExecution(ctx context.Context, stepExec *model.StepExecution) error {
	args := m.Called(ctx, stepExec)
	return args.Error(0)
}
func (m *MockStepExecutionRepository) UpdateStepExecution(ctx context.Context, stepExec *model.StepExecution) error {
	args := m.Called(ctx, stepExec)
	return args.Error(0)
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
	args := m.Called(ctx, data)
	return args.Error(0)
}
func (m *MockStepExecutionRepository) FindCheckpointData(ctx context.Context, stepExecutionID string) (*model.CheckpointData, error) {
	args := m.Called(ctx, stepExecutionID)
	if res, ok := args.Get(0).(*model.CheckpointData); ok {
		return res, args.Error(1)
	}
	return nil, args.Error(1)
}

// MockJobRepository combines all mocks into one JobRepository interface
type MockJobRepository struct {
	*MockJobInstanceRepository
	*MockJobExecutionRepository
	*MockStepExecutionRepository
}

func NewMockJobRepository() *MockJobRepository {
	return &MockJobRepository{
		MockJobInstanceRepository:   &MockJobInstanceRepository{},
		MockJobExecutionRepository:  &MockJobExecutionRepository{},
		MockStepExecutionRepository: &MockStepExecutionRepository{},
	}
}

func (m *MockJobRepository) Close() error {
	return nil
}

// MockStep (minimal implementation)
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

// MockPartitioner implements port.Partitioner (copied from flow_control_test.go)
type MockPartitioner struct {
	mock.Mock
	Partitions map[string]model.ExecutionContext
}

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

// --- Test Case: T_SCALE_1_NEW (Verification of Partition Aggregation) ---

func TestPartitionStep_Aggregation(t *testing.T) {
	ctx := context.Background()

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
	mockRepo.MockStepExecutionRepository.On("UpdateStepExecution", mock.Anything, mock.AnythingOfType("*model.StepExecution")).Return(nil).Times(2) // Controller: STARTED, FAILED
	mockRepo.MockStepExecutionRepository.On("SaveStepExecution", mock.Anything, mock.AnythingOfType("*model.StepExecution")).Return(nil).Times(3)   // Save Worker StepExecution

	// MockStepExecutor expectations (for Worker Step execution)

	// Worker 0: Success, Read=10, Write=5, EC: {p0.status: "OK", p0.count: 10, start: 0}
	mockExecutor.On("ExecuteStep", mock.Anything, workerStep, jobExecution, mock.AnythingOfType("*model.StepExecution")).Return(
		nil,         // 1st return: *model.StepExecution (nil is OK, 4th arg will be updated)
		nil,         // 2nd return: error
		"COMPLETED", // 3rd return: ExitStatus string
		model.ExecutionContext{"p0.status": "OK", "p0.count": 10, "start": 0}, // 4th return: EC (changed from p1.count to p0.count)
		10, // 5th return: ReadCount
		5,  // 6th return: WriteCount
	).Once()

	// Worker 1: Failure (FAILED), Read=5, Write=0, EC: {p1.count: 5, p1.status: "ERROR", start: 100}
	worker1Error := errors.New("worker 1 failed")
	mockExecutor.On("ExecuteStep", mock.Anything, workerStep, jobExecution, mock.AnythingOfType("*model.StepExecution")).Return(
		nil,          // 1st return: *model.StepExecution (nil is OK)
		worker1Error, // 2nd return: error
		"FAILED",     // 3rd return: ExitStatus string
		model.ExecutionContext{"p1.count": 5, "p1.status": "ERROR", "start": 100}, // 4th return: EC (changed from p2.status to p1.status)
		5, // 5th return: ReadCount
		0, // 6th return: WriteCount
	).Once()

	// Worker 2: Success, Read=20, Write=15, EC: {p2.status: "OK", extra: "data", start: 200}
	mockExecutor.On("ExecuteStep", mock.Anything, workerStep, jobExecution, mock.AnythingOfType("*model.StepExecution")).Return(
		nil,         // 1st return: *model.StepExecution (nil is OK)
		nil,         // 2nd return: error
		"COMPLETED", // 3rd return: ExitStatus string
		model.ExecutionContext{"p2.status": "OK", "extra": "data", "start": 200}, // 4th return: EC
		20, // 5th return: ReadCount
		15, // 6th return: WriteCount
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

	// 4.3. Verify aggregation of ExecutionContext (assuming merge order: Worker 0 -> Worker 1 -> Worker 2)
	// NOTE: Since execution order is not guaranteed, the order of EC keys is not guaranteed, but compare with DeepEqual.

	// The 'start' key is non-deterministic due to map iteration order.
	// We will check its presence and that it's one of the expected values,
	// but remove it from the main DeepEqual comparison.
	actualEC := controllerExecution.ExecutionContext
	startVal, startOk := actualEC["start"]
	delete(actualEC, "start") // Remove 'start' for deterministic comparison of other keys

	expectedEC := model.ExecutionContext{
		"p0.status": "OK",    // Worker 0's value
		"p0.count":  10,      // Worker 0's value
		"p1.count":  5,       // Worker 1's value
		"p1.status": "ERROR", // Worker 1's value
		"p2.status": "OK",    // Overwritten by Worker 2's value
		"extra":     "data",
	}

	// Verify Controller ExecutionContext for other keys
	assert.Equal(t, expectedEC, actualEC, "Controller EC should contain merged worker ECs (excluding 'start')")

	// Verify 'start' key separately
	assert.True(t, startOk, "'start' key should be present in ExecutionContext")
	assert.Contains(t, []interface{}{0, 100, 200}, startVal, "'start' value should be one of the partition start values")

	// 4.4. Verify promotion to Job ExecutionContext

	// Keys promotion: p1.count, p2.status
	p1Count, ok := jobExecution.ExecutionContext.GetNested("p1.count")
	assert.True(t, ok)
	assert.Equal(t, 5, p1Count) // Expect Worker 1's value 5 to be promoted

	p2Status, ok := jobExecution.ExecutionContext.GetNested("p2.status")
	assert.True(t, ok)
	assert.Equal(t, "OK", p2Status)

	// JobLevelKeys promotion: p0.status -> job.p0_status
	jobP0Status, ok := jobExecution.ExecutionContext.GetNested("job.p0_status")
	assert.True(t, ok)
	assert.Equal(t, "OK", jobP0Status)

	// 5. Verify mocks
	mockPartitioner.AssertExpectations(t)
	mockExecutor.AssertExpectations(t)
	mockRepo.MockStepExecutionRepository.AssertExpectations(t)
}
