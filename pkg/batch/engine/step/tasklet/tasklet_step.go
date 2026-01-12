package tasklet

import (
	"context"
	"database/sql"
	"time"

	port "github.com/tigerroll/surfin/pkg/batch/core/application/port"
	model "github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	metrics "github.com/tigerroll/surfin/pkg/batch/core/metrics"
	repository "github.com/tigerroll/surfin/pkg/batch/core/domain/repository"
	exception "github.com/tigerroll/surfin/pkg/batch/support/util/exception"
	logger "github.com/tigerroll/surfin/pkg/batch/support/util/logger"
)

// TaskletStep is an implementation of core.Step for Tasklet-oriented processing.
type TaskletStep struct {
	id                     string
	tasklet                port.Tasklet
	jobRepository          repository.JobRepository
	stepExecutionListeners []port.StepExecutionListener
	promotion              *model.ExecutionContextPromotion
	
	// T_TX_DEC: Addition of transaction attributes
	isolationLevel         sql.IsolationLevel
	propagation            string // Holds values like REQUIRED, REQUIRES_NEW as strings
	
	metricRecorder         metrics.MetricRecorder
	tracer                 metrics.Tracer
}

// NewTaskletStep creates a new TaskletStep instance.
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
	// Converts the isolation level specified in JSL to sql.IsolationLevel
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

// SetMetricRecorder implements port.Step.
func (s *TaskletStep) SetMetricRecorder(recorder metrics.MetricRecorder) {
	s.metricRecorder = recorder
}

// SetTracer implements port.Step.
func (s *TaskletStep) SetTracer(tracer metrics.Tracer) {
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
func (s *TaskletStep) ID() string {
	return s.id
}

// StepName returns the step name.
func (s *TaskletStep) StepName() string {
	return s.id
}

// GetTransactionOptions returns the transaction options for this step.
func (s *TaskletStep) GetTransactionOptions() *sql.TxOptions {
	// Propagation attribute is handled by the StepExecutor, so only IsolationLevel is set in TxOptions here.
	return &sql.TxOptions{
		Isolation: s.isolationLevel,
		ReadOnly:  false, // TaskletStep usually performs writes
	}
}

// GetPropagation returns the transaction propagation attribute.
func (s *TaskletStep) GetPropagation() string {
	return s.propagation
}

// notifyBeforeStep calls the BeforeStep method of registered StepExecutionListeners.
func (s *TaskletStep) notifyBeforeStep(ctx context.Context, stepExecution *model.StepExecution) {
	for _, l := range s.stepExecutionListeners {
		l.BeforeStep(ctx, stepExecution)
	}
}

// notifyAfterStep calls the AfterStep method of registered StepExecutionListeners.
func (s *TaskletStep) notifyAfterStep(ctx context.Context, stepExecution *model.StepExecution) {
	for _, l := range s.stepExecutionListeners {
		l.AfterStep(ctx, stepExecution)
	}
}

// Execute runs the Tasklet logic. Transaction management is handled by the StepExecutor.
func (s *TaskletStep) Execute(ctx context.Context, jobExecution *model.JobExecution, stepExecution *model.StepExecution) (err error) {
	logger.Infof("TaskletStep '%s' executing.", s.id)

	// 1. Update StepExecution status to STARTED
	stepExecution.MarkAsStarted()
	if err := s.jobRepository.UpdateStepExecution(ctx, stepExecution); err != nil {
		return exception.NewBatchError(s.id, "Failed to update StepExecution status to STARTED", err, false, false)
	}

	// 2. Set Tasklet Execution Context
	if err := s.tasklet.SetExecutionContext(ctx, stepExecution.ExecutionContext); err != nil {
		stepExecution.MarkAsFailed(err)
		s.jobRepository.UpdateStepExecution(ctx, stepExecution)
		return exception.NewBatchError(s.id, "Failed to set Tasklet ExecutionContext", err, false, false)
	}

	// 3. Listener notification (BeforeStep)
	s.notifyBeforeStep(ctx, stepExecution)

	// 4. Execute Tasklet (transaction management delegated to StepExecutor)
	var exitStatus model.ExitStatus
	
	// Execute Tasklet business logic
	exitStatus, err = s.tasklet.Execute(ctx, stepExecution)

	// 5. Retrieve Tasklet Execution Context and reflect it in StepExecution
	if taskletEC, getErr := s.tasklet.GetExecutionContext(ctx); getErr == nil {
		stepExecution.ExecutionContext = taskletEC
	} else {
		logger.Warnf("TaskletStep '%s': Failed to retrieve ExecutionContext from Tasklet: %v", s.id, getErr)
	}

	// 6. Close Tasklet
	if closeErr := s.tasklet.Close(ctx); closeErr != nil {
		logger.Errorf("TaskletStep '%s': Failed to close Tasklet: %v", s.id, closeErr)
		if err == nil {
			err = closeErr // If Tasklet execution succeeds but Close fails, treat it as an error
		}
	}

	// 7. Update StepExecution status and persist
	if err != nil {
		s.tracer.RecordError(ctx, s.id, err) // Tracing record
		stepExecution.MarkAsFailed(err)
	} else {
		stepExecution.Status = model.BatchStatusCompleted
		stepExecution.ExitStatus = exitStatus
		now := time.Now()
		stepExecution.EndTime = &now
		stepExecution.LastUpdated = now
	}

	// 9. Listener notification (AfterStep)
	s.notifyAfterStep(ctx, stepExecution)

	// 10. Persistence (executed within the StepExecutor's transaction)
	if updateErr := s.jobRepository.UpdateStepExecution(ctx, stepExecution); updateErr != nil {
		logger.Errorf("TaskletStep '%s': Failed to update final StepExecution state: %v", s.id, updateErr)
		if err == nil {
			err = updateErr // If Tasklet execution succeeds but persistence fails, treat it as an error
		}
	}

	logger.Infof("TaskletStep '%s' finished. ExitStatus: %s", s.id, stepExecution.ExitStatus)
	return err
}

// GetExecutionContextPromotion implements port.Step.
func (s *TaskletStep) GetExecutionContextPromotion() *model.ExecutionContextPromotion {
	return s.promotion
}

// Verify that TaskletStep implements the core.Step interface.
var _ port.Step = (*TaskletStep)(nil)
