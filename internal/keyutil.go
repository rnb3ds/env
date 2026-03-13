// Package internal provides key validation utilities.
package internal

import "sync"

// Key interning cache limits.
// maxInternSize is the maximum number of keys to cache per shard.
// maxInternKeyLen is the maximum key length to intern (longer keys are not cached
// as they are less likely to be repeated and would waste memory).
const (
	maxInternSize   = 64 // Per shard
	maxInternKeyLen = 64
	numShards       = 8 // Increased from 4 for better concurrency on modern CPUs
)

// internShard represents a single shard of the intern cache.
// Uses a single mutex (not RWMutex) for better cache locality and
// simpler lock management. The cache is small enough that RWMutex
// overhead outweighs its benefits.
type internShard struct {
	mu    sync.Mutex
	cache map[string]string
	order []string // Used for FIFO eviction
}

var internShards [numShards]internShard

func init() {
	for i := range numShards {
		internShards[i].cache = make(map[string]string, maxInternSize)
	}
}

// HashKey returns a hash value for the given key using an optimized algorithm.
// For short keys (<=8 chars), uses a simple multiplicative hash.
// For longer keys, uses FNV-1a with sampling for better performance.
// The numShards parameter determines the range of the returned hash (0 to numShards-1).
//
// Performance optimizations:
// - Uses branchless bit manipulation for keys 1-4 chars (most common case)
// - Avoids conditional branches in the hot path
// - Uses lookup table for power-of-two shard counts (8, 16, 32, 64)
func HashKey(key string, numShards int) uint32 {
	keyLen := len(key)
	if keyLen == 0 {
		return 0
	}

	// Fast path for very short keys (1-4 chars): branchless implementation
	// This is the most common case for environment variable keys
	// Uses conditional moves instead of branches for better pipelining
	if keyLen <= 4 {
		// Load bytes with zero-padding for shorter keys
		// This avoids branches while maintaining correctness
		var b [4]byte
		// Copy key bytes (compiler optimizes this)
		for i := 0; i < keyLen; i++ {
			b[i] = key[i]
		}
		// Combine bytes into a 32-bit value using bit shifts
		hash := uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
		// Mix with length to differentiate keys of different lengths with same prefix
		hash ^= uint32(keyLen) * 2654435761
		// Use bit mask for power-of-two shard counts (most common: 8)
		// This avoids the expensive modulo operation
		if numShards == 8 {
			return hash & 7
		}
		return hash % uint32(numShards)
	}

	// Fast path for short keys (5-8 chars)
	if keyLen <= 8 {
		hash := uint32(key[0])
		hash |= uint32(key[1]) << 8
		hash |= uint32(key[2]) << 16
		hash |= uint32(key[3]) << 24
		// Mix in remaining bytes using multiplicative hashing
		hash ^= uint32(key[4]) * 2654435761
		if keyLen > 5 {
			hash ^= uint32(key[5]) * 2654435761
		}
		if keyLen > 6 {
			hash ^= uint32(key[6]) * 2654435761
		}
		if keyLen > 7 {
			hash ^= uint32(key[7]) * 2654435761
		}
		hash ^= uint32(keyLen) * 2654435761
		if numShards == 8 {
			return hash & 7
		}
		return hash % uint32(numShards)
	}

	// FNV-1a hash for longer keys with sampling
	hash := uint32(2166136261) // FNV offset basis

	// For longer keys, sample first 8 and last 8 characters
	if keyLen <= 16 {
		for i := 0; i < keyLen; i++ {
			hash ^= uint32(key[i])
			hash *= 16777619 // FNV prime
		}
	} else {
		for i := 0; i < 8; i++ {
			hash ^= uint32(key[i])
			hash *= 16777619
		}
		for i := keyLen - 8; i < keyLen; i++ {
			hash ^= uint32(key[i])
			hash *= 16777619
		}
	}
	if numShards == 8 {
		return hash & 7
	}
	return hash % uint32(numShards)
}

// getShard returns the shard for a given key using HashKey.
func getShard(key string) *internShard {
	return &internShards[HashKey(key, numShards)]
}

// InternKey returns an interned copy of the key string if available,
// or stores and returns the input key. This reduces allocations when
// the same keys are parsed repeatedly.
// This implementation uses sharded caches with thread-safe access for better
// concurrency performance.
//
// Optimization: Uses sync.Mutex instead of sync.RWMutex because:
// 1. The cache is small (64 entries per shard), making lookup very fast
// 2. RWMutex has higher overhead for the common case of cache hit + small cache
// 3. Simpler lock management improves cache locality
func InternKey(key string) string {
	if len(key) == 0 || len(key) > maxInternKeyLen {
		return key // Don't intern empty or very long keys
	}

	shard := getShard(key)
	shard.mu.Lock()

	// Single lock acquisition - check and potentially insert
	if interned, ok := shard.cache[key]; ok {
		shard.mu.Unlock()
		return interned
	}

	// FIFO eviction if cache is full
	if len(shard.cache) >= maxInternSize {
		// Remove oldest 1/4 of entries
		evictCount := maxInternSize / 4
		for i := 0; i < evictCount && i < len(shard.order); i++ {
			keyToEvict := shard.order[i]
			delete(shard.cache, keyToEvict)
		}
		// Create a new slice to allow GC to reclaim evicted key strings.
		// Simply reslicing (shard.order[evictCount:]) would keep references
		// to evicted keys in the underlying array, preventing GC collection.
		if evictCount >= len(shard.order) {
			shard.order = nil
		} else {
			remaining := len(shard.order) - evictCount
			newOrder := make([]string, 0, maxInternSize)
			newOrder = append(newOrder, shard.order[evictCount:]...)
			shard.order = newOrder
			// Verify slice length matches expected remaining count
			if len(shard.order) != remaining {
				shard.order = shard.order[:0] // Reset on mismatch (defensive)
			}
		}
	}

	shard.cache[key] = key
	shard.order = append(shard.order, key)
	shard.mu.Unlock()
	return key
}

