// Package runner provides implementations for job execution runners.
// It includes the FlowJobRunner, which orchestrates job execution based on a flow definition.
package runner

import (
	// Standard library imports
	"context"

	port "github.com/tigerroll/surfin/pkg/batch/core/application/port"
	model "github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	repository "github.com/tigerroll/surfin/pkg/batch/core/domain/repository"
	metrics "github.com/tigerroll/surfin/pkg/batch/core/metrics"
	exception "github.com/tigerroll/surfin/pkg/batch/support/util/exception"
	logger "github.com/tigerroll/surfin/pkg/batch/support/util/logger"
)

// FlowJobRunner is an implementation of JobRunner that executes a job based on its flow definition.
type FlowJobRunner struct {
	jobRepository repository.JobRepository
	stepExecutor  port.StepExecutor
	tracer        metrics.Tracer
}

// NewFlowJobRunner creates a new FlowJobRunner.
func NewFlowJobRunner(
	repo repository.JobRepository,
	executor port.StepExecutor,
	tracer metrics.Tracer,
) *FlowJobRunner {
	return &FlowJobRunner{
		jobRepository: repo,
		stepExecutor:  executor,
		tracer:        tracer,
	}
}

// Run starts the execution according to the job's flow definition.
// This method orchestrates the job flow by executing steps, decisions, and splits.
func (r *FlowJobRunner) Run(ctx context.Context, jobInstance port.Job, jobExecution *model.JobExecution, flowDef *model.FlowDefinition) {
	logger.Infof("FlowJobRunner: Starting execution for Job '%s' (Execution ID: %s).", jobInstance.JobName(), jobExecution.ID)

	// Update JobExecution status to STARTED.
	jobExecution.MarkAsStarted() // Mark the job execution as started.
	if err := r.jobRepository.UpdateJobExecution(ctx, jobExecution); err != nil {
		logger.Errorf("FlowJobRunner: Failed to update JobExecution status to STARTED: %v", err)
		jobExecution.MarkAsFailed(err)
		r.jobRepository.UpdateJobExecution(ctx, jobExecution) // Attempt to save the final status.
		return
	}

	// Start a tracing span for job execution.
	jobCtx, endJobSpan := r.tracer.StartJobSpan(ctx, jobExecution)
	defer endJobSpan()

	// Get the starting element from the flow definition.
	currentElementID := flowDef.StartElement
	var currentElement interface{}
	var ok bool

	for {
		select {
		case <-jobCtx.Done():
			logger.Warnf("FlowJobRunner: Job context cancelled for Job '%s' (Execution ID: %s).", jobInstance.JobName(), jobExecution.ID) // Log cancellation.
			jobExecution.MarkAsStopped()
			r.jobRepository.UpdateJobExecution(jobCtx, jobExecution)
			return
		default:
			// Continue
		}

		currentElement, ok = flowDef.Elements[currentElementID]
		if !ok { // Check if the current element exists in the flow definition.
			err := exception.NewBatchErrorf("flow_runner", "Flow element '%s' not found in flow definition for job '%s'", currentElementID, jobInstance.JobName())
			logger.Errorf("FlowJobRunner: %v", err)
			jobExecution.MarkAsFailed(err)
			r.jobRepository.UpdateJobExecution(jobCtx, jobExecution)
			return
		}

		var exitStatus model.ExitStatus
		var elementErr error

		switch element := currentElement.(type) {
		case port.Step:
			logger.Infof("FlowJobRunner: Executing Step '%s' for Job '%s'.", element.StepName(), jobInstance.JobName()) // Log step execution.

			// Create a new StepExecution.
			stepExecution := model.NewStepExecution(model.NewID(), jobExecution, element.StepName())
			jobExecution.AddStepExecution(stepExecution)      // Add to the list of StepExecutions for the JobExecution.
			jobExecution.CurrentStepName = element.StepName() // Update the current step name.

			// Save the StepExecution initially (workaround if SimpleStepExecutor doesn't save).
			// Although StepExecutor should handle this within a transaction, the current implementation
			// might be lacking, so this compensates. This ensures that the first UpdateStepExecution
			// call within TaskletStep/ChunkStep succeeds.
			if err := r.jobRepository.SaveStepExecution(jobCtx, stepExecution); err != nil {
				elementErr = exception.NewBatchError(element.StepName(), "Failed to save initial StepExecution", err, false, false)
				exitStatus = model.ExitStatusFailed
				logger.Errorf("FlowJobRunner: Failed to save initial StepExecution for Step '%s': %v", element.StepName(), err)
				jobExecution.MarkAsFailed(elementErr)
				r.jobRepository.UpdateJobExecution(jobCtx, jobExecution)
				return // Exit the Run method.
			}
			// Execute the step.
			executedStepExecution, err := r.stepExecutor.ExecuteStep(jobCtx, element, jobExecution, stepExecution)
			if err != nil {
				elementErr = err
				exitStatus = model.ExitStatusFailed
				logger.Errorf("FlowJobRunner: Step '%s' failed: %v", element.StepName(), err)
			} else {
				exitStatus = executedStepExecution.ExitStatus
				logger.Infof("FlowJobRunner: Step '%s' completed with ExitStatus: %s", element.StepName(), exitStatus)
			}

			// Promote ExecutionContext from Step to Job.
			if promotion := element.GetExecutionContextPromotion(); promotion != nil {
				for _, key := range promotion.Keys {
					if val, ok := executedStepExecution.ExecutionContext.Get(key); ok {
						jobExecution.ExecutionContext.Put(key, val)
					}
				}
				for stepKey, jobKey := range promotion.JobLevelKeys {
					if val, ok := executedStepExecution.ExecutionContext.Get(stepKey); ok {
						jobExecution.ExecutionContext.Put(jobKey, val)
					}
				}
			}

		case port.Decision:
			logger.Infof("FlowJobRunner: Executing Decision '%s' for Job '%s'.", element.DecisionName(), jobInstance.JobName())
			// Determine the next path based on the decision.
			decisionExitStatus, err := element.Decide(jobCtx, jobExecution, jobExecution.Parameters)
			if err != nil {
				elementErr = err
				exitStatus = model.ExitStatusFailed
				logger.Errorf("FlowJobRunner: Decision '%s' failed: %v", element.DecisionName(), err)
			} else {
				exitStatus = decisionExitStatus
				logger.Infof("FlowJobRunner: Decision '%s' resulted in ExitStatus: %s", element.DecisionName(), exitStatus)
			}

		case port.Split:
			logger.Infof("FlowJobRunner: Executing Split '%s' for Job '%s'.", element.ID(), jobInstance.JobName())
			// TODO: Implement parallel execution for Split.
			// Currently, it returns an error as it's not yet implemented.
			elementErr = exception.NewBatchErrorf("flow_runner", "Split execution is not yet implemented for Split '%s'", element.ID())
			exitStatus = model.ExitStatusFailed
			logger.Errorf("FlowJobRunner: %v", elementErr)

		default:
			elementErr = exception.NewBatchErrorf("flow_runner", "Unknown flow element type for ID '%s': %T", currentElementID, currentElement)
			exitStatus = model.ExitStatusFailed
			logger.Errorf("FlowJobRunner: %v", elementErr)
		}

		// Search for the next transition rule.
		nextRule, found := flowDef.GetTransitionRule(currentElementID, exitStatus, elementErr != nil)
		if !found {
			// If a specific rule is not found, try a wildcard or default rule.
			nextRule, found = flowDef.GetTransitionRule(currentElementID, model.ExitStatusUnknown, elementErr != nil) // Check for '*'
		}

		if !found { // If no transition rule is found, the job terminates as failed.
			err := exception.NewBatchErrorf("flow_runner", "No transition rule found for element '%s' with ExitStatus '%s' (error: %v)", currentElementID, exitStatus, elementErr)
			logger.Errorf("FlowJobRunner: %v", err)
			jobExecution.MarkAsFailed(err)
			r.jobRepository.UpdateJobExecution(jobCtx, jobExecution)
			return
		}

		// Apply the transition rule.
		if nextRule.Transition.End {
			jobExecution.MarkAsCompleted()
			if elementErr != nil { // If there was an error but the transition is 'end', still mark as completed.
				jobExecution.AddFailureException(elementErr)
			}
			logger.Infof("FlowJobRunner: Job '%s' (Execution ID: %s) completed with ExitStatus: %s (Transition: END).", jobInstance.JobName(), jobExecution.ID, jobExecution.ExitStatus)
			break // Exit the loop.
		} else if nextRule.Transition.Fail {
			jobExecution.MarkAsFailed(elementErr)
			logger.Infof("FlowJobRunner: Job '%s' (Execution ID: %s) failed with ExitStatus: %s (Transition: FAIL).", jobInstance.JobName(), jobExecution.ID, jobExecution.ExitStatus)
			break // Exit the loop.
		} else if nextRule.Transition.Stop {
			jobExecution.MarkAsStopped()
			logger.Infof("FlowJobRunner: Job '%s' (Execution ID: %s) stopped with ExitStatus: %s (Transition: STOP).", jobInstance.JobName(), jobExecution.ID, jobExecution.ExitStatus)
			break // Exit the loop.
		} else if nextRule.Transition.To != "" {
			currentElementID = nextRule.Transition.To
			logger.Debugf("FlowJobRunner: Transitioning to next element: '%s'", currentElementID)
		} else {
			// This should not happen if validation is correct, but as a safeguard.
			err := exception.NewBatchErrorf("flow_runner", "Invalid transition rule for element '%s': no 'to', 'end', 'fail', or 'stop' specified", currentElementID)
			logger.Errorf("FlowJobRunner: %v", err)
			jobExecution.MarkAsFailed(err) // Mark job as failed.
			break // Exit the loop.
		}
	}

	// Final update of JobExecution (if not already updated by a break condition).
	if !jobExecution.Status.IsFinished() { // If the loop ends without an explicit final status, consider it completed.
		jobExecution.MarkAsCompleted()
	}
	if err := r.jobRepository.UpdateJobExecution(jobCtx, jobExecution); err != nil {
		logger.Errorf("FlowJobRunner: Failed to update final JobExecution status: %v", err)
	}
	logger.Infof("FlowJobRunner: Job '%s' (Execution ID: %s) finished with status: %s, ExitStatus: %s",
		jobInstance.JobName(), jobExecution.ID, jobExecution.Status, jobExecution.ExitStatus)
}
