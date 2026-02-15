package reader

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	core "github.com/tigerroll/surfin/pkg/batch/core/application/port"
	config "github.com/tigerroll/surfin/pkg/batch/core/config"
	model "github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	"github.com/tigerroll/surfin/pkg/batch/support/util/exception"
	logger "github.com/tigerroll/surfin/pkg/batch/support/util/logger"

	weather_entity "github.com/tigerroll/surfin/example/weather/internal/domain/entity"

	configbinder "github.com/tigerroll/surfin/pkg/batch/support/util/configbinder"
)

// HourlyForecastAPIReaderConfig is a configuration struct specific to the Reader (for JSL property binding).
type HourlyForecastAPIReaderConfig struct {
	APIEndpoint string `yaml:"apiEndpoint"`
	APIKey      string `yaml:"apiKey"`
}

const (
	ModuleHourlyForecastAPIReader = "HourlyForecastAPIReader"
	ReaderContextKey              = "reader_context"
	ForecastDataKey               = "forecastData"
	CurrentIndexKey               = "currentIndex"
)

type HourlyForecastAPIReader struct {
	config       *HourlyForecastAPIReaderConfig
	client       *http.Client
	forecastData *weather_entity.OpenMeteoForecast
	currentIndex int

	// stepExecutionContext holds the reference to the Step's ExecutionContext.
	stepExecutionContext model.ExecutionContext
	// readerState holds the reader's internal state.
	readerState model.ExecutionContext
	resolver    core.ExpressionResolver
}

func NewHourlyForecastAPIReader(
	cfg *config.Config,
	resolver core.ExpressionResolver,
	properties map[string]string,
) (*HourlyForecastAPIReader, error) {
	// 1. Define configuration struct and set default values.
	hourlyForecastAPIReaderCfg := &HourlyForecastAPIReaderConfig{
		APIEndpoint: cfg.Surfin.Batch.APIEndpoint,
		APIKey:      cfg.Surfin.Batch.APIKey,
	}

	// 2. Automatic binding of JSL properties.
	if err := configbinder.BindProperties(properties, hourlyForecastAPIReaderCfg); err != nil {
		return nil, exception.NewBatchError(ModuleHourlyForecastAPIReader, "Failed to bind properties", err, false, false)
	}

	// 3. Validation
	if hourlyForecastAPIReaderCfg.APIEndpoint == "" {
		return nil, fmt.Errorf("HourlyForecastAPIReaderConfig.APIEndpoint is not configured")
	}

	return &HourlyForecastAPIReader{
		config:   hourlyForecastAPIReaderCfg,
		client:   &http.Client{Timeout: 10 * time.Second},
		resolver: resolver,
		// stepExecutionContext is set during Open.
		stepExecutionContext: model.NewExecutionContext(), // Initialized (will be overwritten later)
		readerState:          model.NewExecutionContext(), // Initialize EC for reader-specific state
	}, nil
}

// Open opens resources and restores state from ExecutionContext.
func (r *HourlyForecastAPIReader) Open(ctx context.Context, ec model.ExecutionContext) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	logger.Debugf("HourlyForecastAPIReader.Open is called.")
	// Set stepExecutionContext and restore internal state from EC.
	r.stepExecutionContext = ec
	if err := r.restoreReaderStateFromExecutionContext(ctx); err != nil {
		return err
	}

	// Fetch data from API if forecastData is empty (initial run or restart)
	if r.forecastData == nil || len(r.forecastData.Hourly.Time) == 0 {
		return r.fetchWeatherData(ctx)
	}
	return nil
}

