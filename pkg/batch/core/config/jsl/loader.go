package jsl

import (
	"fmt"

	"gopkg.in/yaml.v3"

	"surfin/pkg/batch/support/util/exception"
	logger "surfin/pkg/batch/support/util/logger"
)

// LoadedJobDefinitions holds all loaded JSL job definitions.
// This map is used by the JobFactory to retrieve job definitions by their ID.
var LoadedJobDefinitions = make(map[string]Job)

// LoadJSLDefinitionFromBytes loads job definitions from a single JSL YAML file byte slice.
// This function is typically used by the application to load a single embedded JSL file.
func LoadJSLDefinitionFromBytes(data []byte) error {
	logger.Infof("Starting JSL definition loading.") // Log message for starting JSL definition loading.

	var jobDef Job
	if err := yaml.Unmarshal(data, &jobDef); err != nil {
		return exception.NewBatchError("jsl_loader", "Failed to parse JSL file", err, false, false)
	}

	if jobDef.ID == "" {
		return exception.NewBatchError("jsl_loader", "'id' is not defined in JSL file", nil, false, false) // Error if job ID is missing.
	}
	if jobDef.Name == "" {
		return exception.NewBatchError("jsl_loader", fmt.Sprintf("JSL job '%s' does not have 'name' defined", jobDef.ID), nil, false, false) // Error if job name is missing.
	}
	if jobDef.Flow.StartElement == "" {
		return exception.NewBatchError("jsl_loader", fmt.Sprintf("JSL job '%s' flow does not have 'start-element' defined", jobDef.ID), nil, false, false) // Error if flow start-element is missing.
	}
	if len(jobDef.Flow.Elements) == 0 {
		return exception.NewBatchError("jsl_loader", fmt.Sprintf("JSL job '%s' flow does not have 'elements' defined", jobDef.ID), nil, false, false) // Error if flow elements are missing.
	}

	if _, exists := LoadedJobDefinitions[jobDef.ID]; exists {
		return exception.NewBatchError("jsl_loader", fmt.Sprintf("JSL Job ID '%s' is duplicated", jobDef.ID), nil, false, false) // Error if job ID is duplicated.
	}

	LoadedJobDefinitions[jobDef.ID] = jobDef
	logger.Infof("Loaded JSL job '%s'.", jobDef.ID)
	logger.Infof("JSL definition loading completed. Number of jobs loaded: %d", len(LoadedJobDefinitions))
	return nil
}

// GetJobDefinition retrieves a JSL Job definition by its ID.
func GetJobDefinition(jobID string) (Job, bool) {
	job, ok := LoadedJobDefinitions[jobID]
	return job, ok
}

// GetLoadedJobCount returns the number of loaded JSL job definitions.
func GetLoadedJobCount() int {
	return len(LoadedJobDefinitions)
}
