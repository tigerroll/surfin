package writer

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"path/filepath"
	"strings"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/mitchellh/mapstructure"
	"github.com/xitongsys/parquet-go/parquet"
	"github.com/xitongsys/parquet-go/writer"

	"github.com/tigerroll/surfin/pkg/batch/adapter/storage"
	"github.com/tigerroll/surfin/pkg/batch/core/application/port"
	"github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	"github.com/tigerroll/surfin/pkg/batch/support/util/exception"
	"github.com/tigerroll/surfin/pkg/batch/support/util/logger"
)

// ParquetWriterConfig holds the configuration for a ParquetWriter.
type ParquetWriterConfig struct {
	// StorageRef is the name of the storage connection to use (e.g., "gcs_bucket", "s3_bucket").
	// This reference is resolved to an actual storage connection at runtime.
	StorageRef string `mapstructure:"storageRef"`
	// OutputBaseDir is the base directory within the storage bucket where Parquet files will be stored
	// (e.g., "weather/hourly_forecast").
	OutputBaseDir string `mapstructure:"outputBaseDir"`
	// CompressionType specifies the compression algorithm to use for Parquet files (e.g., "SNAPPY", "GZIP", "NONE").
	// If not specified, "SNAPPY" is used as the default.
	CompressionType string `mapstructure:"compressionType"`
}

// ParquetWriter implements the port.ItemWriter interface for writing structured data to Parquet files.
// It buffers items in memory, partitioned by a key derived from the item, and writes them to
// Parquet files in the configured storage upon Close.
type ParquetWriter[T any] struct {
	name                      string
	config                    *ParquetWriterConfig
	storageConnectionResolver storage.StorageConnectionResolver
	// itemPrototype is a pointer to a zero-value instance of the item type.
	// It is used by the Parquet library for schema reflection to determine the Parquet file structure.
	itemPrototype *T
	// partitionKeyFunc is a function that extracts a partition key (e.g., "dt=YYYY-MM-DD") from an item.
	// Items with the same partition key are grouped together into a single Parquet file.
	partitionKeyFunc func(T) (string, error)

	// storageConn holds the active storage connection instance, resolved during the Open phase.
	storageConn storage.StorageConnection
	// bufferedItems stores items in memory, grouped by their partition key.
	// The map key is the partition key (e.g., "dt=YYYY-MM-DD"), and the value is a slice of items
	// belonging to that partition.
	bufferedItems map[string][]T
	// totalRecordsBuffered tracks the total number of items currently held in the buffer across all partitions.
	totalRecordsBuffered int64
	// stepExecutionContext holds the current execution context for the step, provided by the framework.
	// It can be used to store and retrieve state relevant to the step's execution.
	stepExecutionContext model.ExecutionContext
}

// NewParquetWriter creates a new instance of ParquetWriter.
// It initializes the writer with the given name, properties, storage resolver,
// item prototype for schema reflection, and a function to determine the partition key.
//
// Parameters:
//
//	name: The unique name of the writer.
//	properties: Configuration properties for the writer.
//	storageConnectionResolver: Resolver for storage connections.
//	itemPrototype: A prototype instance of the item type for schema reflection.
//	partitionKeyFunc: A function to extract the partition key from an item.
//
// Returns:
//
//	A port.ItemWriter instance and an error if creation fails.
func NewParquetWriter[T any](
	name string,
	properties map[string]interface{},
	storageConnectionResolver storage.StorageConnectionResolver,
	itemPrototype *T, // A pointer to a zero-value instance of the item type for schema reflection.
	partitionKeyFunc func(T) (string, error),
) (port.ItemWriter[T], error) {
	var config ParquetWriterConfig
	if err := mapstructure.Decode(properties, &config); err != nil {
		return nil, exception.NewBatchError(
			"writer", // Module
			fmt.Sprintf("Failed to decode ParquetWriter properties for '%s': %v", name, err),
			err,
			false,
			false,
		)
	}

	// Validate required settings
	if config.StorageRef == "" {
		return nil, exception.NewBatchError( // Module
			"writer",
			fmt.Sprintf("ParquetWriter '%s' requires 'storageRef' property to be set.", name),
			nil,
			false,
			false,
		)
	}
	if config.OutputBaseDir == "" {
		return nil, exception.NewBatchError( // Module
			"writer",
			fmt.Sprintf("ParquetWriter '%s' requires 'outputBaseDir' property to be set.", name),
			nil,
			false,
			false,
		)
	}

	// Set default CompressionType
	if config.CompressionType == "" { // Default to SNAPPY if not specified
		config.CompressionType = "SNAPPY"
	}

	return &ParquetWriter[T]{
		name:                      name,
		config:                    &config,
		storageConnectionResolver: storageConnectionResolver,
		itemPrototype:             itemPrototype,
		partitionKeyFunc:          partitionKeyFunc,
		bufferedItems:             make(map[string][]T), // Initialize
		totalRecordsBuffered:      0,                    // Initialize
	}, nil
}

