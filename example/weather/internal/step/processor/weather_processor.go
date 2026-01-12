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

var tokyoLocation *time.Location

func init() {
	var err error
	tokyoLocation, err = time.LoadLocation("Asia/Tokyo")
	if err != nil {
		tokyoLocation = time.UTC
		logger.Warnf("Failed to load timezone 'Asia/Tokyo'. Falling back to UTC: %v", err)
	}
}

// WeatherProcessorConfig is a configuration struct specific to the Processor (for JSL property binding).
// It is currently empty but may be extended in the future.
type WeatherProcessorConfig struct {
	DummySetting string `yaml:"dummySetting,omitempty"`
}

type WeatherProcessor struct {
	config           *config.Config
	resolver         core.ExpressionResolver
	executionContext model.ExecutionContext
	properties       map[string]string
	processorConfig  *WeatherProcessorConfig
}

// NewWeatherProcessor simplifies the signature by removing DB-related dependencies.
func NewWeatherProcessor(
	cfg *config.Config,
	resolver core.ExpressionResolver,
	properties map[string]string,
) (*WeatherProcessor, error) {

	processorCfg := &WeatherProcessorConfig{}

	// Automatic binding of JSL properties
	if err := configbinder.BindProperties(properties, processorCfg); err != nil {
		return nil, exception.NewBatchError("weather_processor", "Failed to bind properties", err, false, false)
	}

	return &WeatherProcessor{
		config:           cfg,
		resolver:         resolver,
		executionContext: model.NewExecutionContext(),
		properties:       properties,
		processorConfig:  processorCfg,
	}, nil
}

func (p *WeatherProcessor) Process(ctx context.Context, item any) (any, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	forecast, ok := item.(*weather_entity.OpenMeteoForecast)
	if !ok {
		return nil, exception.NewBatchError("weather_processor", fmt.Sprintf("unexpected input item type: %T", item), nil, false, true)
	}

	collectedAt := time.Now().In(tokyoLocation)

	// The Reader returns OpenMeteoForecast containing only 1 hour of data, so only index 0 is processed.
	if len(forecast.Hourly.Time) == 0 {
		logger.Warnf("WeatherProcessor: No hourly data found to process. Filtering item.")
		return nil, nil // Filtering (returning nil means the item is filtered out)
	}

	parsedTime, err := time.ParseInLocation("2006-01-02T15:04", forecast.Hourly.Time[0], tokyoLocation) // Custom layout matching API response format
	if err != nil {
		logger.Warnf("WeatherProcessor: Failed to parse time string '%s': %v", forecast.Hourly.Time[0], err)
		return nil, exception.NewBatchError("weather_processor", fmt.Sprintf("failed to parse time: %s", forecast.Hourly.Time[0]), err, false, true)
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

func (p *WeatherProcessor) SetExecutionContext(ctx context.Context, ec model.ExecutionContext) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	p.executionContext = ec
	logger.Debugf("WeatherProcessor.SetExecutionContext is called.")
	return nil
}

func (p *WeatherProcessor) GetExecutionContext(ctx context.Context) (model.ExecutionContext, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	logger.Debugf("WeatherProcessor.GetExecutionContext is called.")
	return p.executionContext, nil
}

var _ core.ItemProcessor[any, any] = (*WeatherProcessor)(nil)
