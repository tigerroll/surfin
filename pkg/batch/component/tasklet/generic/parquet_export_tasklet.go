// Package generic provides general-purpose tasklet implementations.
// These tasklets are designed to be reusable across various batch jobs.
package generic

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/mitchellh/mapstructure"

	"github.com/tigerroll/surfin/pkg/batch/adapter/database"
	"github.com/tigerroll/surfin/pkg/batch/adapter/storage"
	reader "github.com/tigerroll/surfin/pkg/batch/component/step/reader"
	writer "github.com/tigerroll/surfin/pkg/batch/component/step/writer"
	coreAdapter "github.com/tigerroll/surfin/pkg/batch/core/adapter"
	"github.com/tigerroll/surfin/pkg/batch/core/application/port"
	coreConfig "github.com/tigerroll/surfin/pkg/batch/core/config"
	configjsl "github.com/tigerroll/surfin/pkg/batch/core/config/jsl"
	"github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	"github.com/tigerroll/surfin/pkg/batch/support/util/exception"
	"github.com/tigerroll/surfin/pkg/batch/support/util/logger"
)

// GenericParquetExportTaskletConfig holds the configuration for [GenericParquetExportTasklet].
type GenericParquetExportTaskletConfig struct {
	// DbRef is the name of the database connection to use for reading data.
	DbRef string `mapstructure:"dbRef"`
	// StorageRef is the name of the storage connection to use for writing Parquet files.
	StorageRef string `mapstructure:"storageRef"`
	// OutputBaseDir is the base directory within the storage bucket for exported files (e.g., "weather/hourly_forecast").
	OutputBaseDir string `mapstructure:"outputBaseDir"`
	// ReadBufferSize is the number of items to read from the database and process in a single batch.
	ReadBufferSize int `mapstructure:"readBufferSize"`
	// ParquetCompressionType is the compression type for Parquet files (e.g., "SNAPPY", "GZIP", "NONE").
	ParquetCompressionType string `mapstructure:"parquetCompressionType"`
	// TableName is the name of the database table to read from.
	TableName string `mapstructure:"tableName"`
	// SQLSelectColumns is a comma-separated string of columns to select from the database table.
	SQLSelectColumns string `mapstructure:"sqlSelectColumns"`
	// SQLOrderBy is an optional ORDER BY clause for the SQL query.
	SQLOrderBy string `mapstructure:"sqlOrderBy"`
	// PartitionDefinitions defines multiple partition keys and their formats.
	// Each definition corresponds to a level in the hierarchical partition path.
	PartitionDefinitions []PartitionDefinition `mapstructure:"partitionDefinitions"`
}

// PartitionDefinition defines a single partition key within a multi-level partitioning scheme.
type PartitionDefinition struct {
	// Key is the name of the struct field to use for this partition.
	Key string `mapstructure:"key"`
	// Format is the format string for the partition key.
	// For time.Time fields, use Go's reference time layout (e.g., "2006-01-02", "15").
	// For other types, it can be a format specifier for fmt.Sprintf (e.g., "dt=%s", "code=%d").
	// If empty, the value will be converted to string using its default string representation.
	Format string `mapstructure:"format"`
	// Prefix is an optional prefix for the partition directory name (e.g., "dt=", "hour=").
	// If empty, the key name will be used as prefix.
	Prefix string `mapstructure:"prefix"`
}

// GenericParquetExportTasklet implements the [port.Tasklet] interface for exporting data from a database
// to Parquet files in a specified storage location. It supports dynamic schema inference based on a
// provided item prototype and partitioning based on a configurable column.
type GenericParquetExportTasklet[T any] struct {
	config                    *GenericParquetExportTaskletConfig
	dbConnectionResolver      database.DBConnectionResolver
	storageConnectionResolver storage.StorageConnectionResolver
	parquetWriter             port.ItemWriter[T]
	// partitionKeyFunc is dynamically generated to extract the partition key from an item of type T.
	partitionKeyFunc func(T) (string, error)
	// stepExecutionContext holds the tasklet's execution context, managed by the framework.
	stepExecutionContext model.ExecutionContext
}

