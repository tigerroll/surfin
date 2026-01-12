package skip

import (
	"github.com/tigerroll/surfin/pkg/batch/support/util/exception"
)

// SkipPolicy is an interface that defines the logic for determining whether to skip an error that occurred during item processing.
// This policy manages skipability, skip limits, and the current skip count.
type SkipPolicy interface {
	// ShouldSkip determines if a given error is skippable based on the current policy.
	// err: The error to evaluate.
	// Returns: true if the error is skippable, false otherwise.
	ShouldSkip(err error) bool
	// CanSkip determines if further skips are allowed within the current skip limit.
	// Returns: true if skipping is allowed, false otherwise.
	CanSkip() bool
	// IncrementSkipCount increments the count of skipped items by 1.
	IncrementSkipCount()
	// GetSkipCount returns the total number of items skipped so far.
	// Returns: The current skip count.
	GetSkipCount() int
	// GetSkipLimit returns the maximum number of skips configured for this policy.
	// Returns: The skip limit.
	GetSkipLimit() int
}

// DefaultSkipPolicyFactory is a factory for creating SkipPolicy.
// This factory generates instances of defaultSkipPolicy based on configuration.
type DefaultSkipPolicyFactory struct{}

// NewDefaultSkipPolicyFactory creates a new DefaultSkipPolicyFactory.
// Returns: A pointer to the new DefaultSkipPolicyFactory.
func NewDefaultSkipPolicyFactory() *DefaultSkipPolicyFactory {
	return &DefaultSkipPolicyFactory{}
}

// Create creates a new SkipPolicy instance based on the specified skip limit and list of skippable exceptions.
// skipLimit: The maximum number of skips allowed. 0 means no skips are allowed.
// skippableExceptions: A list of string representations of exception types considered skippable.
// Returns: A new SkipPolicy instance and any error that occurred during creation.
func (f *DefaultSkipPolicyFactory) Create(skipLimit int, skippableExceptions []string) (SkipPolicy, error) {
	return &defaultSkipPolicy{
		skipLimit:           skipLimit,
		skippableExceptions: skippableExceptions,
		currentSkipCount:    0,
	}, nil
}

// defaultSkipPolicy is the default implementation of SkipPolicy.
// This policy operates based on the configured skip limit and list of skippable exceptions.
type defaultSkipPolicy struct {
	skipLimit           int
	skippableExceptions []string
	currentSkipCount    int
}

// ShouldSkip determines if an error is skippable.
// The determination is made with the following priority:
// 1. Whether the skip limit has not been exceeded.
// 2. Whether the error is of type BatchError and its IsSkippable flag is true.
// 3. Whether the error type or message is included in the configured list of skippable exceptions.
// err: The error to evaluate.
// Returns: true if the error is skippable, false otherwise.
func (p *defaultSkipPolicy) ShouldSkip(err error) bool {
	if err == nil {
		return false
	}

	// 1. Check if skip limit is exceeded
	// If skipLimit is 0, skipping is not allowed.
	if p.skipLimit > 0 && p.currentSkipCount >= p.skipLimit {
		return false // Limit exceeded
	}
	// If skipLimit is 0 and currentSkipCount is 0, ShouldSkip returns false.
	// This is because skipLimit=0 means "do not skip".
	if p.skipLimit == 0 {
		return false
	}

	// 2. Check BatchError flag
	if be, ok := err.(*exception.BatchError); ok && be.IsSkippable() {
		return true
	}

	// 3. Match against configured skippable exceptions list
	for _, typeName := range p.skippableExceptions {
		if exception.IsErrorOfType(err, typeName) {
			return true
		}
	}

	return false
}

// CanSkip determines if further skips are allowed within the current skip limit.
// If skipLimit is 0, skipping is not allowed.
// If skipLimit is greater than 0, it is allowed if the current skip count is less than the limit.
// Returns: true if skipping is allowed, false otherwise.
func (p *defaultSkipPolicy) CanSkip() bool {
	// If skipLimit is 0, skipping is not allowed.
	// If skipLimit is greater than 0, it is allowed if the current skip count is less than the limit.
	return p.skipLimit > 0 && p.currentSkipCount < p.skipLimit
}

// IncrementSkipCount increments the count of skipped items by 1.
func (p *defaultSkipPolicy) IncrementSkipCount() {
	p.currentSkipCount++
}

// GetSkipCount returns the total number of items skipped so far.
// Returns: The current skip count.
func (p *defaultSkipPolicy) GetSkipCount() int {
	return p.currentSkipCount
}

// GetSkipLimit returns the maximum number of skips configured for this policy.
// Returns: The skip limit.
func (p *defaultSkipPolicy) GetSkipLimit() int {
	return p.skipLimit
}

// Verify interfaces
var _ SkipPolicy = (*defaultSkipPolicy)(nil)
