package jsl

import (
	"context"
	"fmt"
	"reflect"
	tx "github.com/tigerroll/surfin/pkg/batch/core/tx"
	config "github.com/tigerroll/surfin/pkg/batch/core/config"
	core "github.com/tigerroll/surfin/pkg/batch/core/application/port"
	model "github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	job "github.com/tigerroll/surfin/pkg/batch/core/domain/repository"
	step_factory "github.com/tigerroll/surfin/pkg/batch/engine/step/factory"
	exception "github.com/tigerroll/surfin/pkg/batch/support/util/exception"
	logger "github.com/tigerroll/surfin/pkg/batch/support/util/logger"

	yaml "gopkg.in/yaml.v3"
)

// resolveComponentRefProperties resolves dynamic expressions within the ComponentRef's Properties map.
// JobExecution/StepExecution are nil during JSL converter execution.
// Therefore, only expressions that do not depend on JobParameters or EC (e.g., environment variables) are resolved.
func resolveComponentRefProperties(resolver core.ExpressionResolver, ref *ComponentRef) (map[string]string, error) {
	module := "jsl_converter"
	if ref == nil || len(ref.Properties) == 0 {
		return ref.Properties, nil // Return as is if no properties
	}
	
	resolvedProps := make(map[string]string, len(ref.Properties))
	for key, value := range ref.Properties {
		// Use context.Background() since JobExecution/StepExecution are nil during JSL converter execution.
		resolvedValue, err := resolver.Resolve(context.Background(), value, nil, nil)
		if err != nil {
			// Return error if resolution fails
			return nil, exception.NewBatchError(module, fmt.Sprintf("Failed to resolve property '%s' for ComponentRef '%s'", key, ref.Ref), err, false, false)
		}
		resolvedProps[key] = resolvedValue
	}
	return resolvedProps, nil
}