// NewParquetWriterBuilder generates a function that conforms to the JSL ComponentBuilder signature for ParquetWriter.
// This builder function can be used by the batch framework to dynamically create ParquetWriter instances
// based on configuration properties.
//
// Parameters:
//
//	storageConnectionResolver: The resolver responsible for providing storage connections.
//	itemPrototype: A prototype instance of the item type for schema reflection.
//	partitionKeyFunc: A function to extract the partition key from an item.
//
// Returns:
//
//	A function that can create a ParquetWriter instance based on properties.
func NewParquetWriterBuilder[T any](
	storageConnectionResolver storage.StorageConnectionResolver,
	itemPrototype *T, // A pointer to a zero-value instance of the item type for schema reflection.
	partitionKeyFunc func(T) (string, error),
) func(properties map[string]interface{}) (port.ItemWriter[T], error) {
	return func(properties map[string]interface{}) (port.ItemWriter[T], error) {
		return NewParquetWriter( // Call the main constructor
			"", // Temporary measure: JSL component name not resolved
			properties,
			storageConnectionResolver,
			itemPrototype,
			partitionKeyFunc,
		)
	}
}

// Open initializes the ParquetWriter by resolving the necessary storage connection,
// storing the provided execution context, and preparing internal buffers for item accumulation.
//
// Parameters:
//
//	ctx: The context for the operation, typically used for cancellation and deadlines.
//	ec: The current execution context for the step, which can be used to store and retrieve state.
//
// Returns:
//
//	An error if the storage connection cannot be resolved or any other initialization fails.
func (w *ParquetWriter[T]) Open(ctx context.Context, ec model.ExecutionContext) error {
	logger.Debugf("ParquetWriter '%s': Open called.", w.name)

	// Resolve the storage connection based on the configured StorageRef.
	conn, err := w.storageConnectionResolver.ResolveStorageConnection(ctx, w.config.StorageRef)
	if err != nil {
		return exception.NewBatchError(
			"writer",
			fmt.Sprintf("Failed to resolve storage connection '%s' for ParquetWriter '%s': %v", w.config.StorageRef, w.name, err),
			err,
			false,
			false,
		)
	}

	// Store the resolved storage.StorageConnection instance.
	w.storageConn = conn

	// Save the stepExecutionContext.
	w.stepExecutionContext = ec

	// Initialize/clear internal buffers for item accumulation.
	w.bufferedItems = make(map[string][]T)
	w.totalRecordsBuffered = 0

	logger.Infof("ParquetWriter '%s': Opened successfully. Target storage: '%s', Base directory: '%s'", w.name, w.config.StorageRef, w.config.OutputBaseDir)
	return nil
}

// Write accumulates a chunk of received items into an internal buffer.
// For each item, it extracts a partition key using the configured `partitionKeyFunc`
// and adds the item to the corresponding buffer.
// This method does not perform any actual Parquet file writing or storage uploads;
// these operations are deferred until the `Close` method is called.
//
// Parameters:
//
//	ctx: The context for the operation.
//	items: A slice of items to be written.
//
// Returns:
//
//	An error if a partition key cannot be extracted from an item.
func (w *ParquetWriter[T]) Write(ctx context.Context, items []T) error {
	if w.bufferedItems == nil {
		w.bufferedItems = make(map[string][]T)
	}

	// Iterate through each item in the received items slice.
	for _, item := range items {
		// Extract the partition key for each item using partitionKeyFunc.
		partitionKey, err := w.partitionKeyFunc(item)
		if err != nil { // If partition key extraction fails, return an error.
			return exception.NewBatchError(
				"writer",
				fmt.Sprintf("Failed to get partition key for item in ParquetWriter '%s': %v", w.name, err),
				err,
				false,
				false,
			)
		}

		// Add the item to the internal bufferedItems map, grouped by partition key.
		w.bufferedItems[partitionKey] = append(w.bufferedItems[partitionKey], item)

		// Increment the total count of buffered records.
		w.totalRecordsBuffered++
	}

	logger.Debugf("ParquetWriter '%s': Buffered %d items. Total buffered: %d.", w.name, len(items), w.totalRecordsBuffered)
	// No actual Parquet file writing or storage upload occurs in this method; items are just buffered.
	return nil
}

