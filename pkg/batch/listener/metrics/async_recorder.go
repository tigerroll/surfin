package metrics

import (
	"context"
	"sync"
	"time"

	"surfin/pkg/batch/core/domain/model"
	config "surfin/pkg/batch/core/config"
	"surfin/pkg/batch/core/metrics"
	"surfin/pkg/batch/support/util/logger"

	"go.uber.org/fx"
)

// MetricEvent represents a metric event to be recorded asynchronously.
type MetricEvent struct {
	Type          string
	JobExecution  *model.JobExecution
	StepExecution *model.StepExecution // Used for step execution events
	// Other event data can be added here as needed
	StepName      string            // For item-level metrics
	Count         int               // For ItemWrite and ChunkCommit counts
	Reason        string            // For ItemSkip and ItemRetry reasons
	Duration      time.Duration     // For duration metrics
	Tags          map[string]string // For duration metric tags
}

// Metric event type constants
const (
	MetricEventTypeJobStart       = "job_start"
	MetricEventTypeJobEnd         = "job_end"
	MetricEventTypeStepStart      = "step_start"
	MetricEventTypeStepEnd        = "step_end"
	MetricEventTypeItemRead       = "item_read"
	MetricEventTypeItemProcess    = "item_process"
	MetricEventTypeItemWrite      = "item_write"
	MetricEventTypeItemSkip       = "item_skip"
	MetricEventTypeItemRetry      = "item_retry"
	MetricEventTypeChunkCommit    = "chunk_commit"
	MetricEventTypeRecordDuration = "record_duration"
)

// AsyncMetricRecorder asynchronously records metrics by pushing events to a channel
// and processing them in a separate goroutine.
type AsyncMetricRecorder struct {
	eventQueue   chan MetricEvent
	stopCh       chan struct{}
	wg           sync.WaitGroup
	syncRecorder metrics.MetricRecorder // The concrete instance that performs actual metric recording
}

// NewAsyncMetricRecorder creates a new asynchronous metric recorder.
// bufferSize: The buffer size for the event queue. If 0 or less, a default value is used.
// syncRec: The synchronous recorder that performs the actual metric recording.
func NewAsyncMetricRecorder(bufferSize int, syncRec metrics.MetricRecorder) *AsyncMetricRecorder {
	if bufferSize <= 0 {
		bufferSize = 100 // Default buffer size
	}
	r := &AsyncMetricRecorder{
		eventQueue:   make(chan MetricEvent, bufferSize),
		stopCh:       make(chan struct{}),
		syncRecorder: syncRec,
	}
	r.wg.Add(1)
	go r.run() // Start the worker goroutine
	logger.Debugf("AsyncMetricRecorder: Worker goroutine started (buffer size: %d).", bufferSize)
	return r
}

// run is the worker goroutine that reads events from the event queue and processes them with the synchronous recorder.
func (r *AsyncMetricRecorder) run() {
	defer r.wg.Done()
	for {
		select {
		case event := <-r.eventQueue:
			r.processEvent(event)
		case <-r.stopCh:
			// Upon receiving a stop signal, process all remaining events in the queue before exiting.
			remainingEvents := len(r.eventQueue)
			for i := 0; i < remainingEvents; i++ { // Loop for the number of remainingEvents
				event := <-r.eventQueue
				r.processEvent(event)
			}
			logger.Debugf("AsyncMetricRecorder: Worker goroutine stopped. Processed %d remaining events.", remainingEvents)
			return
		}
	}
}

// processEvent processes the received metric event.
func (r *AsyncMetricRecorder) processEvent(event MetricEvent) {
	// A new background context is used here because the event payload does not contain the original context.
	// Consider including the original context in the event structure if needed.
	ctx := context.Background() 
	switch event.Type {
	case MetricEventTypeJobStart:
		r.syncRecorder.RecordJobStart(ctx, event.JobExecution)
	case MetricEventTypeJobEnd:
		r.syncRecorder.RecordJobEnd(ctx, event.JobExecution)
	case MetricEventTypeStepStart:
		r.syncRecorder.RecordStepStart(ctx, event.StepExecution)
	case MetricEventTypeStepEnd:
		r.syncRecorder.RecordStepEnd(ctx, event.StepExecution)
	case MetricEventTypeItemRead:
		r.syncRecorder.RecordItemRead(ctx, event.StepName)
	case MetricEventTypeItemProcess:
		r.syncRecorder.RecordItemProcess(ctx, event.StepName)
	case MetricEventTypeItemWrite:
		r.syncRecorder.RecordItemWrite(ctx, event.StepName, event.Count)
	case MetricEventTypeItemSkip:
		r.syncRecorder.RecordItemSkip(ctx, event.StepName, event.Reason)
	case MetricEventTypeItemRetry:
		r.syncRecorder.RecordItemRetry(ctx, event.StepName, event.Reason)
	case MetricEventTypeChunkCommit:
		r.syncRecorder.RecordChunkCommit(ctx, event.StepName, event.Count)
	case MetricEventTypeRecordDuration:
		r.syncRecorder.RecordDuration(ctx, event.StepName, event.Duration, event.Tags) // Using StepName as a generic name
	default:
		logger.Warnf("AsyncMetricRecorder: Unknown metric event type: %s", event.Type)
	}
}

