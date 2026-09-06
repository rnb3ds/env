package internal

import (
	"errors"
	"sort"
	"strings"
	"testing"
)

// ============================================================================
// YAML Flatten Tests
// ============================================================================

// defaultFlattenCfg returns the standard flatten configuration used by most
// cases; individual tests override fields as needed.
func defaultFlattenCfg() YAMLFlattenConfig {
	return YAMLFlattenConfig{
		KeyDelimiter:     "_",
		ArrayIndexFormat: "underscore",
		NullAsEmpty:      true,
		NumberAsString:   true,
		BoolAsString:     true,
		MaxDepth:         10,
	}
}

// bracketFlattenCfg returns the standard configuration with bracket array
// indexing ("KEY[0]" instead of "KEY_0").
func bracketFlattenCfg() YAMLFlattenConfig {
	cfg := defaultFlattenCfg()
	cfg.ArrayIndexFormat = "bracket"
	return cfg
}

func TestFlattenYAML(t *testing.T) {
	tests := []struct {
		name    string
		value   *Value
		cfg     YAMLFlattenConfig
		want    map[string]string // keys asserted when non-nil
		wantLen int               // expected len(result); -1 skips the check
		wantErr bool
		check   func(t *testing.T, result map[string]string) // extra shape assertions
	}{
		{
			name:    "nil value yields empty map",
			value:   nil,
			cfg:     defaultFlattenCfg(),
			wantLen: 0,
		},
		{
			name: "simple map",
			value: &Value{
				Type: ValueTypeMap,
				Map: map[string]*Value{
					"KEY1": NewScalarValue("value1", 1, 1),
					"KEY2": NewScalarValue("value2", 1, 1),
				},
			},
			cfg:     defaultFlattenCfg(),
			want:    map[string]string{"KEY1": "value1", "KEY2": "value2"},
			wantLen: 2,
		},
		{
			name: "nested map",
			value: &Value{
				Type: ValueTypeMap,
				Map: map[string]*Value{
					"DATABASE": {
						Type: ValueTypeMap,
						Map: map[string]*Value{
							"HOST": NewScalarValue("localhost", 2, 3),
							"PORT": NewScalarValue("5432", 2, 3),
						},
					},
				},
			},
			cfg:     defaultFlattenCfg(),
			want:    map[string]string{"DATABASE_HOST": "localhost", "DATABASE_PORT": "5432"},
			wantLen: 2,
		},
		{
			name: "array with underscore format",
			value: &Value{
				Type: ValueTypeMap,
				Map: map[string]*Value{
					"ITEMS": {
						Type: ValueTypeArray,
						Array: []*Value{
							NewScalarValue("one", 1, 1),
							NewScalarValue("two", 1, 1),
							NewScalarValue("three", 1, 1),
						},
					},
				},
			},
			cfg:     defaultFlattenCfg(),
			want:    map[string]string{"ITEMS_0": "one", "ITEMS_1": "two", "ITEMS_2": "three"},
			wantLen: 3,
		},
		{
			name: "array with bracket format",
			value: &Value{
				Type: ValueTypeMap,
				Map: map[string]*Value{
					"ITEMS": {
						Type: ValueTypeArray,
						Array: []*Value{
							NewScalarValue("one", 1, 1),
							NewScalarValue("two", 1, 1),
						},
					},
				},
			},
			cfg:     bracketFlattenCfg(),
			want:    map[string]string{"ITEMS[0]": "one", "ITEMS[1]": "two"},
			wantLen: 2,
		},
		{
			name: "long key with bracket format uses builder path",
			value: &Value{
				Type: ValueTypeMap,
				Map: map[string]*Value{
					strings.Repeat("K", 70): {
						Type:  ValueTypeArray,
						Array: []*Value{NewScalarValue("a", 1, 1), NewScalarValue("b", 1, 1)},
					},
				},
			},
			cfg:     bracketFlattenCfg(),
			wantLen: 2,
			check: func(t *testing.T, result map[string]string) {
				for k := range result {
					if !strings.Contains(k, "[") {
						t.Errorf("expected bracket in key %q", k)
					}
				}
			},
		},
		{
			name: "long key with underscore format uses builder path",
			value: &Value{
				Type: ValueTypeMap,
				Map: map[string]*Value{
					strings.Repeat("K", 70): {
						Type:  ValueTypeArray,
						Array: []*Value{NewScalarValue("a", 1, 1)},
					},
				},
			},
			cfg:     defaultFlattenCfg(),
			wantLen: 1,
			check: func(t *testing.T, result map[string]string) {
				for k := range result {
					if !strings.HasSuffix(k, "_0") {
						t.Errorf("expected underscore index suffix in key %q", k)
					}
				}
			},
		},
		{
			name: "empty nested map flattens to empty string",
			value: &Value{
				Type: ValueTypeMap,
				Map: map[string]*Value{
					"EMPTY": {Type: ValueTypeMap, Map: map[string]*Value{}},
				},
			},
			cfg:     defaultFlattenCfg(),
			want:    map[string]string{"EMPTY": ""},
			wantLen: 1,
		},
		{
			name: "empty array flattens to empty string",
			value: &Value{
				Type: ValueTypeMap,
				Map: map[string]*Value{
					"EMPTY": {Type: ValueTypeArray, Array: []*Value{}},
				},
			},
			cfg:     defaultFlattenCfg(),
			want:    map[string]string{"EMPTY": ""},
			wantLen: 1,
		},
		{
			name: "max depth exceeded returns error",
			value: &Value{
				Type: ValueTypeMap,
				Map: map[string]*Value{
					"A": {
						Type: ValueTypeMap,
						Map: map[string]*Value{
							"B": {
								Type: ValueTypeMap,
								Map: map[string]*Value{
									"C": NewScalarValue("deep", 1, 1),
								},
							},
						},
					},
				},
			},
			cfg: func() YAMLFlattenConfig {
				cfg := defaultFlattenCfg()
				cfg.MaxDepth = 2
				return cfg
			}(),
			wantErr: true,
		},
		{
			name:    "scalar with no prefix is dropped",
			value:   NewScalarValue("standalone", 1, 1),
			cfg:     defaultFlattenCfg(),
			wantLen: 0,
		},
		{
			name: "nil value in map is skipped",
			value: &Value{
				Type: ValueTypeMap,
				Map: map[string]*Value{
					"KEY": nil,
				},
			},
			cfg:     defaultFlattenCfg(),
			wantLen: 0,
		},
		{
			name: "nil element in array keeps sibling indexes",
			value: &Value{
				Type: ValueTypeMap,
				Map: map[string]*Value{
					"ITEMS": {
						Type: ValueTypeArray,
						Array: []*Value{
							NewScalarValue("one", 1, 1),
							nil,
							NewScalarValue("three", 1, 1),
						},
					},
				},
			},
			cfg:     defaultFlattenCfg(),
			want:    map[string]string{"ITEMS_0": "one", "ITEMS_2": "three"},
			wantLen: 2,
		},
		{
			name: "long nested key exceeds inline builder threshold",
			value: &Value{
				Type: ValueTypeMap,
				Map: map[string]*Value{
					"a" + strings.Repeat("b", 70): {
						Type: ValueTypeMap,
						Map: map[string]*Value{
							"nested": NewScalarValue("value", 1, 1),
						},
					},
				},
			},
			cfg:     defaultFlattenCfg(),
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
			name: "float is preserved when NumberAsString is off",
			value: &Value{
				Type: ValueTypeMap,
				Map: map[string]*Value{
					"pi": NewScalarValue("3.14", 1, 1),
				},
			},
			cfg: func() YAMLFlattenConfig {
				cfg := defaultFlattenCfg()
				cfg.NumberAsString = false
				return cfg
			}(),
			want:    map[string]string{"PI": "3.14"},
			wantLen: 1,
		},
		{
			name: "complex structure",
			value: &Value{
				Type: ValueTypeMap,
				Map: map[string]*Value{
					"APP": {
						Type: ValueTypeMap,
						Map: map[string]*Value{
							"NAME":    NewScalarValue("myapp", 1, 1),
							"VERSION": NewScalarValue("1.0.0", 1, 1),
							"FEATURES": {
								Type: ValueTypeArray,
								Array: []*Value{
									NewScalarValue("auth", 1, 1),
									NewScalarValue("logging", 1, 1),
								},
							},
						},
					},
					"DATABASE": {
						Type: ValueTypeMap,
						Map: map[string]*Value{
							"HOST": NewScalarValue("localhost", 1, 1),
							"PORT": NewScalarValue("5432", 1, 1),
						},
					},
				},
			},
			cfg: defaultFlattenCfg(),
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := FlattenYAML(tt.value, tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Fatalf("FlattenYAML() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if err != nil {
				t.Fatalf("FlattenYAML() error = %v", err)
			}
			for key, exp := range tt.want {
				if result[key] != exp {
					t.Errorf("result[%q] = %q, want %q", key, result[key], exp)
				}
			}
			if tt.wantLen >= 0 && len(result) != tt.wantLen {
				t.Errorf("len(result) = %d, want %d (got keys: %v)", len(result), tt.wantLen, keysOf(result))
			}
			if tt.check != nil {
				tt.check(t, result)
			}
		})
	}
}

