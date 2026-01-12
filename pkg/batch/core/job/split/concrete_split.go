package split

import (
	port "surfin/pkg/batch/core/application/port"
)

// ConcreteSplit is a concrete implementation of the Split interface.
// It holds the steps to be executed in parallel.
type ConcreteSplit struct {
	id    string
	steps []port.Step
}

// NewConcreteSplit creates a new instance of ConcreteSplit.
func NewConcreteSplit(id string, steps []port.Step) *ConcreteSplit {
	return &ConcreteSplit{
		id:    id,
		steps: steps,
	}
}

// ID returns the split ID.
func (s *ConcreteSplit) ID() string {
	return s.id
}

// Steps returns the steps in the split.
func (s *ConcreteSplit) Steps() []port.Step {
	return s.steps
}

// Verify that ConcreteSplit implements the port.Split interface.
var _ port.Split = (*ConcreteSplit)(nil)
