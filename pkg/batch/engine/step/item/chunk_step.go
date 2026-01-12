package item

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"fmt"

	config "surfin/pkg/batch/core/config"
	port "surfin/pkg/batch/core/application/port"
	model "surfin/pkg/batch/core/domain/model"
	metrics "surfin/pkg/batch/core/metrics"
	repository "surfin/pkg/batch/core/domain/repository"
	"surfin/pkg/batch/engine/step/retry"
	"surfin/pkg/batch/engine/step/skip"
	exception "surfin/pkg/batch/support/util/exception"
	logger "surfin/pkg/batch/support/util/logger"
	tx "surfin/pkg/batch/core/tx"
)

// ChunkStep is an implementation of core.Step for chunk-oriented processing.
type ChunkStep struct {
	id                     string
	reader                 port.ItemReader[any]
	processor              port.ItemProcessor[any, any]
	writer                 port.ItemWriter[any]
	chunkSize              int
	commitInterval         int
	jobRepository          repository.JobRepository
	stepExecutionListeners []port.StepExecutionListener
	itemReadListeners      []port.ItemReadListener
	itemProcessListeners   []port.ItemProcessListener
	itemWriteListeners     []port.ItemWriteListener
	skipListeners          []port.SkipListener
	retryItemListeners     []port.RetryItemListener
	chunkListeners         []port.ChunkListener
	promotion              *model.ExecutionContextPromotion
	
	txManager              tx.TransactionManager // For managing chunk processing transactions.

	// T_TX_DEC: Addition of transaction attributes
	isolationLevel         sql.IsolationLevel
	propagation            string // Holds values like REQUIRED, REQUIRES_NEW as strings

	// Policies
	retryPolicy     retry.RetryPolicy // Global retry policy (for chunk operations)
	itemRetryPolicy retry.RetryPolicy // Item retry policy (for item read/process)
	skipPolicy      skip.SkipPolicy   // Item skip policy
	
	// Metrics and Tracing
	metricRecorder metrics.MetricRecorder
	tracer         metrics.Tracer
}

// Verify that ChunkStep implements the core.ItemWriter[any] interface.
var _ port.Step = (*ChunkStep)(nil)

// NewJSLAdaptedStep creates a new ChunkStep instance configured from JSL.
func NewJSLAdaptedStep(
	id string,
	reader port.ItemReader[any],
	processor port.ItemProcessor[any, any],
	writer port.ItemWriter[any],
	chunkSize int,
	commitInterval int,
	retryConfig *config.RetryConfig,
	itemRetryConfig config.ItemRetryConfig,
	itemSkipConfig config.ItemSkipConfig,
	jobRepository repository.JobRepository,
	stepExecutionListeners []port.StepExecutionListener,
	itemReadListeners []port.ItemReadListener,
	itemProcessListeners []port.ItemProcessListener,
	itemWriteListeners []port.ItemWriteListener,
	skipListeners []port.SkipListener,
	retryItemListeners []port.RetryItemListener,
	chunkListeners []port.ChunkListener,
	promotion *model.ExecutionContextPromotion,
	isolationLevel string, // Isolation level string passed from JSL
	propagation string, // Propagation attribute string passed from JSL
	txManager tx.TransactionManager,
	metricRecorder metrics.MetricRecorder,
	tracer metrics.Tracer,
) (*ChunkStep) {
	// Note: RetryPolicy and SkipPolicy creation should ideally be delegated to factories,
	// but for simplicity, we use default implementations here.
	
	// Item Retry Policy
	itemRetryPolicy := retry.NewDefaultRetryPolicyFactory().Create(
		itemRetryConfig.MaxAttempts,
		itemRetryConfig.InitialInterval,
		itemRetryConfig.RetryableExceptions,
	)
	
	// Item Skip Policy
	itemSkipPolicy, _ := skip.NewDefaultSkipPolicyFactory().Create(
		itemSkipConfig.SkipLimit,
		itemSkipConfig.SkippableExceptions,
	)

	// Global Retry Policy (for chunk operations, not item level)
	globalRetryPolicy := retry.NewDefaultRetryPolicyFactory().Create(
		retryConfig.MaxAttempts,
		retryConfig.InitialInterval,
		[]string{}, // Global retry exceptions are usually handled by the caller (JobRunner)
	)

	// Convert the isolation level specified in JSL to sql.IsolationLevel
	isoLevel := parseIsolationLevel(isolationLevel)

	return &ChunkStep{
		id:                     id,
		reader:                 reader,
		processor:              processor,
		writer:                 writer,
		chunkSize:              chunkSize,
		commitInterval:         commitInterval,
		jobRepository:          jobRepository,
		stepExecutionListeners: stepExecutionListeners,
		itemReadListeners:      itemReadListeners,
		itemProcessListeners:   itemProcessListeners,
		itemWriteListeners:     itemWriteListeners,
		skipListeners:          skipListeners,
		retryItemListeners:     retryItemListeners,
		chunkListeners:         chunkListeners,
		promotion:              promotion,
		retryPolicy:            globalRetryPolicy, // Global retry policy for chunk operations
		itemRetryPolicy:        itemRetryPolicy,   // Item retry policy
		skipPolicy:             itemSkipPolicy,    // Item skip policy
		isolationLevel:         isoLevel,
		propagation:            propagation,
		txManager:              txManager,
		metricRecorder:         metricRecorder,
		tracer:                 tracer,
		// Note: Item retry logic is handled internally within the chunk loop using itemRetryPolicy
	}
}

