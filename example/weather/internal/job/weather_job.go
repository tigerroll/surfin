package job

import (
	port "surfin/pkg/batch/core/application/port"
	config "surfin/pkg/batch/core/config"
	"surfin/pkg/batch/core/config/support"
	model "surfin/pkg/batch/core/domain/model"
	repository "surfin/pkg/batch/core/domain/repository"
	jobRunner "surfin/pkg/batch/core/job/runner"
	metrics "surfin/pkg/batch/core/metrics"
)

// NewWeatherJobBuilder creates a JobBuilder function for the weatherJob.
// This builder uses the generic FlowJob implementation from the core framework.
func NewWeatherJobBuilder() support.JobBuilder {
	return func(
		jobRepository repository.JobRepository, 
		cfg *config.Config, 
		listeners []port.JobExecutionListener, 
		flow *model.FlowDefinition,
		metricRecorder metrics.MetricRecorder,
		tracer metrics.Tracer,
	) (port.Job, error) {
		// The job name "weatherJob" must match the ID in the JSL file (job.yaml).
		// SimpleJob の代わりに runner.FlowJob を返す
		jobInstance := jobRunner.NewFlowJob(
			"weatherJob", // ID
			"weatherJob", // Name
			flow,
			jobRepository,
			listeners,
			metricRecorder,
			tracer,
		)
		return jobInstance, nil
	}
}
