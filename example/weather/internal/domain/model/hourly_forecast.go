package model

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

// TableName specifies the table name for HourlyForecast.
func (HourlyForecast) TableName() string {
	return "hourly_forecast"
}