// GetExecutionContextPromotion implements port.Step.
func (s *ChunkStep) GetExecutionContextPromotion() *model.ExecutionContextPromotion {
	return s.promotion
}

// SetMetricRecorder implements port.Step.
func (s *ChunkStep) SetMetricRecorder(recorder metrics.MetricRecorder) {
	s.metricRecorder = recorder
}

// SetTracer implements port.Step.
func (s *ChunkStep) SetTracer(tracer metrics.Tracer) {
	s.tracer = tracer
}

// parseIsolationLevel converts a JSL string to sql.IsolationLevel.
func parseIsolationLevel(level string) sql.IsolationLevel {
	switch level {
	case "READ_UNCOMMITTED":
		return sql.LevelReadUncommitted
	case "READ_COMMITTED":
		return sql.LevelReadCommitted
	case "WRITE_COMMITTED":
		return sql.LevelWriteCommitted
	case "REPEATABLE_READ":
		return sql.LevelRepeatableRead
	case "SERIALIZABLE":
		return sql.LevelSerializable
	default:
		// Default depends on framework configuration or database default
		return sql.LevelDefault
	}
}

// ID returns the step ID.
func (s *ChunkStep) ID() string {
	return s.id
}

// StepName returns the step name.
func (s *ChunkStep) StepName() string {
	return s.id
}

// GetTransactionOptions returns the transaction options for this step.
func (s *ChunkStep) GetTransactionOptions() *sql.TxOptions {
	// Propagation attribute is handled by the StepExecutor, so only IsolationLevel is set in TxOptions here.
	return &sql.TxOptions{
		Isolation: s.isolationLevel,
		ReadOnly:  false, // ChunkStep usually performs writes
	}
}

// GetPropagation returns the transaction propagation attribute.
func (s *ChunkStep) GetPropagation() string {
	return s.propagation
}

// --- Listener Notifiers ---

func (s *ChunkStep) notifyRetryRead(ctx context.Context, err error) {
	s.tracer.RecordError(ctx, s.id, err)
	s.metricRecorder.RecordItemRetry(ctx, s.id, "read")
	for _, l := range s.retryItemListeners {
		l.OnRetryRead(ctx, err)
	}
}

func (s *ChunkStep) notifySkipRead(ctx context.Context, err error) {
	s.tracer.RecordError(ctx, s.id, err)
	s.metricRecorder.RecordItemSkip(ctx, s.id, "read")
	for _, l := range s.skipListeners {
		l.OnSkipRead(ctx, err)
	}
}

func (s *ChunkStep) notifyRetryProcess(ctx context.Context, item any, err error) {
	s.tracer.RecordError(ctx, s.id, err)
	s.metricRecorder.RecordItemRetry(ctx, s.id, "process")
	for _, l := range s.retryItemListeners {
		l.OnRetryProcess(ctx, item, err)
	}
}

