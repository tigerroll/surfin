// Package port defines the core interfaces (ports) for the batch application.
// These interfaces abstract the application's capabilities and dependencies,
// allowing for flexible implementation and testing.
package port

import (
	"context"
	"database/sql"
	"errors"
	"github.com/tigerroll/surfin/pkg/batch/core/adaptor"
	model "github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	metrics "github.com/tigerroll/surfin/pkg/batch/core/metrics"
	tx "github.com/tigerroll/surfin/pkg/batch/core/tx"
)

// Standard errors
var ErrNoMoreItems = errors.New("no more items to read")

// ErrExecutionContextNotSupported is returned when a component does not support getting or setting ExecutionContext.
var ErrExecutionContextNotSupported = errors.New("execution context not supported by this component")

// FlowElement is the basic interface representing an element (Step, Decision, Split) in a job flow.
// Each flow element has a unique ID.
type FlowElement interface {
	// ID returns the unique identifier of the flow element.
	//
	// Returns:
	//   string: The unique identifier of the flow element.
	ID() string
}

// JobRunner is the interface responsible for executing the entire flow of a Job.
type JobRunner interface {
	// Run starts the execution according to the job's flow definition. This method is expected to run asynchronously.
	//
	// Parameters:
	//   ctx: The context for the operation.
	//   jobInstance: The job instance to run.
	//   jobExecution: The current JobExecution instance.
	//   flowDef: The flow definition of the job.
	Run(ctx context.Context, jobInstance Job, jobExecution *model.JobExecution, flowDef *model.FlowDefinition)
}

// Job is the interface for an executable batch job.
// It is executed by a JobRunner and defines the entire job flow.
type Job interface {
	// Run executes the entire job flow.
	//
	// Parameters:
	//   ctx: The context for the operation.
	//   jobExecution: The current JobExecution instance.
	//   jobParameters: The job parameters for the execution.
	//
	// Returns:
	//   error: An error if the job execution fails.
	Run(ctx context.Context, jobExecution *model.JobExecution, jobParameters model.JobParameters) error
	// JobName returns the logical name of the job.
	//
	// Returns:
	//   string: The logical name of the job.
	JobName() string
	// ID returns the unique ID of the job definition.
	//
	// Returns:
	//   string: The unique identifier string for the job.
	ID() string
	// GetFlow returns the job's flow definition structure.
	//
	// Returns:
	//   *model.FlowDefinition: The flow definition of the job.
	GetFlow() *model.FlowDefinition
	// ValidateParameters validates job parameters before job execution.
	//
	// Parameters:
	//   params: The job parameters to validate.
	//
	// Returns:
	//   error: An error if validation fails.
	ValidateParameters(params model.JobParameters) error
}

// Step is the interface for a single step executed within a job.
// It is implemented as Chunk-oriented, Tasklet-oriented, or a Partitioning controller.
type Step interface {
	// Execute executes the business logic of the step.
	// Transaction boundaries are established by the StepExecutor.
	//
	// Parameters:
	//   ctx: The context for the operation.
	//   jobExecution: The current JobExecution instance.
	//   stepExecution: The current StepExecution instance.
	//
	// Returns:
	//   error: An error if the step execution encounters a fatal issue or exceeds retry/skip limits.
	Execute(ctx context.Context, jobExecution *model.JobExecution, stepExecution *model.StepExecution) error
	// StepName returns the logical name of the step.
	//
	// Returns:
	//   string: The logical name string for the step.
	StepName() string
	// ID returns the unique ID of the step definition.
	//
	// Returns:
	//   string: The unique identifier string for the step.
	ID() string
	// GetTransactionOptions returns the transaction options (e.g., isolation level) for this step.
	//
	// Returns:
	//   *sql.TxOptions: A pointer to sql.TxOptions configured with the step's isolation level.
	GetTransactionOptions() *sql.TxOptions
	// GetPropagation returns the transaction propagation attribute (e.g., REQUIRED, REQUIRES_NEW, NESTED).
	//
	// Returns:
	//   string: The propagation attribute as a string.
	GetPropagation() string

	// SetMetricRecorder sets the MetricRecorder.
	//
	// Parameters:
	//   recorder: The MetricRecorder instance to be used.
	SetMetricRecorder(recorder metrics.MetricRecorder)
	// SetTracer sets the Tracer.
	//
	// Parameters:
	//   tracer: The Tracer instance to be used.
	SetTracer(tracer metrics.Tracer)
	// GetExecutionContextPromotion returns the ExecutionContext promotion settings for this step.
	//
	// Returns:
	//   *model.ExecutionContextPromotion: A pointer to the ExecutionContextPromotion configuration.
	GetExecutionContextPromotion() *model.ExecutionContextPromotion
}

