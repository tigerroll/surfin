package incrementer

import (
	"fmt"

	port "surfin/pkg/batch/core/application/port"
	core "surfin/pkg/batch/core/domain/model"
	logger "surfin/pkg/batch/support/util/logger"
)

// RunIDIncrementer is an implementation of JobParametersIncrementer that adds or increments "run.id" in job parameters.
// It sets "run.id" to 1 if it does not exist, or increments its value if it does.
type RunIDIncrementer struct {
	name string
}

// NewRunIDIncrementer creates a new instance of RunIDIncrementer.
func NewRunIDIncrementer(name string) *RunIDIncrementer {
	return &RunIDIncrementer{
		name: name,
	}
}

// GetNext adds or increments "run.id" in the given JobParameters and returns the new parameters.
func (i *RunIDIncrementer) GetNext(params core.JobParameters) core.JobParameters {
	nextParams := core.NewJobParameters()
	// Copy existing parameters
	for k, v := range params.Params {
		nextParams.Put(k, v)
	}

	currentRunID, ok := params.GetInt(i.name)
	if !ok {
		// Set to 1 if "run.id" does not exist
		nextParams.Put(i.name, 1)
		logger.Debugf("JobParametersIncrementer '%s': '%s' not found, setting to 1.", i.name, i.name)
	} else {
		// Increment if it exists
		nextRunID := currentRunID + 1
		nextParams.Put(i.name, nextRunID)
		logger.Debugf("JobParametersIncrementer '%s': Incrementing '%s' from %d to %d.", i.name, i.name, currentRunID, nextRunID)
	}

	return nextParams
}

// String returns the string representation of RunIDIncrementer.
func (i *RunIDIncrementer) String() string {
	return fmt.Sprintf("RunIDIncrementer[name=%s]", i.name)
}

// Ensure RunIDIncrementer implements core.JobParametersIncrementer
var _ port.JobParametersIncrementer = (*RunIDIncrementer)(nil)