// NewGenericParquetExportTasklet creates a new [GenericParquetExportTasklet] instance.
//
// Parameters:
//
//	properties: Configuration properties for the tasklet, typically from JSL.
//	dbConnectionResolver: Resolver for database connections.
//	storageConnectionResolver: Resolver for storage connections.
//	itemPrototype: A pointer to a zero-value instance of the item type (T). This is used for
//	               Parquet schema inference and for dynamically generating the partition key function.
//
// Returns:
//
//	port.Tasklet: A new instance of [GenericParquetExportTasklet].
//	error: An error if initialization or configuration decoding fails.
func NewGenericParquetExportTasklet[T any](
	properties map[string]interface{},
	dbConnectionResolver database.DBConnectionResolver,
	storageConnectionResolver storage.StorageConnectionResolver,
	itemPrototype *T, // itemPrototype is a prototype instance of the item type, injected by Fx for Parquet schema inference.
) (port.Tasklet, error) {
	var config GenericParquetExportTaskletConfig
	var metadata mapstructure.Metadata

	logger.Debugf("NewGenericParquetExportTasklet received properties before decode: %+v", properties)

	if pd, ok := properties["partitionDefinitions"]; ok {
		logger.Debugf("DEBUG: Type of properties[\"partitionDefinitions\"]: %T", pd)
		if slicePd, isSlice := pd.([]interface{}); isSlice {
			for i, item := range slicePd {
				logger.Debugf("DEBUG:   Element %d of partitionDefinitions: Type=%T, Value=%+v", i, item, item)
			}
		}
	}

	decoderConfig := &mapstructure.DecoderConfig{
		Metadata:         &metadata,
		Result:           &config,
		TagName:          "mapstructure",
		WeaklyTypedInput: true,
	}
	decoder, err := mapstructure.NewDecoder(decoderConfig)
	if err != nil {
		return nil, exception.NewBatchError(
			"tasklet",
			fmt.Sprintf("Failed to create mapstructure decoder for GenericParquetExportTasklet: %v", err),
			err,
			false,
			false,
		)
	}

	if err := decoder.Decode(properties); err != nil {
		logger.Errorf("Failed to decode GenericParquetExportTasklet properties. Input properties: %+v", properties)
		return nil, exception.NewBatchError(
			"tasklet",
			fmt.Sprintf("Failed to decode GenericParquetExportTasklet properties: %v", err),
			err,
			false,
			false,
		)
	}

	if len(metadata.Unused) > 0 {
		logger.Warnf("Mapstructure unused keys for GenericParquetExportTasklet: %v", metadata.Unused)
	}
	logger.Debugf("NewGenericParquetExportTasklet decoded config: %+v", config)
	logger.Debugf("Decoded config.PartitionDefinitions length: %d", len(config.PartitionDefinitions))

	// Validate required configurations.
	if config.DbRef == "" {
		return nil, exception.NewBatchError("tasklet", "dbRef is required for GenericParquetExportTasklet", nil, false, false)
	}
	if config.StorageRef == "" {
		return nil, exception.NewBatchError("tasklet", "storageRef is required for GenericParquetExportTasklet", nil, false, false)
	}
	if config.OutputBaseDir == "" {
		return nil, exception.NewBatchError("tasklet", "outputBaseDir is required for GenericParquetExportTasklet", nil, false, false)
	}
	if config.TableName == "" {
		return nil, exception.NewBatchError("tasklet", "tableName is required for GenericParquetExportTasklet", nil, false, false)
	}
	if config.SQLSelectColumns == "" {
		return nil, exception.NewBatchError("tasklet", "sqlSelectColumns is required for GenericParquetExportTasklet", nil, false, false)
	}

	// Set default for ReadBufferSize.
	if config.ReadBufferSize == 0 {
		config.ReadBufferSize = 1000 // Default value.
	}

	// Set default for ParquetCompressionType.
	if config.ParquetCompressionType == "" {
		config.ParquetCompressionType = "SNAPPY" // Default value.
	}

	// Dynamically generate partitionKeyFunc using reflection.
	// This function now iterates through PartitionDefinitions to build a multi-level partition path.
	partitionKeyFunc := func(item T) (string, error) {
		val := reflect.ValueOf(item)
		if val.Kind() == reflect.Ptr {
			val = val.Elem()
		}
		if val.Kind() != reflect.Struct {
			return "", exception.NewBatchError(
				"tasklet",
				fmt.Sprintf("Item type must be a struct or a pointer to a struct, got %s", val.Kind()),
				nil,
				false,
				false,
			)
		}

		if len(config.PartitionDefinitions) == 0 {
			// If no partition definitions are provided, return an empty string for the partition path.
			// The writer will then write to the base directory without partitioning.
			return "", nil
		}

		var partitionParts []string
		for _, def := range config.PartitionDefinitions {
			field := val.FieldByName(def.Key)
			if !field.IsValid() {
				return "", exception.NewBatchError(
					"tasklet",
					fmt.Sprintf("Partition key field '%s' not found in item type %T", def.Key, item),
					nil,
					false,
					false,
				)
			}
			if !field.CanInterface() {
				return "", exception.NewBatchError(
					"tasklet",
					fmt.Sprintf("Partition key field '%s' in item type %T is not exportable (unexported field)", def.Key, item),
					nil,
					false,
					false,
				)
			}

			var formattedValue string
			switch field.Kind() {
			case reflect.Struct:
				if t, ok := field.Interface().(time.Time); ok {
					if def.Format == "" {
						// Default format for time.Time if not specified
						formattedValue = t.Format("2006-01-02T15-04-05")
					} else {
						formattedValue = t.Format(def.Format)
					}
				} else {
					return "", exception.NewBatchError(
						"tasklet",
						fmt.Sprintf("Unsupported struct type for partition key '%s': %s. Expected time.Time.", def.Key, field.Type()),
						nil,
						false,
						false,
					)
				}
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				// Check if the type is specifically UnixMillis (e.g., from weather_entity.go)
				if field.Type().Name() == "UnixMillis" {
					unixMilli := field.Int()
					t := time.Unix(0, unixMilli*int64(time.Millisecond))
					if def.Format == "" {
						// Default format for UnixMillis if format is empty
						formattedValue = t.Format("2006-01-02T15-04-05")
					} else {
						formattedValue = t.Format(def.Format)
					}
				} else {
					// For other integer types (like WeatherCode), use fmt.Sprintf
					if def.Format == "" {
						formattedValue = fmt.Sprintf("%v", field.Interface())
					} else {
						formattedValue = fmt.Sprintf(def.Format, field.Interface())
					}
				}
			case reflect.Float32, reflect.Float64:
				if def.Format == "" {
					formattedValue = fmt.Sprintf("%v", field.Interface())
				} else {
					formattedValue = fmt.Sprintf(def.Format, field.Interface())
				}
			case reflect.String:
				if def.Format == "" {
					formattedValue = field.String()
				} else {
					// For string types, use fmt.Sprintf with the interface value
					formattedValue = fmt.Sprintf(def.Format, field.Interface())
				}
			default:
				// Fallback for other primitive types or if format is not specified
				if def.Format == "" {
					formattedValue = fmt.Sprintf("%v", field.Interface())
				} else {
					formattedValue = fmt.Sprintf(def.Format, field.Interface())
				}
			}

			prefix := def.Prefix
			if prefix == "" {
				// If prefix is not specified, use the lowercased key name followed by "="
				prefix = fmt.Sprintf("%s=", strings.ToLower(def.Key))
			}
			partitionParts = append(partitionParts, fmt.Sprintf("%s%s", prefix, formattedValue))
		}

		return strings.Join(partitionParts, "/"), nil
	}

	// Initialize parquetWriter.
	// Construct properties for writer.NewParquetWriter.
	parquetWriterProps := map[string]interface{}{
		"storageRef":      config.StorageRef,
		"outputBaseDir":   config.OutputBaseDir,
		"compressionType": config.ParquetCompressionType,
	}

	pw, err := writer.NewParquetWriter[T](
		"genericParquetExportTaskletWriter", // Name for the writer
		parquetWriterProps,
		storageConnectionResolver,
		itemPrototype,
		partitionKeyFunc, // Use the dynamically generated function
	)
	if err != nil {
		return nil, exception.NewBatchError(
			"tasklet",
			fmt.Sprintf("Failed to create ParquetWriter: %v", err),
			err,
			false,
			false,
		)
	}

	return &GenericParquetExportTasklet[T]{
		config:                    &config,
		dbConnectionResolver:      dbConnectionResolver,
		storageConnectionResolver: storageConnectionResolver,
		parquetWriter:             pw,
		partitionKeyFunc:          partitionKeyFunc,
	}, nil
}

