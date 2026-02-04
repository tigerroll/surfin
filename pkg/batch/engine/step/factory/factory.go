package factory

import (
	"github.com/tigerroll/surfin/pkg/batch/core/adapter"
	port "github.com/tigerroll/surfin/pkg/batch/core/application/port"
	config "github.com/tigerroll/surfin/pkg/batch/core/config"
	model "github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	repository "github.com/tigerroll/surfin/pkg/batch/core/domain/repository"
	metrics "github.com/tigerroll/surfin/pkg/batch/core/metrics"
	tx "github.com/tigerroll/surfin/pkg/batch/core/tx"
	itemstep "github.com/tigerroll/surfin/pkg/batch/engine/step/item"
	partitionstep "github.com/tigerroll/surfin/pkg/batch/engine/step/partition"
	taskletstep "github.com/tigerroll/surfin/pkg/batch/engine/step/tasklet"
	logger "github.com/tigerroll/surfin/pkg/batch/support/util/logger"
	"go.uber.org/fx"
)

// StepFactory is an interface for creating `port.Step` instances.
//
// This factory is responsible for converting step definitions defined in JSL (Job Specification Language)
// into concrete, executable Step objects. It abstracts the complexity of
// instantiating different types of steps (Chunk, Tasklet, Partition) and
// injecting their required dependencies.
type StepFactory interface {
	// CreateChunkStep constructs a chunk-oriented Step.
	// Each argument represents the properties of the chunk step defined in JSL, along with its associated components (Reader, Processor, Writer) and listeners.
	//
	// Parameters:
	//   name: The unique identifier name of the step.
	//   reader: An implementation of the `port.ItemReader` interface for reading items.
	//   processor: An implementation of the `port.ItemProcessor` interface for processing items.
	//   writer: An implementation of the `port.ItemWriter` interface for writing items.
	//   chunkSize: The maximum number of items to process in a single chunk.
	//   commitInterval: The interval at which transactions are committed (typically the same as `chunkSize`).
	//   retryConfig: Step-level retry configuration.
	//   itemRetryConfig: Item-level retry configuration.
	//   itemSkipConfig: Item-level skip configuration.
	//   stepExecutionListeners: A list of `port.StepExecutionListener` to apply to this step.
	//   itemReadListeners: A list of `port.ItemReadListener` to apply to this step.
	//   itemProcessListeners: A list of `port.ItemProcessListener` to apply to this step.
	//   itemWriteListeners: A list of `port.ItemWriteListener` to apply to this step.
	//   skipListeners: A list of `port.SkipListener` to apply to this step.
	//   retryItemListeners: A list of `port.RetryItemListener` to apply to this step.
	//   chunkListeners: A list of `port.ChunkListener` to apply to this step.
	//   promotion: Promotion settings from `StepExecutionContext` to `JobExecutionContext`.
	//   isolationLevel: The transaction isolation level for this step (e.g., "SERIALIZABLE").
	//   propagation: The transaction propagation attribute for this step (e.g., "REQUIRED", "REQUIRES_NEW").
	//
	// Returns:
	//   `port.Step`: The constructed chunk-oriented step.
	//   error: An error if step creation fails.
	CreateChunkStep(
		name string,
		reader port.ItemReader[any],
		processor port.ItemProcessor[any, any],
		writer port.ItemWriter[any],
		chunkSize int,
		commitInterval int,
		retryConfig *config.RetryConfig,
		itemRetryConfig config.ItemRetryConfig,
		itemSkipConfig config.ItemSkipConfig,
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
	) (port.Step, error)

	// CreateTaskletStep constructs a tasklet-oriented Step.
	// Each argument represents the properties of the tasklet step defined in JSL, along with its associated component (Tasklet) and listeners.
	//
	// Parameters:
	//   name: The unique identifier name of the step.
	//   tasklet: An implementation of the `port.Tasklet` interface to be executed.
	//   stepExecutionListeners: A list of `port.StepExecutionListener` to apply to this step.
	//   promotion: Promotion settings from `StepExecutionContext` to `JobExecutionContext`.
	//   isolationLevel: The transaction isolation level for this step (e.g., "SERIALIZABLE").
	//   propagation: The transaction propagation attribute for this step (e.g., "REQUIRED", "REQUIRES_NEW").
	//
	// Returns:
	//   `port.Step`: The constructed tasklet-oriented step.
	//   error: An error if step creation fails.
	CreateTaskletStep(
		name string,
		tasklet port.Tasklet,
		stepExecutionListeners []port.StepExecutionListener,
		promotion *model.ExecutionContextPromotion,
		isolationLevel string,
		propagation string,
	) (port.Step, error)

	// CreatePartitionStep constructs a partition-oriented Step (controller step).
	// This step is used to distribute processing across multiple worker steps.
	//
	// Parameters:
	//   name: The unique identifier name of the step.
	//   partitioner: An implementation of the `port.Partitioner` interface for generating partitions.
	//   workerStep: An implementation of the `port.Step` interface to be executed in each partition.
	//               This represents the actual work unit that will be distributed.
	//   gridSize: The number of partitions to generate (or the maximum number if the partitioner generates fewer).
	//   jobRepository: An implementation of the `repository.JobRepository` interface for persisting job metadata.
	//   stepExecutionListeners: A list of `port.StepExecutionListener` to apply to this step.
	//   promotion: Promotion settings from `StepExecutionContext` to `JobExecutionContext`.
	//
	// Returns:
	//   `port.Step`: The constructed partition-oriented controller step.
	//   error: An error if step creation fails.
	CreatePartitionStep(
		name string,
		partitioner port.Partitioner,
		workerStep port.Step,
		gridSize int,
		jobRepository repository.JobRepository,
		stepExecutionListeners []port.StepExecutionListener,
		promotion *model.ExecutionContextPromotion,
	) (port.Step, error)
}

