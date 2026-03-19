package env

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// ============================================================================
// parseBool Tests
// ============================================================================

func TestParseBool(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
		wantErr  bool
	}{
		{"", false, false},
		{"0", false, false},
		{"1", true, false},
		{"true", true, false},
		{"TRUE", true, false},
		{"false", false, false},
		{"FALSE", false, false},
		{"yes", true, false},
		{"Yes", true, false},
		{"YES", true, false},
		{"no", false, false},
		{"No", false, false},
		{"NO", false, false},
		{"on", true, false},
		{"On", true, false},
		{"ON", true, false},
		{"off", false, false},
		{"Off", false, false},
		{"OFF", false, false},
		{"enabled", true, false},
		{"Enabled", true, false},
		{"ENABLED", true, false},
		{"disabled", false, false},
		{"Disabled", false, false},
		{"DISABLED", false, false},
		{"y", false, true},
		{"Y", false, true},
		{"n", false, true},
		{"N", false, true},
		{"invalid", false, true},
		{"  ", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := parseBool(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseBool(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if result != tt.expected {
				t.Errorf("parseBool(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

// ============================================================================
// parseDuration Tests
// ============================================================================

func TestParseDuration(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Duration
		wantErr  bool
	}{
		{"", 0, true},
		{"0", 0, false},
		{"30s", 30 * time.Second, false},
		{"5m", 5 * time.Minute, false},
		{"1h", time.Hour, false},
		{"1.5h", 90 * time.Minute, false},
		{"invalid", 0, true},
		{"  30s", 30 * time.Second, false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := parseDuration(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseDuration(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if result != tt.expected {
				t.Errorf("parseDuration(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

// ============================================================================
// parseInt Tests
// ============================================================================

func TestParseInt(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
		wantErr  bool
	}{
		{"", 0, true},
		{"0", 0, false},
		{"42", 42, false},
		{"-42", -42, false},
		{"123", 123, false},
		{"invalid", 0, true},
		{"  42", 42, false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := parseInt(tt.input, 64)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseInt(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if result != tt.expected {
				t.Errorf("parseInt(%q) = %d, want %d", tt.input, result, tt.expected)
			}
		})
	}
}

// ============================================================================
// parseSliceElement Edge Case Tests
// ============================================================================

func TestParseSliceElement(t *testing.T) {
	// Test string type (no conversion needed)
	t.Run("string type", func(t *testing.T) {
		cfg := DefaultConfig()
		loader, err := New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer loader.Close()

		loader.Set("KEY_0", "value1")
		loader.Set("KEY_1", "value2")

		result := GetSliceFrom[string](loader, "KEY")
		if len(result) != 2 || result[0] != "value1" || result[1] != "value2" {
			t.Errorf("GetSliceFrom[string]() = %v, want [value1 value2]", result)
		}
	})

	// Test int type
	t.Run("int type", func(t *testing.T) {
		cfg := DefaultConfig()
		loader, err := New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer loader.Close()

		loader.Set("NUM_0", "42")
		loader.Set("NUM_1", "-10")

		result := GetSliceFrom[int64](loader, "NUM")
		if len(result) != 2 || result[0] != 42 || result[1] != -10 {
			t.Errorf("GetSliceFrom[int64]() = %v, want [42 -10]", result)
		}
	})

	// Test uint type
	t.Run("uint type", func(t *testing.T) {
		cfg := DefaultConfig()
		loader, err := New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer loader.Close()

		loader.Set("UNS_0", "10")
		loader.Set("UNS_1", "20")

		result := GetSliceFrom[uint64](loader, "UNS")
		if len(result) != 2 || result[0] != 10 || result[1] != 20 {
			t.Errorf("GetSliceFrom[uint64]() = %v, want [10 20]", result)
		}
	})

	// Test float64 type
	t.Run("float64 type", func(t *testing.T) {
		cfg := DefaultConfig()
		loader, err := New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer loader.Close()

		loader.Set("RATES_0", "1.5")
		loader.Set("RATES_1", "2.75")

		result := GetSliceFrom[float64](loader, "RATES")
		if len(result) != 2 || result[0] != 1.5 || result[1] != 2.75 {
			t.Errorf("GetSliceFrom[float64]() = %v, want [1.5 2.75]", result)
		}
	})

	// Test bool type
	t.Run("bool type", func(t *testing.T) {
		cfg := DefaultConfig()
		loader, err := New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer loader.Close()

		loader.Set("FLAGS_0", "true")
		loader.Set("FLAGS_1", "false")
		loader.Set("FLAGS_2", "yes")
		loader.Set("FLAGS_3", "no")

		result := GetSliceFrom[bool](loader, "FLAGS")
		if len(result) != 4 || result[0] != true || result[1] != false || result[2] != true || result[3] != false {
			t.Errorf("GetSliceFrom[bool]() = %v, want [true false true false]", result)
		}
	})

	// Test duration type
	t.Run("duration type", func(t *testing.T) {
		cfg := DefaultConfig()
		loader, err := New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer loader.Close()

		loader.Set("TIMES_0", "5s")
		loader.Set("TIMES_1", "1m30s")

		result := GetSliceFrom[time.Duration](loader, "TIMES")
		if len(result) != 2 || result[0] != 5*time.Second || result[1] != 90*time.Second {
			t.Errorf("GetSliceFrom[duration]() = %v, want [5s 1m30s]", result)
		}
	})

	// Test parse error returns default
	t.Run("parse error returns default", func(t *testing.T) {
		cfg := DefaultConfig()
		loader, err := New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer loader.Close()

		loader.Set("BAD_INT_0", "not_a_number")

		result := GetSliceFrom[int64](loader, "BAD_INT", []int64{42})
		if len(result) != 1 || result[0] != 42 {
			t.Errorf("GetSliceFrom[int64]() with bad value = %v, want [42]", result)
		}
	})
}

// ============================================================================
// Marshal Tests
// ============================================================================

func TestMarshal(t *testing.T) {
	tests := []struct {
		name    string
		input   map[string]string
		wantErr bool
	}{
		{
			name:  "simple",
			input: map[string]string{"KEY": "value", "OTHER": "other"},
		},
		{
			name:  "empty",
			input: map[string]string{},
		},
		{
			name:  "special chars",
			input: map[string]string{"SPECIAL": "value with \"quotes\""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Marshal(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Marshal() error = %v", err)
			}
		})
	}
}

// ============================================================================
// Marshal With Format Tests
// ============================================================================

func TestMarshalWithFormat(t *testing.T) {
	input := map[string]string{
		"APP_NAME": "myapp",
		"APP_PORT": "8080",
		"DEBUG":    "true",
	}

	t.Run("default format (dotenv)", func(t *testing.T) {
		result, err := Marshal(input)
		if err != nil {
			t.Errorf("Marshal() error = %v", err)
			return
		}
		if !strings.Contains(result, "APP_NAME=myapp") {
			t.Errorf("expected .env format, got: %s", result)
		}
	})

	t.Run("explicit dotenv format", func(t *testing.T) {
		result, err := Marshal(input, FormatEnv)
		if err != nil {
			t.Errorf("Marshal() error = %v", err)
			return
		}
		if !strings.Contains(result, "APP_NAME=myapp") {
			t.Errorf("expected .env format, got: %s", result)
		}
	})

	t.Run("json format", func(t *testing.T) {
		result, err := Marshal(input, FormatJSON)
		if err != nil {
			t.Errorf("Marshal() error = %v", err)
			return
		}
		if !strings.Contains(result, `"APP"`) || !strings.Contains(result, `"NAME"`) {
			t.Errorf("expected JSON format with nested structure, got: %s", result)
		}
	})

	t.Run("yaml format", func(t *testing.T) {
		result, err := Marshal(input, FormatYAML)
		if err != nil {
			t.Errorf("Marshal() error = %v", err)
			return
		}
		if !strings.Contains(result, "APP_NAME: myapp") {
			t.Errorf("expected YAML format, got: %s", result)
		}
	})
}

func TestMarshalWithStruct(t *testing.T) {
	type AppConfig struct {
		Name string `env:"APP_NAME"`
		Port int    `env:"APP_PORT"`
	}

	config := AppConfig{
		Name: "myapp",
		Port: 8080,
	}

	t.Run("struct to dotenv", func(t *testing.T) {
		result, err := Marshal(config)
		if err != nil {
			t.Errorf("Marshal() error = %v", err)
			return
		}
		if !strings.Contains(result, "APP_NAME=myapp") {
			t.Errorf("expected .env format with APP_NAME=myapp, got: %s", result)
		}
		if !strings.Contains(result, "APP_PORT=8080") {
			t.Errorf("expected .env format with APP_PORT=8080, got: %s", result)
		}
	})

	t.Run("struct to json", func(t *testing.T) {
		result, err := Marshal(config, FormatJSON)
		if err != nil {
			t.Errorf("Marshal() error = %v", err)
			return
		}
		if !strings.Contains(result, `"NAME"`) || !strings.Contains(result, `"myapp"`) {
			t.Errorf("expected JSON format, got: %s", result)
		}
	})

	t.Run("struct to yaml", func(t *testing.T) {
		result, err := Marshal(config, FormatYAML)
		if err != nil {
			t.Errorf("Marshal() error = %v", err)
			return
		}
		if !strings.Contains(result, "APP_NAME: myapp") {
			t.Errorf("expected YAML format, got: %s", result)
		}
	})

	t.Run("struct pointer", func(t *testing.T) {
		result, err := Marshal(&config)
		if err != nil {
			t.Errorf("Marshal() error = %v", err)
			return
		}
		if !strings.Contains(result, "APP_NAME=myapp") {
			t.Errorf("expected .env format, got: %s", result)
		}
	})

	t.Run("nil input", func(t *testing.T) {
		_, err := Marshal(nil)
		if err == nil {
			t.Error("expected error for nil input")
		}
	})
}

// TestMarshalAlwaysSorted verifies that Marshal always outputs sorted keys
func TestMarshalAlwaysSorted(t *testing.T) {
	input := map[string]string{
		"Z_KEY": "z",
		"A_KEY": "a",
		"M_KEY": "m",
	}

	t.Run("dotenv format is sorted", func(t *testing.T) {
		result, err := Marshal(input)
		if err != nil {
			t.Errorf("Marshal() error = %v", err)
			return
		}
		// Verify order
		aIdx := strings.Index(result, "A_KEY")
		mIdx := strings.Index(result, "M_KEY")
		zIdx := strings.Index(result, "Z_KEY")
		if !(aIdx < mIdx && mIdx < zIdx) {
			t.Errorf("keys not in sorted order: A=%d, M=%d, Z=%d", aIdx, mIdx, zIdx)
		}
	})

	t.Run("json format is sorted", func(t *testing.T) {
		result, err := Marshal(input, FormatJSON)
		if err != nil {
			t.Errorf("Marshal() error = %v", err)
			return
		}
		if !strings.Contains(result, "{") {
			t.Errorf("expected JSON format, got: %s", result)
		}
	})

	t.Run("yaml format is sorted", func(t *testing.T) {
		result, err := Marshal(input, FormatYAML)
		if err != nil {
			t.Errorf("Marshal() error = %v", err)
			return
		}
		// Verify order in YAML
		aIdx := strings.Index(result, "A_KEY:")
		mIdx := strings.Index(result, "M_KEY:")
		zIdx := strings.Index(result, "Z_KEY:")
		if !(aIdx < mIdx && mIdx < zIdx) {
			t.Errorf("keys not in sorted order: A=%d, M=%d, Z=%d", aIdx, mIdx, zIdx)
		}
	})
}

// ============================================================================
// IsMarshalError Tests
// ============================================================================

func TestIsMarshalError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "is marshal error",
			err:      &MarshalError{Field: "test", Message: "test"},
			expected: true,
		},
		{
			name:     "is validation error",
			err:      &ValidationError{Field: "test", Message: "test"},
			expected: false,
		},
		{
			name:     "is other error",
			err:      errors.New("test"),
			expected: false,
		},
		{
			name:     "is nil",
			err:      nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsMarshalError(tt.err)
			if result != tt.expected {
				t.Errorf("IsMarshalError() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// ============================================================================
// MarshalStruct Tests
// ============================================================================

func TestMarshalStruct(t *testing.T) {
	t.Run("basic struct", func(t *testing.T) {
		type TestMarshalConfig struct {
			Name string `env:"NAME"`
			Port int    `env:"PORT"`
		}

		c := TestMarshalConfig{
			Name: "test",
			Port: 8080,
		}

		result, err := MarshalStruct(&c)
		if err != nil {
			t.Fatalf("MarshalStruct() error = %v", err)
		}

		if result["NAME"] != "test" {
			t.Errorf("result[\"NAME\"] = %q, want %q", result["NAME"], "test")
		}
		if result["PORT"] != "8080" {
			t.Errorf("result[\"PORT\"] = %q, want %q", result["PORT"], "8080")
		}
	})

	t.Run("empty struct", func(t *testing.T) {
		type TestEmptyMarshalConfig struct{}

		result, err := MarshalStruct(&TestEmptyMarshalConfig{})
		if err != nil {
			t.Fatalf("MarshalStruct() error = %v", err)
		}
		if len(result) != 0 {
			t.Errorf("result = %v, want empty map", result)
		}
	})
}

// ============================================================================
// UnmarshalInto Tests
// ============================================================================

func TestUnmarshalInto(t *testing.T) {
	t.Run("basic struct", func(t *testing.T) {
		type TestUnmarshalConfig struct {
			Name string `env:"NAME"`
			Port int    `env:"PORT"`
		}

		data := map[string]string{
			"NAME": "test",
			"PORT": "8080",
		}

		var c TestUnmarshalConfig
		err := UnmarshalInto(data, &c)
		if err != nil {
			t.Fatalf("UnmarshalInto() error = %v", err)
		}
		if c.Name != "test" {
			t.Errorf("c.Name = %q, want %q", c.Name, "test")
		}
		if c.Port != 8080 {
			t.Errorf("c.Port = %d, want %d", c.Port, 8080)
		}
	})

	t.Run("nil value", func(t *testing.T) {
		type TestUnmarshalConfigNil struct {
			Name string `env:"NAME"`
		}
		var c *TestUnmarshalConfigNil
		err := UnmarshalInto(nil, &c)
		if err == nil {
			t.Error("UnmarshalInto(nil) should return error for pointer to nil pointer")
		}
	})

	t.Run("nil pointer", func(t *testing.T) {
		type TestUnmarshalConfigPtr struct {
			Name string `env:"NAME"`
		}
		data := map[string]string{
			"NAME": "test",
		}
		var c *TestUnmarshalConfigPtr
		err := UnmarshalInto(data, &c)
		if err == nil {
			t.Error("UnmarshalInto() should return error for pointer to nil pointer")
		}
	})

	t.Run("non-pointer", func(t *testing.T) {
		data := map[string]string{
			"NAME": "test",
		}
		var c int
		err := UnmarshalInto(data, c)
		if err == nil {
			t.Error("UnmarshalInto() should return error for non-pointer")
		}
	})

	t.Run("non-struct pointer", func(t *testing.T) {
		data := map[string]string{}
		var c string
		err := UnmarshalInto(data, &c)
		if err == nil {
			t.Error("UnmarshalInto() should return error for pointer to non-struct")
		}
	})
}

// ============================================================================
// UnmarshalStruct (String Version) Tests
// ============================================================================

func TestUnmarshalStructFromString(t *testing.T) {
	t.Run("env format to struct", func(t *testing.T) {
		type TestConfig struct {
			Host string `env:"HOST"`
			Port int    `env:"PORT"`
		}

		data := "HOST=localhost\nPORT=8080"
		var cfg TestConfig
		err := UnmarshalStruct(data, &cfg)
		if err != nil {
			t.Fatalf("UnmarshalStruct() error = %v", err)
		}
		if cfg.Host != "localhost" {
			t.Errorf("cfg.Host = %q, want %q", cfg.Host, "localhost")
		}
		if cfg.Port != 8080 {
			t.Errorf("cfg.Port = %d, want %d", cfg.Port, 8080)
		}
	})

	t.Run("json format to struct", func(t *testing.T) {
		type TestConfig struct {
			Host string `env:"SERVER_HOST"`
			Port int    `env:"SERVER_PORT"`
		}

		data := `{"server": {"host": "localhost", "port": 8080}}`
		var cfg TestConfig
		err := UnmarshalStruct(data, &cfg, FormatJSON)
		if err != nil {
			t.Fatalf("UnmarshalStruct() error = %v", err)
		}
		if cfg.Host != "localhost" {
			t.Errorf("cfg.Host = %q, want %q", cfg.Host, "localhost")
		}
		if cfg.Port != 8080 {
			t.Errorf("cfg.Port = %d, want %d", cfg.Port, 8080)
		}
	})

	t.Run("yaml format to struct", func(t *testing.T) {
		type TestConfig struct {
			Host string `env:"SERVER_HOST"`
			Port int    `env:"SERVER_PORT"`
		}

		data := "server:\n  host: localhost\n  port: 8080\n"
		var cfg TestConfig
		err := UnmarshalStruct(data, &cfg, FormatYAML)
		if err != nil {
			t.Fatalf("UnmarshalStruct() error = %v", err)
		}
		if cfg.Host != "localhost" {
			t.Errorf("cfg.Host = %q, want %q", cfg.Host, "localhost")
		}
		if cfg.Port != 8080 {
			t.Errorf("cfg.Port = %d, want %d", cfg.Port, 8080)
		}
	})

	t.Run("auto-detect json", func(t *testing.T) {
		type TestConfig struct {
			Host string `env:"SERVER_HOST"`
		}

		data := `{"server": {"host": "auto-detected"}}`
		var cfg TestConfig
		err := UnmarshalStruct(data, &cfg, FormatAuto)
		if err != nil {
			t.Fatalf("UnmarshalStruct() error = %v", err)
		}
		if cfg.Host != "auto-detected" {
			t.Errorf("cfg.Host = %q, want %q", cfg.Host, "auto-detected")
		}
	})
}

// ============================================================================
// UnmarshalMap Tests
// ============================================================================

func TestUnmarshalMap(t *testing.T) {
	t.Run("env format", func(t *testing.T) {
		data := "KEY=value\nPORT=8080"
		result, err := UnmarshalMap(data)
		if err != nil {
			t.Fatalf("UnmarshalMap() error = %v", err)
		}
		if result["KEY"] != "value" {
			t.Errorf("result[\"KEY\"] = %q, want %q", result["KEY"], "value")
		}
		if result["PORT"] != "8080" {
			t.Errorf("result[\"PORT\"] = %q, want %q", result["PORT"], "8080")
		}
	})

	t.Run("json format", func(t *testing.T) {
		data := `{"database": {"host": "localhost", "port": 5432}}`
		result, err := UnmarshalMap(data, FormatJSON)
		if err != nil {
			t.Fatalf("UnmarshalMap() error = %v", err)
		}
		if result["DATABASE_HOST"] != "localhost" {
			t.Errorf("result[\"DATABASE_HOST\"] = %q, want %q", result["DATABASE_HOST"], "localhost")
		}
		if result["DATABASE_PORT"] != "5432" {
			t.Errorf("result[\"DATABASE_PORT\"] = %q, want %q", result["DATABASE_PORT"], "5432")
		}
	})

	t.Run("yaml format", func(t *testing.T) {
		data := "database:\n  host: localhost\n  port: 5432\n"
		result, err := UnmarshalMap(data, FormatYAML)
		if err != nil {
			t.Fatalf("UnmarshalMap() error = %v", err)
		}
		if result["DATABASE_HOST"] != "localhost" {
			t.Errorf("result[\"DATABASE_HOST\"] = %q, want %q", result["DATABASE_HOST"], "localhost")
		}
		if result["DATABASE_PORT"] != "5432" {
			t.Errorf("result[\"DATABASE_PORT\"] = %q, want %q", result["DATABASE_PORT"], "5432")
		}
	})

	t.Run("auto-detect json", func(t *testing.T) {
		data := `{"key": "value"}`
		result, err := UnmarshalMap(data, FormatAuto)
		if err != nil {
			t.Fatalf("UnmarshalMap() error = %v", err)
		}
		if result["KEY"] != "value" {
			t.Errorf("result[\"KEY\"] = %q, want %q", result["KEY"], "value")
		}
	})

	t.Run("auto-detect yaml", func(t *testing.T) {
		data := "key: value\nother: test"
		result, err := UnmarshalMap(data, FormatAuto)
		if err != nil {
			t.Fatalf("UnmarshalMap() error = %v", err)
		}
		if result["KEY"] != "value" {
			t.Errorf("result[\"KEY\"] = %q, want %q", result["KEY"], "value")
		}
	})

	t.Run("auto-detect env (default)", func(t *testing.T) {
		data := "KEY=value\nOTHER=test"
		result, err := UnmarshalMap(data, FormatAuto)
		if err != nil {
			t.Fatalf("UnmarshalMap() error = %v", err)
		}
		if result["KEY"] != "value" {
			t.Errorf("result[\"KEY\"] = %q, want %q", result["KEY"], "value")
		}
	})
}

// ============================================================================
// detectDataFormat Tests
// ============================================================================

func TestDetectDataFormat(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected FileFormat
	}{
		{"json object", `{"key": "value"}`, FormatJSON},
		{"json array", `[1, 2, 3]`, FormatJSON},
		{"yaml with colon", "key: value", FormatYAML},
		{"yaml with list", "- item1\n- item2", FormatYAML},
		{"env format", "KEY=value", FormatEnv},
		{"env with comment", "# comment\nKEY=value", FormatEnv},
		{"empty string", "", FormatEnv},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectDataFormat(tt.input)
			if result != tt.expected {
				t.Errorf("detectDataFormat(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}
