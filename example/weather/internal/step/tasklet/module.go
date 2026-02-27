package tasklet

import (
	"database/sql" // For sql.Rows in scanFunc
	"go.uber.org/fx"

	entity "github.com/tigerroll/surfin/example/weather/internal/domain/entity" // For entity.WeatherDataToStore
	"github.com/tigerroll/surfin/pkg/batch/adapter/database"                    // For database.DBConnectionResolver
	"github.com/tigerroll/surfin/pkg/batch/adapter/storage"                     // For storage.StorageConnectionResolver
	genericTasklet "github.com/tigerroll/surfin/pkg/batch/component/tasklet/generic"
	configjsl "github.com/tigerroll/surfin/pkg/batch/core/config/jsl" // For configjsl.ComponentBuilder
)

// Module is the Fx module for the tasklet components.
var Module = fx.Options(
	// Register the ComponentBuilder for HourlyForecastExportTasklet
	fx.Provide(fx.Annotate(
		// Provide NewHourlyForecastExportTaskletBuilder directly, allowing Fx to resolve its dependencies.
		NewHourlyForecastExportTaskletBuilder,
		fx.ResultTags(`name:"hourlyForecastExportTasklet"`), // Tag with the JSL reference name.
	)),
)

// NewHourlyForecastExportTaskletBuilderParams defines the dependencies for NewHourlyForecastExportTaskletBuilder.
type NewHourlyForecastExportTaskletBuilderParams struct {
	fx.In
	DBConnectionResolver      database.DBConnectionResolver
	StorageConnectionResolver storage.StorageConnectionResolver
}

// NewHourlyForecastExportTaskletBuilder creates a JSL ComponentBuilder for HourlyForecastExportTasklet.
// This builder is specifically for exporting weather forecast data to Parquet.
func NewHourlyForecastExportTaskletBuilder(params NewHourlyForecastExportTaskletBuilderParams) configjsl.ComponentBuilder {
	return genericTasklet.NewGenericParquetExportTaskletBuilder[entity.WeatherDataToStore](
		params.DBConnectionResolver,
		params.StorageConnectionResolver,
		&entity.WeatherDataToStore{}, // Prototype instance for schema reflection
		func(rows *sql.Rows) (entity.WeatherDataToStore, error) {
			var data entity.WeatherDataToStore
			err := rows.Scan(
				&data.Time,
				&data.WeatherCode,
				&data.Temperature2M,
				&data.Latitude,
				&data.Longitude,
				&data.CollectedAt,
			)
			return data, err
		},
	)
}
