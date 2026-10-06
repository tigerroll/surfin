// Package partition provides the PartitionStep implementation for the batch engine.
package partition

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"golang.org/x/sync/semaphore"

	port "github.com/tigerroll/surfin/pkg/batch/core/application/port"
	model "github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	repository "github.com/tigerroll/surfin/pkg/batch/core/domain/repository"
	metrics "github.com/tigerroll/surfin/pkg/batch/core/metrics"
	exception "github.com/tigerroll/surfin/pkg/batch/support/util/exception"
	logger "github.com/tigerroll/surfin/pkg/batch/support/util/logger"
)

// PartitionStep acts as a controller step that executes a worker step across multiple parallel partitions.
// It manages the lifecycle of worker partitions, enforces concurrency limits, and aggregates execution results.
type PartitionStep struct {
	id          string
	partitioner port.Partitioner
	workerStep  port.Step
	gridSize    int
	// concurrency specifies the maximum number of partitions to execute in parallel.
	concurrency            int
	jobRepository          repository.JobRepository
	stepExecutionListeners []port.StepExecutionListener
	promotion              *model.ExecutionContextPromotion
	stepExecutor           port.StepExecutor // SimpleStepExecutor or RemoteStepExecutor
	metricRecorder         metrics.MetricRecorder
	tracer                 metrics.Tracer
}

// NewPartitionStep initializes a new PartitionStep.
//
// Parameters:
//
//	id: The unique identifier for the step.
//	partitioner: The component responsible for generating partitions.
//	workerStep: The step to be executed within each partition.
//	gridSize: The total number of partitions to create.
//	concurrency: The maximum number of partitions to execute in parallel.
//	jobRepository: The repository for persisting job metadata.
//	stepExecutionListeners: Listeners to be notified during step execution.
//	promotion: Configuration for promoting execution context to the job level.
//	stepExecutor: The executor responsible for running the worker step.
func NewPartitionStep(
	id string,
	partitioner port.Partitioner,
	workerStep port.Step,
	gridSize int,
	concurrency int,
	jobRepository repository.JobRepository,
	stepExecutionListeners []port.StepExecutionListener,
	promotion *model.ExecutionContextPromotion,
	stepExecutor port.StepExecutor,
) port.Step {
	return &PartitionStep{
		id:                     id,
		partitioner:            partitioner,
		workerStep:             workerStep,
		gridSize:               gridSize,
		concurrency:            concurrency,
		jobRepository:          jobRepository,
		stepExecutionListeners: stepExecutionListeners,
		promotion:              promotion,
		stepExecutor:           stepExecutor,
		tracer:                 metrics.NewNoOpTracer(),
		metricRecorder:         metrics.NewNoOpMetricRecorder(),
	}
}

// SetMetricRecorder implements the port.Step interface.
func (s *PartitionStep) SetMetricRecorder(recorder metrics.MetricRecorder) {
	s.metricRecorder = recorder
}

// SetTracer implements the port.Step interface.
func (s *PartitionStep) SetTracer(tracer metrics.Tracer) {
	s.tracer = tracer
}

// GetExecutionContextPromotion implements the port.Step interface.
// PartitionStep does not support execution context promotion directly.
func (s *PartitionStep) GetExecutionContextPromotion() *model.ExecutionContextPromotion {
	return nil
}

// ID returns the unique identifier of the step.
func (s *PartitionStep) ID() string {
	return s.id
}

// StepName returns the logical name of the step.
func (s *PartitionStep) StepName() string {
	return s.id
}

// GetTransactionOptions returns the transaction options for this step.
// As a controller step, PartitionStep does not require a transaction boundary.
func (s *PartitionStep) GetTransactionOptions() *sql.TxOptions {
	return nil
}

// GetPropagation returns the transaction propagation attribute.
// As a controller step, PartitionStep does not require a transaction boundary.
func (s *PartitionStep) GetPropagation() string {
	return ""
}

// notifyBeforeStep triggers the BeforeStep event for all registered listeners.
func (s *PartitionStep) notifyBeforeStep(ctx context.Context, stepExecution *model.StepExecution) {
	for _, l := range s.stepExecutionListeners {
		l.BeforeStep(ctx, stepExecution)
	}
}

