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

// TableName specifies the table name for WeatherDataToStore.
func (WeatherDataToStore) TableName() string {
	return "hourly_forecast"
}