// RemoteJobSubmitter delegates worker step execution to a remote environment.
type RemoteJobSubmitter interface {
	// SubmitWorkerJob submits the execution of the specified StepExecution to a remote environment.
	// Upon successful submission, it returns the ID or handle of the remote execution.
	//
	// Parameters:
	//   ctx: The context for the operation.
	//   jobExecution: The current JobExecution instance.
	//   stepExecution: The StepExecution to submit.
	//   workerStep: The worker Step to execute remotely.
	//
	// Returns:
	//   string: The ID or handle of the remote execution.
	//   error: An error if submission fails.
	SubmitWorkerJob(ctx context.Context, jobExecution *model.JobExecution, stepExecution *model.StepExecution, workerStep Step) (string, error)

	// AwaitCompletion waits for the completion of a remotely submitted job and reflects the result in StepExecution.
	//
	// Parameters:
	//   ctx: The context for the operation.
	//   remoteJobID: The ID or handle of the remote job.
	//   stepExecution: The StepExecution to update with the result.
	//
	// Returns:
	//   error: An error if waiting or updating fails.
	AwaitCompletion(ctx context.Context, remoteJobID string, stepExecution *model.StepExecution) error
}

// StepExecutor is an interface that abstracts the execution of a worker step.
// It is used to transparently switch between local execution (Simple) or remote execution (Remote).
type StepExecutor interface {
	// ExecuteStep executes the specified Step and returns the completed StepExecution and any error.
	// This method is responsible for establishing the transaction boundaries for StepExecution.
	//
	// Parameters:
	//   ctx: The context for the operation.
	//   step: The Step to execute.
	//   jobExecution: The current JobExecution instance.
	//   stepExecution: The current StepExecution instance.
	//
	// Returns:
	//   *model.StepExecution: The completed StepExecution.
	//   error: An error if execution fails.
	ExecuteStep(ctx context.Context, step Step, jobExecution *model.JobExecution, stepExecution *model.StepExecution) (*model.StepExecution, error)
}

// ExpressionResolver is an interface for resolving dynamic expressions (e.g., #{jobParameters['key']})
// defined in JSL based on Job/Step Execution Context and Job Parameters.
type ExpressionResolver interface {
	// Resolve resolves expressions within the given string and returns the resulting string.
	//
	// Parameters:
	//   ctx: The context for the operation.
	//   expression: The string containing expressions to resolve (e.g., "#{jobParameters['key']}").
	//   jobExecution: The current JobExecution.
	//   stepExecution: The current StepExecution (may be nil if called from StepExecutor).
	//
	// Returns:
	//   string: The resolved string.
	//   error: An error if resolution fails.
	Resolve(ctx context.Context, expression string, jobExecution *model.JobExecution, stepExecution *model.StepExecution) (string, error)
}

// DBConnectionResolver resolves the required database connection name (data source name) based on the execution context.
type DBConnectionResolver interface {
	// ResolveDBConnectionName resolves the database connection name (e.g., "metadata", "workload") based on the execution context.
	//
	// Parameters:
	//   ctx: The context for the operation.
	//   jobExecution: The current JobExecution.
	//   stepExecution: The current StepExecution (may be nil for TaskletStep).
	//   defaultName: The default connection name if resolution fails.
	//
	// Returns:
	//   string: The resolved database connection name.
	//   error: An error if resolution fails.
	ResolveDBConnectionName(ctx context.Context, jobExecution *model.JobExecution, stepExecution *model.StepExecution, defaultName string) (string, error)

	// ResolveDBConnection resolves a database connection instance by name.
	// This method is responsible for ensuring that the returned connection is valid and re-establishes
	// the connection if necessary.
	//
	// Parameters:
	//   ctx: The context for the operation.
	//   name: The name of the database connection to resolve (e.g., "metadata", "workload").
	//
	// Returns:
	//   adaptor.DBConnection: The resolved database connection instance.
	//   error: An error that occurred during connection resolution or re-establishment.
	ResolveDBConnection(ctx context.Context, name string) (adaptor.DBConnection, error)
}

