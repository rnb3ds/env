package internal

import (
	"strings"
	"testing"
)

// ============================================================================
// YAML Lexer Tests
// ============================================================================

func TestYAMLLexer_SimpleKey(t *testing.T) {
	input := "key: value"
	lexer := NewYAMLLexer(input)

	tokens, err := lexer.Tokenize()
	if err != nil {
		t.Fatalf("Tokenize() error = %v", err)
	}

	// Expected: Key, Value, EOF
	if len(tokens) < 2 {
		t.Fatalf("expected at least 2 tokens, got %d", len(tokens))
	}

	if tokens[0].Type != TokenKey {
		t.Errorf("first token type = %v, want TokenKey", tokens[0].Type)
	}
	if tokens[0].Value != "key" {
		t.Errorf("first token value = %q, want %q", tokens[0].Value, "key")
	}

	if tokens[1].Type != TokenValue {
		t.Errorf("second token type = %v, want TokenValue", tokens[1].Type)
	}
	if tokens[1].Value != "value" {
		t.Errorf("second token value = %q, want %q", tokens[1].Value, "value")
	}
}

func TestYAMLLexer_NestedKeys(t *testing.T) {
	input := `database:
  host: localhost
  port: 5432`
	lexer := NewYAMLLexer(input)

	tokens, err := lexer.Tokenize()
	if err != nil {
		t.Fatalf("Tokenize() error = %v", err)
	}

	// Find nested keys
	foundHost := false
	foundPort := false
	for _, tok := range tokens {
		if tok.Value == "host" && tok.Indent == 1 {
			foundHost = true
		}
		if tok.Value == "port" && tok.Indent == 1 {
			foundPort = true
		}
	}

	if !foundHost {
		t.Error("expected to find 'host' key with indent 1")
	}
	if !foundPort {
		t.Error("expected to find 'port' key with indent 1")
	}
}

func TestYAMLLexer_Array(t *testing.T) {
	input := `items:
  - one
  - two
  - three`
	lexer := NewYAMLLexer(input)

	tokens, err := lexer.Tokenize()
	if err != nil {
		t.Fatalf("Tokenize() error = %v", err)
	}

	// Count dash tokens
	dashCount := 0
	for _, tok := range tokens {
		if tok.Type == TokenDash {
			dashCount++
		}
	}

	if dashCount != 3 {
		t.Errorf("expected 3 dash tokens, got %d", dashCount)
	}
}

func TestYAMLLexer_DocumentStart(t *testing.T) {
	input := `---
key: value`
	lexer := NewYAMLLexer(input)

	tokens, err := lexer.Tokenize()
	if err != nil {
		t.Fatalf("Tokenize() error = %v", err)
	}

	if len(tokens) < 1 {
		t.Fatal("expected at least 1 token")
	}

	if tokens[0].Type != TokenDocumentStart {
		t.Errorf("first token type = %v, want TokenDocumentStart", tokens[0].Type)
	}
}

func TestYAMLLexer_Comments(t *testing.T) {
	input := `# This is a comment
key: value # inline comment`
	lexer := NewYAMLLexer(input)

	tokens, err := lexer.Tokenize()
	if err != nil {
		t.Fatalf("Tokenize() error = %v", err)
	}

	// Count comment tokens
	commentCount := 0
	for _, tok := range tokens {
		if tok.Type == TokenComment {
			commentCount++
		}
	}

	if commentCount != 2 {
		t.Errorf("expected 2 comment tokens, got %d", commentCount)
	}
}

func TestYAMLLexer_QuotedStrings(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantKey string
		wantVal string
	}{
		{
			name:    "double quoted value",
			input:   `key: "value with spaces"`,
			wantKey: "key",
			wantVal: "value with spaces",
		},
		{
			name:    "single quoted value",
			input:   `key: 'value with spaces'`,
			wantKey: "key",
			wantVal: "value with spaces",
		},
		{
			name:    "double quoted key",
			input:   `"quoted key": value`,
			wantKey: "quoted key",
			wantVal: "value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lexer := NewYAMLLexer(tt.input)
			tokens, err := lexer.Tokenize()
			if err != nil {
				t.Fatalf("Tokenize() error = %v", err)
			}

			if len(tokens) < 2 {
				t.Fatalf("expected at least 2 tokens, got %d", len(tokens))
			}

			if tokens[0].Type != TokenKey || tokens[0].Value != tt.wantKey {
				t.Errorf("key token = %q, want %q", tokens[0].Value, tt.wantKey)
			}

			if tokens[1].Type != TokenValue || tokens[1].Value != tt.wantVal {
				t.Errorf("value token = %q, want %q", tokens[1].Value, tt.wantVal)
			}
		})
	}
}

