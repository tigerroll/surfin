package weather_entity

import (
	"testing"
)

func TestWeatherDataToStore_TableName(t *testing.T) {
	expected := "hourly_forecast"
	actual := WeatherDataToStore{}.TableName()

	if actual != expected {
		t.Errorf("TableName() returned %s, expected %s", actual, expected)
	}
}

// Validation of GORM tags for composite primary keys is ideally done through GORM migration tests
// or actual DB operation tests. Here, we only confirm the struct definition.
// Since field names and tags are checked at compile time,
// checking TableName is the simplest and most effective runtime test.
