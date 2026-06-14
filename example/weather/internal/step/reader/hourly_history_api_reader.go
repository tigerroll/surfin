// Package reader provides implementations of ItemReader for fetching weather data from external APIs.
package reader

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	weather_entity "github.com/tigerroll/surfin/example/weather/internal/domain/entity"
	"github.com/tigerroll/surfin/pkg/batch/adapter/webproxy"
	coreAdapter "github.com/tigerroll/surfin/pkg/batch/core/adapter"
	core "github.com/tigerroll/surfin/pkg/batch/core/application/port"
	config "github.com/tigerroll/surfin/pkg/batch/core/config"
	model "github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	"github.com/tigerroll/surfin/pkg/batch/support/util/configbinder"
	"github.com/tigerroll/surfin/pkg/batch/support/util/exception"
	logger "github.com/tigerroll/surfin/pkg/batch/support/util/logger"
)

// HourlyHistoryAPIReaderConfig holds configuration settings for the HourlyHistoryAPIReader.
type HourlyHistoryAPIReaderConfig struct {
	WebProxyRef string `yaml:"webProxyRef,omitempty"`
	Latitude    string `yaml:"latitude,omitempty"`
	Longitude   string `yaml:"longitude,omitempty"`
	Timezone    string `yaml:"timezone,omitempty"`
	StartDate   string `yaml:"startDate,omitempty"`
	EndDate     string `yaml:"endDate,omitempty"`
}

const (
	// ModuleHourlyHistoryAPIReader is the identifier used for batch errors originating from this reader.
	ModuleHourlyHistoryAPIReader = "HourlyHistoryAPIReader"
	// HistoryReaderContextKey is the key used to store the reader's internal state within the Step's ExecutionContext.
	HistoryReaderContextKey = "history_reader_context"
)

// HourlyHistoryAPIReader implements the core.ItemReader interface to fetch historical weather data
// from the Open-Meteo Archive API and provide it as a stream of records.
type HourlyHistoryAPIReader struct {
	config       *HourlyHistoryAPIReaderConfig
	webProxyConn *webproxy.WebProxyConnection
	csvReader    *csv.Reader
	respBody     io.ReadCloser
	resolver     core.ExpressionResolver

	stepExecutionContext model.ExecutionContext
	readerState          model.ExecutionContext
}

// NewHourlyHistoryAPIReader creates a new instance of HourlyHistoryAPIReader.
// It binds JSL properties to the reader's configuration and resolves the required WebProxy connection.
func NewHourlyHistoryAPIReader(
	cfg *config.Config,
	resolver core.ExpressionResolver,
	resourceProviders map[string]coreAdapter.ResourceProvider,
	properties map[string]interface{},
) (*HourlyHistoryAPIReader, error) {
	readerCfg := &HourlyHistoryAPIReaderConfig{}
	if err := configbinder.BindProperties(properties, readerCfg); err != nil {
		return nil, exception.NewBatchError(ModuleHourlyHistoryAPIReader, "Failed to bind properties", err, false, false)
	}

	var webProxyConn *webproxy.WebProxyConnection
	if readerCfg.WebProxyRef != "" {
		provider, ok := resourceProviders["webproxy"]
		if !ok {
			return nil, exception.NewBatchError(ModuleHourlyHistoryAPIReader, fmt.Sprintf("WebProxyProvider not found for ref '%s'", readerCfg.WebProxyRef), nil, false, false)
		}
		conn, err := provider.GetConnection(readerCfg.WebProxyRef)
		if err != nil {
			return nil, exception.NewBatchError(ModuleHourlyHistoryAPIReader, fmt.Sprintf("Failed to get WebProxyConnection for ref '%s'", readerCfg.WebProxyRef), err, false, false)
		}
		wpConn, ok := conn.(*webproxy.WebProxyConnection)
		if !ok {
			return nil, exception.NewBatchError(ModuleHourlyHistoryAPIReader, fmt.Sprintf("Resolved connection for ref '%s' is not a WebProxyConnection", readerCfg.WebProxyRef), nil, false, false)
		}
		webProxyConn = wpConn
	}

	return &HourlyHistoryAPIReader{
		config:       readerCfg,
		webProxyConn: webProxyConn,
		resolver:     resolver,
		readerState:  model.NewExecutionContext(),
	}, nil
}

// Open initializes the reader, establishes the API connection, and prepares the CSV stream.
func (r *HourlyHistoryAPIReader) Open(ctx context.Context, ec model.ExecutionContext) error {
	logger.Debugf("HourlyHistoryAPIReader.Open called.")
	r.stepExecutionContext = ec

	// Fetch data from API
	return r.fetchHourlyHistoryData(ctx)
}