// NewGenericParquetExportTaskletBuilder generates a function that conforms to the JSL [configjsl.ComponentBuilder] signature.
//
// Parameters:
//
//	dbConnectionResolver: Resolver for database connections.
//	storageConnectionResolver: Resolver for storage connections.
//	itemPrototype: A prototype instance of the item type for schema reflection.
//
// Returns:
//
//	A function that can create a GenericParquetExportTasklet instance based on properties.
func NewGenericParquetExportTaskletBuilder[T any](
	dbConnectionResolver database.DBConnectionResolver,
	storageConnectionResolver storage.StorageConnectionResolver,
	itemPrototype *T, // A pointer to a zero-value instance of the item type for schema reflection.
) configjsl.ComponentBuilder { // Uses the JSL ComponentBuilder type.
	return func(
		cfg *coreConfig.Config, // Part of the JSL ComponentBuilder signature.
		resolver port.ExpressionResolver, // Part of the JSL ComponentBuilder signature.
		resourceProviders map[string]coreAdapter.ResourceProvider, // Part of the JSL ComponentBuilder signature.
		properties map[string]interface{},
	) (interface{}, error) {
		// Call NewGenericParquetExportTasklet, passing captured dependencies and provided properties.
		// The return value is port.Tasklet, which can be returned as interface{}.
		return NewGenericParquetExportTasklet[T](
			properties,
			dbConnectionResolver,
			storageConnectionResolver,
			itemPrototype,
		)
	}
}

