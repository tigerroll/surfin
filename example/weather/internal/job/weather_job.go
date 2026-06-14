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

// NewHistoryJobBuilder creates a JobBuilder function for the historyJob.
func NewHistoryJobBuilder() support.JobBuilder {
	return func(
		jobRepository repository.JobRepository,
		cfg *config.Config,
		listeners []port.JobExecutionListener,
		flow *model.FlowDefinition,
		metricRecorder metrics.MetricRecorder,
		tracer metrics.Tracer,
	) (port.Job, error) {
		jobInstance := jobRunner.NewFlowJob(
			"historyJob", // ID
			"historyJob", // Name
			flow,
			jobRepository,
			listeners,
			metricRecorder,
			tracer,
		)
		return jobInstance, nil
	}
}

// NewForecastJobBuilder creates a JobBuilder function for the forecastJob.
func NewForecastJobBuilder() support.JobBuilder {
	return func(
		jobRepository repository.JobRepository,
		cfg *config.Config,
		listeners []port.JobExecutionListener,
		flow *model.FlowDefinition,
		metricRecorder metrics.MetricRecorder,
		tracer metrics.Tracer,
	) (port.Job, error) {
		jobInstance := jobRunner.NewFlowJob(
			"forecastJob", // ID
			"forecastJob", // Name
			flow,
			jobRepository,
			listeners,
			metricRecorder,
			tracer,
		)
		return jobInstance, nil
	}
}
