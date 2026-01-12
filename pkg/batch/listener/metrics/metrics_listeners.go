package metrics

import (
	"context"

	port "surfin/pkg/batch/core/application/port"
	model "surfin/pkg/batch/core/domain/model"
	"surfin/pkg/batch/core/metrics"
)

// --- Job Execution Listener ---

type MetricsJobListener struct {
	recorder metrics.MetricRecorder
}

func NewMetricsJobListener(recorder metrics.MetricRecorder) port.JobExecutionListener {
	return &MetricsJobListener{recorder: recorder}
}

func (l *MetricsJobListener) BeforeJob(ctx context.Context, jobExecution *model.JobExecution) {
	l.recorder.RecordJobStart(ctx, jobExecution)
}

func (l *MetricsJobListener) AfterJob(ctx context.Context, jobExecution *model.JobExecution) {
	l.recorder.RecordJobEnd(ctx, jobExecution)
}

var _ port.JobExecutionListener = (*MetricsJobListener)(nil)

// --- Step Execution Listener ---

type MetricsStepListener struct {
	recorder metrics.MetricRecorder
}

func NewMetricsStepListener(recorder metrics.MetricRecorder) port.StepExecutionListener {
	return &MetricsStepListener{recorder: recorder}
}

func (l *MetricsStepListener) BeforeStep(ctx context.Context, stepExecution *model.StepExecution) {
	l.recorder.RecordStepStart(ctx, stepExecution)
}

func (l *MetricsStepListener) AfterStep(ctx context.Context, stepExecution *model.StepExecution) {
	l.recorder.RecordStepEnd(ctx, stepExecution)
}

var _ port.StepExecutionListener = (*MetricsStepListener)(nil)

// --- Chunk Listener ---

type MetricsChunkListener struct {
	recorder metrics.MetricRecorder
}

func NewMetricsChunkListener(recorder metrics.MetricRecorder) port.ChunkListener {
	return &MetricsChunkListener{recorder: recorder}
}

func (l *MetricsChunkListener) BeforeChunk(ctx context.Context, stepExecution *model.StepExecution) {
	// No-op
}

func (l *MetricsChunkListener) AfterChunk(ctx context.Context, stepExecution *model.StepExecution) {
	// Records the number of chunk commits (count is fixed at 1).
	l.recorder.RecordChunkCommit(ctx, stepExecution.StepName, 1)
	
	// Metrics for Read/Write/Filter should ideally be recorded individually in ItemRead/Write/Process Listeners,
	// but here only chunk commits are recorded.
}

var _ port.ChunkListener = (*MetricsChunkListener)(nil)

// --- Item Read Listener ---

type MetricsItemReadListener struct {
	recorder metrics.MetricRecorder
}

func NewMetricsItemReadListener(recorder metrics.MetricRecorder) port.ItemReadListener {
	return &MetricsItemReadListener{recorder: recorder}
}

func (l *MetricsItemReadListener) OnReadError(ctx context.Context, err error) {
	// Metrics are recorded directly within ChunkStep, so no action is taken here.
}

var _ port.ItemReadListener = (*MetricsItemReadListener)(nil)

// --- Item Process Listener ---

type MetricsItemProcessListener struct {
	recorder metrics.MetricRecorder
}

func NewMetricsItemProcessListener(recorder metrics.MetricRecorder) port.ItemProcessListener {
	return &MetricsItemProcessListener{recorder: recorder}
}

func (l *MetricsItemProcessListener) OnProcessError(ctx context.Context, item interface{}, err error) {
	// Metrics are recorded directly within ChunkStep, so no action is taken here.
}

func (l *MetricsItemProcessListener) OnSkipInProcess(ctx context.Context, item interface{}, err error) {
	// Metrics are recorded directly within ChunkStep, so no action is taken here.
}

var _ port.ItemProcessListener = (*MetricsItemProcessListener)(nil)

// --- Item Write Listener ---

type MetricsItemWriteListener struct {
	recorder metrics.MetricRecorder
}

func NewMetricsItemWriteListener(recorder metrics.MetricRecorder) port.ItemWriteListener {
	return &MetricsItemWriteListener{recorder: recorder}
}

func (l *MetricsItemWriteListener) OnWriteError(ctx context.Context, items []interface{}, err error) {
	// Metrics are recorded directly within ChunkStep, so no action is taken here.
}

func (l *MetricsItemWriteListener) OnSkipInWrite(ctx context.Context, item interface{}, err error) {
	// Metrics are recorded directly within ChunkStep, so no action is taken here.
}

var _ port.ItemWriteListener = (*MetricsItemWriteListener)(nil)

// --- Skip Listener ---

type MetricsSkipListener struct {
	recorder metrics.MetricRecorder
}

func NewMetricsSkipListener(recorder metrics.MetricRecorder) port.SkipListener {
	return &MetricsSkipListener{recorder: recorder}
}

func (l *MetricsSkipListener) OnSkipRead(ctx context.Context, err error) {
	// Metrics are recorded directly within ChunkStep, so no action is taken here.
}

func (l *MetricsSkipListener) OnSkipProcess(ctx context.Context, item interface{}, err error) {
	// Metrics are recorded directly within ChunkStep, so no action is taken here.
}

func (l *MetricsSkipListener) OnSkipWrite(ctx context.Context, item interface{}, err error) {
	// Metrics are recorded directly within ChunkStep, so no action is taken here.
}

var _ port.SkipListener = (*MetricsSkipListener)(nil)

// --- Retry Item Listener ---

type MetricsRetryItemListener struct {
	recorder metrics.MetricRecorder
}

func NewMetricsRetryItemListener(recorder metrics.MetricRecorder) port.RetryItemListener {
	return &MetricsRetryItemListener{recorder: recorder}
}

func (l *MetricsRetryItemListener) OnRetryRead(ctx context.Context, err error) {
	// Metrics are recorded directly within ChunkStep, so no action is taken here.
}

func (l *MetricsRetryItemListener) OnRetryProcess(ctx context.Context, item interface{}, err error) {
	// Metrics are recorded directly within ChunkStep, so no action is taken here.
}

func (l *MetricsRetryItemListener) OnRetryWrite(ctx context.Context, items []interface{}, err error) {
	// Metrics are recorded directly within ChunkStep, so no action is taken here.
}

var _ port.RetryItemListener = (*MetricsRetryItemListener)(nil)
