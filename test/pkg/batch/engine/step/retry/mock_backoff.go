package retry

import (
	"context"
	"time"

	"github.com/stretchr/testify/mock"
)

// MockBackoffWaiter is a mock implementation of BackoffWaiter for testing.
type MockBackoffWaiter struct {
	mock.Mock
}

// Wait mocks the waiting process.
func (m *MockBackoffWaiter) Wait(ctx context.Context, duration time.Duration) error {
	args := m.Called(ctx, duration)
	return args.Error(0)
}
