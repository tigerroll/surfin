package processor

import (
	"context"
	"fmt"
	"time"

	core "github.com/tigerroll/surfin/pkg/batch/core/application/port"
	config "github.com/tigerroll/surfin/pkg/batch/core/config"
	model "github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	configbinder "github.com/tigerroll/surfin/pkg/batch/support/util/configbinder"
	"github.com/tigerroll/surfin/pkg/batch/support/util/exception"
	"github.com/tigerroll/surfin/pkg/batch/support/util/logger"

	weather_entity "github.com/tigerroll/surfin/example/weather/internal/domain/entity"
)

// tokyoLocation stores the time.Location for Asia/Tokyo.
// It is initialized in the init() function.
var tokyoLocation *time.Location

func init() {
	var err error
	tokyoLocation, err = time.LoadLocation("Asia/Tokyo")
	if err != nil {
		tokyoLocation = time.UTC
		logger.Warnf("Failed to load timezone 'Asia/Tokyo'. Falling back to UTC: %v", err)
	}
}

// HourlyForecastTransformProcessorConfig is a configuration struct specific to the Processor (for JSL property binding).
// It is currently empty but may be extended in the future.
type HourlyForecastTransformProcessorConfig struct {
	DummySetting string `yaml:"dummySetting,omitempty"`
}

// HourlyForecastTransformProcessor transforms OpenMeteo forecast data into a storable format.
type HourlyForecastTransformProcessor struct {
	config           *config.Config
	resolver         core.ExpressionResolver
	executionContext model.ExecutionContext
	properties       map[string]interface{}
	processorConfig  *HourlyForecastTransformProcessorConfig
}

// NewHourlyForecastTransformProcessor creates a new instance of HourlyForecastTransformProcessor.
// It binds JSL properties to the processor's configuration.
//
// Parameters:
//
//	cfg: The application configuration.
//	resolver: The expression resolver for dynamic property resolution.
//	properties: A map of properties defined in the JSL for this processor.
//
// Returns:
//
//	A new HourlyForecastTransformProcessor instance or an error if configuration binding fails.
func NewHourlyForecastTransformProcessor(
	cfg *config.Config,
	resolver core.ExpressionResolver,
	properties map[string]interface{},
) (*HourlyForecastTransformProcessor, error) {

	processorCfg := &HourlyForecastTransformProcessorConfig{}

	if err := configbinder.BindProperties(properties, processorCfg); err != nil {
		return nil, exception.NewBatchError("hourly_forecast_transform_processor", "Failed to bind properties", err, false, false)
	}

	return &HourlyForecastTransformProcessor{
		config:           cfg,
		resolver:         resolver,
		executionContext: model.NewExecutionContext(),
		properties:       properties,
		processorConfig:  processorCfg,
	}, nil
}

// Process transforms an OpenMeteoForecast item into a WeatherDataToStore item.
// It extracts relevant hourly forecast data and converts it into a format suitable for storage.
//
// Parameters:
//
//	ctx: The context for the operation.
//	item: The input item, expected to be of type *weather_entity.OpenMeteoForecast.
//
// Returns:
//
//	The transformed item (*weather_entity.WeatherDataToStore) or nil if the item is filtered out,
//	or an error if the input type is unexpected or parsing fails.
func (p *HourlyForecastTransformProcessor) Process(ctx context.Context, item any) (any, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	forecast, ok := item.(*weather_entity.OpenMeteoForecast)
	if !ok {
		return nil, exception.NewBatchError("hourly_forecast_transform_processor", fmt.Sprintf("unexpected input item type: %T", item), nil, false, true)
	}

	collectedAt := time.Now().In(tokyoLocation)

	if len(forecast.Hourly.Time) == 0 {
		logger.Warnf("HourlyForecastTransformProcessor: No hourly data found to process. Filtering item.")
		return nil, nil // Returning nil filters out the item.
	}

	parsedTime, err := time.ParseInLocation("2006-01-02T15:04", forecast.Hourly.Time[0], tokyoLocation)
	if err != nil {
		logger.Warnf("HourlyForecastTransformProcessor: Failed to parse time string '%s': %v", forecast.Hourly.Time[0], err)
		return nil, exception.NewBatchError("hourly_forecast_transform_processor", fmt.Sprintf("failed to parse time: %s", forecast.Hourly.Time[0]), err, false, true)
	}

	dataToStore := &weather_entity.WeatherDataToStore{
		Time:          parsedTime,
		WeatherCode:   forecast.Hourly.WeatherCode[0],
		Temperature2M: forecast.Hourly.Temperature2M[0],
		Latitude:      forecast.Latitude,
		Longitude:     forecast.Longitude,
		CollectedAt:   collectedAt,
	}

	return dataToStore, nil // Returns a single *WeatherDataToStore
}

// SetExecutionContext sets the execution context for the processor.
//
// Parameters:
//
//	ctx: The context for the operation.
//	ec: The ExecutionContext to be set.
//
// Returns:
//
//	An error if the context is cancelled.
func (p *HourlyForecastTransformProcessor) SetExecutionContext(ctx context.Context, ec model.ExecutionContext) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	p.executionContext = ec
	return nil
}

// GetExecutionContext retrieves the current execution context of the processor.
//
// Parameters:
//
//	ctx: The context for the operation.
//
// Returns:
//
//	The current ExecutionContext or an error if the context is cancelled.
func (p *HourlyForecastTransformProcessor) GetExecutionContext(ctx context.Context) (model.ExecutionContext, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	return p.executionContext, nil
}

var _ core.ItemProcessor[any, any] = (*HourlyForecastTransformProcessor)(nil)
