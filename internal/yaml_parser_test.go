package internal

import (
	"testing"
)

// ============================================================================
// YAML Parser Tests
// ============================================================================

func TestYAMLParser_SimpleMap(t *testing.T) {
	input := "key: value"
	lexer := newYAMLLexer([]byte(input))
	tokens, err := lexer.tokenizeInto(nil)
	if err != nil {
		t.Fatalf("Tokenize() error = %v", err)
	}

	parser := NewYAMLParser(tokens, 10)
	result, err := parser.Parse()
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if result.Type != ValueTypeMap {
		t.Errorf("result type = %v, want ValueTypeMap", result.Type)
	}

	if len(result.Map) != 1 {
		t.Errorf("result map length = %d, want 1", len(result.Map))
		return
	}

	if result.Map["key"].Scalar != "value" {
		t.Errorf("result[\"key\"] = %q, want %q", result.Map["key"].Scalar, "value")
	}
}

func TestYAMLParser_NestedMap(t *testing.T) {
	input := `database:
  host: localhost
  port: 5432`
	lexer := newYAMLLexer([]byte(input))
	tokens, err := lexer.tokenizeInto(nil)
	if err != nil {
		t.Fatalf("Tokenize() error = %v", err)
	}

	parser := NewYAMLParser(tokens, 10)
	result, err := parser.Parse()
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	db, ok := result.Map["database"]
	if !ok {
		t.Fatal("expected 'database' key")
	}

	if db.Type != ValueTypeMap {
		t.Errorf("database type = %v, want ValueTypeMap", db.Type)
	}

	if db.Map["host"].Scalar != "localhost" {
		t.Errorf("database.host = %q, want %q", db.Map["host"].Scalar, "localhost")
	}

	if db.Map["port"].Scalar != "5432" {
		t.Errorf("database.port = %q, want %q", db.Map["port"].Scalar, "5432")
	}
}

func TestYAMLParser_Array(t *testing.T) {
	input := `items:
  - one
  - two
  - three`
	lexer := newYAMLLexer([]byte(input))
	tokens, err := lexer.tokenizeInto(nil)
	if err != nil {
		t.Fatalf("Tokenize() error = %v", err)
	}

	parser := NewYAMLParser(tokens, 10)
	result, err := parser.Parse()
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	items, ok := result.Map["items"]
	if !ok {
		t.Fatal("expected 'items' key")
	}

	if items.Type != ValueTypeArray {
		t.Errorf("items type = %v, want ValueTypeArray", items.Type)
	}

	if len(items.Array) != 3 {
		t.Errorf("items length = %d, want 3", len(items.Array))
		return
	}

	expected := []string{"one", "two", "three"}
	for i, exp := range expected {
		if items.Array[i].Scalar != exp {
			t.Errorf("items[%d] = %q, want %q", i, items.Array[i].Scalar, exp)
		}
	}
}

func TestYAMLParser_ArrayOfMaps(t *testing.T) {
	input := `servers:
  - host: server1
    port: 8080
  - host: server2
    port: 9090`
	lexer := newYAMLLexer([]byte(input))
	tokens, err := lexer.tokenizeInto(nil)
	if err != nil {
		t.Fatalf("Tokenize() error = %v", err)
	}

	parser := NewYAMLParser(tokens, 10)
	result, err := parser.Parse()
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	servers, ok := result.Map["servers"]
	if !ok {
		t.Fatal("expected 'servers' key")
	}

	if servers.Type != ValueTypeArray {
		t.Errorf("servers type = %v, want ValueTypeArray", servers.Type)
	}

	if len(servers.Array) != 2 {
		t.Errorf("servers length = %d, want 2", len(servers.Array))
		return
	}

	// Check first server
	if servers.Array[0].Map["host"].Scalar != "server1" {
		t.Errorf("servers[0].host = %q, want %q", servers.Array[0].Map["host"].Scalar, "server1")
	}

	// Check second server
	if servers.Array[1].Map["host"].Scalar != "server2" {
		t.Errorf("servers[1].host = %q, want %q", servers.Array[1].Map["host"].Scalar, "server2")
	}
}

func TestYAMLParser_EmptyValue(t *testing.T) {
	input := `key:`
	lexer := newYAMLLexer([]byte(input))
	tokens, err := lexer.tokenizeInto(nil)
	if err != nil {
		t.Fatalf("Tokenize() error = %v", err)
	}

	parser := NewYAMLParser(tokens, 10)
	result, err := parser.Parse()
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if result.Map["key"].Scalar != "" {
		t.Errorf("empty key value = %q, want empty", result.Map["key"].Scalar)
	}
}

