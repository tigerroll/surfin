package weather_entity

import (
	"time"
)

// HourlyHistoryToStore represents the historical weather data to be stored.
type HourlyHistoryToStore struct {
	Time          time.Time `gorm:"column:time;primaryKey" parquet:"time,type=INT64,convertedtype=TIMESTAMP_MILLIS"`
	WeatherCode   int       `gorm:"column:weather_code" parquet:"weather_code,type=INT32"`
	Temperature2M float64   `gorm:"column:temperature_2m" parquet:"temperature_2m,type=DOUBLE"`
	Latitude      float64   `gorm:"column:latitude;primaryKey" parquet:"latitude,type=DOUBLE"`
	Longitude     float64   `gorm:"column:longitude;primaryKey" parquet:"longitude,type=DOUBLE"`
	CollectedAt   time.Time `gorm:"column:collected_at" parquet:"collected_at,type=INT64,convertedtype=TIMESTAMP_MILLIS"`
}

// TableName specifies the table name for HourlyHistoryToStore.
func (HourlyHistoryToStore) TableName() string {
	return "hourly_history"
}
