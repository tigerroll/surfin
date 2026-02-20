package tasklet

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mitchellh/mapstructure"
	"github.com/xitongsys/parquet-go/parquet"
	"github.com/xitongsys/parquet-go/writer"

	weatherModel "github.com/tigerroll/surfin/example/weather/internal/domain/model"
	"github.com/tigerroll/surfin/pkg/batch/adapter/database"
	"github.com/tigerroll/surfin/pkg/batch/adapter/storage"
	coreAdapter "github.com/tigerroll/surfin/pkg/batch/core/adapter" // Added for coreAdapter.ResourceProvider.
	"github.com/tigerroll/surfin/pkg/batch/core/application/port"
	"github.com/tigerroll/surfin/pkg/batch/core/config"     // Added for config.Config.
	"github.com/tigerroll/surfin/pkg/batch/core/config/jsl" // Added for jsl.ComponentBuilder.
	"github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	"github.com/tigerroll/surfin/pkg/batch/support/util/logger"
)

// Verify that HourlyForecastExportTasklet implements the port.Tasklet interface.
var _ port.Tasklet = (*HourlyForecastExportTasklet)(nil)

// HourlyForecastExportTaskletConfig holds configuration for the HourlyForecastExportTasklet.
type HourlyForecastExportTaskletConfig struct {
	// DbRef is the name of the database connection to use for reading hourly forecast data.
	DbRef string `mapstructure:"dbRef"`
	// StorageRef is the name of the storage connection to use for exporting Parquet files.
	StorageRef string `mapstructure:"storageRef"`
	// OutputBaseDir is the base directory within the storage bucket for exported files.
	OutputBaseDir string `mapstructure:"outputBaseDir"`
}

// HourlyForecastExportTasklet is a Tasklet that exports hourly forecast data from a database
// to Parquet files in a Hive-partitioned directory structure using a local storage adapter.
type HourlyForecastExportTasklet struct {
	config                    *HourlyForecastExportTaskletConfig
	dbConnectionResolver      database.DBConnectionResolver
	storageConnectionResolver storage.StorageConnectionResolver
}

// Close is required to satisfy the Tasklet interface.
// This tasklet does not hold long-term resources, so it does nothing.
func (t *HourlyForecastExportTasklet) Close(ctx context.Context) error {
	logger.Debugf("HourlyForecastExportTasklet for DbRef '%s' closed.", t.config.DbRef)
	return nil
}

// GetExecutionContext is required to satisfy the Tasklet interface.
// This tasklet does not maintain persistent execution context, so it returns an empty context.
func (t *HourlyForecastExportTasklet) GetExecutionContext(ctx context.Context) (model.ExecutionContext, error) {
	return model.NewExecutionContext(), nil
}

// SetExecutionContext is required to satisfy the Tasklet interface.
// This tasklet does not maintain persistent execution context, so it does nothing.
func (t *HourlyForecastExportTasklet) SetExecutionContext(ctx context.Context, context model.ExecutionContext) error {
	return nil
}

// NewHourlyForecastExportTasklet creates a new instance of HourlyForecastExportTasklet.
func NewHourlyForecastExportTasklet(
	properties map[string]string,
	dbConnectionResolver database.DBConnectionResolver,
	storageConnectionResolver storage.StorageConnectionResolver,
) (port.Tasklet, error) { // Returns port.Tasklet.
	logger.Debugf("HourlyForecastExportTasklet builder received properties: %v", properties)

	cfg := &HourlyForecastExportTaskletConfig{}
	if err := mapstructure.Decode(properties, cfg); err != nil {
		return nil, fmt.Errorf("failed to decode properties into HourlyForecastExportTaskletConfig: %w", err)
	}

	return &HourlyForecastExportTasklet{
		config:                    cfg,
		dbConnectionResolver:      dbConnectionResolver,
		storageConnectionResolver: storageConnectionResolver,
	}, nil
}

