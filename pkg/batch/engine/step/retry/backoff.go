package retry

import (
	"context"
	"time"
)

// BackoffWaiter abstracts the waiting process between retries.
type BackoffWaiter interface {
	// Wait waits for the specified duration. It returns an error immediately if the context is canceled.
	Wait(ctx context.Context, duration time.Duration) error
}

// RealBackoffWaiter is the default implementation that waits for the actual time to pass.
type RealBackoffWaiter struct{}

// Wait waits for the specified duration using time.NewTimer.
func (w *RealBackoffWaiter) Wait(ctx context.Context, duration time.Duration) error {
	if duration <= 0 {
		return nil
	}

	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