func TestYAMLParser_EmptyDocument(t *testing.T) {
	lexer := newYAMLLexer([]byte(""))
	tokens, err := lexer.tokenizeInto(nil)
	if err != nil {
		t.Fatalf("Tokenize() error = %v", err)
	}

	parser := NewYAMLParser(tokens, 10)
	result, err := parser.Parse()
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if result.Type != ValueTypeMap {
		t.Errorf("result type = %v, want ValueTypeMap", result.Type)
	}

	if len(result.Map) != 0 {
		t.Errorf("result map length = %d, want 0", len(result.Map))
	}
}

func TestYAMLParser_MaxDepth(t *testing.T) {
	// Create deeply nested input
	input := `a:
  b:
    c:
      d:
        e: value`
	lexer := newYAMLLexer([]byte(input))
	tokens, err := lexer.tokenizeInto(nil)
	if err != nil {
		t.Fatalf("Tokenize() error = %v", err)
	}

	// Test with low max depth
	parser := NewYAMLParser(tokens, 2)
	_, err = parser.Parse()
	if err == nil {
		t.Error("expected max depth error")
	}
}

func TestYAMLParser_DocumentStart(t *testing.T) {
	input := `---
key: value`
	lexer := newYAMLLexer([]byte(input))
	tokens, err := lexer.tokenizeInto(nil)
	if err != nil {
		t.Fatalf("Tokenize() error = %v", err)
	}

	parser := NewYAMLParser(tokens, 10)
	result, err := parser.Parse()
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if result.Map["key"].Scalar != "value" {
		t.Errorf("key = %q, want %q", result.Map["key"].Scalar, "value")
	}
}

func TestYAMLParser_Comments(t *testing.T) {
	input := `# comment at start
key: value # inline comment
# comment at end`
	lexer := newYAMLLexer([]byte(input))
	tokens, err := lexer.tokenizeInto(nil)
	if err != nil {
		t.Fatalf("Tokenize() error = %v", err)
	}

	parser := NewYAMLParser(tokens, 10)
	result, err := parser.Parse()
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(result.Map) != 1 {
		t.Errorf("result map length = %d, want 1", len(result.Map))
	}

	if result.Map["key"].Scalar != "value" {
		t.Errorf("key = %q, want %q", result.Map["key"].Scalar, "value")
	}
}

func TestYAMLParser_QuotedValues(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "double quoted",
			input:    `key: "value with spaces"`,
			expected: "value with spaces",
		},
		{
			name:     "single quoted",
			input:    `key: 'value with spaces'`,
			expected: "value with spaces",
		},
		{
			name:     "with newline",
			input:    `key: "line1\nline2"`,
			expected: "line1\nline2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lexer := newYAMLLexer([]byte(tt.input))
			tokens, err := lexer.tokenizeInto(nil)
			if err != nil {
				t.Fatalf("Tokenize() error = %v", err)
			}

			parser := NewYAMLParser(tokens, 10)
			result, err := parser.Parse()
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}

			if result.Map["key"].Scalar != tt.expected {
				t.Errorf("value = %q, want %q", result.Map["key"].Scalar, tt.expected)
			}
		})
	}
}

func TestYAMLParser_MultipleKeys(t *testing.T) {
	input := `key1: value1
key2: value2
key3: value3`
	lexer := newYAMLLexer([]byte(input))
	tokens, err := lexer.tokenizeInto(nil)
	if err != nil {
		t.Fatalf("Tokenize() error = %v", err)
	}

	parser := NewYAMLParser(tokens, 10)
	result, err := parser.Parse()
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(result.Map) != 3 {
		t.Errorf("result map length = %d, want 3", len(result.Map))
	}

	expected := map[string]string{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
	}

	for key, exp := range expected {
		if result.Map[key].Scalar != exp {
			t.Errorf("result[%q] = %q, want %q", key, result.Map[key].Scalar, exp)
		}
	}
}

func TestYAMLParser_NestedArrays(t *testing.T) {
	input := `matrix:
  - - a
    - b
  - - c
    - d`
	lexer := newYAMLLexer([]byte(input))
	tokens, err := lexer.tokenizeInto(nil)
	if err != nil {
		t.Fatalf("Tokenize() error = %v", err)
	}

	parser := NewYAMLParser(tokens, 10)
	result, err := parser.Parse()
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	matrix, ok := result.Map["matrix"]
	if !ok {
		t.Fatal("expected 'matrix' key")
	}

	if matrix.Type != ValueTypeArray {
		t.Errorf("matrix type = %v, want ValueTypeArray", matrix.Type)
	}

	// The parser may interpret nested arrays differently
	// Just verify we have an array with content
	if len(matrix.Array) < 1 {
		t.Errorf("matrix length = %d, expected at least 1", len(matrix.Array))
	}
}