// Close finalizes the writing process by converting all buffered data into Parquet files
// and uploading them to the configured storage.
// It iterates through each partition, creates a Parquet file for the items in that partition,
// and then uploads the file. Any errors encountered during this process are aggregated.
//
// Parameters:
//
//	ctx: The context for the operation.
//
// Returns:
//
//	An error if any part of the finalization or upload process fails.
func (w *ParquetWriter[T]) Close(ctx context.Context) error {
	logger.Debugf("ParquetWriter '%s' Close called. Total records buffered: %d.", w.name, w.totalRecordsBuffered)

	// Skip processing if no records are buffered.
	if w.totalRecordsBuffered == 0 {
		logger.Infof("ParquetWriter '%s': No records buffered, skipping Parquet file generation.", w.name)
		// If a storage connection was established in Open, close it here.
		if w.storageConn != nil {
			if err := w.storageConn.Close(); err != nil { // Ensure the connection is closed even if no data was written.
				return exception.NewBatchError(
					"writer",
					fmt.Sprintf("Failed to close storage connection for ParquetWriter '%s': %v", w.name, err),
					err,
					false,
					false,
				)
			}
		}
		return nil
	}

	// Determine the Parquet compression codec from the configuration string.
	compressionCodec, err := getCompressionCodec(w.config.CompressionType)
	if err != nil {
		return exception.NewBatchError(
			"writer",
			fmt.Sprintf("Invalid compression type '%s' for ParquetWriter '%s': %v", w.config.CompressionType, w.name, err),
			err,
			false,
			false,
		)
	}

	var multiErr error // Used to aggregate multiple errors that might occur during processing.

outerLoop: // Label for the outer loop, allowing to continue to the next partition on error.
	// Iterate through the bufferedItems map, processing each partition.
	for partitionKey, items := range w.bufferedItems {
		logger.Debugf("ParquetWriter '%s': Processing partition '%s' with %d items.", w.name, partitionKey, len(items))

		// Create a new bytes.Buffer to hold the Parquet file content for the current partition.
		buf := new(bytes.Buffer)

		// Create a Parquet writer that writes to the buffer.
		// itemPrototype is used for schema inference. The row group size is set to the number of items
		// to ensure all items for this partition are in a single row group.
		pw, err := writer.NewParquetWriterFromWriter(buf, w.itemPrototype, int64(len(items)))
		if err != nil {
			multiErr = multierror.Append(multiErr, exception.NewBatchError(
				"writer",
				fmt.Sprintf("Failed to create Parquet writer for partition '%s' in ParquetWriter '%s': %v", partitionKey, w.name, err),
				err,
				false,
				false,
			))
			continue outerLoop // Move to the next partition.
		}
		// Apply the configured compression type.
		pw.CompressionType = compressionCodec

		// Write all buffered items for the current partition to the Parquet writer.
		for _, item := range items {
			// If an individual item write fails, record the error and skip the rest of this partition.
			if err := pw.Write(item); err != nil {
				// Consider individual item write errors as fatal for the current partition,
				// record the error, and break processing for this partition.
				multiErr = multierror.Append(multiErr, exception.NewBatchError(
					"writer",
					fmt.Sprintf("Failed to write item to Parquet for partition '%s' in ParquetWriter '%s': %v", partitionKey, w.name, err),
					err,
					false,
					false,
				))
				continue outerLoop // Break from inner loop and move to the next partition.
			}
		}

		// Finalize the Parquet file by calling WriteStop().
		// A defer-recover block is used to catch potential panics from the Parquet library and convert them into errors.
		func() {
			defer func() {
				if r := recover(); r != nil {
					err := fmt.Errorf("Parquet writer panicked during WriteStop for partition '%s' in ParquetWriter '%s': %v", partitionKey, w.name, r)
					multiErr = multierror.Append(multiErr, exception.NewBatchError(
						"writer",
						err.Error(),
						err,
						false,
						false,
					))
					logger.Errorf("ParquetWriter '%s': Recovered from panic during WriteStop: %v", w.name, r)
				}
			}()
			if err := pw.WriteStop(); err != nil {
				// Workaround for potential nil pointer dereference issues in the Parquet library's WriteStop.
				if strings.Contains(err.Error(), "invalid memory address or nil pointer dereference") {
					// If it's a nil pointer dereference, re-panic to be caught by the defer recover() block.
					// If it's a nil pointer dereference error, re-panic it so the defer recover() can catch it.
					// This ensures the error is logged as a panic recovery and added to multiErr.
					panic(fmt.Sprintf("Parquet writer returned nil pointer dereference error: %v", err))
				}
				multiErr = multierror.Append(multiErr, exception.NewBatchError(
					"writer",
					fmt.Sprintf("Failed to stop Parquet writer for partition '%s' in ParquetWriter '%s': %v", partitionKey, w.name, err),
					err,
					false,
					false,
				))
			}
		}()

		// Generate a Hive-style path (OutputBaseDir/partitionKey/) and a unique filename.
		// The filename includes a timestamp and a random string to ensure uniqueness.
		fileName := fmt.Sprintf("data_%s_%s.parquet", time.Now().Format("20060102150405"), generateRandomString(8))
		objectName := filepath.Join(w.config.OutputBaseDir, partitionKey, fileName)

		// Upload the generated Parquet file content to the configured storage.
		logger.Debugf("ParquetWriter '%s': Uploading %d bytes to %s/%s", w.name, buf.Len(), w.config.StorageRef, objectName)
		if err := w.storageConn.Upload(ctx, w.config.StorageRef, objectName, buf, "application/octet-stream"); err != nil {
			multiErr = multierror.Append(multiErr, exception.NewBatchError(
				"writer",
				fmt.Sprintf("Failed to upload Parquet file for partition '%s' to '%s' in ParquetWriter '%s': %v", partitionKey, objectName, w.name, err),
				err,
				false,
				false,
			))
		} else {
			logger.Infof("ParquetWriter '%s': Successfully uploaded Parquet file for partition '%s' to %s", w.name, partitionKey, objectName)
		}
	}

	// After processing all partitions, clear the internal buffer and reset the record count.
	w.bufferedItems = make(map[string][]T)
	w.totalRecordsBuffered = 0

	// Finally, close the resolved storage connection.
	if w.storageConn != nil {
		if err := w.storageConn.Close(); err != nil {
			multiErr = multierror.Append(multiErr, exception.NewBatchError(
				"writer",
				fmt.Sprintf("Failed to close storage connection for ParquetWriter '%s': %v", w.name, err),
				err,
				false,
				false,
			))
		}
	}

	return multiErr
}

