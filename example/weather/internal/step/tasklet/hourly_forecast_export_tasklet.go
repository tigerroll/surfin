// Package tasklet provides tasklet implementations for the weather example.
// This includes HourlyForecastExportTasklet for exporting weather data.
package tasklet

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/mitchellh/mapstructure"
	"github.com/xitongsys/parquet-go/parquet"
	"github.com/xitongsys/parquet-go/writer"
	"golang.org/x/sync/errgroup"

	weatherModel "github.com/tigerroll/surfin/example/weather/internal/domain/model"
	"github.com/tigerroll/surfin/pkg/batch/adapter/database"
	"github.com/tigerroll/surfin/pkg/batch/adapter/storage"
	readerComponent "github.com/tigerroll/surfin/pkg/batch/component/step/reader"
	coreAdapter "github.com/tigerroll/surfin/pkg/batch/core/adapter"
	"github.com/tigerroll/surfin/pkg/batch/core/application/port"
	"github.com/tigerroll/surfin/pkg/batch/core/config"
	"github.com/tigerroll/surfin/pkg/batch/core/config/jsl"
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
	// ReadBufferSize defines the buffer size for the channel between reader and writer goroutines.
	ReadBufferSize int `mapstructure:"readBufferSize"`
}

// HourlyForecastExportTasklet is a Tasklet that exports hourly forecast data from a database
// to Parquet files in a Hive-partitioned directory structure using a local storage adapter.
// It implements a hybrid pipeline using goroutines and channels for efficient processing.
type HourlyForecastExportTasklet struct {
	config                    *HourlyForecastExportTaskletConfig
	dbConnectionResolver      database.DBConnectionResolver
	storageConnectionResolver storage.StorageConnectionResolver
}

// Close is required to satisfy the Tasklet interface.
// This tasklet does not hold long-term resources that need explicit closing, so it performs no operation.
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
// This tasklet does not maintain persistent execution context, so it performs no operation.
func (t *HourlyForecastExportTasklet) SetExecutionContext(ctx context.Context, context model.ExecutionContext) error {
	return nil
}

// NewHourlyForecastExportTasklet creates a new instance of HourlyForecastExportTasklet.
//
// properties: A map of string properties for configuration.
// dbConnectionResolver: Resolver for database connections.
// storageConnectionResolver: Resolver for storage connections.
//
// Returns: A port.Tasklet instance and an error if configuration decoding fails.
func NewHourlyForecastExportTasklet(
	properties map[string]string,
	dbConnectionResolver database.DBConnectionResolver,
	storageConnectionResolver storage.StorageConnectionResolver,
) (port.Tasklet, error) {
	logger.Debugf("HourlyForecastExportTasklet builder received properties: %v", properties)

	cfg := &HourlyForecastExportTaskletConfig{
		ReadBufferSize: 1000, // Default buffer size
	}
	if err := mapstructure.Decode(properties, cfg); err != nil {
		return nil, fmt.Errorf("failed to decode properties into HourlyForecastExportTaskletConfig: %w", err)
	}

	return &HourlyForecastExportTasklet{
		config:                    cfg,
		dbConnectionResolver:      dbConnectionResolver,
		storageConnectionResolver: storageConnectionResolver,
	}, nil
}