func TestYAMLLexer_EscapeSequences(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"newline", `"value\nline2"`, "value\nline2"},
		{"tab", `"value\ttab"`, "value\ttab"},
		{"carriage return", `"value\rreturn"`, "value\rreturn"},
		{"backslash", `"path\\to\\file"`, "path\\to\\file"},
		{"escaped quote", `"say \"hello\""`, `say "hello"`},
		{"escaped single quote", `'\''`, `'`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := "key: " + tt.input
			lexer := NewYAMLLexer(input)
			tokens, err := lexer.Tokenize()
			if err != nil {
				t.Fatalf("Tokenize() error = %v", err)
			}

			// Find the value token
			for _, tok := range tokens {
				if tok.Type == TokenValue {
					if tok.Value != tt.expected {
						t.Errorf("value = %q, want %q", tok.Value, tt.expected)
					}
					return
				}
			}
			t.Error("no value token found")
		})
	}
}

func TestYAMLLexer_UnterminatedString(t *testing.T) {
	input := `key: "unterminated`
	lexer := NewYAMLLexer(input)

	_, err := lexer.Tokenize()
	if err == nil {
		t.Error("expected error for unterminated string")
	}
}

func TestYAMLLexer_Empty(t *testing.T) {
	lexer := NewYAMLLexer("")

	tokens, err := lexer.Tokenize()
	if err != nil {
		t.Fatalf("Tokenize() error = %v", err)
	}

	if len(tokens) != 1 || tokens[0].Type != TokenEOF {
		t.Errorf("expected single EOF token for empty input")
	}
}

func TestYAMLLexer_IndentDedent(t *testing.T) {
	input := `root:
  nested:
    deep: value
  back: here`
	lexer := NewYAMLLexer(input)

	tokens, err := lexer.Tokenize()
	if err != nil {
		t.Fatalf("Tokenize() error = %v", err)
	}

	indentCount := 0
	dedentCount := 0
	for _, tok := range tokens {
		if tok.Type == TokenIndent {
			indentCount++
		}
		if tok.Type == TokenDedent {
			dedentCount++
		}
	}

	if indentCount == 0 {
		t.Error("expected at least one indent token")
	}
	if dedentCount == 0 {
		t.Error("expected at least one dedent token")
	}
}

func TestYAMLLexer_MultipleDocuments(t *testing.T) {
	input := `---
doc1: value
---
doc2: value2`
	lexer := NewYAMLLexer(input)

	tokens, err := lexer.Tokenize()
	if err != nil {
		t.Fatalf("Tokenize() error = %v", err)
	}

	docStartCount := 0
	for _, tok := range tokens {
		if tok.Type == TokenDocumentStart {
			docStartCount++
		}
	}

	if docStartCount != 2 {
		t.Errorf("expected 2 document start tokens, got %d", docStartCount)
	}
}

func TestYAMLLexer_CRLF(t *testing.T) {
	input := "key1: value1\r\nkey2: value2"
	lexer := NewYAMLLexer(input)

	tokens, err := lexer.Tokenize()
	if err != nil {
		t.Fatalf("Tokenize() error = %v", err)
	}

	// Should find both keys
	keyCount := 0
	for _, tok := range tokens {
		if tok.Type == TokenKey {
			keyCount++
		}
	}

	if keyCount != 2 {
		t.Errorf("expected 2 key tokens with CRLF, got %d", keyCount)
	}
}