// getCompressionCodec converts a string representation of a compression type
// (e.g., "SNAPPY", "GZIP", "NONE") into its corresponding Parquet compression codec enum.
// It returns an error for unsupported compression types.
func getCompressionCodec(compressionType string) (parquet.CompressionCodec, error) {
	switch strings.ToUpper(compressionType) {
	case "SNAPPY":
		return parquet.CompressionCodec_SNAPPY, nil
	case "GZIP":
		return parquet.CompressionCodec_GZIP, nil
	case "NONE", "": // NONE or empty string means uncompressed
		return parquet.CompressionCodec_UNCOMPRESSED, nil
	default:
		return 0, fmt.Errorf("unsupported compression type: %s", compressionType)
	}
}

// generateRandomString generates a cryptographically insecure random string of the specified length.
// It is primarily used to enhance filename uniqueness for output files.
func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	seededRand := rand.New(rand.NewSource(time.Now().UnixNano()))
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}

// SetExecutionContext sets the execution context for the writer.
// This method is typically called by the batch framework to provide the current
// step execution context, allowing the writer to store or retrieve state.
//
// Parameters:
//
//	ctx: The context for the operation.
//	ec: The [model.ExecutionContext] to be set.
//
// Returns:
//
//	An error if the context cannot be set (though typically this method does not fail).
func (w *ParquetWriter[T]) SetExecutionContext(ctx context.Context, ec model.ExecutionContext) error {
	logger.Debugf("ParquetWriter '%s': SetExecutionContext called.", w.name)
	w.stepExecutionContext = ec
	return nil
}

// GetExecutionContext retrieves the current [model.ExecutionContext] associated with the writer.
// This context typically holds state relevant to the current step's execution.
//
// Parameters:
//
//	ctx: The context for the operation.
//
// Returns:
//
//	The current [model.ExecutionContext].
//	An error if the context cannot be retrieved (though typically this method does not fail).
func (w *ParquetWriter[T]) GetExecutionContext(ctx context.Context) (model.ExecutionContext, error) {
	logger.Debugf("ParquetWriter '%s': GetExecutionContext called.", w.name)
	return w.stepExecutionContext, nil
}

// GetTargetResourceName returns the name of the target resource connection
// that this writer uses for its output. This corresponds to the `StorageRef` configuration.
func (w *ParquetWriter[T]) GetTargetResourceName() string {
	return w.config.StorageRef
}

// GetResourcePath returns the base path or identifier within the target resource
// where this writer stores its output files. This corresponds to the `OutputBaseDir` configuration.
func (w *ParquetWriter[T]) GetResourcePath() string {
	return w.config.OutputBaseDir
}

// Verify that [ParquetWriter] satisfies the [port.ItemWriter] interface at compile time.
var _ port.ItemWriter[any] = (*ParquetWriter[any])(nil)