func (s *ChunkStep) notifySkipProcess(ctx context.Context, item any, err error) {
	s.tracer.RecordError(ctx, s.id, err)
	s.metricRecorder.RecordItemSkip(ctx, s.id, "process")
	for _, l := range s.skipListeners {
		l.OnSkipProcess(ctx, item, err)
	}
}

func (s *ChunkStep) notifyRetryWrite(ctx context.Context, items []any, err error) {
	s.tracer.RecordError(ctx, s.id, err)
	s.metricRecorder.RecordItemRetry(ctx, s.id, "write")
	itemsInterface := make([]interface{}, len(items))
	for i, item := range items {
		itemsInterface[i] = item
	}
	for _, l := range s.retryItemListeners {
		l.OnRetryWrite(ctx, itemsInterface, err)
	}
}

func (s *ChunkStep) notifySkipWrite(ctx context.Context, item any, err error) {
	s.tracer.RecordError(ctx, s.id, err)
	s.metricRecorder.RecordItemSkip(ctx, s.id, "write")
	
	itemInterface := item
	
	for _, l := range s.itemWriteListeners {
		l.OnSkipInWrite(ctx, itemInterface, err)
	}
	for _, l := range s.skipListeners {
		l.OnSkipWrite(ctx, itemInterface, err)
	}
}

