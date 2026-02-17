package adapter

import (
	"context"
)

// ResourceConnection represents a generic connection to any resource (e.g., database, storage).
type ResourceConnection interface {
	// Close closes the resource connection.
	Close() error
	// Type returns the type of the resource (e.g., "mysql", "s3").
	Type() string
	// Name returns the connection name (e.g., "metadata", "workload").
	Name() string
}

// ResourceProvider is an interface responsible for providing resource connections based on configuration.
type ResourceProvider interface {
	// GetConnection retrieves a resource connection with the specified name.
	GetConnection(name string) (ResourceConnection, error)
	// CloseAll closes all connections managed by this provider.
	CloseAll() error
	// Type returns the type of resource handled by this provider (e.g., "database", "storage").
	Type() string
	// Name returns the unique name of this resource provider (e.g., "database", "s3").
	Name() string
}

// ResourceConnectionResolver is an interface that resolves the required resource connection instance based on the execution context.
type ResourceConnectionResolver interface {
	// ResolveConnection resolves a resource connection instance by name.
	// This method is responsible for ensuring that the returned connection is valid and re-established if necessary.
	ResolveConnection(ctx context.Context, name string) (ResourceConnection, error)

	// ResolveConnectionName resolves the name of the resource connection based on the execution context.
	// This method allows dynamic selection of resource connections (e.g., based on job parameters or step context).
	// jobExecution and stepExecution are passed as interface{} to avoid circular dependencies with model package.
	ResolveConnectionName(ctx context.Context, jobExecution interface{}, stepExecution interface{}, defaultName string) (string, error)
}