// Read reads the next item from the data source.
func (r *HourlyForecastAPIReader) Read(ctx context.Context) (any, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// If forecastData is nil or empty (e.g., initial state or after reading all data), return EOF.
	// Expect new data to be fetched in Open or the next step cycle.
	if r.forecastData == nil || r.currentIndex >= len(r.forecastData.Hourly.Time) {
		logger.Debugf("HourlyForecastAPIReader: Finished reading all weather data. Returning EOF.")
		// Reset state after reading all data.
		r.forecastData = nil
		r.currentIndex = 0
		return nil, io.EOF
	}

	// The return type of Read method is unified to *weather_entity.OpenMeteoForecast pointer.
	// Processor is expected to receive this pointer.
	itemToProcess := &weather_entity.OpenMeteoForecast{
		Latitude:  r.forecastData.Latitude,
		Longitude: r.forecastData.Longitude,
		Hourly: weather_entity.Hourly{
			Time:          []string{r.forecastData.Hourly.Time[r.currentIndex]},
			WeatherCode:   []int{r.forecastData.Hourly.WeatherCode[r.currentIndex]},
			Temperature2M: []float64{r.forecastData.Hourly.Temperature2M[r.currentIndex]},
		},
	}

	r.currentIndex++
	return itemToProcess, nil
}

// Close releases resources.
func (r *HourlyForecastAPIReader) Close(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	logger.Debugf("HourlyForecastAPIReader.Close is called.")
	// Save internal state to the Step's ExecutionContext.
	if err := r.saveReaderStateToExecutionContext(ctx); err != nil {
		logger.Errorf("HourlyForecastAPIReader.Close: Failed to save internal state: %v", err)
	}
	// Retaining error return value to match ItemReader interface signature.
	return nil
}

// SetExecutionContext sets the ExecutionContext and restores the reader's state.
func (r *HourlyForecastAPIReader) SetExecutionContext(ctx context.Context, ec model.ExecutionContext) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	r.stepExecutionContext = ec                          // Set the Step's ExecutionContext
	return r.restoreReaderStateFromExecutionContext(ctx) // Restore reader state from EC
}

// GetExecutionContext retrieves the reader's ExecutionContext state.
func (r *HourlyForecastAPIReader) GetExecutionContext(ctx context.Context) (model.ExecutionContext, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	logger.Debugf("HourlyForecastAPIReader.GetExecutionContext is called.")

	// Save internal state to "reader_context" in the Step ExecutionContext.
	if err := r.saveReaderStateToExecutionContext(ctx); err != nil {
		return nil, err
	}

	return r.readerState, nil // Return the reader's own state.
}

func (r *HourlyForecastAPIReader) fetchWeatherData(ctx context.Context) error {
	logger.Infof("Fetching weather data from Open-Meteo API...")

	latitude := 35.6586
	longitude := 139.7454

	url := fmt.Sprintf("%s/forecast?latitude=%f&longitude=%f&hourly=temperature_2m,weather_code&timezone=Asia/Tokyo&forecast_days=3",
		r.config.APIEndpoint, latitude, longitude)
	if r.config.APIKey != "" {
		url = fmt.Sprintf("%s&apikey=%s", url, r.config.APIKey)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return exception.NewBatchError(ModuleHourlyForecastAPIReader, "Failed to create API request", err, false, false)
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return exception.NewBatchError(ModuleHourlyForecastAPIReader, "API call failed", err, true, false)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		bodyString := strings.TrimSpace(string(bodyBytes))
		errMsg := fmt.Sprintf("Error response from API: Status code %d, Body: %s", resp.StatusCode, bodyString)
		isRetryable := resp.StatusCode >= 500
		return exception.NewBatchError(ModuleHourlyForecastAPIReader, errMsg, errors.New(bodyString), isRetryable, false)
	}

	var forecast weather_entity.OpenMeteoForecast
	if err := json.NewDecoder(resp.Body).Decode(&forecast); err != nil {
		return exception.NewBatchError(ModuleHourlyForecastAPIReader, "Failed to decode API response", err, false, false)
	}

	r.forecastData = &forecast
	r.currentIndex = 0
	logger.Debugf("Successfully fetched %d hourly weather records from API.", len(r.forecastData.Hourly.Time))

	return nil
}

