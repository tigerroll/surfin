package support

import (
	"fmt"
	port "github.com/tigerroll/surfin/pkg/batch/core/application/port"
	config "github.com/tigerroll/surfin/pkg/batch/core/config"
	jsl "github.com/tigerroll/surfin/pkg/batch/core/config/jsl"
	model "github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	repository "github.com/tigerroll/surfin/pkg/batch/core/domain/repository"
	metrics "github.com/tigerroll/surfin/pkg/batch/core/metrics"
	tx "github.com/tigerroll/surfin/pkg/batch/core/tx"
	step_factory "github.com/tigerroll/surfin/pkg/batch/engine/step/factory"
	exception "github.com/tigerroll/surfin/pkg/batch/support/util/exception"
	"go.uber.org/fx"
)

// JobBuilder is a function type for creating a specific Job.
// This function takes a JobRepository, Config, a list of JobExecutionListeners, a FlowDefinition,
// a MetricRecorder, and a Tracer, and returns a port.Job interface and an error.
type JobBuilder func(
	jobRepository repository.JobRepository, // The job repository.
	cfg *config.Config,
	listeners []port.JobExecutionListener,
	flow *model.FlowDefinition,
	metricRecorder metrics.MetricRecorder,
	tracer metrics.Tracer,
) (port.Job, error)

// JobFactory is a central factory for constructing key elements of the batch framework,
// such as jobs, steps, components, and listeners, based on JSL (Job Specification Language) definitions.
// This factory manages registered builder functions and resolves dependencies to generate executable objects.
// Each field represents a dependency required for the JobFactory to fulfill its responsibilities.
type JobFactory struct {
	config                           *config.Config
	jobRepository                    repository.JobRepository
	expressionResolver               port.ExpressionResolver
	dbConnectionResolver             port.DBConnectionResolver
	componentBuilders                map[string]jsl.ComponentBuilder
	jobBuilders                      map[string]JobBuilder
	jobListenerBuilders              map[string]jsl.JobExecutionListenerBuilder
	stepListenerBuilders             map[string]jsl.StepExecutionListenerBuilder
	itemReadListenerBuilders         map[string]jsl.ItemReadListenerBuilder
	itemProcessListenerBuilders      map[string]jsl.ItemProcessListenerBuilder
	itemWriteListenerBuilders        map[string]jsl.ItemWriteListenerBuilder
	skipListenerBuilders             map[string]jsl.SkipListenerBuilder
	retryItemListenerBuilders        map[string]jsl.LoggingRetryItemListenerBuilder
	chunkListenerBuilders            map[string]jsl.ChunkListenerBuilder
	decisionBuilders                 map[string]jsl.ConditionalDecisionBuilder
	splitBuilders                    map[string]jsl.SplitBuilder
	partitionerBuilders              map[string]port.PartitionerBuilder
	jobParametersIncrementerBuilders map[string]jsl.JobParametersIncrementerBuilder
	notificationListenerBuilders     map[string]jsl.NotificationListenerBuilder // ADD: For L4.4
	metadataTxManager                tx.TransactionManager                      // ADD: For passing to JSL converter
	stepFactory                      step_factory.StepFactory
	metricRecorder                   metrics.MetricRecorder
	tracer                           metrics.Tracer
}

// JobFactoryParams defines the parameters that the NewJobFactory function
// receives via dependency injection (Fx).
// Each field represents a dependency required for the JobFactory to fulfill its responsibilities.
type JobFactoryParams struct {
	fx.In
	Cfg               *config.Config            // Global configuration for the framework.
	Repo              repository.JobRepository  // JobRepository used for persisting job metadata.
	Resolver          port.ExpressionResolver   // ExpressionResolver for resolving dynamic expressions within JSL.
	MetricRecorder    metrics.MetricRecorder    // MetricRecorder for recording metrics.
	Tracer            metrics.Tracer            // Tracer for distributed tracing.
	DBResolver        port.DBConnectionResolver // DBConnectionResolver for resolving database connection names.
	MetadataTxManager tx.TransactionManager     `name:"metadata"` // ADD: Metadata TxManager
	StepFactory       step_factory.StepFactory  // StepFactory for building steps.
}

