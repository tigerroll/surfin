package logging

import (
	"context"

	port "github.com/tigerroll/surfin/pkg/batch/core/application/port"
	model "github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	logger "github.com/tigerroll/surfin/pkg/batch/support/util/logger"
)

// --- Job Execution Listener ---

type LoggingJobListener struct {
	properties map[string]interface{}
}

func NewLoggingJobListener(properties map[string]interface{}) port.JobExecutionListener {
	return &LoggingJobListener{properties: properties}
}

func (l *LoggingJobListener) BeforeJob(ctx context.Context, jobExecution *model.JobExecution) {
	logger.Infof("JobExecutionListener: BeforeJob - JobName: %s, ID: %s, Params: %+v", jobExecution.JobName, jobExecution.ID, jobExecution.Parameters)
}

func (l *LoggingJobListener) AfterJob(ctx context.Context, jobExecution *model.JobExecution) {
	logger.Infof("JobExecutionListener: AfterJob - JobName: %s, Status: %s, ExitStatus: %s", jobExecution.JobName, jobExecution.Status, jobExecution.ExitStatus)
}

var _ port.JobExecutionListener = (*LoggingJobListener)(nil)

// --- Step Execution Listener ---

type LoggingStepListener struct {
	properties map[string]interface{}
}

func NewLoggingStepListener(properties map[string]interface{}) port.StepExecutionListener {
	return &LoggingStepListener{properties: properties}
}

func (l *LoggingStepListener) BeforeStep(ctx context.Context, stepExecution *model.StepExecution) {
	logger.Infof("StepExecutionListener: BeforeStep - StepName: %s, ID: %s", stepExecution.StepName, stepExecution.ID)
}

func (l *LoggingStepListener) AfterStep(ctx context.Context, stepExecution *model.StepExecution) {
	logger.Infof("StepExecutionListener: AfterStep - StepName: %s, Status: %s, ExitStatus: %s", stepExecution.StepName, stepExecution.Status, stepExecution.ExitStatus)
}

var _ port.StepExecutionListener = (*LoggingStepListener)(nil)

// --- Chunk Listener ---

type LoggingChunkListener struct {
	properties map[string]interface{}
}

func NewLoggingChunkListener(properties map[string]interface{}) port.ChunkListener {
	return &LoggingChunkListener{properties: properties}
}

func (l *LoggingChunkListener) BeforeChunk(ctx context.Context, stepExecution *model.StepExecution) {
	logger.Debugf("ChunkListener: BeforeChunk - StepName: %s", stepExecution.StepName)
}

func (l *LoggingChunkListener) AfterChunk(ctx context.Context, stepExecution *model.StepExecution) {
	logger.Debugf("ChunkListener: AfterChunk - StepName: %s, Read: %d, Write: %d", stepExecution.StepName, stepExecution.ReadCount, stepExecution.WriteCount)
}

var _ port.ChunkListener = (*LoggingChunkListener)(nil)

// --- Item Read Listener ---

type LoggingItemReadListener struct {
	properties map[string]interface{}
}

func NewLoggingItemReadListener(properties map[string]interface{}) port.ItemReadListener {
	return &LoggingItemReadListener{properties: properties}
}

func (l *LoggingItemReadListener) OnReadError(ctx context.Context, err error) {
	logger.Errorf("ItemReadListener: OnReadError - %v", err)
}

var _ port.ItemReadListener = (*LoggingItemReadListener)(nil)

// --- Item Process Listener ---

type LoggingItemProcessListener struct {
	properties map[string]interface{}
}

func NewLoggingItemProcessListener(properties map[string]interface{}) port.ItemProcessListener {
	return &LoggingItemProcessListener{properties: properties}
}

func (l *LoggingItemProcessListener) OnProcessError(ctx context.Context, item interface{}, err error) {
	logger.Errorf("ItemProcessListener: OnProcessError - Item: %+v, Error: %v", item, err)
}

func (l *LoggingItemProcessListener) OnSkipInProcess(ctx context.Context, item interface{}, err error) {
	logger.Warnf("ItemProcessListener: OnSkipInProcess - Item: %+v, Error: %v", item, err)
}

var _ port.ItemProcessListener = (*LoggingItemProcessListener)(nil)

// --- Item Write Listener ---

type LoggingItemWriteListener struct {
	properties map[string]interface{}
}

func NewLoggingItemWriteListener(properties map[string]interface{}) port.ItemWriteListener {
	return &LoggingItemWriteListener{properties: properties}
}

func (l *LoggingItemWriteListener) OnWriteError(ctx context.Context, items []interface{}, err error) {
	logger.Errorf("ItemWriteListener: OnWriteError - Items count: %d, Error: %v", len(items), err)
}

func (l *LoggingItemWriteListener) OnSkipInWrite(ctx context.Context, item interface{}, err error) {
	logger.Warnf("ItemWriteListener: OnSkipInWrite - Skipping item: %+v, Error: %v", item, err)
}

var _ port.ItemWriteListener = (*LoggingItemWriteListener)(nil)

// --- Skip Listener ---

type LoggingSkipListener struct {
	properties map[string]interface{}
}

func NewLoggingSkipListener(properties map[string]interface{}) port.SkipListener {
	return &LoggingSkipListener{properties: properties}
}

func (l *LoggingSkipListener) OnSkipRead(ctx context.Context, err error) {
	logger.Warnf("SkipListener: OnSkipRead - Skipping item due to error: %v", err)
}

func (l *LoggingSkipListener) OnSkipProcess(ctx context.Context, item interface{}, err error) {
	logger.Warnf("SkipListener: OnSkipProcess - Skipping item: %+v, Error: %v", item, err)
}

func (l *LoggingSkipListener) OnSkipWrite(ctx context.Context, item interface{}, err error) {
	logger.Warnf("SkipListener: OnSkipWrite - Skipping item: %+v, Error: %v", item, err)
}

var _ port.SkipListener = (*LoggingSkipListener)(nil)

// --- Retry Item Listener ---

type LoggingRetryItemListener struct {
	properties map[string]interface{}
}

func NewLoggingRetryItemListener(properties map[string]interface{}) port.RetryItemListener {
	return &LoggingRetryItemListener{properties: properties}
}

func (l *LoggingRetryItemListener) OnRetryRead(ctx context.Context, err error) {
	logger.Warnf("RetryItemListener: OnRetryRead - Retrying read operation due to error: %v", err)
}

func (l *LoggingRetryItemListener) OnRetryProcess(ctx context.Context, item interface{}, err error) {
	logger.Warnf("RetryItemListener: OnRetryProcess - Retrying process operation for item: %+v, Error: %v", item, err)
}

func (l *LoggingRetryItemListener) OnRetryWrite(ctx context.Context, items []interface{}, err error) {
	logger.Warnf("RetryItemListener: OnRetryWrite - Retrying write operation for %d items, Error: %v", len(items), err)
}

var _ port.RetryItemListener = (*LoggingRetryItemListener)(nil)
