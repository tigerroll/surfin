package item

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"

	coreAdapter "github.com/tigerroll/surfin/pkg/batch/core/adapter" // Imports the core adapter package.
	port "github.com/tigerroll/surfin/pkg/batch/core/application/port"
	config "github.com/tigerroll/surfin/pkg/batch/core/config"
	model "github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	repository "github.com/tigerroll/surfin/pkg/batch/core/domain/repository"
	metrics "github.com/tigerroll/surfin/pkg/batch/core/metrics"
	tx "github.com/tigerroll/surfin/pkg/batch/core/tx"
	"github.com/tigerroll/surfin/pkg/batch/engine/step/retry"
	"github.com/tigerroll/surfin/pkg/batch/engine/step/skip"
	exception "github.com/tigerroll/surfin/pkg/batch/support/util/exception"
	logger "github.com/tigerroll/surfin/pkg/batch/support/util/logger"

	"github.com/tigerroll/surfin/pkg/batch/adapter/database" // Required for type assertion.
)

// ChunkStep implements the [port.Step] interface for chunk-oriented batch processing.
// It orchestrates item reading, processing, and writing, while managing transactions,
// checkpointing, and fault tolerance (retries and skips).
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

	isolationLevel sql.IsolationLevel // The transaction isolation level for this step.
	propagation    string             // The transaction propagation attribute (e.g., "REQUIRED", "REQUIRES_NEW").

	// Policies
	retryPolicy     retry.RetryPolicy // Global retry policy (for chunk operations)
	itemRetryPolicy retry.RetryPolicy // Item retry policy (for item read/process)
	skipPolicy      skip.SkipPolicy   // Item skip policy

	// Metrics and Tracing
	metricRecorder metrics.MetricRecorder
	tracer         metrics.Tracer

	// dbResolver is used to resolve database connections dynamically.
	dbResolver       coreAdapter.ResourceConnectionResolver
	txManagerFactory tx.TransactionManagerFactory // Transaction manager factory for creating transaction managers.

	// currentStepExecution holds the current step execution for checkpointing in Close.
	currentStepExecution *model.StepExecution
}

// Verify that [ChunkStep] implements the [port.Step] interface.
var _ port.Step = (*ChunkStep)(nil)

// NewJSLAdaptedStep creates a new [ChunkStep] configured from JSL definitions.
//
// Parameters:
//
//	id: Unique identifier for the step.
//	reader: [port.ItemReader] for data input.
//	processor: [port.ItemProcessor] for data transformation.
//	writer: [port.ItemWriter] for data output.
//	chunkSize: Maximum number of items per chunk.
//	commitInterval: Interval for transaction commits.
//	retryConfig: Configuration for chunk-level retries.
//	itemRetryConfig: Configuration for item-level retries.
//	itemSkipConfig: Configuration for item-level skips.
//	jobRepository: Repository for persisting job metadata.
//	stepExecutionListeners: Listeners for step lifecycle events.
//	itemReadListeners: Listeners for read events.
//	itemProcessListeners: Listeners for process events.
//	itemWriteListeners: Listeners for write events.
//	skipListeners: Listeners for skip events.
//	retryItemListeners: Listeners for retry events.
//	chunkListeners: Listeners for chunk lifecycle events.
//	promotion: Settings for promoting execution context to job level.
//	isolationLevel: Transaction isolation level string (e.g., "SERIALIZABLE").
//	propagation: Transaction propagation attribute (e.g., "REQUIRED").
//	txManagerFactory: Factory for creating transaction managers.
//	metricRecorder: Recorder for step metrics.
//	tracer: Tracer for distributed tracing.
//	dbResolver: Resolver for dynamic database connections.
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
	isolationLevel string,
	propagation string,
	txManagerFactory tx.TransactionManagerFactory,
	metricRecorder metrics.MetricRecorder,
	tracer metrics.Tracer,
	dbResolver coreAdapter.ResourceConnectionResolver,
) *ChunkStep {
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
		txManagerFactory:       txManagerFactory,
		metricRecorder:         metricRecorder,
		tracer:                 tracer,
		dbResolver:             dbResolver,
	}
}

// GetExecutionContextPromotion returns the ExecutionContext promotion settings for this step.
// This method implements the [port.Step] interface.
//
// Returns:
//
//	*model.ExecutionContextPromotion: A pointer to the [model.ExecutionContextPromotion] configuration.
func (s *ChunkStep) GetExecutionContextPromotion() *model.ExecutionContextPromotion {
	return s.promotion
}