// NewJobFactory creates a new instance of JobFactory.
//
// p: Receives required dependencies through the JobFactoryParams struct.
// Returns: A pointer to the initialized JobFactory.
func NewJobFactory(p JobFactoryParams) *JobFactory {
	return &JobFactory{
		config:                           p.Cfg,
		jobRepository:                    p.Repo,
		expressionResolver:               p.Resolver,
		dbConnectionResolver:             p.DBResolver,
		metricRecorder:                   p.MetricRecorder,
		tracer:                           p.Tracer,
		componentBuilders:                make(map[string]jsl.ComponentBuilder),
		jobBuilders:                      make(map[string]JobBuilder),
		jobListenerBuilders:              make(map[string]jsl.JobExecutionListenerBuilder),
		stepListenerBuilders:             make(map[string]jsl.StepExecutionListenerBuilder),
		itemReadListenerBuilders:         make(map[string]jsl.ItemReadListenerBuilder),
		itemProcessListenerBuilders:      make(map[string]jsl.ItemProcessListenerBuilder),
		itemWriteListenerBuilders:        make(map[string]jsl.ItemWriteListenerBuilder),
		skipListenerBuilders:             make(map[string]jsl.SkipListenerBuilder),
		retryItemListenerBuilders:        make(map[string]jsl.LoggingRetryItemListenerBuilder),
		chunkListenerBuilders:            make(map[string]jsl.ChunkListenerBuilder),
		decisionBuilders:                 make(map[string]jsl.ConditionalDecisionBuilder),
		splitBuilders:                    make(map[string]jsl.SplitBuilder),
		partitionerBuilders:              make(map[string]port.PartitionerBuilder),
		jobParametersIncrementerBuilders: make(map[string]jsl.JobParametersIncrementerBuilder),
		notificationListenerBuilders:     make(map[string]jsl.NotificationListenerBuilder), // Initialize
		metadataTxManager:                p.MetadataTxManager,                              // ADD: Initialize metadataTxManager
		stepFactory:                      p.StepFactory,
	}
}

// GetConfig returns a reference to the Config held by the JobFactory.
func (f *JobFactory) GetConfig() *config.Config {
	return f.config
}

// RegisterComponentBuilder registers a component builder function with the given name.
//
// name: The reference name of the component.
// builder: The function to build the component.
func (f *JobFactory) RegisterComponentBuilder(name string, builder jsl.ComponentBuilder) {
	f.componentBuilders[name] = builder
}

// RegisterJobBuilder registers a job builder function with the given name.
//
// name: The reference name of the job.
// builder: The function to build the job.
func (f *JobFactory) RegisterJobBuilder(name string, builder JobBuilder) {
	f.jobBuilders[name] = builder
}

// RegisterJobListenerBuilder registers a JobExecutionListener builder function with the given name.
//
// name: The reference name of the listener.
// builder: The function to build the JobExecutionListener.
func (f *JobFactory) RegisterJobListenerBuilder(name string, builder jsl.JobExecutionListenerBuilder) {
	f.jobListenerBuilders[name] = builder
}

// RegisterNotificationListenerBuilder registers a NotificationListener builder function with the given name.
//
// name: The reference name of the listener.
// builder: The function to build the NotificationListener.
func (f *JobFactory) RegisterNotificationListenerBuilder(name string, builder jsl.NotificationListenerBuilder) {
	f.notificationListenerBuilders[name] = builder
}

// RegisterStepExecutionListenerBuilder registers a StepExecutionListener builder function with the given name.
//
// name: The reference name of the listener.
// builder: The function to build the StepExecutionListener.
func (f *JobFactory) RegisterStepExecutionListenerBuilder(name string, builder jsl.StepExecutionListenerBuilder) {
	f.stepListenerBuilders[name] = builder
}

