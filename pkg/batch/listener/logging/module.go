package logging

import (
	"go.uber.org/fx"
	
	config "github.com/tigerroll/surfin/pkg/batch/core/config"
	port "github.com/tigerroll/surfin/pkg/batch/core/application/port"
	jsl "github.com/tigerroll/surfin/pkg/batch/core/config/jsl"
	support "github.com/tigerroll/surfin/pkg/batch/core/config/support"
	"github.com/tigerroll/surfin/pkg/batch/support/util/logger"
)

// NewLoggingJobListenerBuilder creates a ComponentBuilder for LoggingJobListener.
func NewLoggingJobListenerBuilder() jsl.JobExecutionListenerBuilder {
	return func(
		_ *config.Config, 
		properties map[string]string,
	) (port.JobExecutionListener, error) {
		return NewLoggingJobListener(properties), nil
	}
}

// NewLoggingStepListenerBuilder creates a ComponentBuilder for LoggingStepListener.
func NewLoggingStepListenerBuilder() jsl.StepExecutionListenerBuilder {
	return func(
		_ *config.Config, 
		properties map[string]string,
	) (port.StepExecutionListener, error) {
		return NewLoggingStepListener(properties), nil
	}
}

// NewLoggingChunkListenerBuilder creates a ComponentBuilder for LoggingChunkListener.
func NewLoggingChunkListenerBuilder() jsl.ChunkListenerBuilder {
	return func(
		_ *config.Config, 
		properties map[string]string,
	) (port.ChunkListener, error) {
		return NewLoggingChunkListener(properties), nil
	}
}

// NewLoggingItemReadListenerBuilder creates a ComponentBuilder for LoggingItemReadListener.
func NewLoggingItemReadListenerBuilder() jsl.ItemReadListenerBuilder {
	return func(
		_ *config.Config, 
		properties map[string]string,
	) (port.ItemReadListener, error) {
		return NewLoggingItemReadListener(properties), nil
	}
}

// NewLoggingItemProcessListenerBuilder creates a ComponentBuilder for LoggingItemProcessListener.
func NewLoggingItemProcessListenerBuilder() jsl.ItemProcessListenerBuilder {
	return func(
		_ *config.Config, 
		properties map[string]string,
	) (port.ItemProcessListener, error) {
		return NewLoggingItemProcessListener(properties), nil
	}
}

// NewLoggingItemWriteListenerBuilder creates a ComponentBuilder for LoggingItemWriteListener.
func NewLoggingItemWriteListenerBuilder() jsl.ItemWriteListenerBuilder {
	return func(
		_ *config.Config, 
		properties map[string]string,
	) (port.ItemWriteListener, error) {
		return NewLoggingItemWriteListener(properties), nil
	}
}

// NewLoggingSkipListenerBuilder creates a ComponentBuilder for LoggingSkipListener.
func NewLoggingSkipListenerBuilder() jsl.SkipListenerBuilder {
	return func(
		_ *config.Config, 
		properties map[string]string,
	) (port.SkipListener, error) {
		return NewLoggingSkipListener(properties), nil
	}
}

// NewLoggingRetryItemListenerBuilder creates a ComponentBuilder for LoggingRetryItemListener.
func NewLoggingRetryItemListenerBuilder() jsl.LoggingRetryItemListenerBuilder {
	return func(
		_ *config.Config, 
		properties map[string]string,
	) (port.RetryItemListener, error) {
		return NewLoggingRetryItemListener(properties), nil
	}
}

// AllListenerBuilders is a struct to receive all listener builders from Fx.
type AllListenerBuilders struct {
	fx.In
	JobListenerBuilder    jsl.JobExecutionListenerBuilder `name:"loggingJobListener"`
	StepListenerBuilder   jsl.StepExecutionListenerBuilder `name:"loggingStepListener"`
	ChunkListenerBuilder  jsl.ChunkListenerBuilder `name:"loggingChunkListener"`
	ItemReadListenerBuilder jsl.ItemReadListenerBuilder `name:"loggingItemReadListener"`
	ItemProcessListenerBuilder jsl.ItemProcessListenerBuilder `name:"loggingItemProcessListener"`
	ItemWriteListenerBuilder jsl.ItemWriteListenerBuilder `name:"loggingItemWriteListener"`
	SkipListenerBuilder   jsl.SkipListenerBuilder `name:"loggingSkipListener"`
	RetryItemListenerBuilder jsl.LoggingRetryItemListenerBuilder `name:"loggingRetryItemListener"`
}

// RegisterAllListeners registers all listener builders with the JobFactory.
func RegisterAllListeners(jf *support.JobFactory, builders AllListenerBuilders) {
	jf.RegisterJobListenerBuilder("loggingJobListener", builders.JobListenerBuilder)
	jf.RegisterStepExecutionListenerBuilder("loggingStepListener", builders.StepListenerBuilder)
	jf.RegisterChunkListenerBuilder("loggingChunkListener", builders.ChunkListenerBuilder)
	jf.RegisterItemReadListenerBuilder("loggingItemReadListener", builders.ItemReadListenerBuilder)
	jf.RegisterItemProcessListenerBuilder("loggingItemProcessListener", builders.ItemProcessListenerBuilder)
	jf.RegisterItemWriteListenerBuilder("loggingItemWriteListener", builders.ItemWriteListenerBuilder)
	jf.RegisterSkipListenerBuilder("loggingSkipListener", builders.SkipListenerBuilder)
	jf.RegisterRetryItemListenerBuilder("loggingRetryItemListener", builders.RetryItemListenerBuilder)
	logger.Debugf("All logging listeners registered with JobFactory.")
}

// Module aggregates all listener components provided by this package.
var Module = fx.Options(
	// Job Listener
	fx.Provide(fx.Annotate(NewLoggingJobListenerBuilder, fx.ResultTags(`name:"loggingJobListener"`))),
	// Step Listener
	fx.Provide(fx.Annotate(NewLoggingStepListenerBuilder, fx.ResultTags(`name:"loggingStepListener"`))),
	// Chunk Listener
	fx.Provide(fx.Annotate(NewLoggingChunkListenerBuilder, fx.ResultTags(`name:"loggingChunkListener"`))),
	// Item Read Listener
	fx.Provide(fx.Annotate(NewLoggingItemReadListenerBuilder, fx.ResultTags(`name:"loggingItemReadListener"`))),
	// Item Process Listener
	fx.Provide(fx.Annotate(NewLoggingItemProcessListenerBuilder, fx.ResultTags(`name:"loggingItemProcessListener"`))),
	// Item Write Listener
	fx.Provide(fx.Annotate(NewLoggingItemWriteListenerBuilder, fx.ResultTags(`name:"loggingItemWriteListener"`))),
	// Skip Listener
	fx.Provide(fx.Annotate(NewLoggingSkipListenerBuilder, fx.ResultTags(`name:"loggingSkipListener"`))),
	// Retry Item Listener
	fx.Provide(fx.Annotate(NewLoggingRetryItemListenerBuilder, fx.ResultTags(`name:"loggingRetryItemListener"`))),
	
	// Register all builders
	fx.Invoke(RegisterAllListeners),
)
