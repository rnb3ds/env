package internal

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// ============================================================================
// JSON Flatten Tests
// ============================================================================

// defaultJSONCfg returns the standard flatten configuration used by most
// cases; individual cases override fields as needed.
func defaultJSONCfg() JSONFlattenConfig {
	return JSONFlattenConfig{
		KeyDelimiter:     "_",
		ArrayIndexFormat: "underscore",
		NullAsEmpty:      true,
		NumberAsString:   true,
		BoolAsString:     true,
		MaxDepth:         10,
	}
}

func TestFlattenJSON(t *testing.T) {
	longKey := strings.Repeat("K", 70)

	tests := []struct {
		name        string
		input       string
		cfg         JSONFlattenConfig
		want        map[string]string // keys asserted when non-nil
		wantLen     int               // expected len(result); -1 skips the check
		wantErr     bool
		wantJSONErr bool // error must unwrap to *JSONError
		check       func(t *testing.T, result map[string]string)
	}{
		{
			name:    "empty input yields empty map",
			input:   "",
			cfg:     defaultJSONCfg(),
			wantLen: 0,
		},
		{
			name:    "root null yields empty map",
			input:   `null`,
			cfg:     defaultJSONCfg(),
			wantLen: 0,
		},
		{
			name:    "root scalar yields empty map",
			input:   `"hello"`,
			cfg:     defaultJSONCfg(),
			wantLen: 0,
		},
		{
			name:    "root array yields indexed items",
			input:   `["a", "b", "c"]`,
			cfg:     defaultJSONCfg(),
			wantLen: 3,
			check: func(t *testing.T, result map[string]string) {
				values := make(map[string]bool, len(result))
				for _, v := range result {
					values[v] = true
				}
				for _, want := range []string{"a", "b", "c"} {
					if !values[want] {
						t.Errorf("root array missing value %q (got %v)", want, result)
					}
				}
			},
		},
		{
			name:    "simple object",
			input:   `{"key1": "value1", "key2": "value2"}`,
			cfg:     defaultJSONCfg(),
			want:    map[string]string{"KEY1": "value1", "KEY2": "value2"},
			wantLen: 2,
		},
		{
			name:  "nested object",
			input: `{"database": {"host": "localhost", "port": 5432}}`,
			cfg:   defaultJSONCfg(),
			want:  map[string]string{"DATABASE_HOST": "localhost", "DATABASE_PORT": "5432"},
		},
		{
			name:  "array with underscore format",
			input: `{"items": ["one", "two", "three"]}`,
			cfg:   defaultJSONCfg(),
			want:  map[string]string{"ITEMS_0": "one", "ITEMS_1": "two", "ITEMS_2": "three"},
		},
		{
			name:  "array with bracket format",
			input: `{"items": ["a", "b", "c"]}`,
			cfg: func() JSONFlattenConfig {
				cfg := defaultJSONCfg()
				cfg.ArrayIndexFormat = "bracket"
				return cfg
			}(),
			want: map[string]string{"ITEMS[0]": "a", "ITEMS[1]": "b", "ITEMS[2]": "c"},
		},
		{
			name: "array of objects",
			input: `{"servers": [
				{"host": "server1", "port": 8080},
				{"host": "server2", "port": 9090}
			]}`,
			cfg: defaultJSONCfg(),
			want: map[string]string{
				"SERVERS_0_HOST": "server1",
				"SERVERS_0_PORT": "8080",
				"SERVERS_1_HOST": "server2",
				"SERVERS_1_PORT": "9090",
			},
			wantLen: 4,
		},
		{
			name: "nested array",
			input: `{"matrix": [
				["a", "b"],
				["c", "d"]
			]}`,
			cfg:     defaultJSONCfg(),
			want:    map[string]string{"MATRIX_0_0": "a", "MATRIX_0_1": "b", "MATRIX_1_0": "c", "MATRIX_1_1": "d"},
			wantLen: 4,
		},
		{
			name:  "null as empty",
			input: `{"key": null}`,
			cfg: func() JSONFlattenConfig {
				cfg := defaultJSONCfg()
				cfg.NullAsEmpty = true
				return cfg
			}(),
			want: map[string]string{"KEY": ""},
		},
		{
			name:  "null preserved",
			input: `{"key": null}`,
			cfg: func() JSONFlattenConfig {
				cfg := defaultJSONCfg()
				cfg.NullAsEmpty = false
				return cfg
			}(),
			want: map[string]string{"KEY": "null"},
		},
		{
			name:  "true as string",
			input: `{"enabled": true}`,
			cfg:   defaultJSONCfg(),
			want:  map[string]string{"ENABLED": "true"},
		},
		{
			name:  "false as string",
			input: `{"enabled": false}`,
			cfg:   defaultJSONCfg(),
			want:  map[string]string{"ENABLED": "false"},
		},
		{
			name:  "bool rendered when BoolAsString is off",
			input: `{"flag": true}`,
			cfg: func() JSONFlattenConfig {
				cfg := defaultJSONCfg()
				cfg.BoolAsString = false
				return cfg
			}(),
			want: map[string]string{"FLAG": "true"},
		},
		{
			name:  "integer",
			input: `{"count": 42}`,
			cfg:   defaultJSONCfg(),
			want:  map[string]string{"COUNT": "42"},
		},
		{
			name:  "float",
			input: `{"rate": 3.14}`,
			cfg:   defaultJSONCfg(),
			want:  map[string]string{"RATE": "3.14"},
		},
		{
			name:  "float as integer",
			input: `{"count": 42.0}`,
			cfg:   defaultJSONCfg(),
			want:  map[string]string{"COUNT": "42"},
		},
		{
			name:  "negative number",
			input: `{"temp": -10}`,
			cfg:   defaultJSONCfg(),
			want:  map[string]string{"TEMP": "-10"},
		},
		{
			name:  "number rendered when NumberAsString is off",
			input: `{"count": 42}`,
			cfg: func() JSONFlattenConfig {
				cfg := defaultJSONCfg()
				cfg.NumberAsString = false
				return cfg
			}(),
			want: map[string]string{"COUNT": "42"},
		},
		{
			name:    "empty object produces no entries",
			input:   `{"empty": {}}`,
			cfg:     defaultJSONCfg(),
			wantLen: 0,
		},
		{
			name:    "empty array produces no entries",
			input:   `{"empty": []}`,
			cfg:     defaultJSONCfg(),
			wantLen: 0,
		},
		{
			name:        "invalid JSON returns JSONError",
			input:       `{invalid json}`,
			cfg:         defaultJSONCfg(),
			wantErr:     true,
			wantJSONErr: true,
		},
		{
			name: "max depth exceeded returns error",
			input: `{
				"a": {
					"b": {
						"c": {
							"d": "deep"
						}
					}
				}
			}`,
			cfg: func() JSONFlattenConfig {
				cfg := defaultJSONCfg()
				cfg.MaxDepth = 2
				return cfg
			}(),
			wantErr: true,
		},
		{
			name: "complex structure",
			input: `{
				"app": {
					"name": "myapp",
					"version": "1.0.0",
					"features": ["auth", "logging"]
				},
				"database": {
					"host": "localhost",
					"port": 5432
				}
			}`,
			cfg: defaultJSONCfg(),
			want: map[string]string{
				"APP_NAME":       "myapp",
				"APP_VERSION":    "1.0.0",
				"APP_FEATURES_0": "auth",
				"APP_FEATURES_1": "logging",
				"DATABASE_HOST":  "localhost",
				"DATABASE_PORT":  "5432",
			},
			wantLen: 6,
		},
		{
			// Boundary: combined key length crosses the builder fast-path
			// threshold for nested keys.
			name:    "long nested key",
			input:   fmt.Sprintf(`{"%s": {"nested": "value"}}`, longKey),
			cfg:     defaultJSONCfg(),
			wantLen: 1,
			check: func(t *testing.T, result map[string]string) {
				for k := range result {
					if len(k) < 70 {
						t.Errorf("expected key length >= 70, got %d: %q", len(k), k)
					}
				}
			},
		},
		{
			name:    "long key with bracket format uses builder path",
			input:   fmt.Sprintf(`{"%s": ["a", "b"]}`, longKey),
			cfg:     func() JSONFlattenConfig { cfg := defaultJSONCfg(); cfg.ArrayIndexFormat = "bracket"; return cfg }(),
			wantLen: 2,
			check: func(t *testing.T, result map[string]string) {
				values := make(map[string]bool, len(result))
				for k, v := range result {
					if !strings.Contains(k, "[") {
						t.Errorf("expected bracket in key %q", k)
					}
					values[v] = true
				}
				for _, want := range []string{"a", "b"} {
					if !values[want] {
						t.Errorf("missing value %q (got %v)", want, result)
					}
				}
			},
		},
		{
			name:    "long key with underscore format uses builder path",
			input:   fmt.Sprintf(`{"%s": ["a"]}`, longKey),
			cfg:     defaultJSONCfg(),
			wantLen: 1,
			check: func(t *testing.T, result map[string]string) {
				for k := range result {
					if len(k) < 70 {
						t.Errorf("expected key length >= 70, got %d: %q", len(k), k)
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := FlattenJSON([]byte(tt.input), tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Fatalf("FlattenJSON() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantJSONErr {
				var jsonErr *JSONError
				if !errors.As(err, &jsonErr) {
					t.Fatalf("error type = %T, want *JSONError", err)
				}
				return
			}
			if tt.wantErr {
				return
			}
			for key, exp := range tt.want {
				if result[key] != exp {
					t.Errorf("result[%q] = %q, want %q", key, result[key], exp)
				}
			}
			// wantLen defaults to len(want) so a row that lists expected keys
			// also pins the result size without repeating the count.
			wantLen := tt.wantLen
			if wantLen == 0 && len(tt.want) > 0 {
				wantLen = len(tt.want)
			}
			if len(result) != wantLen {
				t.Errorf("len(result) = %d, want %d (got keys: %v)", len(result), wantLen, keysOf(result))
			}
			if tt.check != nil {
				tt.check(t, result)
			}
		})
	}
}

// ============================================================================
// buildKey Tests (JSON)
// ============================================================================

func TestBuildKey_JSON(t *testing.T) {
	cfg := JSONFlattenConfig{KeyDelimiter: "_"}

	tests := []struct {
		prefix   string
		key      string
		expected string
	}{
		{"", "key", "KEY"},
		{"APP", "key", "APP_KEY"},
		{"APP_DATABASE", "host", "APP_DATABASE_HOST"},
		{"", "lower", "LOWER"},
	}

	for _, tt := range tests {
		t.Run(tt.prefix+"_"+tt.key, func(t *testing.T) {
			result := buildKey(tt.prefix, tt.key, cfg)
			if result != tt.expected {
				t.Errorf("buildKey(%q, %q) = %q, want %q", tt.prefix, tt.key, result, tt.expected)
			}
		})
	}
}

// ============================================================================
// buildArrayIndex Tests (JSON)
// ============================================================================

func TestBuildArrayIndex_JSON(t *testing.T) {
	tests := []struct {
		name     string
		prefix   string
		index    int
		cfg      JSONFlattenConfig
		expected string
	}{
		{
			name:     "underscore format",
			prefix:   "ITEMS",
			index:    0,
			cfg:      JSONFlattenConfig{ArrayIndexFormat: "underscore", KeyDelimiter: "_"},
			expected: "ITEMS_0",
		},
		{
			name:     "bracket format",
			prefix:   "ITEMS",
			index:    5,
			cfg:      JSONFlattenConfig{ArrayIndexFormat: "bracket"},
			expected: "ITEMS[5]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildArrayIndex(tt.prefix, tt.index, tt.cfg)
			if result != tt.expected {
				t.Errorf("buildArrayIndex(%q, %d) = %q, want %q", tt.prefix, tt.index, result, tt.expected)
			}
		})
	}
}

// ============================================================================
// Depth Propagation, Pre-Scanner & Default-Branch Coverage
// ============================================================================

// TestFlattenJSON_DepthErrorPropagation pins the fail-late depth contract:
// inputs whose bracket nesting passes the conservative pre-scan but whose
// value depth still reaches MaxDepth must surface a *JSONError from
// flattenValue, propagated through the container loops back to FlattenJSON.
func TestFlattenJSON_DepthErrorPropagation(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"error propagates through map loops", `{"a":{"b":1}}`},
		{"error propagates through array loops", `[["x"]]`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := defaultJSONCfg()
			cfg.MaxDepth = 2
			_, err := FlattenJSON([]byte(tt.input), cfg)
			if err == nil {
				t.Fatalf("FlattenJSON(%s) error = nil, want depth error", tt.input)
			}
			var jsonErr *JSONError
			if !errors.As(err, &jsonErr) {
				t.Errorf("error = %T, want *JSONError", err)
			}
		})
	}
}

