package reader

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"strconv"

	"github.com/tigerroll/surfin/pkg/batch/core/domain/model"
)

const readCountKey = "csv.reader.readCount"

// Option defines a functional option for configuring CsvCursorReader.
type Option[T any] func(*CsvCursorReader[T])

// WithComma sets the field delimiter for the CSV reader.
func WithComma[T any](comma rune) Option[T] {
	return func(r *CsvCursorReader[T]) {
		r.comma = comma
	}
}

// WithLazyQuotes enables or disables lazy quote handling.
func WithLazyQuotes[T any](lazy bool) Option[T] {
	return func(r *CsvCursorReader[T]) {
		r.lazyQuotes = lazy
	}
}

// CsvCursorReader is a streaming reader for CSV data, designed for efficient
// processing of large datasets by keeping memory usage constant.
type CsvCursorReader[T any] struct {
	reader     io.Reader
	csvReader  *csv.Reader
	name       string
	readCount  int
	mapper     func([]string) (T, error)
	ec         model.ExecutionContext
	comma      rune
	lazyQuotes bool
}

// NewCsvCursorReader creates a new instance of CsvCursorReader.
func NewCsvCursorReader[T any](reader io.Reader, name string, mapper func([]string) (T, error), opts ...Option[T]) *CsvCursorReader[T] {
	r := &CsvCursorReader[T]{
		reader: reader,
		name:   name,
		mapper: mapper,
		comma:  ',', // Default value
	}

	// Apply options
	for _, opt := range opts {
		opt(r)
	}

	return r
}

// Open initializes the reader and sets up the CSV parser.
// It restores the state from the ExecutionContext if a read count is present,
// allowing the reader to resume from the last processed record.
func (r *CsvCursorReader[T]) Open(ctx context.Context, ec model.ExecutionContext) error {
	r.ec = ec
	r.csvReader = csv.NewReader(r.reader)
	r.csvReader.Comma = r.comma
	r.csvReader.LazyQuotes = r.lazyQuotes

	// Retrieve readCount from ExecutionContext
	val, ok := ec.Get(readCountKey)
	if ok {
		if strVal, ok := val.(string); ok {
			parsed, err := strconv.Atoi(strVal)
			if err != nil {
				return fmt.Errorf("invalid readCount format in ExecutionContext: %w", err)
			}
			r.readCount = parsed
		} else if intVal, ok := val.(int); ok {
			r.readCount = intVal
		}
	}

	// Skip records to resume from the last read position
	for i := 0; i < r.readCount; i++ {
		_, err := r.csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to skip records during Open: %w", err)
		}
	}

	return nil
}

// Read reads the next record, maps it to type T using the provided mapper,
// and updates the internal read count in the ExecutionContext.
func (r *CsvCursorReader[T]) Read(ctx context.Context) (T, error) {
	record, err := r.csvReader.Read()
	if err != nil {
		var zero T
		return zero, err
	}

	item, err := r.mapper(record)
	if err != nil {
		var zero T
		return zero, fmt.Errorf("failed to map csv record: %w", err)
	}

	r.readCount++
	// Persist the current read count to the ExecutionContext
	r.ec[readCountKey] = strconv.Itoa(r.readCount)

	return item, nil
}

// Close releases any resources associated with the underlying reader if it implements io.Closer.
func (r *CsvCursorReader[T]) Close(ctx context.Context) error {
	if closer, ok := r.reader.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}
