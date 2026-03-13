package internal

import (
	"testing"
)

func TestIsValidJSONKey(t *testing.T) {
	tests := []struct {
		key      string
		expected bool
	}{
		// Valid keys
		{"valid_key", true},
		{"ValidKey123", true},
		{"key-with-dash", true},
		{"key.with.dot", true},
		{"key@with@at", true},
		{"a", true},
		{"KEY", true},
		{"a_b-c.d@e", true},

		// SECURITY: Keys with brackets are now invalid to prevent
		// key confusion attacks and ambiguity with array indexing
		{"key[0]", false},
		{"123", false},          // pure numeric keys are rejected
		{"a_b-c.d@e[1]", false}, // contains brackets

		// Invalid keys
		{"", false},               // empty
		{"key with space", false}, // space
		{"key:colon", false},      // colon
		{"key#hash", false},       // hash
		{"key/slash", false},      // slash
		{"key\\backslash", false}, // backslash
		{"key'quote", false},      // quote
		{"key\"double", false},    // double quote
		{"key,comma", false},      // comma
		{"key;semicolon", false},  // semicolon
		{"key!exclaim", false},    // exclamation
		{"key?question", false},   // question mark
		{"key(paren)", false},     // parentheses
		{"key{brace}", false},     // braces
		{"key<angle>", false},     // angle brackets
		{"key=equals", false},     // equals
		{"key+plus", false},       // plus
		{"key*asterisk", false},   // asterisk
		{"key&ersand", false},     // ampersand
		{"key%percent", false},    // percent
		{"key$dollar", false},     // dollar
		{"key|pipe", false},       // pipe
		{"key^caret", false},      // caret
		{"key~tilde", false},      // tilde
		{"key`backtick", false},   // backtick
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			result := IsValidJSONKey(tt.key)
			if result != tt.expected {
				t.Errorf("IsValidJSONKey(%q) = %v, want %v", tt.key, result, tt.expected)
			}
		})
	}
}