// ItemReader is the interface for a data reading step.
// O is the type of item to be read.
type ItemReader[O any] interface {
	// Open opens resources and restores state from ExecutionContext.
	//
	// Parameters:
	//   ctx: The context for the operation.
	//   ec: The ExecutionContext at the start of reading.
	//
	// Returns:
	//   error: An error if opening fails.
	Open(ctx context.Context, ec model.ExecutionContext) error
	// Read reads the next item. Returns ErrNoMoreItems if no more items are available.
	//
	// Parameters:
	//   ctx: The context for the operation.
	//
	// Returns:
	//   O: The next item.
	//   error: ErrNoMoreItems if no more items are available, or another error if reading fails.
	Read(ctx context.Context) (O, error)
	// Close closes resources and saves state.
	//
	// Parameters:
	//   ctx: The context for the operation.
	//
	// Returns:
	//   error: An error if closing fails.
	Close(ctx context.Context) error
	// SetExecutionContext sets the state of the ItemReader to the ExecutionContext.
	//
	// Parameters:
	//   ctx: The context for the operation.
	//   ec: The ExecutionContext to set.
	//
	// Returns:
	//   error: An error if setting fails.
	SetExecutionContext(ctx context.Context, ec model.ExecutionContext) error
	// GetExecutionContext retrieves the current state of the ItemReader as ExecutionContext.
	//
	// Parameters:
	//   ctx: The context for the operation.
	//
	// Returns:
	//   model.ExecutionContext: The current ExecutionContext.
	//   error: An error if retrieval fails.
	GetExecutionContext(ctx context.Context) (model.ExecutionContext, error)
}

// ItemProcessor is the interface for an item processing step.
// I is the type of input item, O is the type of output item.
type ItemProcessor[I, O any] interface {
	// Process processes an input item and returns an output item. Returns nil if the item is filtered during processing.
	//
	// Parameters:
	//   ctx: The context for the operation.
	//   item: The input item to be processed.
	//
	// Returns:
	//   O: The processed item, or nil if filtered.
	//   error: An error if processing fails.
	Process(ctx context.Context, item I) (O, error)
	// SetExecutionContext sets the state of the ItemProcessor to the ExecutionContext.
	//
	// Parameters:
	//   ctx: The context for the operation.
	//   ec: The ExecutionContext to set.
	//
	// Returns:
	//   error: An error if setting fails.
	SetExecutionContext(ctx context.Context, ec model.ExecutionContext) error
	// GetExecutionContext retrieves the current state of the ItemProcessor as ExecutionContext.
	//
	// Parameters:
	//   ctx: The context for the operation.
	//
	// Returns:
	//   model.ExecutionContext: The current ExecutionContext.
	//   error: An error if retrieval fails.
	GetExecutionContext(ctx context.Context) (model.ExecutionContext, error)
}

// ItemWriter is the interface for a data writing step.
// I is the type of item to be written.
type ItemWriter[I any] interface {
	// Open opens resources and restores state from ExecutionContext.
	//
	// Parameters:
	//   ctx: The context for the operation.
	//   ec: The ExecutionContext at the start of writing.
	//
	// Returns:
	//   error: An error if opening fails.
	Open(ctx context.Context, ec model.ExecutionContext) error
	// Write persists a list of items.
	//
	// Parameters:
	//   ctx: The context for the operation.
	//   tx: The current transaction.
	//   items: The list of items to be written.
	//
	// Returns:
	//   error: An error if writing fails.
	Write(ctx context.Context, tx tx.Tx, items []I) error
	// Close closes resources and saves state.
	//
	// Parameters:
	//   ctx: The context for the operation.
	//
	// Returns:
	//   error: An error if closing fails.
	Close(ctx context.Context) error
	// SetExecutionContext sets the state of the ItemWriter to the ExecutionContext.
	//
	// Parameters:
	//   ctx: The context for the operation.
	//   ec: The ExecutionContext to set.
	//
	// Returns:
	//   error: An error if setting fails.
	SetExecutionContext(ctx context.Context, ec model.ExecutionContext) error
	// GetExecutionContext retrieves the current state of the ItemWriter as ExecutionContext.
	//
	// Parameters:
	//   ctx: The context for the operation.
	//
	// Returns:
	//   model.ExecutionContext: The current ExecutionContext.
	//   error: An error if retrieval fails.
	GetExecutionContext(ctx context.Context) (model.ExecutionContext, error)

	// GetTargetDBName returns the name of the target database for this writer (e.g., "workload", "metadata").
	//
	// Returns:
	//   string: The name of the target database.
	GetTargetDBName() string
	// GetTableName returns the name of the target table for this writer.
	//
	// Returns:
	//   string: The name of the target table.
	GetTableName() string
}

