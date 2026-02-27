// Package tasklet provides the TaskletStep implementation for the batch engine.
package tasklet

import (
	"context"
	"database/sql"
	"time"

	port "github.com/tigerroll/surfin/pkg/batch/core/application/port"
	model "github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	repository "github.com/tigerroll/surfin/pkg/batch/core/domain/repository"
	metrics "github.com/tigerroll/surfin/pkg/batch/core/metrics"
	exception "github.com/tigerroll/surfin/pkg/batch/support/util/exception"
	logger "github.com/tigerroll/surfin/pkg/batch/support/util/logger"
)

// TaskletStep is an implementation of core.Step for Tasklet-oriented processing.
// It wraps a port.Tasklet and manages its lifecycle within a batch step execution,
// including status updates, listener notifications, and transaction options.
type TaskletStep struct {
	id                     string
	tasklet                port.Tasklet
	jobRepository          repository.JobRepository
	stepExecutionListeners []port.StepExecutionListener
	promotion              *model.ExecutionContextPromotion

	// Transaction attributes.
	isolationLevel sql.IsolationLevel
	propagation    string // Holds values like REQUIRED, REQUIRES_NEW as strings.

	metricRecorder metrics.MetricRecorder
	tracer         metrics.Tracer
}

// NewTaskletStep creates a new TaskletStep instance.
//
// Parameters:
//
//	id: The unique identifier for this step.
//	tasklet: The underlying port.Tasklet to be executed.
//	jobRepository: The repository for persisting job and step execution data.
//	stepExecutionListeners: A slice of listeners to be notified during step execution.
//	promotion: Configuration for promoting execution context.
//	isolationLevel: The transaction isolation level as a string (e.g., "READ_COMMITTED").
//	propagation: The transaction propagation behavior as a string (e.g., "REQUIRED").
//	metricRecorder: The metric recorder for capturing step metrics.
//	tracer: The tracer for distributed tracing.
//
// Returns:
//
//	A new port.Step instance.
func NewTaskletStep(
	id string,
	tasklet port.Tasklet,
	jobRepository repository.JobRepository,
	stepExecutionListeners []port.StepExecutionListener,
	promotion *model.ExecutionContextPromotion,
	isolationLevel string,
	propagation string,
	metricRecorder metrics.MetricRecorder,
	tracer metrics.Tracer,
) port.Step {
	// Converts the isolation level specified in JSL to sql.IsolationLevel.
	isoLevel := parseIsolationLevel(isolationLevel)

	return &TaskletStep{
		id:                     id,
		tasklet:                tasklet,
		jobRepository:          jobRepository,
		stepExecutionListeners: stepExecutionListeners,
		promotion:              promotion,
		isolationLevel:         isoLevel,
		propagation:            propagation,
		metricRecorder:         metricRecorder,
		tracer:                 tracer,
	}
}

// SetMetricRecorder sets the MetricRecorder for the step.
// This method implements the port.Step interface.
//
// Parameters:
//
//	recorder: The MetricRecorder instance to be used.
func (s *TaskletStep) SetMetricRecorder(recorder metrics.MetricRecorder) {
	s.metricRecorder = recorder
}

// SetTracer sets the Tracer for the step.
// This method implements the port.Step interface.
//
// Parameters:
//
//	tracer: The Tracer instance to be used.
func (s *TaskletStep) SetTracer(tracer metrics.Tracer) {
	s.tracer = tracer
}

// parseIsolationLevel converts a JSL string representation of an isolation level to sql.IsolationLevel.
//
// Parameters:
//
//	level: The string representation of the isolation level.
//
// Returns:
//
//	sql.IsolationLevel: The corresponding SQL isolation level. Defaults to sql.LevelDefault if unrecognized.
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
		// Default depends on framework configuration or database default.
		return sql.LevelDefault
	}
}

// ID returns the unique identifier of the step.
// This method implements the port.Step interface.
//
// Returns:
//
//	string: The unique identifier of the step.
func (s *TaskletStep) ID() string {
	return s.id
}

// StepName returns the logical name of the step.
// For TaskletStep, the step name is the same as its ID.
// This method implements the port.Step interface.
//
// Returns:
//
//	string: The logical name of the step.
func (s *TaskletStep) StepName() string {
	return s.id
}

// GetTransactionOptions returns the transaction options for this step.
// This method implements the port.Step interface.
//
// Returns:
//
//	*sql.TxOptions: A pointer to sql.TxOptions configured with the step's isolation level.
func (s *TaskletStep) GetTransactionOptions() *sql.TxOptions {
	// Propagation attribute is handled by the StepExecutor, so only IsolationLevel is set in TxOptions here.
	return &sql.TxOptions{
		Isolation: s.isolationLevel,
		ReadOnly:  false, // TaskletStep usually performs writes.
	}
}

// GetPropagation returns the transaction propagation attribute.
// This method implements the port.Step interface.
//
// Returns:
//
//	string: The propagation attribute as a string (e.g., "REQUIRED", "REQUIRES_NEW").
func (s *TaskletStep) GetPropagation() string {
	return s.propagation
}

// notifyBeforeStep calls the BeforeStep method of all registered StepExecutionListeners.
//
// Parameters:
//
//	ctx: The context for the operation.
//	stepExecution: The current StepExecution instance.
func (s *TaskletStep) notifyBeforeStep(ctx context.Context, stepExecution *model.StepExecution) {
	for _, l := range s.stepExecutionListeners {
		l.BeforeStep(ctx, stepExecution)
	}
}

