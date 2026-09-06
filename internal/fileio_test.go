package internal

import (
	"strings"
	"testing"
)

// TestMarshalEnv_ValueEscaping pins escapeValueToBuilder's quoting rules via
// the public MarshalEnv path (single-key map; each output line is "K=<escaped>\n").
func TestMarshalEnv_ValueEscaping(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple value", "simple", "simple"},
		{"value with space", "value with space", `"value with space"`},
		{"value with newline", "line1\nline2", `"line1\nline2"`},
		{"value with tab", "col1\tcol2", `"col1\tcol2"`},
		{"value with quote", `say "hello"`, `"say \"hello\""`},
		{"value with hash", "value#comment", `"value#comment"`},
		// A lone backslash neither triggers quoting nor is escaped.
		{"value with backslash only", `path\to\file`, `path\to\file`},
		{"empty value", "", `""`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := MarshalEnv(map[string]string{"K": tt.input}, false)
			if err != nil {
				t.Errorf("MarshalEnv() error = %v", err)
				return
			}
			if want := "K=" + tt.expected + "\n"; result != want {
				t.Errorf("MarshalEnv() = %q, want %q", result, want)
			}
		})
	}
}

func TestMarshalEnv(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]string
		sorted   bool
		contains []string
	}{
		{
			name: "simple map",
			input: map[string]string{
				"KEY1": "value1",
				"KEY2": "value2",
			},
			sorted:   false,
			contains: []string{"KEY1=value1", "KEY2=value2"},
		},
		{
			name: "sorted output",
			input: map[string]string{
				"B_KEY": "b",
				"A_KEY": "a",
			},
			sorted:   true,
			contains: []string{"A_KEY=a", "B_KEY=b"},
		},
		{
			name:     "empty map",
			input:    map[string]string{},
			sorted:   false,
			contains: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := MarshalEnv(tt.input, tt.sorted)
			if err != nil {
				t.Errorf("MarshalEnv() error = %v", err)
				return
			}
			for _, c := range tt.contains {
				if !strings.Contains(result, c) {
					t.Errorf("MarshalEnv() should contain %q, got %q", c, result)
				}
			}
		})
	}
}

func TestMarshalEnvSorted(t *testing.T) {
	input := map[string]string{
		"Z_KEY": "z",
		"A_KEY": "a",
		"M_KEY": "m",
	}

	result, err := MarshalEnv(input, true)
	if err != nil {
		t.Errorf("MarshalEnv() error = %v", err)
		return
	}

	// Verify order: A_KEY should appear before M_KEY before Z_KEY
	aIdx := strings.Index(result, "A_KEY")
	mIdx := strings.Index(result, "M_KEY")
	zIdx := strings.Index(result, "Z_KEY")

	if !(aIdx < mIdx && mIdx < zIdx) {
		t.Errorf("keys not in sorted order: A=%d, M=%d, Z=%d", aIdx, mIdx, zIdx)
	}
}

// ============================================================================
// MarshalEnvAs Tests (Multi-format Marshal)
// ============================================================================

func TestMarshalEnvAs(t *testing.T) {
	input := map[string]string{
		"APP_NAME": "myapp",
		"APP_PORT": "8080",
		"DEBUG":    "true",
	}

	t.Run("dotenv format", func(t *testing.T) {
		result, err := MarshalEnvAs(input, FormatEnv, false)
		if err != nil {
			t.Errorf("MarshalEnvAs() error = %v", err)
			return
		}
		if !strings.Contains(result, "APP_NAME=myapp") {
			t.Errorf("expected .env format, got: %s", result)
		}
	})

	t.Run("json format", func(t *testing.T) {
		result, err := MarshalEnvAs(input, FormatJSON, false)
		if err != nil {
			t.Errorf("MarshalEnvAs() error = %v", err)
			return
		}
		if !strings.Contains(result, `"APP"`) || !strings.Contains(result, `"NAME"`) {
			t.Errorf("expected JSON format with nested structure, got: %s", result)
		}
	})

	t.Run("yaml format", func(t *testing.T) {
		result, err := MarshalEnvAs(input, FormatYAML, false)
		if err != nil {
			t.Errorf("MarshalEnvAs() error = %v", err)
			return
		}
		if !strings.Contains(result, "APP_NAME: myapp") {
			t.Errorf("expected YAML format, got: %s", result)
		}
	})

	t.Run("invalid format", func(t *testing.T) {
		_, err := MarshalEnvAs(input, MarshalFormat(99), false)
		if err == nil {
			t.Error("expected error for invalid format")
		}
	})
}

