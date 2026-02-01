// Package config provides core configuration structures and utilities for the batch framework.
// This file defines an interface and implementation for expanding environment variables within configuration data.
package config

import (
	"os"
)

// EnvironmentExpander provides functionality to expand environment variable placeholders
// within an input byte slice.
type EnvironmentExpander interface {
	// Expand takes a byte slice as input, expands any environment variable placeholders
	// (e.g., ${VAR} or $VAR) within it, and returns the expanded byte slice.
	//
	// Parameters:
	//   input: The byte slice containing data with potential environment variable placeholders.
	//
	// Returns:
	//   The byte slice with placeholders expanded, and an error if the expansion process fails.
	Expand(input []byte) ([]byte, error)
}

// OsEnvironmentExpander is an implementation of the EnvironmentExpander interface
// that uses Go's standard library `os.ExpandEnv` function to expand environment variables.
type OsEnvironmentExpander struct{}

// NewOsEnvironmentExpander creates and returns a new instance of OsEnvironmentExpander.
func NewOsEnvironmentExpander() *OsEnvironmentExpander {
	return &OsEnvironmentExpander{}
}

// Expand uses `os.ExpandEnv` to expand environment variables within the input byte slice.
// `os.ExpandEnv` replaces ${VAR} or $VAR in the string with the value of the environment
// variable VAR. If VAR is not set, it is replaced by an empty string.
// Note: `os.ExpandEnv` itself does not return an error, so the returned error will always be nil.
//
// Parameters:
//
//	input: The byte slice containing data with potential environment variable placeholders.
//
// Returns:
//
//	The byte slice with placeholders expanded, and always a nil error.
func (e *OsEnvironmentExpander) Expand(input []byte) ([]byte, error) {
	expandedString := os.ExpandEnv(string(input))
	return []byte(expandedString), nil
}
