// Package jsl defines the models for the Job Specification Language (JSL) in the Surfin Batch Framework.
// It is used to declaratively describe the structure, flow, and components of batch jobs in YAML format.
package jsl

import (
	coreAdapter "github.com/tigerroll/surfin/pkg/batch/core/adapter" // Imports the generic adapter interface.
	core "github.com/tigerroll/surfin/pkg/batch/core/application/port"
	config "github.com/tigerroll/surfin/pkg/batch/core/config"
)

// JSLDefinitionBytes holds the content of a JSL file as a byte slice.
// This is used when loading JSL definitions into memory.
type JSLDefinitionBytes []byte

// Job represents the top-level structure of a JSL file, containing the entire batch job definition.
type Job struct {
	// ID is the unique identifier for the job.
	ID string `yaml:"id"`
	// Name is the logical name of the job.
	Name string `yaml:"name"`
	// Description is an optional description for the job.
	Description string `yaml:"description,omitempty"`
	// Flow defines the execution flow of the job.
	Flow Flow `yaml:"flow"`
	// Listeners is an optional list of JobExecutionListener references applied to this job.
	Listeners []ComponentRef `yaml:"listeners,omitempty"`
	// Incrementer is an optional reference to a JobParametersIncrementer.
	Incrementer ComponentRef `yaml:"incrementer,omitempty"`
}

// Flow represents a sequence of steps or decisions, defining the execution order of a job.
type Flow struct {
	// StartElement is the ID of the starting element in the flow.
	StartElement string `yaml:"start-element"`
	// Elements is a map of all elements (Step, Decision, Split) within the flow.
	// The key is the element's ID, and the value is the corresponding element definition.
	Elements map[string]interface{} `yaml:"elements"`
}

// Step represents a single processing unit within a job.
// In JSR352, a step is either chunk-oriented or tasklet-oriented, but cannot be both simultaneously.
type Step struct {
	// ID is the unique identifier for the step.
	ID string `yaml:"id"`
	// Description is an optional description for the step.
	Description string `yaml:"description,omitempty"`
	// Reader is an optional reference to an ItemReader component, used only for chunk-oriented steps.
	Reader ComponentRef `yaml:"reader,omitempty"`
	// Processor is an optional reference to an ItemProcessor component, used only for chunk-oriented steps.
	Processor ComponentRef `yaml:"processor,omitempty"`
	// Writer is an optional reference to an ItemWriter component, used only for chunk-oriented steps.
	Writer ComponentRef `yaml:"writer,omitempty"`
	// Chunk defines the properties for chunk-oriented processing.
	Chunk *Chunk `yaml:"chunk,omitempty"`
	// Tasklet is an optional reference to a Tasklet component, used only for tasklet-oriented steps.
	Tasklet ComponentRef `yaml:"tasklet,omitempty"`
	// Partition defines the properties for partitioning.
	Partition *Partition `yaml:"partition,omitempty"`
	// Propagation specifies the transaction propagation attribute (e.g., "REQUIRED", "REQUIRES_NEW").
	Propagation string `yaml:"propagation,omitempty"`
	// IsolationLevel specifies the transaction isolation level for a tasklet step or overrides chunk settings.
	IsolationLevel string `yaml:"isolation-level,omitempty"`
	// Transitions defines the transition rules from this step.
	Transitions []Transition `yaml:"transitions,omitempty"`
	// Listeners is an optional list of StepExecutionListener references applied to this step.
	Listeners []ComponentRef `yaml:"listeners,omitempty"`
	// ItemReadListeners is an optional list of ItemReadListener references applied to this step.
	ItemReadListeners []ComponentRef `yaml:"item-read-listeners,omitempty"`
	// ItemProcessListeners is an optional list of ItemProcessListener references applied to this step.
	ItemProcessListeners []ComponentRef `yaml:"item-process-listeners,omitempty"`
	// ItemWriteListeners is an optional list of ItemWriteListener references applied to this step.
	ItemWriteListeners []ComponentRef `yaml:"item-write-listeners,omitempty"`
	// SkipListeners is an optional list of SkipListener references applied to this step.
	SkipListeners []ComponentRef `yaml:"skip-listeners,omitempty"`
	// RetryItemListeners is an optional list of RetryItemListener references applied to this step.
	RetryItemListeners []ComponentRef `yaml:"retry-item-listeners,omitempty"`
	// ChunkListeners is an optional list of ChunkListener references applied to this step.
	ChunkListeners []ComponentRef `yaml:"chunk-listeners,omitempty"`
	// ExecutionContextPromotion defines the promotion settings from StepExecutionContext to JobExecutionContext.
	ExecutionContextPromotion *ExecutionContextPromotion `yaml:"execution-context-promotion,omitempty"`
	// TransactionAttribute is an optional string representation of transaction attributes.
	TransactionAttribute string `yaml:"transaction-attribute,omitempty"`
}

