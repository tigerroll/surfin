package flow

import (
	"context"
	"fmt"

	port "github.com/tigerroll/surfin/pkg/batch/core/application/port"
	model "github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	logger "github.com/tigerroll/surfin/pkg/batch/support/util/logger"
)

// ConditionalDecision is a more flexible implementation of the Decision interface.
// It dynamically determines the ExitStatus based on properties passed from JSL.
type ConditionalDecision struct {
	id            string
	properties    map[string]interface{}
	conditionKey  string
	expectedValue string
	defaultStatus model.ExitStatus
	resolver      port.ExpressionResolver
}

// NewConditionalDecision creates a new instance of ConditionalDecision.
func NewConditionalDecision(id string) *ConditionalDecision {
	return &ConditionalDecision{
		id:            id,
		properties:    make(map[string]interface{}),
		defaultStatus: model.ExitStatusFailed,
	}
}

// SetResolver sets the ExpressionResolver.
func (d *ConditionalDecision) SetResolver(resolver port.ExpressionResolver) {
	d.resolver = resolver
}

// SetProperties sets the properties injected from JSL.
func (d *ConditionalDecision) SetProperties(properties map[string]interface{}) {
	d.properties = properties
	if key, ok := properties["conditionKey"]; ok {
		if s, isString := key.(string); isString {
			d.conditionKey = s
		} else {
			logger.Warnf("ConditionalDecision '%s': 'conditionKey' property is not a string, ignoring.", d.id)
		}
	}
	if val, ok := properties["expectedValue"]; ok {
		if s, isString := val.(string); isString {
			d.expectedValue = s
		} else {
			logger.Warnf("ConditionalDecision '%s': 'expectedValue' property is not a string, ignoring.", d.id)
		}
	}
	if statusStr, ok := properties["defaultStatus"]; ok {
		if s, isString := statusStr.(string); isString {
			d.defaultStatus = model.ExitStatus(s)
		} else {
			logger.Warnf("ConditionalDecision '%s': 'defaultStatus' property is not a string, ignoring.", d.id)
		}
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
		if statusVal, ok := d.properties["exit.status"]; ok {
			if statusStr, isString := statusVal.(string); isString {
				logger.Debugf("Decision '%s' determined exit status '%s' from static property.", d.id, statusStr)
				return model.ExitStatus(statusStr), nil
			} else {
				logger.Warnf("ConditionalDecision '%s': 'exit.status' property is not a string. Returning default status '%s'.", d.id, d.defaultStatus)
			}
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
