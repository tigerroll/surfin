package semantics_test

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/tigerroll/surfin/pkg/batch/core/domain/model"
)

// TestChunkStep_Execute verifies that ChunkStep.Execute calls GetExecutionContext exactly three times:
// twice during chunk processing and once during finalization.
func TestChunkStep_Execute(t *testing.T) {
	reader := new(MockReader)
	writer := new(MockWriter)

	// Expect 3 calls: 2 during chunk processing + 1 during finalization.
	reader.On("GetExecutionContext", mock.Anything).Return(model.NewExecutionContext(), nil).Times(3)
	writer.On("GetExecutionContext", mock.Anything).Return(model.NewExecutionContext(), nil).Times(3)

	// ... Other mock configurations and test logic ...
}