// Open initializes the tasklet and prepares resources.
// It calls the [port.ItemWriter.Open] method on the internal [parquetWriter] to prepare it for writing.
// The [model.ExecutionContext] from the [model.StepExecution] is saved for internal use.
//
// Parameters:
//
//	ctx: The context for the operation.
//	stepExecution: The current [model.StepExecution] instance, containing the execution context.
//
// Returns:
//
//	error: An error if the internal [parquetWriter] fails to open.
func (t *GenericParquetExportTasklet[T]) Open(ctx context.Context, stepExecution *model.StepExecution) error {
	logger.Debugf("GenericParquetExportTasklet Open called.")
	if err := t.parquetWriter.Open(ctx, stepExecution.ExecutionContext); err != nil {
		return exception.NewBatchError(
			"tasklet",
			fmt.Sprintf("Failed to open ParquetWriter: %v", err),
			err,
			false,
			false,
		)
	}
	t.stepExecutionContext = stepExecution.ExecutionContext // Save the execution context
	return nil
}

// Execute contains the core logic of the tasklet.
// It reads data from the configured database table using a [reader.SqlCursorReader],
// processes it, and writes it to Parquet files via the internal [parquetWriter].
// The reading and writing operations are performed concurrently using goroutines and channels.
//
// Parameters:
//
//	ctx: The context for the operation.
//	stepExecution: The current [model.StepExecution] instance.
//
// Returns:
//
//	model.ExitStatus: The exit status of the tasklet (e.g., [model.ExitStatusCompleted] or [model.ExitStatusFailed]).
//	error: An error if any critical operation (e.g., database connection, read, write) fails.
func (t *GenericParquetExportTasklet[T]) Execute(ctx context.Context, stepExecution *model.StepExecution) (model.ExitStatus, error) {
	logger.Debugf("GenericParquetExportTasklet Execute called.")

	// 1. Resolve the database connection
	dbConn, err := t.dbConnectionResolver.ResolveDBConnection(ctx, t.config.DbRef)
	if err != nil {
		return model.ExitStatusFailed, exception.NewBatchError(
			"tasklet",
			fmt.Sprintf("Failed to resolve database connection '%s': %v", t.config.DbRef, err),
			err,
			false,
			false,
		)
	}
	// Ensure the connection is closed when the tasklet finishes execution
	defer func() {
		if closeErr := dbConn.Close(); closeErr != nil {
			logger.Errorf("Failed to close database connection '%s': %v", t.config.DbRef, closeErr)
		}
	}()

	// Get the underlying *sql.DB instance
	sqlDB, err := dbConn.GetSQLDB() // Use GetSQLDB() as per the updated interface
	if err != nil {
		return model.ExitStatusFailed, exception.NewBatchError(
			"tasklet",
			fmt.Sprintf("Failed to get underlying *sql.DB from connection '%s': %v", t.config.DbRef, err),
			err,
			false,
			false,
		)
	}

	// 2. Construct the SQL query
	sqlQuery := fmt.Sprintf("SELECT %s FROM %s", t.config.SQLSelectColumns, t.config.TableName)
	if t.config.SQLOrderBy != "" {
		sqlQuery = fmt.Sprintf("%s ORDER BY %s", sqlQuery, t.config.SQLOrderBy)
	}
	logger.Debugf("Constructed SQL query: %s", sqlQuery)

	// 9. Initialize SqlCursorReader
	// The name for SqlCursorReader should be unique per step to ensure correct state management in ExecutionContext.
	// We use the step name for this purpose.

	// Dynamically generate scanFunc using dbConn.ScanRowsToStruct
	scanFunc := func(rows *sql.Rows) (T, error) {
		var item T
		if err := dbConn.ScanRowsToStruct(rows, &item); err != nil {
			return item, err
		}
		return item, nil
	}

	sqlReader := reader.NewSqlCursorReader[T](
		sqlDB,
		fmt.Sprintf("%s_sql_reader", stepExecution.StepName), // Unique name for the reader
		sqlQuery,
		nil,      // No additional arguments for the query as it's fully constructed
		scanFunc, // Use the dynamically generated scanFunc
	)

	// Open the reader
	if err := sqlReader.Open(ctx, stepExecution.ExecutionContext); err != nil {
		// If it's io.EOF, it means no data, which is not an error for the tasklet.
		// With the fix to SqlCursorReader, io.EOF is now returned by Read(), not Open().
		// So, this check is no longer needed here.
		return model.ExitStatusFailed, exception.NewBatchError(
			"tasklet",
			fmt.Sprintf("Failed to open SqlCursorReader for step '%s': %v", stepExecution.StepName, err),
			err,
			false,
			false,
		)
	}
	// Ensure the reader is closed
	defer func() {
		if closeErr := sqlReader.Close(ctx); closeErr != nil {
			logger.Errorf("Failed to close SqlCursorReader for step '%s': %v", stepExecution.StepName, closeErr)
		}
	}()

	// 10. Implement concurrent processing pipeline
	itemsChan := make(chan T, t.config.ReadBufferSize) // Buffered channel for items
	errChan := make(chan error, 2)                     // Channel to signal errors from goroutines
	var wg sync.WaitGroup

	// Reader Goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(itemsChan) // Close the channel when reading is done

		for {
			item, err := sqlReader.Read(ctx)
			if err != nil {
				if err == io.EOF {
					logger.Debugf("SqlCursorReader for step '%s' finished reading.", stepExecution.StepName)
					return // Reading complete
				}
				// Other errors
				select {
				case errChan <- exception.NewBatchError(
					"tasklet",
					fmt.Sprintf("Failed to read item from SqlCursorReader for step '%s': %v", stepExecution.StepName, err),
					err,
					false,
					false,
				):
				case <-ctx.Done():
					// Context cancelled, stop trying to send error
				}
				return // Error occurred, stop reading
			}

			select {
			case itemsChan <- item:
				// Item sent successfully
			case <-ctx.Done():
				// Context cancelled, stop reading
				logger.Warnf("Context cancelled during item reading for step '%s'.", stepExecution.StepName)
				select {
				case errChan <- ctx.Err():
				default:
				}
				return
			}
		}
	}()

	// Writer Goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		batch := make([]T, 0, t.config.ReadBufferSize) // Use ReadBufferSize as batch size

		for item := range itemsChan { // Loop until itemsChan is closed
			batch = append(batch, item)
			if len(batch) >= t.config.ReadBufferSize {
				if err := t.parquetWriter.Write(ctx, batch); err != nil {
					select {
					case errChan <- exception.NewBatchError(
						"tasklet",
						fmt.Sprintf("Failed to write batch to ParquetWriter for step '%s': %v", stepExecution.StepName, err),
						err,
						false,
						false,
					):
					case <-ctx.Done():
					}
					return // Error occurred, stop writing
				}
				batch = make([]T, 0, t.config.ReadBufferSize) // Reset batch
			}
		}

		// Write any remaining items in the batch after the channel is closed
		if len(batch) > 0 {
			if err := t.parquetWriter.Write(ctx, batch); err != nil {
				select {
				case errChan <- exception.NewBatchError(
					"tasklet",
					fmt.Sprintf("Failed to write final batch to ParquetWriter for step '%s': %v", stepExecution.StepName, err),
					err,
					false,
					false,
				):
				case <-ctx.Done():
				}
				return
			}
		}
		logger.Debugf("ParquetWriter for step '%s' finished writing all items.", stepExecution.StepName)
	}()

	wg.Wait() // Wait for both reader and writer goroutines to finish

	close(errChan) // Close errChan after all goroutines are done
	var pipelineErr error
	for err := range errChan {
		if err != nil {
			pipelineErr = err // Just take the first error, or aggregate them
			break
		}
	}

	if pipelineErr != nil {
		return model.ExitStatusFailed, pipelineErr
	}

	logger.Infof("GenericParquetExportTasklet for step '%s' completed successfully.", stepExecution.StepName)
	return model.ExitStatusCompleted, nil
}