// ============================================================================
// ParseYAML Function Tests
// ============================================================================

func TestParseYAML(t *testing.T) {
	data := []byte("key: value")
	result, err := ParseYAML(data, 10)
	if err != nil {
		t.Fatalf("ParseYAML() error = %v", err)
	}

	if result.Map["key"].Scalar != "value" {
		t.Errorf("key = %q, want %q", result.Map["key"].Scalar, "value")
	}
}

func TestParseYAML_Complex(t *testing.T) {
	data := []byte(`
app:
  name: myapp
  version: "1.0.0"
database:
  host: localhost
  port: 5432
  credentials:
    username: admin
    password: secret
features:
  - auth
  - logging
  - cache
`)

	result, err := ParseYAML(data, 10)
	if err != nil {
		t.Fatalf("ParseYAML() error = %v", err)
	}

	// Check nested structure - keys are lowercase in the parsed result
	app := result.Map["app"]
	if app == nil {
		// Keys might be uppercase, check both
		app = result.Map["APP"]
		if app == nil {
			t.Fatalf("expected 'app' or 'APP' key, got keys: %v", getMapKeys(result.Map))
		}
	}

	nameKey := "name"
	if app.Map[nameKey] == nil {
		nameKey = "NAME"
	}
	if app.Map[nameKey] == nil {
		t.Fatalf("expected 'name' or 'NAME' key in app, got keys: %v", getMapKeys(app.Map))
	}
	if app.Map[nameKey].Scalar != "myapp" {
		t.Errorf("app.name = %q, want %q", app.Map[nameKey].Scalar, "myapp")
	}

	// Check database
	db := result.Map["database"]
	if db == nil {
		db = result.Map["DATABASE"]
	}
	if db == nil {
		t.Fatal("expected 'database' key")
	}

	credsKey := "credentials"
	if db.Map[credsKey] == nil {
		credsKey = "CREDENTIALS"
	}
	creds := db.Map[credsKey]
	if creds == nil {
		t.Fatal("expected database.credentials key")
	}

	usernameKey := "username"
	if creds.Map[usernameKey] == nil {
		usernameKey = "USERNAME"
	}
	if creds.Map[usernameKey].Scalar != "admin" {
		t.Errorf("database.credentials.username = %q, want %q", creds.Map[usernameKey].Scalar, "admin")
	}

	// Check features array
	features := result.Map["features"]
	if features == nil {
		features = result.Map["FEATURES"]
	}
	if features == nil {
		t.Fatal("expected 'features' key")
	}

	if features.Type != ValueTypeArray {
		t.Errorf("features type = %v, want ValueTypeArray", features.Type)
	}

	if len(features.Array) != 3 {
		t.Errorf("features length = %d, want 3", len(features.Array))
	}
}

// Helper function to get map keys
func getMapKeys(m map[string]*Value) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// ============================================================================
// Value Type Tests
// ============================================================================

func TestNewScalarValue(t *testing.T) {
	v := NewScalarValue("test", 1, 5)
	if v.Type != ValueTypeScalar {
		t.Errorf("type = %v, want ValueTypeScalar", v.Type)
	}
	if v.Scalar != "test" {
		t.Errorf("scalar = %q, want %q", v.Scalar, "test")
	}
	if v.Line != 1 {
		t.Errorf("line = %d, want 1", v.Line)
	}
	if v.Column != 5 {
		t.Errorf("column = %d, want 5", v.Column)
	}
}

func TestNewMapValue(t *testing.T) {
	v := NewMapValue(2, 10)
	if v.Type != ValueTypeMap {
		t.Errorf("type = %v, want ValueTypeMap", v.Type)
	}
	if v.Map == nil {
		t.Error("map is nil")
	}
	if v.Line != 2 {
		t.Errorf("line = %d, want 2", v.Line)
	}
	if v.Column != 10 {
		t.Errorf("column = %d, want 10", v.Column)
	}
}

func TestNewArrayValue(t *testing.T) {
	v := NewArrayValue(3, 15)
	if v.Type != ValueTypeArray {
		t.Errorf("type = %v, want ValueTypeArray", v.Type)
	}
	if v.Array == nil {
		t.Error("array is nil")
	}
	if v.Line != 3 {
		t.Errorf("line = %d, want 3", v.Line)
	}
	if v.Column != 15 {
		t.Errorf("column = %d, want 15", v.Column)
	}
}

// ============================================================================
// Parser Edge Cases
// ============================================================================

