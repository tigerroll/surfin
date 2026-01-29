package partition_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	port "github.com/tigerroll/surfin/pkg/batch/core/application/port"
	model "github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	metrics "github.com/tigerroll/surfin/pkg/batch/core/metrics"
	partition "github.com/tigerroll/surfin/pkg/batch/engine/step/partition"
	testutil "github.com/tigerroll/surfin/pkg/batch/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockRemoteJobSubmitter is a mock implementation of the port.RemoteJobSubmitter interface.
type MockRemoteJobSubmitter struct {
	mock.Mock
}

func (m *MockRemoteJobSubmitter) SubmitWorkerJob(ctx context.Context, jobExecution *model.JobExecution, stepExecution *model.StepExecution, workerStep port.Step) (string, error) {
	args := m.Called(ctx, jobExecution, stepExecution, workerStep)
	return args.String(0), args.Error(1)
}

func (m *MockRemoteJobSubmitter) AwaitCompletion(ctx context.Context, remoteJobID string, stepExecution *model.StepExecution) error {
	args := m.Called(ctx, remoteJobID, stepExecution)
	return args.Error(0)
}

// MockStep (minimal implementation) can be reused from flow_control_test.go, but defined locally here.
type MockWorkerStep struct {
	IDValue string
}

func (m *MockWorkerStep) ID() string       { return m.IDValue }
func (m *MockWorkerStep) StepName() string { return m.IDValue }
func (m *MockWorkerStep) Execute(ctx context.Context, jobExecution *model.JobExecution, stepExecution *model.StepExecution) error {
	return nil
}
func (m *MockWorkerStep) GetExecutionContextPromotion() *model.ExecutionContextPromotion { return nil }
func (m *MockWorkerStep) GetTransactionOptions() *sql.TxOptions                          { return nil }
func (m *MockWorkerStep) GetPropagation() string                                         { return "" }
func (m *MockWorkerStep) SetMetricRecorder(recorder metrics.MetricRecorder)              {}
func (m *MockWorkerStep) SetTracer(tracer metrics.Tracer)                                {}

// TestRemoteStepExecutor_SuccessfulExecution verifies that remote execution succeeds and results are merged.
func TestRemoteStepExecutor_SuccessfulExecution(t *testing.T) {
	ctx := context.Background()

	// 1. Setup
	mockSubmitter := new(MockRemoteJobSubmitter)
	mockTracer := metrics.NewNoOpTracer()

	executor := partition.NewRemoteStepExecutor(mockSubmitter, mockTracer)

	jobExecution := testutil.NewTestJobExecution("jobInstID", "remoteJob", model.NewJobParameters())
	stepExecution := testutil.NewTestStepExecution(jobExecution, "remoteWorkerStep")
	workerStep := &MockWorkerStep{IDValue: "workerStep"}

	remoteJobID := "bird-job-12345"

	// 2. Set Expectations

	// 2.1. SubmitWorkerJob expectations
	mockSubmitter.On("SubmitWorkerJob", mock.Anything, jobExecution, stepExecution, workerStep).Return(remoteJobID, nil).Once()

	// 2.2. AwaitCompletion expectations
	// Simulate AwaitCompletion updating StepExecution and returning nil error
	mockSubmitter.On("AwaitCompletion", mock.Anything, remoteJobID, stepExecution).Return(nil).Run(func(args mock.Arguments) {
		// Simulate updating StepExecution after successful remote execution
		se := args.Get(2).(*model.StepExecution)

		// Update statistics and EC
		se.ReadCount = 100
		se.WriteCount = 90
		se.ExecutionContext.Put("remote_status", "COMPLETED")
		se.ExecutionContext.Put("remote_read_count", 100)
		se.Status = model.BatchStatusCompleted
		se.ExitStatus = model.ExitStatusCompleted
		now := time.Now()
		se.EndTime = &now
		se.LastUpdated = now
	}).Once()

	// 3. Execute
	resultExec, err := executor.ExecuteStep(ctx, workerStep, jobExecution, stepExecution)

	// 4. Assertions
	assert.NoError(t, err)
	assert.Equal(t, model.BatchStatusCompleted, resultExec.Status)
	assert.Equal(t, 100, resultExec.ReadCount)
	assert.Equal(t, 90, resultExec.WriteCount)

	remoteStatus, ok := resultExec.ExecutionContext.GetString("remote_status")
	assert.True(t, ok)
	assert.Equal(t, "COMPLETED", remoteStatus)

	// 5. Verify mocks
	mockSubmitter.AssertExpectations(t)
}

