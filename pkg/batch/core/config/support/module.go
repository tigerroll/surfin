package support

import (
	"go.uber.org/fx"
	port "github.com/tigerroll/surfin/pkg/batch/core/application/port"
)

// PartitionerBuildersParams is an Fx parameter struct for aggregating PartitionerBuilders.
type PartitionerBuildersParams struct {
	fx.In
	NoOpBuilder port.PartitionerBuilder `name:"noOpPartitioner"`
}

// RegisterPartitionerBuilders registers PartitionerBuilders with the JobFactory.
func RegisterPartitionerBuilders(jf *JobFactory, p PartitionerBuildersParams) {
	jf.RegisterPartitionerBuilder("noOpPartitioner", p.NoOpBuilder)
}

// Module defines Fx options related to JobFactory.
var Module = fx.Options(
	fx.Provide(NewJobFactory),
	fx.Invoke(RegisterPartitionerBuilders),
)