// ComponentRef refers to a registered component (e.g., reader, processor, writer, tasklet).
type ComponentRef struct {
	// Ref is the reference name of the component.
	Ref string `yaml:"ref"`
	// Properties is an optional map of properties injected from JSL.
	Properties map[string]string `yaml:"properties,omitempty"`
}

// Chunk defines the chunk-oriented processing properties for a step.
type Chunk struct {
	// ItemCount specifies the number of items to be processed in a chunk.
	ItemCount int `yaml:"item-count"`
	// CommitInterval corresponds to the commit interval in JSR352.
	CommitInterval int `yaml:"commit-interval"`
	// IsolationLevel specifies the transaction isolation level (e.g., "SERIALIZABLE").
	IsolationLevel string `yaml:"isolation-level,omitempty"`
}

// Decision represents a conditional branching point in the flow.
type Decision struct {
	// ID is the unique identifier for the Decision.
	ID string `yaml:"id"`
	// Description is an optional description for the Decision.
	Description string `yaml:"description,omitempty"`
	// Ref is the reference name of the Decision Builder.
	Ref string `yaml:"ref"`
	// Properties is an optional map of properties injected from JSL.
	Properties map[string]string `yaml:"properties,omitempty"`
	// Transitions defines the transition rules from this Decision.
	Transitions []Transition `yaml:"transitions"`
}

// Split represents the parallel execution of multiple steps.
type Split struct {
	// ID is the unique identifier for the Split.
	ID string `yaml:"id"`
	// Description is an optional description for the Split.
	Description string `yaml:"description,omitempty"`
	// Steps is a list of step IDs to be executed in parallel.
	Steps []string `yaml:"steps"`
	// Transitions defines the transition rules from this Split.
	Transitions []Transition `yaml:"transitions,omitempty"`
}

// Partition defines the partitioning properties for a step (controller step).
type Partition struct {
	// Partitioner is a reference to the Partitioner component.
	Partitioner ComponentRef `yaml:"partitioner"`
	// Step is the ID of the worker step (referencing another step in JSL).
	Step string `yaml:"step"`
	// GridSize specifies the number of partitions to create.
	GridSize int `yaml:"grid-size"`
}

// ComponentBuilder is a generic function type for building core components such as ItemReader, ItemProcessor, and ItemWriter.
// This builder provides access to framework configuration, expression resolvers, and resource connection resolvers,
// allowing components to access dynamic property resolution logic.
//
// Parameters:
//
//	cfg: The global framework configuration.
//	resolver: The expression resolver for dynamic property resolution.
//	dbResolver: The resource connection resolver for resolving database connections.
//	properties: A map of properties injected from JSL.
//
// Returns:
//
//	The constructed component instance and an error if construction fails.
type ComponentBuilder func(
	cfg *config.Config,
	resolver core.ExpressionResolver,
	dbResolver coreAdapter.ResourceConnectionResolver, // Type changed to coreAdapter.ResourceConnectionResolver.
	properties map[string]string,
) (interface{}, error)

// JobExecutionListenerBuilder is a function type for building JobExecutionListeners.
//
// Parameters:
//
//	cfg: The global framework configuration.
//	properties: Listener-specific properties injected from JSL.
//
// Returns:
//
//	The constructed JobExecutionListener instance and an error.
type JobExecutionListenerBuilder func(cfg *config.Config, properties map[string]string) (core.JobExecutionListener, error)

// NotificationListenerBuilder is a function type for building notification listeners.
//
// Parameters:
//
//	cfg: The global framework configuration.
//	properties: Listener-specific properties injected from JSL.
//
// Returns:
//
//	The constructed NotificationListener instance (which satisfies the JobExecutionListener interface) and an error.
type NotificationListenerBuilder func(cfg *config.Config, properties map[string]string) (core.JobExecutionListener, error)

// JobParametersIncrementerBuilder is a function type for building JobParametersIncrementers.
//
// Parameters:
//
//	cfg: The global framework configuration.
//	properties: Incrementer-specific properties injected from JSL.
//
// Returns:
//
//	The constructed JobParametersIncrementer instance and an error.
type JobParametersIncrementerBuilder func(cfg *config.Config, properties map[string]string) (core.JobParametersIncrementer, error)

