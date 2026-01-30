// Package job provides the Fx module for the "helloWorldJob" component.
// It registers the job builder with the JobFactory.
package job

import "go.uber.org/fx"
import support "github.com/tigerroll/surfin/pkg/batch/core/config/support"
import logger "github.com/tigerroll/surfin/pkg/batch/support/util/logger"

// RegisterHelloWorldJobBuilder registers the created JobBuilder with the JobFactory.
//
// It ensures that the "helloWorldJob" can be instantiated by the framework
// when referenced in JSL (Job Specification Language) files.
func RegisterHelloWorldJobBuilder(
	jf *support.JobFactory,
	builder support.JobBuilder,
) {
	// Register the JobBuilder with the JobFactory using the key "helloWorldJob".
	// This key must match the 'id' field in the JSL (e.g., job.yaml).
	jf.RegisterJobBuilder("helloWorldJob", builder)
	logger.Debugf("JobBuilder for helloWorldJob registered with JobFactory. JSL id: 'helloWorldJob'")
}

// provideHelloWorldJobBuilder provides the NewHelloWorldJob function as a support.JobBuilder type.
// The dependencies of NewHelloWorldJob are resolved when the JobBuilder returned by this function
// is actually invoked by the framework.
func provideHelloWorldJobBuilder() support.JobBuilder {
	return NewHelloWorldJob
}

// Module defines the Fx options for the helloWorldJob component.
var Module = fx.Options(
	fx.Provide(fx.Annotate(
		provideHelloWorldJobBuilder,           // The provideHelloWorldJobBuilder function returns a support.JobBuilder type.
		fx.ResultTags(`name:"helloWorldJob"`), // Tags the result so JobFactory can retrieve the JobBuilder by this name.
	)),
	fx.Invoke(fx.Annotate(
		RegisterHelloWorldJobBuilder,
		fx.ParamTags(``, `name:"helloWorldJob"`),
	)),
)
