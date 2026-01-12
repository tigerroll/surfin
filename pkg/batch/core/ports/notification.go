package ports

import (
	"context"
	model "github.com/tigerroll/surfin/pkg/batch/core/domain/model"
)

// Notifier is an abstract interface for notifying external systems about job execution results.
type Notifier interface {
	// NotifyJobCompletion notifies about job completion (success/failure/stop).
	NotifyJobCompletion(ctx context.Context, execution *model.JobExecution)
}
