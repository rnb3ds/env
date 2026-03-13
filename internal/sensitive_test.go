package internal

import (
	"strings"
	"testing"
)

func TestIsKey(t *testing.T) {
	tests := []struct {
		key      string
		expected bool
	}{
		// Sensitive keys
		{"PASSWORD", true},
		{"password", true},
		{"DB_PASSWORD", true},
		{"API_KEY", true},
		{"SECRET_TOKEN", true},
		{"PRIVATE_KEY", true},
		{"ACCESS_KEY", true},
		{"SECRET_KEY", true},
		{"CREDENTIAL", true},
		{"AUTH_TOKEN", true},
		{"SESSION_ID", true},
		{"COOKIE_SECRET", true},
		{"PASSPHRASE", true},

		// SECURITY: DATABASE_URL is now considered sensitive because
		// it typically contains credentials (e.g., postgresql://user:password@host/db)
		{"DATABASE_URL", true},
		{"DB_PASSWORD", true},
		{"CONNECTION_STRING", true},

		// Non-sensitive keys
		{"HOSTNAME", false},
		{"PORT", false},
		{"DEBUG", false},
		{"LOG_LEVEL", false},
		{"SERVER_NAME", false},
		{"TIMEOUT", false},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			result := IsKey(tt.key)
			if result != tt.expected {
				t.Errorf("IsKey(%q) = %v, want %v", tt.key, result, tt.expected)
			}
		})
	}
}

func TestMaskValue(t *testing.T) {
	tests := []struct {
		name       string
		key        string
		value      string
		contains   string
		notContain string
	}{
		{
			name:       "sensitive value masked",
			key:        "PASSWORD",
			value:      "secret123",
			contains:   "MASKED",
			notContain: "secret123",
		},
		{
			name:       "short non-sensitive value unchanged",
			key:        "PORT",
			value:      "8080",
			contains:   "8080",
			notContain: "",
		},
		{
			name:       "long non-sensitive value truncated",
			key:        "HOSTNAME",
			value:      "verylonghostnamethatneedstobetruncated.example.com",
			contains:   "...",
			notContain: "example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MaskValue(tt.key, tt.value)
			if !strings.Contains(result, tt.contains) {
				t.Errorf("MaskValue() = %q, want to contain %q", result, tt.contains)
			}
			if tt.notContain != "" && strings.Contains(result, tt.notContain) {
				t.Errorf("MaskValue() = %q, should not contain %q", result, tt.notContain)
			}
		})
	}
}

func TestMaskKey(t *testing.T) {
	tests := []struct {
		key      string
		expected string
	}{
		{"AB", "***"},        // len <= 3
		{"ABC", "***"},       // len == 3
		{"ABCD", "AB***"},    // len > 3
		{"API_KEY", "AP***"}, // len > 3
		{"", "***"},          // empty
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			result := MaskKey(tt.key)
			if result != tt.expected {
				t.Errorf("MaskKey(%q) = %q, want %q", tt.key, result, tt.expected)
			}
		})
	}
}

func TestMaskInString(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
	}{
		{"short string unchanged", "short", 50},
		{"exact limit unchanged", "exactly50charactersexactly50charactersexactly50c", 50},
		{"long string truncated", strings.Repeat("a", 100), 50},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MaskInString(tt.input)
			if len(result) > 53 { // maxLen + "..."
				t.Errorf("MaskInString() length = %d, should be <= 53", len(result))
			}
		})
	}
}

func TestSanitizeForLog(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		notContain string
	}{
		{
			name:       "password masked",
			input:      "password=secret123",
			notContain: "secret123",
		},
		{
			name:       "token masked",
			input:      "token=abc123xyz",
			notContain: "abc123xyz",
		},
		{
			name:       "api_key masked",
			input:      "api_key=mysecretkey",
			notContain: "mysecretkey",
		},
		{
			name:       "control chars removed",
			input:      "value\x00with\x01nulls",
			notContain: "\x00",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeForLog(tt.input)
			if strings.Contains(result, tt.notContain) {
				t.Errorf("SanitizeForLog() = %q, should not contain %q", result, tt.notContain)
			}
		})
	}
}

func TestPatternsList(t *testing.T) {
	// Verify Patterns list is not empty
	if len(Patterns) == 0 {
		t.Error("Patterns list should not be empty")
	}

	// Verify each pattern is uppercase
	for _, pattern := range Patterns {
		if pattern != strings.ToUpper(pattern) {
			t.Errorf("Pattern %q should be uppercase", pattern)
		}
	}
}
