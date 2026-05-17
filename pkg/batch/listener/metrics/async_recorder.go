package metrics

import (
	"context"
	"fmt"
	"sync"

	config "github.com/tigerroll/surfin/pkg/batch/core/config"
	"github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	"github.com/tigerroll/surfin/pkg/batch/core/metrics"
	"github.com/tigerroll/surfin/pkg/batch/support/util/logger"

	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/fx"
)

// MetricEvent represents a metric event to be recorded asynchronously.
// It encapsulates all necessary data for various metric types.
type MetricEvent struct {
	Type          string               // The type of metric event (e.g., "job_start", "item_read").
	JobExecution  *model.JobExecution  // The JobExecution associated with the event, if applicable.
	StepExecution *model.StepExecution // The StepExecution associated with the event, if applicable.
	Count         int64                // Numeric count for events like item reads, writes, or chunk commits.
	Err           error                // The error associated with skip or retry events.
	Duration      float64              // The duration value for time-based metrics, typically in seconds.
	Attrs         []attribute.KeyValue // OpenTelemetry attributes to associate with the metric.
	Name          string               // The name of the metric, particularly for RecordDuration.
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
	MetricEventTypeExecutionError = "execution_error"
)

// AsyncMetricRecorder asynchronously records metrics by pushing events to a channel
// and processing them in a dedicated worker goroutine.
type AsyncMetricRecorder struct {
	eventQueue   chan MetricEvent       // Buffered channel for incoming metric events.
	stopCh       chan struct{}          // Channel to signal the worker goroutine to stop.
	wg           sync.WaitGroup         // Used to wait for the worker goroutine to finish.
	syncRecorder metrics.MetricRecorder // The concrete instance that performs actual metric recording

	// Used for aggregating high-frequency events.
	mu       sync.Mutex
	counters map[string]int64
}

// NewAsyncMetricRecorder creates a new asynchronous metric recorder.
//
// Parameters:
//
//	bufferSize: The buffer size for the event queue. If 0 or less, a default value is used.
//	syncRec: The synchronous recorder that performs the actual metric recording.
//
// Returns:
//
//	A new instance of AsyncMetricRecorder.
func NewAsyncMetricRecorder(bufferSize int, syncRec metrics.MetricRecorder) *AsyncMetricRecorder {
	if bufferSize <= 0 { // Ensure a positive buffer size.
		bufferSize = 100 // Default buffer size
	}
	r := &AsyncMetricRecorder{
		eventQueue:   make(chan MetricEvent, bufferSize),
		stopCh:       make(chan struct{}),
		syncRecorder: syncRec,
		counters:     make(map[string]int64),
	}
	r.wg.Add(1)
	go r.run() // Start the worker goroutine
	logger.Debugf("AsyncMetricRecorder: Worker goroutine started (buffer size: %d).", bufferSize)
	return r
}

