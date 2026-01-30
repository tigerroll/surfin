// Package job provides the implementation of the "helloWorldJob" batch job.
// This job demonstrates a basic batch process within the Surfin Batch Framework.
package job

import (
	// Standard library imports
	"context"

	port "github.com/tigerroll/surfin/pkg/batch/core/application/port"
	config "github.com/tigerroll/surfin/pkg/batch/core/config"
	model "github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	repository "github.com/tigerroll/surfin/pkg/batch/core/domain/repository"
	metrics "github.com/tigerroll/surfin/pkg/batch/core/metrics"
	logger "github.com/tigerroll/surfin/pkg/batch/support/util/logger"
)

// HelloWorldJob is a simple job that implements the port.Job interface.
// For this tutorial, it does not directly contain JobRunner logic but
// holds the FlowDefinition passed from the JobFactory.
type HelloWorldJob struct {
	id             string
	name           string
	flow           *model.FlowDefinition
	jobRepository  repository.JobRepository
	cfg            *config.Config
	listeners      []port.JobExecutionListener
	metricRecorder metrics.MetricRecorder
	tracer         metrics.Tracer
}

// NewHelloWorldJob creates a new instance of HelloWorldJob.
func NewHelloWorldJob(
	jobRepository repository.JobRepository,
	cfg *config.Config,
	listeners []port.JobExecutionListener,
	flow *model.FlowDefinition,
	metricRecorder metrics.MetricRecorder,
	tracer metrics.Tracer,
) (port.Job, error) {
	return &HelloWorldJob{
		id:             "helloWorldJob", // Matches JSL ID
		name:           "Hello World Batch Job",
		flow:           flow,
		jobRepository:  jobRepository,
		cfg:            cfg,
		listeners:      listeners,
		metricRecorder: metricRecorder,
		tracer:         tracer,
	}, nil
}

// Run contains the job's execution logic.
// The SimpleJobLauncher calls this Run method using a JobRunner.
// Therefore, this method does not directly execute the flow but delegates
// the processing to the JobRunner. This specific Job implementation
// does not directly reference the JobRunner, so it performs no operation
// or just logs.
func (j *HelloWorldJob) Run(ctx context.Context, jobExecution *model.JobExecution, jobParameters model.JobParameters) error {
	logger.Infof("HelloWorldJob.Run called for JobExecution ID: %s", jobExecution.ID)
	// The actual flow execution is handled by the JobRunner.
	return nil
}

// JobName returns the logical name of the job.
func (j *HelloWorldJob) JobName() string {
	return j.name
}

// ID returns the unique ID of the job definition.
func (j *HelloWorldJob) ID() string {
	return j.id
}

// GetFlow returns the job's flow definition structure.
func (j *HelloWorldJob) GetFlow() *model.FlowDefinition {
	return j.flow
}

// ValidateParameters validates job parameters before job execution.
func (j *HelloWorldJob) ValidateParameters(params model.JobParameters) error {
	// Parameter validation is not needed for this tutorial.
	return nil
}

// HelloWorldJob confirms that it implements the port.Job interface.
var _ port.Job = (*HelloWorldJob)(nil)
