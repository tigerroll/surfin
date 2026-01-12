// pkg/batch/job/split/module.go

package split

import (
	"go.uber.org/fx"

	port "github.com/tigerroll/surfin/pkg/batch/core/application/port"
	"github.com/tigerroll/surfin/pkg/batch/core/config/jsl"
	"github.com/tigerroll/surfin/pkg/batch/core/config/support"
	"github.com/tigerroll/surfin/pkg/batch/support/util/logger"
)

// NewConcreteSplitBuilder creates a builder for a concrete implementation of the core.Split interface.
func NewConcreteSplitBuilder() jsl.SplitBuilder {
	return func(id string, steps []port.Step) (port.Split, error) {
		return NewConcreteSplit(id, steps), nil
	}
}

// RegisterSplitBuilder registers the created SplitBuilder with the JobFactory.
func RegisterSplitBuilder(
	jf *support.JobFactory,
	builder jsl.SplitBuilder,
) {
	// "concreteSplit" is the key for the default Split implementation used by the JSL converter.
	jf.RegisterSplitBuilder("concreteSplit", builder)
	logger.Debugf("Split builder 'concreteSplit' registered with JobFactory.")
}

// Module defines Fx options for Split-related components provided by the framework.
var Module = fx.Options(
	fx.Provide(fx.Annotate(
		NewConcreteSplitBuilder,
		fx.ResultTags(`name:"concreteSplit"`),
	)),
	fx.Invoke(fx.Annotate(
		RegisterSplitBuilder,
		fx.ParamTags(``, `name:"concreteSplit"`),
	)),
)
