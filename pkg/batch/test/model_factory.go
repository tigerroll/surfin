package test

import (
	"time"

	model "surfin/pkg/batch/core/domain/model"
	"github.com/google/uuid"
)

// NewTestJobParameters creates JobParameters for testing.
func NewTestJobParameters(params map[string]interface{}) model.JobParameters {
	jp := model.NewJobParameters()
	for k, v := range params {
		jp.Put(k, v)
	}
	return jp
}

// NewTestJobInstance creates a JobInstance for testing.
func NewTestJobInstance(jobName string, params model.JobParameters) *model.JobInstance {
	// Use core.NewJobInstance to also calculate the hash.
	return model.NewJobInstance(jobName, params)
}

// NewTestJobExecution creates a JobExecution for testing.
func NewTestJobExecution(jobInstanceID, jobName string, params model.JobParameters) *model.JobExecution {
	return model.NewJobExecution(jobInstanceID, jobName, params)
}

// NewTestStepExecution creates a StepExecution for testing.
func NewTestStepExecution(jobExecution *model.JobExecution, stepName string) *model.StepExecution {
	return model.NewStepExecution(uuid.New().String(), jobExecution, stepName)
}

// MarkExecutionAsCompleted sets the JobExecution to a completed state.
func MarkExecutionAsCompleted(je *model.JobExecution) {
	je.MarkAsCompleted()
}

// MarkExecutionAsFailed sets the JobExecution to a failed state.
func MarkExecutionAsFailed(je *model.JobExecution, err error) {
	je.MarkAsFailed(err)
}

// MarkStepAsCompleted sets the StepExecution to a completed state.
func MarkStepAsCompleted(se *model.StepExecution) {
	se.MarkAsCompleted()
}

// MarkStepAsFailed sets the StepExecution to a failed state.
func MarkStepAsFailed(se *model.StepExecution, err error) {
	se.MarkAsFailed(err)
}

// NewTestExecutionContext creates an ExecutionContext for testing.
func NewTestExecutionContext(data map[string]interface{}) model.ExecutionContext {
	ec := model.NewExecutionContext()
	for k, v := range data {
		ec.Put(k, v)
	}
	return ec
}

// NewTimePtr returns a pointer to time.Time.
func NewTimePtr(t time.Time) *time.Time {
	return &t
}
