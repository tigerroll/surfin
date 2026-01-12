package job

import (
	port "github.com/tigerroll/surfin/pkg/batch/core/application/port"
	config "github.com/tigerroll/surfin/pkg/batch/core/config"
	"github.com/tigerroll/surfin/pkg/batch/core/config/support"
	model "github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	repository "github.com/tigerroll/surfin/pkg/batch/core/domain/repository"
	jobRunner "github.com/tigerroll/surfin/pkg/batch/core/job/runner"
	metrics "github.com/tigerroll/surfin/pkg/batch/core/metrics"
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
		// Returns runner.FlowJob instead of SimpleJob.
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
