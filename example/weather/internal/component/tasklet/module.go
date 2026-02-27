// Package tasklet provides Fx modules for application-specific tasklets.
package tasklet

import (
	"go.uber.org/fx"

	entity "github.com/tigerroll/surfin/example/weather/internal/domain/entity"
	"github.com/tigerroll/surfin/pkg/batch/adapter/database"
	"github.com/tigerroll/surfin/pkg/batch/adapter/storage"
	tasklet "github.com/tigerroll/surfin/pkg/batch/component/tasklet/generic" // Use alias to avoid package name conflict.
	configjsl "github.com/tigerroll/surfin/pkg/batch/core/config/jsl"
)

// Module provides the Fx module for the `GenericParquetExportTasklet` builder
// specialized for `entity.HourlyForecast`.
var Module = fx.Options(
	fx.Provide(
		fx.Annotate(
			func(
				dbResolver database.DBConnectionResolver,
				storageResolver storage.StorageConnectionResolver,
			) configjsl.ComponentBuilder {
				// Provides a zero-value instance of `itemPrototype` for Parquet schema inference.
				itemPrototype := &entity.HourlyForecast{}
				// Returns the builder function for GenericParquetExportTasklet.
				return tasklet.NewGenericParquetExportTaskletBuilder[entity.HourlyForecast](
					dbResolver,
					storageResolver,
					itemPrototype,
				)
			},
			// Tags this component builder with the JSL name "genericParquetExportTasklet" for `JobFactory` registration.
			fx.ResultTags(`name:"genericParquetExportTasklet"`),
		),
	),
)
