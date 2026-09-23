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

// ParquetWriter implements port.ItemWriter to buffer items in memory, partition them,
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

// Open resolves the storage connection and prepares internal buffers.
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
func (w *ParquetWriter[T]) Update(ctx context.Context, ec model.ExecutionContext) error {
	return nil
}

// Write buffers items for later writing. It does not perform I/O operations until Flush is called.
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
			multiErr = multierror.Append(multiErr, exception.NewBatchError(
				"writer",
				fmt.Sprintf("Failed to write items to Parquet for partition '%s' in ParquetWriter '%s': %v", partitionKey, w.name, err),
				err,
				false,
				false,
			))
			writeSuccessful = false
		}

		// Close the writer to finalize the file. This writes the footer and metadata.
		// We perform strict error handling; if closing fails, mark the write as unsuccessful.
		if writeSuccessful {
			if err := pw.Close(); err != nil {
				logger.Errorf("ParquetWriter '%s': Failed to close Parquet writer for partition '%s': %v", w.name, partitionKey, err)
				multiErr = multierror.Append(multiErr, exception.NewBatchError(
					"writer",
					fmt.Sprintf("Failed to close Parquet writer for partition '%s' in ParquetWriter '%s': %v", partitionKey, w.name, err),
					err,
					false,
					false,
				))
				writeSuccessful = false
			}
		}

		// Generate the output file name based on the configured format.
		fileName := w.config.FileNameFormat
		if fileName == "" {
			fileName = "data_#{timestamp}_#{random}.parquet"
		}

		fileName = strings.ReplaceAll(fileName, "#{tablename}", w.config.TableName)
		fileName = strings.ReplaceAll(fileName, "#{timestamp}", time.Now().Format("20060102150405"))
		fileName = strings.ReplaceAll(fileName, "#{random}", generateRandomString(8))
		fileName = strings.ReplaceAll(fileName, "#{uuid}", uuid.New().String())

		w.partitionSequence[partitionKey]++
		fileName = strings.ReplaceAll(fileName, "#{sequence}", fmt.Sprintf("%05d", w.partitionSequence[partitionKey]))

		objectName := filepath.Join(w.config.OutputBaseDir, partitionKey, fileName)

		// Upload the generated Parquet file to the configured storage.
		targetBucketName := w.storageConn.Config().BucketName
		logger.Debugf("ParquetWriter '%s': Uploading %d bytes to %s/%s", w.name, buf.Len(), targetBucketName, objectName)
		if writeSuccessful && buf.Len() > 0 {
			if err := w.storageConn.Upload(ctx, targetBucketName, objectName, buf, "application/octet-stream"); err != nil {
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
		} else {
			logger.Warnf("ParquetWriter '%s': Skipping upload for partition '%s' due to unsuccessful write or empty buffer.", w.name, partitionKey)
		}
	}

	w.bufferedItems = make(map[string][]T)

	return multiErr
}

// Close finalizes the writing process by flushing remaining data and closing the storage connection.
func (w *ParquetWriter[T]) Close(ctx context.Context) error {
	logger.Debugf("ParquetWriter '%s' Close called.", w.name)

	var flushErr error
	if err := w.Flush(ctx); err != nil {
		flushErr = err
		logger.Errorf("ParquetWriter '%s': Error during final Flush in Close: %v", w.name, err)
	}

	var closeErr error
	if w.storageConn != nil {
		if err := w.storageConn.Close(); err != nil {
			closeErr = exception.NewBatchError(
				"writer",
				fmt.Sprintf("Failed to close storage connection for ParquetWriter '%s': %v", w.name, err),
				err,
				false,
				false,
			)
		}
	}

	if flushErr != nil {
		return flushErr
	}
	return closeErr
}

// generateRandomString generates a random alphanumeric string of the specified length.
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
func (w *ParquetWriter[T]) SetExecutionContext(ctx context.Context, ec model.ExecutionContext) error {
	logger.Debugf("ParquetWriter '%s': SetExecutionContext called.", w.name)
	w.stepExecutionContext = ec
	return nil
}

// GetExecutionContext retrieves the current execution context.
func (w *ParquetWriter[T]) GetExecutionContext(ctx context.Context) (model.ExecutionContext, error) {
	logger.Debugf("ParquetWriter '%s': GetExecutionContext called.", w.name)
	return w.stepExecutionContext, nil
}

// GetTargetResourceName returns the name of the target storage resource.
func (w *ParquetWriter[T]) GetTargetResourceName() string {
	return w.config.StorageRef
}

// GetResourcePath returns the base path within the target resource.
func (w *ParquetWriter[T]) GetResourcePath() string {
	return w.config.OutputBaseDir
}

// Verify that [ParquetWriter] satisfies the [port.ItemWriter] interface at compile time.
var _ port.ItemWriter[any] = (*ParquetWriter[any])(nil)

// Verify that [ParquetWriter] satisfies the [port.ItemFlusher] interface at compile time.
var _ port.ItemFlusher = (*ParquetWriter[any])(nil)

// Verify that [ParquetWriter] satisfies the [port.ItemStream] interface at compile time.
var _ port.ItemStream = (*ParquetWriter[any])(nil)