// notifyAfterStep triggers the AfterStep event for all registered listeners.
func (s *PartitionStep) notifyAfterStep(ctx context.Context, stepExecution *model.StepExecution) {
	for _, l := range s.stepExecutionListeners {
		l.AfterStep(ctx, stepExecution)
	}
}

// determineAggregatedStatus calculates the final status of the controller step based on the
// execution results of all worker partitions, adhering to the defined Failure Matrix.
// Priority: FAILED > STOPPED/CANCELLED > COMPLETED
func (s *PartitionStep) determineAggregatedStatus(workerExecutions []*model.StepExecution) (model.JobStatus, model.ExitStatus) {
	var (
		hasFailed    bool
		hasStopped   bool
		hasCancelled bool
	)

	for _, exec := range workerExecutions {
		switch exec.Status {
		case model.BatchStatusFailed, model.BatchStatusAbandoned:
			hasFailed = true
		case model.BatchStatusStopped:
			hasStopped = true
		case model.BatchStatusCancelled:
			hasCancelled = true
		}
	}

	// 1. Validate that all workers are in a known terminal state.
	for _, exec := range workerExecutions {
		if exec.Status != model.BatchStatusCompleted &&
			exec.Status != model.BatchStatusFailed &&
			exec.Status != model.BatchStatusStopped &&
			exec.Status != model.BatchStatusCancelled &&
			exec.Status != model.BatchStatusAbandoned {
			return model.BatchStatusFailed, model.ExitStatusFailed
		}
	}

	// 2. Apply Failure Matrix priority
	if hasFailed {
		return model.BatchStatusFailed, model.ExitStatusFailed
	}
	if hasStopped || hasCancelled {
		return model.BatchStatusStopped, model.ExitStatusStopped
	}

	return model.BatchStatusCompleted, model.ExitStatusCompleted
}