// ConvertJSLToCoreFlow converts a JSL Flow definition into a core.FlowDefinition.
// componentBuilders holds builders for components (Reader, Processor, Writer, Tasklet) referenced in JSL.
// jobRepository is used if the repository is needed within the step.
// cfg is the overall application configuration. // FIX: Added period.
// listenerBuilders are maps for dynamically building step-level and item-level listeners.
func ConvertJSLToCoreFlow(
	jslFlow Flow,
	componentBuilders map[string]ComponentBuilder,
	jobRepository job.JobRepository,
	cfg *config.Config,
	decisionBuilders map[string]ConditionalDecisionBuilder,
	splitBuilders map[string]SplitBuilder,
	stepFactory step_factory.StepFactory,
	txManager tx.TransactionManager,
	partitionerBuilders map[string]core.PartitionerBuilder,
	resolver core.ExpressionResolver,
	dbResolver core.DBConnectionResolver,
	stepListenerBuilders map[string]StepExecutionListenerBuilder,
	itemReadListenerBuilders map[string]ItemReadListenerBuilder,
	itemProcessListenerBuilders map[string]ItemProcessListenerBuilder,
	itemWriteListenerBuilders map[string]ItemWriteListenerBuilder,
	skipListenerBuilders map[string]SkipListenerBuilder,
	retryItemListenerBuilders map[string]LoggingRetryItemListenerBuilder,
	chunkListenerBuilders map[string]ChunkListenerBuilder,
) (*model.FlowDefinition, error) {
	module := "jsl_converter"
	flowDef := model.NewFlowDefinition(jslFlow.StartElement)

	if _, ok := jslFlow.Elements[jslFlow.StartElement]; !ok {
		return nil, exception.NewBatchErrorf(module, "Flow 'start-element' '%s' not found in 'elements'", jslFlow.StartElement)
	}

	// First pass: Build all steps, decisions, and splits to resolve potential forward references (e.g., in Split steps).
	builtElements := make(map[string]interface{})
	for id, element := range jslFlow.Elements {
		elementBytes, err := yaml.Marshal(element)
		if err != nil {
			return nil, exception.NewBatchError(module, fmt.Sprintf("Failed to marshal flow element '%s'", id), err, false, false)
		}

		var (
			jslStep    Step
			jslDecision Decision
			jslSplit   Split
		)

		// Attempt to unmarshal as Step
		if err := yaml.Unmarshal(elementBytes, &jslStep); err == nil && (jslStep.Reader.Ref != "" || jslStep.Tasklet.Ref != "" || jslStep.Partition != nil) {
			if jslStep.ID == "" {
				return nil, exception.NewBatchError(module, fmt.Sprintf("Step element '%s' requires an ID", id), nil, false, false)
			}
			if jslStep.ID != id {
				return nil, exception.NewBatchError(module, fmt.Sprintf("Step ID '%s' does not match map key '%s'", jslStep.ID, id), nil, false, false)
			}

			var stepExecListeners []core.StepExecutionListener
			for _, listenerRef := range jslStep.Listeners {
				builder, found := stepListenerBuilders[listenerRef.Ref]
				if !found {
					return nil, exception.NewBatchErrorf(module, "StepExecutionListener builder '%s' is not registered", listenerRef.Ref)
				}
				listenerInstance, err := builder(cfg, listenerRef.Properties) // FIX: Added properties.
				if err != nil {
					return nil, exception.NewBatchError(module, fmt.Sprintf("Failed to build StepExecutionListener '%s'", listenerRef.Ref), err, false, false)
				}
				stepExecListeners = append(stepExecListeners, listenerInstance)
			}

			var itemReadListeners []core.ItemReadListener
			var itemProcessListeners []core.ItemProcessListener
			var itemWriteListeners []core.ItemWriteListener
			var retryItemListeners []core.RetryItemListener
			var chunkListeners []core.ChunkListener

			if jslStep.Chunk != nil {
				for _, listenerRef := range jslStep.ItemReadListeners {
					builder, found := itemReadListenerBuilders[listenerRef.Ref]
					if !found {
						return nil, exception.NewBatchErrorf(module, "ItemReadListener builder '%s' is not registered", listenerRef.Ref)
					}
					listenerInstance, err := builder(cfg, listenerRef.Properties) // FIX: Added properties.
					if err != nil {
						return nil, exception.NewBatchError(module, fmt.Sprintf("Failed to build ItemReadListener '%s'", listenerRef.Ref), err, false, false)
					}
					itemReadListeners = append(itemReadListeners, listenerInstance)
				}

				for _, listenerRef := range jslStep.ItemProcessListeners {
					builder, found := itemProcessListenerBuilders[listenerRef.Ref]
					if !found {
						return nil, exception.NewBatchErrorf(module, "ItemProcessListener builder '%s' is not registered", listenerRef.Ref)
					}
					listenerInstance, err := builder(cfg, listenerRef.Properties) // FIX: Added properties.
					if err != nil {
						return nil, exception.NewBatchError(module, fmt.Sprintf("Failed to build ItemProcessListener '%s'", listenerRef.Ref), err, false, false)
					}
					itemProcessListeners = append(itemProcessListeners, listenerInstance)
				}

				for _, listenerRef := range jslStep.ItemWriteListeners {
					builder, found := itemWriteListenerBuilders[listenerRef.Ref]
					if !found {
						return nil, exception.NewBatchErrorf(module, "ItemWriteListener builder '%s' is not registered", listenerRef.Ref)
					}
					listenerInstance, err := builder(cfg, listenerRef.Properties) // FIX: Added properties.
					if err != nil {
						return nil, exception.NewBatchError(module, fmt.Sprintf("Failed to build ItemWriteListener '%s'", listenerRef.Ref), err, false, false)
					}
					itemWriteListeners = append(itemWriteListeners, listenerInstance)
				}

				for _, listenerRef := range jslStep.RetryItemListeners {
					builder, found := retryItemListenerBuilders[listenerRef.Ref]
					if !found {
						return nil, exception.NewBatchErrorf(module, "RetryItemListener builder '%s' is not registered", listenerRef.Ref)
					}
					listenerInstance, err := builder(cfg, listenerRef.Properties) // FIX: Pass cfg and properties.
					if err != nil {
						return nil, exception.NewBatchError(module, fmt.Sprintf("Failed to build RetryItemListener '%s'", listenerRef.Ref), err, false, false)
					}
					retryItemListeners = append(retryItemListeners, listenerInstance)
				}

				for _, listenerRef := range jslStep.ChunkListeners {
					builder, found := chunkListenerBuilders[listenerRef.Ref]
					if !found {
						return nil, exception.NewBatchErrorf(module, "ChunkListener builder '%s' is not registered", listenerRef.Ref)
					}
					listenerInstance, err := builder(cfg, listenerRef.Properties) // FIX: Added properties.
					if err != nil {
						return nil, exception.NewBatchError(module, fmt.Sprintf("Failed to build ChunkListener '%s'", listenerRef.Ref), err, false, false)
					}
					chunkListeners = append(chunkListeners, listenerInstance)
				}
			}

			var skipListeners []core.SkipListener
			for _, listenerRef := range jslStep.SkipListeners {
				builder, found := skipListenerBuilders[listenerRef.Ref]
				if !found {
					return nil, exception.NewBatchErrorf(module, "SkipListener builder '%s' is not registered", listenerRef.Ref)
				}
				listenerInstance, err := builder(cfg, listenerRef.Properties) // FIX: Added properties.
				if err != nil {
					return nil, exception.NewBatchError(module, fmt.Sprintf("Failed to build SkipListener '%s'", listenerRef.Ref), err, false, false)
				}
				skipListeners = append(skipListeners, listenerInstance)
			}

			var coreECPromotion *model.ExecutionContextPromotion
			if jslStep.ExecutionContextPromotion != nil {
				coreECPromotion = &model.ExecutionContextPromotion{
					Keys:         jslStep.ExecutionContextPromotion.Keys,
					JobLevelKeys: jslStep.ExecutionContextPromotion.JobLevelKeys,
				}
			}

			var coreStep core.Step
			if jslStep.Chunk != nil {
				if jslStep.Reader.Ref == "" || jslStep.Processor.Ref == "" || jslStep.Writer.Ref == "" {
					return nil, exception.NewBatchErrorf(module, "Chunk Step '%s' requires reader, processor, and writer components", id)
				}

				// Resolve Reader/Processor/Writer properties
				resolvedReaderProps, err := resolveComponentRefProperties(resolver, &jslStep.Reader)
				if err != nil {
					return nil, err
				}
				resolvedProcessorProps, err := resolveComponentRefProperties(resolver, &jslStep.Processor)
				if err != nil {
					return nil, err
				}
				resolvedWriterProps, err := resolveComponentRefProperties(resolver, &jslStep.Writer)
				if err != nil {
					return nil, err
				}
				
				// Update ComponentRef with resolved properties
				resolvedReaderRef := jslStep.Reader
				resolvedReaderRef.Properties = resolvedReaderProps
				resolvedProcessorRef := jslStep.Processor
				resolvedProcessorRef.Properties = resolvedProcessorProps
				resolvedWriterRef := jslStep.Writer
				resolvedWriterRef.Properties = resolvedWriterProps

				r, p, w, err := buildReaderWriterProcessor(module, componentBuilders, cfg, jobRepository, resolver, dbResolver, &resolvedReaderRef, &resolvedProcessorRef, &resolvedWriterRef)
				if err != nil {
					return nil, err
				}

				// Get the TxManager used by ItemWriter (depends on ItemWriter's construction logic)
				// Mimic WeatherItemWriter's constructor logic to get the DB name.
				dbName, ok := resolvedWriterProps["targetDBName"]
				if !ok || dbName == "" {
					dbName, ok = resolvedWriterProps["database"]
					if !ok || dbName == "" {
						dbName = "workload" // Default value
					}
				}

				// TxManager is now passed directly from JobFactory
				chunkTxManager := txManager

				chunkIsolationLevel := jslStep.Chunk.IsolationLevel

				coreStep, err = stepFactory.CreateChunkStep(
					jslStep.ID,
					r,
					p,
					w,
					jslStep.Chunk.ItemCount,
					jslStep.Chunk.CommitInterval,
					&cfg.Surfin.Batch.Retry,
					cfg.Surfin.Batch.ItemRetry,
					cfg.Surfin.Batch.ItemSkip,
					stepExecListeners,
					itemReadListeners,
					itemProcessListeners,
					itemWriteListeners,
					skipListeners,
					retryItemListeners,
					chunkListeners,
					coreECPromotion,
					chunkIsolationLevel,
					jslStep.Propagation,
					chunkTxManager,
				)
				if err != nil {
					return nil, exception.NewBatchError(module, fmt.Sprintf("Failed to build Chunk Step '%s'", id), err, false, false)
				}
				logger.Debugf("Chunk Step '%s' built.", id)

			} else if jslStep.Tasklet.Ref != "" {
				taskletBuilder, ok := componentBuilders[jslStep.Tasklet.Ref]
				if !ok {
					return nil, exception.NewBatchErrorf(module, "Tasklet component builder '%s' not found", jslStep.Tasklet.Ref)
				}

				// Resolve Tasklet properties
				resolvedTaskletProps, err := resolveComponentRefProperties(resolver, &jslStep.Tasklet)
				if err != nil {
					return nil, err
				}

				taskletInstance, err := taskletBuilder(cfg, jobRepository, resolver, dbResolver, resolvedTaskletProps)
				if err != nil {
					return nil, exception.NewBatchError(module, fmt.Sprintf("Failed to build Tasklet '%s'", jslStep.Tasklet.Ref), err, false, false)
				}
				t, isTasklet := taskletInstance.(core.Tasklet)
				if !isTasklet {
					return nil, exception.NewBatchErrorf(module, "Tasklet '%s' not found or is of incorrect type (Expected: core.Tasklet, Actual: %s)", jslStep.Tasklet.Ref, reflect.TypeOf(taskletInstance))
				}

				coreStep, err = stepFactory.CreateTaskletStep(
					jslStep.ID,
					t,
					stepExecListeners,
					coreECPromotion,
					jslStep.IsolationLevel,
					jslStep.Propagation,
				)
				if err != nil {
					return nil, exception.NewBatchError(module, fmt.Sprintf("Failed to build Tasklet Step '%s'", id), err, false, false)
				}
				logger.Debugf("Tasklet Step '%s' built.", id)

				// --- Partition Step (Controller) Construction ---
			} else if jslStep.Partition != nil {
				if jslStep.Chunk != nil || jslStep.Tasklet.Ref != "" {
					return nil, exception.NewBatchErrorf(module, "Step '%s' can only define one of chunk, tasklet, or partition", id)
				}

				// Resolve Partitioner properties
				resolvedPartitionerProps, err := resolveComponentRefProperties(resolver, &jslStep.Partition.Partitioner)
				if err != nil {
					return nil, err
				}

				// Update Partitioner ComponentRef with resolved properties
				resolvedPartitionerRef := jslStep.Partition.Partitioner
				resolvedPartitionerRef.Properties = resolvedPartitionerProps

				// Build Partitioner
				partitionerBuilder, ok := partitionerBuilders[resolvedPartitionerRef.Ref]
				if !ok {
					return nil, exception.NewBatchErrorf(module, "Partitioner component builder '%s' not found", resolvedPartitionerRef.Ref)
				}
				// PartitionerBuilder only accepts properties map[string]string
				partitionerInstance, err := partitionerBuilder(resolvedPartitionerRef.Properties)
				if err != nil {
					return nil, exception.NewBatchError(module, fmt.Sprintf("Failed to build Partitioner '%s'", resolvedPartitionerRef.Ref), err, false, false)
				}
				partitioner, isPartitioner := partitionerInstance.(core.Partitioner)
				if !isPartitioner {
					return nil, exception.NewBatchErrorf(module, "Partitioner '%s' not found or is of incorrect type (Expected: core.Partitioner, Actual: %s)", resolvedPartitionerRef.Ref, reflect.TypeOf(partitionerInstance))
				}

				// Temporarily save information needed for PartitionStep construction
				// Hold the JSL definition ID of the Worker Step, as it needs to be built in the second pass
				builtElements[id] = struct {
					IsPartition       bool
					JSLStep           Step
					Partitioner       core.Partitioner
					StepExecListeners []core.StepExecutionListener
					CoreECPromotion   *model.ExecutionContextPromotion
				}{
					IsPartition: true,
					JSLStep:     jslStep,
					Partitioner: partitioner,
					StepExecListeners: stepExecListeners,
					CoreECPromotion: coreECPromotion,
				}
				logger.Debugf("Flow element '%s' (Partition Controller) temporarily saved construction info.", id)

			} else {
				return nil, exception.NewBatchErrorf(module, "Step '%s' must define either chunk, tasklet, or partition", id)
			}
			// Store coreStep here for non-Partition Steps
			if jslStep.Partition == nil {
				builtElements[id] = coreStep
			}
			logger.Debugf("Flow element '%s' (Step) temporarily built.", id)

			// Attempt to unmarshal as Decision (Requires ID and Ref)
		} else if err := yaml.Unmarshal(elementBytes, &jslDecision); err == nil && jslDecision.ID != "" && jslDecision.Ref != "" {
			if jslDecision.ID == "" {
				return nil, exception.NewBatchError(module, fmt.Sprintf("Decision element '%s' requires an ID", id), nil, false, false)
			}
			if jslDecision.ID != id {
				return nil, exception.NewBatchError(module, fmt.Sprintf("Decision ID '%s' does not match map key '%s'", jslDecision.ID, id), nil, false, false)
			}
			if len(jslDecision.Transitions) == 0 {
				return nil, exception.NewBatchError(module, fmt.Sprintf("Decision '%s' has no transition rules defined", id), nil, false, false)
			}

			// Use Ref to find the builder
			builder, found := decisionBuilders[jslDecision.Ref]
			if !found {
				return nil, exception.NewBatchErrorf(module, "Decision Builder '%s' (Referenced by: %s) is not registered", jslDecision.Ref, jslDecision.ID)
			}

			// Resolve Decision properties
			resolvedDecisionProps, err := resolveComponentRefProperties(resolver, &ComponentRef{Ref: jslDecision.Ref, Properties: jslDecision.Properties})
			if err != nil {
				return nil, err
			}

			coreDecision, err := builder(jslDecision.ID, resolvedDecisionProps, resolver)
			if err != nil {
				return nil, exception.NewBatchError(module, fmt.Sprintf("Failed to build Decision '%s'", jslDecision.ID), err, false, false)
			}
			builtElements[id] = coreDecision
			logger.Debugf("Flow element '%s' (ConditionalDecision) temporarily built.", id)

		} else if err := yaml.Unmarshal(elementBytes, &jslSplit); err == nil && jslSplit.ID != "" && len(jslSplit.Steps) > 0 {
			if jslSplit.ID == "" {
				return nil, exception.NewBatchError(module, fmt.Sprintf("Split element '%s' requires an ID", id), nil, false, false)
			}
			if jslSplit.ID != id {
				return nil, exception.NewBatchError(module, fmt.Sprintf("Split ID '%s' does not match map key '%s'", jslSplit.ID, id), nil, false, false)
			}
			if len(jslSplit.Steps) == 0 {
				return nil, exception.NewBatchError(module, fmt.Sprintf("Split '%s' requires at least one step", id), nil, false, false)
			}

			var coreSteps []core.Step
			for _, stepID := range jslSplit.Steps {
				s, ok := builtElements[stepID].(core.Step)
				if !ok {
					// Check if the element exists at all, maybe it's a Decision or Split itself (which is not allowed in steps list)
					if _, exists := jslFlow.Elements[stepID]; !exists {
						return nil, exception.NewBatchErrorf(module, "Element '%s' referenced by Split '%s' not found in flow definition", stepID, jslSplit.ID)
					}
					// Ensure steps within a Split are of type Step, as Partition Steps are not allowed inside a Split.
					if _, isPartition := builtElements[stepID].(struct {
						IsPartition       bool
						JSLStep           Step
						Partitioner       core.Partitioner
						StepExecListeners []core.StepExecutionListener
						CoreECPromotion   *model.ExecutionContextPromotion
					}); isPartition {
						return nil, exception.NewBatchErrorf(module, "Element '%s' referenced by Split '%s' is a Partition Step, which is not allowed inside a Split.", stepID, jslSplit.ID)
					}
					return nil, exception.NewBatchErrorf(module, "Element '%s' referenced by Split '%s' is not of type Step", stepID, jslSplit.ID)
				}
				coreSteps = append(coreSteps, s)
			}
			builder, found := splitBuilders["concreteSplit"]
			if !found {
				return nil, exception.NewBatchErrorf(module, "Split builder 'concreteSplit' is not registered")
			}
			coreSplit, err := builder(jslSplit.ID, coreSteps)
			if err != nil {
				return nil, exception.NewBatchError(module, fmt.Sprintf("Failed to build Split '%s'", jslSplit.ID), err, false, false)
			}

			builtElements[id] = coreSplit
			logger.Debugf("Flow element '%s' (Split) temporarily built.", id)
		} else {
			return nil, exception.NewBatchErrorf(module, "Unknown flow element type or missing required fields: ID '%s', Data: %s", id, string(elementBytes))
		}
	}

	// Second pass: Add elements to flowDef and resolve Split references
	for id, element := range jslFlow.Elements {
		elementBytes, err := yaml.Marshal(element)
		if err != nil {
			return nil, exception.NewBatchError(module, fmt.Sprintf("Failed to re-marshal flow element '%s'", id), err, false, false)
		}

		var (
			jslStep     Step
			jslDecision Decision
			jslSplit    Split
		)

		// Try to unmarshal as Step
		if err := yaml.Unmarshal(elementBytes, &jslStep); err == nil && (jslStep.Reader.Ref != "" || jslStep.Tasklet.Ref != "" || jslStep.Partition != nil) {

			var coreElement core.FlowElement

			// --- Final Construction of Partition Step (Controller) ---
			if jslStep.Partition != nil {
				// 1. Retrieve Partitioner from temporarily saved information
				tempInfo, ok := builtElements[id].(struct {
					IsPartition       bool
					JSLStep           Step
					Partitioner       core.Partitioner
					StepExecListeners []core.StepExecutionListener
					CoreECPromotion   *model.ExecutionContextPromotion
				})
				if !ok || !tempInfo.IsPartition {
					return nil, exception.NewBatchErrorf(module, "Temporary construction information for Partition Step '%s' not found", id)
				}

				// 2. Build Worker Step (Worker Step references another JSL Step definition)
				workerJSLVal, ok := jslFlow.Elements[jslStep.Partition.Step]
				if !ok {
					return nil, exception.NewBatchErrorf(module, "Worker Step '%s' referenced by Partition Step '%s' not found in flow definition", jslStep.Partition.Step, id)
				}

				// Convert Worker Step JSL definition to Step type
				workerBytes, err := yaml.Marshal(workerJSLVal)
				if err != nil {
					return nil, exception.NewBatchError(module, fmt.Sprintf("Failed to marshal Worker Step '%s'", jslStep.Partition.Step), err, false, false)
				}
				var workerStepJSL Step
				if err := yaml.Unmarshal(workerBytes, &workerStepJSL); err != nil {
					return nil, exception.NewBatchError(module, fmt.Sprintf("Failed to parse Worker Step '%s'", jslStep.Partition.Step), err, false, false)
				}

				// Worker Step must be either Chunk or Tasklet
				if workerStepJSL.Chunk == nil && workerStepJSL.Tasklet.Ref == "" {
					return nil, exception.NewBatchErrorf(module, "Worker Step '%s' must define either Chunk or Tasklet", jslStep.Partition.Step)
				}

				// Worker Step listeners should be held by the Worker itself, but here we build the core instance of the Worker Step.
				// Worker Step EC Promotion is also unnecessary (Worker EC is not merged into Controller EC).

				var workerCoreStep core.Step
				if workerStepJSL.Chunk != nil {
					// Resolve Worker Step Reader/Processor/Writer properties
					resolvedWorkerReaderProps, err := resolveComponentRefProperties(resolver, &workerStepJSL.Reader)
					if err != nil {
						return nil, err
					}
					resolvedWorkerProcessorProps, err := resolveComponentRefProperties(resolver, &workerStepJSL.Processor)
					if err != nil {
						return nil, err
					}
					resolvedWorkerWriterProps, err := resolveComponentRefProperties(resolver, &workerStepJSL.Writer)
					if err != nil {
						return nil, err
					}

					resolvedWorkerReaderRef := workerStepJSL.Reader
					resolvedWorkerReaderRef.Properties = resolvedWorkerReaderProps
					resolvedWorkerProcessorRef := workerStepJSL.Processor
					resolvedWorkerProcessorRef.Properties = resolvedWorkerProcessorProps
					resolvedWorkerWriterRef := workerStepJSL.Writer
					resolvedWorkerWriterRef.Properties = resolvedWorkerWriterProps

					r, p, w, err := buildReaderWriterProcessor(module, componentBuilders, cfg, jobRepository, resolver, dbResolver, &resolvedWorkerReaderRef, &resolvedWorkerProcessorRef, &resolvedWorkerWriterRef)
					if err != nil {
						return nil, err
					}

					// Identify the TxManager used by the Worker Chunk Step
					workerDBName, ok := resolvedWorkerWriterProps["targetDBName"]
					if !ok || workerDBName == "" {
						workerDBName, ok = resolvedWorkerWriterProps["database"]
						if !ok || workerDBName == "" {
							workerDBName = "workload" // Default value
						}
					}
					// TxManager is now passed directly from JobFactory
					workerTxManager := txManager

					workerIsolationLevel := workerStepJSL.Chunk.IsolationLevel

					// Worker Step does not inherit retry/skip/listener settings from the Controller, so pass empty lists/default settings
					workerCoreStep, err = stepFactory.CreateChunkStep(
						workerStepJSL.ID,
						r, p, w,
						workerStepJSL.Chunk.ItemCount,
						workerStepJSL.Chunk.CommitInterval,
						&cfg.Surfin.Batch.Retry,
						cfg.Surfin.Batch.ItemRetry,
						cfg.Surfin.Batch.ItemSkip,
						[]core.StepExecutionListener{}, // Worker Step does not have listeners.
						[]core.ItemReadListener{},
						[]core.ItemProcessListener{},
						[]core.ItemWriteListener{},
						[]core.SkipListener{},
						[]core.RetryItemListener{},
						[]core.ChunkListener{},
						nil, // EC Promotion is unnecessary (nil).
						workerIsolationLevel,
						workerStepJSL.Propagation,
						workerTxManager,
					)
				} else if workerStepJSL.Tasklet.Ref != "" {
					taskletBuilder, ok := componentBuilders[workerStepJSL.Tasklet.Ref]
					if !ok {
						return nil, exception.NewBatchErrorf(module, "Worker Tasklet builder '%s' not found", workerStepJSL.Tasklet.Ref)
					}

					// Resolve Worker Tasklet properties
					resolvedWorkerTaskletProps, err := resolveComponentRefProperties(resolver, &workerStepJSL.Tasklet)
					if err != nil {
						return nil, err
					}

					taskletInstance, err := taskletBuilder(cfg, jobRepository, resolver, dbResolver, resolvedWorkerTaskletProps)
					if err != nil {
						return nil, exception.NewBatchError(module, fmt.Sprintf("Failed to build Worker Tasklet '%s'", workerStepJSL.Tasklet.Ref), err, false, false)
					}
					t, isTasklet := taskletInstance.(core.Tasklet)
					if !isTasklet {
						return nil, exception.NewBatchErrorf(module, "Worker Tasklet '%s' is of incorrect type", workerStepJSL.Tasklet.Ref)
					}
					workerCoreStep, err = stepFactory.CreateTaskletStep(
						workerStepJSL.ID,
						t,
						[]core.StepExecutionListener{}, // Worker Step does not have listeners
						nil, // EC Promotion is unnecessary
						workerStepJSL.IsolationLevel,
						workerStepJSL.Propagation,
					)
				}

				if err != nil {
					return nil, exception.NewBatchError(module, fmt.Sprintf("Failed to build Worker Step '%s'", jslStep.Partition.Step), err, false, false)
				}

				// 3. Final construction of Partition Step (Controller)
				coreElement, err = stepFactory.CreatePartitionStep(
					jslStep.ID,
					tempInfo.Partitioner,
					workerCoreStep,
					jslStep.Partition.GridSize,
					jobRepository,
					tempInfo.StepExecListeners,
					tempInfo.CoreECPromotion,
				)
				if err != nil {
					return nil, exception.NewBatchError(module, fmt.Sprintf("Failed to build Partition Step '%s'", id), err, false, false)
				}
				logger.Debugf("Partition Step '%s' (Controller) built.", id)

			} else {
				// Chunk Step or Tasklet Step case
				coreElement = builtElements[id].(core.FlowElement)
			}

			// Add element and transitions
			err = flowDef.AddElement(id, coreElement)
			if err != nil {
				return nil, exception.NewBatchError(module, fmt.Sprintf("Failed to add step '%s' to flow", id), err, false, false)
			}
			for _, transition := range jslStep.Transitions {
				if err := validateTransition(id, transition, jslFlow.Elements); err != nil {
					return nil, err
				}
				flowDef.AddTransitionRule(id, transition.On, transition.To, transition.End, transition.Fail, transition.Stop)
			}
			logger.Debugf("Step '%s' added to flow.", id)

			// Try to unmarshal as Decision
		} else if err := yaml.Unmarshal(elementBytes, &jslDecision); err == nil && jslDecision.ID != "" && jslDecision.Ref != "" {
			coreElement := builtElements[id].(core.FlowElement)
			err := flowDef.AddElement(id, coreElement)
			if err != nil {
				return nil, exception.NewBatchError(module, fmt.Sprintf("Failed to add Decision '%s' to flow", id), err, false, false)
			}
			for _, transition := range jslDecision.Transitions {
				if err := validateTransition(id, transition, jslFlow.Elements); nil != err {
					return nil, err
				}
				flowDef.AddTransitionRule(id, transition.On, transition.To, transition.End, transition.Fail, transition.Stop)
			}
			logger.Debugf("Decision '%s' added to flow.", id)

			// Try to unmarshal as Split
		} else if err := yaml.Unmarshal(elementBytes, &jslSplit); err == nil && jslSplit.ID != "" && len(jslSplit.Steps) > 0 {
			coreSplit := builtElements[id].(core.FlowElement)
			err = flowDef.AddElement(id, coreSplit)
			if err != nil {
				return nil, exception.NewBatchError(module, fmt.Sprintf("Failed to add Split '%s' to flow", id), err, false, false)
			}
			for _, transition := range jslSplit.Transitions {
				if err := validateTransition(id, transition, jslFlow.Elements); err != nil {
					return nil, err
				}
				flowDef.AddTransitionRule(id, transition.On, transition.To, transition.End, transition.Fail, transition.Stop)
			}
			logger.Debugf("Split '%s' added to flow.", id)
		} else {
			return nil, exception.NewBatchErrorf(module, "Unknown flow element type or missing required fields: ID '%s', Data: %s", id, string(elementBytes))
		}
	}

	return flowDef, nil
}

