// Package writer provides implementations of the [port.ItemWriter] interface.
package writer

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/go-multierror"
	"github.com/mitchellh/mapstructure"
	parquet "github.com/parquet-go/parquet-go"

	"github.com/tigerroll/surfin/pkg/batch/adapter/storage"
	"github.com/tigerroll/surfin/pkg/batch/core/application/port"
	"github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	"github.com/tigerroll/surfin/pkg/batch/support/util/exception"
	"github.com/tigerroll/surfin/pkg/batch/support/util/logger"
)

// ParquetWriterConfig holds the configuration for a ParquetWriter, including storage and compression settings.
type ParquetWriterConfig struct {
	// StorageRef is the identifier for the storage connection to use.
	StorageRef string `mapstructure:"storageRef"`
	// OutputBaseDir is the base directory path within the storage bucket for Parquet files.
	OutputBaseDir string `mapstructure:"outputBaseDir"`
	// CompressionType specifies the compression algorithm (e.g., "SNAPPY", "GZIP", "NONE").
	// Defaults to "SNAPPY".
	CompressionType string `mapstructure:"compressionType"`
	// FileNameFormat is the pattern for generating output file names.
	// Supported placeholders: #{tablename}, #{timestamp}, #{uuid}, #{random}, #{sequence}.
	FileNameFormat string `mapstructure:"fileNameFormat"`
	// TableName is the table name used in the file name generation.
	TableName string `mapstructure:"tableName"`
}

// ParquetWriter implements [port.ItemWriter] to buffer items in memory, partition them,
// and write them to Parquet files upon flushing.
type ParquetWriter[T any] struct {
	name                      string
	config                    *ParquetWriterConfig
	storageConnectionResolver storage.StorageConnectionResolver
	itemPrototype             *T
	partitionKeyFunc          func(T) (string, error)
	storageConn               storage.StorageAdapter
	bufferedItems             map[string][]T
	stepExecutionContext      model.ExecutionContext
	partitionSequence         map[string]int
}

// NewParquetWriter initializes a new ParquetWriter instance with the provided configuration and dependencies.
//
// Parameters:
//   - name: A unique name for the writer instance.
//   - properties: Configuration properties for the writer.
//   - storageConnectionResolver: Resolver to obtain the storage connection.
//   - itemPrototype: A prototype instance of the item type T.
//   - partitionKeyFunc: A function to determine the partition key for an item.
func NewParquetWriter[T any](
	name string,
	properties map[string]interface{},
	storageConnectionResolver storage.StorageConnectionResolver,
	itemPrototype *T,
	partitionKeyFunc func(T) (string, error),
) (port.ItemWriter[T], error) {
	var config ParquetWriterConfig
	if err := mapstructure.Decode(properties, &config); err != nil {
		return nil, exception.NewBatchError(
			"writer",
			fmt.Sprintf("Failed to decode ParquetWriter properties for '%s': %v", name, err),
			err,
			false,
			false,
		)
	}

	if config.StorageRef == "" {
		return nil, exception.NewBatchError(
			"writer",
			fmt.Sprintf("ParquetWriter '%s' requires 'storageRef' property to be set.", name),
			nil,
			false,
			false,
		)
	}
	if config.OutputBaseDir == "" {
		return nil, exception.NewBatchError(
			"writer",
			fmt.Sprintf("ParquetWriter '%s' requires 'outputBaseDir' property to be set.", name),
			nil,
			false,
			false,
		)
	}

	if config.CompressionType == "" {
		config.CompressionType = "SNAPPY"
	}

	return &ParquetWriter[T]{
		name:                      name,
		config:                    &config,
		storageConnectionResolver: storageConnectionResolver,
		itemPrototype:             itemPrototype,
		partitionKeyFunc:          partitionKeyFunc,
		bufferedItems:             make(map[string][]T),
		partitionSequence:         make(map[string]int),
	}, nil
}