// Tasklet is the interface for a step that performs a single operation.
// It corresponds to JSR352's Tasklet.
type Tasklet interface {
	// Execute executes the business logic of the Tasklet.
	// stepExecution: The current StepExecution.
	// Returns: An ExitStatus such as ExitStatus.COMPLETED upon success.
	Execute(ctx context.Context, stepExecution *model.StepExecution) (model.ExitStatus, error)
	// Close releases resources.
	Close(ctx context.Context) error
	// SetExecutionContext sets the ExecutionContext.
	SetExecutionContext(ctx context.Context, ec model.ExecutionContext) error
	// GetExecutionContext retrieves the ExecutionContext.
	GetExecutionContext(ctx context.Context) (model.ExecutionContext, error)
}

// NotificationListener is a dedicated listener for sending notifications after job execution completion.
type NotificationListener interface {
	// OnJobCompletion is called after a job completes (success, failure, stop, etc.).
	// jobExecution: Information about the completed JobExecution.
	OnJobCompletion(ctx context.Context, jobExecution *model.JobExecution)
}

// RetryItemListener is an interface for handling item-level retry events.
type RetryItemListener interface {
	// OnRetryRead is called before an item read is retried.
	OnRetryRead(ctx context.Context, err error)
	// OnRetryProcess is called before an item process is retried.
	// item: The item to be retried.
	// err: The error that occurred.
	OnRetryProcess(ctx context.Context, item interface{}, err error)
	// OnRetryWrite is called before an item write is retried.
	// items: The list of items to be retried.
	OnRetryWrite(ctx context.Context, items []interface{}, err error)
}

// SkipListener is an interface for handling item skip events.
type SkipListener interface {
	// OnSkipRead is called after a skip occurs during reading.
	OnSkipRead(ctx context.Context, err error)
	// OnSkipProcess is called after a skip occurs during processing.
	// item: The skipped item.
	// err: The error that occurred.
	OnSkipProcess(ctx context.Context, item interface{}, err error)
	// OnSkipWrite is called after a skip occurs during writing.
	// item: The skipped item.
	// err: The error that occurred.
	OnSkipWrite(ctx context.Context, item interface{}, err error)
}

// StepExecutionListener is an interface for handling step execution events.
type StepExecutionListener interface {
	// BeforeStep is called just before a step execution starts.
	BeforeStep(ctx context.Context, stepExecution *model.StepExecution)
	// AfterStep is called after a step execution completes (regardless of success or failure).
	AfterStep(ctx context.Context, stepExecution *model.StepExecution)
}

// ChunkListener is an interface for handling chunk processing events.
type ChunkListener interface {
	// BeforeChunk is called just before chunk processing (read, process, write) begins.
	// ctx: The context.
	// stepExecution: The current StepExecution.
	BeforeChunk(ctx context.Context, stepExecution *model.StepExecution)
	// AfterChunk is called after chunk processing completes (after commit or rollback).
	AfterChunk(ctx context.Context, stepExecution *model.StepExecution)
}

// JobExecutionListener is an interface for handling job execution events.
type JobExecutionListener interface {
	// BeforeJob is called just before a job execution starts.
	BeforeJob(ctx context.Context, jobExecution *model.JobExecution)
	// AfterJob is called after a job execution completes (regardless of success or failure).
	AfterJob(ctx context.Context, jobExecution *model.JobExecution)
}

// ItemReadListener is an interface for handling item read events.
type ItemReadListener interface {
	// OnReadError is called after an error occurs during item reading.
	// ctx: The context.
	// err: The error that occurred.
	OnReadError(ctx context.Context, err error)
}

// ItemProcessListener is an interface for handling item process events.
type ItemProcessListener interface {
	// OnProcessError is called after an error occurs during item processing.
	OnProcessError(ctx context.Context, item interface{}, err error)
	// OnSkipInProcess is called after a skip occurs during processing.
	// item: The skipped item.
	// err: The error that occurred.
	OnSkipInProcess(ctx context.Context, item interface{}, err error)
}

