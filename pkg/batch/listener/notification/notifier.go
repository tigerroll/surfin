package notification

import (
	"context"
	"fmt"
	"time"

	coreport "github.com/tigerroll/surfin/pkg/batch/core/application/port" // JobExecutionListener, NotificationListener
	model "github.com/tigerroll/surfin/pkg/batch/core/domain/model"        // JobExecution, BatchStatus
	"github.com/tigerroll/surfin/pkg/batch/core/ports"
	"github.com/tigerroll/surfin/pkg/batch/support/util/logger"
)

// DummyNotifier is a dummy implementation that only logs notifications.
type DummyNotifier struct{}

// NewDummyNotifier creates a new instance of DummyNotifier.
func NewDummyNotifier() ports.Notifier {
	logger.Infof("Notification: Initializing Dummy Notifier.")
	return &DummyNotifier{}
}

// NotifyJobCompletion notifies of job completion.
func (n *DummyNotifier) NotifyJobCompletion(ctx context.Context, execution *model.JobExecution) {
	duration := time.Duration(0)
	if execution.EndTime != nil {
		duration = execution.EndTime.Sub(execution.StartTime)
	}

	message := fmt.Sprintf(
		"Job Notification: Job '%s' (ID: %s) finished with Status: %s, ExitStatus: %s. Duration: %s, Failures: %d",
		execution.JobName,
		execution.ID,
		execution.Status,
		execution.ExitStatus,
		duration,
		len(execution.Failures),
	)

	if execution.Status == model.BatchStatusCompleted {
		logger.Infof(message)
	} else {
		logger.Warnf(message)
	}
}

var _ ports.Notifier = (*DummyNotifier)(nil)

// NotificationListenerImpl is an implementation of coreport.NotificationListener that sends notifications using a Notifier.
type NotificationListenerImpl struct {
	notifier ports.Notifier
}

// NewNotificationListenerImpl creates a new instance of NotificationListenerImpl.
func NewNotificationListenerImpl(notifier ports.Notifier) coreport.NotificationListener {
	return &NotificationListenerImpl{notifier: notifier}
}

// OnJobCompletion sends a notification when a job completes.
func (l *NotificationListenerImpl) OnJobCompletion(ctx context.Context, jobExecution *model.JobExecution) {
	l.notifier.NotifyJobCompletion(ctx, jobExecution)
}

var _ coreport.NotificationListener = (*NotificationListenerImpl)(nil)

// NotificationListenerAdapter adapts core.NotificationListener to core.JobExecutionListener.
type NotificationListenerAdapter struct {
	coreport.NotificationListener
}

// BeforeJob exists to satisfy JobExecutionListener requirements but does nothing.
func (a *NotificationListenerAdapter) BeforeJob(ctx context.Context, jobExecution *model.JobExecution) {
}

// AfterJob calls the NotificationListener's OnJobCompletion.
func (a *NotificationListenerAdapter) AfterJob(ctx context.Context, jobExecution *model.JobExecution) {
	a.NotificationListener.OnJobCompletion(ctx, jobExecution)
}

var _ coreport.JobExecutionListener = (*NotificationListenerAdapter)(nil)
