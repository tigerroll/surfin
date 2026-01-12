package generic

import (
	"context"
	"strconv"
	"strings"

	port "github.com/tigerroll/surfin/pkg/batch/core/application/port"
	model "github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	exception "github.com/tigerroll/surfin/pkg/batch/support/util/exception"
	logger "github.com/tigerroll/surfin/pkg/batch/support/util/logger"
)

// ExecutionContextWriterTasklet is a [port.Tasklet] that writes values specified in JSL properties to the [model.ExecutionContext].
// It is primarily used for testing and debugging purposes.
type ExecutionContextWriterTasklet struct {
	id string
	ec model.ExecutionContext
	properties map[string]string
}

// NewExecutionContextWriterTasklet creates a new instance of [ExecutionContextWriterTasklet].
func NewExecutionContextWriterTasklet(id string, properties map[string]string) port.Tasklet {
	return &ExecutionContextWriterTasklet{
		id: id,
		ec: model.NewExecutionContext(),
		properties: properties,
	}
}

// Execute writes values to the [model.ExecutionContext] based on the tasklet's properties.
// Properties are expected to be in "key.type=value" format.
// Example: "count.int=10", "name.string=test", "flag.bool=true".
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
func (t *ExecutionContextWriterTasklet) Close(ctx context.Context) error {
	return nil
}

// SetExecutionContext sets the [model.ExecutionContext] for the tasklet.
func (t *ExecutionContextWriterTasklet) SetExecutionContext(ctx context.Context, ec model.ExecutionContext) error {
	t.ec = ec
	return nil
}

// GetExecutionContext retrieves the current [model.ExecutionContext] from the tasklet.
func (t *ExecutionContextWriterTasklet) GetExecutionContext(ctx context.Context) (model.ExecutionContext, error) {
	return t.ec, nil
}

// Verify that [ExecutionContextWriterTasklet] satisfies the [port.Tasklet] interface.
var _ port.Tasklet = (*ExecutionContextWriterTasklet)(nil)