// TestRemoteStepExecutor_SubmissionFailure verifies the case where remote job submission fails.
func TestRemoteStepExecutor_SubmissionFailure(t *testing.T) {
	ctx := context.Background()

	// 1. Setup
	mockSubmitter := new(MockRemoteJobSubmitter)
	mockTracer := metrics.NewNoOpTracer()
	executor := partition.NewRemoteStepExecutor(mockSubmitter, mockTracer)

	jobExecution := testutil.NewTestJobExecution("jobInstID", "remoteJob", model.NewJobParameters())
	stepExecution := testutil.NewTestStepExecution(jobExecution, "remoteWorkerStep")
	workerStep := &MockWorkerStep{IDValue: "workerStep"}

	submissionError := errors.New("API submission failed")

	// 2. Set Expectations
	mockSubmitter.On("SubmitWorkerJob", mock.Anything, jobExecution, stepExecution, workerStep).Return("", submissionError).Once()
	mockSubmitter.AssertNotCalled(t, "AwaitCompletion")

	// 3. Execute
	resultExec, err := executor.ExecuteStep(ctx, workerStep, jobExecution, stepExecution)

	// 4. Assertions
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to submit remote worker")

	// Execution errors are not reflected in StepExecution (SubmitWorkerJob does not change StepExecution status)
	assert.Equal(t, model.BatchStatusStarting, resultExec.Status)

	// 5. Verify mocks
	mockSubmitter.AssertExpectations(t)
}

// TestRemoteStepExecutor_AwaitCompletionFailure verifies the case where an error occurs while waiting for remote job completion.
func TestRemoteStepExecutor_AwaitCompletionFailure(t *testing.T) {
	ctx := context.Background()

	// 1. Setup
	mockSubmitter := new(MockRemoteJobSubmitter)
	mockTracer := metrics.NewNoOpTracer()
	executor := partition.NewRemoteStepExecutor(mockSubmitter, mockTracer)

	jobExecution := testutil.NewTestJobExecution("jobInstID", "remoteJob", model.NewJobParameters())
	stepExecution := testutil.NewTestStepExecution(jobExecution, "remoteWorkerStep")
	workerStep := &MockWorkerStep{IDValue: "workerStep"}

	remoteJobID := "bird-job-12345"
	completionError := errors.New("remote worker failed during execution")

	// 2. Set Expectations
	mockSubmitter.On("SubmitWorkerJob", mock.Anything, jobExecution, stepExecution, workerStep).Return(remoteJobID, nil).Once()

	// Simulate AwaitCompletion returning an error
	mockSubmitter.On("AwaitCompletion", mock.Anything, remoteJobID, stepExecution).Return(completionError).Once()

	// 3. Execute
	resultExec, err := executor.ExecuteStep(ctx, workerStep, jobExecution, stepExecution)

	// 4. Assertions
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "an error occurred while waiting for remote worker completion")

	// If AwaitCompletion returns an error, the state of StepExecution depends on the remote state,
	// but RemoteStepExecutor wraps and returns the error.
	// Since RemoteJobSubmitter.AwaitCompletion is expected to update StepExecution status before returning an error,
	// here we verify that StepExecution status remains Starting (because the RemoteJobSubmitter mock does not simulate status update).
	// In a real RemoteJobSubmitter, it would update StepExecution to FAILED on remote failure before returning an error.
	// The purpose of this test is to verify RemoteStepExecutor's logic, so we confirm that AwaitCompletion returned an error.
	assert.Equal(t, model.BatchStatusStarting, resultExec.Status)

	// 5. Verify mocks
	mockSubmitter.AssertExpectations(t)
}

// Verify interfaces
var _ port.RemoteJobSubmitter = (*MockRemoteJobSubmitter)(nil)
