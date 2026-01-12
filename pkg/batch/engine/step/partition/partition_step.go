package partition

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"

	"github.com/google/uuid"
	port "github.com/tigerroll/surfin/pkg/batch/core/application/port"
	model "github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	repository "github.com/tigerroll/surfin/pkg/batch/core/domain/repository"
	exception "github.com/tigerroll/surfin/pkg/batch/support/util/exception"
	logger "github.com/tigerroll/surfin/pkg/batch/support/util/logger"
	metrics "github.com/tigerroll/surfin/pkg/batch/core/metrics"
)

// PartitionStep is an implementation of core.Step that executes a worker step in parallel partitions.
// This acts as the Controller Step.
type PartitionStep struct {
	id                     string
	partitioner            port.Partitioner
	workerStep             port.Step
	gridSize               int
	jobRepository          repository.JobRepository
	stepExecutionListeners []port.StepExecutionListener
	promotion              *model.ExecutionContextPromotion
	stepExecutor           port.StepExecutor // SimpleStepExecutor or RemoteStepExecutor
	// Fields to satisfy Step interface requirements (PartitionStep itself usually doesn't record, but holds to pass to Worker)
	metricRecorder         metrics.MetricRecorder
	tracer                 metrics.Tracer
}

// NewPartitionStep creates a new PartitionStep instance.
func NewPartitionStep(
	id string,
	partitioner port.Partitioner,
	workerStep port.Step,
	gridSize int,
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
		jobRepository:          jobRepository,
		stepExecutionListeners: stepExecutionListeners,
		promotion:              promotion,
		stepExecutor:           stepExecutor,
	}
}

// SetMetricRecorder implements port.Step.
func (s *PartitionStep) SetMetricRecorder(recorder metrics.MetricRecorder) {
	s.metricRecorder = recorder
}

// SetTracer implements port.Step.
func (s *PartitionStep) SetTracer(tracer metrics.Tracer) {
	s.tracer = tracer
}

// GetExecutionContextPromotion implements port.Step.
// PartitionStep does not have its own ExecutionContextPromotion, so it returns nil.
func (s *PartitionStep) GetExecutionContextPromotion() *model.ExecutionContextPromotion {
	return nil
}

// ID returns the step ID.
func (s *PartitionStep) ID() string {
	return s.id
}

// StepName returns the step name.
func (s *PartitionStep) StepName() string {
	return s.id
}

// GetTransactionOptions returns the transaction options for this step.
// PartitionStep is a controller step and does not require a transaction boundary for its execution.
func (s *PartitionStep) GetTransactionOptions() *sql.TxOptions {
	// PartitionStep is a controller and does not require a transaction for itself, so it returns nil.
	return nil
}

// GetPropagation returns the transaction propagation attribute.
// PartitionStep is a controller step and does not require a transaction boundary for its execution.
func (s *PartitionStep) GetPropagation() string {
	// JSL does not define a propagation attribute for Partition Step itself, so it returns an empty string.
	return "" 
}

// notifyBeforeStep calls the BeforeStep method of registered StepExecutionListeners.
func (s *PartitionStep) notifyBeforeStep(ctx context.Context, stepExecution *model.StepExecution) {
	for _, l := range s.stepExecutionListeners {
		l.BeforeStep(ctx, stepExecution)
	}
}

// notifyAfterStep calls the AfterStep method of registered StepExecutionListeners.
func (s *PartitionStep) notifyAfterStep(ctx context.Context, stepExecution *model.StepExecution) {
	for _, l := range s.stepExecutionListeners {
		l.AfterStep(ctx, stepExecution)
	}
}

