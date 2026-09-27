package semantics_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	dbconfig "github.com/tigerroll/surfin/pkg/batch/adapter/database/config"
	port "github.com/tigerroll/surfin/pkg/batch/core/application/port"
	"github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	"github.com/tigerroll/surfin/pkg/batch/core/domain/repository"
	"github.com/tigerroll/surfin/pkg/batch/support/util/exception"
	"github.com/tigerroll/surfin/test/pkg/batch/test"
)

// TestFailureMatrix_CommitFailure verifies system behavior when a transaction commit fails,
// ensuring proper rollback and error propagation.
// This test corresponds to the "Write | Fatal Failure" case in the Failure Matrix.
func TestFailureMatrix_CommitFailure(t *testing.T) {
	step, reader, processor, writer, repo, txManager, metricRecorder, tracer, _, dbConn, _ := test.SetupChunkStep(t)

	// Set timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Reset mocks
	txManager.Mock.ExpectedCalls = nil
	repo.Mock.ExpectedCalls = nil

	// Setup processor expectations
	processor.On("Process", mock.Anything, mock.Anything).Return("processed_item1", nil).Maybe()
	processor.On("GetExecutionContext", mock.Anything).Return(model.NewExecutionContext(), nil).Maybe()

	// Setup metric recorder expectations
	metricRecorder.On("RecordItemRead", mock.Anything, mock.Anything, mock.Anything).Maybe()
	metricRecorder.On("RecordItemProcess", mock.Anything, mock.Anything, mock.Anything).Maybe()
	metricRecorder.On("RecordItemWrite", mock.Anything, mock.Anything, mock.Anything).Maybe()
	metricRecorder.On("RecordExecutionError", mock.Anything, mock.Anything).Maybe()

	// Setup tracer expectations
	tracer.On("RecordError", mock.Anything, mock.Anything, mock.Anything).Maybe()

	// Setup DB connection expectations
	dbConn.On("IsTableNotExistError", mock.Anything).Return(false).Maybe()
	dbConn.On("Config").Return(dbconfig.DatabaseConfig{}).Maybe()

	// Setup Open expectations
	reader.On("Open", mock.Anything, mock.Anything).Return(nil).Once()
	writer.On("Open", mock.Anything, mock.Anything).Return(nil).Once()

	// Setup Read expectations
	reader.On("Read", mock.Anything).Return("item1", nil).Once()
	reader.On("Read", mock.Anything).Return(nil, port.ErrNoMoreItems).Maybe()

	// Setup writer metadata expectations
	writer.On("GetTargetResourceName").Return("mock_db").Maybe()
	writer.On("GetResourcePath").Return("mock_path").Maybe()
	// Setup Write expectations
	writer.On("Write", mock.Anything, mock.Anything).Return(nil).Maybe()

	// Setup GetExecutionContext expectations
	reader.On("GetExecutionContext", mock.Anything).Return(model.NewExecutionContext(), nil).Maybe()
	writer.On("GetExecutionContext", mock.Anything).Return(model.NewExecutionContext(), nil).Maybe()

	// Setup Update expectations
	reader.On("Update", mock.Anything, mock.Anything).Return(nil).Maybe()
	processor.On("Update", mock.Anything, mock.Anything).Return(nil).Maybe()
	writer.On("Update", mock.Anything, mock.Anything).Return(nil).Maybe()

	// Setup FindCheckpointData expectations
	repo.On("FindCheckpointData", mock.Anything, mock.Anything).Return(nil, repository.ErrCheckpointDataNotFound).Maybe()

	// Setup Begin expectations
	mockTx := new(test.MockTx)
	mockTx.On("IsTableNotExistError", mock.Anything).Return(false).Maybe()
	mockTx.On("Config").Return(dbconfig.DatabaseConfig{}).Maybe()
	txManager.On("Begin", mock.Anything, mock.Anything).Return(mockTx, nil).Once()

	// Configure to fail on commit
	txManager.On("Commit", mock.Anything).Return(errors.New("db commit failed")).Once()
	repo.On("UpdateStepExecution", mock.Anything, mock.Anything).Return(nil).Maybe()

	// Setup Close expectations
	reader.On("Close", mock.Anything).Return(nil).Maybe()
	writer.On("Close", mock.Anything).Return(nil).Maybe()

	jobExec := model.NewJobExecution("1", "job", model.NewJobParameters())
	err := step.Execute(ctx, jobExec, model.NewStepExecution("1", jobExec, "step"))

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Failed to commit transaction")
}