// NewParquetWriterBuilder returns a builder function for creating ParquetWriter instances.
//
// Parameters:
//   - storageConnectionResolver: Resolver to obtain the storage connection.
//   - itemPrototype: A prototype instance of the item type T.
//   - partitionKeyFunc: A function to determine the partition key for an item.
func NewParquetWriterBuilder[T any](
	storageConnectionResolver storage.StorageConnectionResolver,
	itemPrototype *T,
	partitionKeyFunc func(T) (string, error),
) func(properties map[string]interface{}) (port.ItemWriter[T], error) {
	return func(properties map[string]interface{}) (port.ItemWriter[T], error) {
		return NewParquetWriter(
			"",
			properties,
			storageConnectionResolver,
			itemPrototype,
			partitionKeyFunc,
		)
	}
}

// Verify that [ParquetWriter] implements the [port.IdempotentWriter] interface at compile time.
var _ port.IdempotentWriter[any] = (*ParquetWriter[any])(nil)

// IsIdempotent returns false because ParquetWriter does not guarantee idempotency by default.
func (w *ParquetWriter[T]) IsIdempotent() bool {
	return false
}

// Open resolves the storage connection and prepares internal buffers.
//
// Parameters:
//   - ctx: The context for the operation.
//   - ec: The ExecutionContext associated with the current step.
func (w *ParquetWriter[T]) Open(ctx context.Context, ec model.ExecutionContext) error {
	logger.Debugf("ParquetWriter '%s': Open called.", w.name)

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

	w.storageConn = conn
	w.stepExecutionContext = ec
	w.bufferedItems = make(map[string][]T)

	logger.Infof("ParquetWriter '%s': Opened successfully. Target storage: '%s', Base directory: '%s'", w.name, w.config.StorageRef, w.config.OutputBaseDir)
	return nil
}

// Update updates the state (checkpoint) after a chunk is committed.
//
// Parameters:
//   - ctx: The context for the operation.
//   - ec: The ExecutionContext associated with the current step.
func (w *ParquetWriter[T]) Update(ctx context.Context, ec model.ExecutionContext) error {
	return nil
}

// Write buffers items for later writing. It does not perform I/O operations until Flush is called.
//
// Parameters:
//   - ctx: The context for the operation.
//   - items: A slice of items to be buffered.
func (w *ParquetWriter[T]) Write(ctx context.Context, items []T) error {
	if w.bufferedItems == nil {
		w.bufferedItems = make(map[string][]T)
	}

	for _, item := range items {
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

		w.bufferedItems[partitionKey] = append(w.bufferedItems[partitionKey], item)
	}

	logger.Debugf("ParquetWriter '%s': Buffered %d items.", w.name, len(items))
	return nil
}

