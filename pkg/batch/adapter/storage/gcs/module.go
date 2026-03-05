package gcs

import (
	"go.uber.org/fx"

	storageAdapter "github.com/tigerroll/surfin/pkg/batch/adapter/storage"
)

// Module is the Fx module for the GCS storage adapter.
var Module = fx.Options(
	// Provides GCSProvider as a StorageProvider and includes it in the storage_providers group.
	fx.Provide(fx.Annotate(
		NewGCSProvider,
		fx.As(new(storageAdapter.StorageProvider)),
		fx.ResultTags(`group:"storage_providers"`),
	)),
)
