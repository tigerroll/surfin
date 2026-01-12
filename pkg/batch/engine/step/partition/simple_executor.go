package partition

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	core "github.com/tigerroll/surfin/pkg/batch/core/application/port"
	model "github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	metrics "github.com/tigerroll/surfin/pkg/batch/core/metrics"
	tx "github.com/tigerroll/surfin/pkg/batch/core/tx"
	exception "github.com/tigerroll/surfin/pkg/batch/support/util/exception"
	logger "github.com/tigerroll/surfin/pkg/batch/support/util/logger"

	item "github.com/tigerroll/surfin/pkg/batch/engine/step/item"
)

// SimpleStepExecutor is the simplest implementation of the StepExecutor interface,
// executing the worker Step synchronously within the caller's Go routine.
// This executor is responsible for establishing the transaction boundary for the Step execution.
type SimpleStepExecutor struct {
	tracer    metrics.Tracer
	recorder  metrics.MetricRecorder
	txManager tx.TransactionManager
}

// NewSimpleStepExecutor creates a new instance of SimpleStepExecutor.
func NewSimpleStepExecutor(tracer metrics.Tracer, recorder metrics.MetricRecorder, txManager tx.TransactionManager) core.StepExecutor {
	return &SimpleStepExecutor{
		tracer:    tracer,
		recorder:  recorder,
		txManager: txManager,
	}
}