// Close gracefully stops the recorder and processes all remaining events in the queue.
func (r *AsyncMetricRecorder) Close() {
	logger.Debugf("AsyncMetricRecorder: Sending shutdown signal...")
	close(r.stopCh) // Send stop signal
	r.wg.Wait()     // Wait for the worker goroutine to finish
	logger.Debugf("AsyncMetricRecorder: Shutdown complete.")
}

// sendEvent sends an event to the queue, logging a warning if the queue is full.
func (r *AsyncMetricRecorder) sendEvent(event MetricEvent, id string) {
	select {
	case r.eventQueue <- event:
		// Event added to queue
	default:
		logger.Warnf("AsyncMetricRecorder: Event queue is full (type: %s, ID: %s). Event discarded.", event.Type, id)
	}
}

// RecordJobStart asynchronously records the start event of a JobExecution.
func (r *AsyncMetricRecorder) RecordJobStart(ctx context.Context, execution *model.JobExecution) {
	r.sendEvent(MetricEvent{Type: MetricEventTypeJobStart, JobExecution: execution}, execution.ID)
}

// RecordJobEnd asynchronously records the end event of a JobExecution.
func (r *AsyncMetricRecorder) RecordJobEnd(ctx context.Context, execution *model.JobExecution) {
	r.sendEvent(MetricEvent{Type: MetricEventTypeJobEnd, JobExecution: execution}, execution.ID)
}

// RecordStepStart asynchronously records the start event of a StepExecution.
func (r *AsyncMetricRecorder) RecordStepStart(ctx context.Context, execution *model.StepExecution) {
	r.sendEvent(MetricEvent{Type: MetricEventTypeStepStart, StepExecution: execution}, execution.ID)
}

// RecordStepEnd asynchronously records the end event of a StepExecution.
func (r *AsyncMetricRecorder) RecordStepEnd(ctx context.Context, execution *model.StepExecution) {
	r.sendEvent(MetricEvent{Type: MetricEventTypeStepEnd, StepExecution: execution}, execution.ID)
}

// RecordItemRead asynchronously records the successful item read event.
func (r *AsyncMetricRecorder) RecordItemRead(ctx context.Context, stepName string) {
	r.sendEvent(MetricEvent{Type: MetricEventTypeItemRead, StepName: stepName}, stepName)
}

// RecordItemProcess asynchronously records the successful item process event.
func (r *AsyncMetricRecorder) RecordItemProcess(ctx context.Context, stepName string) {
	r.sendEvent(MetricEvent{Type: MetricEventTypeItemProcess, StepName: stepName}, stepName)
}

// RecordItemWrite asynchronously records the successful item write event.
func (r *AsyncMetricRecorder) RecordItemWrite(ctx context.Context, stepName string, count int) {
	r.sendEvent(MetricEvent{Type: MetricEventTypeItemWrite, StepName: stepName, Count: count}, stepName)
}

// RecordItemSkip asynchronously records the item skip event.
func (r *AsyncMetricRecorder) RecordItemSkip(ctx context.Context, stepName string, reason string) {
	r.sendEvent(MetricEvent{Type: MetricEventTypeItemSkip, StepName: stepName, Reason: reason}, stepName)
}

// RecordItemRetry asynchronously records the item retry event.
func (r *AsyncMetricRecorder) RecordItemRetry(ctx context.Context, stepName string, reason string) {
	r.sendEvent(MetricEvent{Type: MetricEventTypeItemRetry, StepName: stepName, Reason: reason}, stepName)
}

// RecordChunkCommit asynchronously records the chunk commit event.
func (r *AsyncMetricRecorder) RecordChunkCommit(ctx context.Context, stepName string, count int) {
	r.sendEvent(MetricEvent{Type: MetricEventTypeChunkCommit, StepName: stepName, Count: count}, stepName)
}

// RecordDuration asynchronously records the execution time event of a specific operation.
func (r *AsyncMetricRecorder) RecordDuration(ctx context.Context, name string, duration time.Duration, tags map[string]string) {
	r.sendEvent(MetricEvent{Type: MetricEventTypeRecordDuration, StepName: name, Duration: duration, Tags: tags}, name)
}

// Ensures AsyncMetricRecorder implements the metrics.MetricRecorder interface at compile time.
var _ metrics.MetricRecorder = (*AsyncMetricRecorder)(nil)

// NewAsyncMetricRecorderWrapper is a helper function for use with fx.Decorate.
// It takes fx.Lifecycle and config.Config and calls AsyncMetricRecorder.Close() on shutdown.
func NewAsyncMetricRecorderWrapper(lc fx.Lifecycle, cfg *config.Config, syncRecorder metrics.MetricRecorder) metrics.MetricRecorder {
	// If metricsAsyncBufferSize is not set or is 0 or less, use the default of 100.
	bufferSize := cfg.Surfin.Batch.MetricsAsyncBufferSize
	if bufferSize <= 0 {
		bufferSize = 100
	}
	asyncRecorder := NewAsyncMetricRecorder(bufferSize, syncRecorder) 
	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			asyncRecorder.Close()
			return nil
		},
	})
	logger.Debugf("MetricRecorder decorated with asynchronous wrapper.")
	return asyncRecorder
}