// Execute performs the tasklet's core logic: reading hourly forecast data from a database
// and exporting it to Parquet files in a Hive-partitioned structure.
// It utilizes a hybrid pipeline with goroutines and channels for concurrent processing.
//
// ctx: The context for the operation.
// stepExecution: The current StepExecution, used for accessing ExecutionContext.
//
// Returns: An ExitStatus indicating success or failure, and an error if any occurs.
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

	// Get the underlying *sql.DB from the DBConnection.
	// This relies on the database.DBConnection interface now having a GetSQLDB() method.
	var sqlDB *sql.DB
	sqlDB, err = dbConn.GetSQLDB()
	if err != nil {
		return model.ExitStatusFailed, fmt.Errorf("failed to get *sql.DB from DBConnection '%s': %w", t.config.DbRef, err)
	}

	// Initialize SqlCursorReader.
	// The SQL query is adjusted to match the fields of weatherModel.HourlyForecast.
	query := "SELECT " +
		"CAST(EXTRACT(EPOCH FROM time) * 1000 AS BIGINT) AS time, " +
		"weather_code, " +
		"temperature_2m, " +
		"latitude, " +
		"longitude, " +
		"CAST(EXTRACT(EPOCH FROM collected_at) * 1000 AS BIGINT) AS collected_at " +
		"FROM hourly_forecast ORDER BY time ASC"

	// The name of SqlCursorReader is used as a restartability key in the ExecutionContext.
	readerName := "hourlyForecastReader"
	sqlReader := readerComponent.NewSqlCursorReader[weatherModel.HourlyForecast](
		sqlDB,
		readerName,
		query,
		nil, // No query arguments
		func(rows *sql.Rows) (weatherModel.HourlyForecast, error) {
			var hf weatherModel.HourlyForecast
			err := rows.Scan(&hf.Time, &hf.WeatherCode, &hf.Temperature2M, &hf.Latitude, &hf.Longitude, &hf.CollectedAt)
			return hf, err
		},
	)

	// Channel between Reader and Writer goroutines.
	dataCh := make(chan weatherModel.HourlyForecast, t.config.ReadBufferSize)
	g, gCtx := errgroup.WithContext(ctx)

	// Reader Goroutine
	g.Go(func() error {
		defer close(dataCh) // Close the channel when the reader goroutine finishes.
		logger.Infof("SqlCursorReader '%s' opening...", readerName)
		if err := sqlReader.Open(gCtx, stepExecution.ExecutionContext); err != nil {
			if err == io.EOF { // No data is not an error.
				logger.Infof("SqlCursorReader '%s': No data to read.", readerName)
				return nil
			}
			return fmt.Errorf("failed to open SqlCursorReader '%s': %w", readerName, err)
		}
		defer func() {
			if err := sqlReader.Close(gCtx); err != nil {
				logger.Errorf("Failed to close SqlCursorReader '%s': %v", readerName, err)
			}
		}()

		for {
			item, err := sqlReader.Read(gCtx)
			if err != nil {
				if err == io.EOF {
					logger.Infof("SqlCursorReader '%s': Finished reading all data.", readerName)
					break
				}
				return fmt.Errorf("failed to read from SqlCursorReader '%s': %w", readerName, err)
			}
			select {
			case dataCh <- item:
			case <-gCtx.Done(): // Context cancellation check.
				logger.Warnf("SqlCursorReader '%s': Context cancelled during read.", readerName)
				return gCtx.Err()
			}
		}
		return nil
	})

	// Writer Goroutine.
	g.Go(func() error {
		// Resolve storage connection once for the writer goroutine.
		rawStorageConn, err := t.storageConnectionResolver.ResolveStorageConnection(gCtx, t.config.StorageRef)
		if err != nil {
			return fmt.Errorf("failed to resolve storage connection '%s': %w", t.config.StorageRef, err)
		}
		defer func() {
			if err := rawStorageConn.Close(); err != nil {
				logger.Errorf("Failed to close storage connection '%s': %v", t.config.StorageRef, err)
			}
		}()

		// Type assert to storage.StorageConnection to access the Upload method.
		concreteStorageConn, ok := rawStorageConn.(storage.StorageConnection)
		if !ok {
			return fmt.Errorf("resolved storage connection '%s' does not implement storage.StorageConnection interface", t.config.StorageRef)
		}

		// Group data by date for Parquet export
		forecastsByDate := make(map[string][]weatherModel.HourlyForecast) // Key: YYYY-MM-DD.

		for {
			select {
			case item, ok := <-dataCh:
				if !ok { // Channel closed and empty
					logger.Infof("Writer Goroutine: Data channel closed. Processing remaining %d records.", len(forecastsByDate))
					// Process any remaining buffered data before exiting
					return t.processAndUploadParquet(gCtx, concreteStorageConn, forecastsByDate)
				}
				// Convert int64 (milliseconds) to time.Time and format the date.
				recordTime := time.UnixMilli(item.Time)
				dateStr := recordTime.Format("2006-01-02")
				forecastsByDate[dateStr] = append(forecastsByDate[dateStr], item)

				// Optional: Flush based on a certain number of records or time.
				// For simplicity, flushing occurs only when the channel closes or context is done.
				// In a real-world scenario, periodic or batch-size-based flushing might be desired.

			case <-gCtx.Done(): // Context cancellation check.
				logger.Warnf("Writer Goroutine: Context cancelled. Processing remaining %d records.", len(forecastsByDate))
				return t.processAndUploadParquet(gCtx, concreteStorageConn, forecastsByDate) // Attempt to flush remaining data.
			}
		}
	})

	// Wait for goroutines to complete.
	if err := g.Wait(); err != nil {
		logger.Errorf("HourlyForecastExportTasklet pipeline failed: %v", err)
		return model.ExitStatusFailed, err
	}

	logger.Infof("HourlyForecastExportTasklet completed successfully.")
	return model.ExitStatusCompleted, nil
}

