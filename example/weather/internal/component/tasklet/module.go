// Package tasklet provides Fx modules for application-specific tasklets.
package tasklet

import (
	"database/sql"
	"time"

	"go.uber.org/fx"

	entity "github.com/tigerroll/surfin/example/weather/internal/domain/entity"
	"github.com/tigerroll/surfin/pkg/batch/adapter/database"
	"github.com/tigerroll/surfin/pkg/batch/adapter/storage"
	tasklet "github.com/tigerroll/surfin/pkg/batch/component/tasklet/generic" // Use alias to avoid package name conflict.
	configjsl "github.com/tigerroll/surfin/pkg/batch/core/config/jsl"
)

// ScanHourlyForecast scans `sql.Rows` into an `entity.HourlyForecast` instance.
// It aligns with the `sqlSelectColumns` defined in JSL (e.g., "time, weather_code, temperature_2m, latitude, longitude, collected_at")
// and the `HourlyForecast` struct's field types and order.
func ScanHourlyForecast(rows *sql.Rows) (entity.HourlyForecast, error) {
	var item entity.HourlyForecast
	var dbTime, dbCollectedAt time.Time // Temporary variables to scan time.Time from the database.

	err := rows.Scan(
		&dbTime, // Scan into time.Time type.
		&item.WeatherCode,
		&item.Temperature2M,
		&item.Latitude,
		&item.Longitude,
		&dbCollectedAt, // Scan into time.Time type.
	)
	if err != nil {
		return item, err
	}

	// Convert time.Time to Unix milliseconds (int64) and store in the struct.
	item.Time = dbTime.UnixMilli()
	item.CollectedAt = dbCollectedAt.UnixMilli()

	return item, nil
}

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
				// Provides a scan function (`ScanHourlyForecast`) for the specific type (`entity.HourlyForecast`).
				scanFunc := ScanHourlyForecast

				// Returns the builder function for GenericParquetExportTasklet.
				return tasklet.NewGenericParquetExportTaskletBuilder[entity.HourlyForecast](
					dbResolver,
					storageResolver,
					itemPrototype,
					scanFunc,
				)
			},
			// Tags this component builder with the JSL name "genericParquetExportTasklet" for `JobFactory` registration.
			fx.ResultTags(`name:"genericParquetExportTasklet"`),
		),
	),
)
