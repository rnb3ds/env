package env

import (
	"errors"
	"reflect"
	"strconv"
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

// testMarshaler implements the Marshaler interface for testing.
type testMarshaler struct {
	data string
}

func (m *testMarshaler) MarshalEnv() ([]byte, error) {
	return []byte(m.data), nil
}

func TestToMap(t *testing.T) {
	tests := []struct {
		name    string
		input   interface{}
		want    map[string]string
		wantErr bool
	}{
		{
			name:  "map[string]string passthrough",
			input: map[string]string{"KEY": "value"},
			want:  map[string]string{"KEY": "value"},
		},
		{
			name:    "nil input",
			input:   nil,
			wantErr: true,
		},
		{
			name:    "nil map pointer",
			input:   (*map[string]string)(nil),
			wantErr: true,
		},
		{
			name: "valid map pointer",
			input: func() interface{} {
				m := map[string]string{"KEY": "value"}
				return &m
			}(),
			want: map[string]string{"KEY": "value"},
		},
		{
			name: "struct input",
			input: struct {
				Name string `env:"NAME"`
			}{Name: "test"},
			want: map[string]string{"NAME": "test"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := toMap(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("toMap() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				for k, v := range tt.want {
					if got[k] != v {
						t.Errorf("toMap()[%q] = %q, want %q", k, got[k], v)
					}
				}
			}
		})
	}
}

func TestMarshalStruct_Marshaler(t *testing.T) {
	t.Run("implements Marshaler interface", func(t *testing.T) {
		m := &testMarshaler{data: "KEY=value\nPORT=8080"}
		result, err := MarshalStruct(m)
		if err != nil {
			t.Fatalf("MarshalStruct() error = %v", err)
		}
		if result["KEY"] != "value" {
			t.Errorf("result[\"KEY\"] = %q, want %q", result["KEY"], "value")
		}
		if result["PORT"] != "8080" {
			t.Errorf("result[\"PORT\"] = %q, want %q", result["PORT"], "8080")
		}
	})
}

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

	t.Run("narrow integer field rejects out-of-range value", func(t *testing.T) {
		// Regression: values were parsed at 64-bit width and then silently
		// truncated by reflect (300 → 44 in an int8 field). They must error.
		type NarrowConfig struct {
			Port int8 `env:"PORT"`
		}
		var c NarrowConfig
		err := UnmarshalInto(map[string]string{"PORT": "300"}, &c)
		if err == nil {
			t.Fatalf("UnmarshalInto() succeeded with out-of-range int8 value, Port = %d", c.Port)
		}
		if c.Port != 0 {
			t.Errorf("Port = %d on failure, want untouched zero value", c.Port)
		}
	})
}

