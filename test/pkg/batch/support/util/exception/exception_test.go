package exception_test

import (
	"errors"
	"fmt"
	"testing"

	"surfin/pkg/batch/support/util/exception"

	"github.com/stretchr/testify/assert"
)

// Custom error type for testing reflection and type matching
type CustomError struct {
	Msg string
}

func (e *CustomError) Error() string {
	return fmt.Sprintf("CustomError: %s", e.Msg)
}

func TestNewBatchError(t *testing.T) {
	originalErr := errors.New("db connection refused")
	// NewBatchError signature is (module, message, originalErr, isSkippable, isRetryable)
	be := exception.NewBatchError("db", "failed to connect", originalErr, false, true) // S=false, R=true

	assert.Equal(t, "db", be.Module)
	assert.Equal(t, "failed to connect", be.Message)
	assert.Equal(t, originalErr, be.Unwrap())
	assert.True(t, be.IsRetryable())
	assert.False(t, be.IsSkippable())
	assert.Contains(t, be.Error(), "[db] failed to connect: db connection refused")
	assert.NotEmpty(t, be.StackTrace)
}

func TestNewBatchErrorf(t *testing.T) {
	// Case 1: Only message args
	be1 := exception.NewBatchErrorf("reader", "item %d not found", 10)
	assert.False(t, be1.IsRetryable())
	assert.False(t, be1.IsSkippable())
	assert.Nil(t, be1.Unwrap())
	assert.Contains(t, be1.Error(), "[reader] item 10 not found")

	// Case 2: Message args + isRetryable (Single bool argument is interpreted as isRetryable=true)
	be2 := exception.NewBatchErrorf("net", "timeout occurred", true)
	assert.True(t, be2.IsRetryable())
	assert.False(t, be2.IsSkippable())
	assert.Nil(t, be2.Unwrap())

	// Case 3: Message args + isSkippable + isRetryable
	// Input order: (..., isSkippable, isRetryable)
	be3 := exception.NewBatchErrorf("item", "data error in item %d", 5, true, false) // S=true, R=false
	assert.False(t, be3.IsRetryable())
	assert.True(t, be3.IsSkippable())
	assert.Nil(t, be3.Unwrap())

	// Case 4: Message args + originalErr
	originalErr4 := errors.New("io error")
	be4 := exception.NewBatchErrorf("io", "read failed", originalErr4)
	assert.False(t, be4.IsRetryable())
	assert.False(t, be4.IsSkippable())
	assert.Equal(t, originalErr4, be4.Unwrap())

	// Case 5: Message args + isRetryable + originalErr (Single bool argument is interpreted as isRetryable=true)
	originalErr5 := errors.New("transient error")
	be5 := exception.NewBatchErrorf("db", "lock contention", true, originalErr5)
	assert.True(t, be5.IsRetryable())
	assert.False(t, be5.IsSkippable())
	assert.Equal(t, originalErr5, be5.Unwrap())

	// Case 6: Message args + isSkippable + isRetryable + originalErr (Full set)
	originalErr6 := errors.New("data format error")
	// Input order: (..., isSkippable, isRetryable, originalErr)
	be6 := exception.NewBatchErrorf("proc", "format error", true, true, originalErr6) // S=true, R=true
	assert.True(t, be6.IsRetryable())
	assert.True(t, be6.IsSkippable())
	assert.Equal(t, originalErr6, be6.Unwrap())
}

func TestNewOptimisticLockingFailureException(t *testing.T) {
	be := exception.NewOptimisticLockingFailureException("repo", "version mismatch", nil)

	assert.False(t, be.IsRetryable())
	assert.False(t, be.IsSkippable())
	assert.True(t, errors.Is(be, exception.ErrOptimisticLockingFailure))
	assert.Contains(t, be.Error(), "version mismatch")
}

func TestIsTemporaryAndIsFatal(t *testing.T) {
	// Temporary (Retryable: R=true, S=false)
	// NewBatchError signature is (..., isSkippable, isRetryable)
	retryableErr := exception.NewBatchError("net", "timeout", errors.New("timeout"), false, true) // S=false, R=true
	assert.True(t, exception.IsTemporary(retryableErr))
	assert.False(t, exception.IsFatal(retryableErr)) // R=true, S=false -> Fatal=false

	// Fatal (Not Skippable, Not Retryable: R=false, S=false)
	fatalErr := exception.NewBatchError("data", "invalid format", errors.New("invalid argument"), false, false) // S=false, R=false
	assert.False(t, exception.IsTemporary(fatalErr))
	assert.True(t, exception.IsFatal(fatalErr)) // R=false, S=false -> Fatal=true

	// Skippable (Skippable, Not Retryable: R=false, S=true)
	skippableErr := exception.NewBatchError("item", "bad record", errors.New("bad record"), true, false) // S=true, R=false
	assert.False(t, exception.IsTemporary(skippableErr))
	assert.False(t, exception.IsFatal(skippableErr)) // R=false, S=true -> Fatal=false

	// Both (Skippable and Retryable: R=true, S=true)
	bothErr := exception.NewBatchError("item", "transient bad record", errors.New("transient bad record"), true, true) // S=true, R=true
	assert.True(t, exception.IsTemporary(bothErr))
	assert.False(t, exception.IsFatal(bothErr)) // R=true, S=true -> Fatal=false

	// General error matching keywords
	timeoutErr := errors.New("connection timeout")
	assert.True(t, exception.IsTemporary(timeoutErr))
	assert.False(t, exception.IsFatal(timeoutErr))

	permErr := errors.New("permission denied")
	assert.False(t, exception.IsTemporary(permErr))
	assert.True(t, exception.IsFatal(permErr))
}

func TestIsErrorOfType(t *testing.T) {
	// Register custom error for testing
	exception.RegisterErrorType("CustomErrorType", &CustomError{})
	
	// 1. Sentinel Error Match (OptimisticLockingFailureException)
	olfe := exception.NewOptimisticLockingFailureException("repo", "update failed", nil)
	assert.True(t, exception.IsErrorOfType(olfe, exception.OptimisticLockingFailureException))

	// 2. Wrapped Error Match (Type Name)
	customErr := &CustomError{Msg: "test"}
	// NewBatchError(..., isSkippable=false, isRetryable=false)
	wrappedErr := exception.NewBatchError("proc", "custom failure", customErr, false, false)
	assert.True(t, exception.IsErrorOfType(wrappedErr, "*exception_test.CustomError")) // Pointer type match

	// 3. Wrapped Error Match (Message Substring)
	assert.True(t, exception.IsErrorOfType(wrappedErr, "custom failure"))
	assert.True(t, exception.IsErrorOfType(wrappedErr, "CustomError: test"))

	// 4. Deeply Wrapped Error Match
	deeplyWrapped := fmt.Errorf("level 2: %w", wrappedErr)
	assert.True(t, exception.IsErrorOfType(deeplyWrapped, "*exception_test.CustomError"))
	assert.False(t, exception.IsErrorOfType(deeplyWrapped, exception.OptimisticLockingFailureException)) // Should be false, testing negative
	assert.False(t, exception.IsErrorOfType(deeplyWrapped, "NonExistentError"))

	// 5. Nil check
	assert.False(t, exception.IsErrorOfType(nil, "any"))
}