// DefaultStepFactory is the default implementation of the `StepFactory` interface.
// It manages the dependencies required for constructing various types of `port.Step` instances.
type DefaultStepFactory struct {
	jobRepository        repository.JobRepository
	stepExecutor         port.StepExecutor
	metricRecorder       metrics.MetricRecorder
	tracer               metrics.Tracer
	dbConnectionResolver adapter.DBConnectionResolver // `dbConnectionResolver` is used to resolve database connections for steps.
	txManagerFactory     tx.TransactionManagerFactory // `txManagerFactory` is the transaction manager factory for creating transaction managers.
}

// DefaultStepFactoryParams defines the parameters that the `NewDefaultStepFactory` function
// receives via dependency injection (Fx).
type DefaultStepFactoryParams struct {
	fx.In
	JobRepository     repository.JobRepository
	MetadataTxManager tx.TransactionManager `name:"metadata"` // Requests the metadata TxManager (used by JobRepository).
	StepExecutor      port.StepExecutor
	MetricRecorder    metrics.MetricRecorder
	Tracer            metrics.Tracer
	DBResolver        adapter.DBConnectionResolver // `DBResolver` is the database connection resolver.
	TxFactory         tx.TransactionManagerFactory // `TxFactory` is the transaction manager factory.
}

// NewDefaultStepFactory creates a new instance of `DefaultStepFactory`.
//
// Parameters:
//
//	p: The `DefaultStepFactoryParams` struct containing injected dependencies.
//
// Returns: A pointer to the initialized `DefaultStepFactory`.
func NewDefaultStepFactory(
	p DefaultStepFactoryParams,
) *DefaultStepFactory {
	return &DefaultStepFactory{
		jobRepository:        p.JobRepository,
		stepExecutor:         p.StepExecutor,
		metricRecorder:       p.MetricRecorder,
		tracer:               p.Tracer,
		dbConnectionResolver: p.DBResolver, // Injected DBConnectionResolver
		txManagerFactory:     p.TxFactory,  // Injected TransactionManagerFactory
	}
}

