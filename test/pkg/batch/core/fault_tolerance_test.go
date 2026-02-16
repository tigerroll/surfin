package core_test

import (
	"context"
	"errors"
	"testing"

	port "github.com/tigerroll/surfin/pkg/batch/core/application/port"
	model "github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	"github.com/tigerroll/surfin/pkg/batch/support/util/exception"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// --- Mock Components for Chunk Processing ---

type TestItem struct {
	ID    int
	Value string
}

// MockReader simulates ItemReader behavior with controlled failures.
type MockReader struct {
	readCount int
	failAt    int
	failCount int // Number of times to fail retryable errors before succeeding
	skipAt    int
	items     []TestItem
	ec        model.ExecutionContext
}

func (m *MockReader) IncrementReadCount() {
	m.readCount++
}

func NewMockReader(items []TestItem, failAt, skipAt int, failCount int) *MockReader {
	return &MockReader{
		items:     items,
		failAt:    failAt,
		skipAt:    skipAt,
		ec:        model.NewExecutionContext(),
		failCount: failCount,
	}
}

func (m *MockReader) Open(ctx context.Context, ec model.ExecutionContext) error {
	m.ec = ec
	if current, ok := ec.GetInt("readCount"); ok {
		m.readCount = current
	}
	return nil
}
func (m *MockReader) Close(ctx context.Context) error { return nil }
func (m *MockReader) SetExecutionContext(ctx context.Context, ec model.ExecutionContext) error {
	m.ec = ec
	return nil
}
func (m *MockReader) GetExecutionContext(ctx context.Context) (model.ExecutionContext, error) {
	m.ec.Put("readCount", m.readCount)
	return m.ec, nil
}

func (m *MockReader) Read(ctx context.Context) (TestItem, error) {
	if m.readCount >= len(m.items) {
		return TestItem{}, port.ErrNoMoreItems // Use port.ErrNoMoreItems
	}

	item := m.items[m.readCount]

	// Check for failure/skip based on item ID
	if item.ID == m.failAt {
		// Retryable error: Do NOT advance m.readCount. Next Read attempt gets the same item.
		if m.failCount > 0 {
			m.failCount--
			return TestItem{}, exception.NewBatchError("reader", "transient read failure", errors.New("db timeout"), false, true)
		}
		// If failCount reached 0, proceed as successful read
	}
	if item.ID == m.skipAt {
		// Skippable error: Do NOT advance m.readCount. The item is skipped, and the next Read attempt gets the next item.
		return TestItem{}, exception.NewBatchError("reader", "skippable read failure", errors.New("bad format"), true, false)
	}

	// Successful read
	m.readCount++
	return item, nil
}

// MockProcessor simulates ItemProcessor behavior with controlled failures.
type MockProcessor struct {
	failAt    int
	failCount int // Number of times to fail retryable errors before succeeding
	skipAt    int
}

func NewMockProcessor(failAt, skipAt, failCount int) *MockProcessor {
	return &MockProcessor{
		failAt:    failAt,
		skipAt:    skipAt,
		failCount: failCount,
	}
}

func (m *MockProcessor) Process(ctx context.Context, item TestItem) (TestItem, error) {
	if item.ID == m.failAt {
		// Retryable error
		if m.failCount > 0 {
			m.failCount--
			return TestItem{}, exception.NewBatchError("processor", "transient process failure", errors.New("service unavailable"), false, true)
		}
	}
	if item.ID == m.skipAt {
		// Skippable error
		return TestItem{}, exception.NewBatchError("processor", "skippable process failure", errors.New("invalid item data"), true, false)
	}
	return item, nil
}
func (m *MockProcessor) SetExecutionContext(ctx context.Context, ec model.ExecutionContext) error {
	return nil
}
func (m *MockProcessor) GetExecutionContext(ctx context.Context) (model.ExecutionContext, error) {
	return model.NewExecutionContext(), nil
}

// MockWriter simulates ItemWriter behavior with controlled failures.
type MockWriter struct {
	failAt       int
	failCount    int // Number of times to fail retryable errors before succeeding
	skipAt       int
	writtenItems []TestItem
}

func (m *MockWriter) Open(ctx context.Context, ec model.ExecutionContext) error { return nil }
func (m *MockWriter) Close(ctx context.Context) error                           { return nil }
func (m *MockWriter) SetExecutionContext(ctx context.Context, ec model.ExecutionContext) error {
	return nil
}
func (m *MockWriter) GetExecutionContext(ctx context.Context) (model.ExecutionContext, error) {
	return model.NewExecutionContext(), nil
}

// ResetWrittenItems simulates transaction rollback by clearing the written items.
func (m *MockWriter) ResetWrittenItems() {
	m.writtenItems = nil
}

func (m *MockWriter) Write(ctx context.Context, items []TestItem) error { // Removed tx.Tx from signature
	shouldFailTransiently := false

	// Check if any item triggers the transient failure AND we still have failures left
	if m.failAt != 0 && m.failCount > 0 {
		for _, item := range items {
			if item.ID == m.failAt {
				shouldFailTransiently = true
				break
			}
		}
	}

	if shouldFailTransiently {
		m.failCount--
		// Return transient error, triggering chunk rollback/retry
		return exception.NewBatchError("writer", "transient write failure", errors.New("db deadlock"), false, true)
	}

	// If no transient failure, proceed with actual writing and checking skippable errors
	for _, item := range items {
		if item.ID == m.skipAt {
			// Skippable error (Write skip usually requires chunk splitting, which we simulate by returning a skippable error)
			// Note: In real Spring Batch, skippable write errors trigger chunk splitting/rollback. Here we simulate the error type.
			return exception.NewBatchError("writer", "skippable write failure", errors.New("duplicate key"), true, false)
		}
		m.writtenItems = append(m.writtenItems, item)
	}
	return nil
}

// --- Chunk Execution Simulator (Simplified for L.2 testing) ---

// SimulateChunkExecution simulates the core loop of a ChunkStep, focusing on fault tolerance logic.
// It takes maxRetries and maxSkips configuration.
func SimulateChunkExecution(
	ctx context.Context,
	se *model.StepExecution,
	reader *MockReader, // Use concrete type for simplicity in this test
	processor *MockProcessor,
	writer *MockWriter,
	chunkSize int,
	maxRetries int,
	maxSkips int,
) error {
	// Simplified transaction simulation: we only track metrics, no actual DB interaction.

	// 1. Read Loop
	items := make([]TestItem, 0, chunkSize)

	for len(items) < chunkSize {
		readAttempts := 0

		// Simulate Read Retry Loop
		for {
			item, err := reader.Read(ctx)

			// Check for EOF (using the error defined in MockReader)
			if err != nil && errors.Is(err, port.ErrNoMoreItems) {
				goto ProcessPhase
			}

			if err != nil {
				be, isBatchError := err.(*exception.BatchError)

				if isBatchError && be.IsRetryable() && readAttempts < maxRetries { // maxRetries is the number of retries
					readAttempts++
					// Simulate rollback of read count if necessary (MockReader handles its internal counter)
					continue // Retry read
				}

				if isBatchError && be.IsSkippable() && se.SkipReadCount < maxSkips {
					se.SkipReadCount++
					se.AddFailureException(err) // Record the error
					reader.IncrementReadCount() // Advance the Reader's internal counter to skip the skipped item
					goto NextItemRead           // Simulate skipping the item and moving to the next read
				}

				// Fatal error or retry limit reached
				se.MarkAsFailed(err)
				return err
			}

			// Successful read
			se.ReadCount++
			items = append(items, item)
			break
		}
	NextItemRead:
	}

ProcessPhase:
	if len(items) == 0 {
		return nil // Nothing to process/write
	}

	// 2. Process Loop
	processedItems := make([]TestItem, 0, len(items))

	for _, item := range items {
		processAttempts := 0

		// Simulate Process Retry Loop
		for {
			processedItem, err := processor.Process(ctx, item)

			if err != nil {
				be, isBatchError := err.(*exception.BatchError)

				if isBatchError && be.IsRetryable() && processAttempts < maxRetries { // maxRetries is the number of retries
					processAttempts++
					continue // Retry process
				}

				if isBatchError && be.IsSkippable() && se.SkipProcessCount < maxSkips {
					se.SkipProcessCount++
					se.AddFailureException(err)
					se.FilterCount++ // Skipped items are filtered out
					goto NextItem    // Skip to next item
				}

				// Fatal error or retry limit reached
				se.MarkAsFailed(err)
				return err
			}

			// Successful process
			processedItems = append(processedItems, processedItem)
			break
		}
	NextItem:
	}

	if len(processedItems) == 0 {
		return nil
	}

	// 3. Write Phase (Simplified: only one write attempt, focusing on error type)
	writeAttempts := 0

	// Simulate Write Retry Loop (Chunk Retry)
	for {
		err := writer.Write(ctx, processedItems) // Removed nil argument

		if err != nil {
			be, isBatchError := err.(*exception.BatchError)

			if isBatchError && be.IsRetryable() && writeAttempts < maxRetries {
				writeAttempts++

				// Simulate transaction rollback: clear items written in this chunk
				writer.ResetWrittenItems()

				// Simulate transaction rollback and chunk retry
				se.RollbackCount++ // Increment rollback count
				continue           // Retry write (and implicitly, the whole chunk)
			}

			if isBatchError && be.IsSkippable() && se.SkipWriteCount < maxSkips {
				// Skippable write error usually triggers chunk splitting/rollback/re-execution
				// For this simplified test, we just increment skip count and fail the chunk execution
				se.SkipWriteCount++
				se.AddFailureException(err)
				// In a real scenario, this would trigger complex chunk splitting logic.
				// Here, we treat it as a failure that needs external handling (like chunk splitting).
				return errors.New("skippable write error occurred, chunk needs splitting")
			}

			// Fatal error or retry limit reached
			se.MarkAsFailed(err)
			return err
		}

		// Successful write
		se.WriteCount += len(processedItems)
		se.CommitCount++
		return nil
	}
}

func TestFaultTolerance_ReadSkip(t *testing.T) {
	ctx := context.Background()
	jobExec := model.NewJobExecution(uuid.New().String(), "testJob", model.NewJobParameters())
	se := model.NewStepExecution(uuid.New().String(), jobExec, "testStep")

	// Items: 1, 2, 3, 4. Skip item 3 (ID=3)
	items := []TestItem{{ID: 1}, {ID: 2}, {ID: 3}, {ID: 4}} // 4 items total. Item 3 will fail/skip.
	reader := NewMockReader(items, 0, 3, 0)                 // Skip at ID=3, No retryable failure expected

	// MockProcessor and MockWriter are used to ensure the flow continues correctly
	processor := NewMockProcessor(0, 0, 0)
	writer := &MockWriter{}

	// Max skips: 1, Max retries: 0
	err := SimulateChunkExecution(ctx, se, reader, processor, writer, 5, 0, 1) // Chunk size 5, but only 4 items available

	assert.NoError(t, err)
	assert.Equal(t, 3, se.ReadCount, "3 items should be successfully read (ID 1, 2, 4)")
	assert.Equal(t, 1, se.SkipReadCount, "1 item should be skipped during read")
	assert.Equal(t, 3, se.WriteCount, "3 items should be written")
	assert.Len(t, writer.writtenItems, 3)
}

func TestFaultTolerance_ReadRetry(t *testing.T) {
	ctx := context.Background()
	jobExec := model.NewJobExecution(uuid.New().String(), "testJob", model.NewJobParameters())
	se := model.NewStepExecution(uuid.New().String(), jobExec, "testStep")

	// Items: 1, 2, 3, 4. Fail item 3 (ID=3)
	items := []TestItem{{ID: 1}, {ID: 2}, {ID: 3}, {ID: 4}} // 4 items total. Item 3 will fail/retry.
	reader := NewMockReader(items, 3, 0, 1)                 // Fail at ID=3 once (failCount=1)
	processor := NewMockProcessor(0, 0, 0)
	writer := &MockWriter{}

	// Max retries: 1 (Should succeed on the second attempt for item 3)
	err := SimulateChunkExecution(ctx, se, reader, processor, writer, 4, 1, 0)

	assert.NoError(t, err)
	assert.Equal(t, 4, se.ReadCount, "4 items should be successfully read (after 1 retry)")
	assert.Equal(t, 0, se.SkipReadCount)
	assert.Equal(t, 4, se.WriteCount)
	assert.Len(t, writer.writtenItems, 4)
}

func TestFaultTolerance_ProcessSkip(t *testing.T) {
	ctx := context.Background()
	jobExec := model.NewJobExecution(uuid.New().String(), "testJob", model.NewJobParameters())
	se := model.NewStepExecution(uuid.New().String(), jobExec, "testStep")

	// Items: 1, 2, 3, 4, 5. Skip item 3 (ID=3) during process
	items := []TestItem{{ID: 1}, {ID: 2}, {ID: 3}, {ID: 4}, {ID: 5}}
	reader := NewMockReader(items, 0, 0, 0)
	processor := NewMockProcessor(0, 3, 0) // Skip at ID=3
	writer := &MockWriter{}

	// Max skips: 1, Max retries: 0
	err := SimulateChunkExecution(ctx, se, reader, processor, writer, 5, 0, 1)

	assert.NoError(t, err)
	assert.Equal(t, 5, se.ReadCount)
	assert.Equal(t, 1, se.SkipProcessCount, "1 item should be skipped during process")
	assert.Equal(t, 1, se.FilterCount, "Skipped item should be filtered")
	assert.Equal(t, 4, se.WriteCount, "4 items should be written")
	assert.Len(t, writer.writtenItems, 4)
}

func TestFaultTolerance_ProcessRetry(t *testing.T) {
	ctx := context.Background()
	jobExec := model.NewJobExecution(uuid.New().String(), "testJob", model.NewJobParameters())
	se := model.NewStepExecution(uuid.New().String(), jobExec, "testStep")

	// Items: 1, 2, 3, 4, 5. Fail item 3 (ID=3) during process (Retryable error)
	items := []TestItem{{ID: 1}, {ID: 2}, {ID: 3}, {ID: 4}, {ID: 5}} // 5 items total
	reader := NewMockReader(items, 0, 0, 0)
	processor := NewMockProcessor(3, 0, 1) // Fail at ID=3 once (failCount=1)
	writer := &MockWriter{}

	// Max retries: 1 (Should succeed on the second attempt for item 3)
	err := SimulateChunkExecution(ctx, se, reader, processor, writer, 5, 1, 1)

	assert.NoError(t, err)
	assert.Equal(t, 5, se.ReadCount)
	assert.Equal(t, 0, se.SkipProcessCount)
	assert.Equal(t, 0, se.FilterCount)
	assert.Equal(t, 5, se.WriteCount)
	assert.Len(t, writer.writtenItems, 5)
}

func TestFaultTolerance_ProcessRetry_Fatal(t *testing.T) {
	ctx := context.Background()
	jobExec := model.NewJobExecution(uuid.New().String(), "testJob", model.NewJobParameters())
	se := model.NewStepExecution(uuid.New().String(), jobExec, "testStep")

	// Items: 1, 2, 3, 4, 5. Fail item 3 (ID=3) during process (Retryable error)
	items := []TestItem{{ID: 1}, {ID: 2}, {ID: 3}, {ID: 4}, {ID: 5}} // 5 items total
	reader := NewMockReader(items, 0, 0, 0)                          // FailCount=0: Reader never fails
	processor := NewMockProcessor(3, 0, 1)                           // Fail at ID=3 once (failCount=1)
	writer := &MockWriter{}

	// Max retries: 0 (Should fail immediately)
	err := SimulateChunkExecution(ctx, se, reader, processor, writer, 5, 0, 1)

	assert.Error(t, err)
	assert.Equal(t, model.BatchStatusFailed, se.Status)
	assert.Equal(t, 5, se.ReadCount, "All 5 items should be read before process failure (Chunk Read is atomic)")
	assert.Equal(t, 0, se.WriteCount)
	assert.Contains(t, se.Failures[0], "transient process failure")
}

func TestFaultTolerance_WriteSkip_ChunkSplitting(t *testing.T) {
	ctx := context.Background()
	jobExec := model.NewJobExecution(uuid.New().String(), "testJob", model.NewJobParameters())
	se := model.NewStepExecution(uuid.New().String(), jobExec, "testStep")

	// Items: 1, 2, 3, 4, 5. Skip item 3 (ID=3) during write (Skippable error)
	items := []TestItem{{ID: 1}, {ID: 2}, {ID: 3}, {ID: 4}, {ID: 5}}
	reader := NewMockReader(items, 0, 0, 0)
	processor := NewMockProcessor(0, 0, 0)
	writer := &MockWriter{skipAt: 3}

	// Max skips: 1, Max retries: 0
	err := SimulateChunkExecution(ctx, se, reader, processor, writer, 5, 0, 1)

	// Write skip causes chunk failure requiring external splitting logic (simulated by returning an error)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "skippable write error occurred, chunk needs splitting")

	// Metrics update before returning the error
	assert.Equal(t, 5, se.ReadCount)
	assert.Equal(t, 1, se.SkipWriteCount)
	assert.Equal(t, 0, se.WriteCount, "Write count should be 0 due to simulated rollback/failure")
	// Rollback count is not explicitly handled in this simplified simulator, but SkipWriteCount is checked.
}

func TestFaultTolerance_WriteRetry(t *testing.T) {
	ctx := context.Background()
	jobExec := model.NewJobExecution(uuid.New().String(), "testJob", model.NewJobParameters())
	se := model.NewStepExecution(uuid.New().String(), jobExec, "testStep")

	// Items: 1, 2, 3, 4, 5. Fail item 3 (ID=3) during write (Retryable error)
	items := []TestItem{{ID: 1}, {ID: 2}, {ID: 3}, {ID: 4}, {ID: 5}} // 5 items total
	reader := NewMockReader(items, 0, 0, 0)
	processor := NewMockProcessor(0, 0, 0)

	// Writer fails once at item ID=3 (failCount=1)
	writer := &MockWriter{failAt: 3, failCount: 1}

	// Max retries: 1 (Should retry the whole chunk once internally and succeed on the second attempt)

	err := SimulateChunkExecution(ctx, se, reader, processor, writer, 5, 1, 0)

	assert.NoError(t, err)
	assert.Equal(t, 5, se.ReadCount)
	assert.Equal(t, 5, se.WriteCount)
	assert.Equal(t, 1, se.RollbackCount)  // Rollback count incremented once due to retry
	assert.Len(t, writer.writtenItems, 5) // Final written items should be 5
}
