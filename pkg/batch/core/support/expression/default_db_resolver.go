package expression

import (
	"context"
	port "github.com/tigerroll/surfin/pkg/batch/core/application/port"
	model "github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	"github.com/tigerroll/surfin/pkg/batch/support/util/logger"
	"strings"
)

// DefaultDBConnectionResolver is the default implementation of DBConnectionResolver.
// Currently, it always returns the default name (typically "workload").
// In the future, logic for dynamic resolution based on JobParameters or ExecutionContext will be added here.
type DefaultDBConnectionResolver struct {
	resolver port.ExpressionResolver
}

// NewDefaultDBConnectionResolver creates a new instance of DefaultDBConnectionResolver.
func NewDefaultDBConnectionResolver(resolver port.ExpressionResolver) port.DBConnectionResolver {
	return &DefaultDBConnectionResolver{resolver: resolver}
}

// ResolveDBConnectionName resolves the database connection name (data source name) based on the execution context.
func (r *DefaultDBConnectionResolver) ResolveDBConnectionName(ctx context.Context, jobExecution *model.JobExecution, stepExecution *model.StepExecution, defaultName string) (string, error) {

	// 1. If defaultName is an expression (e.g., #{jobParameters['db_name']}), attempt to resolve it.
	if strings.Contains(defaultName, "#{") {
		resolvedName, err := r.resolver.Resolve(ctx, defaultName, jobExecution, stepExecution)
		if err == nil {
			// Update even if the resolution result is an empty string.
			defaultName = resolvedName
		} else {
			logger.Warnf("DBConnectionResolver: Failed to resolve dynamic expression '%s'. Using original default value: %v", defaultName, err)
		}
	}

	// The actual dynamic routing logic will be implemented upon completion of J.1/J.2.

	if defaultName == "" {
		// If no default name is provided, use the framework's default "workload".
		defaultName = "workload"
	}

	logger.Debugf("DBConnectionResolver: Resolved database connection name to '%s'.", defaultName)
	return defaultName, nil
}

// Verify that DefaultDBConnectionResolver implements the core.DBConnectionResolver interface.
var _ port.DBConnectionResolver = (*DefaultDBConnectionResolver)(nil)
