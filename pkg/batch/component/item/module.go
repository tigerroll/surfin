// Package item provides Fx modules for various item-related components,
// including No-Op readers/writers, pass-through processors, and execution context writers.
package item

import (
	"go.uber.org/fx"

	port "github.com/tigerroll/surfin/pkg/batch/core/application/port"
	config "github.com/tigerroll/surfin/pkg/batch/core/config"
	jsl "github.com/tigerroll/surfin/pkg/batch/core/config/jsl"
	support "github.com/tigerroll/surfin/pkg/batch/core/config/support"
	job "github.com/tigerroll/surfin/pkg/batch/core/domain/repository"
	"github.com/tigerroll/surfin/pkg/batch/support/util/logger"
)

// NewNoOpItemReaderComponentBuilder creates a jsl.ComponentBuilder for a No-Op ItemReader.
func NewNoOpItemReaderComponentBuilder() jsl.ComponentBuilder {
	return func(
		_ *config.Config,
		_ job.JobRepository,
		_ port.ExpressionResolver,
		_ port.DBConnectionResolver,
		_ map[string]string,
	) (interface{}, error) {
		// NewNoOpItemReader does not require any dependencies.
		return NewNoOpItemReader[any](), nil
	}
}

// NewPassThroughItemProcessorComponentBuilder creates a jsl.ComponentBuilder for a Pass-Through ItemProcessor.
func NewPassThroughItemProcessorComponentBuilder() jsl.ComponentBuilder {
	return func(
		_ *config.Config,
		_ job.JobRepository,
		_ port.ExpressionResolver,
		_ port.DBConnectionResolver,
		_ map[string]string,
	) (interface{}, error) {
		// NewPassThroughItemProcessor does not require any dependencies.
		return NewPassThroughItemProcessor[any](), nil
	}
}

// NewNoOpItemWriterComponentBuilder creates a jsl.ComponentBuilder for a No-Op ItemWriter.
func NewNoOpItemWriterComponentBuilder() jsl.ComponentBuilder {
	return func(
		_ *config.Config,
		_ job.JobRepository,
		_ port.ExpressionResolver,
		_ port.DBConnectionResolver,
		_ map[string]string,
	) (interface{}, error) {
		// NewNoOpItemWriter does not require any dependencies.
		return NewNoOpItemWriter[any](), nil
	}
}

// NewExecutionContextItemWriterComponentBuilder creates a jsl.ComponentBuilder for ExecutionContextItemWriter.
func NewExecutionContextItemWriterComponentBuilder() jsl.ComponentBuilder {
	return func(
		cfg *config.Config,
		repo job.JobRepository,
		resolver port.ExpressionResolver,
		dbResolver port.DBConnectionResolver,
		properties map[string]string,
	) (interface{}, error) {
		// Arguments unnecessary for this component are ignored.
		_ = cfg
		_ = repo
		_ = resolver
		_ = dbResolver

		key, ok := properties["key"]
		if !ok || key == "" {
			key = "writer_context"
		}
		return NewExecutionContextItemWriter[any](key), nil
	}
}

// genericItemBuilders is a struct to receive all generic component builders from Fx.
type genericItemBuilders struct {
	fx.In
	NoOpReaderBuilder          jsl.ComponentBuilder `name:"noOpItemReader"`
	PassThroughProcBuilder     jsl.ComponentBuilder `name:"passThroughItemProcessor"`
	NoOpWriterBuilder          jsl.ComponentBuilder `name:"noOpItemWriter"`
	ExecutionContextItemWriter jsl.ComponentBuilder `name:"executionContextItemWriter"`
}

// RegisterGenericItemBuilders registers all generic component builders with the JobFactory.
func RegisterGenericItemBuilders(jf *support.JobFactory, builders genericItemBuilders) {
	jf.RegisterComponentBuilder("noOpItemReader", builders.NoOpReaderBuilder)
	jf.RegisterComponentBuilder("passThroughItemProcessor", builders.PassThroughProcBuilder)
	jf.RegisterComponentBuilder("noOpItemWriter", builders.NoOpWriterBuilder)
	jf.RegisterComponentBuilder("executionContextItemWriter", builders.ExecutionContextItemWriter)
	logger.Debugf("Generic item components (noOpItemReader, passThroughItemProcessor, noOpItemWriter, executionContextItemWriter) were registered with JobFactory.")
}

// Module defines Fx options for generic item-related components provided by the framework.
var Module = fx.Options(
	fx.Provide(fx.Annotate(
		NewNoOpItemReaderComponentBuilder,
		fx.ResultTags(`name:"noOpItemReader"`),
	)),
	fx.Provide(fx.Annotate(
		NewPassThroughItemProcessorComponentBuilder,
		fx.ResultTags(`name:"passThroughItemProcessor"`),
	)),
	fx.Provide(fx.Annotate(
		NewNoOpItemWriterComponentBuilder,
		fx.ResultTags(`name:"noOpItemWriter"`),
	)),
	fx.Provide(fx.Annotate(
		NewExecutionContextItemWriterBuilder,
		fx.ResultTags(`name:"executionContextItemWriter"`),
	)),
	fx.Invoke(RegisterGenericItemBuilders),
)