// Read retrieves the next historical weather record from the CSV stream.
// Returns io.EOF when the stream is exhausted.
func (r *HourlyHistoryAPIReader) Read(ctx context.Context) (any, error) {
	record, err := r.csvReader.Read()
	if err == io.EOF {
		return nil, io.EOF
	}
	if err != nil {
		return nil, exception.NewBatchError(ModuleHourlyHistoryAPIReader, "Failed to read CSV record", err, false, false)
	}

	// CSV format: time,temperature_2m,weather_code
	parsedTime, err := time.Parse("2006-01-02T15:04", record[0])
	if err != nil {
		return nil, exception.NewBatchError(ModuleHourlyHistoryAPIReader, "Failed to parse time", err, false, false)
	}

	temp, err := strconv.ParseFloat(record[1], 64)
	if err != nil {
		return nil, exception.NewBatchError(ModuleHourlyHistoryAPIReader, "Failed to parse temperature", err, false, false)
	}

	code, err := strconv.Atoi(record[2])
	if err != nil {
		return nil, exception.NewBatchError(ModuleHourlyHistoryAPIReader, "Failed to parse weather code", err, false, false)
	}

	lat, _ := strconv.ParseFloat(r.config.Latitude, 64)
	lon, _ := strconv.ParseFloat(r.config.Longitude, 64)

	return &weather_entity.HourlyHistoryToStore{
		Time:          parsedTime,
		WeatherCode:   code,
		Temperature2M: temp,
		Latitude:      lat,
		Longitude:     lon,
		CollectedAt:   time.Now(),
	}, nil
}

// Close releases the API response body and associated resources.
func (r *HourlyHistoryAPIReader) Close(ctx context.Context) error {
	if r.respBody != nil {
		return r.respBody.Close()
	}
	return nil
}

// SetExecutionContext sets the step's execution context.
func (r *HourlyHistoryAPIReader) SetExecutionContext(ctx context.Context, ec model.ExecutionContext) error {
	r.stepExecutionContext = ec
	return nil
}

// GetExecutionContext returns the reader's current execution context.
func (r *HourlyHistoryAPIReader) GetExecutionContext(ctx context.Context) (model.ExecutionContext, error) {
	return r.readerState, nil
}

// fetchHourlyHistoryData performs the API request to Open-Meteo and initializes the CSV reader.
func (r *HourlyHistoryAPIReader) fetchHourlyHistoryData(ctx context.Context) error {
	logger.Infof("Fetching hourly history weather data...")

	lat := "52.52"
	lon := "13.41"
	if r.config.Latitude != "" {
		lat = r.config.Latitude
	}
	if r.config.Longitude != "" {
		lon = r.config.Longitude
	}

	timezone := r.config.Timezone
	if timezone == "" {
		timezone = "UTC"
	}

	endDate := r.config.EndDate
	if endDate == "" {
		endDate = time.Now().AddDate(0, 0, -3).Format("2006-01-02")
	}
	startDate := r.config.StartDate
	if startDate == "" {
		startDate = time.Now().AddDate(0, 0, -10).Format("2006-01-02")
	}

	url := fmt.Sprintf("https://archive-api.open-meteo.com/v1/archive?latitude=%s&longitude=%s&start_date=%s&end_date=%s&hourly=temperature_2m,weather_code&timezone=%s&format=csv",
		lat, lon, startDate, endDate, timezone)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return exception.NewBatchError(ModuleHourlyHistoryAPIReader, "Failed to create API request", err, false, false)
	}

	httpClient := &http.Client{Timeout: 10 * time.Second}
	if r.webProxyConn != nil {
		httpClient = r.webProxyConn.GetClient()
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return exception.NewBatchError(ModuleHourlyHistoryAPIReader, "API call failed", err, true, false)
	}

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return exception.NewBatchError(ModuleHourlyHistoryAPIReader, fmt.Sprintf("API error: %d", resp.StatusCode), errors.New(string(bodyBytes)), true, false)
	}

	r.respBody = resp.Body
	r.csvReader = csv.NewReader(r.respBody)
	r.csvReader.FieldsPerRecord = -1 // Allow variable field counts for header/data rows

	// Skip header lines (Open-Meteo CSV has 3 header lines)
	for i := 0; i < 3; i++ {
		if _, err := r.csvReader.Read(); err != nil {
			return exception.NewBatchError(ModuleHourlyHistoryAPIReader, "Failed to skip CSV header", err, false, false)
		}
	}

	return nil
}

var _ core.ItemReader[any] = (*HourlyHistoryAPIReader)(nil)