func TestYAMLLexer_Tabs(t *testing.T) {
	input := "root:\n\t\tnested: value"
	lexer := NewYAMLLexer(input)

	tokens, err := lexer.Tokenize()
	if err != nil {
		t.Fatalf("Tokenize() error = %v", err)
	}

	// Tabs should be treated as 2 spaces for indentation
	foundNested := false
	for _, tok := range tokens {
		if tok.Value == "nested" && tok.Type == TokenKey {
			foundNested = true
			if tok.Indent < 1 {
				t.Errorf("nested key indent = %d, expected >= 1", tok.Indent)
			}
		}
	}

	if !foundNested {
		t.Error("expected to find 'nested' key")
	}
}

func TestYAMLLexer_ColonInValue(t *testing.T) {
	input := `url: "https://example.com:8080"`
	lexer := NewYAMLLexer(input)

	tokens, err := lexer.Tokenize()
	if err != nil {
		t.Fatalf("Tokenize() error = %v", err)
	}

	if len(tokens) < 2 {
		t.Fatal("expected at least 2 tokens")
	}

	if tokens[1].Value != "https://example.com:8080" {
		t.Errorf("value = %q, want %q", tokens[1].Value, "https://example.com:8080")
	}
}

func TestYAMLLexer_DashInKey(t *testing.T) {
	input := `api-key: value`
	lexer := NewYAMLLexer(input)

	tokens, err := lexer.Tokenize()
	if err != nil {
		t.Fatalf("Tokenize() error = %v", err)
	}

	if len(tokens) < 1 {
		t.Fatal("expected at least 1 token")
	}

	if tokens[0].Type != TokenKey {
		t.Errorf("first token type = %v, want TokenKey", tokens[0].Type)
	}
	if tokens[0].Value != "api-key" {
		t.Errorf("key = %q, want %q", tokens[0].Value, "api-key")
	}
}

func TestYAMLLexer_LineColumn(t *testing.T) {
	input := `key1: value1
key2: value2`
	lexer := NewYAMLLexer(input)

	tokens, err := lexer.Tokenize()
	if err != nil {
		t.Fatalf("Tokenize() error = %v", err)
	}

	// First key should be at line 1
	if tokens[0].Line != 1 {
		t.Errorf("first key line = %d, want 1", tokens[0].Line)
	}

	// Find second key
	for _, tok := range tokens {
		if tok.Value == "key2" && tok.Type == TokenKey {
			if tok.Line != 2 {
				t.Errorf("second key line = %d, want 2", tok.Line)
			}
			return
		}
	}
	t.Error("did not find key2")
}

func TestYAMLLexer_NextToken(t *testing.T) {
	input := "key: value"
	lexer := NewYAMLLexer(input)

	// Test NextToken one at a time
	tok, err := lexer.NextToken()
	if err != nil {
		t.Fatalf("NextToken() error = %v", err)
	}
	if tok.Type != TokenKey {
		t.Errorf("first token type = %v, want TokenKey", tok.Type)
	}

	tok, err = lexer.NextToken()
	if err != nil {
		t.Fatalf("NextToken() error = %v", err)
	}
	if tok.Type != TokenValue {
		t.Errorf("second token type = %v, want TokenValue", tok.Type)
	}

	tok, err = lexer.NextToken()
	if err != nil {
		t.Fatalf("NextToken() error = %v", err)
	}
	if tok.Type != TokenEOF {
		t.Errorf("third token type = %v, want TokenEOF", tok.Type)
	}
}

func TestYAMLLexer_EscapeAtEndOfString(t *testing.T) {
	input := `key: "value\"`
	lexer := NewYAMLLexer(input)

	_, err := lexer.Tokenize()
	if err == nil {
		t.Error("expected error for escape at end of string")
	}
}

func TestYAMLLexer_NullByte(t *testing.T) {
	input := `key: "value\0null"`
	lexer := NewYAMLLexer(input)

	// SECURITY FIX: \0 escape should now return an error at lexer level
	// for defense in depth. Null bytes can cause:
	// - Log injection (log entries truncated at null byte)
	// - String truncation vulnerabilities in C interop
	// - Bypass of security controls that don't expect nulls
	_, err := lexer.Tokenize()
	if err == nil {
		t.Error("SECURITY: expected error for \\0 escape - null bytes are not allowed")
		return
	}
	// Verify the error message mentions null byte
	if !strings.Contains(err.Error(), "null byte") {
		t.Errorf("error message should mention null byte, got: %v", err)
	}
}
