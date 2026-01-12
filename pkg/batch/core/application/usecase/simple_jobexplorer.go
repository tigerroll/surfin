package usecase

import (
	"context"
	"fmt"

	model "github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	job "github.com/tigerroll/surfin/pkg/batch/core/domain/repository"
	exception "github.com/tigerroll/surfin/pkg/batch/support/util/exception"
	logger "github.com/tigerroll/surfin/pkg/batch/support/util/logger"
)

// SimpleJobExplorer is a simple implementation of the JobExplorer interface.
// It queries batch metadata using a JobRepository.
type SimpleJobExplorer struct {
	jobRepository job.JobRepository
}

// Verify that SimpleJobExplorer implements the JobExplorer interface.
var _ JobExplorer = (*SimpleJobExplorer)(nil)

// NewSimpleJobExplorer creates a new instance of SimpleJobExplorer.
func NewSimpleJobExplorer(jobRepository job.JobRepository) *SimpleJobExplorer {
	return &SimpleJobExplorer{
		jobRepository: jobRepository,
	}
}

// GetJobExecution retrieves a JobExecution by its ID.
func (e *SimpleJobExplorer) GetJobExecution(ctx context.Context, executionID string) (*model.JobExecution, error) {
	logger.Infof("JobExplorer: GetJobExecution method called. Execution ID: %s", executionID)
	jobExecution, err := e.jobRepository.FindJobExecutionByID(ctx, executionID)
	if err != nil {
		return nil, exception.NewBatchError("job_explorer", fmt.Sprintf("Failed to retrieve JobExecution (ID: %s)", executionID), err, false, false)
	}
	logger.Debugf("Retrieved JobExecution (ID: %s) from JobRepository.", executionID)
	return jobExecution, nil
}

// GetJobExecutions retrieves all JobExecutions associated with the specified JobInstance.
func (e *SimpleJobExplorer) GetJobExecutions(ctx context.Context, instanceID string) ([]*model.JobExecution, error) {
	logger.Infof("JobExplorer: GetJobExecutions method called. Instance ID: %s", instanceID)

	jobInstance, err := e.jobRepository.FindJobInstanceByID(ctx, instanceID)
	if err != nil {
		return nil, exception.NewBatchError("job_explorer", fmt.Sprintf("Failed to retrieve JobInstance (ID: %s)", instanceID), err, false, false)
	}
	if jobInstance == nil {
		logger.Warnf("JobInstance (ID: %s) not found.", instanceID)
		return []*model.JobExecution{}, nil
	}

	jobExecutions, err := e.jobRepository.FindJobExecutionsByJobInstance(ctx, jobInstance)
	if err != nil {
		return nil, exception.NewBatchError("job_explorer", fmt.Sprintf("Failed to retrieve JobExecutions associated with JobInstance (ID: %s)", instanceID), err, false, false)
	}

	logger.Debugf("Retrieved %d JobExecutions associated with JobInstance (ID: %s).", len(jobExecutions), instanceID)
	return jobExecutions, nil
}

// GetLastJobExecution retrieves the latest JobExecution for a given JobInstance.
func (e *SimpleJobExplorer) GetLastJobExecution(ctx context.Context, instanceID string) (*model.JobExecution, error) {
	logger.Infof("JobExplorer: GetLastJobExecution method called. Instance ID: %s", instanceID)

	jobExecution, err := e.jobRepository.FindLatestRestartableJobExecution(ctx, instanceID)
	if err != nil {
		return nil, exception.NewBatchError("job_explorer", fmt.Sprintf("Failed to retrieve the latest JobExecution for JobInstance (ID: %s)", instanceID), err, false, false)
	}

	if jobExecution != nil {
		logger.Debugf("Retrieved latest JobExecution (ID: %s) for JobInstance (ID: %s) from JobRepository.", jobExecution.ID, instanceID)
	} else {
		logger.Warnf("Latest JobExecution for JobInstance (ID: %s) not found.", instanceID)
	}

	return jobExecution, nil
}

// GetJobInstance retrieves a JobInstance by its ID.
func (e *SimpleJobExplorer) GetJobInstance(ctx context.Context, instanceID string) (*model.JobInstance, error) {
	logger.Infof("JobExplorer: GetJobInstance method called. Instance ID: %s", instanceID)
	jobInstance, err := e.jobRepository.FindJobInstanceByID(ctx, instanceID)
	if err != nil {
		return nil, exception.NewBatchError("job_explorer", fmt.Sprintf("Failed to retrieve JobInstance (ID: %s)", instanceID), err, false, false)
	}

	if jobInstance != nil {
		logger.Debugf("Retrieved JobInstance (ID: %s) from JobRepository.", instanceID)
	} else {
		logger.Warnf("JobInstance (ID: %s) not found.", instanceID)
	}

	return jobInstance, nil
}

// GetJobInstances searches for JobInstances matching the specified job name and parameters.
func (e *SimpleJobExplorer) GetJobInstances(ctx context.Context, jobName string, params model.JobParameters) ([]*model.JobInstance, error) {
	logger.Infof("JobExplorer: GetJobInstances method called. Job Name: %s, Parameters: %+v", jobName, params)

	jobInstances, err := e.jobRepository.FindJobInstancesByJobNameAndPartialParameters(ctx, jobName, params)
	if err != nil {
		return nil, exception.NewBatchError("job_explorer", fmt.Sprintf("Failed to search for JobInstance (JobName: %s, Parameters: %+v)", jobName, params), err, false, false)
	}

	if len(jobInstances) > 0 {
		logger.Debugf("Retrieved %d JobInstances matching JobName '%s' and Parameters.", len(jobInstances), jobName)
	} else {
		logger.Warnf("No JobInstances found matching JobName '%s' and Parameters.", jobName)
	}

	return jobInstances, nil
}

// GetJobNames retrieves all registered job names.
func (e *SimpleJobExplorer) GetJobNames(ctx context.Context) ([]string, error) {
	logger.Infof("JobExplorer: GetJobNames method called.")
	jobNames, err := e.jobRepository.GetJobNames(ctx)
	if err != nil {
		return nil, exception.NewBatchError("job_explorer", "Failed to retrieve registered job names", err, false, false)
	}
	logger.Debugf("Retrieved %d job names.", len(jobNames))
	return jobNames, nil
}

// GetParameters retrieves the JobParameters for the specified JobExecution.
func (e *SimpleJobExplorer) GetParameters(ctx context.Context, executionID string) (model.JobParameters, error) {
	logger.Infof("JobExplorer: GetParameters method called. Execution ID: %s", executionID)

	jobExecution, err := e.jobRepository.FindJobExecutionByID(ctx, executionID)
	if err != nil {
		return model.NewJobParameters(), exception.NewBatchError("job_explorer", fmt.Sprintf("Failed to retrieve JobExecution (ID: %s)", executionID), err, false, false)
	}
	if jobExecution == nil {
		logger.Warnf("JobExecution (ID: %s) not found.", executionID)
		return model.NewJobParameters(), exception.NewBatchErrorf("job_explorer", "JobExecution (ID: %s) not found", executionID)
	}

	logger.Debugf("Retrieved JobParameters for JobExecution (ID: %s).", executionID)
	return jobExecution.Parameters, nil
}
