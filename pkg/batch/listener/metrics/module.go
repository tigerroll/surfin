package metrics

import (
	"go.uber.org/fx"

	port "github.com/tigerroll/surfin/pkg/batch/core/application/port"
	config "github.com/tigerroll/surfin/pkg/batch/core/config"
	jsl "github.com/tigerroll/surfin/pkg/batch/core/config/jsl"
	support "github.com/tigerroll/surfin/pkg/batch/core/config/support"
	"github.com/tigerroll/surfin/pkg/batch/core/metrics"
	"github.com/tigerroll/surfin/pkg/batch/support/util/logger"
)

// NewMetricsJobListenerBuilder creates a ComponentBuilder for MetricsJobListener.
func NewMetricsJobListenerBuilder(recorder metrics.MetricRecorder) jsl.JobExecutionListenerBuilder {
	return func(
		_ *config.Config,
		_ map[string]string,
	) (port.JobExecutionListener, error) {
		// Assumed NewMetricsJobListener is defined in metrics_listeners.go
		return NewMetricsJobListener(recorder), nil
	}
}

// NewMetricsStepListenerBuilder creates a ComponentBuilder for MetricsStepListener.
func NewMetricsStepListenerBuilder(recorder metrics.MetricRecorder) jsl.StepExecutionListenerBuilder {
	return func(
		_ *config.Config,
		_ map[string]string,
	) (port.StepExecutionListener, error) {
		return NewMetricsStepListener(recorder), nil
	}
}

// NewMetricsChunkListenerBuilder creates a ComponentBuilder for MetricsChunkListener.
func NewMetricsChunkListenerBuilder(recorder metrics.MetricRecorder) jsl.ChunkListenerBuilder {
	return func(
		_ *config.Config,
		_ map[string]string,
	) (port.ChunkListener, error) {
		return NewMetricsChunkListener(recorder), nil
	}
}

// NewMetricsItemReadListenerBuilder creates a ComponentBuilder for MetricsItemReadListener.
func NewMetricsItemReadListenerBuilder(recorder metrics.MetricRecorder) jsl.ItemReadListenerBuilder {
	return func(
		_ *config.Config,
		_ map[string]string,
	) (port.ItemReadListener, error) {
		return NewMetricsItemReadListener(recorder), nil
	}
}

// NewMetricsItemProcessListenerBuilder creates a ComponentBuilder for MetricsItemProcessListener.
func NewMetricsItemProcessListenerBuilder(recorder metrics.MetricRecorder) jsl.ItemProcessListenerBuilder {
	return func(
		_ *config.Config,
		_ map[string]string,
	) (port.ItemProcessListener, error) {
		return NewMetricsItemProcessListener(recorder), nil
	}
}

// NewMetricsItemWriteListenerBuilder creates a ComponentBuilder for MetricsItemWriteListener.
func NewMetricsItemWriteListenerBuilder(recorder metrics.MetricRecorder) jsl.ItemWriteListenerBuilder {
	return func(
		_ *config.Config,
		_ map[string]string,
	) (port.ItemWriteListener, error) {
		return NewMetricsItemWriteListener(recorder), nil
	}
}

// NewMetricsSkipListenerBuilder creates a ComponentBuilder for MetricsSkipListener.
func NewMetricsSkipListenerBuilder(recorder metrics.MetricRecorder) jsl.SkipListenerBuilder {
	return func(
		_ *config.Config,
		_ map[string]string,
	) (port.SkipListener, error) {
		return NewMetricsSkipListener(recorder), nil
	}
}

// NewMetricsRetryItemListenerBuilder creates a ComponentBuilder for MetricsRetryItemListener.
func NewMetricsRetryItemListenerBuilder(recorder metrics.MetricRecorder) jsl.LoggingRetryItemListenerBuilder {
	return func(
		_ *config.Config,
		_ map[string]string,
	) (port.RetryItemListener, error) { // Corrected return type to port.RetryItemListener
		return NewMetricsRetryItemListener(recorder), nil
	}
}