// ExecuteStep executes the worker Step synchronously.
// This method establishes the transaction boundary for the entire Step execution.
func (e *SimpleStepExecutor) ExecuteStep(ctx context.Context, step core.Step, jobExecution *model.JobExecution, stepExecution *model.StepExecution) (*model.StepExecution, error) {
	workerStepName := stepExecution.StepName
	logger.Infof("Partition Step Executor: Starting Worker Step '%s'.", workerStepName)

	// Start a Span using the Tracer
	ctx, finishSpan := e.tracer.StartStepSpan(ctx, stepExecution)
	defer finishSpan()

	// StepExecution should already have a reference to JobExecution, but set it just in case.
	stepExecution.JobExecution = jobExecution

	var txAdapter tx.Tx
	var err error
	var txStartedByExecutor bool = false // Tracks if the Executor started the transaction.

	// T_TX_DEC: Inject MetricRecorder and Tracer into the Step
	step.SetMetricRecorder(e.recorder)
	step.SetTracer(e.tracer)

	// T_TX_DEC_3: Variable to hold suspended transaction
	var suspendedTx tx.Tx

	// T_TX_DEC_6: Savepoint name
	var savepointName string

	// Determine if it's a ChunkStep (ChunkStep manages transactions internally, so no external transaction is needed)
	_, isChunkStep := step.(*item.ChunkStep)

	// T_TX_DEC: Get transaction options and propagation attribute from Step
	var txOpts *sql.TxOptions
	var propagation string = "REQUIRED" // Default value

	if stepWithOptions, ok := step.(interface {
		GetTransactionOptions() *sql.TxOptions
		GetPropagation() string // Assume TaskletStep has this method
	}); ok {
		txOpts = stepWithOptions.GetTransactionOptions()
		propagation = stepWithOptions.GetPropagation()
		logger.Debugf("Using custom transaction options for step '%s'. Isolation Level: %v, Propagation: %s", workerStepName, txOpts.Isolation, propagation)
	} else if stepWithOptions, ok := step.(interface{ GetTransactionOptions() *sql.TxOptions }); ok {
		txOpts = stepWithOptions.GetTransactionOptions()
		logger.Debugf("Using custom transaction options for step '%s'. Isolation Level: %v, Propagation: %s (Default)", workerStepName, txOpts.Isolation, propagation)
	} else {
		txOpts = nil
	}

	// 1. Transaction start/join logic (only if not a ChunkStep)
	if !isChunkStep {
		// Get existing transaction from context
		existingTx, hasExistingTx := ctx.Value("tx").(tx.Tx)

		switch propagation {
		case "REQUIRED", "": // REQUIRED or default (empty string)
			if hasExistingTx {
				// Join existing transaction (do nothing)
				txAdapter = existingTx
				logger.Debugf("Propagation REQUIRED: Joining existing transaction for Worker Step '%s'.", workerStepName)
			} else {
				// Start a new transaction
				txAdapter, err = e.txManager.Begin(ctx, txOpts)
				if err != nil {
					e.tracer.RecordError(ctx, "simple_executor", err)
					return stepExecution, exception.NewBatchError("simple_executor", fmt.Sprintf("Failed to begin transaction for Worker Step '%s'", workerStepName), err, false, false)
				}
				txStartedByExecutor = true
				logger.Debugf("Propagation REQUIRED: Starting new transaction for Worker Step '%s'.", workerStepName)
			}

		case "REQUIRES_NEW":
			// Always start a new transaction regardless of whether an existing transaction exists
			if hasExistingTx {
				// Suspend existing transaction (remove from context)
				suspendedTx = existingTx
				logger.Debugf("Propagation REQUIRES_NEW: Suspending existing transaction for Worker Step '%s'.", workerStepName)
			}

			txAdapter, err = e.txManager.Begin(ctx, txOpts)
			if err != nil {
				e.tracer.RecordError(ctx, "simple_executor", err)
				return stepExecution, exception.NewBatchError("simple_executor", fmt.Sprintf("Failed to begin new transaction for Worker Step '%s'", workerStepName), err, false, false)
			}
			txStartedByExecutor = true
			logger.Debugf("Propagation REQUIRES_NEW: Starting new transaction for Worker Step '%s'.", workerStepName)

		case "NOT_SUPPORTED":
			// Does not support transactions
			if hasExistingTx {
				// Suspend existing transaction
				suspendedTx = existingTx
				// Do not include Tx in the new context
				ctx = context.WithValue(ctx, "tx", nil) // Remove Tx from context
				logger.Debugf("Propagation NOT_SUPPORTED: Suspending existing transaction for Worker Step '%s'.", workerStepName)
			} else {
				logger.Debugf("Propagation NOT_SUPPORTED: No transaction active for Worker Step '%s'.", workerStepName)
			}
			// txAdapter remains nil

		case "SUPPORTS": // T_TX_DEC_4: Join existing transaction if present, otherwise no transaction
			if hasExistingTx {
				txAdapter = existingTx
				logger.Debugf("Propagation SUPPORTS: Joining existing transaction for Worker Step '%s'.", workerStepName)
			} else {
				// Execute without transaction
				logger.Debugf("Propagation SUPPORTS: No existing transaction found. Running without transaction for Worker Step '%s'.", workerStepName)
			}
			// txAdapter remains existingTx or nil. txStartedByExecutor remains false.

		case "NEVER": // T_TX_DEC_4: No external transaction must exist
			if hasExistingTx {
				// Error because external transaction exists
				return stepExecution, exception.NewBatchErrorf("simple_executor", "Propagation NEVER: Existing transaction found for Worker Step '%s'. Execution aborted.", workerStepName)
			}
			// Execute without transaction
			logger.Debugf("Propagation NEVER: No transaction active for Worker Step '%s'.", workerStepName)

		case "MANDATORY": // T_TX_DEC_5: External transaction is mandatory
			if !hasExistingTx {
				// Error because external transaction does not exist
				return stepExecution, exception.NewBatchErrorf("simple_executor", "Propagation MANDATORY: No existing transaction found for Worker Step '%s'. Execution aborted.", workerStepName)
			}
			// Join existing transaction
			txAdapter = existingTx
			logger.Debugf("Propagation MANDATORY: Joining existing transaction for Worker Step '%s'.", workerStepName)
			// txStartedByExecutor remains false

		case "NESTED": // T_TX_DEC_6: Nested transaction (savepoint)
			if hasExistingTx {
				// Create a savepoint within the existing transaction
				txAdapter = existingTx
				savepointName = "SP_" + stepExecution.ID // Use StepExecution ID to create a unique savepoint name

				if spErr := txAdapter.Savepoint(savepointName); spErr != nil {
					// Savepoint creation failure is fatal
					return stepExecution, exception.NewBatchError("simple_executor", fmt.Sprintf("Propagation NESTED: Failed to create savepoint '%s' for Worker Step '%s'", savepointName, workerStepName), spErr, false, false)
				}
				logger.Debugf("Propagation NESTED: Created savepoint '%s' within existing transaction for Worker Step '%s'.", savepointName, workerStepName)
				// txStartedByExecutor remains false (because it's joining an external transaction)
			} else {
				// If no external transaction exists, start a new transaction similar to REQUIRED
				txAdapter, err = e.txManager.Begin(ctx, txOpts)
				if err != nil {
					e.tracer.RecordError(ctx, "simple_executor", err)
					return stepExecution, exception.NewBatchError("simple_executor", fmt.Sprintf("Failed to begin new transaction for Worker Step '%s'", workerStepName), err, false, false)
				}
				txStartedByExecutor = true
				logger.Debugf("Propagation NESTED: Starting new transaction (acting as REQUIRED) for Worker Step '%s'.", workerStepName)
			}

		default:
			// Unsupported propagation attribute is an error
			return stepExecution, exception.NewBatchErrorf("simple_executor", "Unsupported transaction propagation attribute: %s", propagation)
		}

		// If a transaction was started, create a new context
		if txAdapter != nil {
			ctx = context.WithValue(ctx, "tx", txAdapter)
		}
	} else {
		// For ChunkStep, transactions are managed internally, so no external transaction is started.
		logger.Debugf("ChunkStep detected. Skipping external transaction start by SimpleStepExecutor.")
	}

	// 2. Step execution
	executionCtx := core.GetContextWithStepExecution(ctx, stepExecution)
	err = step.Execute(executionCtx, jobExecution, stepExecution)

	// 3. Transaction finalization
	if txStartedByExecutor {
		// Only process commit/rollback if the Executor started the transaction
		if err != nil {
			// If an error occurred, rollback
			if rbErr := e.txManager.Rollback(txAdapter); rbErr != nil {
				logger.Errorf("SimpleStepExecutor: Failed to rollback transaction for Worker Step '%s': %v", workerStepName, rbErr)
			}
			logger.Errorf("Partition Step Executor: Worker Step '%s' failed: %v", workerStepName, err)
			e.tracer.RecordError(ctx, "simple_executor", err)
			return stepExecution, exception.NewBatchError("partition_worker", fmt.Sprintf("Worker Step '%s' execution failed", workerStepName), err, false, false)
		}

		// If successful, commit
		if commitErr := e.txManager.Commit(txAdapter); commitErr != nil {
			err = exception.NewBatchError("simple_executor", fmt.Sprintf("Failed to commit transaction for Worker Step '%s'", workerStepName), commitErr, false, false)
			logger.Errorf("Partition Step Executor: Worker Step '%s' failed during commit: %v", workerStepName, err)
			e.tracer.RecordError(ctx, "simple_executor", err)
			return stepExecution, err
		}
	} else if txAdapter != nil && (propagation == "REQUIRED" || propagation == "" || propagation == "SUPPORTS" || propagation == "MANDATORY" || propagation == "NESTED") && !txStartedByExecutor {
		// If joined an existing transaction with REQUIRED, SUPPORTS, MANDATORY, NESTED, commit/rollback is delegated externally

		if propagation == "NESTED" {
			// For NESTED, if no error, release savepoint (GORM automatically releases on commit, so do nothing here)

			if err != nil {
				// If an error occurred during Step execution, rollback to savepoint
				if rbErr := txAdapter.RollbackToSavepoint(savepointName); rbErr != nil {
					logger.Errorf("SimpleStepExecutor: Failed to rollback to savepoint '%s' for Worker Step '%s': %v", savepointName, workerStepName, rbErr)
					// Rollback failure is joined with the original error
					err = errors.Join(err, rbErr)
				} else {
					logger.Debugf("Propagation NESTED: Rolled back to savepoint '%s' for Worker Step '%s'.", savepointName, workerStepName)
				}
				// External transaction is still active, so return the error
				return stepExecution, err
			}

			// If successful, savepoint is automatically committed (depends on GORM behavior)
			logger.Debugf("Propagation NESTED: Transaction management delegated to external caller (Savepoint '%s' released/committed) for Worker Step '%s'.", savepointName, workerStepName)

		} else {
			// For REQUIRED, SUPPORTS, MANDATORY
			logger.Debugf("Propagation %s: Transaction management delegated to external caller for Worker Step '%s'.", propagation, workerStepName)
		}

	} else if isChunkStep && err == nil {
		// If ChunkStep succeeded, no external transaction was started, so do nothing.
	} else if isChunkStep && err != nil {
		// If ChunkStep failed, it has already been rolled back internally.
	}

	// T_TX_DEC_3: Restore suspended transaction
	if suspendedTx != nil {
		// For REQUIRES_NEW or NOT_SUPPORTED, restore the suspended transaction to the context
		// NOTE: Go's Context is immutable, so only logging is performed here
		logger.Debugf("Restoring suspended transaction context for Worker Step '%s'.", workerStepName)
	}

	logger.Infof("Partition Step Executor: Worker Step '%s' completed. Status: %s", workerStepName, stepExecution.Status)
	return stepExecution, nil
}

// Verify that SimpleStepExecutor implements the core.StepExecutor interface.
var _ core.StepExecutor = (*SimpleStepExecutor)(nil)
