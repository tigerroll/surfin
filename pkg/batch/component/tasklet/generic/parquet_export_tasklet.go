package generic

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"reflect"
	"sync" // Add this import
	"time"

	"github.com/mitchellh/mapstructure"

	"github.com/tigerroll/surfin/pkg/batch/adapter/database"
	"github.com/tigerroll/surfin/pkg/batch/adapter/storage"
	reader "github.com/tigerroll/surfin/pkg/batch/component/step/reader" // Add this import
	writer "github.com/tigerroll/surfin/pkg/batch/component/step/writer"
	coreAdapter "github.com/tigerroll/surfin/pkg/batch/core/adapter"
	"github.com/tigerroll/surfin/pkg/batch/core/application/port"
	coreConfig "github.com/tigerroll/surfin/pkg/batch/core/config"
	configjsl "github.com/tigerroll/surfin/pkg/batch/core/config/jsl"
	"github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	"github.com/tigerroll/surfin/pkg/batch/support/util/exception"
	"github.com/tigerroll/surfin/pkg/batch/support/util/logger"
)

// GenericParquetExportTaskletConfig holds the configuration for GenericParquetExportTasklet.
type GenericParquetExportTaskletConfig struct {
	DbRef                  string `mapstructure:"dbRef"`
	StorageRef             string `mapstructure:"storageRef"`
	OutputBaseDir          string `mapstructure:"outputBaseDir"`
	ReadBufferSize         int    `mapstructure:"readBufferSize"`
	ParquetCompressionType string `mapstructure:"parquetCompressionType"`
	TableName              string `mapstructure:"tableName"`
	SQLSelectColumns       string `mapstructure:"sqlSelectColumns"`
	SQLOrderBy             string `mapstructure:"sqlOrderBy"`
	PartitionKeyColumn     string `mapstructure:"partitionKeyColumn"`
	PartitionKeyFormat     string `mapstructure:"partitionKeyFormat"`
}

// GenericParquetExportTasklet implements the port.Tasklet interface for exporting data to Parquet files.
type GenericParquetExportTasklet[T any] struct {
	config                    *GenericParquetExportTaskletConfig
	dbConnectionResolver      database.DBConnectionResolver
	storageConnectionResolver storage.StorageConnectionResolver
	parquetWriter             port.ItemWriter[T]
	// scanFunc is injected by Fx and scans sql.Rows into type T.
	scanFunc func(rows *sql.Rows) (T, error)
	// partitionKeyFunc is dynamically generated to extract the partition key.
	partitionKeyFunc func(T) (string, error)
	// stepExecutionContext holds the tasklet's execution context.
	stepExecutionContext model.ExecutionContext
}

