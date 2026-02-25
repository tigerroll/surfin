package weather_entity

import "time"

// Hourly represents the hourly weather forecast data retrieved from the Open Meteo API.
type Hourly struct {
	Time          []string  `json:"time"`
	WeatherCode   []int     `json:"weather_code"`
	Temperature2M []float64 `json:"temperature_2m"`
}

// OpenMeteoForecast represents the raw weather forecast data retrieved from the Open Meteo API.
type OpenMeteoForecast struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Hourly    Hourly  `json:"hourly"`
}

// WeatherDataToStore represents the weather forecast data to be stored in Redshift.
type WeatherDataToStore struct {
	Time          time.Time `gorm:"column:time;primaryKey"`
	WeatherCode   int       `gorm:"column:weather_code"`
	Temperature2M float64   `gorm:"column:temperature_2m"`
	Latitude      float64   `gorm:"column:latitude;primaryKey"`
	Longitude     float64   `gorm:"column:longitude;primaryKey"`
	CollectedAt   time.Time `gorm:"column:collected_at"`
}

// HourlyForecast represents the hourly weather data for export.
// It includes parquet tags for serialization to Parquet format.
// It also includes GORM tags to allow direct mapping from database queries.
type HourlyForecast struct {
	Time          int64   `gorm:"column:time;primaryKey" parquet:"name=time,type=INT64,convertedtype=TIMESTAMP_MILLIS"`
	WeatherCode   int32   `gorm:"column:weather_code" parquet:"name=weather_code,type=INT32"`
	Temperature2M float64 `gorm:"column:temperature_2m" parquet:"name=temperature_2m,type=DOUBLE"`
	Latitude      float64 `gorm:"column:latitude;primaryKey" parquet:"name=latitude,type=DOUBLE"`
	Longitude     float64 `gorm:"column:longitude;primaryKey" parquet:"name=longitude,type=DOUBLE"`
	CollectedAt   int64   `gorm:"column:collected_at" parquet:"name=collected_at,type=INT64,convertedtype=TIMESTAMP_MILLIS"`
}

// TableName specifies the table name for WeatherDataToStore.
func (WeatherDataToStore) TableName() string {
	return "hourly_forecast"
}