// Execute runs the chunk-oriented step logic.
func (s *ChunkStep) Execute(ctx context.Context, jobExecution *model.JobExecution, stepExecution *model.StepExecution) error {
	
	logger.Infof("ChunkStep '%s' executing.", s.id)
	
	// 1. Update StepExecution status to STARTED
	stepExecution.MarkAsStarted()
	if err := s.jobRepository.UpdateStepExecution(ctx, stepExecution); err != nil {
		return exception.NewBatchError(s.id, "Failed to update StepExecution status to STARTED", err, false, false)
	}

	// 2. Load checkpoint data and open components
	var checkpointEC model.ExecutionContext
	checkpointData, err := s.jobRepository.FindCheckpointData(ctx, stepExecution.ID)
	if err != nil && !errors.Is(err, repository.ErrCheckpointDataNotFound) {
		return exception.NewBatchError(s.id, "Failed to load checkpoint data", err, false, false)
	}
	if checkpointData != nil {
		checkpointEC = checkpointData.ExecutionContext
		logger.Infof("Checkpoint data loaded for step '%s'. Restoring state.", s.id)
		
		// Restore statistics (T1/T2 Step 1.1.3)
		if rc, ok := checkpointEC.GetInt("readCount"); ok {
			stepExecution.ReadCount = rc
		}
		if wc, ok := checkpointEC.GetInt("writeCount"); ok {
			stepExecution.WriteCount = wc
		}
	} else {
		checkpointEC = model.NewExecutionContext()
	}

	if err := s.reader.Open(ctx, checkpointEC); err != nil {
		return exception.NewBatchError(s.id, "Failed to open ItemReader", err, false, false)
	}
	if s.writer != nil {
		if err := s.writer.Open(ctx, checkpointEC); err != nil {
			s.reader.Close(ctx)
			return exception.NewBatchError(s.id, "Failed to open ItemWriter", err, false, false)
		}
	}

	// 3. Chunk processing loop
	var chunkError error
	
	// Chunk processing statistics (start from current StepExecution values)
	readCount := stepExecution.ReadCount
	writeCount := stepExecution.WriteCount
	commitCount := stepExecution.CommitCount
	
	// Declare processedItem and processErr for reassignment within the loop
	var processedItem any
	var processErr error
	var processAttempts int // Declared outside the loop
	
	// Main chunk processing loop
	for {
		// 3.1. Begin transaction
		txAdapter, err := s.txManager.Begin(ctx, s.GetTransactionOptions())
		if err != nil {
			chunkError = exception.NewBatchError(s.id, "Failed to begin transaction for chunk", err, false, false)
			break
		}
		txCtx := context.WithValue(ctx, "tx", txAdapter) // Store transaction in context
		
		// Listener notification (BeforeChunk)
		for _, l := range s.chunkListeners {
			l.BeforeChunk(txCtx, stepExecution)
		}

		var itemsToWrite []any
		currentChunkReadCount := 0
		isEOF := false
		
		// 3.2. Chunk read/process loop
		for currentChunkReadCount < s.chunkSize {
			var item any
			var readErr error
			readAttempts := 0

			// Retry loop (Read)
			for {
				item, readErr = s.reader.Read(txCtx)

				if readErr != nil {
					if errors.Is(readErr, port.ErrNoMoreItems) || errors.Is(readErr, io.EOF) {
						isEOF = true
						goto EndReadLoop // Exit the entire read loop
					}

					// Check if retryable
					if s.itemRetryPolicy.ShouldRetry(readErr) && readAttempts < s.itemRetryPolicy.GetMaxAttempts() {
						readAttempts++
						logger.Warnf("ChunkStep '%s': Item read failed (Attempt %d/%d). Retrying: %v", s.id, readAttempts, s.itemRetryPolicy.GetMaxAttempts(), readErr)
						s.notifyRetryRead(txCtx, readErr)
						// TODO: Backoff wait
						continue // Retry
					}

					// Check if skippable
					if s.skipPolicy.ShouldSkip(readErr) && stepExecution.SkipReadCount < s.skipPolicy.GetSkipLimit() {
						s.skipPolicy.IncrementSkipCount()
						stepExecution.SkipReadCount++
						stepExecution.AddFailureException(readErr)
						logger.Warnf("ChunkStep '%s': Item read skipped (Skip Count: %d/%d): %v", s.id, s.skipPolicy.GetSkipCount(), s.skipPolicy.GetSkipLimit(), readErr)
						s.notifySkipRead(txCtx, readErr)
						goto NextItemRead // Go to next item
					}

					// Fatal error or retry/skip limit exceeded
					chunkError = exception.NewBatchError(s.id, "Item read failed (Fatal or limit reached)", readErr, false, false)
					goto EndChunkLoop // Exit the entire chunk processing
				}

				// Read successful
				s.metricRecorder.RecordItemRead(txCtx, s.id) // Record metric
				break
			}

			// Processing after successful read
			readCount++
			currentChunkReadCount++
			
			// Process
			processAttempts = 0 // Reset for each item
			
			// Process retry loop
			for {
				processedItem, processErr = s.processor.Process(txCtx, item)
				
				if processErr != nil {
					be, isBatchError := processErr.(*exception.BatchError)

					// Check if retryable
					if isBatchError && be.IsRetryable() && processAttempts < s.itemRetryPolicy.GetMaxAttempts() {
						processAttempts++
						logger.Warnf("ChunkStep '%s': Item process failed (Attempt %d/%d). Retrying: %v", s.id, processAttempts, s.itemRetryPolicy.GetMaxAttempts(), processErr)
						s.notifyRetryProcess(txCtx, item, processErr)
						// TODO: Backoff wait
						continue // Retry
					}

					// Check if skippable
					if s.skipPolicy.ShouldSkip(processErr) && stepExecution.SkipProcessCount < s.skipPolicy.GetSkipLimit() {
						s.skipPolicy.IncrementSkipCount()
						stepExecution.SkipProcessCount++
						stepExecution.FilterCount++ // Skipped items are considered filtered
						stepExecution.AddFailureException(processErr)
						logger.Warnf("ChunkStep '%s': Item process skipped (Skip Count: %d/%d): %v", s.id, s.skipPolicy.GetSkipCount(), s.skipPolicy.GetSkipLimit(), processErr)
						s.notifySkipProcess(txCtx, item, processErr)
						goto NextItemRead // Go to next item
					}

					// Fatal error or retry/skip limit exceeded
					chunkError = exception.NewBatchError(s.id, "Item process failed (Fatal or limit reached)", processErr, false, false)
					goto EndReadLoop // Exit the entire chunk processing
				}
				
				// Process successful
				s.metricRecorder.RecordItemProcess(txCtx, s.id) // Record metric
				break
			}
			
			if processedItem != nil {
				itemsToWrite = append(itemsToWrite, processedItem)
			} else {
				// Count of filtered items
				stepExecution.FilterCount++
			}

		NextItemRead: // Jump here if skipped, to the next loop iteration
		}
		
	EndReadLoop:
		
		if chunkError != nil {
			// If an error occurred during read or process, rollback the transaction
			s.txManager.Rollback(txAdapter)
			
			// Listener notification (AfterChunk - failed)
			for _, l := range s.chunkListeners {
				l.AfterChunk(txCtx, stepExecution)
			}
			break
		}
		
		// If read ended with EOF and there are no items to write, rollback transaction and exit loop
		if currentChunkReadCount == 0 && len(itemsToWrite) == 0 {
			s.txManager.Rollback(txAdapter) // Rollback as transaction was started but nothing was done
			
			// Listener notification (AfterChunk - successful/empty)
			for _, l := range s.chunkListeners {
				l.AfterChunk(txCtx, stepExecution)
			}
			break
		}
		
		// 3.3. Write
		if len(itemsToWrite) > 0 && s.writer != nil {
			writeAttempts := 0
			var writeErr error

			// Write retry loop (Chunk Retry / Chunk Splitting)
			for {
				writeErr = s.writer.Write(txCtx, txAdapter, itemsToWrite)

				if writeErr != nil {
					be, isBatchError := writeErr.(*exception.BatchError)

					// 1. Transient error (retry the entire chunk)
					if isBatchError && be.IsRetryable() && writeAttempts < s.itemRetryPolicy.GetMaxAttempts() {
						writeAttempts++
						logger.Warnf("ChunkStep '%s': Item write failed (Attempt %d/%d). Retrying chunk: %v", s.id, writeAttempts, s.itemRetryPolicy.GetMaxAttempts(), writeErr)
						s.notifyRetryWrite(txCtx, itemsToWrite, writeErr)
						
						// Rollback transaction and continue outer chunk loop (retry)
						s.txManager.Rollback(txAdapter)
						stepExecution.RollbackCount++
						// TODO: Backoff wait
						goto RetryChunk // Go to outer chunk loop
					}

					// 2. Skippable error (chunk splitting)
					if s.skipPolicy.ShouldSkip(writeErr) && stepExecution.SkipWriteCount < s.skipPolicy.GetSkipLimit() {
						s.skipPolicy.IncrementSkipCount()
						stepExecution.AddFailureException(writeErr)
						logger.Warnf("ChunkStep '%s': Item write failed (Skip Count: %d/%d). Triggering chunk splitting.", s.id, s.skipPolicy.GetSkipCount(), s.skipPolicy.GetSkipLimit())
						
						// 2.1. Rollback transaction
						s.txManager.Rollback(txAdapter)
						stepExecution.RollbackCount++
						
						// 2.2. Execute chunk splitting process
						// If chunk splitting succeeds, error items are skipped, and remaining items are committed.
						_, fatalErr := s.HandleSkippableWriteFailure(txCtx, itemsToWrite, stepExecution) // Ignore remainingItems
						
						if fatalErr != nil {
							chunkError = fatalErr
							goto EndChunkLoop
						}
						
						// Chunk splitting succeeded, and all items were processed, so exit this chunk loop,
						// and return to the beginning of the outer chunk loop (to read the next chunk).
						goto EndChunkTransaction
					}

					// 3. Fatal error or retry/skip limit exceeded
					// If a write error occurs, rollback the transaction
					s.txManager.Rollback(txAdapter)
					chunkError = exception.NewBatchError(s.id, "Item write failed (Fatal or limit reached)", writeErr, false, false)
					goto EndChunkLoop // Exit the entire chunk processing
				}
				
				// Write successful
				s.metricRecorder.RecordItemWrite(txCtx, s.id, len(itemsToWrite)) // Record metric
				break
			}
			writeCount += len(itemsToWrite)
		}
		
		// 3.4. Commit transaction
		if commitErr := s.txManager.Commit(txAdapter); commitErr != nil {
			chunkError = exception.NewBatchError(s.id, "Failed to commit transaction for chunk", commitErr, false, false)
			
			// Listener notification (AfterChunk - failed)
			for _, l := range s.chunkListeners {
				l.AfterChunk(txCtx, stepExecution)
			}
			break
		}
		commitCount++
		
		// Listener notification (AfterChunk - successful)
		for _, l := range s.chunkListeners {
			l.AfterChunk(txCtx, stepExecution)
		}
		
		// 3.5. Save checkpoint (T1/T2 Step 1.1.2)
		// After successful commit, save Reader/Writer state and statistics
		if err := s.saveCheckpoint(ctx, stepExecution, readCount, writeCount); err != nil {
			logger.Errorf("ChunkStep '%s': Failed to save checkpoint after commit: %v", s.id, err)
			// Checkpoint save failure is not fatal, but log it
		}
		
		// 3.6. Determine end of chunk processing
		if isEOF {
			// If read ended with EOF
			logger.Debugf("ChunkStep '%s': Reached EOF. Exiting chunk loop.", s.id)
			break
		}
	RetryChunk: // Jump here on write retry
	EndChunkTransaction: // Jump target on successful chunk splitting or EOF
		; // Empty statement to allow label definition
	} // End of chunk loop
EndChunkLoop:

	// 4. Finalization
	
	// Update statistics
	stepExecution.ReadCount = readCount
	stepExecution.WriteCount = writeCount
	stepExecution.CommitCount = commitCount
	
	// Close Reader/Writer
	if closeErr := s.reader.Close(ctx); closeErr != nil {
		logger.Warnf("Failed to close ItemReader: %v", closeErr)
		if chunkError == nil {
			chunkError = closeErr
		}
	}
	if s.writer != nil {
		if closeErr := s.writer.Close(ctx); closeErr != nil {
			logger.Warnf("Failed to close ItemWriter: %v", closeErr)
			if chunkError == nil {
				chunkError = closeErr
			}
		}
	}

	// ADDED: Update StepExecution.ExecutionContext to the latest state
	// This ensures that data written by the reader/writer to ExecutionContext (e.g., decision.condition)
	// is reflected in the StepExecution object, enabling promotion to JobExecution.
	finalStepEC := model.NewExecutionContext()
	if s.reader != nil {
		if readerEC, err := s.reader.GetExecutionContext(ctx); err == nil {
			for k, v := range readerEC {
				finalStepEC.Put(k, v)
			}
		} else if !errors.Is(err, port.ErrExecutionContextNotSupported) {
			logger.Warnf("ChunkStep '%s': Failed to get ExecutionContext from ItemReader for final update: %v", s.id, err)
		}
	}

	if s.writer != nil {
		if writerEC, err := s.writer.GetExecutionContext(ctx); err == nil {
			for k, v := range writerEC {
				finalStepEC.Put(k, v)
			}
		} else if !errors.Is(err, port.ErrExecutionContextNotSupported) {
			logger.Warnf("ChunkStep '%s': Failed to get ExecutionContext from ItemWriter for final update: %v", s.id, err)
		}
	}

	// Include statistics in the final ExecutionContext
	finalStepEC.Put("readCount", readCount)
	finalStepEC.Put("writeCount", writeCount)
	
	// Update StepExecution's ExecutionContext field
	stepExecution.ExecutionContext = finalStepEC

	// 5. Update StepExecution status
	if chunkError != nil {
		s.tracer.RecordError(ctx, s.id, chunkError)
		stepExecution.MarkAsFailed(chunkError)
	} else {
		stepExecution.MarkAsCompleted()
	}
	
	// 6. Persist
	if updateErr := s.jobRepository.UpdateStepExecution(ctx, stepExecution); updateErr != nil {
		logger.Errorf("ChunkStep '%s': Failed to update final StepExecution state: %v", s.id, updateErr)
		if chunkError == nil {
			chunkError = updateErr
		}
	}
	
	logger.Infof("ChunkStep '%s' finished. ExitStatus: %s", s.id, stepExecution.ExitStatus)
	return chunkError
}