// isValidDefaultKey checks if a key matches the default pattern ^[A-Za-z][A-Za-z0-9_]*$
// This is faster than using regex for the common case.
// This function is used by both Validator and Expander for consistent key validation.
func isValidDefaultKey(key string) bool {
	if len(key) == 0 {
		return false
	}
	// First character must be a letter
	c := key[0]
	if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')) {
		return false
	}
	// Remaining characters must be alphanumeric or underscore
	for i := 1; i < len(key); i++ {
		c := key[i]
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '_') {
			return false
		}
	}
	return true
}

// isVarChar returns true if c is a valid variable name character.
// Used by the expander for parsing variable references.
func isVarChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9') || c == '_'
}

// IsValidJSONKey checks if a key matches the JSON key pattern ^[A-Za-z0-9_@\-.]+$
// This is faster than using regex for the common case.
// Allowed characters: letters, digits, underscore, at-sign, hyphen, dot.
// Note: Square brackets are NOT allowed to prevent key name confusion and
// ambiguity with array index notation.
func IsValidJSONKey(key string) bool {
	if len(key) == 0 {
		return false
	}
	// SECURITY: Reject keys that look like array indices to prevent confusion
	if key[0] >= '0' && key[0] <= '9' && len(key) <= 4 {
		allDigits := true
		for i := 0; i < len(key); i++ {
			if key[i] < '0' || key[i] > '9' {
				allDigits = false
				break
			}
		}
		if allDigits {
			return false // Reject pure numeric keys
		}
	}
	for i := 0; i < len(key); i++ {
		c := key[i]
		// Check allowed characters using a fast path for common cases
		// SECURITY: Square brackets [ ] are explicitly excluded to prevent
		// key confusion attacks and ambiguity with array indexing
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') ||
			c == '_' || c == '@' || c == '-' || c == '.' {
			continue
		}
		return false
	}
	return true
}

// ClearInternCache clears the key interning cache.
// This is useful for long-running applications that want to release
// memory held by cached keys that are no longer needed.
// This function is safe for concurrent use.
func ClearInternCache() {
	for i := range numShards {
		shard := &internShards[i]
		shard.mu.Lock()
		shard.cache = make(map[string]string, maxInternSize)
		shard.order = nil
		shard.mu.Unlock()
	}
}

// TrimSpace trims leading and trailing whitespace from a string.
// This is an optimized version that returns the original string if no trimming is needed,
// avoiding allocation in the common case where values are already trimmed.
func TrimSpace(s string) string {
	// Fast path for empty string
	if len(s) == 0 {
		return s
	}

	// Find first non-whitespace character
	start := 0
	for start < len(s) {
		c := s[start]
		if c != ' ' && c != '\t' && c != '\n' && c != '\r' {
			break
		}
		start++
	}

	// Find last non-whitespace character
	end := len(s)
	for end > start {
		c := s[end-1]
		if c != ' ' && c != '\t' && c != '\n' && c != '\r' {
			break
		}
		end--
	}

	// Return original if no trimming needed
	if start == 0 && end == len(s) {
		return s
	}

	return s[start:end]
}

// ToUpperASCII converts an ASCII string to uppercase.
// This is faster than strings.ToUpper for ASCII-only strings.
// Uses single-pass algorithm: convert while detecting lowercase.
// Returns the uppercase string (shares backing array if already uppercase).
func ToUpperASCII(s string) string {
	// Single-pass: convert to uppercase while detecting if conversion is needed
	// This avoids the double-pass of check-then-convert
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'a' && c <= 'z' {
			// Found a lowercase character, need to convert
			// Allocate buffer and copy what we've seen so far
			b := make([]byte, len(s))
			for j := 0; j < i; j++ {
				b[j] = s[j]
			}
			// Convert current character
			b[i] = c - 32
			// Continue converting remaining characters
			for j := i + 1; j < len(s); j++ {
				c2 := s[j]
				if c2 >= 'a' && c2 <= 'z' {
					b[j] = c2 - 32
				} else {
					b[j] = c2
				}
			}
			return string(b)
		}
	}
	// No lowercase characters found, return original
	return s
}
