package incrementer

import (
	"fmt"
	"strconv"
	"time"

	port "surfin/pkg/batch/core/application/port"
	core "surfin/pkg/batch/core/domain/model"
	logger "surfin/pkg/batch/support/util/logger"
)

// TimestampIncrementer is an implementation of JobParametersIncrementer that adds "timestamp" to job parameters.
// It sets the current Unix milliseconds if "timestamp" does not exist, or updates its value if it does.
type TimestampIncrementer struct {
	name string
}

// NewTimestampIncrementer creates a new instance of TimestampIncrementer.
func NewTimestampIncrementer(name string) *TimestampIncrementer {
	return &TimestampIncrementer{
		name: name,
	}
}

// GetNext adds or updates "timestamp" in the given JobParameters and returns the new parameters.
func (i *TimestampIncrementer) GetNext(params core.JobParameters) core.JobParameters {
	nextParams := core.NewJobParameters()
	// Copy existing parameters
	for k, v := range params.Params {
		nextParams.Put(k, v)
	}

	// Set current Unix milliseconds
	timestamp := time.Now().UnixMilli()
	nextParams.Put(i.name, strconv.FormatInt(timestamp, 10)) // Use i.name and store as string
	logger.Debugf("JobParametersIncrementer '%s': Setting '%s' to %d.", i.name, i.name, timestamp)

	return nextParams
}

// String returns the string representation of TimestampIncrementer.
func (i *TimestampIncrementer) String() string {
	return fmt.Sprintf("TimestampIncrementer[name=%s]", i.name)
}

// Ensure TimestampIncrementer implements core.JobParametersIncrementer
var _ port.JobParametersIncrementer = (*TimestampIncrementer)(nil)
