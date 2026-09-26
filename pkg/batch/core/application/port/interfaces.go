// Package port defines the core interfaces (ports) for the batch application.
// These interfaces abstract the application's capabilities and dependencies,
// allowing for flexible implementation and testing.
package port

import (
	"context"
	"database/sql"
	"errors"
	model "github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	metrics "github.com/tigerroll/surfin/pkg/batch/core/metrics"
)

// Standard errors
var (
	// ErrNoMoreItems is returned when a reader has no more items to provide.
	ErrNoMoreItems = errors.New("no more items to read")

	// ErrExecutionContextNotSupported is returned when a component does not support getting or setting ExecutionContext.
	ErrExecutionContextNotSupported = errors.New("execution context not supported by this component")
)

// ItemStream defines the lifecycle for components that need to maintain state (checkpointing).
type ItemStream interface {
	// Open initializes the component and restores state from the provided ExecutionContext.
	Open(ctx context.Context, ec model.ExecutionContext) error
	// Update updates the state (checkpoint) after a chunk is committed.
	Update(ctx context.Context, ec model.ExecutionContext) error
	// Close releases resources used by the component.
	Close(ctx context.Context) error
}

// FlowElement represents a basic unit (Step, Decision, Split) in a job flow.
type FlowElement interface {
	// ID returns the unique identifier of the flow element.
	ID() string
}

// JobRunner is responsible for executing the entire flow of a Job.
type JobRunner interface {
	// Run starts the execution according to the job's flow definition.
	// This method is expected to run asynchronously.
	Run(ctx context.Context, jobInstance Job, jobExecution *model.JobExecution, flowDef *model.FlowDefinition)
}

// Job represents an executable batch job.
type Job interface {
	// Run executes the entire job flow.
	Run(ctx context.Context, jobExecution *model.JobExecution, jobParameters model.JobParameters) error
	// JobName returns the logical name of the job.
	JobName() string
	// ID returns the unique ID of the job definition.
	ID() string
	// GetFlow returns the job's flow definition structure.
	GetFlow() *model.FlowDefinition
	// ValidateParameters validates job parameters before job execution.
	ValidateParameters(params model.JobParameters) error
}

// Step represents a single unit of work executed within a job.
// It can be implemented as Chunk-oriented, Tasklet-oriented, or a Partitioning controller.
type Step interface {
	// Execute executes the business logic of the step.
	// Transaction boundaries are established by the StepExecutor.
	Execute(ctx context.Context, jobExecution *model.JobExecution, stepExecution *model.StepExecution) error
	// StepName returns the logical name of the step.
	StepName() string
	// ID returns the unique ID of the step definition.
	ID() string
	// GetTransactionOptions returns the transaction options (e.g., isolation level) for this step.
	GetTransactionOptions() *sql.TxOptions
	// GetPropagation returns the transaction propagation attribute (e.g., REQUIRED, REQUIRES_NEW, NESTED).
	GetPropagation() string
	// SetMetricRecorder sets the MetricRecorder for the step.
	SetMetricRecorder(recorder metrics.MetricRecorder)
	// SetTracer sets the Tracer for the step.
	SetTracer(tracer metrics.Tracer)
	// GetExecutionContextPromotion returns the ExecutionContext promotion settings for this step.
	GetExecutionContextPromotion() *model.ExecutionContextPromotion
}

// RemoteJobSubmitter delegates worker step execution to a remote environment.
type RemoteJobSubmitter interface {
	// SubmitWorkerJob submits the execution of the specified StepExecution to a remote environment.
	// Returns the ID or handle of the remote execution upon success.
	SubmitWorkerJob(ctx context.Context, jobExecution *model.JobExecution, stepExecution *model.StepExecution, workerStep Step) (string, error)
	// AwaitCompletion waits for the completion of a remotely submitted job and updates the StepExecution.
	AwaitCompletion(ctx context.Context, remoteJobID string, stepExecution *model.StepExecution) error
}

// StepExecutor abstracts the execution of a worker step, allowing transparent switching
// between local (Simple) and remote execution.
type StepExecutor interface {
	// ExecuteStep executes the specified Step and returns the completed StepExecution.
	// This method is responsible for establishing transaction boundaries.
	ExecuteStep(ctx context.Context, step Step, jobExecution *model.JobExecution, stepExecution *model.StepExecution) (*model.StepExecution, error)
}

// ExpressionResolver resolves dynamic expressions (e.g., #{jobParameters['key']})
// defined in JSL based on Job/Step Execution Context and Job Parameters.
type ExpressionResolver interface {
	// Resolve resolves expressions within the given string and returns the resulting string.
	Resolve(ctx context.Context, expression string, jobExecution *model.JobExecution, stepExecution *model.StepExecution) (string, error)
}

