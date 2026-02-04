package listener

import (
	"context"
	port "github.com/tigerroll/surfin/pkg/batch/core/application/port"
	model "github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	"github.com/tigerroll/surfin/pkg/batch/support/util/logger"
)

// JobCompletionSignaler is a JobExecutionListener that closes a channel
// when a job completes, signaling its completion to external components.
type JobCompletionSignaler struct {
	// JobDoneChan is the channel that will be closed upon job completion.
	JobDoneChan chan struct{}
}

// NewJobCompletionSignaler creates a new instance of JobCompletionSignaler.
//
// Parameters:
//
//	jobDoneChan: The channel to be closed when the job completes.
//
// Returns:
//
//	A pointer to a new `JobCompletionSignaler` instance.
func NewJobCompletionSignaler(jobDoneChan chan struct{}) *JobCompletionSignaler {
	return &JobCompletionSignaler{
		JobDoneChan: jobDoneChan,
	}
}

// BeforeJob is part of the JobExecutionListener interface but does nothing in this implementation.
//
// Parameters:
//
//	ctx: The context for the operation.
//	jobExecution: The `JobExecution` instance before the job starts.
func (l *JobCompletionSignaler) BeforeJob(ctx context.Context, jobExecution *model.JobExecution) {
	// No-op
}

// AfterJob closes the JobDoneChan when the job completes.
// It ensures the channel is not already closed before attempting to close it.
//
// Parameters:
//
//	ctx: The context for the operation.
//	jobExecution: The `JobExecution` instance after the job completes.
func (l *JobCompletionSignaler) AfterJob(ctx context.Context, jobExecution *model.JobExecution) {
	logger.Infof("JobCompletionSignaler: Job '%s' (ID: %s) completed. Closing JobDoneChan.", jobExecution.JobName, jobExecution.ID)
	// Check if the channel is already closed or receivable before closing.
	select {
	case <-l.JobDoneChan:
		// Channel is already closed or a value has been sent (should not happen for struct{} channel).
		// Do nothing, as it's already signaled.
	default:
		// Channel is not closed, so close it.
		close(l.JobDoneChan)
	}
}

// Verify that JobCompletionSignaler implements the port.JobExecutionListener interface.
var _ port.JobExecutionListener = (*JobCompletionSignaler)(nil)