// TestFailureMatrix_Write_TransientError verifies that a transient write error triggers a chunk retry.
// This test corresponds to the "Write | Transient Failure" case in the Failure Matrix.
func TestFailureMatrix_Write_TransientError(t *testing.T) {
	step, reader, processor, writer, repo, txManager, metricRecorder, tracer, _, _, _ := test.SetupChunkStep(t)

	// Set timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Reset mocks
	writer.Mock.ExpectedCalls = nil
	txManager.Mock.ExpectedCalls = nil
	reader.Mock.ExpectedCalls = nil
	repo.Mock.ExpectedCalls = nil
	metricRecorder.Mock.ExpectedCalls = nil
	tracer.Mock.ExpectedCalls = nil

	// Setup reader/writer expectations
	reader.On("GetExecutionContext", mock.Anything).Return(model.NewExecutionContext(), nil).Maybe()
	writer.On("GetExecutionContext", mock.Anything).Return(model.NewExecutionContext(), nil).Maybe()

	// Setup tracer expectations
	tracer.On("RecordError", mock.Anything, mock.Anything, mock.Anything).Maybe()

	// Setup metric recorder expectations
	metricRecorder.On("RecordItemRead", mock.Anything, mock.Anything, mock.Anything).Maybe()
	metricRecorder.On("RecordItemProcess", mock.Anything, mock.Anything, mock.Anything).Maybe()
	metricRecorder.On("RecordItemWrite", mock.Anything, mock.Anything, mock.Anything).Maybe()
	metricRecorder.On("RecordExecutionError", mock.Anything, mock.Anything).Maybe()
	metricRecorder.On("RecordItemRetry", mock.Anything, mock.Anything, mock.Anything).Maybe()

	// Setup writer metadata expectations
	writer.On("GetTargetResourceName").Return("mock_db").Maybe()
	writer.On("GetResourcePath").Return("mock_path").Maybe()

	// Setup read expectations
	// 1. Define read expectations (Times)
	// Set sufficient count (30) to allow for retries
	reader.On("Read", mock.Anything).Return("item1", nil).Times(30)
	// 2. Define EOF expectation (Once)
	reader.On("Read", mock.Anything).Return(nil, port.ErrNoMoreItems).Once()

	// Setup write expectations
	// 1. Define failure expectation (Once)
	transientErr := exception.NewBatchError("writer", "transient write failure", errors.New("db deadlock"), false, true)
	writer.On("Write", mock.Anything, mock.Anything).Return(transientErr).Once()
	// 2. Define success expectation (Maybe) to handle retries and subsequent chunk processing
	writer.On("Write", mock.Anything, mock.Anything).Return(nil).Maybe()

	// Setup transaction expectations
	mockTx := new(test.MockTx)
	mockTx.On("IsTableNotExistError", mock.Anything).Return(false).Maybe()
	mockTx.On("Config").Return(dbconfig.DatabaseConfig{}).Maybe()

	txManager.On("Begin", mock.Anything, mock.Anything).Return(mockTx, nil).Maybe()
	txManager.On("Commit", mock.Anything).Return(nil).Maybe()
	txManager.On("Rollback", mock.Anything).Return(nil).Maybe()

	// Setup update expectations
	reader.On("Update", mock.Anything, mock.Anything).Return(nil).Maybe()
	processor.On("Update", mock.Anything, mock.Anything).Return(nil).Maybe()
	writer.On("Update", mock.Anything, mock.Anything).Return(nil).Maybe()

	// Other expectations (Maybe)
	processor.On("Process", mock.Anything, mock.Anything).Return("processed_item1", nil).Maybe()
	repo.On("UpdateStepExecution", mock.Anything, mock.Anything).Return(nil).Maybe()

	// Setup FindCheckpointData expectations
	repo.On("FindCheckpointData", mock.Anything, mock.Anything).Return(nil, repository.ErrCheckpointDataNotFound).Maybe()

	reader.On("Open", mock.Anything, mock.Anything).Return(nil).Maybe()
	writer.On("Open", mock.Anything, mock.Anything).Return(nil).Maybe()
	reader.On("Close", mock.Anything).Return(nil).Maybe()
	writer.On("Close", mock.Anything).Return(nil).Maybe()

	jobExec := model.NewJobExecution("1", "job", model.NewJobParameters())
	err := step.Execute(ctx, jobExec, model.NewStepExecution("1", jobExec, "step"))

	assert.NoError(t, err)
	// Update expectation count to 3
	writer.AssertNumberOfCalls(t, "Write", 3)
}