// CreateChunkStep constructs a new `ChunkStep` instance.
// This method is part of the `StepFactory` interface implementation.
// Parameters:
//
//	name: The unique identifier name of the step.
//	reader: An implementation of the ItemReader interface for reading items.
//	processor: An implementation of the ItemProcessor interface for processing items.
//	writer: An implementation of the ItemWriter interface for writing items.
//	chunkSize: The maximum number of items to process at once.
//	commitInterval: The interval at which transactions are committed (usually the same as chunkSize).
//	retryConfig: Step-level retry configuration.
//	itemRetryConfig: Item-level retry configuration.
//	itemSkipConfig: Item-level skip configuration.
//	stepExecutionListeners: A list of StepExecutionListeners to apply to this step.
//	itemReadListeners: A list of ItemReadListeners to apply to this step.
//	itemProcessListeners: A list of ItemProcessListeners to apply to this step.
//	itemWriteListeners: A list of ItemWriteListeners to apply to this step.
//	skipListeners: A list of SkipListeners to apply to this step.
//	retryItemListeners: A list of RetryItemListeners to apply to this step.
//	chunkListeners: A list of ChunkListeners to apply to this step.
//	promotion: Promotion settings from StepExecutionContext to JobExecutionContext.
//	isolationLevel: The transaction isolation level for this step.
//	propagation: The transaction propagation attribute for this step.
//
// Returns:
//
//	`port.Step`: The constructed chunk-oriented step.
//	error: An error if step creation fails.
func (f *DefaultStepFactory) CreateChunkStep(
	name string,
	reader port.ItemReader[any],
	processor port.ItemProcessor[any, any],
	writer port.ItemWriter[any],
	chunkSize int,
	commitInterval int,
	retryConfig *config.RetryConfig,
	itemRetryConfig config.ItemRetryConfig,
	itemSkipConfig config.ItemSkipConfig,
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
) (port.Step, error) {
	step := itemstep.NewJSLAdaptedStep(
		name,
		reader,
		processor,
		writer,
		chunkSize,
		commitInterval,
		retryConfig,
		itemRetryConfig,
		itemSkipConfig,
		f.jobRepository,
		stepExecutionListeners,
		itemReadListeners,
		itemProcessListeners,
		itemWriteListeners,
		skipListeners,
		retryItemListeners,
		chunkListeners, // Chunk-specific listeners
		promotion,
		isolationLevel,
		propagation,
		f.txManagerFactory, // Injected TransactionManagerFactory
		f.metricRecorder,
		f.tracer,
		f.dbConnectionResolver,
	)
	logger.Debugf("Chunk Step '%s' built.", name)
	return step, nil
}

// CreateTaskletStep constructs a new `TaskletStep` instance.
// This method is part of the `StepFactory` interface implementation.
// name: The unique identifier name of the step.
// tasklet: An implementation of the Tasklet interface to be executed.
// stepExecutionListeners: A list of StepExecutionListeners to apply to this step.
// promotion: Promotion settings from StepExecutionContext to JobExecutionContext.
// isolationLevel: The transaction isolation level for this step.
// propagation: The transaction propagation attribute for this step.
//
// Returns:
//
//	`port.Step`: The constructed tasklet-oriented step.
//	error: An error if step creation fails.
func (f *DefaultStepFactory) CreateTaskletStep(
	name string,
	tasklet port.Tasklet,
	stepExecutionListeners []port.StepExecutionListener,
	promotion *model.ExecutionContextPromotion,
	isolationLevel string,
	propagation string,
) (port.Step, error) {
	step := taskletstep.NewTaskletStep(
		name,
		tasklet,         // The tasklet to execute
		f.jobRepository, // Use StepFactory dependency
		stepExecutionListeners,
		promotion,
		isolationLevel,
		propagation,
		f.metricRecorder,
		f.tracer,
	)
	logger.Debugf("Tasklet Step '%s' built.", name)
	return step, nil
}

// CreatePartitionStep constructs a new `PartitionStep` (controller step) instance.
// This method is part of the `StepFactory` interface implementation.
// This step uses the specified Partitioner to create multiple partitions and
// executes the workerStep in each partition.
//
// name: The unique identifier name of the step.
// partitioner: An implementation of the Partitioner interface for generating partitions.
// workerStep: An implementation of the Step interface to be executed in each partition.
// gridSize: The number of partitions to generate (or the maximum number if the partitioner generates fewer).
// jobRepository: An implementation of the JobRepository interface for persisting job metadata.
// stepExecutionListeners: A list of StepExecutionListeners to apply to this step.
// promotion: Promotion settings from StepExecutionContext to JobExecutionContext.
// Returns:
//
//	`port.Step`: The constructed partition-oriented controller step.
//	error: An error if step creation fails.
func (f *DefaultStepFactory) CreatePartitionStep(
	name string,
	partitioner port.Partitioner,
	workerStep port.Step,
	gridSize int,
	jobRepository repository.JobRepository,
	stepExecutionListeners []port.StepExecutionListener,
	promotion *model.ExecutionContextPromotion,
) (port.Step, error) {
	step := partitionstep.NewPartitionStep(
		name,
		partitioner,
		workerStep,
		gridSize,
		jobRepository,
		stepExecutionListeners,
		promotion,
		f.stepExecutor,
	)
	logger.Debugf("Partition Step '%s' built.", name)
	return step, nil
}

// Verify that DefaultStepFactory implements the StepFactory interface.
var _ StepFactory = (*DefaultStepFactory)(nil)
