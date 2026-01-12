package partition_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	port "surfin/pkg/batch/core/application/port"
	model "surfin/pkg/batch/core/domain/model"
	metrics "surfin/pkg/batch/core/metrics"
	partition "surfin/pkg/batch/engine/step/partition"
	testutil "surfin/pkg/batch/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockRemoteJobSubmitter は port.RemoteJobSubmitter のモック実装です。
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

// MockStep (最小限の実装) は flow_control_test.go から再利用可能ですが、ここではローカルに定義します。
type MockWorkerStep struct {
	IDValue string
}

func (m *MockWorkerStep) ID() string       { return m.IDValue }
func (m *MockWorkerStep) StepName() string { return m.IDValue }
func (m *MockWorkerStep) Execute(ctx context.Context, jobExecution *model.JobExecution, stepExecution *model.StepExecution) error {
	return nil
}
func (m *MockWorkerStep) GetTransactionOptions() *sql.TxOptions             { return nil }
func (m *MockWorkerStep) GetPropagation() string                            { return "" }
func (m *MockWorkerStep) SetMetricRecorder(recorder metrics.MetricRecorder) {}
func (m *MockWorkerStep) SetTracer(tracer metrics.Tracer)                   {}

// TestRemoteStepExecutor_SuccessfulExecution はリモート実行が成功し、結果がマージされることを検証します。
func TestRemoteStepExecutor_SuccessfulExecution(t *testing.T) {
	ctx := context.Background()

	// 1. セットアップ
	mockSubmitter := new(MockRemoteJobSubmitter)
	mockTracer := metrics.NewNoOpTracer()

	executor := partition.NewRemoteStepExecutor(mockSubmitter, mockTracer)

	jobExecution := testutil.NewTestJobExecution("jobInstID", "remoteJob", model.NewJobParameters())
	stepExecution := testutil.NewTestStepExecution(jobExecution, "remoteWorkerStep")
	workerStep := &MockWorkerStep{IDValue: "workerStep"}

	remoteJobID := "bird-job-12345"

	// 2. 期待値の設定

	// 2.1. SubmitWorkerJob の期待値
	mockSubmitter.On("SubmitWorkerJob", mock.Anything, jobExecution, stepExecution, workerStep).Return(remoteJobID, nil).Once()

	// 2.2. AwaitCompletion の期待値
	// AwaitCompletion は StepExecution を更新し、nil エラーを返すことをシミュレート
	mockSubmitter.On("AwaitCompletion", mock.Anything, remoteJobID, stepExecution).Return(nil).Run(func(args mock.Arguments) {
		// リモート実行が成功した後の状態をシミュレートして StepExecution を更新
		se := args.Get(2).(*model.StepExecution)

		// 統計情報と EC の更新
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

	// 3. 実行
	resultExec, err := executor.ExecuteStep(ctx, workerStep, jobExecution, stepExecution)

	// 4. アサーション
	assert.NoError(t, err)
	assert.Equal(t, model.BatchStatusCompleted, resultExec.Status)
	assert.Equal(t, 100, resultExec.ReadCount)
	assert.Equal(t, 90, resultExec.WriteCount)

	remoteStatus, ok := resultExec.ExecutionContext.GetString("remote_status")
	assert.True(t, ok)
	assert.Equal(t, "COMPLETED", remoteStatus)

	// 5. モックの検証
	mockSubmitter.AssertExpectations(t)
}

// TestRemoteStepExecutor_SubmissionFailure はリモートジョブの投入が失敗した場合を検証します。
func TestRemoteStepExecutor_SubmissionFailure(t *testing.T) {
	ctx := context.Background()

	// 1. セットアップ
	mockSubmitter := new(MockRemoteJobSubmitter)
	mockTracer := metrics.NewNoOpTracer()
	executor := partition.NewRemoteStepExecutor(mockSubmitter, mockTracer)

	jobExecution := testutil.NewTestJobExecution("jobInstID", "remoteJob", model.NewJobParameters())
	stepExecution := testutil.NewTestStepExecution(jobExecution, "remoteWorkerStep")
	workerStep := &MockWorkerStep{IDValue: "workerStep"}

	submissionError := errors.New("API submission failed")

	// 2. 期待値の設定
	mockSubmitter.On("SubmitWorkerJob", mock.Anything, jobExecution, stepExecution, workerStep).Return("", submissionError).Once()
	mockSubmitter.AssertNotCalled(t, "AwaitCompletion")

	// 3. 実行
	resultExec, err := executor.ExecuteStep(ctx, workerStep, jobExecution, stepExecution)

	// 4. アサーション
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to submit remote worker")

	// 実行エラーは StepExecution に反映されない (SubmitWorkerJob は StepExecution の状態を変更しないため)
	assert.Equal(t, model.BatchStatusStarting, resultExec.Status)

	// 5. モックの検証
	mockSubmitter.AssertExpectations(t)
}

// TestRemoteStepExecutor_AwaitCompletionFailure はリモートジョブの待機中にエラーが発生した場合を検証します。
func TestRemoteStepExecutor_AwaitCompletionFailure(t *testing.T) {
	ctx := context.Background()

	// 1. セットアップ
	mockSubmitter := new(MockRemoteJobSubmitter)
	mockTracer := metrics.NewNoOpTracer()
	executor := partition.NewRemoteStepExecutor(mockSubmitter, mockTracer)

	jobExecution := testutil.NewTestJobExecution("jobInstID", "remoteJob", model.NewJobParameters())
	stepExecution := testutil.NewTestStepExecution(jobExecution, "remoteWorkerStep")
	workerStep := &MockWorkerStep{IDValue: "workerStep"}

	remoteJobID := "bird-job-12345"
	completionError := errors.New("remote worker failed during execution")

	// 2. 期待値の設定
	mockSubmitter.On("SubmitWorkerJob", mock.Anything, jobExecution, stepExecution, workerStep).Return(remoteJobID, nil).Once()

	// AwaitCompletion がエラーを返すことをシミュレート
	mockSubmitter.On("AwaitCompletion", mock.Anything, remoteJobID, stepExecution).Return(completionError).Once()

	// 3. 実行
	resultExec, err := executor.ExecuteStep(ctx, workerStep, jobExecution, stepExecution)

	// 4. アサーション
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "an error occurred while waiting for remote worker completion")

	// AwaitCompletion がエラーを返した場合、StepExecution の状態はリモートの状態に依存するが、
	// RemoteStepExecutor はエラーをラップして返す。
	// RemoteJobSubmitter.AwaitCompletion が StepExecution の状態を更新してからエラーを返すことが期待されるため、
	// ここでは StepExecution の状態が Starting のままであることを確認する (RemoteJobSubmitter のモックが状態更新をシミュレートしていないため)。
	// 実際の RemoteJobSubmitter は、リモート失敗時に StepExecution を FAILED に更新してからエラーを返す。
	// テストの目的は RemoteStepExecutor のロジック検証なので、AwaitCompletion がエラーを返したことを確認する。
	assert.Equal(t, model.BatchStatusStarting, resultExec.Status)

	// 5. モックの検証
	mockSubmitter.AssertExpectations(t)
}

// Verify interfaces
var _ port.RemoteJobSubmitter = (*MockRemoteJobSubmitter)(nil)
