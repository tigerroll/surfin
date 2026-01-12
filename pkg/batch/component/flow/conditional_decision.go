package flow

import (
	"context"
	"fmt"

	port "surfin/pkg/batch/core/application/port"
	model "surfin/pkg/batch/core/domain/model"
	logger "surfin/pkg/batch/support/util/logger"
)

// ConditionalDecision is a more flexible implementation of the Decision interface.
// It dynamically determines the ExitStatus based on properties passed from JSL.
type ConditionalDecision struct {
	id            string
	properties    map[string]string
	conditionKey  string
	expectedValue string
	defaultStatus model.ExitStatus
	resolver      port.ExpressionResolver
}

// NewConditionalDecision creates a new instance of ConditionalDecision.
func NewConditionalDecision(id string) *ConditionalDecision {
	return &ConditionalDecision{
		id:            id,
		properties:    make(map[string]string),
		defaultStatus: model.ExitStatusFailed,
	}
}

// SetResolver sets the ExpressionResolver.
func (d *ConditionalDecision) SetResolver(resolver port.ExpressionResolver) {
	d.resolver = resolver
}

// SetProperties sets the properties injected from JSL.
func (d *ConditionalDecision) SetProperties(properties map[string]string) {
	d.properties = properties
	if key, ok := properties["conditionKey"]; ok {
		d.conditionKey = key
	}
	if val, ok := properties["expectedValue"]; ok {
		d.expectedValue = val
	}
	if statusStr, ok := properties["defaultStatus"]; ok {
		d.defaultStatus = model.ExitStatus(statusStr)
	}
	logger.Debugf("ConditionalDecision '%s': Properties set. conditionKey='%s', expectedValue='%s', defaultStatus='%s'",
		d.id, d.conditionKey, d.expectedValue, d.defaultStatus)
}

// Decide determines the ExitStatus based on the value in the ExecutionContext.
func (d *ConditionalDecision) Decide(ctx context.Context, jobExecution *model.JobExecution, jobParameters model.JobParameters) (model.ExitStatus, error) {
	if d.resolver == nil {
		return model.ExitStatusFailed, fmt.Errorf("ExpressionResolver is not set for ConditionalDecision '%s'", d.id)
	}

	logger.Debugf("ConditionalDecision '%s' invoked. conditionKey='%s', expectedValue='%s'", d.id, d.conditionKey, d.expectedValue)

	if d.conditionKey == "" {
		// If a static exit.status property is set, prioritize it.
		if status, ok := d.properties["exit.status"]; ok {
			logger.Debugf("Decision '%s' determined exit status '%s' from static property.", d.id, status)
			return model.ExitStatus(status), nil
		}
		logger.Warnf("ConditionalDecision '%s': conditionKey is not set. Returning default status '%s'.", d.id, d.defaultStatus)
		return d.defaultStatus, nil
	}

	// 1. Resolve conditionKey (may contain JobParameters or JobExecutionContext expressions)
	resolvedConditionKey, err := d.resolver.Resolve(ctx, d.conditionKey, jobExecution, nil)
	if err != nil {
		logger.Errorf("Decision '%s': Failed to resolve conditionKey expression '%s': %v", d.id, d.conditionKey, err)
		return model.ExitStatusFailed, err
	}

	// First, check JobExecutionContext
	actualValue, ok := jobExecution.ExecutionContext.GetNested(resolvedConditionKey)
	if !ok {
		// Next, check StepExecutionContext (ExecutionContext of the current step)
		// However, since Decision is not a step, it usually refers to JobExecutionContext.
		// Here, we simply look at JobExecutionContext only.
		logger.Warnf("ConditionalDecision '%s': Key '%s' not found in JobExecutionContext. Returning default status '%s'.", d.id, resolvedConditionKey, d.defaultStatus)
		return d.defaultStatus, nil
	}

	resolvedExpectedValue, err := d.resolver.Resolve(ctx, d.expectedValue, jobExecution, nil)
	if err != nil {
		logger.Errorf("Decision '%s': Failed to resolve expectedValue expression '%s': %v", d.id, d.expectedValue, err)
		return model.ExitStatusFailed, err
	}

	// Convert the retrieved value to a string for comparison
	actualValueStr := fmt.Sprintf("%v", actualValue) // ExecutionContext values are always compared as strings
	if actualValueStr == resolvedExpectedValue {
		logger.Infof("ConditionalDecision '%s': Condition matched ('%s' == '%s'). Returning ExitStatusCompleted.", d.id, actualValueStr, resolvedExpectedValue)
		return model.ExitStatusCompleted, nil
	}

	logger.Infof("ConditionalDecision '%s': Condition did not match ('%s' != '%s'). Returning default status '%s'.", d.id, actualValueStr, resolvedExpectedValue, d.defaultStatus)
	return d.defaultStatus, nil
}

// DecisionName returns the name of the Decision.
func (d *ConditionalDecision) DecisionName() string {
	return d.id
}

// ID returns the ID of the Decision.
func (d *ConditionalDecision) ID() string {
	return d.id
}

// Verify that ConditionalDecision implements the core.Decision interface
var _ port.Decision = (*ConditionalDecision)(nil)