// saveCheckpoint retrieves the state of the Reader/Writer and saves it to the JobRepository.
func (s *ChunkStep) saveCheckpoint(ctx context.Context, stepExecution *model.StepExecution, readCount, writeCount int) error {
	currentEC := model.NewExecutionContext()
	
	// Get Reader state
	if s.reader != nil {
		if ec, err := s.reader.GetExecutionContext(ctx); err == nil {
			for k, v := range ec {
				currentEC.Put(k, v)
			}
		} else if !errors.Is(err, port.ErrExecutionContextNotSupported) {
			return fmt.Errorf("failed to get ExecutionContext from ItemReader: %w", err)
		}
	}

	// Get Writer state and merge
	if s.writer != nil {
		if ec, err := s.writer.GetExecutionContext(ctx); err == nil {
			for k, v := range ec {
				currentEC.Put(k, v)
			}
		} else if !errors.Is(err, port.ErrExecutionContextNotSupported) {
			return fmt.Errorf("failed to get ExecutionContext from ItemWriter: %w", err)
		}
	}

	if len(currentEC) > 0 {
		// Include statistics in the checkpoint (to restore statistics on restart)
		currentEC.Put("readCount", readCount)
		currentEC.Put("writeCount", writeCount)
		
		checkpointToSave := &model.CheckpointData{
			StepExecutionID:  stepExecution.ID,
			ExecutionContext: currentEC,
		}
		
		if err := s.jobRepository.SaveCheckpointData(ctx, checkpointToSave); err != nil {
			return fmt.Errorf("failed to save checkpoint data: %w", err)
		}
		logger.Debugf("Checkpoint data saved successfully for step '%s'. Read: %d, Write: %d", s.id, readCount, writeCount)
	}
	return nil
}

