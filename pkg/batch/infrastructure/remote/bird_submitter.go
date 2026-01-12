package remote

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.uber.org/fx"

	port "github.com/tigerroll/surfin/pkg/batch/core/application/port"
	cfg "github.com/tigerroll/surfin/pkg/batch/core/config"
	model "github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	repository "github.com/tigerroll/surfin/pkg/batch/core/domain/repository"
	exception "github.com/tigerroll/surfin/pkg/batch/support/util/exception"
	logger "github.com/tigerroll/surfin/pkg/batch/support/util/logger"
)

// BirdClientSubmitterParams holds the dependencies injected via DI.
type BirdClientSubmitterParams struct {
	fx.In
	Config        *cfg.Config
	JobRepository repository.JobRepository
}

// BirdClientSubmitter is an implementation of core.RemoteJobSubmitter for submitting remote jobs via the Surfin Bird orchestrator's API.
type BirdClientSubmitter struct {
	apiEndpoint     string
	jobRepository   repository.JobRepository
	pollingInterval time.Duration
}

// NewBirdClientSubmitter creates a new instance of BirdClientSubmitter.
// In a real scenario, this would retrieve API endpoints and authentication information from the configuration.
func NewBirdClientSubmitter(p BirdClientSubmitterParams) port.RemoteJobSubmitter {
	batchConfig := p.Config.Surfin.Batch

	pollingInterval := time.Duration(batchConfig.PollingIntervalSeconds) * time.Second
	if pollingInterval == 0 {
		// Fallback for cases where default value is not set or is 0.
		pollingInterval = 10 * time.Second
	}

	return &BirdClientSubmitter{
		apiEndpoint:     batchConfig.APIEndpoint,
		jobRepository:   p.JobRepository,
		pollingInterval: pollingInterval,
	}
}

// SubmitWorkerJob sends StepExecution information to the Surfin Bird API to request remote execution.
func (s *BirdClientSubmitter) SubmitWorkerJob(ctx context.Context, jobExecution *model.JobExecution, stepExecution *model.StepExecution, workerStep port.Step) (string, error) {
	workerStepName := workerStep.StepName()

	logger.Infof("BirdClientSubmitter: Requesting execution of Worker Step '%s' to Surfin Bird API (%s).", workerStepName, s.apiEndpoint)

	// Actual API call logic would go here.

	// For now, we assume the API call is successful and return a dummy job ID.
	remoteJobID := fmt.Sprintf("bird-job-%s", stepExecution.ID)

	logger.Debugf("BirdClientSubmitter: Obtained remote job ID '%s'.", remoteJobID)

	return remoteJobID, nil
}

// AwaitCompletion waits for the completion of a remote job by polling the shared Job Repository.
func (s *BirdClientSubmitter) AwaitCompletion(ctx context.Context, remoteJobID string, stepExecution *model.StepExecution) error {
	logger.Infof("BirdClientSubmitter: Waiting for completion of remote job '%s' (StepExecution ID: %s) by polling the shared repository. Polling interval: %v", remoteJobID, stepExecution.ID, s.pollingInterval)

	ticker := time.NewTicker(s.pollingInterval)
	defer ticker.Stop()

	// Mock enhancement: Counts polling attempts to aid in state transition simulation during tests.
	pollCount := 0

	for {
		select {
		case <-ctx.Done():
			// If context is cancelled (e.g., timeout, job stop request).
			logger.Warnf("BirdClientSubmitter: Waiting for remote job '%s' interrupted by context: %v", remoteJobID, ctx.Err())

			// Attempting to update StepExecution status to STOPPED and persist.
			stepExecution.MarkAsStopped()
			if updateErr := s.jobRepository.UpdateStepExecution(ctx, stepExecution); updateErr != nil {
				logger.Errorf("BirdClientSubmitter: Failed to update StepExecution (ID: %s) status to STOPPED: %v", stepExecution.ID, updateErr)
			}

			return exception.NewBatchError("remote_agent", fmt.Sprintf("Remote execution wait interrupted: %v", ctx.Err()), ctx.Err(), false, true)

		case <-ticker.C:
			// Polling interval elapsed.
			pollCount++
			logger.Debugf("BirdClientSubmitter: Polling #%d (StepExecution ID: %s).", pollCount, stepExecution.ID)

			// 1. Retrieve the latest StepExecution status.
			latestSE, err := s.jobRepository.FindStepExecutionByID(ctx, stepExecution.ID)
			if err != nil {
				if errors.Is(err, repository.ErrStepExecutionNotFound) {
					logger.Errorf("BirdClientSubmitter: StepExecution ID '%s' not found in repository.", stepExecution.ID)
					return exception.NewBatchErrorf("remote_agent", "StepExecution not found: %s", stepExecution.ID)
				}
				if exception.IsOptimisticLockingFailure(err) {
					// Ignore optimistic locking errors and retrieve the latest state in the next poll.
					continue
				}
				// Possible retryable error, such as a DB connection error.
				logger.Errorf("BirdClientSubmitter: An error occurred while retrieving StepExecution ID '%s': %v", stepExecution.ID, err)
				// Continuing polling.
				continue
			}

			// 2. Check status.
			if latestSE.Status.IsFinished() {
				logger.Infof("BirdClientSubmitter: Remote StepExecution ID '%s' reached a terminal state (%s).", stepExecution.ID, latestSE.Status)

				// Copy all fields except ID, JobExecutionID, StepName, StartTime, and Version.
				stepExecution.EndTime = latestSE.EndTime
				stepExecution.Status = latestSE.Status
				stepExecution.ExitStatus = latestSE.ExitStatus
				stepExecution.Failures = latestSE.Failures
				stepExecution.ReadCount = latestSE.ReadCount
				stepExecution.WriteCount = latestSE.WriteCount
				stepExecution.CommitCount = latestSE.CommitCount
				stepExecution.RollbackCount = latestSE.RollbackCount
				stepExecution.FilterCount = latestSE.FilterCount
				stepExecution.SkipReadCount = latestSE.SkipReadCount
				stepExecution.SkipProcessCount = latestSE.SkipProcessCount
				stepExecution.SkipWriteCount = latestSE.SkipWriteCount
				stepExecution.ExecutionContext = latestSE.ExecutionContext
				stepExecution.LastUpdated = latestSE.LastUpdated

				// Determine whether to return an error based on the terminal status.
				if latestSE.Status == model.BatchStatusFailed || latestSE.Status == model.BatchStatusAbandoned {
					// If the remote worker failed.
					return exception.NewBatchErrorf("remote_agent", "Remote StepExecution ID '%s' failed. Status: %s", stepExecution.ID, latestSE.Status)
				}

				return nil // Normal completion.
			}

			logger.Debugf("BirdClientSubmitter: Remote StepExecution ID '%s' is still running (%s).", stepExecution.ID, latestSE.Status)
		}
	}
}

var _ port.RemoteJobSubmitter = (*BirdClientSubmitter)(nil)
