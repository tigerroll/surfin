package partition_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	port "surfin/pkg/batch/core/application/port"
	model "surfin/pkg/batch/core/domain/model"
	metrics "surfin/pkg/batch/core/metrics"
	partition "surfin/pkg/batch/engine/step/partition"
	testutil "surfin/pkg/batch/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// --- Mock Implementations (flow_control_test.go からコピー/調整) ---

// MockStepExecutor implements port.StepExecutor
type MockStepExecutor struct {
	mock.Mock
}

// ExecuteStep は、Worker Step の実行をシミュレートし、結果を StepExecution に反映します。
// 戻り値: (nil, error, ExitStatus string, ExecutionContext, ReadCount int, WriteCount int)
func (m *MockStepExecutor) ExecuteStep(ctx context.Context, step port.Step, jobExecution *model.JobExecution, stepExecution *model.StepExecution) (*model.StepExecution, error) {
	// 最初の3つの引数 (ctx, step, jobExecution) は無視し、4つ目の引数 (stepExecution) を使用してモックを呼び出す
	args := m.Called(ctx, step, jobExecution, stepExecution)

	// 実行結果を StepExecution に反映
	execErr := args.Error(1)

	if execErr != nil {
		stepExecution.MarkAsFailed(execErr)
	} else {
		// 成功の場合、モックの引数から ExitStatus を取得し、StepExecution に設定
		var exitStatus model.ExitStatus = model.ExitStatusCompleted
		if exitStatusStr, ok := args.Get(2).(string); ok {
			exitStatus = model.ExitStatus(exitStatusStr)
		}
		stepExecution.ExitStatus = exitStatus
		stepExecution.MarkAsCompleted()
	}

	// ExecutionContext の更新 (集約検証用)
	if ec, ok := args.Get(3).(model.ExecutionContext); ok {
		stepExecution.ExecutionContext = ec
	}

	// Read/Write Count の更新 (集約検証用)
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

// MockStep (最小限の実装)
type MockStep struct {
	IDValue string
}

func (m *MockStep) ID() string       { return m.IDValue }
func (m *MockStep) StepName() string { return m.IDValue }
func (m *MockStep) Execute(ctx context.Context, jobExecution *model.JobExecution, stepExecution *model.StepExecution) error {
	return nil
}
func (m *MockStep) GetTransactionOptions() *sql.TxOptions             { return nil }
func (m *MockStep) GetPropagation() string                            { return "" }
func (m *MockStep) SetMetricRecorder(recorder metrics.MetricRecorder) {}
func (m *MockStep) SetTracer(tracer metrics.Tracer)                   {}

// MockPartitioner implements port.Partitioner (flow_control_test.go からコピー)
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

// --- Test Case: T_SCALE_1_NEW (パーティション集約の検証) ---

func TestPartitionStep_Aggregation(t *testing.T) {
	ctx := context.Background()

	// 1. セットアップ
	mockRepo := NewMockJobRepository()
	mockExecutor := &MockStepExecutor{}

	// Worker Step (ダミー)
	workerStep := &MockStep{IDValue: "workerStep"}

	// Partitioner (3つのパーティションを返す)
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

	// Promotion設定 (EC集約検証用)
	promotion := &model.ExecutionContextPromotion{
		Keys:         []string{"p1.count", "p2.status"},
		JobLevelKeys: map[string]string{"p0.status": "job.p0_status"},
	}

	// PartitionStep の作成
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

	// 2. モックの期待値設定

	// Partitioner.Partition の期待値
	mockPartitioner.On("Partition", mock.Anything, 3).Return(nil, nil).Once()

	// JobRepository の期待値 (Controller StepExecution の更新)
	// Controller: STARTED (1回), FAILED (1回)
	mockRepo.MockStepExecutionRepository.On("UpdateStepExecution", mock.Anything, mock.AnythingOfType("*model.StepExecution")).Return(nil).Times(2) // Controller: STARTED, FAILED
	mockRepo.MockStepExecutionRepository.On("SaveStepExecution", mock.Anything, mock.AnythingOfType("*model.StepExecution")).Return(nil).Times(3)   // Worker StepExecution の保存

	// MockStepExecutor の期待値 (Worker Step の実行)

	// Worker 0: 成功, Read=10, Write=5, EC: {p0.status: "OK", p0.count: 10, start: 0}
	mockExecutor.On("ExecuteStep", mock.Anything, workerStep, jobExecution, mock.AnythingOfType("*model.StepExecution")).Return(
		nil,         // 1st return: *model.StepExecution (nilでOK, 4th argが更新される)
		nil,         // 2nd return: error
		"COMPLETED", // 3rd return: ExitStatus string
		model.ExecutionContext{"p0.status": "OK", "p0.count": 10, "start": 0}, // 4th return: EC (p1.count -> p0.count に変更)
		10, // 5th return: ReadCount
		5,  // 6th return: WriteCount
	).Once()

	// Worker 1: 失敗 (FAILED), Read=5, Write=0, EC: {p1.count: 5, p1.status: "ERROR", start: 100}
	worker1Error := errors.New("worker 1 failed")
	mockExecutor.On("ExecuteStep", mock.Anything, workerStep, jobExecution, mock.AnythingOfType("*model.StepExecution")).Return(
		nil,          // 1st return: *model.StepExecution (nilでOK)
		worker1Error, // 2nd return: error
		"FAILED",     // 3rd return: ExitStatus string
		model.ExecutionContext{"p1.count": 5, "p1.status": "ERROR", "start": 100}, // 4th return: EC (p2.status -> p1.status に変更)
		5, // 5th return: ReadCount
		0, // 6th return: WriteCount
	).Once()

	// Worker 2: 成功, Read=20, Write=15, EC: {p2.status: "OK", extra: "data", start: 200}
	mockExecutor.On("ExecuteStep", mock.Anything, workerStep, jobExecution, mock.AnythingOfType("*model.StepExecution")).Return(
		nil,         // 1st return: *model.StepExecution (nilでOK)
		nil,         // 2nd return: error
		"COMPLETED", // 3rd return: ExitStatus string
		model.ExecutionContext{"p2.status": "OK", "extra": "data", "start": 200}, // 4th return: EC
		20, // 5th return: ReadCount
		15, // 6th return: WriteCount
	).Once()

	// 3. 実行
	err := partitionStep.Execute(ctx, jobExecution, controllerExecution)

	// 4. アサーション

	// 4.1. 実行結果の検証
	assert.Error(t, err) // Worker 1 が失敗したため、全体としてエラーを返す
	assert.Contains(t, err.Error(), "one or more partitions failed")
	assert.Equal(t, model.BatchStatusFailed, controllerExecution.Status)
	assert.Equal(t, model.ExitStatusFailed, controllerExecution.ExitStatus)

	// 4.2. 統計情報の集約検証
	assert.Equal(t, 35, controllerExecution.ReadCount, "ReadCount should be aggregated (10 + 5 + 20)")
	assert.Equal(t, 20, controllerExecution.WriteCount, "WriteCount should be aggregated (5 + 0 + 15)")

	// 4.3. ExecutionContext の集約検証 (Worker 0 -> Worker 1 -> Worker 2 の順でマージされると仮定)
	// NOTE: 実行順序が不定なため、ECのキーの順序は保証されないが、DeepEqualで比較する。

	expectedEC := model.ExecutionContext{
		"start":     200,     // Worker 2 の値で上書き
		"p0.status": "OK",    // Worker 0 の値
		"p0.count":  10,      // Worker 0 の値
		"p1.count":  5,       // Worker 1 の値
		"p1.status": "ERROR", // Worker 1 の値
		"p2.status": "OK",    // Worker 2 の値で上書き
		"extra":     "data",
	}

	// Controller ExecutionContext の検証
	assert.Equal(t, expectedEC, controllerExecution.ExecutionContext, "Controller EC should contain merged worker ECs")

	// 4.4. Job ExecutionContext への昇格検証

	// Keys promotion: p1.count, p2.status
	p1Count, ok := jobExecution.ExecutionContext.GetNested("p1.count")
	assert.True(t, ok)
	assert.Equal(t, 5, p1Count) // Worker 1 の値 5 が昇格されることを期待

	p2Status, ok := jobExecution.ExecutionContext.GetNested("p2.status")
	assert.True(t, ok)
	assert.Equal(t, "OK", p2Status)

	// JobLevelKeys promotion: p0.status -> job.p0_status
	jobP0Status, ok := jobExecution.ExecutionContext.GetNested("job.p0_status")
	assert.True(t, ok)
	assert.Equal(t, "OK", jobP0Status)

	// 5. モックの検証
	mockPartitioner.AssertExpectations(t)
	mockExecutor.AssertExpectations(t)
	mockRepo.MockStepExecutionRepository.AssertExpectations(t)
}
