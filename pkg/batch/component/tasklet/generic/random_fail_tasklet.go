package generic

import (
	"context"
	"math/rand"
	"strconv"
	"time"

	port "github.com/tigerroll/surfin/pkg/batch/core/application/port"
	model "github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	exception "github.com/tigerroll/surfin/pkg/batch/support/util/exception"
	logger "github.com/tigerroll/surfin/pkg/batch/support/util/logger"
)

// RandomFailTasklet is a [port.Tasklet] that fails with a configured probability.
// It is primarily used for testing and validating retry/restart mechanisms.
type RandomFailTasklet struct {
	id         string
	ec         model.ExecutionContext
	failRate   float64 // Probability of failure (0.0 - 1.0)
	failCount  int     // Number of failures before stopping (0 means always probabilistic)
	currentRun int     // Current execution count
}

// NewRandomFailTasklet creates a new instance of [RandomFailTasklet].
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

// Execute causes the tasklet to fail based on a configured probability or count.
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
func (t *RandomFailTasklet) Close(ctx context.Context) error {
	return nil
}

// SetExecutionContext sets the [model.ExecutionContext] for the tasklet.
func (t *RandomFailTasklet) SetExecutionContext(ctx context.Context, ec model.ExecutionContext) error {
	t.ec = ec
	// Restores currentRun from the previous execution context.
	if run, ok := ec.GetInt("random_fail_tasklet.current_run"); ok {
		t.currentRun = run
	}
	return nil
}

// GetExecutionContext retrieves the current [model.ExecutionContext] from the tasklet.
func (t *RandomFailTasklet) GetExecutionContext(ctx context.Context) (model.ExecutionContext, error) {
	// Saves the current run count.
	t.ec.Put("random_fail_tasklet.current_run", t.currentRun)
	return t.ec, nil
}

// Verify that [RandomFailTasklet] satisfies the [port.Tasklet] interface.
var _ port.Tasklet = (*RandomFailTasklet)(nil)
