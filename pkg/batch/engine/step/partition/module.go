package partition

import (
	"github.com/tigerroll/surfin/pkg/batch/component/partitioner"
	port "github.com/tigerroll/surfin/pkg/batch/core/application/port"
	config "github.com/tigerroll/surfin/pkg/batch/core/config"
	metrics "github.com/tigerroll/surfin/pkg/batch/core/metrics"
	tx "github.com/tigerroll/surfin/pkg/batch/core/tx"
	remote "github.com/tigerroll/surfin/pkg/batch/infrastructure/remote"
	"go.uber.org/fx"
)

// StepExecutorProviderParams defines the dependencies required to select a StepExecutor.
type StepExecutorProviderParams struct {
	fx.In
	Config         *config.Config
	SimpleExecutor port.StepExecutor `name:"simpleStepExecutor"`
	RemoteExecutor port.StepExecutor `name:"remoteStepExecutor"`
}

// SimpleStepExecutorParams defines the dependencies for SimpleStepExecutor.
type SimpleStepExecutorParams struct {
	fx.In
	Tracer            metrics.Tracer
	MetricRecorder    metrics.MetricRecorder // MetricRecorder for metrics collection.
	MetadataTxManager tx.TransactionManager  `name:"metadata"` // Requests the metadata TxManager.
}

// ProvideStepExecutor selects and provides the appropriate core.StepExecutor based on configuration.
func ProvideStepExecutor(p StepExecutorProviderParams) port.StepExecutor {
	ref := p.Config.Surfin.Batch.StepExecutorRef

	switch ref {
	case "remoteStepExecutor":
		return p.RemoteExecutor
	case "simpleStepExecutor", "":
		// Default or explicit local execution
		return p.SimpleExecutor
	default:
		// Use SimpleExecutor as a fallback for unknown references
		return p.SimpleExecutor
	}
}

// RemoteStepExecutorParams defines the dependencies for RemoteStepExecutor.
type RemoteStepExecutorParams struct {
	fx.In
	Submitter port.RemoteJobSubmitter
	Tracer    metrics.Tracer
}

// Module defines the Fx options for PartitionStep related components.
var Module = fx.Options(
	// Provide SimpleStepExecutor with a name tag
	fx.Provide(fx.Annotate(
		func(p SimpleStepExecutorParams) port.StepExecutor {
			// SimpleStepExecutor now accepts Tracer, MetricRecorder, and TxManager.
			return NewSimpleStepExecutor(p.Tracer, p.MetricRecorder, p.MetadataTxManager)
		},
		fx.ResultTags(`name:"simpleStepExecutor"`),
	)),
	// Provide RemoteJobSubmitter (implementation of BirdClientSubmitter)
	fx.Provide(fx.Annotate(
		remote.NewBirdClientSubmitter,
		fx.As(new(port.RemoteJobSubmitter)), // Provide as port.RemoteJobSubmitter
	)),
	// Provide RemoteStepExecutor with a name tag (depends on RemoteJobSubmitter and Tracer)
	fx.Provide(fx.Annotate(
		func(p RemoteStepExecutorParams) port.StepExecutor {
			return NewRemoteStepExecutor(p.Submitter, p.Tracer)
		},
		fx.ResultTags(`name:"remoteStepExecutor"`), // Provide with name tag
	)),
	// Dynamically select and provide the StepExecutor interface
	fx.Provide(fx.Annotate(
		ProvideStepExecutor,
		fx.As(new(port.StepExecutor)),
	)),
	// Provide the builder for NoOpPartitioner
	fx.Provide(fx.Annotate(
		NewNoOpPartitionerBuilder,
		fx.ResultTags(`name:"noOpPartitioner"`),
	)),
)

// NewNoOpPartitionerBuilder returns the JSL builder for NoOpPartitioner.
func NewNoOpPartitionerBuilder() port.PartitionerBuilder {
	return func(properties map[string]string) (port.Partitioner, error) {
		// Use component/partitioner.NewNoOpPartitioner.
		return partitioner.NewNoOpPartitioner(), nil
	}
}
