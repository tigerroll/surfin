package runner

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	port "surfin/pkg/batch/core/application/port"
	model "surfin/pkg/batch/core/domain/model"
	metrics "surfin/pkg/batch/core/metrics"
	repository "surfin/pkg/batch/core/domain/repository"
	exception "surfin/pkg/batch/support/util/exception"
	logger "surfin/pkg/batch/support/util/logger"
)

// FlowJob is an implementation of core.Job that executes a job based on a flow defined in JSL.
type FlowJob struct {
	id            string
	name          string
	flow          *model.FlowDefinition
	jobRepository repository.JobRepository
	jobListeners  []port.JobExecutionListener
	metricRecorder metrics.MetricRecorder
	tracer        metrics.Tracer
	// JobParametersIncrementer is used by JobLauncher, so it is not held directly by FlowJob
}

// Verify that FlowJob implements the core.Job interface.
var _ port.Job = (*FlowJob)(nil)

// NewFlowJob creates a new instance of FlowJob.
func NewFlowJob(
	id string,
	name string,
	flow *model.FlowDefinition,
	jobRepository repository.JobRepository,
	jobListeners []port.JobExecutionListener,
	metricRecorder metrics.MetricRecorder,
	tracer metrics.Tracer,
) *FlowJob {
	return &FlowJob{
		id:            id,
		name:          name,
		flow:          flow,
		jobRepository: jobRepository,
		jobListeners:  jobListeners,
		metricRecorder: metricRecorder,
		tracer:        tracer,
	}
}

// ID returns the job ID.
func (j *FlowJob) ID() string {
	return j.id
}

// JobName returns the job name.
func (j *FlowJob) JobName() string {
	return j.name
}

// GetFlow returns the job flow definition.
func (j *FlowJob) GetFlow() *model.FlowDefinition {
	return j.flow
}

// ValidateParameters validates job parameters.
// Currently returns nil, but job-specific validation logic can be added here.
func (j *FlowJob) ValidateParameters(params model.JobParameters) error {
	// Use params.String() to ensure sensitive parameters are masked during logging (I.4 implementation)
	logger.Debugf("Job '%s': Executing JobParameters validation. Parameters: %s", j.name, params.String())
	// Example check for a required parameter 'flow_id' (skipped/modified here as FlowJob doesn't require it)
	if _, ok := params.GetString("flow_id"); !ok {
		// FlowJob does not require flow_id, so skip check or modify
		// return exception.NewBatchErrorf(j.name, "Required parameter 'flow_id' not found.")
	}
	return nil
}

// notifyBeforeJob calls the BeforeJob method of registered JobExecutionListeners.
func (j *FlowJob) notifyBeforeJob(ctx context.Context, jobExecution *model.JobExecution) {
	for _, l := range j.jobListeners {
		l.BeforeJob(ctx, jobExecution)
	}
}

// notifyAfterJob calls the AfterJob method of registered JobExecutionListeners.
func (j *FlowJob) notifyAfterJob(ctx context.Context, jobExecution *model.JobExecution) {
	for _, l := range j.jobListeners {
		l.AfterJob(ctx, jobExecution)
	}
}

