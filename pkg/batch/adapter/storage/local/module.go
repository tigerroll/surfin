// Package local provides the Fx module for the local storage adapter.
package local

import (
	"go.uber.org/fx"

	storageAdapter "github.com/tigerroll/surfin/pkg/batch/adapter/storage"
)

// Module is the Fx module for the Local storage adapter.
// It provides the LocalProvider and LocalConnectionResolver to the Fx application graph.
var Module = fx.Options(
	// Provide NewLocalProvider and tag it with group:"storage_providers".
	fx.Provide(fx.Annotate(
		NewLocalProvider,
		fx.As(new(storageAdapter.StorageProvider)), // Provide as StorageProvider interface.
		fx.ResultTags(`group:"storage_providers"`), // Tag for Fx to collect into []storage.StorageProvider.
	)),
)
