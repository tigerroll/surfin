// Package tasklet provides implementations for various batch tasklets, including HourlyForecastExportTasklet for exporting weather data.
package tasklet

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"time"

	"github.com/mitchellh/mapstructure"
	"golang.org/x/sync/errgroup"

	weather_entity "github.com/tigerroll/surfin/example/weather/internal/domain/entity"
	"github.com/tigerroll/surfin/pkg/batch/adapter/database"
	"github.com/tigerroll/surfin/pkg/batch/adapter/storage"
	readerComponent "github.com/tigerroll/surfin/pkg/batch/component/step/reader"
	writerComponent "github.com/tigerroll/surfin/pkg/batch/component/step/writer"
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
	// ParquetCompressionType is the compression type for Parquet files (e.g., "SNAPPY", "GZIP", "NONE").
	ParquetCompressionType string `mapstructure:"parquetCompressionType"`
}

// HourlyForecastExportTasklet is a Tasklet that exports hourly forecast data from a database
// to Parquet files in a Hive-partitioned directory structure using a local storage adapter.
// It implements a hybrid pipeline using goroutines and channels for efficient processing.
type HourlyForecastExportTasklet struct {
	config                    *HourlyForecastExportTaskletConfig
	dbConnectionResolver      database.DBConnectionResolver
	storageConnectionResolver storage.StorageConnectionResolver
	// parquetWriter is the ItemWriter responsible for writing data to Parquet files.
	parquetWriter port.ItemWriter[weather_entity.HourlyForecast]
}

// Close is required to satisfy the Tasklet interface.
// This tasklet does not manage long-term resources that require explicit closing within its `Close` method,
// as the `ParquetWriter`'s `Close` is handled internally by `Execute`.
func (t *HourlyForecastExportTasklet) Close(ctx context.Context) error {
	logger.Debugf("HourlyForecastExportTasklet for DbRef '%s' closed.", t.config.DbRef)
	return nil
}

// GetExecutionContext is required to satisfy the Tasklet interface.
// This tasklet does not maintain persistent execution context across executions, thus it returns an empty context.
func (t *HourlyForecastExportTasklet) GetExecutionContext(ctx context.Context) (model.ExecutionContext, error) {
	return model.NewExecutionContext(), nil
}

// SetExecutionContext is required to satisfy the Tasklet interface.
// This tasklet does not maintain persistent execution context, so this method performs no operation.
func (t *HourlyForecastExportTasklet) SetExecutionContext(ctx context.Context, context model.ExecutionContext) error {
	return nil
}