// AllMetricsListenerBuilders is a struct to receive all metrics listener builders from Fx.
type AllMetricsListenerBuilders struct {
	fx.In
	JobListenerBuilder         jsl.JobExecutionListenerBuilder     `name:"metricsJobListener"`
	StepListenerBuilder        jsl.StepExecutionListenerBuilder    `name:"metricsStepListener"`
	ChunkListenerBuilder       jsl.ChunkListenerBuilder            `name:"metricsChunkListener"`
	ItemReadListenerBuilder    jsl.ItemReadListenerBuilder         `name:"metricsItemReadListener"`
	ItemProcessListenerBuilder jsl.ItemProcessListenerBuilder      `name:"metricsItemProcessListener"`
	ItemWriteListenerBuilder   jsl.ItemWriteListenerBuilder        `name:"metricsItemWriteListener"`
	SkipListenerBuilder        jsl.SkipListenerBuilder             `name:"metricsSkipListener"`
	RetryItemListenerBuilder   jsl.LoggingRetryItemListenerBuilder `name:"metricsRetryItemListener"`
}

// RegisterAllMetricsListeners registers all metrics listener builders with the JobFactory.
func RegisterAllMetricsListeners(jf *support.JobFactory, builders AllMetricsListenerBuilders) {
	jf.RegisterJobListenerBuilder("metricsJobListener", builders.JobListenerBuilder)
	jf.RegisterStepExecutionListenerBuilder("metricsStepListener", builders.StepListenerBuilder)
	jf.RegisterChunkListenerBuilder("metricsChunkListener", builders.ChunkListenerBuilder)
	jf.RegisterItemReadListenerBuilder("metricsItemReadListener", builders.ItemReadListenerBuilder)
	jf.RegisterItemProcessListenerBuilder("metricsItemProcessListener", builders.ItemProcessListenerBuilder)
	jf.RegisterItemWriteListenerBuilder("metricsItemWriteListener", builders.ItemWriteListenerBuilder)
	jf.RegisterSkipListenerBuilder("metricsSkipListener", builders.SkipListenerBuilder)
	jf.RegisterRetryItemListenerBuilder("metricsRetryItemListener", builders.RetryItemListenerBuilder)
	logger.Debugf("All metrics listeners registered with JobFactory.")
}

// Module aggregates all listener components provided by this package.
var Module = fx.Options(
	// PrometheusRecorder is provided by infrastructure/metrics, so it is decorated here to be asynchronous.
	// fx.Decorate replaces the existing MetricRecorder (PrometheusRecorder) with AsyncMetricRecorderWrapper.
	fx.Decorate(NewAsyncMetricRecorderWrapper),

	// Job Listener
	fx.Provide(fx.Annotate(NewMetricsJobListenerBuilder, fx.ResultTags(`name:"metricsJobListener"`))),
	// Step Listener
	fx.Provide(fx.Annotate(NewMetricsStepListenerBuilder, fx.ResultTags(`name:"metricsStepListener"`))),
	// Chunk Listener
	fx.Provide(fx.Annotate(NewMetricsChunkListenerBuilder, fx.ResultTags(`name:"metricsChunkListener"`))),
	// Item Read Listener
	fx.Provide(fx.Annotate(NewMetricsItemReadListenerBuilder, fx.ResultTags(`name:"metricsItemReadListener"`))),
	// Item Process Listener
	fx.Provide(fx.Annotate(NewMetricsItemProcessListenerBuilder, fx.ResultTags(`name:"metricsItemProcessListener"`))),
	// Item Write Listener
	fx.Provide(fx.Annotate(NewMetricsItemWriteListenerBuilder, fx.ResultTags(`name:"metricsItemWriteListener"`))),
	// Skip Listener
	fx.Provide(fx.Annotate(NewMetricsSkipListenerBuilder, fx.ResultTags(`name:"metricsSkipListener"`))),
	// Retry Item Listener
	fx.Provide(fx.Annotate(NewMetricsRetryItemListenerBuilder, fx.ResultTags(`name:"metricsRetryItemListener"`))),

	// Register all builders
	fx.Invoke(RegisterAllMetricsListeners),
)
