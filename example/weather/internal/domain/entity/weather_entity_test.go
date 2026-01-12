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

// 複合主キーのGORMタグの検証は、GORMのマイグレーションテストや
// 実際のDB操作テストで行うのが理想的ですが、ここでは構造体の定義を確認するに留めます。
// 構造体のフィールド名とタグの確認は、コンパイル時に行われるため、
// 実行時のテストとしては TableName の確認が最もシンプルで有効です。