// Transition defines the next element to execute based on the exit status.
type Transition struct {
	// On is the ExitStatus or wildcard ("*") that triggers the transition.
	On string `yaml:"on"`
	// To is the ID of the target element. It is optional if End, Fail, or Stop is true.
	To string `yaml:"to,omitempty"`
	// End indicates that the job should complete. If true, the job ends successfully.
	End bool `yaml:"end,omitempty"`
	// Fail indicates that the job should fail. If true, the job ends with a failure.
	Fail bool `yaml:"fail,omitempty"`
	// Stop indicates that the job should stop. If true, the job ends with a stop status.
	Stop bool `yaml:"stop,omitempty"`
}

// ExecutionContextPromotion defines the promotion settings from StepExecutionContext to JobExecutionContext.
type ExecutionContextPromotion struct {
	// Keys is an optional list of keys to promote.
	Keys []string `yaml:"keys,omitempty"`
	// JobLevelKeys is an optional map for renaming promoted keys at the job level (e.g., `stepKey: jobKey`).
	JobLevelKeys map[string]string `yaml:"job-level-keys,omitempty"`
}

// StepExecutionListenerBuilder is a function type for building StepExecutionListeners.
//
// Parameters:
//
//	cfg: The global framework configuration.
//	properties: Listener-specific properties injected from JSL.
//
// Returns:
//
//	The constructed StepExecutionListener instance and an error.
type StepExecutionListenerBuilder func(cfg *config.Config, properties map[string]string) (core.StepExecutionListener, error)

// ItemReadListenerBuilder is a function type for building ItemReadListeners.
//
// Parameters:
//
//	cfg: The global framework configuration.
//	properties: Listener-specific properties injected from JSL.
//
// Returns:
//
//	The constructed ItemReadListener instance and an error.
type ItemReadListenerBuilder func(cfg *config.Config, properties map[string]string) (core.ItemReadListener, error)

// ItemProcessListenerBuilder is a function type for building ItemProcessListeners.
//
// Parameters:
//
//	cfg: The global framework configuration.
//	properties: Listener-specific properties injected from JSL.
//
// Returns:
//
//	The constructed ItemProcessListener instance and an error.
type ItemProcessListenerBuilder func(cfg *config.Config, properties map[string]string) (core.ItemProcessListener, error)

// ItemWriteListenerBuilder is a function type for building ItemWriteListeners.
//
// Parameters:
//
//	cfg: The global framework configuration.
//	properties: Listener-specific properties injected from JSL.
//
// Returns:
//
//	The constructed ItemWriteListener instance and an error.
type ItemWriteListenerBuilder func(cfg *config.Config, properties map[string]string) (core.ItemWriteListener, error)

// SkipListenerBuilder is a function type for building SkipListeners.
//
// Parameters:
//
//	cfg: The global framework configuration.
//	properties: Listener-specific properties injected from JSL.
//
// Returns:
//
//	The constructed SkipListener instance and an error.
type SkipListenerBuilder func(cfg *config.Config, properties map[string]string) (core.SkipListener, error)

// LoggingRetryItemListenerBuilder is a function type for building RetryItemListeners.
//
// Parameters:
//
//	cfg: The global framework configuration.
//	properties: Listener-specific properties injected from JSL.
//
// Returns:
//
//	The constructed RetryItemListener instance and an error.
type LoggingRetryItemListenerBuilder func(cfg *config.Config, properties map[string]string) (core.RetryItemListener, error)

// ChunkListenerBuilder is a function type for building ChunkListeners.
//
// Parameters:
//
//	cfg: The global framework configuration.
//	properties: Listener-specific properties injected from JSL.
//
// Returns:
//
//	The constructed ChunkListener instance and an error.
type ChunkListenerBuilder func(cfg *config.Config, properties map[string]string) (core.ChunkListener, error)

// ConditionalDecisionBuilder is a function type for building core.Decision.
//
// Parameters:
//
//	id: The unique identifier for the Decision.
//	properties: Decision-specific properties injected from JSL.
//	resolver: The expression resolver.
//
// Returns:
//
//	The constructed Decision instance and an error.
type ConditionalDecisionBuilder func(id string, properties map[string]string, resolver core.ExpressionResolver) (core.Decision, error)

// SplitBuilder is a function type for building core.Split.
//
// Parameters:
//
//	id: The unique identifier for the Split.
//	steps: A list of Steps included in this Split.
//
// Returns:
//
//	The constructed Split instance and an error.
type SplitBuilder func(id string, steps []core.Step) (core.Split, error)
