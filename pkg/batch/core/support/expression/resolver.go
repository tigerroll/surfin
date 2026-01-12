// pkg/batch/core/support/expression/resolver.go
package expression

import (
	"context"
	"fmt"
	port "github.com/tigerroll/surfin/pkg/batch/core/application/port"
	model "github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	"github.com/tigerroll/surfin/pkg/batch/support/util/logger"
	"regexp"
	"strings"
)

// DefaultExpressionResolver is an implementation that resolves dynamic property expressions based on the execution context.
type DefaultExpressionResolver struct {
	// In the future, a more complex expression evaluation engine (e.g., Goja, CEL) might be injected here.
}

// NewDefaultExpressionResolver creates a new instance of DefaultExpressionResolver.
func NewDefaultExpressionResolver() port.ExpressionResolver {
	return &DefaultExpressionResolver{}
}

// Regular expression pattern: Captures the form #{...}
var expressionPattern = regexp.MustCompile(`\#\{(.+?)\}`)

// Resolve resolves the given expression string and returns the resulting string.
func (r *DefaultExpressionResolver) Resolve(ctx context.Context, expression string, jobExecution *model.JobExecution, stepExecution *model.StepExecution) (string, error) {
	if !expressionPattern.MatchString(expression) {
		// Return as is if not an expression
		return expression, nil
	}

	// Consider the possibility of multiple #{...} and replace all occurrences
	resolvedString := expressionPattern.ReplaceAllStringFunc(expression, func(match string) string {
		// match is "#{...}"
		innerExpression := strings.TrimSpace(match[2 : len(match)-1]) // Trim the "..." part

		// If JobExecution does not exist, JobParameters cannot be resolved either
		if jobExecution == nil {
			logger.Warnf("ExpressionResolver: Skipping resolution of dynamic expression '%s' because JobExecution is nil.", innerExpression)
			return match // Return the original string if resolution failed
		}

		// 1. Attempt to resolve JobParameters
		if val, err := r.resolveJobParameters(innerExpression, jobExecution); err == nil {
			return val
		}

		// 2. Attempt to resolve JobExecutionContext
		if val, err := r.resolveJobExecutionContext(innerExpression, jobExecution); err == nil {
			return val
		}

		// 3. Attempt to resolve StepExecution (only if StepExecution exists)
		if stepExecution != nil {
			if val, err := r.resolveStepExecution(innerExpression, stepExecution); err == nil {
				return val
			}
		}

		logger.Warnf("ExpressionResolver: Unknown expression or key not found: %s", innerExpression)
		return match // Return the original expression if no pattern matches
	})

	return resolvedString, nil
}

// Resolves the format jobParameters['key']
var jobParamsPattern = regexp.MustCompile(`^jobParameters\['(.+?)'\]$`)

func (r *DefaultExpressionResolver) resolveJobParameters(expr string, jobExecution *model.JobExecution) (string, error) {
	matches := jobParamsPattern.FindStringSubmatch(expr)
	if len(matches) != 2 {
		return "", fmt.Errorf("pattern mismatch")
	}
	key := matches[1]

	if val, ok := jobExecution.Parameters.Params[key]; ok {
		return fmt.Sprintf("%v", val), nil
	}
	return "", fmt.Errorf("key '%s' not found in JobParameters", key)
}

// Resolves the format jobExecutionContext['key']
var jobExecCtxPattern = regexp.MustCompile(`^jobExecutionContext\['(.+?)'\]$`)

func (r *DefaultExpressionResolver) resolveJobExecutionContext(expr string, jobExecution *model.JobExecution) (string, error) {
	matches := jobExecCtxPattern.FindStringSubmatch(expr)
	if len(matches) != 2 {
		return "", fmt.Errorf("pattern mismatch")
	}
	key := matches[1]

	if val, ok := jobExecution.ExecutionContext[key]; ok {
		return fmt.Sprintf("%v", val), nil
	}
	return "", fmt.Errorf("key '%s' not found in JobExecutionContext", key)
}

// Resolves formats like stepExecution.readCount (outside the scope of H.1, but defined as a placeholder for future extension)
var stepExecPattern = regexp.MustCompile(`^stepExecution\.(.+)$`)

func (r *DefaultExpressionResolver) resolveStepExecution(expr string, stepExecution *model.StepExecution) (string, error) {
	// For H.1, StepExecution property resolution is complex, so it is currently unsupported.
	return "", fmt.Errorf("StepExecution property resolution is currently not supported: %s", expr)
}