// Flush writes all buffered items to Parquet files in the configured storage.
// It partitions items based on the partitionKeyFunc, generates files, and uploads them.
// If an error occurs during writing or uploading, it returns a multierror containing all encountered errors.
//
// Parameters:
//   - ctx: The context for the operation.
func (w *ParquetWriter[T]) Flush(ctx context.Context) error {
	logger.Debugf("ParquetWriter '%s' Flush called. Items in buffer: %d.", w.name, len(w.bufferedItems))

	if len(w.bufferedItems) == 0 {
		logger.Debugf("ParquetWriter '%s': No items buffered, skipping Parquet file generation during Flush.", w.name)
		return nil
	}

	// Prepare compression options based on the configuration.
	var compression parquet.WriterOption
	switch strings.ToUpper(w.config.CompressionType) {
	case "GZIP":
		compression = parquet.Compression(&parquet.Gzip)
	case "NONE":
		compression = parquet.Compression(&parquet.Uncompressed)
	case "SNAPPY":
		fallthrough
	default:
		compression = parquet.Compression(&parquet.Snappy)
	}

	var multiErr error

	for partitionKey, items := range w.bufferedItems {
		logger.Debugf("ParquetWriter '%s': Processing partition '%s' with %d items.", w.name, partitionKey, len(items))

		buf := new(bytes.Buffer)

		// Create the Parquet writer. Rely on type inference for the schema.
		pw := parquet.NewGenericWriter[T](buf, compression)

		writeSuccessful := true

		if _, err := pw.Write(items); err != nil {
			logger.Errorf("ParquetWriter '%s': Failed to write items to Parquet for partition '%s': %v", w.name, partitionKey, err)
			multiErr = multierror.Append(multiErr, err)
			writeSuccessful = false
		}

		if err := pw.Close(); err != nil {
			logger.Errorf("ParquetWriter '%s': Failed to close Parquet writer for partition '%s': %v", w.name, partitionKey, err)
			multiErr = multierror.Append(multiErr, err)
			writeSuccessful = false
		}

		if writeSuccessful {
			// Generate file name
			fileName := w.generateFileName(partitionKey)
			fullPath := filepath.Join(w.config.OutputBaseDir, fileName)

			// Upload to storage
			if err := w.storageConn.Upload(ctx, "", fullPath, buf, "application/x-parquet"); err != nil {
				logger.Errorf("ParquetWriter '%s': Failed to upload Parquet file '%s': %v", w.name, fullPath, err)
				multiErr = multierror.Append(multiErr, err)
			} else {
				logger.Infof("ParquetWriter '%s': Successfully uploaded Parquet file '%s'.", w.name, fullPath)
			}
		}
	}

	// Clear buffer after flush
	w.bufferedItems = make(map[string][]T)

	return multiErr
}

// generateFileName generates a file name based on the configured format.
func (w *ParquetWriter[T]) generateFileName(partitionKey string) string {
	format := w.config.FileNameFormat
	if format == "" {
		format = "#{tablename}_#{timestamp}_#{uuid}.parquet"
	}

	// Replace placeholders
	format = strings.ReplaceAll(format, "#{tablename}", w.config.TableName)
	format = strings.ReplaceAll(format, "#{timestamp}", time.Now().Format("20060102150405"))
	format = strings.ReplaceAll(format, "#{uuid}", uuid.New().String())
	format = strings.ReplaceAll(format, "#{random}", fmt.Sprintf("%d", rand.Intn(10000)))
	format = strings.ReplaceAll(format, "#{sequence}", fmt.Sprintf("%d", w.partitionSequence[partitionKey]))

	w.partitionSequence[partitionKey]++

	return format
}

// Close releases resources.
//
// Parameters:
//   - ctx: The context for the operation.
func (w *ParquetWriter[T]) Close(ctx context.Context) error {
	logger.Debugf("ParquetWriter '%s': Close called.", w.name)
	return nil
}

// SetExecutionContext sets the ExecutionContext.
//
// Parameters:
//   - ctx: The context for the operation.
//   - ec: The ExecutionContext to set.
func (w *ParquetWriter[T]) SetExecutionContext(ctx context.Context, ec model.ExecutionContext) error {
	w.stepExecutionContext = ec
	return nil
}

// GetExecutionContext retrieves the ExecutionContext.
//
// Parameters:
//   - ctx: The context for the operation.
//
// Returns:
//   - model.ExecutionContext: The current ExecutionContext.
//   - error: An error if retrieval fails.
func (w *ParquetWriter[T]) GetExecutionContext(ctx context.Context) (model.ExecutionContext, error) {
	return w.stepExecutionContext, nil
}

// GetTargetResourceName returns the target storage name.
//
// Returns:
//   - string: The name of the target storage.
func (w *ParquetWriter[T]) GetTargetResourceName() string {
	return w.config.StorageRef
}

// GetResourcePath returns the base directory path.
//
// Returns:
//   - string: The base directory path.
func (w *ParquetWriter[T]) GetResourcePath() string {
	return w.config.OutputBaseDir
}