// RegisterItemReadListenerBuilder registers an ItemReadListener builder function with the given name.
//
// name: The reference name of the listener.
// builder: The function to build the ItemReadListener.
func (f *JobFactory) RegisterItemReadListenerBuilder(name string, builder jsl.ItemReadListenerBuilder) {
	f.itemReadListenerBuilders[name] = builder
}

// RegisterItemProcessListenerBuilder registers an ItemProcessListener builder function with the given name.
//
// name: The reference name of the listener.
// builder: The function to build the ItemProcessListener.
func (f *JobFactory) RegisterItemProcessListenerBuilder(name string, builder jsl.ItemProcessListenerBuilder) {
	f.itemProcessListenerBuilders[name] = builder
}

// RegisterItemWriteListenerBuilder registers an ItemWriteListener builder function with the given name.
//
// name: The reference name of the listener.
// builder: The function to build the ItemWriteListener.
func (f *JobFactory) RegisterItemWriteListenerBuilder(name string, builder jsl.ItemWriteListenerBuilder) {
	f.itemWriteListenerBuilders[name] = builder
}

// RegisterSkipListenerBuilder registers a SkipListener builder function with the given name.
//
// name: The reference name of the listener.
// builder: The function to build the SkipListener.
func (f *JobFactory) RegisterSkipListenerBuilder(name string, builder jsl.SkipListenerBuilder) {
	f.skipListenerBuilders[name] = builder
}

// RegisterRetryItemListenerBuilder registers a RetryItemListener builder function with the given name.
//
// name: The reference name of the listener.
// builder: The function to build the RetryItemListener.
func (f *JobFactory) RegisterRetryItemListenerBuilder(name string, builder jsl.LoggingRetryItemListenerBuilder) {
	f.retryItemListenerBuilders[name] = builder
}

// RegisterChunkListenerBuilder registers a ChunkListener builder function with the given name.
//
// name: The reference name of the listener.
// builder: The function to build the ChunkListener.
func (f *JobFactory) RegisterChunkListenerBuilder(name string, builder jsl.ChunkListenerBuilder) {
	f.chunkListenerBuilders[name] = builder
}

// RegisterDecisionBuilder registers a Decision builder function with the given name.
//
// name: The reference name of the Decision.
// builder: The function to build the Decision.
func (f *JobFactory) RegisterDecisionBuilder(name string, builder jsl.ConditionalDecisionBuilder) {
	f.decisionBuilders[name] = builder
}

// RegisterSplitBuilder registers a Split builder function with the given name.
//
// name: The reference name of the Split.
// builder: The function to build the Split.
func (f *JobFactory) RegisterSplitBuilder(name string, builder jsl.SplitBuilder) {
	f.splitBuilders[name] = builder
}

// RegisterPartitionerBuilder registers a Partitioner builder function with the given name.
//
// name: The reference name of the Partitioner.
// builder: The function to build the Partitioner.
func (f *JobFactory) RegisterPartitionerBuilder(name string, builder port.PartitionerBuilder) {
	f.partitionerBuilders[name] = builder
}

// RegisterJobParametersIncrementerBuilder registers a JobParametersIncrementer builder function with the given name.
//
// name: The reference name of the incrementer.
// builder: The function to build the JobParametersIncrementer.
func (f *JobFactory) RegisterJobParametersIncrementerBuilder(name string, builder jsl.JobParametersIncrementerBuilder) {
	f.jobParametersIncrementerBuilders[name] = builder
}