// TestFlattenJSON_RootBoolAndNumber pins the root-scalar contract for bool
// and number inputs: like root strings and null, they contribute no keys
// and no error.
func TestFlattenJSON_RootBoolAndNumber(t *testing.T) {
	for _, input := range []string{"true", "42"} {
		result, err := FlattenJSON([]byte(input), defaultJSONCfg())
		if err != nil {
			t.Errorf("FlattenJSON(%s) error = %v, want nil", input, err)
		}
		if len(result) != 0 {
			t.Errorf("FlattenJSON(%s) = %v, want empty map", input, result)
		}
	}
}

// TestScanJSONLimits_MalformedAndStrings covers the pre-scanner's
// tolerance branches: negative nesting clamps to zero, and string bodies
// (including escaped quotes) are skipped so brackets inside JSON strings
// never count toward the depth or the node count.
func TestScanJSONLimits_MalformedAndStrings(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"unbalanced close clamps to zero", `{"a":1}]`, false},
		{"bracket inside string ignored", `{"k": "a[b"}`, false},
		{"escaped quote does not end string", `{"a\":\"b": 1}`, false},
		{"deep real nesting detected", `[[[[[[1]]]]]]`, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, _ := scanJSONLimits([]byte(tt.input), 0, 5, HardMaxJSONNodes); got != tt.want {
				t.Errorf("scanJSONLimits(%s) depth = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// TestScanJSONLimits_NodeCount verifies the node cap path independently of
// the global HardMaxJSONNodes constant.
func TestScanJSONLimits_NodeCount(t *testing.T) {
	// `[1,2,3]` = 1 opening bracket + 2 commas = 3 nodes.
	if _, nodes := scanJSONLimits([]byte(`[1,2,3]`), 0, 10, 2); !nodes {
		t.Error("nodes=true want exceeded for 3 nodes against cap 2")
	}
	if _, nodes := scanJSONLimits([]byte(`[1,2,3]`), 0, 10, 3); nodes {
		t.Error("nodes=false want ok for 3 nodes against cap 3")
	}
	// Brackets and commas inside strings do not count.
	if _, nodes := scanJSONLimits([]byte(`"[,,"`), 0, 10, 2); nodes {
		t.Error("string contents must not count toward the node cap")
	}
}

// TestFlattenJSON_NodeCap (SEC-02): documents whose structural node count
// exceeds HardMaxJSONNodes are rejected by the pre-scan before
// json.Unmarshal materializes a parse tree disproportionate to the input.
func TestFlattenJSON_NodeCap(t *testing.T) {
	// Each "[]," contributes 2 nodes (open bracket + comma).
	input := "[" + strings.Repeat("[],", HardMaxJSONNodes/2+1) + "]"
	_, err := FlattenJSON([]byte(input), defaultJSONCfg())
	if err == nil {
		t.Fatal("expected node cap error")
	}
	var je *JSONError
	if !errors.As(err, &je) {
		t.Fatalf("want *JSONError, got %T: %v", err, err)
	}
	if !strings.Contains(err.Error(), "node count") {
		t.Errorf("error should mention node count: %v", err)
	}
}

// TestFlattenValue_UnsupportedType pins flattenValue's terminal default
// branch with a type json.Unmarshal never produces.
func TestFlattenValue_UnsupportedType(t *testing.T) {
	err := flattenValue(complex128(1+2i), "K", defaultJSONCfg(), map[string]string{}, 0)
	if err == nil {
		t.Fatal("flattenValue(complex128) error = nil, want error")
	}
	var jsonErr *JSONError
	if !errors.As(err, &jsonErr) {
		t.Errorf("error = %T, want *JSONError", err)
	}
}