// processAndUploadParquet is a helper function that processes grouped hourly forecasts
// and uploads them as Parquet files to the configured storage.
//
// ctx: The context for the operation.
// storageConn: The storage connection to use for uploading files.
// forecastsByDate: A map of hourly forecasts grouped by date (YYYY-MM-DD).
//
// Returns: An error if processing or uploading fails.
func (t *HourlyForecastExportTasklet) processAndUploadParquet(
	ctx context.Context,
	storageConn storage.StorageConnection,
	forecastsByDate map[string][]weatherModel.HourlyForecast,
) error {
	if len(forecastsByDate) == 0 {
		return nil // Nothing to process
	}

	for dateStr, dailyForecasts := range forecastsByDate {
		logger.Infof("Processing %d records for date %s.", len(dailyForecasts), dateStr)

		buf := new(bytes.Buffer)
		pw, err := writer.NewParquetWriterFromWriter(buf, new(weatherModel.HourlyForecast), 1)
		if err != nil {
			return fmt.Errorf("failed to create parquet writer for date %s: %w", dateStr, err)
		}
		pw.CompressionType = parquet.CompressionCodec_SNAPPY

		for _, forecast := range dailyForecasts {
			if err = pw.Write(forecast); err != nil {
				return fmt.Errorf("failed to write record to parquet for date %s: %w", dateStr, err)
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
			return fmt.Errorf("failed to stop parquet writer for date %s: %s", dateStr, finalErrorMessage)
		}

		logger.Infof("Successfully converted %d records to Parquet format for date %s. Data size: %d bytes.", len(dailyForecasts), dateStr, buf.Len())

		// Determine the Hive partition path (e.g., dt=YYYY-MM-DD).
		hivePartitionPath := fmt.Sprintf("dt=%s", dateStr)

		// Generate a unique filename for the Parquet file (e.g., hourly_forecast_YYYYMMDD_HHMMSS.parquet).
		fileName := fmt.Sprintf("hourly_forecast_%s_%s.parquet",
			strings.ReplaceAll(dateStr, "-", ""), // Use YYYYMMDD format for the filename.
			time.Now().Format("150405"))          // HHMMSS format for uniqueness.
		objectPath := fmt.Sprintf("%s/%s/%s", t.config.OutputBaseDir, hivePartitionPath, fileName)

		// Upload the Parquet data.
		err = storageConn.Upload(ctx, "", objectPath, buf, "application/x-parquet")
		if err != nil {
			// Return the error directly if upload fails.
			return fmt.Errorf("failed to upload parquet file for date %s to '%s': %w", dateStr, objectPath, err)
		}

		logger.Infof("Successfully uploaded Parquet file for date %s to '%s' using storage connection '%s'.", dateStr, objectPath, t.config.StorageRef)
	}
	// Clear the map after processing to free up memory.
	for k := range forecastsByDate {
		delete(forecastsByDate, k)
	}
	return nil
}

// NewHourlyForecastExportTaskletBuilder creates a jsl.ComponentBuilder that builds HourlyForecastExportTasklet instances.
// This function directly returns a jsl.ComponentBuilder, simplifying the Fx provision.
//
// dbConnectionResolver: Resolver for database connections.
// storageConnectionResolver: Resolver for storage connections.
//
// Returns: A jsl.ComponentBuilder instance.
func NewHourlyForecastExportTaskletBuilder(
	dbConnectionResolver database.DBConnectionResolver,
	storageConnectionResolver storage.StorageConnectionResolver,
) jsl.ComponentBuilder {
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