// Execute performs the tasklet's logic.
func (t *HourlyForecastExportTasklet) Execute(ctx context.Context, stepExecution *model.StepExecution) (model.ExitStatus, error) {
	logger.Infof("HourlyForecastExportTasklet is executing. Config: %+v", t.config)

	dbConn, err := t.dbConnectionResolver.ResolveDBConnection(ctx, t.config.DbRef)
	if err != nil {
		return model.ExitStatusFailed, fmt.Errorf("failed to resolve DB connection '%s': %w", t.config.DbRef, err)
	}
	defer func() {
		if err := dbConn.Close(); err != nil {
			logger.Errorf("Failed to close DB connection '%s': %v", t.config.DbRef, err)
		}
	}()

	var forecasts []weatherModel.HourlyForecast
	// Use map[string]interface{} to pass raw SQL query.
	err = dbConn.ExecuteQuery(ctx, &forecasts, map[string]interface{}{
		"query": "SELECT " +
			"CAST(EXTRACT(EPOCH FROM time) * 1000 AS BIGINT) AS time, " + // Cast to BIGINT.
			"weather_code, " +
			"temperature_2m, " +
			"latitude, " +
			"longitude, " +
			"CAST(EXTRACT(EPOCH FROM collected_at) * 1000 AS BIGINT) AS collected_at " + // Cast to BIGINT.
			"FROM hourly_forecast ORDER BY time ASC",
	})
	if err != nil {
		return model.ExitStatusFailed, fmt.Errorf("failed to query hourly_forecast data from '%s': %w", t.config.DbRef, err)
	}

	logger.Infof("Successfully fetched %d records from hourly_forecast table using DB connection '%s'.", len(forecasts), t.config.DbRef)

	if len(forecasts) == 0 {
		logger.Warnf("No hourly forecast records to export.")
		return model.ExitStatusCompleted, nil
	}

	// Group data by date.
	forecastsByDate := make(map[string][]weatherModel.HourlyForecast) // Key: YYYY-MM-DD.
	for _, forecast := range forecasts {
		// Convert int64 (milliseconds) to time.Time and format the date.
		recordTime := time.UnixMilli(forecast.Time)
		dateStr := recordTime.Format("2006-01-02")
		forecastsByDate[dateStr] = append(forecastsByDate[dateStr], forecast)
	}

	// Resolve storage connection once.
	storageConn, err := t.storageConnectionResolver.ResolveStorageConnection(ctx, t.config.StorageRef)
	if err != nil {
		return model.ExitStatusFailed, fmt.Errorf("failed to resolve storage connection '%s': %w", t.config.StorageRef, err)
	}
	defer func() {
		if err := storageConn.Close(); err != nil {
			logger.Errorf("Failed to close storage connection '%s': %v", t.config.StorageRef, err)
		}
	}()

	// Process each date group.
	for dateStr, dailyForecasts := range forecastsByDate {
		logger.Infof("Processing %d records for date %s.", len(dailyForecasts), dateStr)

		buf := new(bytes.Buffer)
		pw, err := writer.NewParquetWriterFromWriter(buf, new(weatherModel.HourlyForecast), 1)
		if err != nil {
			return model.ExitStatusFailed, fmt.Errorf("failed to create parquet writer for date %s: %w", dateStr, err)
		}
		pw.CompressionType = parquet.CompressionCodec_SNAPPY

		for _, forecast := range dailyForecasts {
			if err = pw.Write(forecast); err != nil {
				return model.ExitStatusFailed, fmt.Errorf("failed to write record to parquet for date %s: %w", dateStr, err)
			}
		}

		// The ParquetWriter's WriteStop() method can panic.
		// Use defer recover() to catch panics and convert them into errors.
		var writeStopErr error
		func() {
			defer func() {
				if r := recover(); r != nil {
					if err, ok := r.(error); ok {
						writeStopErr = err
					} else {
						writeStopErr = fmt.Errorf("panic value: %v", r)
					}
					logger.Errorf("Caught panic during pw.WriteStop() (internal) for date %s: %v", dateStr, writeStopErr)
				}
			}()
			writeStopErr = pw.WriteStop()
		}()

		if writeStopErr != nil {
			var finalErrorMessage string
			func() {
				defer func() {
					if r := recover(); r != nil {
						finalErrorMessage = fmt.Sprintf("error.Error() method panicked: %v", r)
						logger.Errorf("Caught panic when calling Error() on writeStopErr for date %s: %v", dateStr, r)
					}
				}()
				finalErrorMessage = writeStopErr.Error()
			}()

			if strings.Contains(finalErrorMessage, "runtime error: invalid memory address or nil pointer dereference") {
				finalErrorMessage = "internal parquet writer error (possible data corruption or library issue)"
			} else if finalErrorMessage == "" {
				finalErrorMessage = "unknown error (Error() method returned empty string or panicked)"
			}
			return model.ExitStatusFailed, fmt.Errorf("failed to stop parquet writer for date %s: %s", dateStr, finalErrorMessage)
		}

		logger.Infof("Successfully converted %d records to Parquet format for date %s. Data size: %d bytes.", len(dailyForecasts), dateStr, buf.Len())

		// Determine the Hive partition date.
		hivePartitionPath := fmt.Sprintf("dt=%s", dateStr)

		// Generate a unique filename for the Parquet file.
		fileName := fmt.Sprintf("hourly_forecast_%s_%s.parquet",
			strings.ReplaceAll(dateStr, "-", ""), // Use YYYYMMDD format for the filename.
			time.Now().Format("150405"))          // HHMMSS format.
		objectPath := fmt.Sprintf("%s/%s/%s", t.config.OutputBaseDir, hivePartitionPath, fileName)

		// Upload the Parquet data.
		err = storageConn.Upload(ctx, "", objectPath, buf, "application/x-parquet")
		if err != nil {
			return model.ExitStatusFailed, fmt.Errorf("failed to upload parquet file for date %s to '%s': %w", dateStr, objectPath, err)
		}

		logger.Infof("Successfully uploaded Parquet file for date %s to '%s' using storage connection '%s'.", dateStr, objectPath, t.config.StorageRef)
	}

	return model.ExitStatusCompleted, nil
}

// NewHourlyForecastExportTaskletBuilder creates a jsl.ComponentBuilder that builds HourlyForecastExportTasklet instances.
// This function directly returns a jsl.ComponentBuilder, simplifying the Fx provision.
func NewHourlyForecastExportTaskletBuilder(
	dbConnectionResolver database.DBConnectionResolver,
	storageConnectionResolver storage.StorageConnectionResolver,
) jsl.ComponentBuilder { // Returns jsl.ComponentBuilder directly.
	return func(
		cfg *config.Config, // cfg is not used but matches the jsl.ComponentBuilder signature.
		resolver port.ExpressionResolver, // resolver is not used but matches the jsl.ComponentBuilder signature.
		resourceProviders map[string]coreAdapter.ResourceProvider, // resourceProviders is not used but matches the jsl.ComponentBuilder signature.
		properties map[string]string,
	) (interface{}, error) {
		// Pass the necessary dependencies to NewHourlyForecastExportTasklet.
		return NewHourlyForecastExportTasklet(properties, dbConnectionResolver, storageConnectionResolver)
	}
}