// ============================================================================
// JSON Marshal Tests
// ============================================================================

func TestMarshalToJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]string
		sorted   bool
		contains []string
	}{
		{
			name:     "empty map",
			input:    map[string]string{},
			sorted:   false,
			contains: []string{"{}"},
		},
		{
			name: "simple flat map",
			input: map[string]string{
				"NAME": "test",
				"PORT": "8080",
			},
			sorted:   false,
			contains: []string{`"NAME"`, `"test"`, `"PORT"`, `8080`},
		},
		{
			name: "nested keys",
			input: map[string]string{
				"DB_HOST": "localhost",
				"DB_PORT": "5432",
			},
			sorted:   false,
			contains: []string{`"DB"`, `"HOST"`, `"localhost"`},
		},
		{
			name: "boolean values",
			input: map[string]string{
				"DEBUG":   "true",
				"VERBOSE": "false",
			},
			sorted:   false,
			contains: []string{`"DEBUG"`, `true`, `"VERBOSE"`, `false`},
		},
		{
			name: "numeric values",
			input: map[string]string{
				"COUNT": "42",
				"RATIO": "3.14",
			},
			sorted:   false,
			contains: []string{`"COUNT"`, `42`, `"RATIO"`, `3.14`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := marshalToJSON(tt.input, tt.sorted)
			if err != nil {
				t.Errorf("marshalToJSON() error = %v", err)
				return
			}
			for _, c := range tt.contains {
				if !strings.Contains(string(result), c) {
					t.Errorf("marshalToJSON() should contain %q, got %s", c, result)
				}
			}
		})
	}
}

func TestInferJSONType(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{}
	}{
		{"true", true},
		{"false", false},
		{"null", nil},
		{"42", int64(42)},
		{"3.14", float64(3.14)},
		{"hello", "hello"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := inferJSONType(tt.input)
			if result != tt.expected {
				t.Errorf("inferJSONType(%q) = %v (%T), want %v (%T)",
					tt.input, result, result, tt.expected, tt.expected)
			}
		})
	}
}

// ============================================================================
// YAML Marshal Tests
// ============================================================================

func TestMarshalToYAML(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]string
		sorted   bool
		contains []string
	}{
		{
			name:     "empty map",
			input:    map[string]string{},
			sorted:   false,
			contains: []string{},
		},
		{
			name: "simple map",
			input: map[string]string{
				"NAME": "test",
				"PORT": "8080",
			},
			sorted: false,
			// Numeric strings are quoted so the value round-trips as a string
			// (an unquoted 8080 would still parse back to "8080", but bool/null
			// scalars would be coerced — the quoting rule is uniform).
			contains: []string{"NAME: test", `PORT: "8080"`},
		},
		{
			name: "sorted output",
			input: map[string]string{
				"Z_KEY": "z",
				"A_KEY": "a",
			},
			sorted:   true,
			contains: []string{"A_KEY: a", "Z_KEY: z"},
		},
		{
			name: "value with special chars",
			input: map[string]string{
				"MESSAGE": "hello: world",
			},
			sorted:   false,
			contains: []string{`MESSAGE: "hello: world"`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := marshalToYAML(tt.input, tt.sorted)
			if err != nil {
				t.Errorf("marshalToYAML() error = %v", err)
				return
			}
			for _, c := range tt.contains {
				if !strings.Contains(string(result), c) {
					t.Errorf("marshalToYAML() should contain %q, got %s", c, result)
				}
			}
		})
	}
}

