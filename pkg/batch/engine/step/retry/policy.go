package retry

import (
	"github.com/tigerroll/surfin/pkg/batch/support/util/exception"
)

// RetryPolicy is an interface that defines retry logic.
// This interface provides methods to determine if a specific error is retryable,
// and to determine the backoff interval between retries.
type RetryPolicy interface {
	// ShouldRetry determines if a given error is retryable.
	// err: The error to evaluate.
	// Returns: true if the error is retryable, false otherwise.
	ShouldRetry(err error) bool
	// GetBackoffInterval returns the backoff interval (in milliseconds) for a given attempt number.
	// attempt: The current attempt number (starting from 1).
	// Returns: The waiting time (in milliseconds) until the next retry.
	GetBackoffInterval(attempt int) int
	// GetMaxAttempts returns the maximum number of retry attempts.
	// Returns: The maximum number of attempts.
	GetMaxAttempts() int
}

// DefaultRetryPolicyFactory is a factory for creating RetryPolicy.
// This factory generates instances of defaultRetryPolicy based on configuration.
type DefaultRetryPolicyFactory struct{}

// NewDefaultRetryPolicyFactory creates a new DefaultRetryPolicyFactory.
// Returns: A pointer to the new DefaultRetryPolicyFactory.
func NewDefaultRetryPolicyFactory() *DefaultRetryPolicyFactory {
	return &DefaultRetryPolicyFactory{}
}

// Create creates a new RetryPolicy instance based on the given settings.
// maxAttempts: The maximum number of retry attempts.
// initialInterval: The waiting time (in milliseconds) until the first retry.
// retryableExceptions: A list of string representations of exception types considered retryable.
// Returns: A new RetryPolicy instance.
func (f *DefaultRetryPolicyFactory) Create(maxAttempts int, initialInterval int, retryableExceptions []string) RetryPolicy {
	return &defaultRetryPolicy{
		maxAttempts:         maxAttempts,
		initialInterval:     initialInterval,
		retryableExceptions: retryableExceptions,
	}
}

// defaultRetryPolicy is the default implementation of RetryPolicy.
// This policy operates based on the configured maximum attempts, initial interval, and list of retryable exceptions.
type defaultRetryPolicy struct {
	maxAttempts         int
	initialInterval     int
	retryableExceptions []string
}

// GetMaxAttempts returns the maximum number of attempts.
// Returns: The maximum number of attempts.
func (p *defaultRetryPolicy) GetMaxAttempts() int {
	return p.maxAttempts
}

// ShouldRetry determines if an error is retryable.
// The determination is based on the IsRetryable flag of BatchError, or by matching against the configured list of retryable exceptions.
// err: The error to evaluate.
// Returns: true if the error is retryable, false otherwise.
func (p *defaultRetryPolicy) ShouldRetry(err error) bool {
	if err == nil {
		return false
	}

	// 1. Check BatchError flag
	if be, ok := err.(*exception.BatchError); ok && be.IsRetryable() {
		return true
	}

	// 2. Match against configured retryable exceptions list
	for _, typeName := range p.retryableExceptions {
		if exception.IsErrorOfType(err, typeName) {
			return true
		}
	}

	return false
}

// GetBackoffInterval returns the backoff interval based on the specified attempt number.
// The current implementation always returns the initial interval (fixed interval).
// attempt: The current attempt number (not used in this implementation).
// Returns: The waiting time (in milliseconds) until the next retry.
func (p *defaultRetryPolicy) GetBackoffInterval(attempt int) int {
	// Implement simple fixed interval or exponential backoff
	return p.initialInterval
}

// Verify interfaces
var _ RetryPolicy = (*defaultRetryPolicy)(nil)
