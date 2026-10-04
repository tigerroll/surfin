// Package partition provides the PartitionStep implementation for the batch engine.
package partition

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"golang.org/x/sync/semaphore"

	port "github.com/tigerroll/surfin/pkg/batch/core/application/port"
	model "github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	repository "github.com/tigerroll/surfin/pkg/batch/core/domain/repository"
	metrics "github.com/tigerroll/surfin/pkg/batch/core/metrics"
	exception "github.com/tigerroll/surfin/pkg/batch/support/util/exception"
	logger "github.com/tigerroll/surfin/pkg/batch/support/util/logger"
)

// PartitionStep is an implementation of core.Step that executes a worker step in parallel partitions.
// It acts as a controller step, managing the lifecycle of worker partitions and aggregating their results.
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

// NewPartitionStep creates a new PartitionStep instance.
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
// PartitionStep is a controller and does not require a transaction boundary.
func (s *PartitionStep) GetTransactionOptions() *sql.TxOptions {
	return nil
}

// GetPropagation returns the transaction propagation attribute.
// PartitionStep is a controller and does not require a transaction boundary.
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

// determineAggregatedStatus determines the final status of the controller step based on the worker execution results,
// following the defined Failure Matrix.
func (s *PartitionStep) determineAggregatedStatus(workerExecutions []*model.StepExecution) (model.JobStatus, model.ExitStatus) {
	hasFailed := false
	hasStopped := false
	hasCancelled := false

	for _, exec := range workerExecutions {
		if exec.Status == model.BatchStatusFailed || exec.Status == model.BatchStatusAbandoned {
			hasFailed = true
		} else if exec.Status == model.BatchStatusStopped {
			hasStopped = true
		} else if exec.Status == model.BatchStatusCancelled {
			hasCancelled = true
		}
	}

	// Apply Failure Matrix priority: FAILED > STOPPED/CANCELLED > COMPLETED
	if hasFailed {
		return model.BatchStatusFailed, model.ExitStatusFailed
	}
	if hasStopped || hasCancelled {
		return model.BatchStatusStopped, model.ExitStatusStopped
	}
	return model.BatchStatusCompleted, model.ExitStatusCompleted
}

// Execute runs the partitioning logic, managing worker execution, concurrency, and result aggregation.
func (s *PartitionStep) Execute(ctx context.Context, jobExecution *model.JobExecution, controllerExecution *model.StepExecution) (err error) {
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
					errChan <- err
					s.tracer.RecordError(workerCtx, "partition_step", err)
					return
				}
				defer sem.Release(1)
			}

			// Execute the Worker Step using the StepExecutor.
			completedWorkerExec, execErr := s.stepExecutor.ExecuteStep(workerCtx, s.workerStep, jobExecution, workerExec)

			if execErr != nil {
				// Treat context.Canceled as CANCELLED instead of FAILED.
				if errors.Is(execErr, context.Canceled) {
					logger.Infof("PartitionStep '%s': Worker '%s' was cancelled.", s.id, workerExec.StepName)
					completedWorkerExec.Status = model.BatchStatusCancelled
					completedWorkerExec.ExitStatus = model.ExitStatusStopped // Set exit status to STOPPED for cancelled workers.
				} else {
					logger.Errorf("PartitionStep '%s': Worker '%s' failed: %v", s.id, workerExec.StepName, execErr)
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

// promoteExecutionContext handles the promotion of keys from StepExecutionContext to JobExecutionContext.
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