// Run defines the job execution logic.
// It sequentially executes steps and decisions based on the job flow definition.
func (j *FlowJob) Run(ctx context.Context, jobExecution *model.JobExecution, jobParameters model.JobParameters) error {
	logger.Infof("Starting Job '%s' (Execution ID: %s).", j.name, jobExecution.ID)

	// Start tracing
	ctx, finishSpan := j.tracer.StartJobSpan(ctx, jobExecution)
	defer finishSpan()

	// Record job start metric
	j.metricRecorder.RecordJobStart(ctx, jobExecution)

	// Notify before job execution
	j.notifyBeforeJob(ctx, jobExecution)

	// Post-job execution processing (guaranteed execution via defer)
	defer func() {
		// Set job end time (if not already set by MarkAsFailed/Completed/Stopped)
		if jobExecution.EndTime == nil {
			now := time.Now()
			jobExecution.EndTime = &now
		}

		j.notifyAfterJob(ctx, jobExecution)

		// Record job end metric
		j.metricRecorder.RecordJobEnd(ctx, jobExecution)

		logger.Infof("Job '%s' (Execution ID: %s) finished. Final Status: %s, Exit Status: %s",
			j.name, jobExecution.ID, jobExecution.Status, jobExecution.ExitStatus)
		
		// Log StepExecution details without sensitive EC data
		for _, se := range jobExecution.StepExecutions {
			logger.Debugf("  StepExecution Details (Step: %s): %s", se.StepName, se.DebugString())
		}
	}()

	// Start execution from the flow's starting element
	currentElementID := j.flow.StartElement
	
	// If the job is restarted, or resuming from a previously failed step
	if jobExecution.CurrentStepName != "" && (jobExecution.Status == model.BatchStatusRestarting || jobExecution.Status == model.BatchStatusStarted) {
		// If JobLauncher transitioned to RESTARTING or STARTED, resume from CurrentStepName.
		logger.Infof("Job '%s' is restarting/resuming from step '%s'.", j.name, jobExecution.CurrentStepName)
		currentElementID = jobExecution.CurrentStepName
	}

	for {
		select {
		case <-ctx.Done():
			logger.Warnf("Context cancelled, interrupting execution of Job '%s': %v", j.name, ctx.Err())
			jobExecution.AddFailureException(ctx.Err())
			jobExecution.MarkAsStopped()
			j.tracer.RecordError(ctx, "job_runner", ctx.Err())
			return ctx.Err()
		default:
		}

		elementInterface, ok := j.flow.Elements[currentElementID]
		if !ok {
			err := exception.NewBatchErrorf(j.name, "Flow element '%s' not found", currentElementID)
			logger.Errorf("Job '%s': %v", j.name, err)
			jobExecution.AddFailureException(err)
			jobExecution.MarkAsFailed(err)
			j.tracer.RecordError(ctx, "job_runner", err)
			return err
		}

		element, isFlowElement := elementInterface.(port.FlowElement)
		if !isFlowElement {
			err := exception.NewBatchErrorf(j.name, "Flow element '%s' is not a valid FlowElement type: %T", currentElementID, elementInterface)
			jobExecution.AddFailureException(err)
			jobExecution.MarkAsFailed(err)
			j.tracer.RecordError(ctx, "job_runner", err)
			return err
		}

		logger.Debugf("Job '%s': Executing flow element '%s'.", j.name, currentElementID)

		var elementExitStatus model.ExitStatus
		var elementErr error

		switch elem := element.(type) {
		case port.Step:
			stepName := elem.StepName()
			jobExecution.CurrentStepName = stepName // Set the currently executing step name in JobExecution

			// Search for existing StepExecution (for restart/rerun)
			var stepExecution *model.StepExecution
			for _, se := range jobExecution.StepExecutions {
				// Find a StepExecution whose StepName matches and is not yet completed.
				if se.StepName == stepName && se.Status != model.BatchStatusCompleted {
					stepExecution = se
					logger.Infof("Job '%s': Reusing existing StepExecution (ID: %s) for step '%s' (Status: %s).", j.name, stepExecution.ID, stepName, stepExecution.Status)
					break
				}
			}
			
			isNewExecution := stepExecution == nil
			if !isNewExecution && stepExecution.Status == model.BatchStatusStarting {
				// StepExecution created by SimpleJobLauncher during restart is persisted here for the first time.
				isNewExecution = true
			}

			if stepExecution == nil {
				// For new execution
				stepExecution = model.NewStepExecution(model.NewID(), jobExecution, stepName)
				jobExecution.AddStepExecution(stepExecution)
				if err := j.jobRepository.SaveStepExecution(ctx, stepExecution); err != nil {
					logger.Errorf("Job '%s': Failed to save StepExecution (ID: %s): %v", j.name, stepExecution.ID, err)
					jobExecution.AddFailureException(err)
					jobExecution.MarkAsFailed(err)
					j.tracer.RecordError(ctx, "job_runner", err)
					return exception.NewBatchError(j.name, "Error saving new StepExecution", err, false, false)
				}
				logger.Infof("Job '%s': Created and saved new StepExecution (ID: %s).", j.name, stepExecution.ID)
			} else if isNewExecution {
				// Initial persistence of StepExecution created in memory during restart
				// Status should be BatchStatusStarting
				if err := j.jobRepository.SaveStepExecution(ctx, stepExecution); err != nil {
					logger.Errorf("Job '%s': Failed to save restarted StepExecution (ID: %s): %v", j.name, stepExecution.ID, err)
					jobExecution.AddFailureException(err)
					jobExecution.MarkAsFailed(err)
					j.tracer.RecordError(ctx, "job_runner", err)
					return exception.NewBatchError(j.name, "Error saving restarted StepExecution", err, false, false)
				}
				logger.Infof("Job '%s': Saved restarted StepExecution (ID: %s).", j.name, stepExecution.ID)
			} else if stepExecution.Status == model.BatchStatusCompleted {
				// If the step is already completed, skip it and transition to the next element.
				logger.Infof("Job '%s': Step '%s' (ID: %s) already completed. Skipping execution.", j.name, stepName, stepExecution.ID)
				elementExitStatus = model.ExitStatusCompleted
				elementErr = nil
				goto TransitionEvaluation // Skip step execution and proceed to transition evaluation
			}

			// Inject StepExecution into context before execution, especially important for Chunk processing components (Writers)
			executionCtx := port.GetContextWithStepExecution(ctx, stepExecution) 

			elementErr = elem.Execute(executionCtx, jobExecution, stepExecution)
			elementExitStatus = stepExecution.ExitStatus

			// Promote ExecutionContext
			if promotion := elem.GetExecutionContextPromotion(); promotion != nil {
				// 1. Keys promotion
				for _, key := range promotion.Keys {
					if val, ok := stepExecution.ExecutionContext.GetNested(key); ok {
						jobExecution.ExecutionContext.PutNested(key, val)
						logger.Debugf("FlowJob: Promoted key '%s' from Step '%s' to JobExecutionContext.", key, stepName)
					}
				}

				// 2. JobLevelKeys promotion (with renaming)
				for stepKey, jobKey := range promotion.JobLevelKeys {
					if val, ok := stepExecution.ExecutionContext.GetNested(stepKey); ok {
						jobExecution.ExecutionContext.PutNested(jobKey, val)
						logger.Debugf("FlowJob: Promoted and renamed key '%s' to '%s' from Step '%s' in JobExecutionContext.", stepKey, jobKey, stepName)
					}
				}
				// Persist JobExecution after promotion
				if err := j.jobRepository.UpdateJobExecution(ctx, jobExecution); err != nil {
					logger.Errorf("FlowJob: Failed to update JobExecution after context promotion for step '%s': %v", stepName, err)
					// This is not fatal, but promoted context might be lost on restart, so log it.
				}
			}

			if elementErr != nil {
				logger.Errorf("Job '%s': Error occurred during execution of step '%s': %v", j.name, stepName, elementErr)
				jobExecution.AddFailureException(elementErr)
				j.tracer.RecordError(ctx, "job_runner", elementErr)
				if errors.Is(elementErr, context.Canceled) {
					jobExecution.MarkAsStopped()
				} else {
					jobExecution.MarkAsFailed(elementErr)
				}
			} else {
				logger.Infof("Job '%s': Step '%s' completed successfully. ExitStatus: %s", j.name, stepName, elementExitStatus)
			}

		case port.Decision:
			decisionName := elem.ID()
			decisionResult, err := elem.Decide(ctx, jobExecution, jobParameters)
			elementExitStatus = decisionResult
			elementErr = err

			if elementErr != nil {
				logger.Errorf("Job '%s': Error occurred during execution of decision '%s': %v", j.name, decisionName, elementErr)
				jobExecution.AddFailureException(elementErr)
				j.tracer.RecordError(ctx, "job_runner", elementErr)
				if errors.Is(elementErr, context.Canceled) {
					jobExecution.MarkAsStopped()
				} else {
					jobExecution.MarkAsFailed(elementErr)
				}
			} else {
				logger.Infof("Job '%s': Decision '%s' completed. Result: %s", j.name, decisionName, decisionResult)
			}

		case port.Split:
			splitName := elem.ID()
			steps := elem.Steps() // Get port.Step[]
			logger.Infof("Job '%s': Executing Split '%s'. Number of parallel steps: %d", j.name, splitName, len(steps))

			// Start Split Span
			splitCtx, finishSplitSpan := j.tracer.StartStepSpan(ctx, &model.StepExecution{StepName: splitName, JobExecution: jobExecution})
			defer finishSplitSpan()

			var wg sync.WaitGroup
			splitErrors := make(chan error, len(steps))
			splitExitStatuses := make(chan model.ExitStatus, len(steps))

			for _, s := range steps {
				wg.Add(1)
				go func(splitStep port.Step) {
					defer wg.Done()
					stepName := splitStep.StepName()
					logger.Infof("Job '%s': Starting step '%s' within Split.", j.name, stepName)

					// Search for existing StepExecution (for restart/rerun)
					var splitStepExecution *model.StepExecution
					for _, se := range jobExecution.StepExecutions {
						if se.StepName == stepName && se.Status != model.BatchStatusCompleted {
							splitStepExecution = se
							logger.Infof("Job '%s': Reusing existing StepExecution (ID: %s) for step '%s' within Split (Status: %s).", j.name, splitStepExecution.ID, stepName, splitStepExecution.Status)
							break
						}
					}

					if splitStepExecution == nil {
						// For new execution
						splitStepExecution = model.NewStepExecution(model.NewID(), jobExecution, stepName)
						jobExecution.AddStepExecution(splitStepExecution)
					} else if splitStepExecution.Status == model.BatchStatusCompleted {
						// Skip if the step is already completed
						logger.Infof("Job '%s': Step '%s' (ID: %s) already completed within Split. Skipping execution.", j.name, stepName, splitStepExecution.ID)
						splitExitStatuses <- model.ExitStatusCompleted
						return
					}

					if err := j.jobRepository.SaveStepExecution(splitCtx, splitStepExecution); err != nil {
						logger.Errorf("Job '%s': Failed to save StepExecution (ID: %s) within Split: %v", j.name, splitStepExecution.ID, err)
						splitErrors <- exception.NewBatchError(j.name, fmt.Sprintf("Error saving StepExecution '%s' within Split", stepName), err, false, false)
						splitExitStatuses <- model.ExitStatusFailed
						j.tracer.RecordError(splitCtx, "job_runner", err)
						return
					}
					
					// StepExecution.GetContextWithStepExecution has been moved to port.interfaces.go
					executionCtx := port.GetContextWithStepExecution(splitCtx, splitStepExecution)

					err := splitStep.Execute(executionCtx, jobExecution, splitStepExecution)
					if err != nil {
						logger.Errorf("Job '%s': Error occurred during execution of step '%s' within Split: %v", j.name, stepName, err)
						splitErrors <- err
						j.tracer.RecordError(splitCtx, "job_runner", err)
					} else {
						logger.Infof("Job '%s': Step '%s' within Split completed successfully. ExitStatus: %s", j.name, stepName, splitStepExecution.ExitStatus)
					}
					splitExitStatuses <- splitStepExecution.ExitStatus
				}(s)
			}
			wg.Wait()
			close(splitErrors)
			close(splitExitStatuses)

			var combinedSplitError error
			
			for err := range splitErrors {
				if err != nil {
					// Combine into elementErr (treated as the error for the entire Split)
					combinedSplitError = errors.Join(combinedSplitError, err)
				}
			}
			elementErr = combinedSplitError

			// Determine ExitStatus: FAILED if even one error occurred, otherwise COMPLETED
			if elementErr != nil {
				elementExitStatus = model.ExitStatusFailed
			} else {
				elementExitStatus = model.ExitStatusCompleted
			}
			
			logger.Infof("Job '%s': Execution of Split '%s' completed. Result: %s", j.name, splitName, elementExitStatus)

		default:
			err := exception.NewBatchErrorf(j.name, "Unknown flow element type: %T (ID: %s)", element, currentElementID)
			logger.Errorf("Job '%s': %v", j.name, err)
			jobExecution.AddFailureException(err)
			jobExecution.MarkAsFailed(err)
			j.tracer.RecordError(ctx, "job_runner", err)
			return err
		}
	TransitionEvaluation:

		// Evaluate transition rules to determine the next element
		transitionRule, found := j.flow.GetTransitionRule(element.ID(), elementExitStatus, elementErr != nil)
		if !found {
			if elementErr == nil {
				logger.Infof("Job '%s': No transition rule found from flow element '%s'. Completing job.", j.name, element.ID())
				jobExecution.MarkAsCompleted()
			} else {
				logger.Errorf("Job '%s': Error occurred in flow element '%s', but no appropriate transition rule found. Failing job.", j.JobName(), element.ID())
				jobExecution.MarkAsFailed(elementErr)
				j.tracer.RecordError(ctx, "job_runner", elementErr)
			}
			break
		}

		// Handle End, Fail, Stop instructions from the transition rule
		if transitionRule.Transition.End {
			logger.Infof("Job '%s': 'End' transition instructed from flow element '%s'. Completing job.", j.JobName(), element.ID())
			jobExecution.MarkAsCompleted()
			break
		}
		if transitionRule.Transition.Fail {
			logger.Errorf("Job '%s': 'Fail' transition instructed from flow element '%s'. Failing job.", j.JobName(), element.ID())
			failErr := fmt.Errorf("explicit fail transition from %s", element.ID())
			jobExecution.MarkAsFailed(failErr)
			j.tracer.RecordError(ctx, "job_runner", failErr)
			break
		}
		if transitionRule.Transition.Stop {
			logger.Infof("Job '%s': 'Stop' transition instructed from flow element '%s'. Stopping job.", j.JobName(), element.ID())
			jobExecution.MarkAsStopped()
			break
		}

		// Update the next element ID
		currentElementID = transitionRule.Transition.To
	}

	logger.Infof("Execution of Job '%s' (Execution ID: %s) completed.", j.JobName(), jobExecution.ID)
	return nil
}
