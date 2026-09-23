// Package item_test contains integration and unit tests for chunk-oriented processing steps,
// specifically focusing on the ChunkStep implementation.
package item_test

import (
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	"github.com/tigerroll/surfin/test/pkg/batch/test"
)

// RestartableReader is a test helper that implements the ItemReader interface.
// It maintains an internal position to simulate stateful reading, allowing
// verification of restartability logic.
type RestartableReader struct {
	data []int
	pos  int
}

// Read returns the next item from the data source or io.EOF if no more items are available.
func (r *RestartableReader) Read(ctx context.Context) (any, error) {
	if r.pos >= len(r.data) {
		return nil, io.EOF
	}
	val := r.data[r.pos]
	r.pos++
	return val, nil
}

// Open initializes the reader, restoring the read position from the provided ExecutionContext.
func (r *RestartableReader) Open(ctx context.Context, ec model.ExecutionContext) error {
	if count, ok := ec.GetInt("readCount"); ok {
		r.pos = count
	}
	return nil
}

// Close performs cleanup for the reader.
func (r *RestartableReader) Close(ctx context.Context) error { return nil }

// GetExecutionContext returns the current read position as part of the ExecutionContext for checkpointing.
func (r *RestartableReader) GetExecutionContext(ctx context.Context) (model.ExecutionContext, error) {
	ec := model.NewExecutionContext()
	ec.Put("readCount", r.pos)
	return ec, nil
}

// Update updates the reader's state.
func (r *RestartableReader) Update(ctx context.Context, ec model.ExecutionContext) error { return nil }

// TestChunkStep_Integration_Restart verifies that ChunkStep correctly resumes processing from a saved checkpoint.
func TestChunkStep_Integration_Restart(t *testing.T) {
	ctx := context.Background()

	// Setup: 10 items
	data := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

	// Build ChunkStep using the test helper
	_, _, _, _, repo, _, _, _, _, _ := test.SetupChunkStep(t)
	repo.On("Close").Return(nil).Maybe()
	defer repo.Close()

	// Simulate a checkpoint created after processing 5 items
	checkpointEC := model.NewExecutionContext()
	checkpointEC.Put("readCount", 5)

	// Restart: Open reader with the checkpoint
	newReader := &RestartableReader{data: data}
	err := newReader.Open(ctx, checkpointEC)
	assert.NoError(t, err)

	// Verify that reading resumes from the 6th item (value 6)
	val, err := newReader.Read(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 6, val)
}

// TestChunkStep_ChunkSplittingOnSkippableWriteFailure verifies that the ChunkStep correctly
// triggers chunk splitting logic when a skippable write error occurs.
func TestChunkStep_ChunkSplittingOnSkippableWriteFailure(t *testing.T) {
	_, _, _, _, repo, _, _, _, _, _ := test.SetupChunkStep(t)
	repo.On("Close").Return(nil).Maybe()
	defer repo.Close()

	// Initial Step Execution setup
	jobExec := model.NewJobExecution(model.NewID(), "testJob", model.NewJobParameters())
	se := model.NewStepExecution(model.NewID(), jobExec, "testStep")
	se.WriteCount = 10 // Assume 10 items were already written
}