// SetMetricRecorder sets the MetricRecorder for this step.
// This method implements the [port.Step] interface.
//
// Parameters:
//
//	recorder: The [metrics.MetricRecorder] instance to be used.
func (s *ChunkStep) SetMetricRecorder(recorder metrics.MetricRecorder) {
	s.metricRecorder = recorder
}

// SetTracer sets the Tracer for this step.
// This method implements the [port.Step] interface.
//
// Parameters:
//
//	tracer: The [metrics.Tracer] instance to be used.
func (s *ChunkStep) SetTracer(tracer metrics.Tracer) {
	s.tracer = tracer
}

// parseIsolationLevel converts a JSL string to sql.IsolationLevel.
//
// Parameters:
//
//	level: The isolation level as a string (e.g., "READ_UNCOMMITTED", "SERIALIZABLE").
//
// Returns:
//
//	sql.IsolationLevel: The corresponding [sql.IsolationLevel]. Defaults to [sql.LevelDefault] if unrecognized.
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

// ID returns the unique ID of the step definition.
// This method implements the [port.Step] interface.
//
// Returns:
//
//	string: The unique identifier string for the step.
func (s *ChunkStep) ID() string {
	return s.id
}

// StepName returns the logical name of the step.
// This method implements the [port.Step] interface.
//
// Returns:
//
//	string: The logical name string for the step.
func (s *ChunkStep) StepName() string {
	return s.id
}

// GetTransactionOptions returns the transaction options for this step.
// This method implements the [port.Step] interface.
//
// Returns:
//
//	*sql.TxOptions: A pointer to [sql.TxOptions] configured with the step's isolation level.
func (s *ChunkStep) GetTransactionOptions() *sql.TxOptions {
	// Propagation attribute is handled by the StepExecutor, so only IsolationLevel is set in TxOptions here.
	return &sql.TxOptions{
		Isolation: s.isolationLevel,
		ReadOnly:  false, // ChunkStep usually performs writes
	}
}

// GetPropagation returns the transaction propagation attribute.
// This method implements the [port.Step] interface.
//
// Returns:
//
//	string: The propagation attribute as a string (e.g., "REQUIRED", "REQUIRES_NEW").
func (s *ChunkStep) GetPropagation() string {
	return s.propagation
}

// notifyRetryRead notifies listeners that an item read operation is being retried.
func (s *ChunkStep) notifyRetryRead(ctx context.Context, stepExecution *model.StepExecution, err error) {
	s.tracer.RecordError(ctx, s.id, err)
	s.metricRecorder.RecordItemRetry(ctx, stepExecution, err)
	for _, l := range s.retryItemListeners {
		l.OnRetryRead(ctx, err)
	}
}

// notifySkipRead notifies listeners that an item read operation is being skipped.
func (s *ChunkStep) notifySkipRead(ctx context.Context, stepExecution *model.StepExecution, err error) {
	s.tracer.RecordError(ctx, s.id, err)
	s.metricRecorder.RecordItemSkip(ctx, stepExecution, err)
	for _, l := range s.skipListeners {
		l.OnSkipRead(ctx, err)
	}
}

// notifyRetryProcess notifies listeners that an item process operation is being retried.
func (s *ChunkStep) notifyRetryProcess(ctx context.Context, stepExecution *model.StepExecution, item any, err error) {
	s.tracer.RecordError(ctx, s.id, err)
	s.metricRecorder.RecordItemRetry(ctx, stepExecution, err)
	for _, l := range s.retryItemListeners {
		l.OnRetryProcess(ctx, item, err)
	}
}

// notifySkipProcess notifies listeners that an item process operation is being skipped.
func (s *ChunkStep) notifySkipProcess(ctx context.Context, stepExecution *model.StepExecution, item any, err error) {
	s.tracer.RecordError(ctx, s.id, err)
	s.metricRecorder.RecordItemSkip(ctx, stepExecution, err)
	for _, l := range s.skipListeners {
		l.OnSkipProcess(ctx, item, err)
	}
}

// notifyRetryWrite notifies listeners that an item write operation is being retried.
func (s *ChunkStep) notifyRetryWrite(ctx context.Context, stepExecution *model.StepExecution, items []any, err error) {
	s.tracer.RecordError(ctx, s.id, err)
	s.metricRecorder.RecordItemRetry(ctx, stepExecution, err)
	itemsInterface := make([]interface{}, len(items))
	for i, item := range items {
		itemsInterface[i] = item
	}
	for _, l := range s.retryItemListeners {
		l.OnRetryWrite(ctx, itemsInterface, err)
	}
}

