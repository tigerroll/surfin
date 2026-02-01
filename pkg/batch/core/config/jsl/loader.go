package jsl

import (
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/tigerroll/surfin/pkg/batch/core/config"
	"github.com/tigerroll/surfin/pkg/batch/support/util/exception"
	logger "github.com/tigerroll/surfin/pkg/batch/support/util/logger"
)

// LoadedJobDefinitions holds all loaded JSL job definitions, mapped by their ID.
// This map is used by the JobFactory to retrieve job definitions.
var LoadedJobDefinitions = make(map[string]Job)

// LoadJSLDefinitionFromBytes loads job definitions from a single JSL YAML file byte slice.
// This function is typically used by the application to load an embedded JSL file.
// It accepts an EnvironmentExpander to expand environment variables within the JSL bytes
// before unmarshalling them into a Job struct.
//
// Parameters:
//
//	data: The byte slice containing the JSL YAML definition.
//	expander: An implementation of the EnvironmentExpander interface used to expand environment variables.
//
// Returns:
//
//	An error if loading, environment variable expansion, or unmarshalling fails,
//	or if the JSL definition is invalid (e.g., missing ID, name, flow elements, or duplicate ID).
func LoadJSLDefinitionFromBytes(data []byte, expander config.EnvironmentExpander) error {
	logger.Infof("Starting JSL definition loading.")

	// Expand environment variables in the JSL data.
	expandedData, err := expander.Expand(data)
	if err != nil {
		// os.ExpandEnv does not return an error, but this check is kept for future expander implementations.
		return exception.NewBatchError("jsl_loader", "Failed to expand environment variables in JSL definition", err, false, false)
	}

	var jobDef Job
	if err := yaml.Unmarshal(expandedData, &jobDef); err != nil {
		return exception.NewBatchError("jsl_loader", "Failed to parse JSL file", err, false, false)
	}

	if jobDef.ID == "" {
		return exception.NewBatchError("jsl_loader", "'id' is not defined in JSL file", nil, false, false)
	}
	if jobDef.Name == "" {
		return exception.NewBatchError("jsl_loader", fmt.Sprintf("JSL job '%s' does not have 'name' defined", jobDef.ID), nil, false, false)
	}
	if jobDef.Flow.StartElement == "" {
		return exception.NewBatchError("jsl_loader", fmt.Sprintf("JSL job '%s' flow does not have 'start-element' defined", jobDef.ID), nil, false, false)
	}
	if len(jobDef.Flow.Elements) == 0 {
		return exception.NewBatchError("jsl_loader", fmt.Sprintf("JSL job '%s' flow does not have 'elements' defined", jobDef.ID), nil, false, false)
	}

	if _, exists := LoadedJobDefinitions[jobDef.ID]; exists {
		return exception.NewBatchError("jsl_loader", fmt.Sprintf("JSL Job ID '%s' is duplicated", jobDef.ID), nil, false, false)
	}

	LoadedJobDefinitions[jobDef.ID] = jobDef
	logger.Infof("Loaded JSL job '%s'.", jobDef.ID)
	logger.Infof("JSL definition loading completed. Number of jobs loaded: %d", len(LoadedJobDefinitions))
	return nil
}

// GetJobDefinition retrieves a JSL Job definition by its ID.
//
// Parameters:
//
//	jobID: The unique identifier of the job.
//
// Returns:
//
//	The Job struct and a boolean indicating whether the job was found.
func GetJobDefinition(jobID string) (Job, bool) {
	job, ok := LoadedJobDefinitions[jobID]
	return job, ok
}

// GetLoadedJobCount returns the number of loaded JSL job definitions.
func GetLoadedJobCount() int {
	return len(LoadedJobDefinitions)
}