// Execute runs the partitioning logic.
func (s *PartitionStep) Execute(ctx context.Context, jobExecution *model.JobExecution, controllerExecution *model.StepExecution) (err error) {
	logger.Infof("PartitionStep '%s' executing (GridSize: %d).", s.id, s.gridSize)

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
	workerExecutions := make(chan *model.StepExecution, len(partitionContexts))

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

		go func(workerExec *model.StepExecution) {
			defer wg.Done()
			
			// Execute the Worker Step using the StepExecutor.
			completedWorkerExec, execErr := s.stepExecutor.ExecuteStep(ctx, s.workerStep, jobExecution, workerExec)
			
			if execErr != nil {
				logger.Errorf("PartitionStep '%s': Worker '%s' failed: %v", s.id, workerExec.StepName, execErr)
				errChan <- execErr
			} else {
				logger.Infof("PartitionStep '%s': Worker '%s' completed with status: %s", s.id, workerExec.StepName, completedWorkerExec.Status)
			}
			
			// Send the completed Worker StepExecution for aggregation.
			workerExecutions <- completedWorkerExec
		}(workerExecution)
	}

	wg.Wait()
	close(errChan)
	close(workerExecutions)

	// 5. Aggregate results.
	var combinedError error
	for err := range errChan {
		combinedError = errors.Join(combinedError, err)
	}
	
	// Aggregate Worker Execution Context (Promotion should be done on Controller Execution Context, but here we aggregate Worker statistics).
	totalRead := 0
	totalWrite := 0
	
	// Track aggregated ExitStatus.
	var aggregatedExitStatus model.ExitStatus = model.ExitStatusCompleted
	
	// Initialize map to store aggregated Worker EC.
	aggregatedEC := model.NewExecutionContext()
	
	for workerExec := range workerExecutions {
		totalRead += workerExec.ReadCount
		totalWrite += workerExec.WriteCount
		
		// Logic to reflect Worker's ExitStatus in Controller's ExitStatus.
		if workerExec.Status == model.BatchStatusFailed || workerExec.Status == model.BatchStatusAbandoned {
			// If there is a failure or abandonment, the Controller also fails.
			aggregatedExitStatus = model.ExitStatusFailed
			// Aggregate failure messages.
			controllerExecution.Failures = append(controllerExecution.Failures, workerExec.Failures...)
			if combinedError == nil {
				combinedError = fmt.Errorf("one or more partitions failed")
			}
		} else if workerExec.Status == model.BatchStatusStopped {
			// If there is a stop, and no failure, then it's a stop.
			if aggregatedExitStatus != model.ExitStatusFailed {
				aggregatedExitStatus = model.ExitStatusStopped
			}
		}
		// Do nothing if COMPLETED (initial value is COMPLETED).

		// Merge Worker Execution Context content into Controller EC.
		// NOTE: If keys duplicate, the value from the later Worker will overwrite.
		for k, v := range workerExec.ExecutionContext {
			aggregatedEC[k] = v
		}
	}
	
	// Save aggregated results to Controller Execution Context.
	controllerExecution.ReadCount = totalRead
	controllerExecution.WriteCount = totalWrite
	
	// Set aggregated EC to Controller Execution.
	controllerExecution.ExecutionContext = aggregatedEC
	
	// 6. Update Controller StepExecution status.
	if combinedError != nil {
		// If there was a worker failure, wrap the aggregated error and return it.
		wrappedErr := exception.NewBatchError(s.id, "one or more partitions failed", combinedError, false, false)
		controllerExecution.MarkAsFailed(wrappedErr)
		combinedError = wrappedErr // Update the error to be returned finally.
	} else {
		// If no failure, update status based on aggregated ExitStatus.
		switch aggregatedExitStatus {
		case model.ExitStatusStopped:
			controllerExecution.MarkAsStopped()
		case model.ExitStatusCompleted:
			controllerExecution.MarkAsCompleted()
		case model.ExitStatusFailed:
			// If aggregatedExitStatus is FAILED, it should have already been handled in the combinedError != nil block, but just in case.
			// If combinedError is nil but ExitStatus is FAILED (e.g., Worker returned ExitStatus FAILED without an error).
			controllerExecution.MarkAsFailed(fmt.Errorf("partition execution resulted in aggregated status: %s", aggregatedExitStatus))
		default:
			// For other states (e.g., UNKNOWN), treat as failure.
			controllerExecution.MarkAsFailed(fmt.Errorf("partition execution resulted in unexpected aggregated status: %s", aggregatedExitStatus))
		}
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
