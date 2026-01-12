package factory

import (
	config "surfin/pkg/batch/core/config"
	port "surfin/pkg/batch/core/application/port"
	model "surfin/pkg/batch/core/domain/model"
	repository "surfin/pkg/batch/core/domain/repository"
	itemstep "surfin/pkg/batch/engine/step/item"
	partitionstep "surfin/pkg/batch/engine/step/partition"
	taskletstep "surfin/pkg/batch/engine/step/tasklet"
	tx "surfin/pkg/batch/core/tx"
	metrics "surfin/pkg/batch/core/metrics"
	"go.uber.org/fx"
	logger "surfin/pkg/batch/support/util/logger"
)

// StepFactory is an interface for creating port.Step instances.
// This factory is responsible for converting step definitions defined in JSL (Job Specification Language)
// into concrete, executable Step objects.
type StepFactory interface {
	// CreateChunkStep constructs a chunk-oriented Step.
	// Each argument represents the properties of the chunk step defined in JSL, along with its associated components (Reader, Processor, Writer) and listeners.
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
		txManager tx.TransactionManager,
	) (port.Step, error)

	// CreateTaskletStep constructs a tasklet-oriented Step.
	// Each argument represents the properties of the tasklet step defined in JSL, along with its associated component (Tasklet) and listeners.
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

// DefaultStepFactory is the default implementation of the StepFactory interface.
// It manages the dependencies required for constructing Steps.
type DefaultStepFactory struct {
	jobRepository repository.JobRepository
	txManager     tx.TransactionManager
	stepExecutor  port.StepExecutor
	metricRecorder metrics.MetricRecorder
	tracer        metrics.Tracer
}

// DefaultStepFactoryParams defines the parameters that the NewDefaultStepFactory function
// receives through dependency injection (Fx).
type DefaultStepFactoryParams struct {
	fx.In
	JobRepository repository.JobRepository
	MetadataTxManager tx.TransactionManager `name:"metadata"` // Requests the metadata TxManager.
	StepExecutor  port.StepExecutor
	MetricRecorder metrics.MetricRecorder
	Tracer        metrics.Tracer
}

// NewDefaultStepFactory creates a new instance of DefaultStepFactory.
//
// p: Receives the necessary dependencies (JobRepository, MetadataTxManager, StepExecutor, MetricRecorder, Tracer) through the DefaultStepFactoryParams struct.
// Returns: A pointer to the initialized DefaultStepFactory.
func NewDefaultStepFactory(
	p DefaultStepFactoryParams,
) *DefaultStepFactory {
	return &DefaultStepFactory{
		jobRepository: p.JobRepository,
		txManager:     p.MetadataTxManager, // Uses the tagged TxManager.
		stepExecutor:  p.StepExecutor,
		metricRecorder: p.MetricRecorder,
		tracer:        p.Tracer,
	}
}

// CreateChunkStep constructs a new ChunkStep instance.
//
// name: The unique identifier name of the step.
// reader: An implementation of the ItemReader interface for reading items.
// processor: An implementation of the ItemProcessor interface for processing items.
// writer: An implementation of the ItemWriter interface for writing items.
// chunkSize: The maximum number of items to process at once.
// commitInterval: The interval at which transactions are committed (usually the same as chunkSize).
// retryConfig: Step-level retry configuration.
// itemRetryConfig: Item-level retry configuration.
// itemSkipConfig: Item-level skip configuration.
// stepExecutionListeners: A list of StepExecutionListeners to apply to this step.
// itemReadListeners: A list of ItemReadListeners to apply to this step.
// itemProcessListeners: A list of ItemProcessListeners to apply to this step.
// itemWriteListeners: A list of ItemWriteListeners to apply to this step.
// skipListeners: A list of SkipListeners to apply to this step.
// retryItemListeners: A list of RetryItemListeners to apply to this step.
// chunkListeners: A list of ChunkListeners to apply to this step.
// promotion: Promotion settings from StepExecutionContext to JobExecutionContext.
// isolationLevel: The transaction isolation level for this step.
// propagation: The transaction propagation attribute for this step.
// txManager: The transaction manager to use for this step.
// Returns: The constructed port.Step interface and an error.
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
	txManager tx.TransactionManager,
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
		f.jobRepository, // Use StepFactory dependency
		stepExecutionListeners,
		itemReadListeners,
		itemProcessListeners,
		itemWriteListeners,
		skipListeners,
		retryItemListeners,
		chunkListeners,
		promotion,
		isolationLevel,
		propagation,
		txManager,
		f.metricRecorder,
		f.tracer,
	)
	logger.Debugf("Chunk Step '%s' built.", name)
	return step, nil
}

// CreateTaskletStep constructs a new TaskletStep instance.
//
// name: The unique identifier name of the step.
// tasklet: An implementation of the Tasklet interface to be executed.
// stepExecutionListeners: A list of StepExecutionListeners to apply to this step.
// promotion: Promotion settings from StepExecutionContext to JobExecutionContext.
// isolationLevel: The transaction isolation level for this step.
// propagation: The transaction propagation attribute for this step.
//
// Returns: The constructed port.Step interface and an error.
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
		tasklet,
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

// CreatePartitionStep constructs a new PartitionStep (controller step) instance.
// This step uses the specified Partitioner to create multiple partitions and
// executes the workerStep in each partition.
//
// name: The unique identifier name of the step.
// partitioner: An implementation of the Partitioner interface for generating partitions.
// workerStep: An implementation of the Step interface to be executed in each partition.
// gridSize: The number of partitions to generate.
// jobRepository: An implementation of the JobRepository interface for persisting job metadata.
// stepExecutionListeners: A list of StepExecutionListeners to apply to this step.
// promotion: Promotion settings from StepExecutionContext to JobExecutionContext.
// Returns: The constructed port.Step interface and an error.
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