// ItemWriteListener is an interface for handling item write events.
type ItemWriteListener interface {
	// OnWriteError is called after an error occurs during item writing.
	// ctx: The context.
	// items: The list of items for which an error occurred during writing.
	OnWriteError(ctx context.Context, items []interface{}, err error)
	// OnSkipInWrite is called after a skip occurs during writing.
	OnSkipInWrite(ctx context.Context, item interface{}, err error)
}

// Define context key for StepExecution propagation during chunk processing.
type contextKey string

const StepExecutionKey contextKey = "stepExecution"

// GetContextWithStepExecution stores a StepExecution in the Context.
//
// Parameters:
//   ctx: The context for the operation.
//   se: The StepExecution to store.
//
// Returns:
//   context.Context: A new context with the StepExecution stored.
func GetContextWithStepExecution(ctx context.Context, se *model.StepExecution) context.Context {
	return context.WithValue(ctx, StepExecutionKey, se)
}

// GetStepExecutionFromContext retrieves a StepExecution from the Context. Returns nil if not found.
//
// Parameters:
//   ctx: The context for the operation.
//
// Returns:
//   *model.StepExecution: The retrieved StepExecution, or nil if not found.
func GetStepExecutionFromContext(ctx context.Context) *model.StepExecution {
	if se, ok := ctx.Value(StepExecutionKey).(*model.StepExecution); ok {
		return se
	}
	return nil
}

// ItemListener is a composite interface that groups all item-level listener interfaces.
type ItemListener interface{}

// Decision is an interface that defines a conditional branching point in the flow.
type Decision interface {
	// Decide determines the next transition based on the ExecutionContext and other parameters.
	//
	// Parameters:
	//   ctx: The context for the operation.
	//   jobExecution: The current JobExecution.
	//   jobParameters: The current JobParameters.
	//
	// Returns:
	//   model.ExitStatus: The determined ExitStatus for the next transition.
	//   error: An error if decision making fails.
	Decide(ctx context.Context, jobExecution *model.JobExecution, jobParameters model.JobParameters) (model.ExitStatus, error)
	// DecisionName returns the logical name of the Decision.
	//
	// Returns:
	//   string: The logical name of the Decision.
	DecisionName() string
	// ID returns the unique ID of the Decision definition.
	//
	// Returns:
	//   string: The unique identifier string for the Decision.
	ID() string
	// SetProperties sets properties injected from JSL.
	//
	// Parameters:
	//   properties: A map of properties to set.
	SetProperties(properties map[string]string)
}

// Split is the interface for a flow element that executes multiple steps in parallel.
type Split interface {
	// Steps returns a list of Steps to be executed in parallel.
	// A Split contains multiple Steps internally and executes them in parallel.
	//
	// Returns:
	//   []Step: A slice of Steps to be executed in parallel.
	Steps() []Step
	// ID returns the unique ID of the Split definition.
	//
	// Returns:
	//   string: The unique identifier string for the Split.
	ID() string
}

// JobParametersIncrementer is an interface for automatically incrementing JobParameters.
type JobParametersIncrementer interface {
	// GetNext generates the next JobParameters based on the current parameters.
	//
	// Parameters:
	//   params: The current JobParameters.
	//
	// Returns:
	//   model.JobParameters: The next JobParameters.
	GetNext(params model.JobParameters) model.JobParameters
}

// Partitioner divides step execution into multiple partitions.
type Partitioner interface {
	// Partition returns a map of ExecutionContexts based on the specified grid size.
	// Each ExecutionContext is passed to a separate worker (StepExecution).
	//
	// Parameters:
	//   ctx: The context.
	//   gridSize: The number of partitions to create.
	//
	// Returns:
	//   map[string]model.ExecutionContext: A map of ExecutionContexts, where keys are partition names.
	//   error: An error if partitioning fails.
	Partition(ctx context.Context, gridSize int) (map[string]model.ExecutionContext, error)
}

// PartitionerBuilder is a function type for building core.Partitioner.
//
// Parameters:
//   properties: A map of properties for building the Partitioner.
//
// Returns:
//   Partitioner: The built Partitioner instance.
//   error: An error if building fails.
type PartitionerBuilder func(properties map[string]string) (Partitioner, error)
