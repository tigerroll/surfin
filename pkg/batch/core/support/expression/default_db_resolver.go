// Package expression provides utilities for resolving dynamic expressions within the batch framework.
// This file specifically contains the default implementation for resolving database connection names.
package expression

import (
	"context"
	"fmt"
	"strings"

	"github.com/tigerroll/surfin/pkg/batch/core/adaptor"
	port "github.com/tigerroll/surfin/pkg/batch/core/application/port"
	model "github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	logger "github.com/tigerroll/surfin/pkg/batch/support/util/logger"
)

// DefaultDBConnectionResolver is the default implementation of DBConnectionResolver.
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
//	A new instance of port.DBConnectionResolver.
func NewDefaultDBConnectionResolver(resolver port.ExpressionResolver) port.DBConnectionResolver {
	return &DefaultDBConnectionResolver{resolver: resolver}
}

// ResolveDBConnectionName resolves the database connection name (data source name) based on the execution context.
// It attempts to resolve dynamic expressions within the provided defaultName.
//
// Parameters:
//
//	ctx: The context for the operation.
//	jobExecution: The current JobExecution.
//	stepExecution: The current StepExecution (may be nil).
//	defaultName: The default connection name, which might contain expressions.
//
// Returns:
//
//	The resolved database connection name and an error if resolution fails.
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

// ResolveDBConnection is not intended to provide actual database connections in this package.
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
//	An adaptor.DBConnection (always nil) and an error indicating that this operation is not supported here.
func (r *DefaultDBConnectionResolver) ResolveDBConnection(ctx context.Context, name string) (adaptor.DBConnection, error) {
	return nil, fmt.Errorf("DefaultDBConnectionResolver in 'expression' package does not support resolving actual DB connections; it only resolves connection names. Attempted to resolve: %s", name)
}

// Verify that DefaultDBConnectionResolver implements the core.DBConnectionResolver interface.
var _ port.DBConnectionResolver = (*DefaultDBConnectionResolver)(nil)
