package reader_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tigerroll/surfin/pkg/batch/component/step/reader"
	"github.com/tigerroll/surfin/pkg/batch/core/domain/model"
)

// MockCloser is a helper struct that implements io.Reader and io.Closer for testing resource cleanup.
type MockCloser struct {
	io.Reader
	closed bool
}

// Close marks the mock as closed.
func (m *MockCloser) Close() error {
	m.closed = true
	return nil
}

// TestCsvCursorReader_NormalRead verifies that the reader correctly parses CSV data and handles EOF.
func TestCsvCursorReader_NormalRead(t *testing.T) {
	csvData := "id,name\n1,Alice\n2,Bob"
	r := reader.NewCsvCursorReader(strings.NewReader(csvData), "test-reader", func(record []string) (string, error) {
		return record[1], nil
	})

	err := r.Open(context.Background(), model.NewExecutionContext())
	assert.NoError(t, err)

	// 1st record (header)
	item1, err := r.Read(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, "name", item1)

	// 2nd record
	item2, err := r.Read(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, "Alice", item2)

	// 3rd record
	item3, err := r.Read(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, "Bob", item3)

	// EOF
	_, err = r.Read(context.Background())
	assert.Equal(t, io.EOF, err)
}

// TestCsvCursorReader_Restartability verifies that the reader correctly resumes from a specific record count using the ExecutionContext.
func TestCsvCursorReader_Restartability(t *testing.T) {
	csvData := "1\n2\n3\n4"
	r := reader.NewCsvCursorReader(strings.NewReader(csvData), "test-reader", func(record []string) (string, error) {
		return record[0], nil
	})

	// Simulate state where 2 records have already been processed
	ec := model.NewExecutionContext()
	ec["csv.reader.readCount"] = 2

	err := r.Open(context.Background(), ec)
	assert.NoError(t, err)

	// Should start from the 3rd record
	item, err := r.Read(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, "3", item)

	// Next is the 4th record
	item, err = r.Read(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, "4", item)
}

// TestCsvCursorReader_MapperError verifies that the reader correctly propagates errors returned by the mapper function.
func TestCsvCursorReader_MapperError(t *testing.T) {
	csvData := "data"
	r := reader.NewCsvCursorReader(strings.NewReader(csvData), "test-reader", func(record []string) (string, error) {
		return "", errors.New("mapper error")
	})

	err := r.Open(context.Background(), model.NewExecutionContext())
	assert.NoError(t, err)

	_, err = r.Read(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to map csv record")
}

// TestCsvCursorReader_Close verifies that the reader correctly closes the underlying resource.
func TestCsvCursorReader_Close(t *testing.T) {
	mock := &MockCloser{Reader: strings.NewReader("data")}
	r := reader.NewCsvCursorReader(mock, "test-reader", func(record []string) (string, error) {
		return record[0], nil
	})

	err := r.Close(context.Background())
	assert.NoError(t, err)
	assert.True(t, mock.closed, "Close() should have been called on the underlying reader")
}
