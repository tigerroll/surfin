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

	"github.com/tigerroll/surfin/pkg/batch/adapter/webproxy"
	coreAdapter "github.com/tigerroll/surfin/pkg/batch/core/adapter"
)

// HourlyForecastAPIReaderConfig is a configuration struct specific to the Reader (for JSL property binding).
type HourlyForecastAPIReaderConfig struct {
	WebProxyRef string `yaml:"webProxyRef,omitempty"`
}

const (
	// ModuleHourlyForecastAPIReader is the module name used for batch errors originating from this reader.
	ModuleHourlyForecastAPIReader = "HourlyForecastAPIReader"
	// ReaderContextKey is the key used to store the reader's internal state within the Step's ExecutionContext.
	ReaderContextKey = "reader_context"
	// ForecastDataKey is the key used to store the fetched forecast data within the reader's state.
	ForecastDataKey = "forecastData"
	// CurrentIndexKey is the key used to store the current reading index within the reader's state.
	CurrentIndexKey = "currentIndex"
)

// HourlyForecastAPIReader implements the core.ItemReader interface to fetch hourly weather forecast data
// from an external API (e.g., Open-Meteo) and provide it item by item.
type HourlyForecastAPIReader struct {
	// config holds the reader's specific configuration, typically bound from JSL properties.
	config *HourlyForecastAPIReaderConfig
	// webProxyConn is the WebProxyConnection instance used to make API requests.
	webProxyConn *webproxy.WebProxyConnection
	// forecastData stores the entire fetched forecast data from the API.
	forecastData *weather_entity.OpenMeteoForecast
	// currentIndex tracks the current position in the forecastData for item-by-item reading.
	currentIndex int

	// stepExecutionContext holds the reference to the Step's ExecutionContext.
	stepExecutionContext model.ExecutionContext
	// readerState holds the reader's internal state.
	readerState model.ExecutionContext
	// resolver is used to resolve expressions within JSL properties.
	resolver core.ExpressionResolver
}

// NewHourlyForecastAPIReader creates a new instance of HourlyForecastAPIReader.
// It initializes the reader with application configuration, an expression resolver,
// and binds JSL properties to the reader's specific configuration.
//
// Parameters:
//
//	cfg: The application's global configuration.
//	resolver: An ExpressionResolver for dynamic property resolution.
//	resourceProviders: A map of resource providers for external connections (e.g., webproxy).
//	properties: A map of properties defined in the JSL for this reader.
//
// Returns:
//
//	A new HourlyForecastAPIReader instance or an error if configuration binding or validation fails.
func NewHourlyForecastAPIReader(
	cfg *config.Config,
	resolver core.ExpressionResolver,
	resourceProviders map[string]coreAdapter.ResourceProvider,
	properties map[string]interface{},
) (*HourlyForecastAPIReader, error) {
	// 1. Define configuration struct and set default values.
	hourlyForecastAPIReaderCfg := &HourlyForecastAPIReaderConfig{}

	// 2. Automatic binding of JSL properties.
	if err := configbinder.BindProperties(properties, hourlyForecastAPIReaderCfg); err != nil {
		return nil, exception.NewBatchError(ModuleHourlyForecastAPIReader, "Failed to bind properties", err, false, false)
	}

	var webProxyConn *webproxy.WebProxyConnection
	if hourlyForecastAPIReaderCfg.WebProxyRef != "" {
		provider, ok := resourceProviders["webproxy"]
		if !ok {
			return nil, exception.NewBatchError(ModuleHourlyForecastAPIReader, fmt.Sprintf("WebProxyProvider not found for ref '%s'", hourlyForecastAPIReaderCfg.WebProxyRef), nil, false, false)
		}
		conn, err := provider.GetConnection(hourlyForecastAPIReaderCfg.WebProxyRef)
		if err != nil {
			return nil, exception.NewBatchError(ModuleHourlyForecastAPIReader, fmt.Sprintf("Failed to get WebProxyConnection for ref '%s'", hourlyForecastAPIReaderCfg.WebProxyRef), err, false, false)
		}
		wpConn, ok := conn.(*webproxy.WebProxyConnection)
		if !ok {
			return nil, exception.NewBatchError(ModuleHourlyForecastAPIReader, fmt.Sprintf("Resolved connection for ref '%s' is not a WebProxyConnection", hourlyForecastAPIReaderCfg.WebProxyRef), nil, false, false)
		}
		webProxyConn = wpConn
	}

	return &HourlyForecastAPIReader{
		config:       hourlyForecastAPIReaderCfg,
		webProxyConn: webProxyConn,
		resolver:     resolver,
		// stepExecutionContext is set during Open.
		stepExecutionContext: model.NewExecutionContext(), // Initialized (will be overwritten later)
		readerState:          model.NewExecutionContext(), // Initialize EC for reader-specific state
	}, nil
}

// Open opens resources and restores state from ExecutionContext.
//
// Parameters:
//
//	ctx: The context for the operation.
//	ec: The ExecutionContext to restore state from.
//
// Returns:
//
//	An error if opening fails.
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
//
// Parameters:
//
//	ctx: The context for the operation.
//
// Returns:
//
//	The next item from the data source, or io.EOF if no more items are available.
//	An error if reading fails.
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
//
// Parameters:
//
//	ctx: The context for the operation.
//
// Returns:
//
//	An error if closing fails.
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
	return nil
}

// SetExecutionContext sets the ExecutionContext and restores the reader's state.
//
// Parameters:
//
//	ctx: The context for the operation.
//	ec: The ExecutionContext to set.
//
// Returns:
//
//	An error if setting the context or restoring state fails.
func (r *HourlyForecastAPIReader) SetExecutionContext(ctx context.Context, ec model.ExecutionContext) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	r.stepExecutionContext = ec
	return r.restoreReaderStateFromExecutionContext(ctx)
}

// GetExecutionContext retrieves the reader's ExecutionContext state.
//
// Parameters:
//
//	ctx: The context for the operation.
//
// Returns:
//
//	The reader's ExecutionContext state.
//	An error if retrieving or saving state fails.
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

	return r.readerState, nil
}

// fetchWeatherData makes an API call to Open-Meteo to retrieve hourly weather forecast data.
//
// Parameters:
//
//	ctx: The context for the operation.
//
// Returns:
//
//	An error if the API call or data processing fails.
func (r *HourlyForecastAPIReader) fetchWeatherData(ctx context.Context) error {
	logger.Infof("Fetching weather data from Open-Meteo API...")

	latitude := 35.6586
	longitude := 139.7454

	// Construct the URL using a relative path.
	url := fmt.Sprintf("/forecast?latitude=%f&longitude=%f&hourly=temperature_2m,weather_code&timezone=Asia/Tokyo&forecast_days=3",
		latitude, longitude)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return exception.NewBatchError(ModuleHourlyForecastAPIReader, "Failed to create API request", err, false, false)
	}

	var httpClient *http.Client
	if r.webProxyConn != nil {
		httpClient = r.webProxyConn.GetClient()
	} else {
		// Use a default HTTP client if no WebProxy is configured.
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}

	resp, err := httpClient.Do(req)
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

// restoreReaderStateFromExecutionContext restores the reader's internal state (currentIndex and forecastData)
// from the provided ExecutionContext.
//
// Parameters:
//
//	ctx: The context for the operation.
//
// Returns:
//
//	An error if state restoration fails.
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

// saveReaderStateToExecutionContext saves the reader's current internal state (currentIndex and forecastData)
// into the Step's ExecutionContext.
//
// Parameters:
//
//	ctx: The context for the operation.
//
// Returns:
//
//	An error if state saving fails.
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