// ItemReader defines the contract for components that read data items.
// O is the type of item to be read.
type ItemReader[O any] interface {
	// Open opens resources and restores state from the provided [model.ExecutionContext].
	Open(ctx context.Context, ec model.ExecutionContext) error
	// Read reads the next item. Returns [ErrNoMoreItems] if no more items are available.
	Read(ctx context.Context) (O, error)
	// Close closes resources and saves state.
	Close(ctx context.Context) error
	// SetExecutionContext sets the state of the ItemReader.
	SetExecutionContext(ctx context.Context, ec model.ExecutionContext) error
	// GetExecutionContext retrieves the current state of the ItemReader.
	GetExecutionContext(ctx context.Context) (model.ExecutionContext, error)
}

// ItemProcessor defines the contract for components that process data items.
// I is the type of input item, O is the type of output item.
type ItemProcessor[I, O any] interface {
	// Process processes an input item and returns an output item.
	// Returns nil if the item is filtered during processing.
	Process(ctx context.Context, item I) (O, error)
	// SetExecutionContext sets the state of the ItemProcessor.
	SetExecutionContext(ctx context.Context, ec model.ExecutionContext) error
	// GetExecutionContext retrieves the current state of the ItemProcessor.
	GetExecutionContext(ctx context.Context) (model.ExecutionContext, error)
}

// ItemWriter defines the contract for components that write data items.
// I is the type of item to be written.
type ItemWriter[I any] interface {
	// Open opens resources and restores state from the provided [model.ExecutionContext].
	Open(ctx context.Context, ec model.ExecutionContext) error
	// Write persists a batch of items.
	Write(ctx context.Context, items []I) error
	// Close closes resources and saves state.
	Close(ctx context.Context) error
	// SetExecutionContext sets the state of the ItemWriter.
	SetExecutionContext(ctx context.Context, ec model.ExecutionContext) error
	// GetExecutionContext retrieves the current state of the ItemWriter.
	GetExecutionContext(ctx context.Context) (model.ExecutionContext, error)
	// GetTargetResourceName returns the name of the target resource (e.g., "workload_db", "s3_bucket").
	GetTargetResourceName() string
	// GetResourcePath returns the path or identifier within the target resource (e.g., "table_name", "s3_key_prefix").
	GetResourcePath() string
}

// IdempotentWriter is an interface for writers that guarantee idempotent write operations.
type IdempotentWriter[I any] interface {
	ItemWriter[I]
	// IsIdempotent returns true if the writer guarantees idempotency.
	IsIdempotent() bool
}

// ItemFlusher provides the capability to flush a write buffer.
type ItemFlusher interface {
	// Flush writes all currently buffered data to the underlying output destination.
	Flush(ctx context.Context) error
}

// Tasklet represents a step that performs a single operation, corresponding to JSR-352's Tasklet.
type Tasklet interface {
	// Open initializes the tasklet and prepares resources.
	Open(ctx context.Context, stepExecution *model.StepExecution) error
	// Execute executes the business logic of the Tasklet.
	Execute(ctx context.Context, stepExecution *model.StepExecution) (model.ExitStatus, error)
	// Close releases resources.
	Close(ctx context.Context, stepExecution *model.StepExecution) error
	// SetExecutionContext sets the ExecutionContext.
	SetExecutionContext(ec model.ExecutionContext)
	// GetExecutionContext retrieves the current ExecutionContext.
	GetExecutionContext() model.ExecutionContext
}

// NotificationListener is a listener for sending notifications after job execution completion.
type NotificationListener interface {
	// OnJobCompletion is called after a job completes (success, failure, stop, etc.).
	OnJobCompletion(ctx context.Context, jobExecution *model.JobExecution)
}

// RetryItemListener handles item-level retry events.
type RetryItemListener interface {
	// OnRetryRead is called before an item read is retried.
	OnRetryRead(ctx context.Context, err error)
	// OnRetryProcess is called before an item process is retried.
	OnRetryProcess(ctx context.Context, item interface{}, err error)
	// OnRetryWrite is called before an item write is retried.
	OnRetryWrite(ctx context.Context, items []interface{}, err error)
}

// SkipListener handles item skip events.
type SkipListener interface {
	// OnSkipRead is called after a skip occurs during reading.
	OnSkipRead(ctx context.Context, err error)
	// OnSkipProcess is called after a skip occurs during processing.
	OnSkipProcess(ctx context.Context, item interface{}, err error)
	// OnSkipWrite is called after a skip occurs during writing.
	OnSkipWrite(ctx context.Context, item interface{}, err error)
}

