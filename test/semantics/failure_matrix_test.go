package semantics_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	dbconfig "github.com/tigerroll/surfin/pkg/batch/adapter/database/config"
	port "github.com/tigerroll/surfin/pkg/batch/core/application/port"
	"github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	"github.com/tigerroll/surfin/pkg/batch/core/domain/repository"
	"github.com/tigerroll/surfin/test/pkg/batch/test"
)

// TestFailureMatrix_CommitFailure verifies the system behavior when a transaction commit fails.
func TestFailureMatrix_CommitFailure(t *testing.T) {
	step, reader, processor, writer, repo, txManager, metricRecorder, tracer, _, dbConn := test.SetupChunkStep(t)
	ctx := context.Background()

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

// TestFailureMatrix_CheckpointSaveFailure verifies the system behavior when checkpoint saving fails.
func TestFailureMatrix_CheckpointSaveFailure(t *testing.T) {
	step, reader, processor, writer, repo, txManager, metricRecorder, tracer, _, dbConn := test.SetupChunkStep(t)
	ctx := context.Background()

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

	// Setup Commit expectations (return nil for success)
	txManager.On("Commit", mock.Anything).Return(nil).Once()

	// Configure to fail on checkpoint save
	repo.On("SaveCheckpointData", mock.Anything, mock.Anything).Return(errors.New("disk full")).Once()
	repo.On("UpdateStepExecution", mock.Anything, mock.Anything).Return(nil).Maybe()

	// Setup Close expectations
	reader.On("Close", mock.Anything).Return(nil).Maybe()
	writer.On("Close", mock.Anything).Return(nil).Maybe()

	jobExec := model.NewJobExecution("1", "job", model.NewJobParameters())
	err := step.Execute(ctx, jobExec, model.NewStepExecution("1", jobExec, "step"))

	assert.NoError(t, err)
}

// TestFailureMatrix_CloseFailure verifies the system behavior when closing a resource fails.
func TestFailureMatrix_CloseFailure(t *testing.T) {
	step, reader, processor, writer, repo, txManager, metricRecorder, tracer, _, dbConn := test.SetupChunkStep(t)
	ctx := context.Background()

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

	// Setup Commit expectations (return nil for success)
	txManager.On("Commit", mock.Anything).Return(nil).Once()

	// Configure to fail on close
	reader.On("Close", mock.Anything).Return(errors.New("close failed")).Once()
	writer.On("Close", mock.Anything).Return(nil).Maybe() // Writer succeeds
	repo.On("UpdateStepExecution", mock.Anything, mock.Anything).Return(nil).Maybe()

	jobExec := model.NewJobExecution("1", "job", model.NewJobParameters())
	err := step.Execute(ctx, jobExec, model.NewStepExecution("1", jobExec, "step"))

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "close failed")
}

// TestFailureMatrix_UpdateFailure verifies the system behavior when updating a component state fails (Policy C: log and continue).
func TestFailureMatrix_UpdateFailure(t *testing.T) {
	step, reader, processor, writer, repo, txManager, metricRecorder, tracer, _, dbConn := test.SetupChunkStep(t)
	ctx := context.Background()

	// Setup expectations
	processor.On("Process", mock.Anything, mock.Anything).Return("processed_item1", nil).Maybe()
	processor.On("GetExecutionContext", mock.Anything).Return(model.NewExecutionContext(), nil).Maybe()
	metricRecorder.On("RecordItemRead", mock.Anything, mock.Anything, mock.Anything).Maybe()
	metricRecorder.On("RecordItemProcess", mock.Anything, mock.Anything, mock.Anything).Maybe()
	metricRecorder.On("RecordItemWrite", mock.Anything, mock.Anything, mock.Anything).Maybe()
	metricRecorder.On("RecordExecutionError", mock.Anything, mock.Anything).Maybe()
	tracer.On("RecordError", mock.Anything, mock.Anything, mock.Anything).Maybe()
	dbConn.On("IsTableNotExistError", mock.Anything).Return(false).Maybe()
	dbConn.On("Config").Return(dbconfig.DatabaseConfig{}).Maybe()

	reader.On("Open", mock.Anything, mock.Anything).Return(nil).Once()
	writer.On("Open", mock.Anything, mock.Anything).Return(nil).Once()
	reader.On("Read", mock.Anything).Return("item1", nil).Once()
	reader.On("Read", mock.Anything).Return(nil, port.ErrNoMoreItems).Maybe()
	writer.On("GetTargetResourceName").Return("mock_db").Maybe()
	writer.On("GetResourcePath").Return("mock_path").Maybe()
	writer.On("Write", mock.Anything, mock.Anything).Return(nil).Maybe()
	reader.On("GetExecutionContext", mock.Anything).Return(model.NewExecutionContext(), nil).Maybe()
	writer.On("GetExecutionContext", mock.Anything).Return(model.NewExecutionContext(), nil).Maybe()

	// Configure to fail on update
	reader.On("Update", mock.Anything, mock.Anything).Return(errors.New("update failed")).Once()
	// Add success case as well
	reader.On("Update", mock.Anything, mock.Anything).Return(nil).Maybe()
	processor.On("Update", mock.Anything, mock.Anything).Return(nil).Maybe()
	writer.On("Update", mock.Anything, mock.Anything).Return(nil).Maybe()

	repo.On("FindCheckpointData", mock.Anything, mock.Anything).Return(nil, repository.ErrCheckpointDataNotFound).Maybe()
	mockTx := new(test.MockTx)
	mockTx.On("IsTableNotExistError", mock.Anything).Return(false).Maybe()
	mockTx.On("Config").Return(dbconfig.DatabaseConfig{}).Maybe()
	txManager.On("Begin", mock.Anything, mock.Anything).Return(mockTx, nil).Once()
	txManager.On("Commit", mock.Anything).Return(nil).Once()
	repo.On("UpdateStepExecution", mock.Anything, mock.Anything).Return(nil).Maybe()
	reader.On("Close", mock.Anything).Return(nil).Maybe()
	writer.On("Close", mock.Anything).Return(nil).Maybe()

	jobExec := model.NewJobExecution("1", "job", model.NewJobParameters())
	err := step.Execute(ctx, jobExec, model.NewStepExecution("1", jobExec, "step"))

	// Update failure is not fatal, so no error should be returned
	assert.NoError(t, err)
}