// HandleSkippableWriteFailure re-writes items one by one when a skippable write error occurs,
// skipping the item that caused the error and returning the remaining items.
// Returns: (remaining items, fatal error)
func (s *ChunkStep) HandleSkippableWriteFailure(ctx context.Context, originalItems []any, stepExecution *model.StepExecution) ([]any, error) {
	taskletName := s.id
	
	// Remaining items after chunk splitting (expected to be empty if this function succeeds)
	var remainingItems []any
	
	// Hold fatal error if it occurs during chunk splitting
	var fatalError error
	
	// 1. Re-write items one by one to identify errors
	for i, item := range originalItems {
		// 1.1. Begin transaction for a single item
		txAdapter, err := s.txManager.Begin(ctx, s.GetTransactionOptions())
		if err != nil {
			fatalError = exception.NewBatchError(taskletName, "Failed to begin transaction for chunk splitting", err, false, false)
			break
		}
		txCtx := context.WithValue(ctx, "tx", txAdapter)
		
		// 1.2. Write single item
		writeErr := s.writer.Write(txCtx, txAdapter, []any{item})
		
		if writeErr != nil {
			// 1.3. Write failed: Skip processing
			
			// Re-check if skip limit is exceeded
			if s.skipPolicy.ShouldSkip(writeErr) && stepExecution.SkipWriteCount < s.skipPolicy.GetSkipLimit() {
				s.skipPolicy.IncrementSkipCount()
				stepExecution.SkipWriteCount++
				stepExecution.AddFailureException(writeErr)
				s.notifySkipWrite(txCtx, item, writeErr)
				
				// Rollback
				s.txManager.Rollback(txAdapter)
				logger.Warnf("ChunkStep '%s': Item skipped during chunk splitting: %+v", taskletName, item)
				
				// This item was skipped, so do not add to remainingItems
			} else {
				// Skip limit exceeded or fatal error
				s.txManager.Rollback(txAdapter)
				fatalError = exception.NewBatchError(taskletName, fmt.Sprintf("Item write failed during chunk splitting (Fatal or limit reached) for item index %d", i), writeErr, false, false)
				break
			}
		} else {
			// 1.4. Write successful: Commit
			if commitErr := s.txManager.Commit(txAdapter); commitErr != nil {
				fatalError = exception.NewBatchError(taskletName, "Failed to commit transaction during chunk splitting", commitErr, false, false)
				break
			}
			
			// Do not add successful items to remainingItems (as they are already persisted)
			writeCount := stepExecution.WriteCount + 1
			stepExecution.WriteCount = writeCount // Update statistics
			s.metricRecorder.RecordItemWrite(txCtx, s.id, 1) // Record metric
		}
	}
	
	// At the completion of chunk splitting, all items from the original chunk are considered processed.
	// Successful items were committed, and failed items were skipped.
	// Therefore, this function returns an empty remainingItems and fatalError.
	
	return remainingItems, fatalError
}
