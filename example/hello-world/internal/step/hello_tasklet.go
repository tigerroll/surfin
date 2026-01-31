// Package step provides implementations for various batch steps, including tasklets.
// It contains the HelloWorldTasklet, a simple tasklet for demonstration purposes.
package step

import (
	// Standard library imports
	"context"
	"fmt"

	model "github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	configbinder "github.com/tigerroll/surfin/pkg/batch/support/util/configbinder"
	"github.com/tigerroll/surfin/pkg/batch/support/util/exception"
	"github.com/tigerroll/surfin/pkg/batch/support/util/logger"
)

// HelloWorldTaskletConfig is a struct used to bind properties passed from JSL.
// It defines the configuration parameters for the HelloWorldTasklet.
type HelloWorldTaskletConfig struct {
	Message string `yaml:"message"` // Corresponds to properties.message in JSL.
}

// HelloWorldTasklet is a simple implementation of a Tasklet.
// It prints a configurable message to the console.
type HelloWorldTasklet struct {
	config           *HelloWorldTaskletConfig // Configuration for the tasklet.
	executionContext model.ExecutionContext   // ExecutionContext to hold the internal state of the Tasklet.
}

// NewHelloWorldTasklet creates a new instance of HelloWorldTasklet.
// It binds the provided properties to the tasklet's configuration.
func NewHelloWorldTasklet(properties map[string]string) (*HelloWorldTasklet, error) {
	taskletCfg := &HelloWorldTaskletConfig{}

	if err := configbinder.BindProperties(properties, taskletCfg); err != nil { // Binds JSL properties to the struct.
		// isSkippable and isRetryable are set to false.
		return nil, exception.NewBatchError("hello_world_tasklet", "Failed to bind properties", err, false, false)
	}

	if taskletCfg.Message == "" {
		return nil, fmt.Errorf("message property is required for HelloWorldTasklet")
	}

	return &HelloWorldTasklet{
		config:           taskletCfg,
		executionContext: model.NewExecutionContext(),
	}, nil
}

// Execute runs the main business logic of the Tasklet.
// It logs the configured message to the console.
func (t *HelloWorldTasklet) Execute(ctx context.Context, stepExecution *model.StepExecution) (model.ExitStatus, error) {
	select {
	case <-ctx.Done():
		return model.ExitStatusFailed, ctx.Err()
	default:
	}
	// Add a debug log to confirm the message content.
	logger.Debugf("HelloWorldTasklet: Attempting to log message: '%s'", t.config.Message) // Log the message being processed.
	logger.Infof("HelloWorldTasklet: %s", t.config.Message)
	return model.ExitStatusCompleted, nil
}

// Close releases any resources held by the Tasklet.
// For HelloWorldTasklet, there are no specific resources to close.
func (t *HelloWorldTasklet) Close(ctx context.Context) error {
	logger.Debugf("HelloWorldTasklet: Close called.")
	return nil
}

// SetExecutionContext sets the ExecutionContext for the Tasklet.
// This method allows the framework to inject or restore the tasklet's state.
func (t *HelloWorldTasklet) SetExecutionContext(ctx context.Context, ec model.ExecutionContext) error {
	t.executionContext = ec
	return nil
}

// GetExecutionContext retrieves the current ExecutionContext of the Tasklet.
// This method allows the framework to persist the tasklet's state.
func (t *HelloWorldTasklet) GetExecutionContext(ctx context.Context) (model.ExecutionContext, error) {
	return t.executionContext, nil
}
