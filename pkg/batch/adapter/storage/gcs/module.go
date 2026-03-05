package gcs

import (
	"go.uber.org/fx"

	// coreConfig "github.com/tigerroll/surfin/pkg/batch/core/config" // Removed: unused.
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
	// The provision of GCSConnectionResolver is removed.
	// StorageConnectionResolver should be provided centrally by local.NewLocalConnectionResolver (or a more generic NewStorageConnectionResolver).
	// fx.Provide(fx.Annotate(
	// 	NewGCSConnectionResolver,
	// 	fx.As(new(storageAdapter.StorageConnectionResolver)),
	// 	fx.ParamTags(`group:"storage_providers"`),
	// )),
)
