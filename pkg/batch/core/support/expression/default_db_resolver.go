// Package expression provides utilities for resolving dynamic expressions within the batch framework.
// This file specifically contains the default implementation for resolving database connection names.
package expression

import (
	"context"
	"fmt"
	"strings"

	coreAdapter "github.com/tigerroll/surfin/pkg/batch/core/adapter" // Import for coreAdapter.ResourceConnectionResolver
	port "github.com/tigerroll/surfin/pkg/batch/core/application/port"
	model "github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	logger "github.com/tigerroll/surfin/pkg/batch/support/util/logger"
)

// DefaultDBConnectionResolver is the default implementation of ResourceConnectionResolver.
// It primarily focuses on resolving database connection names, potentially from expressions.
type DefaultDBConnectionResolver struct {
	resolver port.ExpressionResolver
}

// NewDefaultDBConnectionResolver creates a new instance of DefaultDBConnectionResolver.
//
// Parameters:
//
//	resolver: An ExpressionResolver used to resolve dynamic expressions within connection names.
//
// Returns:
//
//	A new instance of coreAdapter.ResourceConnectionResolver.
func NewDefaultDBConnectionResolver(resolver port.ExpressionResolver) coreAdapter.ResourceConnectionResolver {
	return &DefaultDBConnectionResolver{resolver: resolver}
}

// ResolveConnectionName resolves the database connection name (data source name) based on the execution context.
// It attempts to resolve dynamic expressions within the provided defaultName.
//
// Parameters:
//
//	ctx: The context for the operation.
//	jobExecution: The current JobExecution (as interface{} to avoid circular dependency).
//	stepExecution: The current StepExecution (as interface{} to avoid circular dependency, may be nil).
//	defaultName: The default connection name, which might contain expressions.
//
// Returns:
//
//	The resolved database connection name and an error if resolution fails.
func (r *DefaultDBConnectionResolver) ResolveConnectionName(ctx context.Context, jobExecution interface{}, stepExecution interface{}, defaultName string) (string, error) {
	var jobExec *model.JobExecution
	if je, ok := jobExecution.(*model.JobExecution); ok {
		jobExec = je
	}
	var stepExec *model.StepExecution
	if se, ok := stepExecution.(*model.StepExecution); ok {
		stepExec = se
	}

	// If defaultName is an expression (e.g., #{jobParameters['db_name']}), attempt to resolve it.
	if strings.Contains(defaultName, "#{") {
		resolvedName, err := r.resolver.Resolve(ctx, defaultName, jobExec, stepExec)
		if err == nil {
			defaultName = resolvedName
		} else {
			logger.Warnf("DBConnectionResolver: Failed to resolve dynamic expression '%s'. Using original default value: %v", defaultName, err)
		}
	}

	if defaultName == "" {
		defaultName = "workload"
	}

	logger.Debugf("DBConnectionResolver: Resolved database connection name to '%s'.", defaultName)
	return defaultName, nil
}

// ResolveConnection is not intended to provide actual database connections in this package.
// This implementation explicitly returns an error, as the 'expression' package's resolver
// is only responsible for resolving connection *names*, not establishing actual connections.
// Actual connection resolution should be handled by a dedicated DBProvider.
//
// Parameters:
//
//	ctx: The context for the operation.
//	name: The name of the database connection to resolve.
//
// Returns:
//
//	An coreAdapter.ResourceConnection (always nil) and an error indicating that this operation is not supported here.
func (r *DefaultDBConnectionResolver) ResolveConnection(ctx context.Context, name string) (coreAdapter.ResourceConnection, error) {
	return nil, fmt.Errorf("DefaultDBConnectionResolver in 'expression' package does not support resolving actual DB connections; it only resolves connection names. Attempted to resolve: %s", name)
}

// Verify that DefaultDBConnectionResolver implements the coreAdapter.ResourceConnectionResolver interface.
var _ coreAdapter.ResourceConnectionResolver = (*DefaultDBConnectionResolver)(nil)