// keysOf returns the sorted keys of m for failure messages.
func keysOf(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// ============================================================================
// Inline JSON Tests
// ============================================================================

func TestFlattenYAML_InlineJSON(t *testing.T) {
	tests := []struct {
		name    string
		scalar  string // inline JSON scalar stored under "key"
		cfg     YAMLFlattenConfig
		want    map[string]string
		wantErr bool
	}{
		{
			name:   "array",
			scalar: `["a", "b", "c"]`,
			cfg:    defaultFlattenCfg(),
			want: map[string]string{
				"KEY_0": "a", "KEY_1": "b", "KEY_2": "c",
			},
		},
		{
			name:   "object",
			scalar: `{"host": "localhost", "port": 8080}`,
			cfg:    defaultFlattenCfg(),
			want: map[string]string{
				"KEY_HOST": "localhost", "KEY_PORT": "8080",
			},
		},
		{
			name:   "nested object",
			scalar: `{"a": {"b": "deep"}}`,
			cfg:    defaultFlattenCfg(),
			want:   map[string]string{"KEY_A_B": "deep"},
		},
		{
			name:   "array of objects",
			scalar: `[{"host": "a"}, {"host": "b"}]`,
			cfg:    defaultFlattenCfg(),
			want:   map[string]string{"KEY_0_HOST": "a", "KEY_1_HOST": "b"},
		},
		{
			name:   "null preserved when NullAsEmpty is off",
			scalar: `{"sub": null}`,
			cfg: func() YAMLFlattenConfig {
				cfg := defaultFlattenCfg()
				cfg.NullAsEmpty = false
				return cfg
			}(),
			want: map[string]string{"KEY_SUB": "null"},
		},
		{
			name:   "bool rendered when BoolAsString is off",
			scalar: `{"flag": true}`,
			cfg: func() YAMLFlattenConfig {
				cfg := defaultFlattenCfg()
				cfg.BoolAsString = false
				return cfg
			}(),
			want: map[string]string{"KEY_FLAG": "true"},
		},
		{
			name:   "number rendered when NumberAsString is off",
			scalar: `{"count": 42}`,
			cfg: func() YAMLFlattenConfig {
				cfg := defaultFlattenCfg()
				cfg.NumberAsString = false
				return cfg
			}(),
			want: map[string]string{"KEY_COUNT": "42"},
		},
		{
			name:   "float is not stringified when NumberAsString is off",
			scalar: `{"ratio": 1.5}`,
			cfg: func() YAMLFlattenConfig {
				cfg := defaultFlattenCfg()
				cfg.NumberAsString = false
				return cfg
			}(),
			want: map[string]string{"KEY_RATIO": "1.5"},
		},
		{
			name:   "invalid JSON treated as regular scalar",
			scalar: `[not valid json]`,
			cfg:    defaultFlattenCfg(),
			want:   map[string]string{"KEY": "[not valid json]"},
		},
		{
			// Depth-limit hits inside inline JSON propagate as errors (the
			// fail-fast pre-scan trips at startDepth+nesting > MaxDepth);
			// only syntax failures fall back to the plain-scalar treatment.
			name:   "depth overflow inside inline JSON errors",
			scalar: `{"a": {"b": {"c": "too deep"}}}`,
			cfg: func() YAMLFlattenConfig {
				cfg := defaultFlattenCfg()
				cfg.MaxDepth = 3
				return cfg
			}(),
			wantErr: true,
		},
		{
			// Boundary: startDepth+nesting == MaxDepth passes the fail-fast
			// pre-check but still trips flattenInlineValue's own depth check
			// (depth >= MaxDepth) one level deeper — and that error now
			// propagates instead of degrading to the raw scalar.
			name:   "exact boundary depth trips recursive check after pre-check passes",
			scalar: `{"a": "b"}`,
			cfg: func() YAMLFlattenConfig {
				cfg := defaultFlattenCfg()
				cfg.MaxDepth = 2
				return cfg
			}(),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value := &Value{
				Type: ValueTypeMap,
				Map: map[string]*Value{
					"key": NewScalarValue(tt.scalar, 1, 1),
				},
			}

			result, err := FlattenYAML(value, tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Fatalf("FlattenYAML() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			for key, exp := range tt.want {
				if result[key] != exp {
					t.Errorf("result[%q] = %q, want %q", key, result[key], exp)
				}
			}
		})
	}
}

// ============================================================================
// convertYAMLScalar Tests
// ============================================================================

func TestConvertYAMLScalar(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		cfg      YAMLFlattenConfig
		expected string
	}{
		{
			name:     "null as empty",
			input:    "null",
			cfg:      YAMLFlattenConfig{NullAsEmpty: true},
			expected: "",
		},
		{
			name:     "null preserved",
			input:    "null",
			cfg:      YAMLFlattenConfig{NullAsEmpty: false},
			expected: "null",
		},
		{
			name:     "tilde as empty",
			input:    "~",
			cfg:      YAMLFlattenConfig{NullAsEmpty: true},
			expected: "",
		},
		{
			name:     "true bool",
			input:    "true",
			cfg:      YAMLFlattenConfig{BoolAsString: true},
			expected: "true",
		},
		{
			name:     "false bool",
			input:    "false",
			cfg:      YAMLFlattenConfig{BoolAsString: true},
			expected: "false",
		},
		{
			name:     "integer",
			input:    "42",
			cfg:      YAMLFlattenConfig{NumberAsString: true},
			expected: "42",
		},
		{
			name:     "float",
			input:    "3.14",
			cfg:      YAMLFlattenConfig{NumberAsString: true},
			expected: "3.14",
		},
		{
			name:     "float as int",
			input:    "42.0",
			cfg:      YAMLFlattenConfig{NumberAsString: true},
			expected: "42",
		},
		{
			name:     "negative number",
			input:    "-42",
			cfg:      YAMLFlattenConfig{NumberAsString: true},
			expected: "-42",
		},
		{
			name:     "regular string",
			input:    "hello world",
			cfg:      YAMLFlattenConfig{},
			expected: "hello world",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertYAMLScalar(tt.input, tt.cfg)
			if result != tt.expected {
				t.Errorf("convertYAMLScalar(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// ============================================================================
// buildYAMLKey Tests
// ============================================================================

func TestBuildYAMLKey(t *testing.T) {
	cfg := YAMLFlattenConfig{KeyDelimiter: "_"}

	tests := []struct {
		prefix   string
		key      string
		expected string
	}{
		{"", "KEY", "KEY"},
		{"APP", "KEY", "APP_KEY"},
		{"APP_DATABASE", "HOST", "APP_DATABASE_HOST"},
		{"", "lower", "LOWER"},
		{"prefix", "MixedCase", "prefix_MIXEDCASE"}, // prefix is not uppercased, only key is
		// Boundary: combined length crosses the fast-path threshold.
		{strings.Repeat("P", 60), strings.Repeat("K", 20), strings.Repeat("P", 60) + "_" + strings.Repeat("K", 20)},
	}

	for _, tt := range tests {
		t.Run(tt.prefix+"_"+tt.key, func(t *testing.T) {
			result := buildYAMLKey(tt.prefix, tt.key, cfg)
			if result != tt.expected {
				t.Errorf("buildYAMLKey(%q, %q) = %q, want %q", tt.prefix, tt.key, result, tt.expected)
			}
		})
	}
}

// ============================================================================
// buildYAMLArrayIndex Tests
// ============================================================================

func TestBuildYAMLArrayIndex(t *testing.T) {
	longPrefix := strings.Repeat("K", 70)

	tests := []struct {
		name     string
		prefix   string
		index    int
		cfg      YAMLFlattenConfig
		expected string
	}{
		{
			name:     "underscore format",
			prefix:   "ITEMS",
			index:    0,
			cfg:      YAMLFlattenConfig{ArrayIndexFormat: "underscore", KeyDelimiter: "_"},
			expected: "ITEMS_0",
		},
		{
			name:     "bracket format",
			prefix:   "ITEMS",
			index:    5,
			cfg:      YAMLFlattenConfig{ArrayIndexFormat: "bracket"},
			expected: "ITEMS[5]",
		},
		{
			name:     "nested underscore",
			prefix:   "SERVERS_PORTS",
			index:    2,
			cfg:      YAMLFlattenConfig{ArrayIndexFormat: "underscore", KeyDelimiter: "_"},
			expected: "SERVERS_PORTS_2",
		},
		{
			// Boundary: >64 chars forces the builder slow path.
			name:     "underscore format long prefix uses builder",
			prefix:   longPrefix,
			index:    7,
			cfg:      YAMLFlattenConfig{ArrayIndexFormat: "underscore", KeyDelimiter: "_"},
			expected: longPrefix + "_7",
		},
		{
			// Boundary: >64 chars forces the builder slow path.
			name:     "bracket format long prefix uses builder",
			prefix:   longPrefix,
			index:    9,
			cfg:      YAMLFlattenConfig{ArrayIndexFormat: "bracket"},
			expected: longPrefix + "[9]",
		},
		{
			// Boundary: exactly 64 chars stays on the fast path.
			name:     "bracket format at threshold stays fast",
			prefix:   strings.Repeat("K", 61),
			index:    1,
			cfg:      YAMLFlattenConfig{ArrayIndexFormat: "bracket"},
			expected: strings.Repeat("K", 61) + "[1]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildYAMLArrayIndex(tt.prefix, tt.index, tt.cfg)
			if result != tt.expected {
				t.Errorf("buildYAMLArrayIndex(%q, %d) = %q, want %q", tt.prefix, tt.index, result, tt.expected)
			}
		})
	}
}

// TestFlattenYAML_QuotedScalarsStayStrings verifies YAML semantics on the read
// path: a quoted scalar is always a string — no null/bool/number coercion and
// no whitespace stripping — even when NullAsEmpty/BoolAsString are enabled.
// Regression: the quoting flag was dropped between lexer and Value tree, so
// `MODE: "null"` read back as "" and `SPACED: "trailing "` lost its space.
func TestFlattenYAML_QuotedScalarsStayStrings(t *testing.T) {
	input := `MODE: "null"
EMPTY_WORD: "~"
FLAG: "true"
RATIO: "3.14"
SPACED: "trailing space "
PLAIN_NULL: null
`

	value, err := ParseYAML([]byte(input), 10)
	if err != nil {
		t.Fatalf("ParseYAML() error = %v", err)
	}
	defer ReleaseValue(value)

	result, err := FlattenYAML(value, defaultFlattenCfg())
	if err != nil {
		t.Fatalf("FlattenYAML() error = %v", err)
	}

	want := map[string]string{
		"MODE":       "null",
		"EMPTY_WORD": "~",
		"FLAG":       "true",
		"RATIO":      "3.14",
		"SPACED":     "trailing space ",
		"PLAIN_NULL": "", // plain null still coerces per NullAsEmpty
	}
	for key, expected := range want {
		if got := result[key]; got != expected {
			t.Errorf("result[%q] = %q, want %q", key, got, expected)
		}
	}
}

// TestEstimateLeafCount pins the one-level-deep sizing heuristic used to
// pre-allocate the result map.
func TestEstimateLeafCount(t *testing.T) {
	tests := []struct {
		name string
		v    *Value
		want int
	}{
		{"nil", nil, 0},
		{"scalar", NewScalarValue("x", 1, 1), 1},
		{
			"map counts entries",
			&Value{Type: ValueTypeMap, Map: map[string]*Value{
				"A": NewScalarValue("1", 1, 1),
				"B": NewScalarValue("2", 1, 1),
				"C": NewScalarValue("3", 1, 1),
			}},
			3,
		},
		{
			"array counts elements",
			&Value{Type: ValueTypeArray, Array: []*Value{
				NewScalarValue("1", 1, 1),
				NewScalarValue("2", 1, 1),
			}},
			2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := estimateLeafCount(tt.v); got != tt.want {
				t.Errorf("estimateLeafCount() = %d, want %d", got, tt.want)
			}
		})
	}
}

// TestLooksLikeNumber pins the cheap numeric pre-filter: it must accept
// anything strconv can parse (digits, sign, exponent, dot) and reject
// everything else without allocating.
func TestLooksLikeNumber(t *testing.T) {
	tests := []struct {
		s    string
		want bool
	}{
		{"", false},
		{"+", false},
		{"-", false},
		{".", false},
		{"abc", false},
		{"1a", false},
		{"0x1F", false},
		{"1_000", false},
		{"0", true},
		{"42", true},
		{"-42", true},
		{"+1.5", true},
		{"1.5e3", true},
		{"-2E-2", true},
		// "e5" passes the pre-filter (charset + digit) but is rejected by
		// strconv downstream — the filter only guarantees "cannot be a number"
		// rejections, not full validation.
		{"e5", true},
	}

	for _, tt := range tests {
		if got := looksLikeNumber(tt.s); got != tt.want {
			t.Errorf("looksLikeNumber(%q) = %v, want %v", tt.s, got, tt.want)
		}
	}
}

// TestFlattenInlineJSON_Boundary drives the inline-JSON handling of unquoted
// YAML scalars: null rendering under both NullAsEmpty settings, number
// formatting under NumberAsString, nested containers, and the depth limit.
func TestFlattenInlineJSON_Boundary(t *testing.T) {
	inlineScalar := func(s string) *Value {
		return &Value{Type: ValueTypeMap, Map: map[string]*Value{
			"DATA": {Type: ValueTypeScalar, Scalar: s},
		}}
	}

	t.Run("mixed array elements", func(t *testing.T) {
		result, err := FlattenYAML(inlineScalar(`[1, "a", null, true, 1.5]`), defaultFlattenCfg())
		if err != nil {
			t.Fatalf("FlattenYAML() error = %v", err)
		}
		want := map[string]string{
			"DATA_0": "1",
			"DATA_1": "a",
			"DATA_2": "", // null with NullAsEmpty=true
			"DATA_3": "true",
			"DATA_4": "1.5",
		}
		for k, v := range want {
			if got := result[k]; got != v {
				t.Errorf("result[%q] = %q, want %q", k, got, v)
			}
		}
	})

	t.Run("null rendered as literal when NullAsEmpty disabled", func(t *testing.T) {
		cfg := defaultFlattenCfg()
		cfg.NullAsEmpty = false
		result, err := FlattenYAML(inlineScalar(`[null]`), cfg)
		if err != nil {
			t.Fatalf("FlattenYAML() error = %v", err)
		}
		if got := result["DATA_0"]; got != "null" {
			t.Errorf("result[DATA_0] = %q, want %q", got, "null")
		}
	})

	t.Run("int-like float formatted without decimal point", func(t *testing.T) {
		cfg := defaultFlattenCfg()
		cfg.NumberAsString = true
		result, err := FlattenYAML(inlineScalar(`[2.0, 3.25]`), cfg)
		if err != nil {
			t.Fatalf("FlattenYAML() error = %v", err)
		}
		if got := result["DATA_0"]; got != "2" {
			t.Errorf("result[DATA_0] = %q, want %q", got, "2")
		}
		if got := result["DATA_1"]; got != "3.25" {
			t.Errorf("result[DATA_1] = %q, want %q", got, "3.25")
		}
	})

	t.Run("float formatted via fmt when NumberAsString disabled", func(t *testing.T) {
		cfg := defaultFlattenCfg()
		cfg.NumberAsString = false
		result, err := FlattenYAML(inlineScalar(`[2.5]`), cfg)
		if err != nil {
			t.Fatalf("FlattenYAML() error = %v", err)
		}
		if got := result["DATA_0"]; got != "2.5" {
			t.Errorf("result[DATA_0] = %q, want %q", got, "2.5")
		}
	})

	t.Run("nested object inside array", func(t *testing.T) {
		result, err := FlattenYAML(inlineScalar(`[{"host": "db", "port": 5432}]`), defaultFlattenCfg())
		if err != nil {
			t.Fatalf("FlattenYAML() error = %v", err)
		}
		if got := result["DATA_0_HOST"]; got != "db" {
			t.Errorf("result[DATA_0_HOST] = %q, want %q", got, "db")
		}
		if got := result["DATA_0_PORT"]; got != "5432" {
			t.Errorf("result[DATA_0_PORT] = %q, want %q", got, "5432")
		}
	})

	t.Run("deeply nested inline JSON errors on depth limit", func(t *testing.T) {
		// Depth-limit hits propagate from inline JSON; only syntax failures
		// fall back to the literal-scalar treatment (see below).
		cfg := defaultFlattenCfg()
		cfg.MaxDepth = 3
		result, err := FlattenYAML(inlineScalar(`[[[1]]]`), cfg)
		if err == nil {
			t.Fatalf("FlattenYAML() error = nil, want depth error (result %v)", result)
		}
		var yamlErr *YAMLError
		if !errors.As(err, &yamlErr) {
			t.Errorf("error = %T, want *YAMLError", err)
		}
	})

	t.Run("malformed inline JSON falls back to scalar", func(t *testing.T) {
		// Starts like inline JSON but does not parse: kept as a literal scalar.
		result, err := FlattenYAML(inlineScalar(`[not json`), defaultFlattenCfg())
		if err != nil {
			t.Fatalf("FlattenYAML() error = %v", err)
		}
		if got, ok := result["DATA"]; !ok || got != "[not json" {
			t.Errorf("result[DATA] = %q (present=%v), want the literal scalar", got, ok)
		}
	})
}

// ============================================================================
// Scalar Conversion, Depth Propagation & Inline-JSON Branches
// ============================================================================

// TestConvertYAMLScalar_ConfigBranches covers the empty-scalar rendering and
// the boolean-casing branches: with BoolAsString=false the scalar's original
// casing is preserved ("TRUE" stays "TRUE"); empty scalars render per
// NullAsEmpty.
func TestConvertYAMLScalar_ConfigBranches(t *testing.T) {
	tests := []struct {
		name                      string
		in                        string
		nullAsEmpty, boolAsString bool
		want                      string
	}{
		{"empty renders null when NullAsEmpty=false", "", false, false, "null"},
		{"empty renders empty when NullAsEmpty=true", "", true, false, ""},
		{"TRUE keeps case when BoolAsString=false", "TRUE", true, false, "TRUE"},
		{"true canonical when BoolAsString=true", "true", true, true, "true"},
		{"False keeps case when BoolAsString=false", "False", true, false, "False"},
		{"false canonical when BoolAsString=true", "false", true, true, "false"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := defaultFlattenCfg()
			cfg.NullAsEmpty = tt.nullAsEmpty
			cfg.BoolAsString = tt.boolAsString
			if got := convertYAMLScalar(tt.in, cfg); got != tt.want {
				t.Errorf("convertYAMLScalar(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestFlattenYAML_ArrayDepthErrorPropagation exercises the array-loop error
// path: an element whose flattening hits MaxDepth must propagate through
// every enclosing array loop.
func TestFlattenYAML_ArrayDepthErrorPropagation(t *testing.T) {
	cfg := defaultFlattenCfg()
	cfg.MaxDepth = 2

	inner := NewArrayValue(1, 1)
	inner.Array = append(inner.Array, NewScalarValue("x", 1, 1))
	mid := NewArrayValue(1, 1)
	mid.Array = append(mid.Array, inner)
	root := NewArrayValue(1, 1)
	root.Array = append(root.Array, mid)

	_, err := FlattenYAML(root, cfg)
	if err == nil {
		t.Fatal("FlattenYAML(triple-nested array) error = nil, want depth error")
	}
	var yamlErr *YAMLError
	if !errors.As(err, &yamlErr) {
		t.Errorf("error = %T, want *YAMLError", err)
	}
}

// TestFlattenInlineJSON_BranchCoverage white-box covers the inline-JSON
// helper: scalar rendering through the default branch plus array/map loop
// error propagation.
func TestFlattenInlineJSON_BranchCoverage(t *testing.T) {
	t.Run("scalar input renders through default branch", func(t *testing.T) {
		result := map[string]string{}
		if err := flattenInlineJSON(`"text"`, "K", defaultFlattenCfg(), result, 0); err != nil {
			t.Fatalf("flattenInlineJSON(scalar) error = %v", err)
		}
		if result["K"] != "text" {
			t.Errorf("result[K] = %q, want %q", result["K"], "text")
		}
	})

	cfg := defaultFlattenCfg()
	cfg.MaxDepth = 2

	t.Run("array item error propagates", func(t *testing.T) {
		result := map[string]string{}
		if err := flattenInlineJSON(`[["x"]]`, "K", cfg, result, 0); err == nil {
			t.Error(`flattenInlineJSON([["x"]]) error = nil, want depth error`)
		}
	})

	t.Run("map value error propagates", func(t *testing.T) {
		result := map[string]string{}
		if err := flattenInlineJSON(`{"a":{"b":"x"}}`, "K", cfg, result, 0); err == nil {
			t.Error("flattenInlineJSON(nested map) error = nil, want depth error")
		}
	})
}

// TestFlattenInlineValue_UnsupportedType pins flattenInlineValue's terminal
// default branch with a type json.Unmarshal never produces.
func TestFlattenInlineValue_UnsupportedType(t *testing.T) {
	result := map[string]string{}
	if err := flattenInlineValue(42, "K", defaultFlattenCfg(), result, 0); err != nil {
		t.Fatalf("flattenInlineValue(42) error = %v, want nil", err)
	}
	if result["K"] != "42" {
		t.Errorf("result[K] = %q, want %q", result["K"], "42")
	}
}