// notifyAfterStep calls the AfterStep method of all registered StepExecutionListeners.
//
// Parameters:
//
//	ctx: The context for the operation.
//	stepExecution: The current StepExecution instance.
func (s *TaskletStep) notifyAfterStep(ctx context.Context, stepExecution *model.StepExecution) {
	for _, l := range s.stepExecutionListeners {
		l.AfterStep(ctx, stepExecution)
	}
}

// Execute runs the Tasklet logic. This method manages the lifecycle of the tasklet
// within the step, including status updates, execution context handling, and listener notifications.
// Transaction management is typically handled by the StepExecutor that calls this method.
//
// Parameters:
//
//	ctx: The context for the operation.
//	jobExecution: The current JobExecution instance.
//	stepExecution: The current StepExecution instance.
//
// Returns:
//
//	error: An error if the step execution encounters a fatal issue.
func (s *TaskletStep) Execute(ctx context.Context, jobExecution *model.JobExecution, stepExecution *model.StepExecution) (err error) {
	logger.Infof("TaskletStep '%s' executing.", s.id)

	// 1. Update StepExecution status to STARTED.
	stepExecution.MarkAsStarted()
	if err := s.jobRepository.UpdateStepExecution(ctx, stepExecution); err != nil {
		return exception.NewBatchError(s.id, "Failed to update StepExecution status to STARTED", err, false, false)
	}

	// 2. Set Tasklet Execution Context.
	// The ExecutionContext from stepExecution is passed to the tasklet.
	if err := s.tasklet.SetExecutionContext(ctx, stepExecution.ExecutionContext); err != nil {
		stepExecution.MarkAsFailed(err)
		s.jobRepository.UpdateStepExecution(ctx, stepExecution)
		return exception.NewBatchError(s.id, "Failed to set Tasklet ExecutionContext", err, false, false)
	}

	// 3. Open the tasklet to initialize resources.
	// This is called once at the beginning of the step.
	if err := s.tasklet.Open(ctx, stepExecution.ExecutionContext); err != nil {
		stepExecution.MarkAsFailed(err)                         // Mark step as failed if Open fails.
		s.jobRepository.UpdateStepExecution(ctx, stepExecution) // Persist failure.
		return exception.NewBatchError(s.id, "Failed to open Tasklet", err, false, false)
	}
	// Ensure the tasklet is closed when the step execution finishes, regardless of success or failure.
	defer func() {
		if closeErr := s.tasklet.Close(ctx); closeErr != nil {
			logger.Errorf("TaskletStep '%s': Failed to close Tasklet: %v", s.id, closeErr)
			// If Close fails, and no other error has occurred, this error might be propagated.
			// However, the primary error should be from Execute if it failed.
			// This defer is mainly for resource cleanup.
		}
	}()

	// 4. Listener notification (BeforeStep).
	s.notifyBeforeStep(ctx, stepExecution)

	// 5. Execute Tasklet's business logic.
	var exitStatus model.ExitStatus
	exitStatus, err = s.tasklet.Execute(ctx, stepExecution)

	// 6. Retrieve Tasklet Execution Context and reflect it in StepExecution.
	// This ensures any changes made by the tasklet to its context are saved.
	if taskletEC, getErr := s.tasklet.GetExecutionContext(ctx); getErr == nil {
		stepExecution.ExecutionContext = taskletEC
	} else {
		logger.Warnf("TaskletStep '%s': Failed to retrieve ExecutionContext from Tasklet: %v", s.id, getErr)
	}

	// 7. Update StepExecution status based on the outcome of tasklet.Execute.
	if err != nil {
		s.tracer.RecordError(ctx, s.id, err) // Record error for tracing.
		stepExecution.MarkAsFailed(err)
	} else {
		stepExecution.Status = model.BatchStatusCompleted
		stepExecution.ExitStatus = exitStatus
		now := time.Now()
		stepExecution.EndTime = &now
		stepExecution.LastUpdated = now
	}

	// 8. Listener notification (AfterStep).
	s.notifyAfterStep(ctx, stepExecution)

	// 9. Persistence (executed within the StepExecutor's transaction).
	// This updates the final state of the StepExecution in the repository.
	if updateErr := s.jobRepository.UpdateStepExecution(ctx, stepExecution); updateErr != nil {
		logger.Errorf("TaskletStep '%s': Failed to update final StepExecution state: %v", s.id, updateErr)
		if err == nil {
			err = updateErr // If Tasklet execution succeeds but persistence fails, treat it as an error.
		}
	}

	logger.Infof("TaskletStep '%s' finished. ExitStatus: %s", s.id, stepExecution.ExitStatus)
	return err
}

// GetExecutionContextPromotion returns the ExecutionContext promotion settings for this step.
// This method implements the port.Step interface.
//
// Returns:
//
//	*model.ExecutionContextPromotion: A pointer to the ExecutionContextPromotion configuration.
func (s *TaskletStep) GetExecutionContextPromotion() *model.ExecutionContextPromotion {
	return s.promotion
}

// Verify that TaskletStep implements the core.Step interface.
var _ port.Step = (*TaskletStep)(nil)