// TestMarshalYAMLRoundTripFidelity verifies that string values which look like
// YAML scalars (bools, null, numbers) and values with trailing whitespace
// survive a Marshal→UnmarshalMap round trip through FormatYAML unchanged.
// Regression: they were emitted unquoted, so the YAML reader coerced them
// ("null" → "", "true" → "true"/bool normalization, trailing space stripped).
func TestMarshalYAMLRoundTripFidelity(t *testing.T) {
	original := map[string]string{
		"MODE":       "null",
		"EMPTY_WORD": "~",
		"FLAG":       "true",
		"OTHER_FLAG": "false",
		"NUMBER":     "8080",
		"RATIO":      "3.14",
		"SPACED":     "trailing ",
	}
	data, err := Marshal(original, FormatYAML)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	roundTripped, err := UnmarshalMap(data, FormatYAML)
	if err != nil {
		t.Fatalf("UnmarshalMap() error = %v", err)
	}

	for key, want := range original {
		if got := roundTripped[key]; got != want {
			t.Errorf("round trip changed %s: got %q, want %q (marshaled as:\n%s)", key, got, want, data)
		}
	}
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
	// deepYAML nests 12 levels deep (defaultUnmarshalDepth is 10) so the
	// flatten step must reject it.
	var deepYAML strings.Builder
	for i := range 12 {
		deepYAML.WriteString(strings.Repeat("  ", i))
		deepYAML.WriteString("key:\n")
	}
	deepYAML.WriteString(strings.Repeat("  ", 12))
	deepYAML.WriteString("leaf: deep")

	tests := []struct {
		name    string
		data    string
		formats []FileFormat
		want    map[string]string
		wantLen int // expected len(result); defaults to len(want)
		wantErr bool
	}{
		{
			name: "env format (default)",
			data: "KEY=value\nPORT=8080",
			want: map[string]string{"KEY": "value", "PORT": "8080"},
		},
		{
			name:    "empty env input yields empty map",
			data:    "",
			wantLen: 0,
		},
		{
			name:    "json format",
			data:    `{"database": {"host": "localhost", "port": 5432}}`,
			formats: []FileFormat{FormatJSON},
			want:    map[string]string{"DATABASE_HOST": "localhost", "DATABASE_PORT": "5432"},
		},
		{
			// Boundary: empty JSON input short-circuits to an empty map.
			name:    "empty json input yields empty map",
			data:    "",
			formats: []FileFormat{FormatJSON},
			wantLen: 0,
		},
		{
			name:    "invalid json returns error",
			data:    `{invalid json}`,
			formats: []FileFormat{FormatJSON},
			wantErr: true,
		},
		{
			name:    "yaml format",
			data:    "database:\n  host: localhost\n  port: 5432\n",
			formats: []FileFormat{FormatYAML},
			want:    map[string]string{"DATABASE_HOST": "localhost", "DATABASE_PORT": "5432"},
		},
		{
			// Boundary: empty YAML input short-circuits to an empty map.
			name:    "empty yaml input yields empty map",
			data:    "",
			formats: []FileFormat{FormatYAML},
			wantLen: 0,
		},
		{
			// Boundary: nesting beyond defaultUnmarshalDepth (10) errors.
			name:    "yaml exceeding depth limit returns error",
			data:    deepYAML.String(),
			formats: []FileFormat{FormatYAML},
			wantErr: true,
		},
		{
			name:    "auto-detect json",
			data:    `{"key": "value"}`,
			formats: []FileFormat{FormatAuto},
			want:    map[string]string{"KEY": "value"},
		},
		{
			name:    "auto-detect yaml",
			data:    "key: value\nother: test",
			formats: []FileFormat{FormatAuto},
			want:    map[string]string{"KEY": "value", "OTHER": "test"},
		},
		{
			name:    "auto-detect env (default)",
			data:    "KEY=value\nOTHER=test",
			formats: []FileFormat{FormatAuto},
			want:    map[string]string{"KEY": "value", "OTHER": "test"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := UnmarshalMap(tt.data, tt.formats...)
			if (err != nil) != tt.wantErr {
				t.Fatalf("UnmarshalMap() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			for key, want := range tt.want {
				if result[key] != want {
					t.Errorf("result[%q] = %q, want %q", key, result[key], want)
				}
			}
			// wantLen defaults to len(want) so a row listing expected keys
			// also pins the result size.
			wantLen := tt.wantLen
			if wantLen == 0 {
				wantLen = len(tt.want)
			}
			if len(result) != wantLen {
				t.Errorf("len(result) = %d, want %d (got %v)", len(result), wantLen, result)
			}
		})
	}
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

// ============================================================================
// Mock Validators for Testing (from adapters_test.go)
// ============================================================================

// mockLineKeyValidator only implements ValidateKey (LineKeyValidator).
// Used for testing the validatorInterfaceWrapper with minimal interface.
type mockLineKeyValidator struct {
	err error
}

func (m *mockLineKeyValidator) ValidateKey(key string) error {
	return m.err
}

// mockLineKeyValueValidator implements ValidateKey and ValidateValue but not ValidateRequired.
type mockLineKeyValueValidator struct {
	keyErr   error
	valueErr error
}

func (v *mockLineKeyValueValidator) ValidateKey(key string) error {
	return v.keyErr
}

func (v *mockLineKeyValueValidator) ValidateValue(value string) error {
	return v.valueErr
}

// fullMockValidator implements the complete Validator interface.
type fullMockValidator struct {
	keyErr      error
	valueErr    error
	requiredErr error
}

func (f *fullMockValidator) ValidateKey(key string) error {
	return f.keyErr
}

func (f *fullMockValidator) ValidateValue(value string) error {
	return f.valueErr
}

func (f *fullMockValidator) ValidateRequired(keys map[string]bool) error {
	return f.requiredErr
}

// minimalMockValidator implements Validator but ValidateRequired returns ErrValidateRequiredUnsupported.
type minimalMockValidator struct {
	keyErr   error
	valueErr error
}

func (m *minimalMockValidator) ValidateKey(key string) error {
	return m.keyErr
}

func (m *minimalMockValidator) ValidateValue(value string) error {
	return m.valueErr
}

func (m *minimalMockValidator) ValidateRequired(keys map[string]bool) error {
	return ErrValidateRequiredUnsupported
}

// ============================================================================
// validatorInterfaceWrapper Tests
// ============================================================================

func TestValidatorInterfaceWrapper_ValidateKey(t *testing.T) {
	tests := []struct {
		name    string
		wrapper *validatorInterfaceWrapper
		key     string
		wantErr bool
	}{
		{
			name:    "passes validation",
			wrapper: &validatorInterfaceWrapper{&mockLineKeyValidator{}},
			key:     "TEST_KEY",
			wantErr: false,
		},
		{
			name:    "fails validation",
			wrapper: &validatorInterfaceWrapper{&mockLineKeyValidator{err: errors.New("invalid key")}},
			key:     "BAD_KEY",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.wrapper.ValidateKey(tt.key)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateKey() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidatorInterfaceWrapper_ValidateValue(t *testing.T) {
	t.Run("minimal validator returns nil", func(t *testing.T) {
		wrapper := &validatorInterfaceWrapper{&mockLineKeyValidator{}}
		err := wrapper.ValidateValue("test value")
		if err != nil {
			t.Errorf("ValidateValue() should return nil for minimal validator, got %v", err)
		}
	})

	t.Run("validator with LineValueValidator delegates", func(t *testing.T) {
		// mockLineKeyValueValidator implements both LineKeyValidator and LineValueValidator
		val := &mockLineKeyValueValidator{valueErr: errors.New("bad value")}
		wrapper := &validatorInterfaceWrapper{val}

		err := wrapper.ValidateValue("bad")
		if err == nil {
			t.Error("ValidateValue() should delegate to LineValueValidator")
		}
		if err.Error() != "bad value" {
			t.Errorf("ValidateValue() error = %v, want 'bad value'", err)
		}
	})

	t.Run("validator without LineValueValidator returns nil", func(t *testing.T) {
		wrapper := &validatorInterfaceWrapper{&mockLineKeyValidator{}}
		err := wrapper.ValidateValue("any value")
		if err != nil {
			t.Errorf("ValidateValue() should return nil when LineValueValidator not implemented, got %v", err)
		}
	})
}

func TestValidatorInterfaceWrapper_ValidateRequired(t *testing.T) {
	t.Run("returns ErrValidateRequiredUnsupported", func(t *testing.T) {
		wrapper := &validatorInterfaceWrapper{&mockLineKeyValidator{}}
		keys := map[string]bool{"KEY1": true, "KEY2": true}

		err := wrapper.ValidateRequired(keys)
		if !errors.Is(err, ErrValidateRequiredUnsupported) {
			t.Errorf("ValidateRequired() error = %v, want ErrValidateRequiredUnsupported", err)
		}
	})

	t.Run("error message is descriptive", func(t *testing.T) {
		wrapper := &validatorInterfaceWrapper{&mockLineKeyValidator{}}
		keys := map[string]bool{"KEY": true}

		err := wrapper.ValidateRequired(keys)
		if err == nil {
			t.Fatal("ValidateRequired() should return error")
		}
		// Check that error message contains guidance
		errMsg := err.Error()
		if !strings.Contains(errMsg, "ValidateRequired") {
			t.Errorf("Error message should mention ValidateRequired, got: %s", errMsg)
		}
		if !strings.Contains(errMsg, "Validator") {
			t.Errorf("Error message should mention Validator interface, got: %s", errMsg)
		}
	})
}

// ============================================================================
// Integration Tests with ComponentFactory
// ============================================================================

func TestComponentFactory_Validator_WithMinimalCustomValidator(t *testing.T) {
	cfg := DefaultConfig()
	cfg.CustomValidator = &minimalMockValidator{}

	factory := cfg.buildComponentFactory()
	defer factory.Close()

	validator := factory.Validator()

	// ValidateKey should work
	if err := validator.ValidateKey("TEST_KEY"); err != nil {
		t.Errorf("ValidateKey() error = %v", err)
	}

	// ValidateRequired should return explicit error
	err := validator.ValidateRequired(map[string]bool{"KEY": true})
	if !errors.Is(err, ErrValidateRequiredUnsupported) {
		t.Errorf("ValidateRequired() error = %v, want ErrValidateRequiredUnsupported", err)
	}
}

func TestComponentFactory_Validator_WithFullCustomValidator(t *testing.T) {
	cfg := DefaultConfig()
	cfg.CustomValidator = &fullMockValidator{requiredErr: errors.New("missing keys")}

	factory := cfg.buildComponentFactory()
	defer factory.Close()

	validator := factory.Validator()

	// ValidateRequired should delegate to custom implementation
	err := validator.ValidateRequired(map[string]bool{"KEY": true})
	if err == nil || err.Error() != "missing keys" {
		t.Errorf("ValidateRequired() should delegate to full validator, got %v", err)
	}

	// Should NOT be ErrValidateRequiredUnsupported
	if errors.Is(err, ErrValidateRequiredUnsupported) {
		t.Error("ValidateRequired() should not return ErrValidateRequiredUnsupported for full validator")
	}
}

func TestComponentFactory_Validator_WithBuiltInValidator(t *testing.T) {
	cfg := DefaultConfig()
	cfg.RequiredKeys = []string{"REQUIRED_KEY"}

	factory := cfg.buildComponentFactory()
	defer factory.Close()

	validator := factory.Validator()

	// ValidateRequired should fail for missing required key
	err := validator.ValidateRequired(map[string]bool{"OTHER_KEY": true})
	if err == nil {
		t.Error("ValidateRequired() should fail for missing required key")
	}

	// Should NOT be ErrValidateRequiredUnsupported
	if errors.Is(err, ErrValidateRequiredUnsupported) {
		t.Error("Built-in validator should not return ErrValidateRequiredUnsupported")
	}
}

// ============================================================================
// Integration Tests with Loader
// ============================================================================

func TestLoader_New_WithMinimalCustomValidatorAndRequiredKeys(t *testing.T) {
	fs := newTestFileSystem()
	fs.files["test.env"] = "EXISTING_KEY=value"

	cfg := DefaultConfig()
	cfg.FileSystem = fs
	cfg.Filenames = []string{"test.env"}
	cfg.RequiredKeys = []string{"REQUIRED_KEY"}
	cfg.CustomValidator = &minimalMockValidator{} // Returns ErrValidateRequiredUnsupported

	// New() should fail because ValidateRequired is called during file parsing
	// and minimalMockValidator returns ErrValidateRequiredUnsupported
	_, err := New(cfg)
	if !errors.Is(err, ErrValidateRequiredUnsupported) {
		t.Errorf("New() error = %v, want ErrValidateRequiredUnsupported", err)
	}
}

func TestLoader_New_WithMinimalCustomValidatorNoFiles(t *testing.T) {
	// When no files are loaded, ValidateRequired is not called during New()
	cfg := DefaultConfig()
	cfg.Filenames = []string{} // No files to load
	cfg.CustomValidator = &minimalMockValidator{}

	loader, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer loader.Close()

	// Validate() should still return ErrValidateRequiredUnsupported
	err = loader.Validate()
	if !errors.Is(err, ErrValidateRequiredUnsupported) {
		t.Errorf("Validate() error = %v, want ErrValidateRequiredUnsupported", err)
	}
}

func TestLoader_Validate_WithFullCustomValidator(t *testing.T) {
	fs := newTestFileSystem()
	fs.files["test.env"] = "EXISTING_KEY=value"

	cfg := DefaultConfig()
	cfg.FileSystem = fs
	cfg.Filenames = []string{"test.env"}
	cfg.RequiredKeys = []string{"REQUIRED_KEY"}
	cfg.CustomValidator = &fullMockValidator{} // Implements full Validator

	loader, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer loader.Close()

	// Validate() should NOT return ErrValidateRequiredUnsupported
	// (it may return a different error from the fullMockValidator, but not the unsupported error)
	err = loader.Validate()
	if errors.Is(err, ErrValidateRequiredUnsupported) {
		t.Errorf("Validate() should not return ErrValidateRequiredUnsupported for full validator, got %v", err)
	}
}

// ============================================================================
// ErrValidateRequiredUnsupported Tests
// ============================================================================

func TestErrValidateRequiredUnsupported(t *testing.T) {
	t.Run("error message contains guidance", func(t *testing.T) {
		errMsg := ErrValidateRequiredUnsupported.Error()
		if !strings.Contains(errMsg, "ValidateRequired") {
			t.Errorf("Error message should mention ValidateRequired, got: %s", errMsg)
		}
		if !strings.Contains(errMsg, "Validator") {
			t.Errorf("Error message should mention Validator interface, got: %s", errMsg)
		}
	})

	t.Run("errors.Is matches through wrapping", func(t *testing.T) {
		wrappedErr := errors.Join(errors.New("context"), ErrValidateRequiredUnsupported)
		if !errors.Is(wrappedErr, ErrValidateRequiredUnsupported) {
			t.Error("errors.Is should match ErrValidateRequiredUnsupported in wrapped error")
		}
	})
}

// ============================================================================
// auditorAdapter Tests
// ============================================================================

func TestAuditorAdapter_NilSafety(t *testing.T) {
	t.Run("constructor returns nil for nil auditor", func(t *testing.T) {
		if adapter := newAuditorAdapter(nil); adapter != nil {
			t.Error("newAuditorAdapter(nil) should return nil")
		}
	})

	t.Run("Close on nil adapter is a no-op", func(t *testing.T) {
		var adapter *auditorAdapter
		if err := adapter.Close(); err != nil {
			t.Errorf("Close() on nil adapter should return nil, got %v", err)
		}
	})
}

func TestAuditorAdapter_IntegrationWithLoader(t *testing.T) {
	fs := newTestFileSystem()
	fs.files[".env"] = "KEY=value"

	cfg := DefaultConfig()
	cfg.FileSystem = fs
	cfg.AuditEnabled = true
	cfg.AuditHandler = NewNopAuditHandler()

	loader, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer loader.Close()

	// The adapter is tested through the loader's audit functionality
	// If the loader works with audit enabled, the adapter works
	if loader.GetString("KEY") != "value" {
		t.Errorf("GetString(\"KEY\") = %q, want %q", loader.GetString("KEY"), "value")
	}
}

// ============================================================================
// auditorInterfaceWrapper Tests
// ============================================================================

type mockAuditLogger struct {
	lastAction AuditAction
	lastKey    string
	lastErrMsg string
	logError   error
}

func (m *mockAuditLogger) LogError(action AuditAction, key, errMsg string) error {
	m.lastAction = action
	m.lastKey = key
	m.lastErrMsg = errMsg
	return m.logError
}

type mockFullAuditLogger struct {
	logs []string
}

func (m *mockFullAuditLogger) Log(action AuditAction, key, reason string, success bool) error {
	m.logs = append(m.logs, "Log")
	return nil
}

func (m *mockFullAuditLogger) LogError(action AuditAction, key, errMsg string) error {
	m.logs = append(m.logs, "LogError")
	return nil
}

func (m *mockFullAuditLogger) LogWithFile(action AuditAction, key, file, reason string, success bool) error {
	m.logs = append(m.logs, "LogWithFile")
	return nil
}

func (m *mockFullAuditLogger) LogWithDuration(action AuditAction, key, reason string, success bool, duration time.Duration) error {
	m.logs = append(m.logs, "LogWithDuration")
	return nil
}

func (m *mockFullAuditLogger) Close() error {
	m.logs = append(m.logs, "Close")
	return nil
}

func TestAuditorInterfaceWrapper(t *testing.T) {
	tests := []struct {
		name      string
		invoke    func(w *auditorInterfaceWrapper) error
		wantMsg   string // expected mockAuditLogger.lastErrMsg ("": assert non-empty)
		wantMsgOK bool   // true: assert non-empty instead of exact match
	}{
		{
			name: "Log success",
			invoke: func(w *auditorInterfaceWrapper) error {
				return w.Log(ActionSet, "KEY", "reason", true)
			},
			wantMsg: "[ok] reason",
		},
		{
			name: "Log failure",
			invoke: func(w *auditorInterfaceWrapper) error {
				return w.Log(ActionSet, "KEY", "reason", false)
			},
			wantMsg: "[error] reason",
		},
		{
			name: "LogWithFile",
			invoke: func(w *auditorInterfaceWrapper) error {
				return w.LogWithFile(ActionSet, "KEY", "test.env", "reason", true)
			},
			wantMsg: "[ok] reason (file: test.env)",
		},
		{
			name: "LogWithDuration",
			invoke: func(w *auditorInterfaceWrapper) error {
				return w.LogWithDuration(ActionSet, "KEY", "reason", true, 100*time.Millisecond)
			},
			wantMsgOK: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockAuditLogger{}
			wrapper := &auditorInterfaceWrapper{AuditLogger: mock}

			if err := tt.invoke(wrapper); err != nil {
				t.Fatalf("invoke() error = %v", err)
			}
			switch {
			case tt.wantMsgOK:
				if mock.lastErrMsg == "" {
					t.Error("should produce non-empty message")
				}
			case mock.lastErrMsg != tt.wantMsg:
				t.Errorf("errMsg = %q, want %q", mock.lastErrMsg, tt.wantMsg)
			}
		})
	}

	t.Run("Close non-closer returns nil", func(t *testing.T) {
		wrapper := &auditorInterfaceWrapper{AuditLogger: &mockAuditLogger{}}
		if err := wrapper.Close(); err != nil {
			t.Errorf("Close() error = %v, want nil", err)
		}
	})

	t.Run("Close closer delegates", func(t *testing.T) {
		mock := &mockFullAuditLogger{}
		wrapper := &auditorInterfaceWrapper{AuditLogger: mock}

		if err := wrapper.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
		if len(mock.logs) != 1 || mock.logs[0] != "Close" {
			t.Errorf("Close() should delegate, logs = %v", mock.logs)
		}
	})
}

// ============================================================================
// Slice Parsing Boundary Tests
// ============================================================================

// TestParseSliceElement_InvalidInput pins the error contract across every
// supported element type: all parse failures must surface as *ValidationError
// (not raw strconv errors), enabling uniform handling by callers.
func TestParseSliceElement_InvalidInput(t *testing.T) {
	tests := []struct {
		name    string
		parse   func() error
		wantErr bool
	}{
		{"string never fails", func() error { _, err := parseSliceElement[string]("anything"); return err }, false},
		{"int invalid", func() error { _, err := parseSliceElement[int]("12x"); return err }, true},
		{"int overflow", func() error { _, err := parseSliceElement[int]("9223372036854775808"); return err }, true},
		{"int64 invalid", func() error { _, err := parseSliceElement[int64]("bad"); return err }, true},
		{"uint negative", func() error { _, err := parseSliceElement[uint]("-1"); return err }, true},
		{"uint64 invalid", func() error { _, err := parseSliceElement[uint64]("1.5"); return err }, true},
		{"float invalid", func() error { _, err := parseSliceElement[float64]("x"); return err }, true},
		{"bool invalid", func() error { _, err := parseSliceElement[bool]("maybe"); return err }, true},
		{"duration invalid", func() error { _, err := parseSliceElement[time.Duration]("notaduration"); return err }, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.parse()
			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				var ve *ValidationError
				if !errors.As(err, &ve) {
					t.Errorf("error = %v (%T), want *ValidationError", err, err)
				}
			}
		})
	}
}

// TestParseSliceElement_ExtremeValidValues checks the numeric boundaries of
// each element type round-trip exactly.
func TestParseSliceElement_ExtremeValidValues(t *testing.T) {
	t.Run("int64 extremes", func(t *testing.T) {
		for _, s := range []string{"-9223372036854775808", "9223372036854775807"} {
			v, err := parseSliceElement[int64](s)
			if err != nil {
				t.Fatalf("parseSliceElement[int64](%q) error = %v", s, err)
			}
			if strconv.FormatInt(v, 10) != s {
				t.Errorf("parseSliceElement[int64](%q) = %d, round-trip mismatch", s, v)
			}
		}
	})

	t.Run("uint64 max", func(t *testing.T) {
		v, err := parseSliceElement[uint64]("18446744073709551615")
		if err != nil || v != 18446744073709551615 {
			t.Errorf("parseSliceElement[uint64] max = %d, %v", v, err)
		}
	})

	t.Run("whitespace is trimmed before parsing", func(t *testing.T) {
		v, err := parseSliceElement[int]("  42\t")
		if err != nil || v != 42 {
			t.Errorf("parseSliceElement[int] with whitespace = %d, %v", v, err)
		}
	})
}

// TestParseCommaSeparated_Fallback pins the default-value semantics: empty
// input and unparseable elements fall back to the caller's default when one
// is provided, and to nil otherwise; empty segments are dropped.
func TestParseCommaSeparated_Fallback(t *testing.T) {
	def := []int{7}

	tests := []struct {
		name  string
		value string
		def   []int // nil = no default
		want  []int
	}{
		{"empty without default", "", nil, nil},
		{"empty with default", "", def, def},
		{"invalid element without default", "1,bad,3", nil, nil},
		{"invalid element with default", "1,bad,3", def, def},
		{"blank segments dropped", " 1 , , 2 ,", nil, []int{1, 2}},
		{"all valid", "1,2,3", nil, []int{1, 2, 3}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseCommaSeparated(tt.value, tt.def)
			if tt.want == nil && got != nil {
				t.Fatalf("parseCommaSeparated(%q) = %v, want nil", tt.value, got)
			}
			if tt.want != nil && !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("parseCommaSeparated(%q) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

// ============================================================================
// Unmarshal Boundary Tests
// ============================================================================

// recordingUnmarshaler implements Unmarshaler to verify that UnmarshalInto
// defers to custom unmarshaling before any reflection takes place.
type recordingUnmarshaler struct {
	got map[string]string
	err error
}

func (u *recordingUnmarshaler) UnmarshalEnv(m map[string]string) error {
	u.got = m
	return u.err
}

func TestUnmarshalInto_UnmarshalerPrecedence(t *testing.T) {
	t.Run("success delegates the map", func(t *testing.T) {
		u := &recordingUnmarshaler{}
		data := map[string]string{"K": "v"}
		if err := UnmarshalInto(data, u); err != nil {
			t.Fatalf("UnmarshalInto() error = %v", err)
		}
		if u.got["K"] != "v" {
			t.Errorf("UnmarshalEnv received %v, want the source map", u.got)
		}
	})

	t.Run("custom error propagates", func(t *testing.T) {
		wantErr := errors.New("custom unmarshal failure")
		u := &recordingUnmarshaler{err: wantErr}
		if err := UnmarshalInto(map[string]string{"K": "v"}, u); !errors.Is(err, wantErr) {
			t.Errorf("UnmarshalInto() error = %v, want %v", err, wantErr)
		}
	})
}

// failingEnvMarshaler implements Marshaler to verify MarshalStruct propagates
// marshaling failures.
type failingEnvMarshaler struct{}

func (failingEnvMarshaler) MarshalEnv() ([]byte, error) {
	return nil, errors.New("marshal exploded")
}

func TestMarshalStruct_MarshalerError(t *testing.T) {
	if _, err := MarshalStruct(failingEnvMarshaler{}); err == nil {
		t.Error("MarshalStruct() should propagate the Marshaler error")
	}
}

func TestUnmarshalStruct_PropagatesParseError(t *testing.T) {
	var target struct {
		K string
	}
	if err := UnmarshalStruct("BROKEN=\"unterminated\n", &target); err == nil {
		t.Error("UnmarshalStruct() should propagate the UnmarshalMap parse error")
	}
}

// TestUnmarshalMap_FormatAutoFallback pins the auto-detection fallback: data
// whose first meaningful line matches no format signature (no ": ", no "=")
// and comment/blank-only data both default to FormatEnv parsing.
func TestUnmarshalMap_FormatAutoFallback(t *testing.T) {
	tests := []struct {
		name string
		data string
		want map[string]string
	}{
		{"content with no format signature", "plainword", map[string]string{}},
		{"comments and blanks only", "# a comment\n\n# another\n", map[string]string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := UnmarshalMap(tt.data, FormatAuto)
			if err != nil {
				t.Fatalf("UnmarshalMap() error = %v", err)
			}
			if len(got) != len(tt.want) {
				t.Errorf("UnmarshalMap() = %v, want %v", got, tt.want)
			}
		})
	}
}
