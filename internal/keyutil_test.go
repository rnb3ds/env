package internal

import (
	"fmt"
	"strings"
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

func TestTrimSpace(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty", "", ""},
		{"no whitespace", "hello", "hello"},
		{"leading space", "  hello", "hello"},
		{"trailing space", "hello  ", "hello"},
		{"both sides", "  hello  ", "hello"},
		{"tabs", "\thello\t", "hello"},
		{"newlines", "\nhello\n", "hello"},
		{"mixed whitespace", " \t\nhello\n\t ", "hello"},
		{"only whitespace", "   \t\n  ", ""},
		{"inner whitespace preserved", "hello world", "hello world"},
		{"single space", " ", ""},
		{"single char", "a", "a"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TrimSpace(tt.input)
			if result != tt.expected {
				t.Errorf("TrimSpace(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestInternKeyBytes_EdgeCases(t *testing.T) {
	ClearInternCache()

	tests := []struct {
		name string
		key  string
		want string
	}{
		{"empty string not interned", "", ""},
		{"simple key", "SIMPLE", "SIMPLE"},
		{"key with underscores", "MY_KEY_NAME", "MY_KEY_NAME"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := InternKeyBytes([]byte(tt.key))
			if got != tt.want {
				t.Errorf("InternKeyBytes(%q) = %q, want %q", tt.key, got, tt.want)
			}
		})
	}
}

func TestInternKeyBytes_Eviction(t *testing.T) {
	ClearInternCache()

	// Fill the cache to trigger eviction
	for i := 0; i < maxInternSize*2; i++ {
		key := "EVICT_KEY_" + strings.Repeat("x", i%20+1)
		InternKeyBytes([]byte(key))
	}

	// Verify cache still works after eviction
	key := "POST_EVICT_KEY"
	if got := InternKeyBytes([]byte(key)); got != key {
		t.Errorf("InternKeyBytes after eviction = %q, want %q", got, key)
	}
}

func TestInternKeyBytes_Consistency(t *testing.T) {
	// Clear cache before test
	ClearInternCache()

	// Test that cache and order slice remain consistent
	// Fill cache beyond capacity to trigger eviction
	for i := 0; i < maxInternSize+50; i++ {
		key := "KEY_" + strings.Repeat("A", i%10)
		interned := InternKeyBytes([]byte(key))
		if interned != key {
			t.Errorf("InternKeyBytes(%q) = %q, want %q", key, interned, key)
		}
	}

	// Verify we can still intern new keys after eviction
	newKey := "NEW_TEST_KEY"
	interned := InternKeyBytes([]byte(newKey))
	if interned != newKey {
		t.Errorf("InternKeyBytes(%q) = %q, want %q", newKey, interned, newKey)
	}
}

func TestHasUpperPrefix(t *testing.T) {
	tests := []struct {
		s      string
		prefix string
		want   bool
	}{
		{"password_value", "PASS", true},
		{"PASSWORD_VALUE", "PASS", true},
		{"PassWord", "PASS", true},
		{"other_value", "PASS", false},
		{"pa", "PASS", false},
		{"", "PASS", false},
		{"value", "", true},
		{"secret_key", "SEC", true},
		{"SecretKey", "SEC", true},
		{"_underscore", "_", true},
		{"123numeric", "123", true},
	}

	for _, tt := range tests {
		name := tt.s + "_" + tt.prefix
		t.Run(name, func(t *testing.T) {
			if got := HasUpperPrefix(tt.s, tt.prefix); got != tt.want {
				t.Errorf("HasUpperPrefix(%q, %q) = %v, want %v", tt.s, tt.prefix, got, tt.want)
			}
		})
	}
}

func TestEqualFoldASCII(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"hello", "HELLO", true},
		{"Hello", "hello", true},
		{"ABC", "ABC", true},
		{"abc", "abc", true},
		{"abc", "abd", false},
		{"abc", "ab", false},
		{"", "", true},
		{"a", "A", true},
	}

	for _, tt := range tests {
		name := tt.a + "_" + tt.b
		t.Run(name, func(t *testing.T) {
			if got := EqualFoldASCII(tt.a, tt.b); got != tt.want {
				t.Errorf("EqualFoldASCII(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestInternKeyBytes_LongKey(t *testing.T) {
	ClearInternCache()
	longKey := strings.Repeat("X", maxInternKeyLen+1)
	if got := InternKeyBytes([]byte(longKey)); got != longKey {
		t.Errorf("InternKeyBytes(long) = %q, want %q", got, longKey)
	}
}

// TestInternKeyBytes_StableAfterBufferReuse verifies the security invariant:
// the returned string must remain valid after the input byte slice is reused
// (as happens with bufio.Scanner's buffer on the next Scan()).
func TestInternKeyBytes_StableAfterBufferReuse(t *testing.T) {
	ClearInternCache()

	// First intern with one buffer content.
	buf := []byte("STABLE_KEY_001")
	got := InternKeyBytes(buf)
	if got != "STABLE_KEY_001" {
		t.Fatalf("InternKeyBytes = %q, want STABLE_KEY_001", got)
	}

	// Mutate the backing buffer (simulating scanner reuse) and interleave a
	// cache miss + hit to stress the intern cache as well.
	for i := 0; i < len(buf) && i < 4; i++ {
		buf[i] = 'Z'
	}
	_ = InternKeyBytes([]byte("OTHER_KEY_002")) // separate entry

	// The earlier returned string must be unaffected by the buffer mutation.
	if got != "STABLE_KEY_001" {
		t.Errorf("interned string corrupted after buffer reuse: got %q", got)
	}

	// Re-interning the original bytes (fresh slice) must yield the same value.
	if again := InternKeyBytes([]byte("STABLE_KEY_001")); again != "STABLE_KEY_001" {
		t.Errorf("re-intern = %q, want STABLE_KEY_001", again)
	}
}

func TestInternKeyBytes_HitIsZeroAlloc(t *testing.T) {
	b := []byte("ZERO_ALLOC_HIT_KEY")
	_ = InternKeyBytes(b) // warm the cache

	n := testing.AllocsPerRun(1000, func() {
		_ = InternKeyBytes(b)
	})
	if n != 0 {
		t.Errorf("InternKeyBytes cache hit allocated %v/op, want 0", n)
	}
}

func TestToUpperASCII(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", ""},
		{"abc", "ABC"},
		{"ABC", "ABC"},
		{"aBc123", "ABC123"},
		{"already_UPPER", "ALREADY_UPPER"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := ToUpperASCII(tt.input); got != tt.want {
				t.Errorf("ToUpperASCII(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestStoreInterned_EvictsWhenFull pins the bounded-eviction contract: when a
// shard is at maxInternSize, inserting one more key evicts exactly one
// existing entry rather than growing the cache.
func TestStoreInterned_EvictsWhenFull(t *testing.T) {
	shard := &internShard{cache: make(map[string]string, maxInternSize)}

	for i := 0; i < maxInternSize; i++ {
		shard.cache[fmt.Sprintf("KEY_%d", i)] = fmt.Sprintf("KEY_%d", i)
	}

	got := storeInterned(shard, "NEW_KEY")

	if got != "NEW_KEY" {
		t.Errorf("storeInterned() = %q, want %q", got, "NEW_KEY")
	}
	if shard.cache["NEW_KEY"] != "NEW_KEY" {
		t.Error("storeInterned() should store the new key in the shard")
	}
	if len(shard.cache) != maxInternSize {
		t.Errorf("len(cache) = %d after insert into a full shard, want %d (one eviction)", len(shard.cache), maxInternSize)
	}
}
