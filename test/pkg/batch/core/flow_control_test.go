package core_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"

	model "surfin/pkg/batch/core/domain/model"
	job "surfin/pkg/batch/core/domain/repository"
	tx "surfin/pkg/batch/core/tx"
	metrics "surfin/pkg/batch/core/metrics"
	partition "surfin/pkg/batch/engine/step/partition" // SimpleStepExecutor を使用するため追加
	testutil "surfin/pkg/batch/test" // 追加

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// --- Mock Implementations for JobRepository components (L.1 Integration Test needs these) ---

// MockJobInstanceRepository implements job.JobInstance
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

// MockJobExecutionRepository implements job.JobExecution
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

// MockStepExecutionRepository implements job.StepExecution
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

// --- Mock Tx and TxManager Implementations ---

// MockTx implements tx.Tx
type MockTx struct {
	mock.Mock
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

// MockTxManager implements tx.TransactionManager
type MockTxManager struct {
	mock.Mock
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

// --- Mock Flow Elements ---

// MockPartitioner implements port.Partitioner
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

// MockStep implements port.Step
type MockStep struct {
	IDValue string
	Repo job.JobRepository // 追加: 永続化シミュレーション用
	// 追加フィールド
	PropagationValue string
	IsolationLevelValue sql.IsolationLevel
	SimulatedError error
	
	// port.Step の要件を満たすためのダミーフィールド
	SetMetricRecorderFunc func(recorder metrics.MetricRecorder)
	SetTracerFunc func(tracer metrics.Tracer)
}

func (m *MockStep) ID() string { return m.IDValue }
func (m *MockStep) StepName() string { return m.IDValue }
func (m *MockStep) Execute(ctx context.Context, jobExecution *model.JobExecution, stepExecution *model.StepExecution) error {
	// 1. STARTED への遷移 (TaskletStep/ChunkStep の Execute 開始時をシミュレート)
	if stepExecution.Status == model.BatchStatusStarting {
		stepExecution.MarkAsStarted()
		if m.Repo != nil {
			m.Repo.UpdateStepExecution(ctx, stepExecution) // STARTED への遷移を永続化 (1回目)
		}
	}

	// エラーをシミュレート
	if m.SimulatedError != nil {
		stepExecution.MarkAsFailed(m.SimulatedError)
		if m.Repo != nil {
			m.Repo.UpdateStepExecution(ctx, stepExecution) // FAILED への遷移を永続化 (2回目)
		}
		return m.SimulatedError
	}
	
	// 成功の場合、COMPLETED に遷移
	stepExecution.MarkAsCompleted()
	if m.Repo != nil {
		m.Repo.UpdateStepExecution(ctx, stepExecution) // COMPLETED への遷移を永続化 (2回目)
	}
	return nil
}
// 追加: port.Step インターフェースのメソッド
func (m *MockStep) GetTransactionOptions() *sql.TxOptions {
	return &sql.TxOptions{Isolation: m.IsolationLevelValue}
}
func (m *MockStep) GetPropagation() string {
	return m.PropagationValue
}
func (m *MockStep) SetMetricRecorder(recorder metrics.MetricRecorder) {
	if m.SetMetricRecorderFunc != nil {
		m.SetMetricRecorderFunc(recorder)
	}
}
func (m *MockStep) SetTracer(tracer metrics.Tracer) {
	if m.SetTracerFunc != nil {
		m.SetTracerFunc(tracer)
	}
}

// MockDecision implements port.Decision
type MockDecision struct {
	IDValue string
	SimulatedResult model.ExitStatus
}

func (m *MockDecision) ID() string { return m.IDValue }
func (m *MockDecision) DecisionName() string { return m.IDValue }
func (m *MockDecision) Decide(ctx context.Context, jobExecution *model.JobExecution, jobParameters model.JobParameters) (model.ExitStatus, error) {
	return m.SimulatedResult, nil
}
func (m *MockDecision) SetProperties(properties map[string]string) {} // Mockなので実装は不要

// --- Test Job Runner Implementation (Simulating core flow logic) ---

// TestJobRunner simulates the core flow execution logic based on FlowDefinition,
// relying on simulated results for element execution and mocking repository updates.
type TestJobRunner struct {
	Repo job.JobRepository
	// Map of element ID to its simulated ExitStatus result
	SimulatedResults map[string]model.ExitStatus
}

func NewTestJobRunner(repo job.JobRepository, results map[string]model.ExitStatus) *TestJobRunner {
	return &TestJobRunner{
		Repo:             repo,
		SimulatedResults: results,
	}
}

// isDecisionElement checks if the element is a MockDecision.
func (r *TestJobRunner) isDecisionElement(element interface{}) bool {
	_, ok := element.(*MockDecision)
	return ok
}

// Run simulates the JobRunner's main loop, handling transitions and updating JobExecution status.
// NOTE: This Run method is for flow control testing and does not simulate StepExecutor's transaction logic.
func (r *TestJobRunner) Run(ctx context.Context, flowDef *model.FlowDefinition, jobExecution *model.JobExecution, jobParameters model.JobParameters) (*model.JobExecution, error) {
	currentElementID := flowDef.StartElement

	// Initial update (STARTING -> STARTED transition is usually handled outside, but we ensure status is STARTED for flow)
	if jobExecution.Status == model.BatchStatusStarting {
		jobExecution.MarkAsStarted()
		r.Repo.UpdateJobExecution(ctx, jobExecution) // 1回目の更新 (STARTING -> STARTED)
	}

	for {
		if currentElementID == "" {
			// Flow finished without explicit 'end' or 'fail' transition (should be caught by rule.Transition.End)
			jobExecution.MarkAsCompleted()
			r.Repo.UpdateJobExecution(ctx, jobExecution)
			return jobExecution, nil
		}

		elementInterface, ok := flowDef.Elements[currentElementID]
		if !ok {
			err := fmt.Errorf("flow element not found: %s", currentElementID)
			jobExecution.MarkAsFailed(err)
			r.Repo.UpdateJobExecution(ctx, jobExecution)
			return nil, err
		}

		// 1. Execute Element (Simulated)
		_, statusFound := r.SimulatedResults[currentElementID]
		
		// Decisionの場合は、SimulatedResultsに依存せず、Decideメソッドの結果を使用する
		if !statusFound && !r.isDecisionElement(elementInterface) {
			err := fmt.Errorf("simulated result not found for element: %s", currentElementID)
			jobExecution.MarkAsFailed(err)
			r.Repo.UpdateJobExecution(ctx, jobExecution)
			return nil, err
		}

		var elementExitStatus model.ExitStatus
		var elementErr error

		switch elem := elementInterface.(type) {
		case *MockStep:
			// --- Step Execution Logic ---
			stepName := elem.StepName()
			jobExecution.CurrentStepName = stepName

			// StepExecutionの検索/作成
			var stepExec *model.StepExecution
			for _, se := range jobExecution.StepExecutions {
				if se.StepName == stepName {
					stepExec = se
					break
				}
			}
			
			if stepExec == nil {
				// New StepExecution
				stepExec = model.NewStepExecution(model.NewID(), jobExecution, stepName)
				jobExecution.AddStepExecution(stepExec)
				// Simulate saving the new StepExecution
				r.Repo.SaveStepExecution(ctx, stepExec)
			}
			
			// Simulate Step execution and status update
			// MockStep.Execute内で STARTED -> COMPLETED/FAILED への遷移と永続化がシミュレートされる
			elementErr = elem.Execute(ctx, jobExecution, stepExec)
			elementExitStatus = stepExec.ExitStatus // MockStep.Executeで設定された最終ExitStatusを取得

			if elementErr != nil {
				// MockStep.Execute内で MarkAsFailed が呼ばれているはず
			} else {
				// MockStep.Execute内で MarkAsCompleted が呼ばれているはず
			}

		case *MockDecision:
			// --- Decision Execution Logic ---
			decisionResult, _ := elem.Decide(ctx, jobExecution, jobParameters)
			elementExitStatus = decisionResult
			elementErr = nil // Mock Decisionはエラーを返さないと仮定

		default:
			err := fmt.Errorf("unknown flow element type: %T (ID: %s)", elementInterface, currentElementID)
			jobExecution.AddFailureException(err)
			jobExecution.MarkAsFailed(err)
			r.Repo.UpdateJobExecution(ctx, jobExecution)
			return nil, err
		}

		// 2. Update JobExecution context (Simulated Step/Decision completion)
		jobExecution.CurrentStepName = currentElementID
		exitStatus := elementExitStatus

		// 3. Find Transition Rule
		transitionRule, found := flowDef.GetTransitionRule(currentElementID, exitStatus, elementErr != nil)
		if !found {
			// Fallback to '*' rule if available
			transitionRule, found = flowDef.GetTransitionRule(currentElementID, model.ExitStatus("*"), elementErr != nil)
			if !found {
				err := fmt.Errorf("no transition found for element %s with status: %s", currentElementID, exitStatus)
				jobExecution.MarkAsFailed(err)
				r.Repo.UpdateJobExecution(ctx, jobExecution)
				return jobExecution, errors.New("flow failed: " + err.Error())
			}
		}

		// 4. Apply Transition
		// isFinalTransition := transitionRule.Transition.End || transitionRule.Transition.Fail || transitionRule.Transition.Stop

		if transitionRule.Transition.End {
			jobExecution.MarkAsCompleted()
			r.Repo.UpdateJobExecution(ctx, jobExecution)
			return jobExecution, nil
		}
		if transitionRule.Transition.Fail {
			jobExecution.MarkAsFailed(errors.New("flow failed due to transition rule"))
			r.Repo.UpdateJobExecution(ctx, jobExecution)
			return jobExecution, errors.New("flow failed due to transition rule")
		}
		if transitionRule.Transition.Stop {
			jobExecution.MarkAsStopped()
			r.Repo.UpdateJobExecution(ctx, jobExecution)
			return jobExecution, nil
		}

		// Move to the next element
		currentElementID = transitionRule.Transition.To
	}
}

func TestJobRunner_FlowControlIntegration(t *testing.T) {
	ctx := context.Background()

	// Setup Mocks
	mockRepo := NewMockJobRepository()

	// Define Job Instance and Execution
	jobName := "flowTestJob"
	params := model.NewJobParameters()
	instance := model.NewJobInstance(jobName, params)

	// Mock Repository Expectations for JobExecution updates during flow
	mockRepo.MockJobExecutionRepository.On("UpdateJobExecution", mock.Anything, mock.AnythingOfType("*model.JobExecution")).Return(nil).Maybe()
	// Mock Repository Expectations for StepExecution saves and updates
	mockRepo.MockStepExecutionRepository.On("SaveStepExecution", mock.Anything, mock.AnythingOfType("*model.StepExecution")).Return(nil).Maybe()
	mockRepo.MockStepExecutionRepository.On("UpdateStepExecution", mock.Anything, mock.AnythingOfType("*model.StepExecution")).Return(nil).Maybe()


	// --- Test Case 1: Simple Sequential Flow (StepA -> StepB -> End) ---
	t.Run("SimpleSequentialFlow", func(t *testing.T) {
		// Reset execution state
		execution := model.NewJobExecution(instance.ID, jobName, params)
		mockRepo.MockJobExecutionRepository.Calls = nil // Reset mock calls
		mockRepo.MockStepExecutionRepository.Calls = nil // Reset mock calls
		mockRepo.MockJobExecutionRepository.On("UpdateJobExecution", mock.Anything, mock.AnythingOfType("*model.JobExecution")).Return(nil).Maybe()
		mockRepo.MockStepExecutionRepository.On("SaveStepExecution", mock.Anything, mock.AnythingOfType("*model.StepExecution")).Return(nil).Maybe()
		mockRepo.MockStepExecutionRepository.On("UpdateStepExecution", mock.Anything, mock.AnythingOfType("*model.StepExecution")).Return(nil).Maybe()


		// 1. Define Flow Definition
		flowDef := model.NewFlowDefinition("stepA")

		stepA := &MockStep{IDValue: "stepA", Repo: mockRepo}
		stepB := &MockStep{IDValue: "stepB", Repo: mockRepo}

		flowDef.AddElement("stepA", stepA)
		flowDef.AddElement("stepB", stepB)

		// Transitions:
		// stepA (COMPLETED) -> stepB
		flowDef.AddTransitionRule("stepA", string(model.ExitStatusCompleted), "stepB", false, false, false)
		// stepB (COMPLETED) -> End
		flowDef.AddTransitionRule("stepB", string(model.ExitStatusCompleted), "", true, false, false)

		// 2. Setup Simulated Results
		simulatedResults := map[string]model.ExitStatus{
			"stepA": model.ExitStatusCompleted,
			"stepB": model.ExitStatusCompleted,
		}

		// 3. Run Test Runner
		runner := NewTestJobRunner(mockRepo, simulatedResults)
		resultExec, err := runner.Run(ctx, flowDef, execution, params)

		// 4. Assertions
		assert.NoError(t, err)
		assert.Equal(t, model.BatchStatusCompleted, resultExec.Status)
		assert.Equal(t, model.ExitStatusCompleted, resultExec.ExitStatus)
		assert.Equal(t, "stepB", resultExec.CurrentStepName) // Last step executed before End transition

		// Expected updates:
		// JobExecution: Initial Start (1) + Final Completion (1) = 2
		// StepExecution: Save (2) + Update (4) = 6 (StepA: S/U(STARTED)/U(COMPLETED), StepB: S/U(STARTED)/U(COMPLETED))
		mockRepo.MockJobExecutionRepository.AssertNumberOfCalls(t, "UpdateJobExecution", 2)
		mockRepo.MockStepExecutionRepository.AssertNumberOfCalls(t, "SaveStepExecution", 2) // Save 2回
		mockRepo.MockStepExecutionRepository.AssertNumberOfCalls(t, "UpdateStepExecution", 4) // Update 4回
	})

	// --- Test Case 2: Decision Flow (StepA -> Decision -> Branch) ---
	t.Run("DecisionFlow", func(t *testing.T) {
		// Reset execution state
		execution := model.NewJobExecution(instance.ID, jobName, params)
		mockRepo.MockJobExecutionRepository.Calls = nil // Reset mock calls
		mockRepo.MockStepExecutionRepository.Calls = nil // Reset mock calls
		mockRepo.MockJobExecutionRepository.On("UpdateJobExecution", mock.Anything, mock.AnythingOfType("*model.JobExecution")).Return(nil).Maybe()
		mockRepo.MockStepExecutionRepository.On("SaveStepExecution", mock.Anything, mock.AnythingOfType("*model.StepExecution")).Return(nil).Maybe()
		mockRepo.MockStepExecutionRepository.On("UpdateStepExecution", mock.Anything, mock.AnythingOfType("*model.StepExecution")).Return(nil).Maybe()


		// 1. Define Flow Definition
		flowDef := model.NewFlowDefinition("stepA")

		stepA := &MockStep{IDValue: "stepA", Repo: mockRepo}
		decision1 := &MockDecision{IDValue: "decision1", SimulatedResult: "SUCCESS"}
		stepSuccess := &MockStep{IDValue: "stepSuccess", Repo: mockRepo}
		stepFailure := &MockStep{IDValue: "stepFailure", Repo: mockRepo}

		flowDef.AddElement("stepA", stepA)
		flowDef.AddElement("decision1", decision1)
		flowDef.AddElement("stepSuccess", stepSuccess)
		flowDef.AddElement("stepFailure", stepFailure)

		// Transitions:
		// stepA (COMPLETED) -> decision1
		flowDef.AddTransitionRule("stepA", string(model.ExitStatusCompleted), "decision1", false, false, false)
		// decision1 (SUCCESS) -> stepSuccess -> End
		flowDef.AddTransitionRule("decision1", "SUCCESS", "stepSuccess", false, false, false)
		flowDef.AddTransitionRule("stepSuccess", string(model.ExitStatusCompleted), "", true, false, false)
		// decision1 (FAILURE) -> stepFailure -> Fail
		flowDef.AddTransitionRule("decision1", "FAILURE", "stepFailure", false, false, false)
		flowDef.AddTransitionRule("stepFailure", string(model.ExitStatusFailed), "", false, true, false) // FAILEDでFail遷移をシミュレート

		// 2. Setup Simulated Results (Decision returns SUCCESS)
		simulatedResults := map[string]model.ExitStatus{
			"stepA": model.ExitStatusCompleted,
			"stepSuccess": model.ExitStatusCompleted,
		}

		// 3. Run Test Runner
		runner := NewTestJobRunner(mockRepo, simulatedResults)
		resultExec, err := runner.Run(ctx, flowDef, execution, params)

		// 4. Assertions (Should follow SUCCESS path)
		assert.NoError(t, err)
		assert.Equal(t, model.BatchStatusCompleted, resultExec.Status)
		assert.Equal(t, model.ExitStatusCompleted, resultExec.ExitStatus)
		assert.Equal(t, "stepSuccess", resultExec.CurrentStepName)

		// Expected updates:
		// JobExecution: Initial Start (1) + Final Completion (1) = 2
		// StepExecution: Save (2) + Update (4) = 4
		mockRepo.MockJobExecutionRepository.AssertNumberOfCalls(t, "UpdateJobExecution", 2)
		mockRepo.MockStepExecutionRepository.AssertNumberOfCalls(t, "SaveStepExecution", 2) // Save 2回
		mockRepo.MockStepExecutionRepository.AssertNumberOfCalls(t, "UpdateStepExecution", 4) // Update 4回
	})

	// --- Test Case 3: Failure Transition ---
	t.Run("FailureTransition", func(t *testing.T) {
		// Reset execution state
		execution := model.NewJobExecution(instance.ID, jobName, params)
		mockRepo.MockJobExecutionRepository.Calls = nil // Reset mock calls
		mockRepo.MockStepExecutionRepository.Calls = nil // Reset mock calls
		mockRepo.MockJobExecutionRepository.On("UpdateJobExecution", mock.Anything, mock.AnythingOfType("*model.JobExecution")).Return(nil).Maybe()
		mockRepo.MockStepExecutionRepository.On("SaveStepExecution", mock.Anything, mock.AnythingOfType("*model.StepExecution")).Return(nil).Maybe()
		mockRepo.MockStepExecutionRepository.On("UpdateStepExecution", mock.Anything, mock.AnythingOfType("*model.StepExecution")).Return(nil).Maybe()


		// 1. Define Flow Definition
		flowDef := model.NewFlowDefinition("stepA")

		stepA := &MockStep{IDValue: "stepA", Repo: mockRepo}
		stepError := &MockStep{IDValue: "stepError", Repo: mockRepo}

		flowDef.AddElement("stepA", stepA)
		flowDef.AddElement("stepError", stepError)

		// Transitions:
		// stepA (COMPLETED) -> stepError
		flowDef.AddTransitionRule("stepA", string(model.ExitStatusCompleted), "stepError", false, false, false)
		// stepError (FAILED) -> Fail (Job fails immediately)
		flowDef.AddTransitionRule("stepError", string(model.ExitStatusFailed), "", false, true, false)
		// Fallback rule for stepError (should not be hit)
		flowDef.AddTransitionRule("stepError", "*", "stepA", false, false, false)

		// 2. Setup Simulated Results (stepError returns FAILED)
		simulatedResults := map[string]model.ExitStatus{
			"stepA": model.ExitStatusCompleted,
			"stepError": model.ExitStatusFailed,
		}
		// MockStepにエラーを注入
		stepError.SimulatedError = errors.New("simulated failure")

		// 3. Run Test Runner
		runner := NewTestJobRunner(mockRepo, simulatedResults)
		resultExec, err := runner.Run(ctx, flowDef, execution, params)

		// 4. Assertions (Should fail)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "flow failed due to transition rule")
		assert.Equal(t, model.BatchStatusFailed, resultExec.Status)
		assert.Equal(t, model.ExitStatusFailed, resultExec.ExitStatus)
		assert.Equal(t, "stepError", resultExec.CurrentStepName)

		// Expected updates:
		// JobExecution: Initial Start (1) + Final Failure (1) = 2
		// StepExecution: Save (2) + Update (4) = 4
		mockRepo.MockJobExecutionRepository.AssertNumberOfCalls(t, "UpdateJobExecution", 2)
		mockRepo.MockStepExecutionRepository.AssertNumberOfCalls(t, "SaveStepExecution", 2) // Save 2回
		mockRepo.MockStepExecutionRepository.AssertNumberOfCalls(t, "UpdateStepExecution", 4) // Update 4回
	})
}

// TestSimpleStepExecutor_NESTED_Propagation verifies that NESTED propagation uses Savepoint/RollbackToSavepoint
// when an external transaction exists and the Step fails.
func TestSimpleStepExecutor_NESTED_Propagation(t *testing.T) {
	ctx := context.Background()
	
	// 1. モックのセットアップ
	mockRepo := NewMockJobRepository()
	mockTxManager := &MockTxManager{}
	
	jobExecution := testutil.NewTestJobExecution("instanceID", "testJob", model.NewJobParameters())
	stepExecution := testutil.NewTestStepExecution(jobExecution, "nestedStep")
	
	// 2. 外部トランザクションのシミュレーション
	// SimpleStepExecutor はコンテキストに "tx" キーが存在するかどうかで外部トランザクションを検出します。
	externalTx := &MockTx{}
	// 外部トランザクションがアクティブなコンテキスト
	ctxWithExternalTx := context.WithValue(ctx, "tx", externalTx)
	
	// 3. MockStep の定義 (NESTED伝播、失敗をシミュレート)
	simulatedError := errors.New("simulated step failure")
	nestedStep := &MockStep{
		Repo: mockRepo, // 注入
		IDValue: "nestedStep",
		PropagationValue: "NESTED",
		SimulatedError: simulatedError, // 実行時にエラーを発生させる
	}
	
	// 4. 期待値の設定
	
	// 4.1. StepExecution の永続化 (Save/Update)
	// StepExecutor の実行前に、JobRunner が SaveStepExecution を呼び出すことをシミュレート
	mockRepo.MockStepExecutionRepository.On("SaveStepExecution", mock.Anything, mock.AnythingOfType("*model.StepExecution")).Return(nil).Once() 
	
	// JobRunnerの役割をシミュレートして初回永続化を実行
	if err := mockRepo.SaveStepExecution(ctx, stepExecution); err != nil {
		t.Fatalf("Failed to simulate initial SaveStepExecution: %v", err)
	}
	
	// UpdateStepExecution (STARTEDへの遷移)
	mockRepo.MockStepExecutionRepository.On("UpdateStepExecution", mock.Anything, mock.AnythingOfType("*model.StepExecution")).Return(nil).Twice() // STARTED (1回目) -> FAILED (2回目)
	
	// 4.2. NESTED トランザクションの期待値
	
	// StepExecutor は Savepoint を呼び出すことを期待
	// Savepoint 名は "SP_" + stepExecution.ID になることを期待
	expectedSavepointName := "SP_" + stepExecution.ID
	externalTx.On("Savepoint", expectedSavepointName).Return(nil).Once()
	
	// Step.Execute がエラーを返した場合、RollbackToSavepoint が呼び出されることを期待
	externalTx.On("RollbackToSavepoint", expectedSavepointName).Return(nil).Once()
	
	// 5. SimpleStepExecutor の実行
	// Tracer は nil で問題ない (ロギングのみ) -> 修正: NoOpTracer と NoOpMetricRecorder を使用
	executor := partition.NewSimpleStepExecutor(metrics.NewNoOpTracer(), metrics.NewNoOpMetricRecorder(), mockTxManager) 
	
	resultExec, err := executor.ExecuteStep(ctxWithExternalTx, nestedStep, jobExecution, stepExecution)
	
	// 6. アサーション
	assert.Error(t, err)
	assert.Contains(t, err.Error(), simulatedError.Error())
	assert.Equal(t, model.BatchStatusFailed, resultExec.Status)
	assert.Equal(t, model.ExitStatusFailed, resultExec.ExitStatus)
	
	// 7. モックの検証
	externalTx.AssertExpectations(t)
	mockRepo.MockStepExecutionRepository.AssertExpectations(t)
	
	// TxManager の Begin/Commit/Rollback は呼ばれないことを確認 (外部トランザクションに参加しているため)
	mockTxManager.AssertNotCalled(t, "Begin")
	mockTxManager.AssertNotCalled(t, "Commit")
	mockTxManager.AssertNotCalled(t, "Rollback")
}

// TestSimpleStepExecutor_NESTED_Propagation_Success verifies NESTED propagation commits implicitly on success.
func TestSimpleStepExecutor_NESTED_Propagation_Success(t *testing.T) {
	ctx := context.Background()
	
	// 1. モックのセットアップ
	mockRepo := NewMockJobRepository()
	mockTxManager := &MockTxManager{}
	
	jobExecution := testutil.NewTestJobExecution("instanceID", "testJob", model.NewJobParameters())
	stepExecution := testutil.NewTestStepExecution(jobExecution, "nestedStep")
	
	// 外部トランザクションのシミュレーション
	externalTx := &MockTx{}
	ctxWithExternalTx := context.WithValue(ctx, "tx", externalTx)
	
	// 3. MockStep の定義 (NESTED伝播、成功をシミュレート)
	nestedStep := &MockStep{
		Repo: mockRepo, // 注入
		IDValue: "nestedStep",
		PropagationValue: "NESTED",
		SimulatedError: nil, // 成功
	}
	
	// 4. 期待値の設定
	mockRepo.MockStepExecutionRepository.On("SaveStepExecution", mock.Anything, mock.AnythingOfType("*model.StepExecution")).Return(nil).Once() // JobRunnerの役割をシミュレート
	if err := mockRepo.SaveStepExecution(ctx, stepExecution); err != nil {
		t.Fatalf("Failed to simulate initial SaveStepExecution: %v", err)
	}
	mockRepo.MockStepExecutionRepository.On("UpdateStepExecution", mock.Anything, mock.AnythingOfType("*model.StepExecution")).Return(nil).Twice() // STARTED -> COMPLETED
	
	// StepExecutor は Savepoint を呼び出すことを期待
	expectedSavepointName := "SP_" + stepExecution.ID
	externalTx.On("Savepoint", expectedSavepointName).Return(nil).Once()
	
	// 成功した場合、RollbackToSavepoint は呼ばれないことを期待
	externalTx.AssertNotCalled(t, "RollbackToSavepoint")
	
	// 5. SimpleStepExecutor の実行
	executor := partition.NewSimpleStepExecutor(metrics.NewNoOpTracer(), metrics.NewNoOpMetricRecorder(), mockTxManager)
	
	resultExec, err := executor.ExecuteStep(ctxWithExternalTx, nestedStep, jobExecution, stepExecution)
	
	// 6. アサーション
	assert.NoError(t, err)
	assert.Equal(t, model.BatchStatusCompleted, resultExec.Status)
	assert.Equal(t, model.ExitStatusCompleted, resultExec.ExitStatus)
	
	// 7. モックの検証
	externalTx.AssertExpectations(t)
	mockRepo.MockStepExecutionRepository.AssertExpectations(t)
	
	// TxManager の Begin/Commit/Rollback は呼ばれないことを確認
	mockTxManager.AssertNotCalled(t, "Begin")
	mockTxManager.AssertNotCalled(t, "Commit")
	mockTxManager.AssertNotCalled(t, "Rollback")
}

// TestSimpleStepExecutor_REQUIRES_NEW_Propagation verifies that REQUIRES_NEW propagation
// suspends the external transaction, starts a new one, and commits/rolls back the new one independently.
func TestSimpleStepExecutor_REQUIRES_NEW_Propagation(t *testing.T) {
	ctx := context.Background()
	
	// 1. モックのセットアップ
	mockRepo := NewMockJobRepository()
	mockTxManager := &MockTxManager{}
	
	jobExecution := testutil.NewTestJobExecution("instanceID", "testJob", model.NewJobParameters())
	stepExecution := testutil.NewTestStepExecution(jobExecution, "requiresNewStep")
	
	// 2. 外部トランザクションのシミュレーション
	externalTx := &MockTx{}
	// 外部トランザクションがアクティブなコンテキスト
	ctxWithExternalTx := context.WithValue(ctx, "tx", externalTx)
	
	// 3. MockStep の定義 (REQUIRES_NEW伝播、成功をシミュレート)
	requiresNewStep := &MockStep{
		Repo: mockRepo, // 注入
		IDValue: "requiresNewStep",
		PropagationValue: "REQUIRES_NEW",
		SimulatedError: nil, // 成功
	}
	
	// 4. 内部トランザクションのモック
	internalTx := &MockTx{}
	
	// 5. 期待値の設定
	
	// 5.1. StepExecution の永続化 (Save/Update)
	mockRepo.MockStepExecutionRepository.On("SaveStepExecution", mock.Anything, mock.AnythingOfType("*model.StepExecution")).Return(nil).Once() // JobRunnerの役割をシミュレート
	if err := mockRepo.SaveStepExecution(ctx, stepExecution); err != nil {
		t.Fatalf("Failed to simulate initial SaveStepExecution: %v", err)
	}
	
	mockRepo.MockStepExecutionRepository.On("UpdateStepExecution", mock.Anything, mock.AnythingOfType("*model.StepExecution")).Return(nil).Twice() // STARTED (1回目) -> COMPLETED (2回目)
	
	// 5.2. REQUIRES_NEW トランザクションの期待値
	
	// TxManager は新しいトランザクションを開始することを期待
	mockTxManager.On("Begin", mock.Anything, mock.Anything).Return(internalTx, nil).Once()
	
	// Stepが成功した場合、内部トランザクションはコミットされることを期待
	mockTxManager.On("Commit", internalTx).Return(nil).Once()
	
	// 外部トランザクション (externalTx) はサスペンドされるため、Commit/Rollback/Savepoint は呼ばれないことを期待
	externalTx.AssertNotCalled(t, "Commit")
	externalTx.AssertNotCalled(t, "Rollback")
	externalTx.AssertNotCalled(t, "Savepoint")
	
	// 6. SimpleStepExecutor の実行
	executor := partition.NewSimpleStepExecutor(metrics.NewNoOpTracer(), metrics.NewNoOpMetricRecorder(), mockTxManager) 
	
	resultExec, err := executor.ExecuteStep(ctxWithExternalTx, requiresNewStep, jobExecution, stepExecution)
	
	// 7. アサーション
	assert.NoError(t, err)
	assert.Equal(t, model.BatchStatusCompleted, resultExec.Status)
	
	// 8. モックの検証
	mockTxManager.AssertExpectations(t)
	mockRepo.MockStepExecutionRepository.AssertExpectations(t)
}

// TestSimpleStepExecutor_REQUIRES_NEW_Propagation_Failure verifies REQUIRES_NEW rolls back the new transaction on failure.
func TestSimpleStepExecutor_REQUIRES_NEW_Propagation_Failure(t *testing.T) {
	ctx := context.Background()
	
	// 1. モックのセットアップ
	mockRepo := NewMockJobRepository()
	mockTxManager := &MockTxManager{}
	
	jobExecution := testutil.NewTestJobExecution("instanceID", "testJob", model.NewJobParameters())
	stepExecution := testutil.NewTestStepExecution(jobExecution, "requiresNewStep")
	
	// 2. 外部トランザクションのシミュレーション
	externalTx := &MockTx{}
	ctxWithExternalTx := context.WithValue(ctx, "tx", externalTx)
	
	// 3. MockStep の定義 (REQUIRES_NEW伝播、失敗をシミュレート)
	simulatedError := errors.New("internal step failure")
	requiresNewStep := &MockStep{
		Repo: mockRepo, // 注入
		IDValue: "requiresNewStep",
		PropagationValue: "REQUIRES_NEW",
		SimulatedError: simulatedError, // 実行時にエラーを発生させる
	}
	
	// 4. 内部トランザクションのモック
	internalTx := &MockTx{}
	
	// 5. 期待値の設定
	
	// 5.1. StepExecution の永続化 (Save/Update)
	mockRepo.MockStepExecutionRepository.On("SaveStepExecution", mock.Anything, mock.AnythingOfType("*model.StepExecution")).Return(nil).Once() // JobRunnerの役割をシミュレート
	if err := mockRepo.SaveStepExecution(ctx, stepExecution); err != nil {
		t.Fatalf("Failed to simulate initial SaveStepExecution: %v", err)
	}
	
	mockRepo.MockStepExecutionRepository.On("UpdateStepExecution", mock.Anything, mock.AnythingOfType("*model.StepExecution")).Return(nil).Twice() // STARTED (1回目) -> FAILED (2回目)
	
	// 5.2. REQUIRES_NEW トランザクションの期待値
	
	// TxManager は新しいトランザクションを開始することを期待
	mockTxManager.On("Begin", mock.Anything, mock.Anything).Return(internalTx, nil).Once()
	
	// Stepが失敗した場合、内部トランザクションはロールバックされることを期待
	mockTxManager.On("Rollback", internalTx).Return(nil).Once()
	
	// Commit は呼ばれないことを期待
	mockTxManager.AssertNotCalled(t, "Commit")
	
	// 外部トランザクション (externalTx) はサスペンドされるため、影響を受けないことを期待
	externalTx.AssertNotCalled(t, "Commit")
	externalTx.AssertNotCalled(t, "Rollback")
	externalTx.AssertNotCalled(t, "Savepoint")
	
	// 6. SimpleStepExecutor の実行
	executor := partition.NewSimpleStepExecutor(metrics.NewNoOpTracer(), metrics.NewNoOpMetricRecorder(), mockTxManager) 
	
	resultExec, err := executor.ExecuteStep(ctxWithExternalTx, requiresNewStep, jobExecution, stepExecution)
	
	// 7. アサーション
	assert.Error(t, err)
	assert.Contains(t, err.Error(), simulatedError.Error())
	assert.Equal(t, model.BatchStatusFailed, resultExec.Status)
	
	// 8. モックの検証
	mockTxManager.AssertExpectations(t)
	mockRepo.MockStepExecutionRepository.AssertExpectations(t)
}