func TestEscapeYAMLValue(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple", "simple"},
		{"", `""`},
		{"has: colon", `"has: colon"`},
		{"has# hash", `"has# hash"`},
		{"has\n newline", `"has\n newline"`},
		{"has \"quote\"", `"has \"quote\""`},
		{"- starts with dash", `"- starts with dash"`},
		// Scalars the YAML reader would type-coerce on reparse are quoted so
		// the string value survives a Marshal→Unmarshal round trip.
		{"true", `"true"`},
		{"null", `"null"`},
		{"~", `"~"`},
		{"42", `"42"`},
		{"3.14", `"3.14"`},
		// Trailing whitespace is stripped from plain scalars — quote it.
		{"trailing ", `"trailing "`},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := escapeYAMLValue(tt.input)
			if result != tt.expected {
				t.Errorf("escapeYAMLValue(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestMarshalToYAMLSorted(t *testing.T) {
	input := map[string]string{
		"Z_KEY": "z",
		"A_KEY": "a",
		"M_KEY": "m",
	}

	result, err := marshalToYAML(input, true)
	if err != nil {
		t.Errorf("marshalToYAML() error = %v", err)
		return
	}

	// Verify order: A_KEY should appear before M_KEY before Z_KEY
	str := string(result)
	aIdx := strings.Index(str, "A_KEY:")
	mIdx := strings.Index(str, "M_KEY:")
	zIdx := strings.Index(str, "Z_KEY:")

	if !(aIdx < mIdx && mIdx < zIdx) {
		t.Errorf("keys not in sorted order: A=%d, M=%d, Z=%d", aIdx, mIdx, zIdx)
	}
}

// TestMarshalEnv_EscapeArms pins the carriage-return and backslash arms of
// escapeValueToBuilder: \r both triggers quoting and escapes, while a lone
// backslash neither triggers quoting nor is escaped.
func TestMarshalEnv_EscapeArms(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"a\rb", `"a\rb"`},
		{`a\b`, `a\b`},
	}
	for _, tt := range tests {
		out, err := MarshalEnv(map[string]string{"K": tt.in}, false)
		if err != nil {
			t.Fatalf("MarshalEnv() error = %v", err)
		}
		if got := strings.TrimSuffix(out, "\n"); got != "K="+tt.want {
			t.Errorf("MarshalEnv(%q) = %q, want %q", tt.in, got, "K="+tt.want)
		}
	}
}

// TestMarshalEnv_EscapeEstimatePath drives the capacity-estimation branch:
// values containing escapable characters take the doubled-length estimate
// path while still marshaling correctly.
func TestMarshalEnv_EscapeEstimatePath(t *testing.T) {
	m := map[string]string{
		"WITH_SPACE": "a b",
		"PLAIN":      "plain",
	}
	out, err := MarshalEnv(m, false)
	if err != nil {
		t.Fatalf("MarshalEnv error = %v", err)
	}
	if !strings.Contains(out, "WITH_SPACE=\"a b\"") {
		t.Errorf("output missing quoted WITH_SPACE line in:\n%s", out)
	}
	if !strings.Contains(out, "PLAIN=plain") {
		t.Errorf("output missing PLAIN line in:\n%s", out)
	}
}

// TestMarshalEnvAs_JSONSorted drives the sorted-keys branch of
// marshalToJSON; the JSON encoder sorts keys itself, so the observable
// contract is simply a well-formed document with all keys present.
func TestMarshalEnvAs_JSONSorted(t *testing.T) {
	m := map[string]string{"B": "2", "A": "1", "C": "3"}
	out, err := MarshalEnvAs(m, FormatJSON, true)
	if err != nil {
		t.Fatalf("MarshalEnvAs error = %v", err)
	}
	for _, k := range []string{`"A"`, `"B"`, `"C"`} {
		if !strings.Contains(out, k) {
			t.Errorf("output missing key %s in:\n%s", k, out)
		}
	}
}

// TestSetNestedValue_ConflictBranch covers the non-map intermediate
// conflict: once "A" holds a scalar, "A_B" cannot nest under it and is
// stored at the root under its full key instead.
func TestSetNestedValue_ConflictBranch(t *testing.T) {
	m := map[string]interface{}{}
	setNestedValue(m, "A", "1") // A becomes a scalar
	setNestedValue(m, "A_B", "2")

	if _, ok := m["A_B"]; !ok {
		t.Fatalf("A_B missing from result %v", m)
	}
	if _, ok := m["A"].(map[string]interface{}); ok {
		t.Error("A became a map, want it to stay a scalar")
	}
}

// TestEscapeYAMLValue_ControlCharArms covers the \r, \t and backslash escape
// arms inside escapeYAMLValue's quoted-scalar path (control chars trigger
// quoting; the backslash is then escaped).
func TestEscapeYAMLValue_ControlCharArms(t *testing.T) {
	tests := []struct{ in, want string }{
		{"a\rb", `"a\rb"`},
		{"a\tb", `"a\tb"`},
		{`a\b` + "\t", `"a\\b\t"`},
	}
	for _, tt := range tests {
		if got := escapeYAMLValue(tt.in); got != tt.want {
			t.Errorf("escapeYAMLValue(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
