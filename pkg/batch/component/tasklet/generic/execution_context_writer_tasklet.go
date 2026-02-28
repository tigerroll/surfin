package generic

import (
	"context"
	"strconv"
	"strings"

	// Package generic provides general-purpose tasklet implementations.
	// These tasklets are designed to be reusable across various batch jobs.

	port "github.com/tigerroll/surfin/pkg/batch/core/application/port"
	model "github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	exception "github.com/tigerroll/surfin/pkg/batch/support/util/exception"
	logger "github.com/tigerroll/surfin/pkg/batch/support/util/logger"
)

// ExecutionContextWriterTasklet is a [port.Tasklet] that writes values specified in JSL properties to the [model.ExecutionContext].
// It is primarily used for testing and debugging purposes, allowing dynamic injection of values into the execution context.
type ExecutionContextWriterTasklet struct {
	id         string
	ec         model.ExecutionContext
	properties map[string]string
}

// NewExecutionContextWriterTasklet creates a new [ExecutionContextWriterTasklet] instance.
//
// Parameters:
//
//	id: The unique identifier for this tasklet.
//	properties: A map of properties where keys are in "key.type" format (e.g., "count.int", "name.string")
//	            and values are string representations to be written to the ExecutionContext.
//
// Returns:
//
//	port.Tasklet: A new instance of [ExecutionContextWriterTasklet].
func NewExecutionContextWriterTasklet(id string, properties map[string]string) port.Tasklet {
	return &ExecutionContextWriterTasklet{
		id:         id,
		ec:         model.NewExecutionContext(),
		properties: properties,
	}
}

// Open initializes the tasklet and prepares any necessary resources.
// For [ExecutionContextWriterTasklet], no specific resources need to be opened.
// The [model.StepExecution] is provided for context, but its ExecutionContext is expected
// to be set via [SetExecutionContext] before [Open] is called.
//
// Parameters:
//
//	ctx: The context for the operation.
//	stepExecution: The current [model.StepExecution] instance.
//
// Returns:
//
//	error: An error if initialization fails.
func (t *ExecutionContextWriterTasklet) Open(ctx context.Context, stepExecution *model.StepExecution) error {
	logger.Debugf("ExecutionContextWriterTasklet '%s' Open called.", t.id)
	// ExecutionContext is already set by SetExecutionContext before Open is called.
	return nil
}

// Execute writes values to the [model.ExecutionContext] based on the tasklet's properties.
// Properties are expected to be in "key.type" format, where 'type' specifies the target data type
// (e.g., "string", "int", "float", "bool").
// Example properties:
//   - "count.int": "10"
//   - "name.string": "test"
//   - "flag.bool": "true"
//
// Parameters:
//
//	ctx: The context for the operation.
//	stepExecution: The current [model.StepExecution] instance.
//
// Returns:
//
//	model.ExitStatus: The exit status of the tasklet (e.g., [model.ExitStatusCompleted]).
//	error: An error if property parsing or type conversion fails.
func (t *ExecutionContextWriterTasklet) Execute(ctx context.Context, stepExecution *model.StepExecution) (model.ExitStatus, error) {
	logger.Infof("ExecutionContextWriterTasklet '%s' executing. Writing %d properties to ExecutionContext.", t.id, len(t.properties))

	for keyWithType, valueStr := range t.properties {
		parts := strings.SplitN(keyWithType, ".", 2)
		if len(parts) != 2 {
			logger.Warnf("Property key '%s' is not in 'key.type' format. Skipping.", keyWithType)
			continue
		}

		key := parts[0]
		typeStr := strings.ToLower(parts[1])

		var value interface{}
		var err error

		switch typeStr {
		case "string":
			value = valueStr
		case "int":
			value, err = strconv.Atoi(valueStr)
		case "float", "float64":
			value, err = strconv.ParseFloat(valueStr, 64)
		case "bool":
			value, err = strconv.ParseBool(valueStr)
		default:
			logger.Warnf("Unknown type '%s' for key '%s'. Treating as string.", typeStr, key)
			value = valueStr
		}

		if err != nil {
			return model.ExitStatusFailed, exception.NewBatchErrorf(t.id, "Failed to convert value '%s' to type '%s' for key '%s': %w", valueStr, typeStr, key, err)
		}

		t.ec.Put(key, value)
		logger.Debugf("Wrote to EC: %s = %v (Type: %s)", key, value, typeStr)
	}

	// Reflect the contents of the ExecutionContext to the StepExecution.
	stepExecution.ExecutionContext = t.ec

	return model.ExitStatusCompleted, nil
}

// Close releases any resources held by the tasklet.
// For [ExecutionContextWriterTasklet], there are no specific resources to close, so it always returns nil.
//
// Parameters:
//
//	ctx: The context for the operation.
//	stepExecution: The current [model.StepExecution] instance.
//
// Returns:
//
//	error: An error if closing fails (always nil for this tasklet).
func (t *ExecutionContextWriterTasklet) Close(ctx context.Context, stepExecution *model.StepExecution) error {
	return nil
}

// SetExecutionContext sets the [model.ExecutionContext] for the tasklet.
// This method is called by the framework to provide the tasklet with its current execution context.
//
// Parameters:
//
//	ec: The [model.ExecutionContext] to set.
func (t *ExecutionContextWriterTasklet) SetExecutionContext(ec model.ExecutionContext) {
	t.ec = ec
}

// GetExecutionContext retrieves the current [model.ExecutionContext] from the tasklet.
//
// Returns:
//
//	model.ExecutionContext: The current [model.ExecutionContext].
func (t *ExecutionContextWriterTasklet) GetExecutionContext() model.ExecutionContext {
	return t.ec
}

// Verify that [ExecutionContextWriterTasklet] satisfies the [port.Tasklet] interface.
var _ port.Tasklet = (*ExecutionContextWriterTasklet)(nil)