// TestFailureMatrix_CheckpointSaveFailure verifies system behavior when checkpoint persistence fails,
// ensuring the step handles the error according to the defined policy.
func TestFailureMatrix_CheckpointSaveFailure(t *testing.T) {
	step, reader, processor, writer, repo, txManager, metricRecorder, tracer, _, dbConn, _ := test.SetupChunkStep(t)

	// Set timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Reset mocks
	repo.Mock.ExpectedCalls = nil
	reader.Mock.ExpectedCalls = nil
	txManager.Mock.ExpectedCalls = nil

	// Add required expectations
	reader.On("Open", mock.Anything, mock.Anything).Return(nil).Maybe()
	writer.On("Open", mock.Anything, mock.Anything).Return(nil).Maybe()

	// Read: 1 success, 1 EOF
	reader.On("Read", mock.Anything).Return("item1", nil).Once()
	reader.On("Read", mock.Anything).Return(nil, port.ErrNoMoreItems).Once()

	// SaveCheckpointData: 1 failure
	repo.On("SaveCheckpointData", mock.Anything, mock.Anything).Return(errors.New("disk full")).Once()
	repo.On("FindCheckpointData", mock.Anything, mock.Anything).Return(nil, repository.ErrCheckpointDataNotFound).Maybe()
	repo.On("UpdateStepExecution", mock.Anything, mock.Anything).Return(nil).Maybe()

	// Setup processor expectations
	processor.On("Process", mock.Anything, mock.Anything).Return("processed_item1", nil).Maybe()
	processor.On("GetExecutionContext", mock.Anything).Return(model.NewExecutionContext(), nil).Maybe()

	// Setup metric recorder expectations
	metricRecorder.On("RecordItemRead", mock.Anything, mock.Anything, mock.Anything).Maybe()
	metricRecorder.On("RecordItemProcess", mock.Anything, mock.Anything, mock.Anything).Maybe()
	metricRecorder.On("RecordItemWrite", mock.Anything, mock.Anything, mock.Anything).Maybe()
	metricRecorder.On("RecordExecutionError", mock.Anything, mock.Anything).Maybe()

	// Setup tracer expectations
	tracer.On("RecordError", mock.Anything, mock.Anything, mock.Anything).Maybe()

	// Setup DB connection expectations
	dbConn.On("IsTableNotExistError", mock.Anything).Return(false).Maybe()
	dbConn.On("Config").Return(dbconfig.DatabaseConfig{}).Maybe()

	// Setup writer metadata expectations
	writer.On("GetTargetResourceName").Return("mock_db").Maybe()
	writer.On("GetResourcePath").Return("mock_path").Maybe()
	// Setup Write expectations
	writer.On("Write", mock.Anything, mock.Anything).Return(nil).Maybe()

	// Add dummy data to ExecutionContext to trigger saveCheckpoint
	ec := model.NewExecutionContext()
	ec.Put("key", "value")

	// Setup GetExecutionContext expectations
	reader.On("GetExecutionContext", mock.Anything).Return(ec, nil).Maybe()
	writer.On("GetExecutionContext", mock.Anything).Return(ec, nil).Maybe()

	// Setup Update expectations
	reader.On("Update", mock.Anything, mock.Anything).Return(nil).Maybe()
	processor.On("Update", mock.Anything, mock.Anything).Return(nil).Maybe()
	writer.On("Update", mock.Anything, mock.Anything).Return(nil).Maybe()

	// Transaction: 1 Begin, 1 Rollback (on failure)
	mockTx := new(test.MockTx)
	mockTx.On("IsTableNotExistError", mock.Anything).Return(false).Maybe()
	mockTx.On("Config").Return(dbconfig.DatabaseConfig{}).Maybe()
	txManager.On("Begin", mock.Anything, mock.Anything).Return(mockTx, nil).Once()
	txManager.On("Rollback", mock.Anything).Return(nil).Once()
	// Ensure Commit is called using Once()
	txManager.On("Commit", mock.Anything).Return(nil).Once()

	// Setup Close expectations
	reader.On("Close", mock.Anything).Return(nil).Maybe()
	writer.On("Close", mock.Anything).Return(nil).Maybe()

	jobExec := model.NewJobExecution("1", "job", model.NewJobParameters())
	err := step.Execute(ctx, jobExec, model.NewStepExecution("1", jobExec, "step"))

	// Check if err is not nil before calling Error()
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "disk full")
	}
}