// run is the worker goroutine that reads events from the event queue and processes them with the synchronous recorder.
// It continuously processes events until a stop signal is received, then processes any remaining events in the queue before exiting.
func (r *AsyncMetricRecorder) run() {
	defer r.wg.Done()
	for {
		select { // Wait for an event or a stop signal.
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
// It dispatches the event to the underlying synchronous metric recorder based on its type.
func (r *AsyncMetricRecorder) processEvent(event MetricEvent) {
	// A new background context is used here as the original context might not be available or relevant for async processing.
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
		r.syncRecorder.RecordItemRead(ctx, event.StepExecution, event.Count)
	case MetricEventTypeItemProcess:
		r.syncRecorder.RecordItemProcess(ctx, event.StepExecution, event.Count)
	case MetricEventTypeItemWrite:
		r.syncRecorder.RecordItemWrite(ctx, event.StepExecution, event.Count)
	case MetricEventTypeItemSkip:
		r.syncRecorder.RecordItemSkip(ctx, event.StepExecution, event.Err)
	case MetricEventTypeItemRetry:
		r.syncRecorder.RecordItemRetry(ctx, event.StepExecution, event.Err)
	case MetricEventTypeChunkCommit:
		r.syncRecorder.RecordChunkCommit(ctx, event.StepExecution, event.Count)
	case MetricEventTypeRecordDuration:
		r.syncRecorder.RecordDuration(ctx, event.Name, event.Duration, event.Attrs...)
	case MetricEventTypeExecutionError:
		r.syncRecorder.RecordExecutionError(ctx, event.Err)
	default:
		logger.Warnf("AsyncMetricRecorder: Unknown metric event type received: %s", event.Type)
	}
}

// Close gracefully stops the asynchronous recorder.
// It sends a stop signal to the worker goroutine and waits for all pending events to be processed.
func (r *AsyncMetricRecorder) Close() {
	logger.Debugf("AsyncMetricRecorder: Sending shutdown signal...")
	close(r.stopCh) // Send stop signal
	r.wg.Wait()     // Wait for the worker goroutine to finish
	logger.Debugf("AsyncMetricRecorder: Shutdown complete.")
}

// sendEvent attempts to send a MetricEvent to the event queue.
// If the queue is full, the event is discarded and a warning is logged to prevent blocking.
func (r *AsyncMetricRecorder) sendEvent(event MetricEvent, id string) {
	select {
	case r.eventQueue <- event:
		// Event added to queue
	default:
		logger.Warnf("AsyncMetricRecorder: Event queue is full (type: %s, ID: %s). Event discarded.", event.Type, id)

		// Record the queue overflow as an error synchronously.
		// Use syncRecorder directly to avoid circular dependencies.
		r.syncRecorder.RecordExecutionError(context.Background(), fmt.Errorf("metric_queue_full: type=%s, id=%s", event.Type, id))
	}
}

// incrementCounter increments the counter for a specific event type.
func (r *AsyncMetricRecorder) incrementCounter(eventType string, count int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.counters[eventType] += count
}

// flushCounters sends all accumulated counters to the queue and resets them.
func (r *AsyncMetricRecorder) flushCounters(stepExecution *model.StepExecution) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for eventType, count := range r.counters {
		if count > 0 {
			r.sendEvent(MetricEvent{Type: eventType, StepExecution: stepExecution, Count: count}, stepExecution.ID)
			r.counters[eventType] = 0 // Reset counter
		}
	}
}

// RecordJobStart records the start of a JobExecution asynchronously.
func (r *AsyncMetricRecorder) RecordJobStart(_ context.Context, execution *model.JobExecution) {
	r.sendEvent(MetricEvent{Type: MetricEventTypeJobStart, JobExecution: execution}, execution.ID)
}

// RecordJobEnd records the end of a JobExecution asynchronously.
func (r *AsyncMetricRecorder) RecordJobEnd(_ context.Context, execution *model.JobExecution) {
	r.sendEvent(MetricEvent{Type: MetricEventTypeJobEnd, JobExecution: execution}, execution.ID)
}

// RecordStepStart records the start of a StepExecution asynchronously.
func (r *AsyncMetricRecorder) RecordStepStart(_ context.Context, execution *model.StepExecution) {
	r.sendEvent(MetricEvent{Type: MetricEventTypeStepStart, StepExecution: execution}, execution.ID)
}

// RecordStepEnd records the end of a StepExecution asynchronously.
func (r *AsyncMetricRecorder) RecordStepEnd(_ context.Context, execution *model.StepExecution) {
	// Flush accumulated counters before ending the step
	r.flushCounters(execution)
	r.sendEvent(MetricEvent{Type: MetricEventTypeStepEnd, StepExecution: execution}, execution.ID)
}

// RecordItemRead records the successful reading of an item asynchronously.
func (r *AsyncMetricRecorder) RecordItemRead(_ context.Context, stepExecution *model.StepExecution, count int64) {
	r.incrementCounter(MetricEventTypeItemRead, count)
}

// RecordItemProcess records the successful processing of an item asynchronously.
func (r *AsyncMetricRecorder) RecordItemProcess(_ context.Context, stepExecution *model.StepExecution, count int64) {
	r.incrementCounter(MetricEventTypeItemProcess, count)
}

// RecordItemWrite records the successful writing of items asynchronously.
func (r *AsyncMetricRecorder) RecordItemWrite(_ context.Context, stepExecution *model.StepExecution, count int64) {
	r.incrementCounter(MetricEventTypeItemWrite, count)
}

// RecordItemSkip records the skipping of an item asynchronously.
func (r *AsyncMetricRecorder) RecordItemSkip(_ context.Context, stepExecution *model.StepExecution, err error) {
	r.sendEvent(MetricEvent{Type: MetricEventTypeItemSkip, StepExecution: stepExecution, Err: err}, stepExecution.ID)
}

// RecordItemRetry records the retry of an item asynchronously.
func (r *AsyncMetricRecorder) RecordItemRetry(_ context.Context, stepExecution *model.StepExecution, err error) {
	r.sendEvent(MetricEvent{Type: MetricEventTypeItemRetry, StepExecution: stepExecution, Err: err}, stepExecution.ID)
}

// RecordChunkCommit records the commitment of a chunk asynchronously.
func (r *AsyncMetricRecorder) RecordChunkCommit(_ context.Context, stepExecution *model.StepExecution, count int64) {
	r.incrementCounter(MetricEventTypeChunkCommit, count)
}

// RecordDuration records the execution time event of a specific operation.
func (r *AsyncMetricRecorder) RecordDuration(_ context.Context, name string, duration float64, attrs ...attribute.KeyValue) {
	r.sendEvent(MetricEvent{Type: MetricEventTypeRecordDuration, Name: name, Duration: duration, Attrs: attrs}, name)
}

// RecordExecutionError records an execution error asynchronously.
func (r *AsyncMetricRecorder) RecordExecutionError(_ context.Context, err error) {
	r.sendEvent(MetricEvent{Type: MetricEventTypeExecutionError, Err: err}, "system")
}

// Ensures AsyncMetricRecorder implements the metrics.MetricRecorder interface at compile time.
var _ metrics.MetricRecorder = (*AsyncMetricRecorder)(nil)

// NewAsyncMetricRecorderWrapper is an Fx decorator function that wraps a synchronous MetricRecorder
// with an asynchronous one.
//
// Parameters:
//
//	lc: The Fx lifecycle object for registering shutdown hooks.
//	appConfig: The application's core configuration, used to determine buffer size.
//	obsConfig: The observability configuration, used to determine if the async wrapper should be applied.
//	syncRecorder: The underlying synchronous MetricRecorder implementation.
//
// Returns:
//
//	A MetricRecorder instance, which is either the wrapped asynchronous recorder or the original synchronous one.
func NewAsyncMetricRecorderWrapper(
	lc fx.Lifecycle,
	appConfig *config.Config,
	syncRecorder metrics.MetricRecorder,
) metrics.MetricRecorder {
	// If metricsAsyncBufferSize is not set or is 0 or less, use the default of 100.
	bufferSize := appConfig.Surfin.Batch.MetricsAsyncBufferSize
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
	logger.Debugf("MetricRecorder decorated with asynchronous wrapper (buffer size: %d).", bufferSize)
	return asyncRecorder
}