// NewGenericParquetExportTasklet creates a new instance of GenericParquetExportTasklet.
func NewGenericParquetExportTasklet[T any](
	properties map[string]string,
	dbConnectionResolver database.DBConnectionResolver,
	storageConnectionResolver storage.StorageConnectionResolver,
	itemPrototype *T, // itemPrototype is a prototype instance of the item type, injected by Fx for Parquet schema inference.
	scanFunc func(rows *sql.Rows) (T, error), // scanFunc is the database scan function, injected by Fx.
) (port.Tasklet, error) {
	var config GenericParquetExportTaskletConfig

	// Use mapstructure.NewDecoder to enable WeaklyTypedInput.
	decoderConfig := &mapstructure.DecoderConfig{
		Metadata:         nil,
		Result:           &config,
		TagName:          "mapstructure", // Explicitly use the "mapstructure" tag.
		WeaklyTypedInput: true,           // Allow converting strings to numeric types.
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
		return nil, exception.NewBatchError(
			"tasklet",
			fmt.Sprintf("Failed to decode GenericParquetExportTasklet properties: %v", err),
			err,
			false,
			false,
		)
	}

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
	if config.TableName == "" { // Set default for ReadBufferSize.
		return nil, exception.NewBatchError("tasklet", "tableName is required for GenericParquetExportTasklet", nil, false, false)
	}
	if config.SQLSelectColumns == "" {
		return nil, exception.NewBatchError("tasklet", "sqlSelectColumns is required for GenericParquetExportTasklet", nil, false, false)
	}
	if config.PartitionKeyColumn == "" {
		return nil, exception.NewBatchError("tasklet", "partitionKeyColumn is required for GenericParquetExportTasklet", nil, false, false)
	}
	if config.PartitionKeyFormat == "" {
		return nil, exception.NewBatchError("tasklet", "partitionKeyFormat is required for GenericParquetExportTasklet", nil, false, false)
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
	// Based on JSL's PartitionKeyColumn and PartitionKeyFormat,
	// it retrieves the value from the specified field of itemPrototype and formats it.
	// The field type is expected to be time.Time or int64 (Unix milliseconds).
	partitionKeyFunc := func(item T) (string, error) {
		val := reflect.ValueOf(item)
		// If T is a pointer, dereference it
		if val.Kind() == reflect.Ptr {
			val = val.Elem()
		}

		if val.Kind() != reflect.Struct {
			return "", exception.NewBatchError(
				"tasklet",
				fmt.Sprintf("PartitionKeyColumn '%s' can only be applied to struct types, got %s", config.PartitionKeyColumn, val.Kind()),
				nil,
				false,
				false,
			)
		}

		field := val.FieldByName(config.PartitionKeyColumn)
		if !field.IsValid() {
			return "", exception.NewBatchError(
				"tasklet",
				fmt.Sprintf("PartitionKeyColumn '%s' not found in item type %T", config.PartitionKeyColumn, item),
				nil,
				false,
				false,
			)
		}

		switch field.Kind() {
		case reflect.Struct:
			if t, ok := field.Interface().(time.Time); ok {
				return t.Format(config.PartitionKeyFormat), nil
			}
			return "", exception.NewBatchError(
				"tasklet",
				fmt.Sprintf("Unsupported type for PartitionKeyColumn '%s': struct (not time.Time)", config.PartitionKeyColumn),
				nil,
				false,
				false,
			)
		case reflect.Int64:
			// Assuming int64 is Unix milliseconds
			unixMilli := field.Int()
			t := time.Unix(0, unixMilli*int64(time.Millisecond))
			return t.Format(config.PartitionKeyFormat), nil
		default:
			return "", exception.NewBatchError(
				"tasklet",
				fmt.Sprintf("Unsupported type for PartitionKeyColumn '%s': %s. Expected time.Time or int64.", config.PartitionKeyColumn, field.Kind()),
				nil,
				false,
				false,
			)
		}
	}

	// Initialize parquetWriter.
	// Construct properties for writer.NewParquetWriter.
	parquetWriterProps := map[string]string{
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
		scanFunc:                  scanFunc,
		parquetWriter:             pw,               // Assign the created writer
		partitionKeyFunc:          partitionKeyFunc, // Assign the generated function
	}, nil
}

// NewGenericParquetExportTaskletBuilder generates a function that conforms to the JSL ComponentBuilder signature.
//
// Parameters:
//
//	dbConnectionResolver: Resolver for database connections.
//	storageConnectionResolver: Resolver for storage connections.
//	itemPrototype: A prototype instance of the item type for schema reflection.
//	scanFunc: A function to scan sql.Rows into type T.
//
// Returns:
//
//	A function that can create a GenericParquetExportTasklet instance based on properties.
func NewGenericParquetExportTaskletBuilder[T any](
	dbConnectionResolver database.DBConnectionResolver,
	storageConnectionResolver storage.StorageConnectionResolver,
	itemPrototype *T, // A pointer to a zero-value instance of the item type for schema reflection.
	scanFunc func(rows *sql.Rows) (T, error),
) configjsl.ComponentBuilder { // Uses the JSL ComponentBuilder type.
	return func(
		cfg *coreConfig.Config, // Part of the JSL ComponentBuilder signature.
		resolver port.ExpressionResolver, // Part of the JSL ComponentBuilder signature.
		resourceProviders map[string]coreAdapter.ResourceProvider, // Part of the JSL ComponentBuilder signature.
		properties map[string]string,
	) (interface{}, error) {
		// Call NewGenericParquetExportTasklet, passing captured dependencies and provided properties.
		// The return value is port.Tasklet, which can be returned as interface{}.
		return NewGenericParquetExportTasklet[T](
			properties,
			dbConnectionResolver,
			storageConnectionResolver,
			itemPrototype,
			scanFunc,
		)
	}
}

// Open initializes the tasklet and prepares resources.
func (t *GenericParquetExportTasklet[T]) Open(ctx context.Context, ec model.ExecutionContext) error {
	logger.Debugf("GenericParquetExportTasklet Open called.")
	// Call Open on parquetWriter.
	if err := t.parquetWriter.Open(ctx, ec); err != nil {
		return exception.NewBatchError(
			"tasklet",
			fmt.Sprintf("Failed to open ParquetWriter: %v", err),
			err,
			false,
			false,
		)
	}
	t.stepExecutionContext = ec // Save the execution context
	return nil
}

// Execute contains the core logic of the tasklet.
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
	sqlReader := reader.NewSqlCursorReader[T](
		sqlDB,
		fmt.Sprintf("%s_sql_reader", stepExecution.StepName), // Unique name for the reader
		sqlQuery,
		nil, // No additional arguments for the query as it's fully constructed
		t.scanFunc,
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
func (t *GenericParquetExportTasklet[T]) Close(ctx context.Context) error {
	logger.Debugf("GenericParquetExportTasklet Close called.")
	// Call Close on parquetWriter.
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
func (t *GenericParquetExportTasklet[T]) SetExecutionContext(ctx context.Context, ec model.ExecutionContext) error {
	logger.Debugf("GenericParquetExportTasklet SetExecutionContext called.")
	t.stepExecutionContext = ec
	// Set ExecutionContext for ParquetWriter as well.
	if t.parquetWriter != nil {
		if err := t.parquetWriter.SetExecutionContext(ctx, ec); err != nil {
			return exception.NewBatchError(
				"tasklet",
				fmt.Sprintf("Failed to set execution context for ParquetWriter: %v", err),
				err,
				false,
				false,
			)
		}
	}
	return nil
}

// GetExecutionContext retrieves the current execution context of the tasklet.
func (t *GenericParquetExportTasklet[T]) GetExecutionContext(ctx context.Context) (model.ExecutionContext, error) {
	logger.Debugf("GenericParquetExportTasklet GetExecutionContext called.")
	return t.stepExecutionContext, nil
}

// Compile-time check to ensure GenericParquetExportTasklet implements port.Tasklet.
var _ port.Tasklet = (*GenericParquetExportTasklet[any])(nil)