// notifySkipWrite notifies listeners that an item write operation is being skipped.
func (s *ChunkStep) notifySkipWrite(ctx context.Context, stepExecution *model.StepExecution, item any, err error) {
	s.tracer.RecordError(ctx, s.id, err)
	s.metricRecorder.RecordItemSkip(ctx, stepExecution, err)

	itemInterface := item

	for _, l := range s.itemWriteListeners {
		l.OnSkipInWrite(ctx, itemInterface, err)
	}
	for _, l := range s.skipListeners {
		l.OnSkipWrite(ctx, itemInterface, err)
	}
}

// Execute runs the chunk-oriented step. It manages the read-process-write loop,
// transaction boundaries, and fault tolerance policies. It updates the
// [model.StepExecution] status and persists checkpoint data upon completion.
//
// Parameters:
//
//	ctx: Context for the operation.
//	jobExecution: Current [model.JobExecution].
//	stepExecution: Current [model.StepExecution].
//
// Returns:
//
//	error: Returns an error if the step fails or exceeds retry/skip limits.
func (s *ChunkStep) Execute(ctx context.Context, jobExecution *model.JobExecution, stepExecution *model.StepExecution) error {
	s.currentStepExecution = stepExecution // Set current step execution for checkpointing in Close.

	// Execute StepExecutionListeners (AfterStep)
	defer func() {
		for _, l := range s.stepExecutionListeners {
			l.AfterStep(ctx, stepExecution)
		}
	}()

	// Execute StepExecutionListeners (BeforeStep)
	for _, l := range s.stepExecutionListeners {
		l.BeforeStep(ctx, stepExecution)
	}

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

		// Restore statistics
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
RetryChunk: // Jump here on write retry
	for {
		var currentTxManager tx.TransactionManager
		var targetResourceName string // Target resource name for ItemWriter
		var resourcePath string       // Path/identifier within the target resource for ItemWriter

		if s.writer != nil {
			targetResourceName = s.writer.GetTargetResourceName()
			resourcePath = s.writer.GetResourcePath()
		}

		// For ChunkStep, the target resource is expected to be a database for transaction management.
		if targetResourceName == "" && s.writer != nil {
			chunkError = exception.NewBatchError(s.id, "ItemWriter must provide a target resource name (expected to be a DB name for ChunkStep) for transaction management.", nil, false, false)
			break
		}

		dbConnAsResource, err := s.dbResolver.ResolveConnection(ctx, targetResourceName)
		if err != nil {
			chunkError = exception.NewBatchError(s.id, fmt.Sprintf("Failed to resolve DB connection '%s' for chunk transaction", targetResourceName), err, false, false)
			break
		}
		dbConn, ok := dbConnAsResource.(database.DBConnection)
		if !ok {
			chunkError = exception.NewBatchError(s.id, fmt.Sprintf("Internal error: Resolved connection '%s' is not a database.DBConnection", targetResourceName), nil, false, false)
			break
		}
		currentTxManager = s.txManagerFactory.NewTransactionManager(dbConn)

		txAdapter, err := currentTxManager.Begin(ctx, s.GetTransactionOptions())
		if err != nil {
			chunkError = exception.NewBatchError(s.id, "Failed to begin transaction for chunk", err, false, false)
			break
		}
		txCtx := tx.ContextWithTx(ctx, txAdapter)

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
				for _, l := range s.itemReadListeners {
					l.BeforeRead(txCtx)
				}
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
						s.notifyRetryRead(txCtx, stepExecution, readErr)
						// TODO: Backoff wait
						continue // Retry
					}

					// Check if skippable
					if s.skipPolicy.ShouldSkip(readErr) && stepExecution.SkipReadCount < s.skipPolicy.GetSkipLimit() {
						s.skipPolicy.IncrementSkipCount()
						stepExecution.SkipReadCount++
						stepExecution.AddFailureException(readErr)
						logger.Warnf("ChunkStep '%s': Item read skipped (Skip Count: %d/%d): %v", s.id, s.skipPolicy.GetSkipCount(), s.skipPolicy.GetSkipLimit(), readErr)
						s.notifySkipRead(txCtx, stepExecution, readErr)
						goto NextItemRead // Go to next item
					}

					// Fatal error or retry/skip limit exceeded
					for _, l := range s.itemReadListeners {
						l.OnReadError(txCtx, readErr)
					}
					chunkError = exception.NewBatchError(s.id, "Item read failed (Fatal or limit reached)", readErr, false, false)
					goto EndChunkLoop // Exit the entire chunk processing
				}

				// Read successful
				for _, l := range s.itemReadListeners {
					l.AfterRead(txCtx, item)
				}
				s.metricRecorder.RecordItemRead(txCtx, stepExecution, 1) // Record metric
				break
			}

			// Processing after successful read
			readCount++
			currentChunkReadCount++

			// Process
			processAttempts = 0 // Reset for each item

			// Process retry loop
			for {
				for _, l := range s.itemProcessListeners {
					l.BeforeProcess(txCtx, item)
				}
				processedItem, processErr = s.processor.Process(txCtx, item)

				if processErr != nil {
					be, isBatchError := processErr.(*exception.BatchError)

					// Check if retryable
					if isBatchError && be.IsRetryable() && processAttempts < s.itemRetryPolicy.GetMaxAttempts() {
						processAttempts++
						logger.Warnf("ChunkStep '%s': Item process failed (Attempt %d/%d). Retrying: %v", s.id, processAttempts, s.itemRetryPolicy.GetMaxAttempts(), processErr)
						s.notifyRetryProcess(txCtx, stepExecution, item, processErr)
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
						s.notifySkipProcess(txCtx, stepExecution, item, processErr)
						for _, l := range s.itemProcessListeners {
							l.OnSkipInProcess(txCtx, item, processErr)
						}
						goto NextItemRead // Go to next item
					}

					// Fatal error or retry/skip limit exceeded
					for _, l := range s.itemProcessListeners {
						l.OnProcessError(txCtx, item, processErr)
					}
					chunkError = exception.NewBatchError(s.id, "Item process failed (Fatal or limit reached)", processErr, false, false)
					goto EndReadLoop // Exit the entire chunk processing
				}

				// Process successful
				for _, l := range s.itemProcessListeners {
					l.AfterProcess(txCtx, item, processedItem)
				}
				s.metricRecorder.RecordItemProcess(txCtx, stepExecution, 1) // Record metric
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
			currentTxManager.Rollback(txAdapter)

			// Listener notification (AfterChunk - failed)
			for _, l := range s.chunkListeners {
				l.AfterChunk(txCtx, stepExecution)
			}
			break
		}

		// If read ended with EOF and there are no items to write, rollback transaction and exit loop
		if currentChunkReadCount == 0 && len(itemsToWrite) == 0 {
			currentTxManager.Rollback(txAdapter) // Rollback as transaction was started but nothing was done

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
			for { // This loop is for retrying the entire chunk write operation.
				for _, l := range s.itemWriteListeners {
					l.BeforeWrite(txCtx, itemsToWrite)
				}
				writeErr = s.writer.Write(txCtx, itemsToWrite)

				if writeErr != nil {
					be, isBatchError := writeErr.(*exception.BatchError)

					// 1. Transient error (retry the entire chunk)
					if isBatchError && be.IsRetryable() && writeAttempts < s.itemRetryPolicy.GetMaxAttempts() {
						writeAttempts++
						logger.Warnf("ChunkStep '%s': Item write failed (Attempt %d/%d). Retrying chunk: %v", s.id, writeAttempts, s.itemRetryPolicy.GetMaxAttempts(), writeErr)
						s.notifyRetryWrite(txCtx, stepExecution, itemsToWrite, writeErr)

						// Rollback transaction and continue outer chunk loop (retry).
						currentTxManager.Rollback(txAdapter)
						stepExecution.RollbackCount++
						// TODO: Backoff wait
						goto RetryChunk // Go to outer chunk loop
					}

					// 2. Skippable error (chunk splitting)
					if s.skipPolicy.ShouldSkip(writeErr) && stepExecution.SkipWriteCount < s.skipPolicy.GetSkipLimit() {
						s.skipPolicy.IncrementSkipCount()
						stepExecution.SkipWriteCount++
						stepExecution.AddFailureException(writeErr)
						logger.Warnf("ChunkStep '%s': Item write failed (Skip Count: %d/%d). Triggering chunk splitting.", s.id, s.skipPolicy.GetSkipCount(), s.skipPolicy.GetSkipLimit())

						// 2.1. Rollback transaction
						currentTxManager.Rollback(txAdapter)
						stepExecution.RollbackCount++

						// 2.2. Execute chunk splitting process
						// If chunk splitting succeeds, error items are skipped, and remaining items are committed.
						_, fatalErr := s.HandleSkippableWriteFailure(txCtx, itemsToWrite, stepExecution, currentTxManager) // Ignore remainingItems

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
					for _, l := range s.itemWriteListeners {
						l.OnWriteError(txCtx, itemsToWrite, writeErr)
					}
					currentTxManager.Rollback(txAdapter)
					chunkError = exception.NewBatchError(s.id, "Item write failed (Fatal or limit reached)", writeErr, false, false)
					goto EndChunkLoop // Exit the entire chunk processing
				}

				// Write successful
				for _, l := range s.itemWriteListeners {
					l.AfterWrite(txCtx, itemsToWrite)
				}
				s.metricRecorder.RecordItemWrite(txCtx, stepExecution, int64(len(itemsToWrite))) // Record metric
				break
			}
			writeCount += len(itemsToWrite)
		}

		if s.writer != nil && targetResourceName != "" && resourcePath != "" {
			// Note: For ChunkStep, targetResourceName is expected to be a database connection name.
			_, err := s.dbResolver.ResolveConnection(ctx, targetResourceName)
			if err != nil {
				logger.Errorf("ChunkStep '%s': Failed to resolve DB connection for post-commit verification for '%s': %v", s.id, targetResourceName, err)
			} else {
				logger.Debugf("ChunkStep '%s': Successfully resolved DB connection '%s' for resource path '%s' for post-commit verification.", s.id, targetResourceName, resourcePath)
			}
		}

		// 3.4. Commit transaction
		if commitErr := currentTxManager.Commit(txAdapter); commitErr != nil {
			chunkError = exception.NewBatchError(s.id, "Failed to commit transaction for chunk", commitErr, false, false)

			// Listener notification (AfterChunk - failed)
			for _, l := range s.chunkListeners {
				l.AfterChunk(txCtx, stepExecution)
			}
			break
		}
		commitCount++

		// 3.5. Update ItemStream state (Update hook)
		// After successful commit, update the state of ItemStream components.
		if stream, ok := s.reader.(port.ItemStream); ok {
			if err := stream.Update(txCtx, stepExecution.ExecutionContext); err != nil {
				logger.Warnf("ChunkStep '%s': Failed to update ItemReader state: %v", s.id, err)
			}
		}
		if stream, ok := s.processor.(port.ItemStream); ok {
			if err := stream.Update(txCtx, stepExecution.ExecutionContext); err != nil {
				logger.Warnf("ChunkStep '%s': Failed to update ItemProcessor state: %v", s.id, err)
			}
		}
		if stream, ok := s.writer.(port.ItemStream); ok {
			if err := stream.Update(txCtx, stepExecution.ExecutionContext); err != nil {
				logger.Warnf("ChunkStep '%s': Failed to update ItemWriter state: %v", s.id, err)
			}
		}

		// Listener notification (AfterChunk - successful)
		for _, l := range s.chunkListeners {
			l.AfterChunk(txCtx, stepExecution)
		}

		// 3.6. Save checkpoint
		// After successful commit, save Reader/Writer state and statistics
		if err := s.saveCheckpoint(ctx, stepExecution, readCount, writeCount); err != nil {
			logger.Errorf("ChunkStep '%s': Failed to save checkpoint after commit: %v", s.id, err)
			// Checkpoint save failure is not fatal, but log it
		}

		// 3.7. Determine end of chunk processing
		if isEOF {
			// If read ended with EOF
			logger.Debugf("ChunkStep '%s': Reached EOF. Exiting chunk loop.", s.id)
			break
		}
	EndChunkTransaction:
		continue
	} // End of chunk loop
