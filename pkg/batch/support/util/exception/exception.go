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

// errorRegistry is a registry that maps error names referenced in JSL to concrete Go error instances.
// It holds error instances (singletons) for comparison using errors.Is.
var errorRegistry = make(map[string]error)

// registryMutex protects access to errorRegistry.
var registryMutex sync.RWMutex

// RegisterErrorType registers an error type in the registry.
// Registered error types are referenced in JSL configurations and by the IsErrorOfType function,
// and are used for error classification and application of processing policies.
//
// name: A unique identifier for the error type (string referenced in JSL).
// prototype: An instance of the error to be registered. Used for comparison with errors.Is.
//
//	For example, `errors.New("...")` or an instance of a custom error type (`&MyError{}`).
//
// If prototype is nil or name is empty, this function will panic.
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

// IsErrorTypeRegistered checks if the specified error type name is registered in the registry.
// name: The error type name to check.
// Returns: true if registered, false otherwise.
func IsErrorTypeRegistered(name string) bool {
	registryMutex.RLock()
	defer registryMutex.RUnlock()
	_, ok := errorRegistry[name]
	return ok
}

// BatchError is a custom error type that occurs during batch processing.
// It holds the module where the error occurred, a message, the wrapped original error,
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
// module: The module where the error occurred.
// message: The error message.
// originalErr: The original error to wrap.
// isSkippable: Whether this error is skippable.
// isRetryable: Whether this error is retryable.
// Returns: A new BatchError instance.
func NewBatchError(module, message string, originalErr error, isSkippable, isRetryable bool) *BatchError {
	// Capture stack trace (for debugging purposes)
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

// NewBatchErrorf creates a new BatchError instance using a format string.
// Optional flags and an error are extracted from the end of the variadic arguments 'a'
// in the order: [isSkippable bool], [isRetryable bool], [originalErr error].
// The remaining arguments are used for fmt.Sprintf.
//
// Order of optional arguments (from the end):
// 1. [originalErr error]
// 2. [isRetryable bool]
// 3. [isSkippable bool]
//
// Examples:
// NewBatchErrorf("reader", "Failed to read item: %s", "item_id_123", true, true, io.EOF)
// -> message: "Failed to read item: item_id_123", isSkippable: true, isRetryable: true, originalErr: io.EOF
//
// NewBatchErrorf("writer", "DB error", false, sql.ErrNoRows)
// -> message: "DB error", isSkippable: false, isRetryable: false, originalErr: sql.ErrNoRows
func NewBatchErrorf(module, format string, a ...interface{}) *BatchError {
	var originalErr error
	isRetryable := false
	isSkippable := false
	args := a

	// Check arguments from the end and extract error, isRetryable, isSkippable in order
	// 1. originalErr (last)
	if len(args) > 0 {
		if err, ok := args[len(args)-1].(error); ok {
			originalErr = err
			args = args[:len(args)-1]
		}
	}

	// 2. isRetryable (second to last)
	if len(args) > 0 {
		if b, ok := args[len(args)-1].(bool); ok {
			isRetryable = b
			args = args[:len(args)-1]
		}
	}

	// 3. isSkippable (third to last)
	if len(args) > 0 {
		if b, ok := args[len(args)-1].(bool); ok {
			isSkippable = b
			args = args[:len(args)-1]
		}
	}

	// Format the message with the remaining arguments
	message := fmt.Sprintf(format, args...)

	// Capture stack trace (for debugging purposes)
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
// This error is typically neither retryable nor skippable.
// module: The module where the error occurred.
// message: The error message.
// originalErr: The original error to wrap.
// Returns: A new BatchError instance.
func NewOptimisticLockingFailureException(module, message string, originalErr error) *BatchError {
	// If originalErr exists, join it with the sentinel error; otherwise, wrap the sentinel error directly.
	var errToWrap error
	if originalErr != nil {
		errToWrap = errors.Join(ErrOptimisticLockingFailure, originalErr)
	} else {
		errToWrap = ErrOptimisticLockingFailure
	}

	// Optimistic locking failures are treated as fatal errors that cannot be retried or skipped.
	return NewBatchError(module, message, errToWrap, false, false)
}

// Error implements the error interface.
// It returns the error's module, message, and the string representation of the original error.
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

// IsRetryable returns whether this error is retryable.
func (e *BatchError) IsRetryable() bool {
	return e.isRetryable
}

// IsSkippable returns whether this error is skippable.
func (e *BatchError) IsSkippable() bool {
	return e.isSkippable
}

// IsBatchError determines if the given error is of type BatchError.
// err: The error to check.
// Returns: true if it is a BatchError, false otherwise.
func IsBatchError(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(*BatchError)
	return ok
}

// IsTemporary determines if an error is temporary (e.g., network error, temporary DB connection issue).
// This function is used by retry logic.
// If it's a BatchError, its IsRetryable flag takes precedence.
// err: The error to check.
// Returns: true if it's a temporary error, false otherwise.
func IsTemporary(err error) bool {
	if err == nil {
		return false
	}
	// Prioritize the IsRetryable flag of BatchError.
	if be, ok := err.(*BatchError); ok {
		return be.IsRetryable()
	}
	// Add other common temporary error detection logic here (e.g., "timeout", "connection refused", "EOF").
	errStr := err.Error()
	return strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "EOF") // io.EOF usually indicates end of data, but can also indicate temporary network disconnection.
}

// IsFatal determines if an error is fatal (cannot be retried or skipped).
// This function is used by skip logic.
// If it's a BatchError, its flags take precedence.
// err: The error to check.
// Returns: true if it's a fatal error, false otherwise.
func IsFatal(err error) bool {
	if err == nil {
		return false
	}
	// Prioritize the flags of BatchError.
	if be, ok := err.(*BatchError); ok {
		// It's fatal if it's neither retryable nor skippable.
		return !be.IsRetryable() && !be.IsSkippable()
	}
	// Add other common fatal error detection logic here (e.g., "invalid argument", "permission denied", "data corruption").
	errStr := err.Error()
	return strings.Contains(errStr, "invalid argument") ||
		strings.Contains(errStr, "permission denied") ||
		strings.Contains(errStr, "data corruption")
}

// IsErrorOfType checks if an error matches a specified type name (string).
// errorTypeName can be a Go error type name (e.g., "*net.OpError", "io.EOF") or a substring of an error message (e.g., "connection refused").
// It checks in order: registered sentinel errors (errors.Is), substring of error message, and type name comparison using reflection.
// err: The error to check.
// errorTypeName: The error type name or substring to compare against.
// Returns: true if it matches, false otherwise.
func IsErrorOfType(err error, errorTypeName string) bool {
	if err == nil {
		return false
	}

	// 1. Comparison with registered sentinel errors using errors.Is
	registryMutex.RLock()
	targetError, ok := errorRegistry[errorTypeName]
	registryMutex.RUnlock()

	if ok {
		// Use errors.Is for registered error instances
		if errors.Is(err, targetError) {
			return true
		}
	}

	// 2. Traverse the error chain and compare by substring of error message or type name
	currentErr := err
	for currentErr != nil {
		// 2-1. Comparison by substring of error message
		if strings.Contains(currentErr.Error(), errorTypeName) {
			return true
		}

		// 2-2. Comparison by type name (using reflection)
		errType := reflect.TypeOf(currentErr)
		if errType != nil {
			// Check both pointer and non-pointer types
			if errType.String() == errorTypeName || (errType.Kind() == reflect.Ptr && errType.Elem().String() == errorTypeName) {
				return true
			}
		}

		// Move to the next wrapped error
		currentErr = errors.Unwrap(currentErr)
	}

	return false
}

// ErrOptimisticLockingFailure is a sentinel error indicating an optimistic locking failure.
var ErrOptimisticLockingFailure = errors.New(OptimisticLockingFailureException)

func init() {
	// Register sentinel errors so that errors.Is can detect them by constant name.
	RegisterErrorType(OptimisticLockingFailureException, ErrOptimisticLockingFailure)

	// --- Registration of common error types that may be referenced in JSL ---
	// Common network-related error names
	RegisterErrorType("io.EOF", errors.New("io.EOF"))
	RegisterErrorType("net.OpError", errors.New("net.OpError"))
	RegisterErrorType("context.DeadlineExceeded", context.DeadlineExceeded)
	RegisterErrorType("context.Canceled", context.Canceled)

	// Common database-related error names
	RegisterErrorType("sql.ErrNoRows", sql.ErrNoRows)
}

// IsOptimisticLockingFailure determines if an error indicates an optimistic locking failure.
// err: The error to check.
// Returns: true if it's an optimistic locking failure, false otherwise.
func IsOptimisticLockingFailure(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, ErrOptimisticLockingFailure)
}

// ExtractErrorMessage extracts the error message string from an error.
// For BatchError, it returns the cleaner Message field.
// Otherwise, it returns the standard Error() string.
// err: The error from which to extract the message.
// Returns: The error message string.
func ExtractErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	// For BatchError, return the usually cleaner Message field.
	if be, ok := err.(*BatchError); ok {
		return be.Message
	}
	// Otherwise, return the standard Error() string.
	return err.Error()
}