// Execute orchestrates the partitioning process. It generates partitions, manages worker
// execution via goroutines with concurrency control, and aggregates the final status,
// statistics, and execution context.
func (s *PartitionStep) Execute(ctx context.Context, jobExecution *model.JobExecution, controllerExecution *model.StepExecution) (err error) {
	// Start the controller span for tracing.
	ctx, finishSpan := s.tracer.StartStepSpan(ctx, controllerExecution)
	defer finishSpan()

	logger.Infof("PartitionStep '%s' executing (GridSize: %d, Concurrency: %d).", s.id, s.gridSize, s.concurrency)

	// 1. Update the status of the Controller StepExecution to STARTED.
	controllerExecution.MarkAsStarted()
	if err := s.jobRepository.UpdateStepExecution(ctx, controllerExecution); err != nil {
		return exception.NewBatchError(s.id, "Failed to update Controller StepExecution status to STARTED", err, false, false)
	}

	// 2. Notify listeners (BeforeStep).
	s.notifyBeforeStep(ctx, controllerExecution)

	// 3. Execute the Partitioner to get a map of ExecutionContexts.
	partitionContexts, err := s.partitioner.Partition(ctx, s.gridSize)
	if err != nil {
		controllerExecution.MarkAsFailed(err)
		s.jobRepository.UpdateStepExecution(ctx, controllerExecution)
		s.notifyAfterStep(ctx, controllerExecution)
		return exception.NewBatchError(s.id, "Failed to execute Partitioner", err, false, false)
	}

	// Defensive check: ensure partitionContexts is not nil to avoid panic.
	if partitionContexts == nil {
		partitionContexts = make(map[string]model.ExecutionContext)
	}

	logger.Infof("PartitionStep '%s': Partitioner returned %d partitions.", s.id, len(partitionContexts))

	// 4. Execute each partition in parallel.
	var wg sync.WaitGroup
	errChan := make(chan error, len(partitionContexts))

	// List of Worker StepExecutions (for aggregation).
	workerExecutionsChan := make(chan *model.StepExecution, len(partitionContexts))

	// Create semaphore only if concurrency is greater than 0
	var sem *semaphore.Weighted
	if s.concurrency > 0 {
		sem = semaphore.NewWeighted(int64(s.concurrency))
	}

	for partitionName, partitionEC := range partitionContexts {
		wg.Add(1)

		// Create Worker StepExecution.
		workerExecutionID := uuid.New().String()
		workerExecution := model.NewStepExecution(workerExecutionID, jobExecution, s.workerStep.StepName())
		workerExecution.ExecutionContext = partitionEC

		// Add Worker StepExecution to JobExecution (JobExecution persistence is handled by JobRunner).
		jobExecution.AddStepExecution(workerExecution)

		// Persist Worker StepExecution (initial state).
		if err := s.jobRepository.SaveStepExecution(ctx, workerExecution); err != nil {
			logger.Errorf("PartitionStep '%s': Failed to save Worker StepExecution '%s': %v", s.id, workerExecutionID, err)
			errChan <- exception.NewBatchError(s.id, fmt.Sprintf("Failed to save Worker StepExecution %s", workerExecutionID), err, false, false)
			wg.Done()
			continue
		}

		logger.Debugf("PartitionStep '%s': Starting worker '%s' (Worker StepExecution ID: %s).", s.id, partitionName, workerExecutionID)

		go func(workerExec *model.StepExecution, pName string) {
			defer wg.Done()

			// Start Span for the worker
			workerCtx, finishSpan := s.tracer.StartStepSpan(ctx, workerExec)
			defer finishSpan()
			s.tracer.RecordEvent(workerCtx, "partition_worker_start", map[string]interface{}{"partition.name": pName})

			// Acquire semaphore only if enabled
			if sem != nil {
				if err := sem.Acquire(workerCtx, 1); err != nil {
					// If the error is context cancellation, mark as CANCELLED; otherwise, mark as FAILED.
					if errors.Is(err, context.Canceled) {
						workerExec.Status = model.BatchStatusCancelled
						workerExec.ExitStatus = model.ExitStatusStopped
					} else {
						workerExec.MarkAsFailed(err)
					}
					errChan <- err
					s.tracer.RecordError(workerCtx, "partition_step", err)
					workerExecutionsChan <- workerExec
					return
				}
				defer sem.Release(1)
			}

			// Start measurement after semaphore acquisition to exclude waiting time from the duration.
			startTime := time.Now()

			// Execute the Worker Step using the StepExecutor.
			completedWorkerExec, execErr := s.stepExecutor.ExecuteStep(workerCtx, s.workerStep, jobExecution, workerExec)

			// Record the execution duration.
			duration := time.Since(startTime).Seconds()
			s.metricRecorder.RecordDuration(workerCtx, "partition_worker_duration", duration,
				attribute.String("partition.name", pName),
				attribute.String("status", completedWorkerExec.Status.String()))

			if execErr != nil {
				// Treat context.Canceled as CANCELLED instead of FAILED.
				if errors.Is(execErr, context.Canceled) {
					logger.Infof("PartitionStep '%s': Worker '%s' was cancelled.", s.id, workerExec.StepName)
					completedWorkerExec.Status = model.BatchStatusCancelled
					completedWorkerExec.ExitStatus = model.ExitStatusStopped // Set exit status to STOPPED for cancelled workers.
				} else {
					logger.Errorf("PartitionStep '%s': Worker '%s' failed: %v", s.id, workerExec.StepName, execErr)
					// Ensure MarkAsFailed is called when an error occurs.
					completedWorkerExec.MarkAsFailed(execErr)
					errChan <- execErr
					s.tracer.RecordError(workerCtx, "partition_step", execErr)
				}
			} else {
				logger.Infof("PartitionStep '%s': Worker '%s' completed with status: %s", s.id, workerExec.StepName, completedWorkerExec.Status)
				s.tracer.RecordEvent(workerCtx, "partition_worker_success", map[string]interface{}{"status": completedWorkerExec.Status.String()})
			}

			// Send the completed Worker StepExecution for aggregation.
			workerExecutionsChan <- completedWorkerExec
		}(workerExecution, partitionName)
	}

	logger.Debugf("PartitionStep '%s': Waiting for workers to finish.", s.id)
	wg.Wait()
	logger.Debugf("PartitionStep '%s': All workers finished.", s.id)
	close(errChan)
	close(workerExecutionsChan)

	// 5. Aggregate results.
	var combinedError error
	for err := range errChan {
		combinedError = errors.Join(combinedError, err)
	}

	// Collect worker execution results into a slice
	var workerExecutions []*model.StepExecution
	for workerExec := range workerExecutionsChan {
		workerExecutions = append(workerExecutions, workerExec)
	}

	// Sort worker executions by ID to ensure deterministic ExecutionContext merge order.
	sort.Slice(workerExecutions, func(i, j int) bool {
		return workerExecutions[i].ID < workerExecutions[j].ID
	})

	// Determine final status
	finalStatus, finalExitStatus := s.determineAggregatedStatus(workerExecutions)

	// Aggregate statistics
	totalRead := 0
	totalWrite := 0
	aggregatedEC := model.NewExecutionContext()

	for _, workerExec := range workerExecutions {
		totalRead += workerExec.ReadCount
		totalWrite += workerExec.WriteCount

		// Aggregate failure messages
		if workerExec.Status == model.BatchStatusFailed {
			controllerExecution.Failures = append(controllerExecution.Failures, workerExec.Failures...)
		}

		// Merge ExecutionContext
		for k, v := range workerExec.ExecutionContext {
			// Skip internal keys used for partition identification
			if k == "partition.name" {
				continue
			}
			if _, exists := aggregatedEC[k]; exists {
				return exception.NewBatchError(s.id, fmt.Sprintf("duplicate ExecutionContext key found: %s", k), nil, false, false)
			}
			aggregatedEC[k] = v
		}
	}

	controllerExecution.ReadCount = totalRead
	controllerExecution.WriteCount = totalWrite
	controllerExecution.ExecutionContext = aggregatedEC

	// 6. Update Controller StepExecution status
	controllerExecution.Status = finalStatus
	controllerExecution.ExitStatus = finalExitStatus

	if finalStatus == model.BatchStatusFailed {
		if combinedError == nil {
			combinedError = fmt.Errorf("one or more partitions failed")
		}
		wrappedErr := exception.NewBatchError(s.id, "one or more partitions failed", combinedError, false, false)
		controllerExecution.MarkAsFailed(wrappedErr)
		combinedError = wrappedErr
	} else if finalStatus == model.BatchStatusStopped {
		controllerExecution.MarkAsStopped()
		// If status is STOPPED, treat as a successful completion by setting combinedError to nil.
		combinedError = nil
	} else {
		controllerExecution.MarkAsCompleted()
	}

	// 7. Promote ExecutionContext (Controller EC -> Job EC).
	if s.promotion != nil {
		s.promoteExecutionContext(controllerExecution, jobExecution)
	}

	// 8. Notify listeners (AfterStep).
	s.notifyAfterStep(ctx, controllerExecution)

	// 9. Persist.
	if updateErr := s.jobRepository.UpdateStepExecution(ctx, controllerExecution); updateErr != nil {
		logger.Errorf("PartitionStep '%s': Failed to update final Controller StepExecution state: %v", s.id, updateErr)
		if combinedError == nil {
			combinedError = updateErr
		}
	}

	logger.Infof("PartitionStep '%s' finished. ExitStatus: %s", s.id, controllerExecution.ExitStatus)
	return combinedError
}

// promoteExecutionContext propagates specific keys from the StepExecutionContext to the JobExecutionContext
// based on the promotion configuration.
func (s *PartitionStep) promoteExecutionContext(stepExecution *model.StepExecution, jobExecution *model.JobExecution) {
	if s.promotion == nil {
		return
	}

	// 1. Keys promotion
	for _, key := range s.promotion.Keys {
		if val, ok := stepExecution.ExecutionContext.GetNested(key); ok {
			jobExecution.ExecutionContext.PutNested(key, val)
			logger.Debugf("PartitionStep '%s': Promoted key '%s' to JobExecutionContext.", s.id, key)
		}
	}

	// 2. JobLevelKeys promotion (with renaming)
	for stepKey, jobKey := range s.promotion.JobLevelKeys {
		if val, ok := stepExecution.ExecutionContext.GetNested(stepKey); ok {
			jobExecution.ExecutionContext.PutNested(jobKey, val)
			logger.Debugf("PartitionStep '%s': Promoted and renamed key '%s' to '%s' in JobExecutionContext.", s.id, stepKey, jobKey)
		}
	}
}

// Verify that PartitionStep implements the core.Step interface.
var _ port.Step = (*PartitionStep)(nil)
