// Package exception provides custom error types and error handling utilities for the Surfin Batch Framework.
// It standardizes errors that occur during batch processing, allowing them to be categorized
// based on retry and skip policies.
package exception

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"sync"
)

// TemporaryNetworkError represents a transient network error.
type TemporaryNetworkError struct{}

func (e *TemporaryNetworkError) Error() string { return "temporary network error" }

// DataConversionError represents an error during data conversion.
type DataConversionError struct{}

func (e *DataConversionError) Error() string { return "data conversion error" }

// errorRegistry maps error names referenced in JSL to concrete Go error instances.
// It holds error instances (singletons) for comparison using errors.Is.
var errorRegistry = make(map[string]error)

// registryMutex protects access to errorRegistry.
var registryMutex sync.RWMutex

// RegisterErrorType registers an error type in the registry.
// Registered error types are referenced in JSL configurations and by the IsErrorOfType function.
//
// name: A unique identifier for the error type (string referenced in JSL).
// prototype: An instance of the error to be registered (e.g., errors.New("...") or &MyError{}).
//
// It panics if name is empty or prototype is nil.
func RegisterErrorType(name string, prototype error) {
	registryMutex.Lock()
	defer registryMutex.Unlock()

	if name == "" {
		panic("Error type name cannot be empty")
	}
	if prototype == nil {
		panic(fmt.Sprintf("Cannot register nil prototype for name: %s", name))
	}

	errorRegistry[name] = prototype
}

// IsErrorTypeRegistered reports whether the specified error type name is registered in the registry.
func IsErrorTypeRegistered(name string) bool {
	registryMutex.RLock()
	defer registryMutex.RUnlock()
	_, ok := errorRegistry[name]
	return ok
}

// BatchError is a custom error type for batch processing.
// It includes the module where the error occurred, a message, the wrapped original error,
// and flags indicating whether it is retryable or skippable.
type BatchError struct {
	// Module indicates the module where the error occurred (e.g., "reader", "processor", "writer", "config").
	Module string
	// Message is a concise description of the error.
	Message string
	// OriginalErr is the wrapped original error.
	OriginalErr error
	// isRetryable indicates whether this error is retryable.
	isRetryable bool
	// isSkippable indicates whether this error is skippable.
	isSkippable bool
	// StackTrace is the stack trace at the time of the error (for debugging).
	StackTrace string
}

// NewBatchError creates a new BatchError instance.
func NewBatchError(module, message string, originalErr error, isSkippable, isRetryable bool) *BatchError {
	buf := make([]byte, 2048)
	n := runtime.Stack(buf, false)
	stackTrace := string(buf[:n])

	return &BatchError{
		Module:      module,
		Message:     message,
		OriginalErr: originalErr,
		isRetryable: isRetryable,
		isSkippable: isSkippable,
		StackTrace:  stackTrace,
	}
}

// NewBatchErrorf creates a new BatchError using a format string.
// The last arguments are optional and extracted in this order:
// [isSkippable bool], [isRetryable bool], [originalErr error].
//
// Example:
// NewBatchErrorf("reader", "Failed to read: %s", "item_id", true, true, io.EOF)
func NewBatchErrorf(module, format string, a ...interface{}) *BatchError {
	var originalErr error
	isRetryable := false
	isSkippable := false
	args := a

	if len(args) > 0 {
		if err, ok := args[len(args)-1].(error); ok {
			originalErr = err
			args = args[:len(args)-1]
		}
	}

	if len(args) > 0 {
		if b, ok := args[len(args)-1].(bool); ok {
			isRetryable = b
			args = args[:len(args)-1]
		}
	}

	if len(args) > 0 {
		if b, ok := args[len(args)-1].(bool); ok {
			isSkippable = b
			args = args[:len(args)-1]
		}
	}

	message := fmt.Sprintf(format, args...)

	buf := make([]byte, 2048)
	n := runtime.Stack(buf, false)
	stackTrace := string(buf[:n])

	return &BatchError{
		Module:      module,
		Message:     message,
		OriginalErr: originalErr,
		isRetryable: isRetryable,
		isSkippable: isSkippable,
		StackTrace:  stackTrace,
	}
}

