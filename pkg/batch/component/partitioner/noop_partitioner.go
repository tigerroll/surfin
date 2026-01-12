package partitioner

import (
	"context"
	port "github.com/tigerroll/surfin/pkg/batch/core/application/port"
	model "github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	logger "github.com/tigerroll/surfin/pkg/batch/support/util/logger"
)

// NoOpPartitioner is a minimal implementation of the [port.Partitioner] interface.
// It always returns gridSize number of empty [model.ExecutionContext] instances.
type NoOpPartitioner struct{}

// NewNoOpPartitioner creates a new instance of [NoOpPartitioner].
func NewNoOpPartitioner() port.Partitioner {
	return &NoOpPartitioner{}
}

// Partition generates a specified number of empty [model.ExecutionContext] instances.
// It returns a map where keys are partition names and values are the corresponding ExecutionContexts.
func (p *NoOpPartitioner) Partition(ctx context.Context, gridSize int) (map[string]model.ExecutionContext, error) {
	logger.Debugf("NoOpPartitioner: Generating %d partitions.", gridSize)
	partitions := make(map[string]model.ExecutionContext, gridSize)
	for i := 0; i < gridSize; i++ {
		partitionName := model.PartitionName(i)
		partitions[partitionName] = model.NewExecutionContext()
	}
	return partitions, nil
}
// Verify that [NoOpPartitioner] satisfies the [port.Partitioner] interface.
var _ port.Partitioner = (*NoOpPartitioner)(nil)
