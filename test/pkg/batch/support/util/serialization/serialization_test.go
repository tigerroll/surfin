package serialization_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	config "surfin/pkg/batch/core/config"
	"surfin/pkg/batch/support/util/serialization"
)

// Setup: テスト用のグローバル設定を一時的に設定/リセットするヘルパー
func setupMaskingConfig(keys []string) func() {
	originalConfig := config.GlobalConfig
	cfg := config.NewConfig()
	cfg.Surfin.Security.MaskedParameterKeys = keys
	config.GlobalConfig = cfg

	return func() {
		config.GlobalConfig = originalConfig
	}
}

func TestGetMaskedJobParametersMap(t *testing.T) {
	defer setupMaskingConfig([]string{"password", "api_key"})()

	params := map[string]interface{}{
		"user":     "alice",
		"password": "secret_password",
		"api_key":  "xyz123",
		"count":    10,
	}

	masked := serialization.GetMaskedJobParametersMap(params)

	if masked["user"] != "alice" {
		t.Errorf("Unmasked key 'user' was incorrectly masked")
	}
	if masked["count"] != 10 {
		t.Errorf("Unmasked key 'count' was incorrectly masked")
	}
	if masked["password"] != "********" {
		t.Errorf("Masked key 'password' was not masked correctly, got %v", masked["password"])
	}
	if masked["api_key"] != "********" {
		t.Errorf("Masked key 'api_key' was not masked correctly, got %v", masked["api_key"])
	}
	if len(masked) != 4 {
		t.Errorf("Map size changed unexpectedly: %d", len(masked))
	}
	
	// nil input test
	maskedNil := serialization.GetMaskedJobParametersMap(nil)
	if len(maskedNil) != 0 {
		t.Errorf("Expected empty map for nil input, got %v", maskedNil)
	}
}

func TestMarshalJobParameters_Masking(t *testing.T) {
	defer setupMaskingConfig([]string{"secret"})()

	params := map[string]interface{}{
		"data":   "public",
		"secret": "hidden_value",
	}

	data, err := serialization.MarshalJobParameters(params)
	if err != nil {
		t.Fatalf("MarshalJobParameters failed: %v", err)
	}

	// マスキングされたJSONが出力されていることを確認
	if !strings.Contains(string(data), `"secret":"********"`) {
		t.Errorf("Marshaled output did not contain masked value: %s", string(data))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Unmarshal masked JSON failed: %v", err)
	}

	if result["data"] != "public" {
		t.Errorf("Data key incorrect: %v", result["data"])
	}
	if result["secret"] != "********" {
		t.Errorf("Secret key was not masked in marshaled output: %v", result["secret"])
	}
}

func TestExecutionContext_MarshalUnmarshal(t *testing.T) {
	original := map[string]interface{}{
		"step": "chunk",
		"index": 5,
		"float_val": 123.45,
	}

	data, err := serialization.MarshalExecutionContext(original)
	if err != nil {
		t.Fatalf("MarshalExecutionContext failed: %v", err)
	}

	var restored map[string]interface{}
	err = serialization.UnmarshalExecutionContext(data, &restored)
	if err != nil {
		t.Fatalf("UnmarshalExecutionContext failed: %v", err)
	}

	// JSON Unmarshal は数値を float64 に変換するため、比較用のマップを調整する
	expectedRestored := map[string]interface{}{
		"step": "chunk",
		"index": float64(5), // JSON Unmarshal converts numbers to float64
		"float_val": 123.45,
	}

	if !reflect.DeepEqual(expectedRestored, restored) {
		t.Errorf("Restored EC mismatch.\nExpected: %v\nRestored: %v", expectedRestored, restored)
	}

	// Test nil input
	dataNil, err := serialization.MarshalExecutionContext(nil)
	if err != nil {
		t.Fatalf("MarshalExecutionContext (nil) failed: %v", err)
	}
	if string(dataNil) != "{}" {
		t.Errorf("Expected empty JSON object for nil EC, got %s", string(dataNil))
	}

	// Test unmarshal into existing map (should clear and overwrite)
	existing := map[string]interface{}{"old_key": "old_value"}
	err = serialization.UnmarshalExecutionContext([]byte(`{"new_key": 1}`), &existing)
	if err != nil {
		t.Fatalf("Unmarshal into existing failed: %v", err)
	}
	if _, ok := existing["old_key"]; ok {
		t.Errorf("Existing key was not cleared")
	}
	if existing["new_key"] != float64(1) { // JSON unmarshal converts numbers to float64
		t.Errorf("New key was not added correctly: %v", existing["new_key"])
	}
}

func TestFailures_MarshalUnmarshal(t *testing.T) {
	original := []string{"err1", "err2"}

	data, err := serialization.MarshalFailures(original)
	if err != nil {
		t.Fatalf("MarshalFailures failed: %v", err)
	}

	var restored []string
	err = serialization.UnmarshalFailures(data, &restored)
	if err != nil {
		t.Fatalf("UnmarshalFailures failed: %v", err)
	}

	if !reflect.DeepEqual(original, restored) {
		t.Errorf("Restored Failures mismatch.\nOriginal: %v\nRestored: %v", original, restored)
	}

	// Test nil input
	dataNil, err := serialization.MarshalFailures(nil)
	if err != nil {
		t.Fatalf("MarshalFailures (nil) failed: %v", err)
	}
	if string(dataNil) != "[]" {
		t.Errorf("Expected empty JSON array for nil Failures, got %s", string(dataNil))
	}
}
