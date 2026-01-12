// Package generic provides Fx modules for generic tasklet components.
package generic

import (
	"go.uber.org/fx"
	port "github.com/tigerroll/surfin/pkg/batch/core/application/port"
	config "github.com/tigerroll/surfin/pkg/batch/core/config"
	jsl "github.com/tigerroll/surfin/pkg/batch/core/config/jsl"
	support "github.com/tigerroll/surfin/pkg/batch/core/config/support"
	job "github.com/tigerroll/surfin/pkg/batch/core/domain/repository"
	"github.com/tigerroll/surfin/pkg/batch/support/util/logger"
)

// NewExecutionContextWriterTaskletComponentBuilderParams defines the dependencies for NewExecutionContextWriterTaskletComponentBuilder.
type NewExecutionContextWriterTaskletComponentBuilderParams struct {
	fx.In
}

// NewExecutionContextWriterTaskletComponentBuilder creates a jsl.ComponentBuilder for ExecutionContextWriterTasklet.
func NewExecutionContextWriterTaskletComponentBuilder() jsl.ComponentBuilder {
	return jsl.ComponentBuilder(func(
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
		return NewExecutionContextWriterTasklet("executionContextWriterTasklet", properties), nil
	})
}

// RegisterExecutionContextWriterTaskletBuilder registers the builder with the JobFactory.
func RegisterExecutionContextWriterTaskletBuilder(jf *support.JobFactory, builder jsl.ComponentBuilder) {
	jf.RegisterComponentBuilder("executionContextWriterTasklet", builder)
	logger.Debugf("Component 'executionContextWriterTasklet' was registered with JobFactory.")
}

// NewRandomFailTaskletComponentBuilderParams defines the dependencies for NewRandomFailTaskletComponentBuilder.
type NewRandomFailTaskletComponentBuilderParams struct {
	fx.In
}

// NewRandomFailTaskletComponentBuilder creates a jsl.ComponentBuilder for RandomFailTasklet.
func NewRandomFailTaskletComponentBuilder() jsl.ComponentBuilder {
	return jsl.ComponentBuilder(func(
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
		return NewRandomFailTasklet("randomFailTasklet", properties), nil
	})
}

// RegisterRandomFailTaskletBuilder registers the builder with the JobFactory.
func RegisterRandomFailTaskletBuilder(jf *support.JobFactory, builder jsl.ComponentBuilder) {
	jf.RegisterComponentBuilder("randomFailTasklet", builder)
	logger.Debugf("Component 'randomFailTasklet' was registered with JobFactory.")
}

// Module provides ComponentBuilders for generic Tasklets.
var Module = fx.Options(
	fx.Provide(fx.Annotate(
		NewExecutionContextWriterTaskletComponentBuilder,
		fx.ResultTags(`name:"executionContextWriterTasklet"`),
	)),
	fx.Invoke(fx.Annotate(
		RegisterExecutionContextWriterTaskletBuilder,
		fx.ParamTags(``, `name:"executionContextWriterTasklet"`),
	)),
	fx.Provide(fx.Annotate(
		NewRandomFailTaskletComponentBuilder,
		fx.ResultTags(`name:"randomFailTasklet"`),
	)),
	fx.Invoke(fx.Annotate(
		RegisterRandomFailTaskletBuilder,
		fx.ParamTags(``, `name:"randomFailTasklet"`),
	)),
)