// buildReaderWriterProcessor is a helper function to build instances of ItemReader, ItemProcessor, and ItemWriter.
func buildReaderWriterProcessor(
	module string,
	componentBuilders map[string]ComponentBuilder,
	cfg *config.Config,
	jobRepository job.JobRepository,
	resolver core.ExpressionResolver,
	dbResolver core.DBConnectionResolver,
	readerRef *ComponentRef,
	processorRef *ComponentRef,
	writerRef *ComponentRef,
) (core.ItemReader[any], core.ItemProcessor[any, any], core.ItemWriter[any], error) {
	var r core.ItemReader[any]
	var p core.ItemProcessor[any, any]
	var w core.ItemWriter[any]

	readerBuilder, ok := componentBuilders[readerRef.Ref]
	if !ok {
		return nil, nil, nil, exception.NewBatchErrorf(module, "Reader builder '%s' not found", readerRef.Ref)
	}
	readerInstance, err := readerBuilder(cfg, jobRepository, resolver, dbResolver, readerRef.Properties)
	if err != nil {
		return nil, nil, nil, exception.NewBatchError(module, fmt.Sprintf("Failed to build Reader '%s'", readerRef.Ref), err, false, false)
	}
	r, isReader := readerInstance.(core.ItemReader[any])
	if !isReader {
		return nil, nil, nil, exception.NewBatchErrorf(module, "Reader '%s' is of incorrect type (Expected: core.ItemReader[any], Actual: %s)", readerRef.Ref, reflect.TypeOf(readerInstance))
	}

	processorBuilder, ok := componentBuilders[processorRef.Ref]
	if !ok {
		return nil, nil, nil, exception.NewBatchErrorf(module, "Processor builder '%s' not found", processorRef.Ref)
	}
	processorInstance, err := processorBuilder(cfg, jobRepository, resolver, dbResolver, processorRef.Properties)
	if err != nil {
		return nil, nil, nil, exception.NewBatchError(module, fmt.Sprintf("Failed to build Processor '%s'", processorRef.Ref), err, false, false)
	}
	p, isProcessor := processorInstance.(core.ItemProcessor[any, any])
	if !isProcessor {
		return nil, nil, nil, exception.NewBatchErrorf(module, "Processor '%s' is of incorrect type (Expected: core.ItemProcessor[any, any], Actual: %s)", processorRef.Ref, reflect.TypeOf(processorInstance))
	}

	writerBuilder, ok := componentBuilders[writerRef.Ref]
	if !ok {
		return nil, nil, nil, exception.NewBatchErrorf(module, "Writer builder '%s' not found", writerRef.Ref)
	}
	writerInstance, err := writerBuilder(cfg, jobRepository, resolver, dbResolver, writerRef.Properties)
	if err != nil {
		return nil, nil, nil, exception.NewBatchError(module, fmt.Sprintf("Failed to build Writer '%s'", writerRef.Ref), err, false, false)
	}
	w, isWriter := writerInstance.(core.ItemWriter[any])
	if !isWriter {
		return nil, nil, nil, exception.NewBatchErrorf(module, "Writer '%s' is of incorrect type (Expected: core.ItemWriter[any], Actual: %s)", writerRef.Ref, reflect.TypeOf(writerInstance))
	}

	return r, p, w, nil
}