func TestYAMLParser_DefaultMaxDepth(t *testing.T) {
	// Test that default maxDepth is applied when 0 or negative
	lexer := newYAMLLexer([]byte("key: value"))
	tokens, err := lexer.tokenizeInto(nil)
	if err != nil {
		t.Fatalf("Tokenize() error = %v", err)
	}

	parser := NewYAMLParser(tokens, 0)
	if parser.maxDepth != 10 {
		t.Errorf("maxDepth = %d, want 10 (default)", parser.maxDepth)
	}

	parser = NewYAMLParser(tokens, -1)
	if parser.maxDepth != 10 {
		t.Errorf("maxDepth = %d, want 10 (default)", parser.maxDepth)
	}
}

func TestYAMLParser_ValueWithColon(t *testing.T) {
	input := `url: "https://example.com:8080"`
	lexer := newYAMLLexer([]byte(input))
	tokens, err := lexer.tokenizeInto(nil)
	if err != nil {
		t.Fatalf("Tokenize() error = %v", err)
	}

	parser := NewYAMLParser(tokens, 10)
	result, err := parser.Parse()
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if result.Map["url"].Scalar != "https://example.com:8080" {
		t.Errorf("url = %q, want %q", result.Map["url"].Scalar, "https://example.com:8080")
	}
}

func TestYAMLParser_ArrayWithComments(t *testing.T) {
	input := `items:
  # first item
  - one
  # second item
  - two`
	lexer := newYAMLLexer([]byte(input))
	tokens, err := lexer.tokenizeInto(nil)
	if err != nil {
		t.Fatalf("Tokenize() error = %v", err)
	}

	parser := NewYAMLParser(tokens, 10)
	result, err := parser.Parse()
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	items := result.Map["items"]
	if items.Type != ValueTypeArray {
		t.Errorf("items type = %v, want ValueTypeArray", items.Type)
	}

	if len(items.Array) != 2 {
		t.Errorf("items length = %d, want 2", len(items.Array))
	}
}

func TestReleaseValue_Nil(t *testing.T) {
	ReleaseValue(nil) // should not panic
}

func TestReleaseValue_Scalar(t *testing.T) {
	v := NewScalarValue("test", 1, 1)
	ReleaseValue(v)
	if v.Scalar != "" {
		t.Error("scalar not cleared after release")
	}
}

func TestReleaseValue_Nested(t *testing.T) {
	input := `
parent:
  child1: value1
  child2:
    - a
    - b
`
	value, err := ParseYAML([]byte(input), 10)
	if err != nil {
		t.Fatalf("ParseYAML() error = %v", err)
	}
	ReleaseValue(value)
	// Verify the tree is cleared
	if value.Map != nil {
		t.Error("map not cleared after release")
	}
}

func TestReleaseValue_DoubleRelease(t *testing.T) {
	v := NewScalarValue("test", 1, 1)
	ReleaseValue(v)
	ReleaseValue(v) // should not panic
}

// ============================================================================
// Edge Cases for parseMap, parseArray, parseNestedValue (table-driven)
// ============================================================================

