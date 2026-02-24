// Package storage defines the common interfaces for various storage adapters.
// These interfaces abstract storage operations, allowing the batch framework
// to interact with different storage backends (e.g., GCS, S3, local file system)
// through a unified API.
package storage

import (
	"context"
	"io"

	coreAdapter "github.com/tigerroll/surfin/pkg/batch/core/adapter"
)

// StorageExecutor defines generic storage operations.
// It is embedded into StorageAdapter to provide concrete storage functionalities.
type StorageExecutor interface {
	// Upload uploads data to the specified bucket and object name.
	// 'data' is the stream of data to upload. 'contentType' is the MIME type of the data.
	Upload(ctx context.Context, bucket, objectName string, data io.Reader, contentType string) error
	// Download downloads data from the specified bucket and object name.
	// It returns a ReadCloser which must be closed by the caller after use.
	Download(ctx context.Context, bucket, objectName string) (io.ReadCloser, error)
	// ListObjects lists objects within the specified bucket and prefix.
	// The 'fn' callback function is called for each object name found, allowing for
	// efficient processing of large numbers of objects without loading all into memory.
	ListObjects(ctx context.Context, bucket, prefix string, fn func(objectName string) error) error
	// DeleteObject deletes the specified object from the bucket.
	DeleteObject(ctx context.Context, bucket, objectName string) error
}

// StorageConnection represents a generic data storage connection.
// It embeds coreAdapter.ResourceConnection and StorageExecutor to provide both
// resource connection capabilities and specific storage operations.
type StorageConnection interface {
	coreAdapter.ResourceConnection // Inherits Close(), Type(), Name()
	StorageExecutor                // Inherits Upload(), Download(), ListObjects(), DeleteObject()

	// Config returns the storage configuration associated with this adapter.
	// Config() storageConfig.StorageConfig // Removed as storageConfig is no longer imported
}

// StorageProvider manages the acquisition and lifecycle of data storage connections.
// It provides similar functionality to coreAdapter.ResourceProvider.
// Note: coreAdapter.ResourceProvider has a Name() method, but StorageProvider does not.
// This is because StorageProvider acts as a provider for a specific "type" (e.g., "storage"),
// and that type itself serves as an identifier. If it needs to be treated as a
// coreAdapter.ResourceProvider (e.g., by ComponentBuilder), a wrapper might be necessary.
type StorageProvider interface {
	// GetConnection retrieves a StorageConnection connection with the specified name.
	GetConnection(name string) (StorageConnection, error)
	// CloseAll closes all connections managed by this provider.
	CloseAll() error
	// Type returns the type of resource handled by this provider (e.g., "database", "storage").
	Type() string
	// ForceReconnect forces the closure and re-establishment of an existing connection with the specified name.
	ForceReconnect(name string) (StorageConnection, error)
}

// StorageConnectionResolver resolves appropriate data storage connection instances based on the execution context.
// It embeds coreAdapter.ResourceConnectionResolver to provide generic resource resolution capabilities.
type StorageConnectionResolver interface {
	coreAdapter.ResourceConnectionResolver // Inherits ResolveConnection(), ResolveConnectionName()

	// ResolveStorageConnection resolves a StorageConnection connection instance by name.
	// This method is responsible for ensuring that the returned connection is valid and re-established if necessary.
	ResolveStorageConnection(ctx context.Context, name string) (StorageConnection, error)

	// ResolveStorageConnectionName resolves the name of the data storage connection based on the execution context.
	// jobExecution and stepExecution are passed as interface{} to avoid circular dependencies with model packages.
	ResolveStorageConnectionName(ctx context.Context, jobExecution interface{}, stepExecution interface{}, defaultName string) (string, error)
}