// OptimisticLockingFailureException is a constant indicating an optimistic locking failure.
const OptimisticLockingFailureException = "OptimisticLockingFailureException"

// NewOptimisticLockingFailureException creates a BatchError indicating an optimistic locking failure.
// This error is treated as fatal (neither retryable nor skippable).
func NewOptimisticLockingFailureException(module, message string, originalErr error) *BatchError {
	var errToWrap error
	if originalErr != nil {
		errToWrap = errors.Join(ErrOptimisticLockingFailure, originalErr)
	} else {
		errToWrap = ErrOptimisticLockingFailure
	}

	return NewBatchError(module, message, errToWrap, false, false)
}

// Error implements the error interface.
func (e *BatchError) Error() string {
	if e.OriginalErr != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Module, e.Message, e.OriginalErr)
	}
	return fmt.Sprintf("[%s] %s", e.Module, e.Message)
}

// Unwrap returns the original error for errors.Unwrap.
func (e *BatchError) Unwrap() error {
	return e.OriginalErr
}

// IsRetryable reports whether this error is retryable.
func (e *BatchError) IsRetryable() bool {
	return e.isRetryable
}

// IsSkippable reports whether this error is skippable.
func (e *BatchError) IsSkippable() bool {
	return e.isSkippable
}

// IsBatchError reports whether the given error is a BatchError.
func IsBatchError(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(*BatchError)
	return ok
}

// IsTemporary reports whether an error is temporary (e.g., network error, temporary DB issue).
// If the error is a BatchError, its IsRetryable flag takes precedence.
func IsTemporary(err error) bool {
	if err == nil {
		return false
	}
	if be, ok := err.(*BatchError); ok {
		return be.IsRetryable()
	}
	errStr := err.Error()
	return strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "EOF")
}

// IsFatal reports whether an error is fatal (cannot be retried or skipped).
// If the error is a BatchError, its flags take precedence.
func IsFatal(err error) bool {
	if err == nil {
		return false
	}
	if be, ok := err.(*BatchError); ok {
		return !be.IsRetryable() && !be.IsSkippable()
	}
	errStr := err.Error()
	return strings.Contains(errStr, "invalid argument") ||
		strings.Contains(errStr, "permission denied") ||
		strings.Contains(errStr, "data corruption")
}

// IsErrorOfType checks if an error matches a specified type name or message substring.
// It checks in order: registered sentinel errors (errors.Is), substring of error message,
// and type name comparison using reflection.
func IsErrorOfType(err error, errorTypeName string) bool {
	if err == nil {
		return false
	}

	registryMutex.RLock()
	targetError, ok := errorRegistry[errorTypeName]
	registryMutex.RUnlock()

	if ok {
		if errors.Is(err, targetError) {
			return true
		}
	}

	currentErr := err
	for currentErr != nil {
		if strings.Contains(currentErr.Error(), errorTypeName) {
			return true
		}

		errType := reflect.TypeOf(currentErr)
		if errType != nil {
			if errType.String() == errorTypeName || (errType.Kind() == reflect.Ptr && errType.Elem().String() == errorTypeName) {
				return true
			}
		}

		currentErr = errors.Unwrap(currentErr)
	}

	return false
}

// ErrOptimisticLockingFailure is a sentinel error indicating an optimistic locking failure.
var ErrOptimisticLockingFailure = errors.New(OptimisticLockingFailureException)

func init() {
	RegisterErrorType(OptimisticLockingFailureException, ErrOptimisticLockingFailure)

	RegisterErrorType("io.EOF", errors.New("io.EOF"))
	RegisterErrorType("net.OpError", errors.New("net.OpError"))
	RegisterErrorType("context.DeadlineExceeded", context.DeadlineExceeded)
	RegisterErrorType("context.Canceled", context.Canceled)

	RegisterErrorType("sql.ErrNoRows", sql.ErrNoRows)
}

// IsOptimisticLockingFailure reports whether an error indicates an optimistic locking failure.
func IsOptimisticLockingFailure(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, ErrOptimisticLockingFailure)
}

// ExtractErrorMessage extracts the error message string from an error.
// For BatchError, it returns the cleaner Message field; otherwise, it returns the standard Error() string.
func ExtractErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	if be, ok := err.(*BatchError); ok {
		return be.Message
	}
	return err.Error()
}
