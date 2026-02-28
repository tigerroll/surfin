package generic

import (
	"context"
	"math/rand"
	"strconv"
	"time"

	// Package generic provides general-purpose tasklet implementations.
	// These tasklets are designed to be reusable across various batch jobs.

	port "github.com/tigerroll/surfin/pkg/batch/core/application/port"
	model "github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	exception "github.com/tigerroll/surfin/pkg/batch/support/util/exception"
	logger "github.com/tigerroll/surfin/pkg/batch/support/util/logger"
)

// RandomFailTasklet is a [port.Tasklet] that simulates failure based on a configured probability or count.
// It is useful for testing and validating the retry and restart mechanisms of the batch framework.
type RandomFailTasklet struct {
	id         string
	ec         model.ExecutionContext
	failRate   float64 // Probability of failure (0.0 - 1.0)
	failCount  int     // Number of failures before stopping (0 means always probabilistic)
	currentRun int     // Current execution count
}

// NewRandomFailTasklet creates a new [RandomFailTasklet] instance.
//
// Parameters:
//
//	id: The unique identifier for this tasklet.
//	properties: A map of properties that can configure the tasklet's behavior:
//	            - "failRate": A float64 string (0.0 to 1.0) representing the probability of failure (default: 0.5).
//	            - "failCount": An int string representing the number of times the tasklet should fail
//	                           before succeeding (0 means always probabilistic, default: 0).
//
// Returns:
//
//	*RandomFailTasklet: A new instance of [RandomFailTasklet].
func NewRandomFailTasklet(id string, properties map[string]string) *RandomFailTasklet {
	failRate := 0.5 // Default 50%
	failCount := 0  // Default 0 (always probabilistic)

	if rateStr, ok := properties["failRate"]; ok {
		if rate, err := strconv.ParseFloat(rateStr, 64); err == nil {
			failRate = rate
		}
	}
	if countStr, ok := properties["failCount"]; ok {
		if count, err := strconv.Atoi(countStr); err == nil {
			failCount = count
		}
	}

	return &RandomFailTasklet{
		id:         id,
		ec:         model.NewExecutionContext(),
		failRate:   failRate,
		failCount:  failCount,
		currentRun: 0,
	}
}

// Open initializes the tasklet and prepares any necessary resources.
// For [RandomFailTasklet], no specific resources need to be opened.
// The [model.StepExecution] is provided for context, but its ExecutionContext is expected
// to be set via [SetExecutionContext] before [Open] is called.
//
// Parameters:
//
//	ctx: The context for the operation.
//	stepExecution: The current [model.StepExecution] instance.
//
// Returns:
//
//	error: An error if initialization fails.
func (t *RandomFailTasklet) Open(ctx context.Context, stepExecution *model.StepExecution) error {
	logger.Debugf("RandomFailTasklet '%s' Open called.", t.id)
	// ExecutionContext is already set by SetExecutionContext before Open is called.
	return nil
}

// Execute causes the tasklet to fail based on a configured probability or count.
// If `failCount` is set, it will fail for that many runs, then succeed.
// If `failCount` is 0, it will fail probabilistically based on `failRate`.
//
// Parameters:
//
//	ctx: The context for the operation.
//	stepExecution: The current [model.StepExecution] instance.
//
// Returns:
//
//	model.ExitStatus: The exit status of the tasklet ([model.ExitStatusCompleted] or [model.ExitStatusFailed]).
//	error: An error if the tasklet is configured to fail.
func (t *RandomFailTasklet) Execute(ctx context.Context, stepExecution *model.StepExecution) (model.ExitStatus, error) {
	t.currentRun++

	shouldFail := false

	if t.failCount > 0 {
		// If a specific failure count is set
		if t.currentRun <= t.failCount {
			shouldFail = true
		}
	} else {
		// If probabilistic failure is set
		rand.Seed(time.Now().UnixNano())
		if rand.Float64() < t.failRate {
			shouldFail = true
		}
	}

	if shouldFail {
		logger.Errorf("RandomFailTasklet '%s' (Run %d): Intentionally failing (Rate: %.2f, Count: %d).", t.id, t.currentRun, t.failRate, t.failCount)
		// The failure is marked as not retryable and not skippable.
		return model.ExitStatusFailed, exception.NewBatchErrorf(t.id, "Random failure occurred on run %d", t.currentRun, false, false)
	}

	logger.Infof("RandomFailTasklet '%s' (Run %d): Completed successfully.", t.id, t.currentRun)
	return model.ExitStatusCompleted, nil
}

// Close releases any resources held by the tasklet.
// For [RandomFailTasklet], there are no specific resources to close, so it always returns nil.
//
// Parameters:
//
//	ctx: The context for the operation.
//	stepExecution: The current [model.StepExecution] instance.
//
// Returns:
//
//	error: An error if closing fails (always nil for this tasklet).
func (t *RandomFailTasklet) Close(ctx context.Context, stepExecution *model.StepExecution) error {
	return nil
}

// SetExecutionContext sets the [model.ExecutionContext] for the tasklet.
// This method is called by the framework to provide the tasklet with its current execution context.
// It also restores the `currentRun` state from the provided context for restartability.
//
// Parameters:
//
//	ec: The [model.ExecutionContext] to set.
func (t *RandomFailTasklet) SetExecutionContext(ec model.ExecutionContext) {
	t.ec = ec
	// Restores currentRun from the previous execution context.
	if run, ok := ec.GetInt("random_fail_tasklet.current_run"); ok {
		t.currentRun = run
	}
}

// GetExecutionContext retrieves the current [model.ExecutionContext] from the tasklet.
// It saves the current `currentRun` count into the context before returning it.
//
// Returns:
//
//	model.ExecutionContext: The current [model.ExecutionContext].
func (t *RandomFailTasklet) GetExecutionContext() model.ExecutionContext {
	// Saves the current run count.
	t.ec.Put("random_fail_tasklet.current_run", t.currentRun)
	return t.ec
}

// Verify that [RandomFailTasklet] satisfies the [port.Tasklet] interface.
var _ port.Tasklet = (*RandomFailTasklet)(nil)