// NewHourlyForecastExportTasklet creates a new instance of `HourlyForecastExportTasklet`.
// This function initializes the tasklet with the provided configuration properties and resolvers,
// including setting up the internal `ParquetWriter`.
//
// Parameters:
//
//	properties: Configuration properties for the tasklet, typically from JSL.
//	dbConnectionResolver: The resolver for database connections, used to obtain the source database.
//	storageConnectionResolver: The resolver for storage connections, used by the internal `ParquetWriter` for output.
//
// Returns:
//
//	A `port.Tasklet` instance or an error if initialization fails.
func NewHourlyForecastExportTasklet(
	properties map[string]string,
	dbConnectionResolver database.DBConnectionResolver,
	storageConnectionResolver storage.StorageConnectionResolver,
) (port.Tasklet, error) {
	logger.Debugf("HourlyForecastExportTasklet builder received properties: %v", properties)

	cfg := &HourlyForecastExportTaskletConfig{
		ReadBufferSize:         1000,     // Default buffer size
		ParquetCompressionType: "SNAPPY", // Default compression type
	}
	if err := mapstructure.Decode(properties, cfg); err != nil {
		return nil, fmt.Errorf("failed to decode properties into HourlyForecastExportTaskletConfig: %w", err)
	}

	// Build configuration properties for the ParquetWriter.
	parquetWriterProps := map[string]string{
		"storageRef":      cfg.StorageRef,
		"outputBaseDir":   cfg.OutputBaseDir,
		"compressionType": cfg.ParquetCompressionType,
	}

	// Create a ParquetWriter instance.
	parquetWriter, err := writerComponent.NewParquetWriter[weather_entity.HourlyForecast](
		"hourlyForecastParquetWriter", // Unique name for the ParquetWriter.
		parquetWriterProps,
		storageConnectionResolver,
		&weather_entity.HourlyForecast{}, // Prototype for Parquet schema inference (pointer to struct).
		func(item weather_entity.HourlyForecast) (string, error) { // Partition key extraction function.
			recordTime := time.UnixMilli(item.Time)
			// Hive-style partition key: dt=YYYY-MM-DD
			return fmt.Sprintf("dt=%s", recordTime.Format("2006-01-02")), nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create ParquetWriter: %w", err)
	}

	return &HourlyForecastExportTasklet{
		config:                    cfg,
		dbConnectionResolver:      dbConnectionResolver,
		storageConnectionResolver: storageConnectionResolver,
		parquetWriter:             parquetWriter,
	}, nil
}

// Execute performs the tasklet's core logic: reading hourly forecast data from the configured database
// and exporting it to Parquet files in a Hive-partitioned directory structure.
// It employs a concurrent pipeline using goroutines and channels for efficient data transfer and processing.
//
// Parameters:
//
//	ctx: The context for the operation, enabling cancellation and timeouts.
//	stepExecution: The current `StepExecution` instance, providing access to the execution context.
//
// Returns:
//
//	An `ExitStatus` indicating the outcome of the execution, and an error if any critical failure occurs.
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

	var sqlDB *sql.DB
	sqlDB, err = dbConn.GetSQLDB()
	if err != nil {
		return model.ExitStatusFailed, fmt.Errorf("failed to get *sql.DB from DBConnection '%s': %w", t.config.DbRef, err)
	}

	// Initialize SqlCursorReader.
	// The SQL query is adjusted to match the fields of weather_entity.HourlyForecast.
	query := "SELECT " +
		"CAST(EXTRACT(EPOCH FROM time) * 1000 AS BIGINT) AS time, " + // Aligns with weather_entity.HourlyForecast's int64 timestamp.
		"weather_code, " +
		"temperature_2m, " +
		"latitude, " +
		"longitude, " +
		"CAST(EXTRACT(EPOCH FROM collected_at) * 1000 AS BIGINT) AS collected_at " +
		"FROM hourly_forecast ORDER BY time ASC"

	// The name of SqlCursorReader is used as a restartability key in the ExecutionContext.
	readerName := "hourlyForecastReader" // Descriptive name for the reader, reflecting the data type.
	sqlReader := readerComponent.NewSqlCursorReader[weather_entity.HourlyForecast](
		sqlDB,
		readerName,
		query,
		nil, // No query arguments
		func(rows *sql.Rows) (weather_entity.HourlyForecast, error) {
			var hf weather_entity.HourlyForecast
			err := rows.Scan(&hf.Time, &hf.WeatherCode, &hf.Temperature2M, &hf.Latitude, &hf.Longitude, &hf.CollectedAt)
			return hf, err
		},
	)

	// Channel between Reader and Writer goroutines.
	dataCh := make(chan weather_entity.HourlyForecast, t.config.ReadBufferSize)
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
		// Open the ParquetWriter.
		if err := t.parquetWriter.Open(gCtx, stepExecution.ExecutionContext); err != nil {
			return fmt.Errorf("failed to open ParquetWriter: %w", err)
		}
		// Defer closing the ParquetWriter to ensure buffered data is written.
		defer func() {
			if err := t.parquetWriter.Close(gCtx); err != nil { // This flushes buffered data.
				logger.Errorf("Failed to close ParquetWriter: %v", err)
			}
		}()

		for {
			select {
			case item, ok := <-dataCh:
				if !ok { // Channel closed and all items processed.
					logger.Infof("Writer Goroutine: Data channel closed. All data processed.")
					return nil // Exit as all data has been written.
				}
				// Write the item using the ParquetWriter. The ParquetWriter handles buffering and flushing.
				if err := t.parquetWriter.Write(gCtx, []weather_entity.HourlyForecast{item}); err != nil {
					return fmt.Errorf("failed to write item to ParquetWriter: %w", err)
				}
			case <-gCtx.Done(): // Context cancellation check.
				logger.Warnf("Writer Goroutine: Context cancelled. ParquetWriter will be closed by deferred call.")
				return gCtx.Err() // The deferred ParquetWriter.Close will be called.
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

// NewHourlyForecastExportTaskletBuilder creates a `jsl.ComponentBuilder` function that constructs `HourlyForecastExportTasklet` instances.
// This builder is designed for use with Fx (Dependency Injection), allowing the tasklet to be configured
// and provided dynamically based on JSL properties.
//
// Parameters:
//
//	dbConnectionResolver: The resolver for database connections, injected by Fx.
//	storageConnectionResolver: The resolver for storage connections, injected by Fx.
//
// Returns:
//
//	A `jsl.ComponentBuilder` function.
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