EndChunkLoop:

	// 4. Finalization

	// Update statistics
	stepExecution.ReadCount = readCount
	stepExecution.WriteCount = writeCount
	stepExecution.CommitCount = commitCount

	// Close Reader/Writer
	if closeErr := s.reader.Close(ctx); closeErr != nil {
		logger.Warnf("ChunkStep '%s': Failed to close ItemReader: %v", s.id, closeErr)
		if chunkError == nil {
			chunkError = closeErr
		}
	}
	if s.writer != nil {
		if closeErr := s.writer.Close(ctx); closeErr != nil {
			logger.Warnf("ChunkStep '%s': Failed to close ItemWriter: %v", s.id, closeErr)
			if chunkError == nil {
				chunkError = closeErr
			}
		}
	}

	// Update StepExecution.ExecutionContext to the latest state
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
		if s.metricRecorder != nil {
			s.metricRecorder.RecordExecutionError(ctx, chunkError) // Record execution error.
		}
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

// Close finalizes the step execution by saving the final checkpoint.
//
// Parameters:
//
//	ctx: The context for the operation.
//
// Returns:
//
//	error: An error if saving the checkpoint fails.
func (s *ChunkStep) Close(ctx context.Context) error {
	if s.currentStepExecution != nil {
		logger.Debugf("ChunkStep '%s': Finalizing checkpoint in Close.", s.id)
		if err := s.saveCheckpoint(ctx, s.currentStepExecution, s.currentStepExecution.ReadCount, s.currentStepExecution.WriteCount); err != nil {
			logger.Errorf("ChunkStep '%s': Failed to save final checkpoint in Close: %v", s.id, err)
			return err
		}
	}
	return nil
}