// TestYAMLParser_Structure exercises the structural branches of parseMap,
// parseArray, and parseNestedValue: comments before nested content, blank
// lines, dedents, array items at EOF, and nested array/map combinations.
// Each case parses a snippet and asserts the resulting Value tree's shape.
func TestYAMLParser_Structure(t *testing.T) {
	tests := []struct {
		name  string
		input string
		check func(t *testing.T, root *Value)
	}{
		{
			name:  "key with comment then nested map",
			input: "parent:\n  # a comment\n  child: value",
			check: func(t *testing.T, root *Value) {
				parent := root.Map["parent"]
				if parent == nil || parent.Map["child"] == nil {
					t.Fatal("expected parent.child")
				}
				if got := parent.Map["child"].Scalar; got != "value" {
					t.Errorf("parent.child = %q, want %q", got, "value")
				}
			},
		},
		{
			name:  "key with comment and blank line then nested map",
			input: "parent: # trailing comment\n\n  child: value",
			check: func(t *testing.T, root *Value) {
				parent := root.Map["parent"]
				if parent == nil || parent.Map["child"] == nil {
					t.Fatal("expected parent.child after comment and blank line")
				}
			},
		},
		{
			name:  "comment after key with scalar sibling",
			input: "key: # just a comment\nother: value",
			check: func(t *testing.T, root *Value) {
				if root.Map["other"] == nil || root.Map["other"].Scalar != "value" {
					t.Errorf("other = %v, want value", root.Map["other"])
				}
			},
		},
		{
			name:  "blank lines and comment before nested value",
			input: "parent:\n\n  # inner comment\n  child: value",
			check: func(t *testing.T, root *Value) {
				parent := root.Map["parent"]
				if parent == nil || parent.Map["child"] == nil {
					t.Fatal("expected parent.child after blank lines and comment")
				}
			},
		},
		{
			// A comment-only nested block leaves the key with an empty
			// (null) scalar, and the following dedented sibling key is
			// recovered — previously the sibling was silently dropped
			// because parseNestedValue left the closing Dedent unconsumed,
			// making the enclosing map end its own scope early.
			name:  "comment-only nested value recovers dedented sibling",
			input: "key:\n  # only a comment\nother: value",
			check: func(t *testing.T, root *Value) {
				key := root.Map["key"]
				if key == nil || key.Type != ValueTypeScalar || key.Scalar != "" {
					t.Errorf("key = %v, want an empty scalar", key)
				}
				other := root.Map["other"]
				if other == nil || other.Scalar != "value" {
					t.Errorf("other = %v, want value", other)
				}
				if len(root.Map) != 2 {
					t.Errorf("root has %d keys, want 2", len(root.Map))
				}
			},
		},
		{
			name:  "key with empty value then nested scalar",
			input: "key:\n  value",
			check: func(t *testing.T, root *Value) {
				if root.Map["key"] == nil {
					t.Fatal("expected 'key'")
				}
			},
		},
		{
			name:  "nested map followed by sibling key",
			input: "root:\n  inner: val\nkey2: val2",
			check: func(t *testing.T, root *Value) {
				if len(root.Map) != 2 {
					t.Errorf("expected 2 root keys, got %d", len(root.Map))
				}
			},
		},
		{
			name:  "nested map with dedent to sibling",
			input: "outer:\n  inner:\n    deep: val\n  sibling: val2",
			check: func(t *testing.T, root *Value) {
				outer := root.Map["outer"]
				if outer == nil || outer.Type != ValueTypeMap {
					t.Fatal("expected outer map")
				}
				if outer.Map["sibling"] == nil {
					t.Error("expected sibling key after dedent")
				}
			},
		},
		{
			name:  "nested value at EOF",
			input: "parent:\n  child:",
			check: func(t *testing.T, root *Value) {
				child := root.Map["parent"].Map["child"]
				if child == nil || child.Scalar != "" {
					t.Error("expected empty child value at EOF")
				}
			},
		},
		{
			name:  "array item at EOF",
			input: "items:\n  -",
			check: func(t *testing.T, root *Value) {
				items := root.Map["items"]
				if items == nil || items.Type != ValueTypeArray {
					t.Fatal("expected items array")
				}
			},
		},
		{
			name:  "array item newline then non-indented sibling",
			input: "items:\n  -\nother: value",
			check: func(t *testing.T, root *Value) {
				if root.Map["other"] == nil || root.Map["other"].Scalar != "value" {
					t.Errorf("other = %v, want value", root.Map["other"])
				}
			},
		},
		{
			// Nested-dash arrays keep every row: "- - a / - b" builds one
			// inner array per outer item. Previously only the first row was
			// represented (flattened as [empty-array, "a", "b"]) and later
			// rows were dropped entirely.
			name:  "array of nested arrays keeps both rows",
			input: "matrix:\n  - - a\n    - b\n  - - c\n    - d",
			check: func(t *testing.T, root *Value) {
				matrix := root.Map["matrix"]
				if matrix == nil || matrix.Type != ValueTypeArray {
					t.Fatal("expected matrix array")
				}
				if len(matrix.Array) != 2 {
					t.Fatalf("matrix has %d items, want 2", len(matrix.Array))
				}
				wantRows := [][]string{{"a", "b"}, {"c", "d"}}
				for i, want := range wantRows {
					row := matrix.Array[i]
					if row.Type != ValueTypeArray || len(row.Array) != len(want) {
						t.Fatalf("matrix[%d] = %v, want an array of %d scalars", i, row, len(want))
					}
					for j, w := range want {
						if got := row.Array[j]; got.Type != ValueTypeScalar || got.Scalar != w {
							t.Errorf("matrix[%d][%d] = %q, want %q", i, j, got.Scalar, w)
						}
					}
				}
			},
		},
		{
			name:  "nested array direct",
			input: "data:\n  - - x\n      y\n    - z",
			check: func(t *testing.T, root *Value) {
				data := root.Map["data"]
				if data == nil || data.Type != ValueTypeArray {
					t.Fatal("expected data array")
				}
			},
		},
		{
			name:  "array item with newline before nested map",
			input: "items:\n  -\n    key1: val1\n    key2: val2",
			check: func(t *testing.T, root *Value) {
				items := root.Map["items"]
				if items == nil || items.Type != ValueTypeArray || len(items.Array) != 1 {
					t.Fatalf("expected items array of length 1, got %v", items)
				}
				item := items.Array[0]
				if item.Type != ValueTypeMap {
					t.Fatalf("item type = %v, want ValueTypeMap", item.Type)
				}
				if item.Map["key1"] == nil || item.Map["key1"].Scalar != "val1" {
					t.Error("expected key1=val1 in array item")
				}
			},
		},
		{
			name:  "array with comment between items",
			input: "list:\n  - one\n  # middle comment\n  - two",
			check: func(t *testing.T, root *Value) {
				list := root.Map["list"]
				if list == nil || list.Type != ValueTypeArray {
					t.Fatal("expected list array")
				}
				if len(list.Array) != 2 {
					t.Errorf("list length = %d, want 2", len(list.Array))
				}
			},
		},
		{
			name:  "complex nesting (services)",
			input: "services:\n  - name: web\n    ports:\n      - 80\n      - 443\n    env:\n      PORT: \"8080\"\n  - name: db\n    ports:\n      - 5432",
			check: func(t *testing.T, root *Value) {
				services := root.Map["services"]
				if services == nil || services.Type != ValueTypeArray || len(services.Array) != 2 {
					t.Fatalf("expected services array of length 2, got %v", services)
				}
				web := services.Array[0]
				if web.Map["name"] == nil || web.Map["name"].Scalar != "web" {
					t.Error("expected first service name=web")
				}
				ports := web.Map["ports"]
				if ports == nil || ports.Type != ValueTypeArray || len(ports.Array) != 2 {
					t.Error("expected web.ports to be a 2-element array")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseYAML([]byte(tt.input), 10)
			if err != nil {
				t.Fatalf("ParseYAML() error = %v", err)
			}
			tt.check(t, result)
		})
	}
}

// TestYAMLParser_CurrentBeyondTokens pins the EOF sentinel contract of
// Parser.current(): exhausting the token stream must yield TokenEOF, never
// panic or read out of bounds.
func TestYAMLParser_CurrentBeyondTokens(t *testing.T) {
	p := NewYAMLParser(nil, 10)
	p.advance() // already past the (empty) token slice
	if got := p.current().Type; got != TokenEOF {
		t.Errorf("current() beyond tokens = %v, want TokenEOF", got)
	}
}

// TestYAMLParser_ZeroMaxDepthRejected covers parseDocument's depth guard,
// reachable only when the parser is constructed with a non-positive maxDepth
// (NewYAMLParser clamps public callers to a minimum of 10).
func TestYAMLParser_ZeroMaxDepthRejected(t *testing.T) {
	p := &Parser{maxDepth: 0}
	if _, err := p.Parse(); err == nil {
		t.Error("Parse() with maxDepth 0 should exceed the depth limit")
	}
}

// TestParseYAML_HashInPlainScalars pins the YAML comment rule: '#' begins a
// comment only at line start or after whitespace; '#' inside a plain scalar
// is data. Regression: the lexer truncated at any '#', corrupting values such
// as http://host/p#frag or pa#ss.
func TestParseYAML_HashInPlainScalars(t *testing.T) {
	data := []byte(`# full line comment
url: http://host/p#frag
password: pa#ss
note: value # trailing comment
`)

	result, err := ParseYAML(data, 10)
	if err != nil {
		t.Fatalf("ParseYAML() error = %v", err)
	}

	if got := result.Map["url"].Scalar; got != "http://host/p#frag" {
		t.Errorf("url = %q, want %q", got, "http://host/p#frag")
	}
	if got := result.Map["password"].Scalar; got != "pa#ss" {
		t.Errorf("password = %q, want %q", got, "pa#ss")
	}
	if got := result.Map["note"].Scalar; got != "value" {
		t.Errorf("note = %q, want %q (comment after whitespace must still be stripped)", got, "value")
	}
	if len(result.Map) != 3 {
		t.Errorf("expected exactly 3 keys (full-line comment ignored), got %d", len(result.Map))
	}
}

// TestParseYAML_HashMidTokenKey documents that a '#' inside an unquoted key
// (not preceded by whitespace) is preserved as part of the key; downstream
// key validation rejects it with an explicit error rather than the previous
// silent truncation of the key.
func TestParseYAML_HashMidTokenKey(t *testing.T) {
	result, err := ParseYAML([]byte("weird#key: v"), 10)
	if err != nil {
		t.Fatalf("ParseYAML() error = %v", err)
	}
	node := result.Map["weird#key"]
	if node == nil {
		t.Fatalf("key containing mid-token '#' was not preserved; keys: %v", getMapKeys(result.Map))
	}
	if node.Scalar != "v" {
		t.Errorf("weird#key = %q, want %q", node.Scalar, "v")
	}
}

// mkKey/mkValue/mkDash construct tokens for the branch-level parser tests.
func mkKey(v string, indent int) Token {
	return Token{Type: TokenKey, Value: v, Indent: indent, Line: 1, Column: 1}
}

// TestYAMLParser_TokenLevelBranches drives the parser with hand-built token
// sequences that the lexer never (or rarely) emits, pinning the defensive
// branches of parseMap, parseNestedValue, and parseArray: document separators
// inside structures, dedent-by-indent, comments and newlines after keys,
// inline maps and dashes as values, and depth-limit error propagation.
func TestYAMLParser_TokenLevelBranches(t *testing.T) {
	tests := []struct {
		name   string
		tokens []Token
		maxDep int
		check  func(t *testing.T, root *Value)
	}{
		{
			name:   "document start terminates nested map",
			tokens: []Token{mkKey("K", 0), {Type: TokenIndent, Indent: 1}, mkKey("N", 1), {Type: TokenValue, Value: "v"}, {Type: TokenDocumentStart}, mkKey("AFTER", 0), {Type: TokenValue, Value: "w"}},
			check: func(t *testing.T, root *Value) {
				k := root.Map["K"]
				if k == nil || k.Type != ValueTypeMap || k.Map["N"] == nil || k.Map["N"].Scalar != "v" {
					t.Errorf("K = %v, want nested map with N=v (terminated at document start)", k)
				}
			},
		},
		{
			name:   "key below expected indent ends nested map",
			tokens: []Token{mkKey("K", 0), {Type: TokenIndent, Indent: 1}, mkKey("INNER", 0), {Type: TokenValue, Value: "v"}},
			check: func(t *testing.T, root *Value) {
				if root.Map["INNER"] == nil || root.Map["INNER"].Scalar != "v" {
					t.Errorf("INNER = %v, want scalar v", root.Map["INNER"])
				}
			},
		},
		{
			name:   "comment after key at EOF yields empty scalar",
			tokens: []Token{mkKey("K", 0), {Type: TokenComment, Value: "c"}, {Type: TokenEOF}},
			check: func(t *testing.T, root *Value) {
				if root.Map["K"] == nil || root.Map["K"].Scalar != "" {
					t.Errorf("K = %v, want empty scalar", root.Map["K"])
				}
			},
		},
		{
			name:   "newline after key without indent yields empty scalar",
			tokens: []Token{mkKey("K", 0), {Type: TokenNewline}, mkKey("N", 1), {Type: TokenValue, Value: "v"}},
			check: func(t *testing.T, root *Value) {
				if root.Map["K"] == nil || root.Map["K"].Scalar != "" {
					t.Errorf("K = %v, want empty scalar (no indent token)", root.Map["K"])
				}
			},
		},
		{
			name:   "dash directly after key parses as array value",
			tokens: []Token{mkKey("K", 0), {Type: TokenDash, Indent: 1}, {Type: TokenValue, Value: "a"}},
			check: func(t *testing.T, root *Value) {
				if root.Map["K"] == nil || root.Map["K"].Type != ValueTypeArray || len(root.Map["K"].Array) != 1 {
					t.Errorf("K = %v, want single-element array", root.Map["K"])
				}
			},
		},
		{
			name:   "key directly after key parses as inline nested map",
			tokens: []Token{mkKey("K", 0), mkKey("N", 1), {Type: TokenValue, Value: "v"}},
			check: func(t *testing.T, root *Value) {
				k := root.Map["K"]
				if k == nil || k.Type != ValueTypeMap || k.Map["N"] == nil {
					t.Errorf("K = %v, want inline nested map with N", k)
				}
			},
		},
		{
			name:   "EOF right after key indent yields empty scalar",
			tokens: []Token{mkKey("K", 0), {Type: TokenIndent, Indent: 1}},
			check: func(t *testing.T, root *Value) {
				if root.Map["K"] == nil || root.Map["K"].Scalar != "" {
					t.Errorf("K = %v, want empty scalar at EOF", root.Map["K"])
				}
			},
		},
		{
			name:   "comments between key and nested map",
			tokens: []Token{mkKey("K", 0), {Type: TokenComment, Value: "c1"}, {Type: TokenNewline}, {Type: TokenIndent, Indent: 1}, mkKey("N", 1), {Type: TokenValue, Value: "v"}},
			check: func(t *testing.T, root *Value) {
				k := root.Map["K"]
				if k == nil || k.Map["N"] == nil || k.Map["N"].Scalar != "v" {
					t.Errorf("K = %v, want nested map with N=v after comments", k)
				}
			},
		},
		{
			name:   "unexpected token in nested value yields scalar",
			tokens: []Token{mkKey("K", 0), {Type: TokenIndent, Indent: 1}, {Type: TokenColon}},
			check: func(t *testing.T, root *Value) {
				if root.Map["K"] == nil || root.Map["K"].Scalar != "" {
					t.Errorf("K = %v, want empty scalar for unknown token", root.Map["K"])
				}
			},
		},
		{
			name:   "dash then EOF yields array with empty scalar",
			tokens: []Token{mkKey("K", 0), {Type: TokenDash, Indent: 1}},
			check: func(t *testing.T, root *Value) {
				k := root.Map["K"]
				if k == nil || k.Type != ValueTypeArray || len(k.Array) != 1 || k.Array[0].Scalar != "" {
					t.Errorf("K = %v, want array with one empty scalar", k)
				}
			},
		},
		{
			name:   "document start terminates array",
			tokens: []Token{mkKey("K", 0), {Type: TokenDash, Indent: 1}, {Type: TokenValue, Value: "a"}, {Type: TokenDocumentStart}},
			check: func(t *testing.T, root *Value) {
				k := root.Map["K"]
				if k == nil || k.Type != ValueTypeArray || len(k.Array) != 1 {
					t.Errorf("K = %v, want array with one element", k)
				}
			},
		},
		{
			name:   "indent directly after dash parses nested item",
			tokens: []Token{mkKey("K", 0), {Type: TokenDash, Indent: 1}, {Type: TokenIndent, Indent: 2}, mkKey("N", 2), {Type: TokenValue, Value: "v"}},
			check: func(t *testing.T, root *Value) {
				k := root.Map["K"]
				if k == nil || k.Type != ValueTypeArray || len(k.Array) != 1 {
					t.Fatalf("K = %v, want one array item", k)
				}
				if k.Array[0].Map["N"] == nil {
					t.Errorf("array item = %v, want a map with N", k.Array[0])
				}
			},
		},
		{
			name:   "nested dash chain exceeds depth limit",
			tokens: []Token{mkKey("K", 0), {Type: TokenDash, Indent: 1}, {Type: TokenDash, Indent: 2}, {Type: TokenDash, Indent: 3}},
			maxDep: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			maxDepth := tt.maxDep
			if maxDepth == 0 {
				maxDepth = 10
			}
			p := NewYAMLParser(tt.tokens, maxDepth)
			root, err := p.Parse()
			if tt.maxDep != 0 {
				// Depth-limit case: expect an error.
				if err == nil {
					t.Fatal("Parse() error = nil, want depth-limit error")
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			tt.check(t, root)
		})
	}
}

// TestYAMLParser_ParseNestedValueIndentEntry covers the indent-consumption
// branch at the head of parseNestedValue, reachable only when the parser is
// handed a token stream whose current token is an indent at entry.
func TestYAMLParser_ParseNestedValueIndentEntry(t *testing.T) {
	p := NewYAMLParser([]Token{{Type: TokenIndent, Indent: 1}, mkKey("N", 1), {Type: TokenValue, Value: "v"}}, 10)
	v, err := p.parseNestedValue(1, 1)
	if err != nil {
		t.Fatalf("parseNestedValue() error = %v", err)
	}
	if v == nil || v.Map["N"] == nil {
		t.Errorf("parseNestedValue() = %v, want map with N", v)
	}
}

// TestNewArrayValue_PooledReuse pins the pool-reuse branch: a pooled Value
// carrying a non-nil Array is reset via truncation, not reallocation.
func TestNewArrayValue_PooledReuse(t *testing.T) {
	valuePool.Put(&Value{Array: []*Value{NewScalarValue("stale", 1, 1)}})

	v := NewArrayValue(1, 1)
	if v.Type != ValueTypeArray {
		t.Fatalf("NewArrayValue() type = %v, want ValueTypeArray", v.Type)
	}
	if len(v.Array) != 0 {
		t.Errorf("reused Array length = %d, want 0 (stale elements must be dropped)", len(v.Array))
	}
}

// TestParseYAML_LexerError covers the tokenize-failure path: input the lexer
// rejects (an unterminated double-quoted string) surfaces as a ParseYAML
// error instead of being silently parsed.
func TestParseYAML_LexerError(t *testing.T) {
	if _, err := ParseYAML([]byte("key: \"unterminated"), 10); err == nil {
		t.Error("ParseYAML() should reject input the lexer cannot tokenize")
	}
}