// validateTransition validates a single transition rule.
func validateTransition(fromElementID string, t Transition, allElements map[string]interface{}) error {
	if t.On == "" {
		return exception.NewBatchError("jsl_converter", fmt.Sprintf("Transition rule for flow element '%s' is missing 'on'", fromElementID), nil, false, false)
	}

	// Check mutual exclusivity of End, Fail, Stop, To
	exclusiveCount := 0
	if t.End {
		exclusiveCount++
	}
	if t.Fail {
		exclusiveCount++
	}
	if t.Stop {
		exclusiveCount++
	}
	if t.To != "" {
		exclusiveCount++
	}

	if exclusiveCount == 0 {
		return exception.NewBatchError("jsl_converter", fmt.Sprintf("Transition rule for flow element '%s' (on: '%s') must define one of 'to', 'end', 'fail', or 'stop'", fromElementID, t.On), nil, false, false)
	}
	if exclusiveCount > 1 {
		return exception.NewBatchError("jsl_converter", fmt.Sprintf("Transition rule for flow element '%s' (on: '%s') defines multiple exclusive attributes ('to', 'end', 'fail', 'stop')", fromElementID, t.On), nil, false, false)
	}

	// If 'to' is specified, ensure the target element exists
	if t.To != "" {
		if _, ok := allElements[t.To]; !ok {
			return exception.NewBatchError("jsl_converter", fmt.Sprintf("Target element '%s' specified by 'to' in transition rule (on: '%s') for flow element '%s' not found", t.To, t.On, fromElementID), nil, false, false)
		}
	}
	return nil
}
