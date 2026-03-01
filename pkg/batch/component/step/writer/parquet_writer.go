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

// ParquetWriterConfig holds the configuration for ParquetWriter.
type ParquetWriterConfig struct {
	// StorageRef is the name of the storage connection to use (e.g., "gcs_bucket", "s3_bucket").
	StorageRef string `mapstructure:"storageRef"`
	// OutputBaseDir is the base directory within the storage bucket for exported files (e.g., "weather/hourly_forecast").
	OutputBaseDir string `mapstructure:"outputBaseDir"`
	// CompressionType is the compression type for Parquet files (e.g., "SNAPPY", "GZIP", "NONE").
	CompressionType string `mapstructure:"compressionType"`
}

// ParquetWriter implements the port.ItemWriter interface for writing structured data to Parquet files.
type ParquetWriter[T any] struct {
	name                      string
	config                    *ParquetWriterConfig
	storageConnectionResolver storage.StorageConnectionResolver
	// itemPrototype is a pointer to a zero-value instance of the item type, used for Parquet schema reflection.
	itemPrototype *T
	// partitionKeyFunc is a function to extract the partition key (e.g., "YYYY-MM-DD") from an item.
	partitionKeyFunc func(T) (string, error)

	// Internal writing state
	// storageConn is the resolved storage connection instance.
	storageConn storage.StorageConnection
	// bufferedItems stores items buffered by partition key (e.g., "dt=YYYY-MM-DD").
	bufferedItems map[string][]T
	// totalRecordsBuffered is the total count of all buffered records.
	totalRecordsBuffered int64
	// stepExecutionContext is the execution context managed by the framework.
	stepExecutionContext model.ExecutionContext
}

// NewParquetWriter creates a new instance of ParquetWriter.
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
			"writer",
			fmt.Sprintf("Failed to decode ParquetWriter properties for %s: %v", name, err),
			err,
			false,
			false,
		)
	}

	// Validate required settings
	if config.StorageRef == "" {
		return nil, exception.NewBatchError(
			"writer",
			fmt.Sprintf("ParquetWriter '%s' requires 'storageRef' property.", name),
			nil,
			false,
			false,
		)
	}
	if config.OutputBaseDir == "" {
		return nil, exception.NewBatchError(
			"writer",
			fmt.Sprintf("ParquetWriter '%s' requires 'outputBaseDir' property.", name),
			nil,
			false,
			false,
		)
	}

	// Set default CompressionType
	if config.CompressionType == "" {
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

// NewParquetWriterBuilder generates a function that conforms to the JSL ComponentBuilder signature.
//
// Parameters:
//
//	storageConnectionResolver: Resolver for storage connections.
//	itemPrototype: A prototype instance of the item type for schema reflection.
//	partitionKeyFunc: A function to extract the partition key from an item. (Note: itemPrototype should be *T)
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
		return NewParquetWriter(
			"", // Temporary measure: JSL component name not resolved
			properties,
			storageConnectionResolver,
			itemPrototype,
			partitionKeyFunc,
		)
	}
}

// Open initializes the writer and prepares resources.
// It resolves the storage connection, stores the execution context, and clears internal buffers.
func (w *ParquetWriter[T]) Open(ctx context.Context, ec model.ExecutionContext) error {
	logger.Debugf("ParquetWriter '%s' Open called.", w.name)

	// 1. Resolve the storage connection based on the configured StorageRef using storageConnectionResolver.
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

	// 2. Store the resolved storage.StorageConnection instance internally.
	w.storageConn = conn

	// 3. Save the stepExecutionContext.
	w.stepExecutionContext = ec

	// Clear internal buffers
	w.bufferedItems = make(map[string][]T)
	w.totalRecordsBuffered = 0

	logger.Infof("ParquetWriter '%s' opened successfully. Target storage: %s, Base directory: %s", w.name, w.config.StorageRef, w.config.OutputBaseDir)
	return nil
}

// Write accumulates chunks of received items into an internal buffer.
// It extracts a partition key for each item and adds it to the corresponding buffer.
// No actual Parquet file writing or storage upload occurs in this method.
func (w *ParquetWriter[T]) Write(ctx context.Context, items []T) error {
	logger.Debugf("ParquetWriter '%s' Write called with %d items.", w.name, len(items))

	if w.bufferedItems == nil {
		w.bufferedItems = make(map[string][]T)
	}

	// 1. Iterate through each item in the received items slice.
	for _, item := range items {
		// 2. Extract the partition key for each item using partitionKeyFunc.
		partitionKey, err := w.partitionKeyFunc(item)
		if err != nil {
			return exception.NewBatchError(
				"writer",
				fmt.Sprintf("Failed to get partition key for item in ParquetWriter '%s': %v", w.name, err),
				err,
				false,
				false,
			)
		}

		// 3. Add the item to the internal bufferedItems map based on the extracted partition key.
		w.bufferedItems[partitionKey] = append(w.bufferedItems[partitionKey], item)

		// 4. Update the totalRecordsBuffered counter.
		w.totalRecordsBuffered++
	}

	logger.Debugf("ParquetWriter '%s' buffered %d items. Total buffered: %d.", w.name, len(items), w.totalRecordsBuffered)
	// 5. No actual Parquet file writing or storage upload occurs in this method.
	return nil
}

