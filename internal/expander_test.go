package internal

import (
	"errors"
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
		{name: "no variables", input: "plain text", expected: "plain text"},
		{name: "simple variable", input: "$VAR1", expected: "value1"},
		{name: "braced variable", input: "${VAR1}", expected: "value1"},
		{name: "nested variable", input: "$NESTED", expected: "value1"},
		{name: "undefined variable", input: "$UNDEFINED", expected: ""},
		{name: "escaped dollar", input: "$$VAR1", expected: "$VAR1"},
		{name: "escaped dollar only", input: "$$", expected: "$"},
		{name: "mixed content", input: "prefix_${VAR1}_suffix", expected: "prefix_value1_suffix"},
		{name: "multiple variables", input: "$VAR1 ${VAR2}", expected: "value1 value2"},
		{name: "variable at end", input: "prefix_$VAR1", expected: "prefix_value1"},
		{name: "variable in middle", input: "start_${VAR1}_end", expected: "start_value1_end"},
		{name: "dollar at end of string", input: "text$", expected: "text$"},
		{name: "dollar with non-var char", input: "$!", expected: "$!"},
		{name: "empty braces", input: "${}", expected: "{}"},
		{name: "unclosed brace", input: "${VAR", expected: "${VAR"},
		{name: "invalid key in braces", input: "${123BAD}", expected: "${123BAD}"},
		{name: "nested variable in default value", input: "${MISSING:-${VAR2}}", expected: "value2"},
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
		vars := map[string]string{
			"VAR":   "actual",
			"EMPTY": "", // present but empty — a valid explicit value
		}
		v, ok := vars[key]
		return v, ok
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
		wantErr  bool
	}{
		{name: "default used when unset", input: "${UNSET:-default}", expected: "default"},
		{name: "default not used when set", input: "${VAR:-default}", expected: "actual"},
		{name: "explicit empty does not trigger default", input: "${EMPTY:-default}", expected: ""},
		{name: "assign default", input: "${UNSET:=assigned}", expected: "assigned"},
		{name: "assign not performed when already set", input: "${VAR:=newval}", expected: "actual"},
		{name: "simple default without vars", input: "${UNSET:-simple_default}", expected: "simple_default"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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

func TestExpanderQuestionOperator(t *testing.T) {
	tests := []struct {
		name    string
		lookup  func(string) (string, bool)
		input   string
		want    string
		wantErr bool
	}{
		{
			name:    "unset variable errors",
			lookup:  func(string) (string, bool) { return "", false },
			input:   "${REQUIRED:?Variable is required}",
			wantErr: true,
		},
		{
			name:   "set variable returns value",
			lookup: func(string) (string, bool) { return "value", true },
			input:  "${REQUIRED:?Variable is required}",
			want:   "value",
		},
		{
			name:    "empty value errors",
			lookup:  func(string) (string, bool) { return "", true },
			input:   "${REQUIRED:?Variable is required}",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exp := NewExpander(ExpanderConfig{
				MaxDepth: 5,
				Lookup:   tt.lookup,
				Mode:     ModeAll,
			})

			result, err := exp.Expand(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Expand() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && result != tt.want {
				t.Errorf("Expand() = %q, want %q", result, tt.want)
			}
		})
	}
}

func TestExpanderDepthLimit(t *testing.T) {
	lookup := func(key string) (string, bool) {
		vars := map[string]string{
			"A": "$B", "B": "$C", "C": "$D",
			"D": "$E", "E": "$F", "F": "$G", "G": "final",
		}
		v, ok := vars[key]
		return v, ok
	}

	exp := NewExpander(ExpanderConfig{MaxDepth: 3, Lookup: lookup, Mode: ModeAll})
	_, err := exp.Expand("$A")
	if err == nil {
		t.Error("expected depth limit error")
	}
}

// TestExpander_ErrExpansionDepthMatching verifies that real expansion errors
// classify correctly via errors.Is: depth-limit and cycle errors match
// ErrExpansionDepth, while the ${VAR:?} required-variable error does not
// (it carries ExpansionRequiredKind).
func TestExpander_ErrExpansionDepthMatching(t *testing.T) {
	// Depth-limit error
	depthLookup := func(key string) (string, bool) {
		vars := map[string]string{"A": "$B", "B": "$C", "C": "$D", "D": "$E", "E": "final"}
		v, ok := vars[key]
		return v, ok
	}
	depthExp := NewExpander(ExpanderConfig{MaxDepth: 2, Lookup: depthLookup, Mode: ModeAll})
	_, depthErr := depthExp.Expand("$A")
	if depthErr == nil {
		t.Fatal("expected depth limit error")
	}
	if !errors.Is(depthErr, ErrExpansionDepth) {
		t.Errorf("depth error should match ErrExpansionDepth, got %v", depthErr)
	}

	// Cycle error
	cycleLookup := func(key string) (string, bool) {
		vars := map[string]string{"A": "$B", "B": "$A"}
		v, ok := vars[key]
		return v, ok
	}
	cycleExp := NewExpander(ExpanderConfig{MaxDepth: 10, Lookup: cycleLookup, Mode: ModeAll})
	_, cycleErr := cycleExp.Expand("$A")
	if cycleErr == nil {
		t.Fatal("expected cycle error")
	}
	if !errors.Is(cycleErr, ErrExpansionDepth) {
		t.Errorf("cycle error should match ErrExpansionDepth, got %v", cycleErr)
	}

	// Required-variable error: must NOT match ErrExpansionDepth
	unsetLookup := func(string) (string, bool) { return "", false }
	reqExp := NewExpander(ExpanderConfig{MaxDepth: 5, Lookup: unsetLookup, Mode: ModeAll})
	_, reqErr := reqExp.Expand("${REQUIRED:?Variable is required}")
	if reqErr == nil {
		t.Fatal("expected required-variable error")
	}
	if errors.Is(reqErr, ErrExpansionDepth) {
		t.Errorf("required-variable error must NOT match ErrExpansionDepth, got %v", reqErr)
	}
}

func TestExpanderCycleDetection(t *testing.T) {
	lookup := func(key string) (string, bool) {
		vars := map[string]string{"A": "$B", "B": "$A"}
		v, ok := vars[key]
		return v, ok
	}

	exp := NewExpander(ExpanderConfig{MaxDepth: 10, Lookup: lookup, Mode: ModeAll})
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
		{name: "empty map", vars: map[string]string{}, hasCycle: false},
		{name: "no cycle", vars: map[string]string{"A": "value", "B": "$A"}, hasCycle: false},
		{name: "direct cycle", vars: map[string]string{"A": "$A"}, hasCycle: true},
		{name: "indirect cycle", vars: map[string]string{"A": "$B", "B": "$C", "C": "$A"}, hasCycle: true},
		{name: "self reference", vars: map[string]string{"A": "$A"}, hasCycle: true},
		{name: "complex nesting no cycle", vars: map[string]string{"A": "${B}", "B": "$C", "C": "value"}, hasCycle: false},
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

func TestExpanderMode(t *testing.T) {
	lookup := func(key string) (string, bool) { return "value", true }

	tests := []struct {
		name     string
		mode     Mode
		input    string
		expected string
	}{
		{name: "ModeNone returns unchanged", mode: ModeNone, input: "$VAR", expected: "$VAR"},
		{name: "ModeEnv expands", mode: ModeEnv, input: "$VAR", expected: "value"},
		{name: "ModeAll expands", mode: ModeAll, input: "$VAR", expected: "value"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exp := NewExpander(ExpanderConfig{
				MaxDepth: 5,
				Lookup:   lookup,
				Mode:     tt.mode,
			})
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

func TestExpanderConfig(t *testing.T) {
	lookup := func(key string) (string, bool) { return "value", true }

	t.Run("nil lookup", func(t *testing.T) {
		exp := NewExpander(ExpanderConfig{MaxDepth: 5, Lookup: nil, Mode: ModeAll})
		result, err := exp.Expand("$VAR")
		if err != nil {
			t.Errorf("Expand() error = %v", err)
		}
		if result != "" {
			t.Errorf("Expand() = %q, want empty", result)
		}
	})

	t.Run("zero max depth uses default", func(t *testing.T) {
		exp := NewExpander(ExpanderConfig{MaxDepth: 0, Lookup: lookup, Mode: ModeAll})
		result, err := exp.Expand("$VAR")
		if err != nil {
			t.Errorf("Expand() error = %v", err)
		}
		if result != "value" {
			t.Errorf("Expand() = %q, want \"value\"", result)
		}
	})

	t.Run("hard max depth cap", func(t *testing.T) {
		exp := NewExpander(ExpanderConfig{MaxDepth: HardMaxExpansionDepth + 1000, Lookup: lookup, Mode: ModeAll})
		if exp.maxDepth > HardMaxExpansionDepth {
			t.Errorf("maxDepth = %d, should be capped at %d", exp.maxDepth, HardMaxExpansionDepth)
		}
	})

	t.Run("custom key pattern", func(t *testing.T) {
		customLookup := func(key string) (string, bool) {
			if key == "my.custom.key" {
				return "value", true
			}
			return "", false
		}
		exp := NewExpander(ExpanderConfig{
			MaxDepth:   5,
			Lookup:     customLookup,
			Mode:       ModeAll,
			KeyPattern: regexp.MustCompile(`^[a-z][a-z0-9.]*$`),
		})
		result, err := exp.Expand("${my.custom.key}")
		if err != nil {
			t.Errorf("Expand() error = %v", err)
		}
		if result != "value" {
			t.Errorf("Expand() = %q, want \"value\"", result)
		}
	})
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
	if !strings.Contains(msg, "depth") || !strings.Contains(msg, "limit") {
		t.Errorf("Error message should contain depth and limit info, got: %s", msg)
	}
}

func TestLineParserExpandAll(t *testing.T) {
	newParser := func(vars map[string]string) *LineParser {
		v := NewValidator(ValidatorConfig{MaxKeyLength: 64, MaxValueLength: 1024})
		a := NewAuditor(nil, nil, nil, false)
		e := NewExpander(ExpanderConfig{
			MaxDepth: 5,
			Lookup: func(key string) (string, bool) {
				val, ok := vars[key]
				return val, ok
			},
			Mode: ModeAll,
		})
		return NewLineParser(LineParserConfig{ExpandVariables: true}, v, a, e)
	}

	t.Run("expands variables in map", func(t *testing.T) {
		lp := newParser(map[string]string{"BASE": "value"})
		result, err := lp.ExpandAll(map[string]string{"KEY": "$BASE"})
		if err != nil {
			t.Fatalf("ExpandAll() error = %v", err)
		}
		if result["KEY"] != "value" {
			t.Errorf("ExpandAll() = %v, want KEY=value", result)
		}
	})

	t.Run("cycle detection returns expansion error", func(t *testing.T) {
		lp := newParser(map[string]string{"A": "$B", "B": "$A"})
		_, err := lp.ExpandAll(map[string]string{"A": "$B", "B": "$A"})
		if err == nil {
			t.Fatal("ExpandAll() expected cycle detection error, got nil")
		}
		var expErr *ExpansionError
		if !errors.As(err, &expErr) {
			t.Errorf("error type = %T, want *ExpansionError", err)
		}
	})
}

func TestExpandAllInMap(t *testing.T) {
	tests := []struct {
		name     string
		mode     Mode
		vars     map[string]string
		expected map[string]string
		wantErr  bool
	}{
		{
			name:     "ModeNone returns original",
			mode:     ModeNone,
			vars:     map[string]string{"A": "$B"},
			expected: map[string]string{"A": "$B"},
		},
		{
			name:     "no expansion needed",
			mode:     ModeAll,
			vars:     map[string]string{"A": "plain", "B": "text"},
			expected: map[string]string{"A": "plain", "B": "text"},
		},
		{
			name:     "expands variables",
			mode:     ModeAll,
			vars:     map[string]string{"BASE": "val", "REF": "$BASE"},
			expected: map[string]string{"BASE": "val", "REF": "val"},
		},
		{
			name:    "cycle detected",
			mode:    ModeAll,
			vars:    map[string]string{"A": "$B", "B": "$A"},
			wantErr: true,
		},
		{
			name:     "empty map",
			mode:     ModeAll,
			vars:     map[string]string{},
			expected: map[string]string{},
		},
		{
			name:     "cross-reference with braced",
			mode:     ModeAll,
			vars:     map[string]string{"HOST": "localhost", "URL": "http://${HOST}:8080"},
			expected: map[string]string{"HOST": "localhost", "URL": "http://localhost:8080"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lookup := func(key string) (string, bool) { return "", false }
			exp := NewExpander(ExpanderConfig{MaxDepth: 5, Lookup: lookup, Mode: tt.mode})
			result, err := exp.ExpandAllInMap(tt.vars)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExpandAllInMap() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			for k, v := range tt.expected {
				if result[k] != v {
					t.Errorf("ExpandAllInMap()[%q] = %q, want %q", k, result[k], v)
				}
			}
		})
	}
}

// TestBuildChain_MasksSensitiveKeys verifies that sensitive keys are masked
// in the expansion chain for error messages.
func TestBuildChain_MasksSensitiveKeys(t *testing.T) {
	lookup := func(key string) (string, bool) { return "", false }

	exp := NewExpander(ExpanderConfig{MaxDepth: 5, Lookup: lookup, Mode: ModeAll})

	tests := []struct {
		name             string
		visited          map[string]bool
		shouldContain    string
		shouldNotContain string
	}{
		{
			name:             "sensitive key PASSWORD is masked",
			visited:          map[string]bool{"HOST": true, "DB_PASSWORD": true, "PORT": true},
			shouldContain:    "HOST",
			shouldNotContain: "DB_PASSWORD",
		},
		{
			name:             "sensitive key SECRET is masked",
			visited:          map[string]bool{"APP_SECRET": true, "TOKEN": true},
			shouldContain:    "AP***",
			shouldNotContain: "APP_SECRET",
		},
		{
			name:             "non-sensitive keys are not masked",
			visited:          map[string]bool{"VAR1": true, "VAR2": true},
			shouldContain:    "VAR1",
			shouldNotContain: "",
		},
		{
			name:             "empty visited returns empty",
			visited:          map[string]bool{},
			shouldContain:    "",
			shouldNotContain: "anything",
		},
		{
			name:             "API_KEY is masked (sensitive)",
			visited:          map[string]bool{"API_KEY": true},
			shouldContain:    "AP***",
			shouldNotContain: "API_KEY",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chain := exp.buildChain(tt.visited)
			if tt.shouldContain != "" && !strings.Contains(chain, tt.shouldContain) {
				t.Errorf("buildChain() = %q, should contain %q", chain, tt.shouldContain)
			}
			if tt.shouldNotContain != "" && strings.Contains(chain, tt.shouldNotContain) {
				t.Errorf("buildChain() = %q, should NOT contain %q", chain, tt.shouldNotContain)
			}
		})
	}
}

// TestExpand_SingleVarPreservesLiteralSuffix verifies the single-variable fast
// paths keep any literal suffix after the reference. Regression test: the fast
// paths returned "" for the whole string when the variable was unset (or the
// key invalid / braces empty), silently dropping text the general path
// (multiple references) preserves — "$FOO bar" with FOO unset must expand to
// " bar", not "".
func TestExpand_SingleVarPreservesLiteralSuffix(t *testing.T) {
	unset := NewExpander(ExpanderConfig{
		MaxDepth: 5,
		Mode:     ModeAll,
		Lookup:   func(string) (string, bool) { return "", false },
	})
	set := NewExpander(ExpanderConfig{
		MaxDepth: 5,
		Mode:     ModeAll,
		Lookup: func(k string) (string, bool) {
			if k == "FOO" {
				return "v", true
			}
			return "", false
		},
	})

	tests := []struct {
		name    string
		e       *Expander
		input   string
		want    string
		wantErr bool
	}{
		{"unset $VAR with suffix", unset, "$FOO bar", " bar", false},
		{"unset ${VAR} with suffix", unset, "${FOO} bar", " bar", false},
		{"unset $VAR whole string", unset, "$FOO", "", false},
		{"unset ${VAR} whole string", unset, "${FOO}", "", false},
		{"invalid key with suffix", unset, "${1BAD} tail", "${1BAD} tail", false},
		{"empty braces with suffix", unset, "${} tail", "{} tail", false},
		{"set $VAR with suffix", set, "$FOO bar", "v bar", false},
		{"default with suffix", unset, "${BAZ:-def} tail", "def tail", false},
		{"general path consistency", unset, "$FOO bar$X", " bar", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.e.Expand(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Expand() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("Expand(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestExpander_RequiredOperatorExpandsValue pins the ${VAR:?msg} semantics for
// the set case: the value must be recursively expanded like the plain ${VAR}
// path. Regression: the branch previously returned the raw value, so with
// HOST=$BASE the reference ${HOST:?required} expanded to the literal "$BASE".
func TestExpander_RequiredOperatorExpandsValue(t *testing.T) {
	lookup := func(key string) (string, bool) {
		vars := map[string]string{"HOST": "$BASE", "BASE": "example.com"}
		v, ok := vars[key]
		return v, ok
	}
	exp := NewExpander(ExpanderConfig{MaxDepth: 5, Lookup: lookup, Mode: ModeAll})

	// General path: reference with a literal suffix.
	got, err := exp.Expand("${HOST:?required}/x")
	if err != nil {
		t.Fatalf("Expand() error = %v", err)
	}
	if got != "example.com/x" {
		t.Errorf("Expand(${HOST:?required}/x) = %q, want %q", got, "example.com/x")
	}

	// Fast path: the reference is the entire input.
	got, err = exp.Expand("${HOST:?required}")
	if err != nil {
		t.Fatalf("Expand() error = %v", err)
	}
	if got != "example.com" {
		t.Errorf("Expand(${HOST:?required}) = %q, want %q", got, "example.com")
	}

	// Plain ${VAR} path for parity.
	got, err = exp.Expand("${HOST}/x")
	if err != nil {
		t.Fatalf("Expand() error = %v", err)
	}
	if got != "example.com/x" {
		t.Errorf("Expand(${HOST}/x) = %q, want %q", got, "example.com/x")
	}
}

// TestExpander_RequiredOperatorCycle verifies that mutually required variables
// (A="${B:?}", B="${A:?}") surface a cycle error instead of recursing until
// the depth limit.
func TestExpander_RequiredOperatorCycle(t *testing.T) {
	lookup := func(key string) (string, bool) {
		vars := map[string]string{"A": "${B:?}", "B": "${A:?}"}
		v, ok := vars[key]
		return v, ok
	}
	exp := NewExpander(ExpanderConfig{MaxDepth: 5, Lookup: lookup, Mode: ModeAll})
	if _, err := exp.Expand("${A:?required}"); err == nil {
		t.Error("expected cycle error for mutually-required variables, got nil")
	}
}

// TestExpander_DollarEdgeCases pins the lexer edge cases of variable
// references: an escaped dollar ($$) and a lone/trailing dollar that has no
// name to expand must survive expansion unchanged.
func TestExpander_DollarEdgeCases(t *testing.T) {
	lookup := func(key string) (string, bool) { return "", false }
	exp := NewExpander(ExpanderConfig{MaxDepth: 5, Lookup: lookup, Mode: ModeAll})

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"escaped dollar", "cost: $$5", "cost: $5"},
		{"lone dollar", "$", "$"},
		{"trailing dollar", "value$", "value$"},
		{"double dollar alone", "$$", "$"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := exp.Expand(tt.input)
			if err != nil {
				t.Fatalf("Expand(%q) error = %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("Expand(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