// Close releases resources used by the tasklet.
// It calls the [port.ItemWriter.Close] method on the internal [parquetWriter] to finalize any pending writes
// and release resources held by the writer.
//
// Parameters:
//
//	ctx: The context for the operation.
//	stepExecution: The current [model.StepExecution] instance.
//
// Returns:
//
//	error: An error if the internal [parquetWriter] fails to close.
func (t *GenericParquetExportTasklet[T]) Close(ctx context.Context, stepExecution *model.StepExecution) error {
	logger.Debugf("GenericParquetExportTasklet Close called.")
	if err := t.parquetWriter.Close(ctx); err != nil {
		return exception.NewBatchError(
			"tasklet",
			fmt.Sprintf("Failed to close ParquetWriter: %v", err),
			err,
			false,
			false,
		)
	}
	return nil
}

// SetExecutionContext sets the execution context for the tasklet.
// This method is called by the framework to provide the tasklet with its current execution context.
// It also propagates the execution context to the internal [parquetWriter].
//
// Parameters:
//
//	ec: The [model.ExecutionContext] to set.
func (t *GenericParquetExportTasklet[T]) SetExecutionContext(ec model.ExecutionContext) { // This method signature is for port.Tasklet
	logger.Debugf("GenericParquetExportTasklet SetExecutionContext called.")
	t.stepExecutionContext = ec
	if t.parquetWriter != nil {
		if err := t.parquetWriter.SetExecutionContext(context.Background(), ec); err != nil {
			logger.Warnf("Failed to set ExecutionContext for ParquetWriter: %v", err)
		}
	}
}

// GetExecutionContext retrieves the current [model.ExecutionContext] of the tasklet.
//
// Returns:
//
//	model.ExecutionContext: The current [model.ExecutionContext].
func (t *GenericParquetExportTasklet[T]) GetExecutionContext() model.ExecutionContext {
	logger.Debugf("GenericParquetExportTasklet GetExecutionContext called.")
	return t.stepExecutionContext
}

// Compile-time check to ensure GenericParquetExportTasklet implements port.Tasklet.
var _ port.Tasklet = (*GenericParquetExportTasklet[any])(nil)