func (r *HourlyForecastAPIReader) restoreReaderStateFromExecutionContext(ctx context.Context) error {
	// Extract reader-specific context from stepExecutionContext
	readerCtxVal, ok := r.stepExecutionContext.Get(ReaderContextKey)
	var readerCtx model.ExecutionContext
	if !ok || readerCtxVal == nil {
		readerCtx = model.NewExecutionContext()
		// New readerCtx is set in the Step EC.
		r.stepExecutionContext.Put(ReaderContextKey, readerCtx)
		logger.Debugf("HourlyForecastAPIReader: Initialized new ReaderExecutionContext.")
	} else if rcv, isEC := readerCtxVal.(model.ExecutionContext); isEC {
		readerCtx = rcv
	} else {
		logger.Warnf("HourlyForecastAPIReader: ExecutionContext ReaderContextKey has unexpected type (%T). Initializing new ExecutionContext.", readerCtxVal)
		readerCtx = model.NewExecutionContext()
		r.stepExecutionContext.Put(ReaderContextKey, readerCtx)
	}
	// Restore reader internal state from readerCtx.
	r.readerState = readerCtx.Copy()
	if idx, ok := r.readerState.GetInt(CurrentIndexKey); ok {
		r.currentIndex = idx
		logger.Debugf("HourlyForecastAPIReader: Restored currentIndex %d from ExecutionContext.", r.currentIndex)
	} else if val, found := r.readerState.Get(CurrentIndexKey); found {
		// If GetInt failed but the key exists (type is not int/float64)
		logger.Warnf("HourlyForecastAPIReader: ExecutionContext currentIndex has unexpected type (%T). Initializing to 0.", val)
		r.currentIndex = 0
	} else {
		r.currentIndex = 0
		logger.Debugf("HourlyForecastAPIReader: Initialized currentIndex to 0.")
	}

	if val, found := r.readerState.GetString(ForecastDataKey); found && val != "" {
		r.forecastData = &weather_entity.OpenMeteoForecast{}
		if err := json.Unmarshal([]byte(val), r.forecastData); err != nil {
			return exception.NewBatchError(ModuleHourlyForecastAPIReader, "Failed to deserialize forecastData JSON", err, false, false)
		}
		logger.Debugf("HourlyForecastAPIReader: Restored forecastData from ExecutionContext. Data count: %d", len(r.forecastData.Hourly.Time))
	} else {
		r.forecastData = nil
		logger.Debugf("HourlyForecastAPIReader: forecastData initialized to nil.")
	}
	return nil
}

func (r *HourlyForecastAPIReader) saveReaderStateToExecutionContext(ctx context.Context) error {
	// Extract reader-specific context from stepExecutionContext (created if not present)
	readerCtxVal, ok := r.stepExecutionContext.Get(ReaderContextKey)
	var readerCtx model.ExecutionContext
	if !ok || readerCtxVal == nil {
		readerCtx = model.NewExecutionContext()
		r.stepExecutionContext.Put(ReaderContextKey, readerCtx)
	} else if rcv, isEC := readerCtxVal.(model.ExecutionContext); isEC {
		readerCtx = rcv
	} else {
		logger.Warnf("HourlyForecastAPIReader: ExecutionContext ReaderContextKey has unexpected type (%T). Initializing new ExecutionContext.", readerCtxVal)
		readerCtx = model.NewExecutionContext()
		r.stepExecutionContext.Put(ReaderContextKey, readerCtx)
	}

	// Save reader internal state to readerCtx AND r.readerState.
	readerCtx.Put(CurrentIndexKey, r.currentIndex)
	r.readerState.Put(CurrentIndexKey, r.currentIndex)

	if r.forecastData != nil {
		forecastJSON, err := json.Marshal(r.forecastData)
		if err != nil {
			logger.Errorf("%s: Failed to encode forecastData: %v", ModuleHourlyForecastAPIReader, err)
			return exception.NewBatchError(ModuleHourlyForecastAPIReader, "Failed to encode forecastData", err, false, false)
		}
		readerCtx.Put(ForecastDataKey, string(forecastJSON))
		r.readerState.Put(ForecastDataKey, string(forecastJSON))
	} else {
		readerCtx.Remove(ForecastDataKey)
		r.readerState.Remove(ForecastDataKey)
	}
	logger.Debugf("HourlyForecastAPIReader: Saved currentIndex (%d) and forecastData state to ExecutionContext.", r.currentIndex)
	return nil
}

var _ core.ItemReader[any] = (*HourlyForecastAPIReader)(nil)