// StepExecutionListener handles step execution lifecycle events.
type StepExecutionListener interface {
	// BeforeStep is called just before a step execution starts.
	BeforeStep(ctx context.Context, stepExecution *model.StepExecution)
	// AfterStep is called after a step execution completes.
	AfterStep(ctx context.Context, stepExecution *model.StepExecution)
}

// ChunkListener handles chunk processing lifecycle events.
type ChunkListener interface {
	// BeforeChunk is called just before chunk processing begins.
	BeforeChunk(ctx context.Context, stepExecution *model.StepExecution)
	// AfterChunk is called after chunk processing completes.
	AfterChunk(ctx context.Context, stepExecution *model.StepExecution)
	// OnError is called if an error occurs during chunk processing.
	OnError(ctx context.Context, stepExecution *model.StepExecution, err error)
}

// JobExecutionListener handles job execution lifecycle events.
type JobExecutionListener interface {
	// BeforeJob is called just before a job execution starts.
	BeforeJob(ctx context.Context, jobExecution *model.JobExecution)
	// AfterJob is called after a job execution completes.
	AfterJob(ctx context.Context, jobExecution *model.JobExecution)
}

// ItemReadListener handles item read events.
type ItemReadListener interface {
	// BeforeRead is called before an item is read.
	BeforeRead(ctx context.Context)
	// AfterRead is called after an item is successfully read.
	AfterRead(ctx context.Context, item any)
	// OnReadError is called after an error occurs during item reading.
	OnReadError(ctx context.Context, err error)
}

// ItemProcessListener handles item process events.
type ItemProcessListener interface {
	// BeforeProcess is called before an item is processed.
	BeforeProcess(ctx context.Context, item any)
	// AfterProcess is called after an item is successfully processed.
	AfterProcess(ctx context.Context, item any, result any)
	// OnProcessError is called after an error occurs during item processing.
	OnProcessError(ctx context.Context, item interface{}, err error)
	// OnSkipInProcess is called after a skip occurs during processing.
	OnSkipInProcess(ctx context.Context, item interface{}, err error)
}

// ItemWriteListener handles item write events.
type ItemWriteListener interface {
	// BeforeWrite is called before items are written.
	BeforeWrite(ctx context.Context, items []any)
	// AfterWrite is called after items are successfully written.
	AfterWrite(ctx context.Context, items []any)
	// OnWriteError is called after an error occurs during item writing.
	OnWriteError(ctx context.Context, items []interface{}, err error)
	// OnSkipInWrite is called after a skip occurs during writing.
	OnSkipInWrite(ctx context.Context, item interface{}, err error)
}

// contextKey is a custom type for context keys to avoid collisions.
type contextKey string

// StepExecutionKey is the context key used to store and retrieve a [model.StepExecution].
const StepExecutionKey contextKey = "stepExecution"

// GetContextWithStepExecution stores a [model.StepExecution] in the provided [context.Context].
func GetContextWithStepExecution(ctx context.Context, se *model.StepExecution) context.Context {
	return context.WithValue(ctx, StepExecutionKey, se)
}

// GetStepExecutionFromContext retrieves a [model.StepExecution] from the [context.Context].
func GetStepExecutionFromContext(ctx context.Context) *model.StepExecution {
	if se, ok := ctx.Value(StepExecutionKey).(*model.StepExecution); ok {
		return se
	}
	return nil
}

// ItemListener is a composite interface that groups all item-level listener interfaces.
type ItemListener interface{}

// Decision defines a conditional branching point in the flow.
type Decision interface {
	// Decide determines the next transition based on the ExecutionContext and other parameters.
	Decide(ctx context.Context, jobExecution *model.JobExecution, jobParameters model.JobParameters) (model.ExitStatus, error)
	// DecisionName returns the logical name of the Decision.
	DecisionName() string
	// ID returns the unique ID of the Decision definition.
	ID() string
	// SetProperties sets properties injected from JSL.
	SetProperties(properties map[string]interface{})
}

// Split represents a flow element that executes multiple steps in parallel.
type Split interface {
	// Steps returns a list of Steps to be executed in parallel.
	Steps() []Step
	// ID returns the unique ID of the Split definition.
	ID() string
}

// JobParametersIncrementer generates the next JobParameters based on the current parameters.
type JobParametersIncrementer interface {
	// GetNext generates the next JobParameters.
	GetNext(params model.JobParameters) model.JobParameters
}

// Partitioner divides step execution into multiple partitions.
type Partitioner interface {
	// Partition returns a map of ExecutionContexts based on the specified grid size.
	Partition(ctx context.Context, gridSize int) (map[string]model.ExecutionContext, error)
}

// PartitionerBuilder is a function type for building a [Partitioner].
type PartitionerBuilder func(properties map[string]interface{}) (Partitioner, error)
