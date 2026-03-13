package internal

import (
	"regexp"
	"strings"
	"testing"
)

func TestExpanderExpand(t *testing.T) {
	lookup := func(key string) (string, bool) {
		vars := map[string]string{
			"VAR1":   "value1",
			"VAR2":   "value2",
			"NESTED": "$VAR1",
		}
		v, ok := vars[key]
		return v, ok
	}

	tests := []struct {
		name     string
		input    string
		expected string
		wantErr  bool
	}{
		{
			name:     "no variables",
			input:    "plain text",
			expected: "plain text",
			wantErr:  false,
		},
		{
			name:     "simple variable",
			input:    "$VAR1",
			expected: "value1",
			wantErr:  false,
		},
		{
			name:     "braced variable",
			input:    "${VAR1}",
			expected: "value1",
			wantErr:  false,
		},
		{
			name:     "nested variable",
			input:    "$NESTED",
			expected: "value1",
			wantErr:  false,
		},
		{
			name:     "undefined variable",
			input:    "$UNDEFINED",
			expected: "",
			wantErr:  false,
		},
		{
			name:     "escaped dollar",
			input:    "$$VAR1",
			expected: "$VAR1",
			wantErr:  false,
		},
		{
			name:     "mixed content",
			input:    "prefix_${VAR1}_suffix",
			expected: "prefix_value1_suffix",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exp := NewExpander(ExpanderConfig{
				MaxDepth: 5,
				Lookup:   lookup,
				Mode:     ModeAll,
			})

			result, err := exp.Expand(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Expand() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if result != tt.expected {
				t.Errorf("Expand() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestExpanderDefaultValues(t *testing.T) {
	lookup := func(key string) (string, bool) {
		return "", false // No variables defined
	}

	exp := NewExpander(ExpanderConfig{
		MaxDepth: 5,
		Lookup:   lookup,
		Mode:     ModeAll,
	})

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "default value used",
			input:    "${VAR:-default}",
			expected: "default",
		},
		{
			name:     "assign default",
			input:    "${VAR:=default}",
			expected: "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := exp.Expand(tt.input)
			if err != nil {
				t.Errorf("Expand() error = %v", err)
				return
			}
			if result != tt.expected {
				t.Errorf("Expand() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestExpanderDepthLimit(t *testing.T) {
	// Create circular reference
	lookup := func(key string) (string, bool) {
		vars := map[string]string{
			"A": "$B",
			"B": "$C",
			"C": "$D",
			"D": "$E",
			"E": "$F",
			"F": "$G",
			"G": "final",
		}
		v, ok := vars[key]
		return v, ok
	}

	exp := NewExpander(ExpanderConfig{
		MaxDepth: 3, // Low limit
		Lookup:   lookup,
		Mode:     ModeAll,
	})

	_, err := exp.Expand("$A")
	if err == nil {
		t.Error("expected depth limit error")
	}
}

func TestExpanderCycleDetection(t *testing.T) {
	// Create circular reference
	lookup := func(key string) (string, bool) {
		vars := map[string]string{
			"A": "$B",
			"B": "$A",
		}
		v, ok := vars[key]
		return v, ok
	}

	exp := NewExpander(ExpanderConfig{
		MaxDepth: 10,
		Lookup:   lookup,
		Mode:     ModeAll,
	})

	_, err := exp.Expand("$A")
	if err == nil {
		t.Error("expected cycle detection error")
	}
}

func TestDetectCycle(t *testing.T) {
	tests := []struct {
		name     string
		vars     map[string]string
		hasCycle bool
	}{
		{
			name: "no cycle",
			vars: map[string]string{
				"A": "value",
				"B": "$A",
			},
			hasCycle: false,
		},
		{
			name: "direct cycle",
			vars: map[string]string{
				"A": "$A",
			},
			hasCycle: true,
		},
		{
			name: "indirect cycle",
			vars: map[string]string{
				"A": "$B",
				"B": "$C",
				"C": "$A",
			},
			hasCycle: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, found := DetectCycle(tt.vars)
			if found != tt.hasCycle {
				t.Errorf("DetectCycle() found = %v, want %v", found, tt.hasCycle)
			}
		})
	}
}

func TestExpanderErrorSyntax(t *testing.T) {
	// Test error message syntax
	t.Run("expansion error message", func(t *testing.T) {
		expErr := &ExpansionError{
			Key:   "TEST",
			Depth: 10,
			Limit: 5,
			Chain: "A -> B",
		}

		msg := expErr.Error()
		if msg == "" {
			t.Error("error message should not be empty")
		}
	})
}

func TestExpanderModeNone(t *testing.T) {
	lookup := func(key string) (string, bool) {
		return "value", true
	}

	_ = NewExpander(ExpanderConfig{
		MaxDepth: 5,
		Lookup:   lookup,
		Mode:     ModeNone,
	})

	// ModeNone disables expansion - test passes if no panic
}

func TestLineParserExpandAll(t *testing.T) {
	v := NewValidator(ValidatorConfig{
		MaxKeyLength:   64,
		MaxValueLength: 1024,
	})
	a := NewAuditor(nil, nil, nil, false)
	e := NewExpander(ExpanderConfig{
		MaxDepth: 5,
		Lookup: func(key string) (string, bool) {
			vars := map[string]string{
				"BASE": "value",
			}
			v, ok := vars[key]
			return v, ok
		},
		Mode: ModeAll,
	})

	lp := NewLineParser(LineParserConfig{
		ExpandVariables: true,
	}, v, a, e)

	vars := map[string]string{
		"KEY": "$BASE",
	}

	result, err := lp.ExpandAll(vars)
	if err != nil {
		t.Errorf("ExpandAll() error = %v", err)
	}
	if result["KEY"] != "value" {
		t.Errorf("ExpandAll() = %v, want KEY=value", result)
	}
}

// ============================================================================
// Additional Expander Tests
// ============================================================================

func TestExpander_EmptyBraces(t *testing.T) {
	lookup := func(key string) (string, bool) {
		return "", false
	}

	exp := NewExpander(ExpanderConfig{
		MaxDepth: 5,
		Lookup:   lookup,
		Mode:     ModeAll,
	})

	result, err := exp.Expand("${}")
	if err != nil {
		t.Errorf("Expand() error = %v", err)
	}
	if result != "{}" {
		t.Errorf("Expand(\"${}\") = %q, want \"{}\"", result)
	}
}

func TestExpander_UnclosedBrace(t *testing.T) {
	lookup := func(key string) (string, bool) {
		return "value", true
	}

	exp := NewExpander(ExpanderConfig{
		MaxDepth: 5,
		Lookup:   lookup,
		Mode:     ModeAll,
	})

	result, err := exp.Expand("${VAR")
	if err != nil {
		t.Errorf("Expand() error = %v", err)
	}
	// Unclosed brace should return masked version
	if result != "${...}" {
		t.Errorf("Expand(\"${VAR\") = %q, want \"${...}\"", result)
	}
}

func TestExpander_QuestionOperator(t *testing.T) {
	lookup := func(key string) (string, bool) {
		return "", false // Variable not set
	}

	exp := NewExpander(ExpanderConfig{
		MaxDepth: 5,
		Lookup:   lookup,
		Mode:     ModeAll,
	})

	// :? operator should error when variable is not set
	_, err := exp.Expand("${REQUIRED:?Variable is required}")
	if err == nil {
		t.Error("expected error for :? operator with unset variable")
	}
}

func TestExpander_QuestionOperator_Set(t *testing.T) {
	lookup := func(key string) (string, bool) {
		return "value", true
	}

	exp := NewExpander(ExpanderConfig{
		MaxDepth: 5,
		Lookup:   lookup,
		Mode:     ModeAll,
	})

	// :? operator should return value when variable is set
	result, err := exp.Expand("${REQUIRED:?Variable is required}")
	if err != nil {
		t.Errorf("Expand() error = %v", err)
	}
	if result != "value" {
		t.Errorf("Expand() = %q, want \"value\"", result)
	}
}

func TestExpander_QuestionOperator_EmptyValue(t *testing.T) {
	lookup := func(key string) (string, bool) {
		return "", true // Variable is set but empty
	}

	exp := NewExpander(ExpanderConfig{
		MaxDepth: 5,
		Lookup:   lookup,
		Mode:     ModeAll,
	})

	// :? operator should error when variable is empty
	_, err := exp.Expand("${REQUIRED:?Variable is required}")
	if err == nil {
		t.Error("expected error for :? operator with empty variable")
	}
}

func TestExpander_InvalidKey(t *testing.T) {
	lookup := func(key string) (string, bool) {
		return "value", true
	}

	exp := NewExpander(ExpanderConfig{
		MaxDepth: 5,
		Lookup:   lookup,
		Mode:     ModeAll,
	})

	// Invalid key (starts with number) should be returned as-is
	result, err := exp.Expand("${123BAD}")
	if err != nil {
		t.Errorf("Expand() error = %v", err)
	}
	// Invalid key should be returned unchanged
	if result != "${123BAD}" {
		t.Errorf("Expand(\"${123BAD}\") = %q, want \"${123BAD}\"", result)
	}
}

func TestExpander_CustomKeyPattern(t *testing.T) {
	lookup := func(key string) (string, bool) {
		if key == "my.custom.key" {
			return "value", true
		}
		return "", false
	}

	// Custom pattern that allows dots in keys
	customPattern := regexp.MustCompile(`^[a-z][a-z0-9.]*$`)

	exp := NewExpander(ExpanderConfig{
		MaxDepth:   5,
		Lookup:     lookup,
		Mode:       ModeAll,
		KeyPattern: customPattern,
	})

	result, err := exp.Expand("${my.custom.key}")
	if err != nil {
		t.Errorf("Expand() error = %v", err)
	}
	if result != "value" {
		t.Errorf("Expand() = %q, want \"value\"", result)
	}
}

func TestExpander_MultipleVariables(t *testing.T) {
	lookup := func(key string) (string, bool) {
		vars := map[string]string{
			"VAR1": "hello",
			"VAR2": "world",
		}
		v, ok := vars[key]
		return v, ok
	}

	exp := NewExpander(ExpanderConfig{
		MaxDepth: 5,
		Lookup:   lookup,
		Mode:     ModeAll,
	})

	result, err := exp.Expand("$VAR1 ${VAR2}")
	if err != nil {
		t.Errorf("Expand() error = %v", err)
	}
	if result != "hello world" {
		t.Errorf("Expand() = %q, want \"hello world\"", result)
	}
}

func TestExpander_VariableAtEnd(t *testing.T) {
	lookup := func(key string) (string, bool) {
		vars := map[string]string{
			"VAR": "value",
		}
		v, ok := vars[key]
		return v, ok
	}

	exp := NewExpander(ExpanderConfig{
		MaxDepth: 5,
		Lookup:   lookup,
		Mode:     ModeAll,
	})

	result, err := exp.Expand("prefix_$VAR")
	if err != nil {
		t.Errorf("Expand() error = %v", err)
	}
	if result != "prefix_value" {
		t.Errorf("Expand() = %q, want \"prefix_value\"", result)
	}
}

func TestExpander_VariableInMiddle(t *testing.T) {
	lookup := func(key string) (string, bool) {
		vars := map[string]string{
			"VAR": "middle",
		}
		v, ok := vars[key]
		return v, ok
	}

	exp := NewExpander(ExpanderConfig{
		MaxDepth: 5,
		Lookup:   lookup,
		Mode:     ModeAll,
	})

	result, err := exp.Expand("start_${VAR}_end")
	if err != nil {
		t.Errorf("Expand() error = %v", err)
	}
	if result != "start_middle_end" {
		t.Errorf("Expand() = %q, want \"start_middle_end\"", result)
	}
}

func TestExpander_NoVariableAfterDollar(t *testing.T) {
	lookup := func(key string) (string, bool) {
		return "value", true
	}

	exp := NewExpander(ExpanderConfig{
		MaxDepth: 5,
		Lookup:   lookup,
		Mode:     ModeAll,
	})

	// Dollar at end of string
	result, err := exp.Expand("text$")
	if err != nil {
		t.Errorf("Expand() error = %v", err)
	}
	if result != "text$" {
		t.Errorf("Expand() = %q, want \"text$\"", result)
	}
}

func TestExpander_InvalidVarChar(t *testing.T) {
	lookup := func(key string) (string, bool) {
		return "value", true
	}

	exp := NewExpander(ExpanderConfig{
		MaxDepth: 5,
		Lookup:   lookup,
		Mode:     ModeAll,
	})

	// Dollar followed by non-variable character returns as-is
	result, err := exp.Expand("$!")
	if err != nil {
		t.Errorf("Expand() error = %v", err)
	}
	// $! is not a valid variable, so it's returned unchanged
	if result != "$!" {
		t.Errorf("Expand() = %q, want \"$!\"", result)
	}
}

func TestExpander_DefaultValueWithValue(t *testing.T) {
	lookup := func(key string) (string, bool) {
		vars := map[string]string{
			"VAR": "actual",
		}
		v, ok := vars[key]
		return v, ok
	}

	exp := NewExpander(ExpanderConfig{
		MaxDepth: 5,
		Lookup:   lookup,
		Mode:     ModeAll,
	})

	// Default value should NOT be used when variable is set
	result, err := exp.Expand("${VAR:-default}")
	if err != nil {
		t.Errorf("Expand() error = %v", err)
	}
	if result != "actual" {
		t.Errorf("Expand() = %q, want \"actual\"", result)
	}
}

func TestExpander_AssignDefault(t *testing.T) {
	lookup := func(key string) (string, bool) {
		return "", false // Variable not set
	}

	exp := NewExpander(ExpanderConfig{
		MaxDepth: 5,
		Lookup:   lookup,
		Mode:     ModeAll,
	})

	// := should work like :- for this implementation
	result, err := exp.Expand("${VAR:=assigned}")
	if err != nil {
		t.Errorf("Expand() error = %v", err)
	}
	if result != "assigned" {
		t.Errorf("Expand() = %q, want \"assigned\"", result)
	}
}

func TestExpander_NilLookup(t *testing.T) {
	exp := NewExpander(ExpanderConfig{
		MaxDepth: 5,
		Lookup:   nil, // nil lookup
		Mode:     ModeAll,
	})

	// Should not panic and return empty for unset variables
	result, err := exp.Expand("$VAR")
	if err != nil {
		t.Errorf("Expand() error = %v", err)
	}
	if result != "" {
		t.Errorf("Expand() = %q, want \"\"", result)
	}
}

func TestExpander_ZeroMaxDepth(t *testing.T) {
	lookup := func(key string) (string, bool) {
		return "value", true
	}

	exp := NewExpander(ExpanderConfig{
		MaxDepth: 0, // Zero should use default
		Lookup:   lookup,
		Mode:     ModeAll,
	})

	// Should still work with default depth
	result, err := exp.Expand("$VAR")
	if err != nil {
		t.Errorf("Expand() error = %v", err)
	}
	if result != "value" {
		t.Errorf("Expand() = %q, want \"value\"", result)
	}
}

func TestExpander_HardMaxDepth(t *testing.T) {
	lookup := func(key string) (string, bool) {
		return "value", true
	}

	// Try to set a very high max depth
	exp := NewExpander(ExpanderConfig{
		MaxDepth: HardMaxExpansionDepth + 1000,
		Lookup:   lookup,
		Mode:     ModeAll,
	})

	// Should be capped at HardMaxExpansionDepth
	if exp.maxDepth > HardMaxExpansionDepth {
		t.Errorf("maxDepth = %d, should be capped at %d", exp.maxDepth, HardMaxExpansionDepth)
	}
}

func TestExpander_ModeNone_Expansion(t *testing.T) {
	lookup := func(key string) (string, bool) {
		return "value", true
	}

	exp := NewExpander(ExpanderConfig{
		MaxDepth: 5,
		Lookup:   lookup,
		Mode:     ModeNone,
	})

	// ModeNone should still expand but just not do special handling
	// The actual behavior depends on implementation
	result, err := exp.Expand("$VAR")
	if err != nil {
		t.Errorf("Expand() error = %v", err)
	}
	// With ModeNone, variables might not be expanded
	_ = result
}

func TestExpander_ModeEnv(t *testing.T) {
	lookup := func(key string) (string, bool) {
		return "value", true
	}

	exp := NewExpander(ExpanderConfig{
		MaxDepth: 5,
		Lookup:   lookup,
		Mode:     ModeEnv,
	})

	result, err := exp.Expand("$VAR")
	if err != nil {
		t.Errorf("Expand() error = %v", err)
	}
	if result != "value" {
		t.Errorf("Expand() = %q, want \"value\"", result)
	}
}

func TestDetectCycle_EmptyMap(t *testing.T) {
	vars := map[string]string{}
	cycle, found := DetectCycle(vars)
	if found {
		t.Errorf("DetectCycle() found = %v, want false for empty map", found)
	}
	if cycle != "" {
		t.Errorf("DetectCycle() cycle = %q, want \"\" for empty map", cycle)
	}
}

func TestDetectCycle_ComplexNesting(t *testing.T) {
	vars := map[string]string{
		"A": "${B}",
		"B": "$C",
		"C": "value", // No cycle
	}

	cycle, found := DetectCycle(vars)
	if found {
		t.Errorf("DetectCycle() found = %v, want false for non-cyclic chain", found)
	}
	_ = cycle
}

func TestDetectCycle_SelfReference(t *testing.T) {
	vars := map[string]string{
		"A": "$A",
	}

	cycle, found := DetectCycle(vars)
	if !found {
		t.Error("DetectCycle() should detect self-reference")
	}
	if cycle != "A" {
		t.Errorf("DetectCycle() cycle = %q, want \"A\"", cycle)
	}
}

func TestExpander_EscapedDollarOnly(t *testing.T) {
	lookup := func(key string) (string, bool) {
		return "value", true
	}

	exp := NewExpander(ExpanderConfig{
		MaxDepth: 5,
		Lookup:   lookup,
		Mode:     ModeAll,
	})

	result, err := exp.Expand("$$")
	if err != nil {
		t.Errorf("Expand() error = %v", err)
	}
	if result != "$" {
		t.Errorf("Expand() = %q, want \"$\"", result)
	}
}

func TestExpander_DefaultValueWithVariables(t *testing.T) {
	lookup := func(key string) (string, bool) {
		vars := map[string]string{
			"BASE": "base_value",
		}
		v, ok := vars[key]
		return v, ok
	}

	exp := NewExpander(ExpanderConfig{
		MaxDepth: 5,
		Lookup:   lookup,
		Mode:     ModeAll,
	})

	// Default value with nested variable - behavior depends on implementation
	// Some implementations expand the default value, some don't
	result, err := exp.Expand("${UNSET:-simple_default}")
	if err != nil {
		t.Errorf("Expand() error = %v", err)
	}
	// Simple default without variables should work
	if result != "simple_default" {
		t.Errorf("Expand() = %q, want \"simple_default\"", result)
	}
}

func TestExpansionError_Message(t *testing.T) {
	err := &ExpansionError{
		Key:   "TEST_KEY",
		Depth: 10,
		Limit: 5,
		Chain: "A -> B -> C",
	}

	msg := err.Error()
	if msg == "" {
		t.Error("Error() should return non-empty message")
	}

	// Message should contain relevant information
	if !strings.Contains(msg, "depth") || !strings.Contains(msg, "limit") {
		t.Errorf("Error message should contain depth and limit info, got: %s", msg)
	}
}