// saveCheckpoint persists the current state of the reader and writer to the
// [repository.JobRepository]. It is called after successful chunk commits to
// ensure restartability.
//
// Parameters:
//
//	ctx: Context for the operation.
//	stepExecution: Current [model.StepExecution].
//	readCount: Total items read.
//	writeCount: Total items written.
//
// Returns:
//
//	error: Returns an error if checkpoint persistence fails.
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

// HandleSkippableWriteFailure implements chunk splitting. When a skippable write
// error occurs, it re-writes items individually to isolate and skip the faulty
// item, allowing the remaining items in the chunk to be committed.
//
// Parameters:
//
//	ctx: Context for the operation.
//	originalItems: Items that caused the write failure.
//	stepExecution: Current [model.StepExecution].
//	currentTxManager: Transaction manager for the current chunk.
//
// Returns:
//
//	[]any: Empty slice (all items are either committed or skipped).
//	error: Returns a fatal error if splitting fails or skip limits are exceeded.
func (s *ChunkStep) HandleSkippableWriteFailure(ctx context.Context, originalItems []any, stepExecution *model.StepExecution, currentTxManager tx.TransactionManager) ([]any, error) {
	taskletName := s.id

	// Remaining items after chunk splitting (expected to be empty if this function succeeds)
	var remainingItems []any

	// Hold fatal error if it occurs during chunk splitting
	var fatalError error

	// 1. Re-write items one by one to identify errors
	for i, item := range originalItems {
		// 1.1. Begin transaction for a single item
		txAdapter, err := currentTxManager.Begin(ctx, s.GetTransactionOptions())
		if err != nil {
			fatalError = exception.NewBatchError(taskletName, "Failed to begin transaction for chunk splitting", err, false, false)
			break
		}
		txCtx := tx.ContextWithTx(ctx, txAdapter)

		// 1.2. Write single item
		writeErr := s.writer.Write(txCtx, []any{item})

		if writeErr != nil {
			// 1.3. Write failed: Skip processing

			// Re-check if skip limit is exceeded
			if s.skipPolicy.ShouldSkip(writeErr) && stepExecution.SkipWriteCount < s.skipPolicy.GetSkipLimit() {
				s.skipPolicy.IncrementSkipCount()
				stepExecution.SkipWriteCount++
				stepExecution.AddFailureException(writeErr)
				s.notifySkipWrite(txCtx, stepExecution, item, writeErr)

				// Rollback
				currentTxManager.Rollback(txAdapter)
				logger.Warnf("ChunkStep '%s': Item skipped during chunk splitting: %+v", taskletName, item)

				// This item was skipped, so do not add to remainingItems
			} else {
				// Skip limit exceeded or fatal error
				currentTxManager.Rollback(txAdapter)
				fatalError = exception.NewBatchError(taskletName, fmt.Sprintf("Item write failed during chunk splitting (Fatal or limit reached) for item index %d", i), writeErr, false, false)
				break
			}
		} else {
			// 1.4. Write successful: Commit
			if commitErr := currentTxManager.Commit(txAdapter); commitErr != nil {
				fatalError = exception.NewBatchError(taskletName, "Failed to commit transaction during chunk splitting", commitErr, false, false)
				break
			}

			// Do not add successful items to remainingItems (as they are already persisted)
			writeCount := stepExecution.WriteCount + 1
			stepExecution.WriteCount = writeCount
			s.metricRecorder.RecordItemWrite(txCtx, stepExecution, 1)
		}
	}

	// At the completion of chunk splitting, all items from the original chunk are considered processed.
	// Successful items were committed, and failed items were skipped.
	// Therefore, this function returns an empty remainingItems and fatalError.

	return remainingItems, fatalError
}
