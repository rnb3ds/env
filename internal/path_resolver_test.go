package internal

import (
	"testing"
)

func TestResolvePath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected []string
	}{
		{
			name:     "simple nested path",
			path:     "database.host",
			expected: []string{"DATABASE_HOST"},
		},
		{
			name:     "three level nested path",
			path:     "a.b.c",
			expected: []string{"A_B_C"},
		},
		{
			name:     "array index access",
			path:     "servers.0",
			expected: []string{"SERVERS_0", "SERVERS[0]"},
		},
		{
			name:     "nested array element",
			path:     "servers.0.host",
			expected: []string{"SERVERS_0_HOST", "SERVERS[0]_HOST"},
		},
		{
			name:     "multiple array indices",
			path:     "matrix.0.1",
			expected: []string{"MATRIX_0_1", "MATRIX[0][1]"},
		},
		{
			name:     "simple uppercase key unchanged",
			path:     "DATABASE_HOST",
			expected: []string{"DATABASE_HOST"},
		},
		{
			name:     "simple lowercase key returns both cases",
			path:     "database_host",
			expected: []string{"database_host", "DATABASE_HOST"},
		},
		{
			name:     "simple mixed case key returns both",
			path:     "autoUpdatesChannel",
			expected: []string{"autoUpdatesChannel", "AUTOUPDATESCHANNEL"},
		},
		{
			name:     "deep nesting with array",
			path:     "config.database.connections.0.port",
			expected: []string{"CONFIG_DATABASE_CONNECTIONS_0_PORT", "CONFIG_DATABASE_CONNECTIONS[0]_PORT"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ResolvePath(tt.path)
			if len(result) != len(tt.expected) {
				t.Errorf("ResolvePath() returned %d candidates, expected %d", len(result), len(tt.expected))
				t.Errorf("got: %v", result)
				t.Errorf("want: %v", tt.expected)
				return
			}
			for i, exp := range tt.expected {
				if result[i] != exp {
					t.Errorf("ResolvePath()[%d] = %q, expected %q", i, result[i], exp)
				}
			}
		})
	}
}

func TestIsNumericIndex(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"0", true},
		{"1", true},
		{"123", true},
		{"", false},
		{"abc", false},
		{"1a", false},
		{"a1", false},
		{"-1", false},
		{"1.5", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := isNumericIndex(tt.input)
			if result != tt.expected {
				t.Errorf("isNumericIndex(%q) = %v, expected %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestExtractNumericIndex(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		wantBase  string
		wantIndex int
		wantOK    bool
	}{
		{
			name:      "simple indexed path",
			path:      "servers.0",
			wantBase:  "servers",
			wantIndex: 0,
			wantOK:    true,
		},
		{
			name:      "nested indexed path",
			path:      "service.cors.origins.0",
			wantBase:  "service.cors.origins",
			wantIndex: 0,
			wantOK:    true,
		},
		{
			name:      "multi-digit index",
			path:      "items.123",
			wantBase:  "items",
			wantIndex: 123,
			wantOK:    true,
		},
		{
			name:      "deep nesting with index",
			path:      "config.database.connections.5",
			wantBase:  "config.database.connections",
			wantIndex: 5,
			wantOK:    true,
		},
		{
			name:      "path without index",
			path:      "database.host",
			wantBase:  "",
			wantIndex: -1,
			wantOK:    false,
		},
		{
			name:      "simple key without dot",
			path:      "hostname",
			wantBase:  "",
			wantIndex: -1,
			wantOK:    false,
		},
		{
			name:      "single numeric key",
			path:      "123",
			wantBase:  "",
			wantIndex: -1,
			wantOK:    false,
		},
		{
			name:      "empty string",
			path:      "",
			wantBase:  "",
			wantIndex: -1,
			wantOK:    false,
		},
		{
			name:      "trailing dot",
			path:      "servers.",
			wantBase:  "",
			wantIndex: -1,
			wantOK:    false,
		},
		{
			name:      "non-numeric suffix",
			path:      "servers.host",
			wantBase:  "",
			wantIndex: -1,
			wantOK:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotBase, gotIndex, gotOK := ExtractNumericIndex(tt.path)
			if gotBase != tt.wantBase || gotIndex != tt.wantIndex || gotOK != tt.wantOK {
				t.Errorf("ExtractNumericIndex(%q) = (%q, %d, %v), want (%q, %d, %v)",
					tt.path, gotBase, gotIndex, gotOK, tt.wantBase, tt.wantIndex, tt.wantOK)
			}
		})
	}
}