// CreateJob constructs a core.Job object corresponding to the specified job name.
// It reads the JSL definition and instantiates the job's flow, components, and listeners
// using registered builders.
//
// jobName: The name of the job to construct.
// Returns: The constructed port.Job interface and an error.
//
//	Returns an error if the JSL definition is not found, the builder is not registered,
//	or component construction fails.
func (f *JobFactory) CreateJob(jobName string) (port.Job, error) {
	jslJob, ok := jsl.GetJobDefinition(jobName)
	if !ok {
		return nil, exception.NewBatchErrorf("job_factory", "JSL definition for Job '%s' not found", jobName)
	}

	jobBuilder, found := f.jobBuilders[jobName]
	if !found {
		return nil, exception.NewBatchErrorf("job_factory", "Builder for Job '%s' not registered", jobName)
	}

	coreFlow, err := jsl.ConvertJSLToCoreFlow(
		jslJob.Flow,
		f.componentBuilders,
		f.jobRepository,
		f.config,
		f.decisionBuilders,
		f.splitBuilders,
		f.stepFactory,
		f.metadataTxManager, // ADD: Pass metadataTxManager
		f.partitionerBuilders,
		f.expressionResolver,
		f.dbConnectionResolver,
		f.stepListenerBuilders,
		f.itemReadListenerBuilders,
		f.itemProcessListenerBuilders,
		f.itemWriteListenerBuilders,
		f.skipListenerBuilders,
		f.retryItemListenerBuilders,
		f.chunkListenerBuilders,
	)
	if err != nil {
		return nil, exception.NewBatchError("job_factory", fmt.Sprintf("Failed to convert JSL flow for job '%s'", jobName), err, false, false)
	}

	var jobListeners []port.JobExecutionListener

	// WB1: Automatic inclusion of default logging listener
	if loggingBuilder, found := f.jobListenerBuilders["loggingJobListener"]; found {
		listenerInstance, err := loggingBuilder(f.config, map[string]string{})
		if err != nil {
			return nil, exception.NewBatchError("job_factory", "Failed to build default loggingJobListener", err, false, false)
		}
		jobListeners = append(jobListeners, listenerInstance)
	}

	for _, listenerRef := range jslJob.Listeners {
		// Search from both JobExecutionListenerBuilder and NotificationListenerBuilder
		builder, found := f.jobListenerBuilders[listenerRef.Ref]
		if !found {
			// Retrieve from NotificationListenerBuilder and type assert to JobExecutionListenerBuilder
			if b, ok := f.notificationListenerBuilders[listenerRef.Ref]; ok {
				builder = jsl.JobExecutionListenerBuilder(b)
				found = true
			}
		}

		if !found {
			return nil, exception.NewBatchErrorf("job_factory", "JobExecutionListener builder '%s' not registered", listenerRef.Ref)
		}

		listenerInstance, err := builder(f.config, listenerRef.Properties)
		if err != nil {
			return nil, exception.NewBatchError("job_factory", fmt.Sprintf("Failed to build JobExecutionListener '%s'", listenerRef.Ref), err, false, false)
		}
		jobListeners = append(jobListeners, listenerInstance)
	}

	jobInstance, err := jobBuilder(
		f.jobRepository,
		f.config,
		jobListeners,
		coreFlow,
		f.metricRecorder,
		f.tracer,
	)
	if err != nil {
		return nil, exception.NewBatchError("job_factory", fmt.Sprintf("Failed to instantiate job '%s'", jobName), err, false, false)
	}

	return jobInstance, nil
}

// GetJobParametersIncrementer constructs and returns the JobParametersIncrementer for the specified job.
// Returns nil if no incrementer is specified in the JSL definition or if the corresponding builder is not found.
//
// jobName: The name of the job for which to retrieve the incrementer.
// Returns: The constructed port.JobParametersIncrementer interface, or nil.
func (f *JobFactory) GetJobParametersIncrementer(jobName string) port.JobParametersIncrementer {
	jslJob, ok := jsl.GetJobDefinition(jobName)
	if !ok || jslJob.Incrementer.Ref == "" {
		return nil
	}

	builder, found := f.jobParametersIncrementerBuilders[jslJob.Incrementer.Ref]
	if !found {
		return nil
	}

	incrementer, err := builder(f.config, jslJob.Incrementer.Properties)
	if err != nil {
		return nil
	}
	return incrementer
}
