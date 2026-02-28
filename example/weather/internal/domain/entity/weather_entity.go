package weather_entity

import (
	"database/sql/driver"
	"fmt"
	"time"
)

// Package weather_entity defines the data structures (entities) used within the weather application's domain.

// UnixMillis is a custom type for storing Unix timestamps in milliseconds.
// It implements the sql.Scanner and driver.Valuer interfaces to handle conversions
// between time.Time and int64 for database operations.
type UnixMillis int64

// Scan implements the sql.Scanner interface.
// It converts a value read from the database into a UnixMillis type.
func (um *UnixMillis) Scan(value interface{}) error {
	if value == nil {
		*um = 0
		return nil
	}
	switch v := value.(type) {
	case time.Time:
		*um = UnixMillis(v.UnixMilli())
		return nil
	case int64:
		*um = UnixMillis(v)
		return nil
	default:
		return fmt.Errorf("cannot scan type %T into UnixMillis", v)
	}
}

// Value implements the driver.Valuer interface.
// It converts a UnixMillis value into a driver.Value for writing to the database.
func (um UnixMillis) Value() (driver.Value, error) {
	if um == 0 {
		return nil, nil
	}
	return time.UnixMilli(int64(um)), nil
}

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
	Time          UnixMillis `gorm:"column:time;primaryKey" parquet:"name=time,type=INT64,convertedtype=TIMESTAMP_MILLIS"`
	WeatherCode   int32      `gorm:"column:weather_code" parquet:"name=weather_code,type=INT32"`
	Temperature2M float64    `gorm:"column:temperature_2m" parquet:"name=temperature_2m,type=DOUBLE"`
	Latitude      float64    `gorm:"column:latitude;primaryKey" parquet:"name=latitude,type=DOUBLE"`
	Longitude     float64    `gorm:"column:longitude;primaryKey" parquet:"name=longitude,type=DOUBLE"`
	CollectedAt   UnixMillis `gorm:"column:collected_at" parquet:"name=collected_at,type=INT64,convertedtype=TIMESTAMP_MILLIS"`
}

// TableName specifies the table name for WeatherDataToStore.
func (WeatherDataToStore) TableName() string {
	return "hourly_forecast"
}
