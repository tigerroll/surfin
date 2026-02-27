// Package step provides implementations for various batch steps, including tasklets.
// It contains the HelloWorldTasklet, a simple tasklet for demonstration purposes.
package step

import (
	// Standard library imports
	"context"
	"fmt"

	port "github.com/tigerroll/surfin/pkg/batch/core/application/port" // Re-add for interface assertion
	model "github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	configbinder "github.com/tigerroll/surfin/pkg/batch/support/util/configbinder"
	"github.com/tigerroll/surfin/pkg/batch/support/util/exception"
	"github.com/tigerroll/surfin/pkg/batch/support/util/logger"
)

// HelloWorldTaskletConfig defines the configuration parameters for the HelloWorldTasklet.
// These parameters are typically bound from JSL (Job Specification Language) properties.
type HelloWorldTaskletConfig struct {
	// Message is the string to be printed by the tasklet, configured via JSL properties.
	Message string `yaml:"message"`
}

// HelloWorldTasklet is a simple implementation of a batch tasklet that prints a configurable message to the console.
// It implements the port.Tasklet interface.
type HelloWorldTasklet struct {
	// config holds the tasklet's configuration parameters.
	config *HelloWorldTaskletConfig
	// executionContext stores the tasklet's internal state, managed by the framework.
	executionContext model.ExecutionContext
	// stepName is the unique identifier for this step, required by the port.Tasklet interface.
	stepName string
}

// NewHelloWorldTasklet creates a new instance of HelloWorldTasklet.
//
// It binds the provided properties map to the tasklet's configuration struct.
//
// Parameters:
//
//	properties: A map of string properties defined in the JSL for this tasklet.
//
// Returns:
//
//	*HelloWorldTasklet: A new instance of the tasklet.
//	error: An error if property binding fails or required properties are missing.
func NewHelloWorldTasklet(properties map[string]string) (*HelloWorldTasklet, error) {
	taskletCfg := &HelloWorldTaskletConfig{}

	if err := configbinder.BindProperties(properties, taskletCfg); err != nil {
		return nil, exception.NewBatchError("hello_world_tasklet", "Failed to bind properties", err, false, false)
	}
	// Validate that the 'message' property is provided.
	if taskletCfg.Message == "" {
		return nil, fmt.Errorf("message property is required for HelloWorldTasklet")
	}

	return &HelloWorldTasklet{
		config:           taskletCfg,
		executionContext: model.NewExecutionContext(),
		stepName:         "helloWorldStep", // Matches the JSL ID for this specific tasklet.
	}, nil
}

// StepName returns the unique name of the step.
// This method is required to implement the port.Tasklet interface.
func (t *HelloWorldTasklet) StepName() string {
	return t.stepName
}

// GetExecutionContextPromotion returns the execution context promotion rules for this step.
//
// This tasklet does not promote any specific keys to the job level, so it returns nil.
func (t *HelloWorldTasklet) GetExecutionContextPromotion() *model.ExecutionContextPromotion {
	return nil
}

// Execute runs the main business logic of the Tasklet.
//
// Parameters:
//
//	ctx: The context for the operation.
//	stepExecution: The current StepExecution instance.
//
// Returns:
//
//	model.ExitStatus: The determined ExitStatus of the step.
//	error: An error encountered during execution.
func (t *HelloWorldTasklet) Execute(ctx context.Context, stepExecution *model.StepExecution) (model.ExitStatus, error) {
	select {
	case <-ctx.Done():
		return model.ExitStatusFailed, ctx.Err()
	default:
	}
	// Log the message configured for this tasklet.
	logger.Debugf("HelloWorldTasklet: Attempting to log message: '%s'", t.config.Message)
	logger.Infof("HelloWorldTasklet: %s", t.config.Message)
	return model.ExitStatusCompleted, nil
}

// Open initializes any resources required by the Tasklet.
// For HelloWorldTasklet, there are no specific resources to open.
func (t *HelloWorldTasklet) Open(ctx context.Context, ec model.ExecutionContext) error {
	logger.Debugf("HelloWorldTasklet: Open method called.")
	return nil
}

// Close releases any resources held by the Tasklet.
// For HelloWorldTasklet, there are no specific resources to close.
func (t *HelloWorldTasklet) Close(ctx context.Context) error {
	logger.Debugf("HelloWorldTasklet: Close method called.")
	return nil
}

// SetExecutionContext sets the ExecutionContext for the Tasklet.
// This method allows the framework to inject or restore the tasklet's state.
func (t *HelloWorldTasklet) SetExecutionContext(ctx context.Context, ec model.ExecutionContext) error {
	logger.Debugf("HelloWorldTasklet: SetExecutionContext method called.")
	t.executionContext = ec
	return nil
}

// GetExecutionContext retrieves the current ExecutionContext of the Tasklet.
// This method allows the framework to persist the tasklet's state.
func (t *HelloWorldTasklet) GetExecutionContext(ctx context.Context) (model.ExecutionContext, error) {
	logger.Debugf("HelloWorldTasklet: GetExecutionContext method called.")
	return t.executionContext, nil
}

// HelloWorldTasklet confirms that it implements the port.Tasklet interface.
// This line will cause a compile error if the interface is not fully implemented.
var _ port.Tasklet = (*HelloWorldTasklet)(nil)