// Close finalizes all buffered data into Parquet files and uploads them to storage.
// It processes items partition by partition, writes them to Parquet format, and uploads to the configured storage.
func (w *ParquetWriter[T]) Close(ctx context.Context) error {
	logger.Debugf("ParquetWriter '%s' Close called. Total records buffered: %d.", w.name, w.totalRecordsBuffered)

	// Skip processing if no records are buffered.
	if w.totalRecordsBuffered == 0 {
		logger.Infof("ParquetWriter '%s': No records buffered, skipping Parquet file generation.", w.name)
		// If a storage connection was established in Open, close it here.
		if w.storageConn != nil {
			if err := w.storageConn.Close(); err != nil {
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

	// Map compression type string to Parquet codec.
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

	var multiErr error // To aggregate errors if multiple partitions fail.

outerLoop: // Label for the outer loop to continue to the next partition on error.
	// 1. Iterate through the bufferedItems map, processing each partition key.
	for partitionKey, items := range w.bufferedItems {
		logger.Debugf("ParquetWriter '%s': Processing partition '%s' with %d items.", w.name, partitionKey, len(items))

		// 2. Create a new bytes.Buffer for each partition key.
		buf := new(bytes.Buffer)

		// 3. Use xitongsys/parquet-go library's writer.NewParquetWriterFromWriter to create a Parquet writer
		// that writes to the buffer. itemPrototype is used to infer the Parquet schema.
		// The row group size is set to the number of buffered items to create one row group per file.
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

		// 4. Apply the configured CompressionType to the Parquet writer.
		pw.CompressionType = compressionCodec

		// 5. Write all buffered items belonging to that partition to the Parquet writer.
		for _, item := range items {
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

		// 6. Call pw.WriteStop() to finalize the Parquet file.
		// Include defer recover() logic to catch panics from the library and convert them to errors.
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
				multiErr = multierror.Append(multiErr, exception.NewBatchError(
					"writer",
					fmt.Sprintf("Failed to stop Parquet writer for partition '%s' in ParquetWriter '%s': %v", partitionKey, w.name, err),
					err,
					false,
					false,
				))
			}
		}()

		// 7. Generate a Hive-style path (OutputBaseDir/dt=YYYY-MM-DD/) and a unique filename (e.g., data_YYYYMMDD_HHMMSS.parquet).
		// Include timestamp and a unique ID in the filename to avoid collisions.
		fileName := fmt.Sprintf("data_%s_%s.parquet", time.Now().Format("20060102150405"), generateRandomString(8))
		objectName := filepath.Join(w.config.OutputBaseDir, partitionKey, fileName)

		// 8. Use storageConn.Upload to upload the buffer content to the generated path.
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

	// 9. After processing all partitions, clear the internal buffer (bufferedItems) and reset totalRecordsBuffered.
	w.bufferedItems = make(map[string][]T)
	w.totalRecordsBuffered = 0

	// 10. Finally, close the resolved storage connection.
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

// getCompressionCodec returns the Parquet compression codec from a string.
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

// generateRandomString generates a random string of the specified length.
// Used to enhance filename uniqueness.
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
// It stores the provided model.ExecutionContext in the internal stepExecutionContext field.
func (w *ParquetWriter[T]) SetExecutionContext(ctx context.Context, ec model.ExecutionContext) error {
	logger.Debugf("ParquetWriter '%s' SetExecutionContext called.", w.name)
	w.stepExecutionContext = ec
	return nil
}

// GetExecutionContext retrieves the current [model.ExecutionContext] of the writer.
// It returns the internally stored stepExecutionContext.
//
// Parameters:
//
//	ctx: The context for the operation.
//
// Returns:
//
//	model.ExecutionContext: The current [model.ExecutionContext].
//	error: An error if retrieving the context fails.
func (w *ParquetWriter[T]) GetExecutionContext(ctx context.Context) (model.ExecutionContext, error) {
	logger.Debugf("ParquetWriter '%s' GetExecutionContext called.", w.name)
	return w.stepExecutionContext, nil
}

// GetTargetResourceName returns the name of the target resource this writer writes to.
// It returns the value of the configured StorageRef.
func (w *ParquetWriter[T]) GetTargetResourceName() string {
	return w.config.StorageRef
}

// GetResourcePath returns the path or identifier within the target resource this writer writes to.
// It returns the value of the configured OutputBaseDir.
func (w *ParquetWriter[T]) GetResourcePath() string {
	return w.config.OutputBaseDir
}

// Verify that [ParquetWriter] satisfies the [port.ItemWriter] interface at compile time.
var _ port.ItemWriter[any] = (*ParquetWriter[any])(nil)
