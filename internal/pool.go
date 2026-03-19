// Package internal provides shared resource pools for the env package.
package internal

import (
	"strings"
	"sync"
)

// ============================================================================
// Shared Strings.Builder Pool
// ============================================================================

// builderPool provides a pool of reusable strings.Builder instances.
// This is shared across multiple packages to reduce allocations for
// string building operations.
var builderPool = sync.Pool{
	New: func() interface{} {
		return new(strings.Builder)
	},
}

// GetBuilder retrieves a strings.Builder from the shared pool.
// Returns a fallback builder if the pool returns an unexpected type.
func GetBuilder() *strings.Builder {
	sb, ok := builderPool.Get().(*strings.Builder)
	if !ok {
		// Fallback: create new builder if pool returns unexpected type
		return new(strings.Builder)
	}
	sb.Reset()
	return sb
}

// PutBuilder returns a strings.Builder to the shared pool.
// Builders with capacity exceeding MaxPooledBuilderSize are discarded
// to prevent memory bloat.
// The builder is reset before returning to pool for hygiene and to allow
// GC to reclaim the previous string data.
//
// SECURITY NOTE: This function depends on strings.Builder.Reset() preserving
// the underlying buffer capacity. The Go standard library guarantees this
// behavior (Reset() only sets len to 0, does not free the buffer).
// If this behavior changes, we would need explicit capacity tracking
// to prevent pooling of oversized builders.
func PutBuilder(sb *strings.Builder) {
	if sb == nil {
		return
	}
	// Reset before capacity check and pooling for hygiene
	sb.Reset()
	// Don't pool very large builders
	if sb.Cap() <= MaxPooledBuilderSize {
		builderPool.Put(sb)
	}
}

// ============================================================================
// Byte Slice Pool for Value Parsing
// ============================================================================

// byteSlicePool provides a pool of reusable byte slices for value parsing.
// This reduces allocations when converting byte slices to strings.
var byteSlicePool = sync.Pool{
	New: func() interface{} {
		buf := make([]byte, 0, 256)
		return &buf
	},
}

// GetByteSlice retrieves a byte slice from the pool.
// Returns a fallback slice if the pool returns an unexpected type.
func GetByteSlice() *[]byte {
	buf, ok := byteSlicePool.Get().(*[]byte)
	if !ok {
		b := make([]byte, 0, 256)
		return &b
	}
	*buf = (*buf)[:0]
	return buf
}

// PutByteSlice returns a byte slice to the pool.
// Slices with capacity exceeding MaxPooledByteSliceSize are discarded.
//
// SECURITY NOTE: This function checks capacity before pooling to prevent
// memory bloat from holding onto large buffers. The capacity check is
// performed after resetting the slice length to 0.
func PutByteSlice(buf *[]byte) {
	if buf == nil {
		return
	}
	// Don't pool very large slices
	if cap(*buf) <= MaxPooledByteSliceSize {
		byteSlicePool.Put(buf)
	}
}

// ============================================================================
// KeysToUpper Map Pool
// ============================================================================

// keysToUpperPool provides a pool of reusable maps for KeysToUpper operations.
// This reduces allocations when converting map keys to uppercase for comparison.
var keysToUpperPool = sync.Pool{
	New: func() interface{} {
		return make(map[string]bool, 64)
	},
}

// getKeysToUpperMap retrieves a map from the pool.
// Returns a fallback map if the pool returns an unexpected type.
func getKeysToUpperMap() map[string]bool {
	m, ok := keysToUpperPool.Get().(map[string]bool)
	if !ok {
		// Fallback: create new map if pool returns unexpected type
		return make(map[string]bool, 64)
	}
	return m
}

// putKeysToUpperMap returns a map to the pool after clearing it.
// Maps with more entries than MaxPooledMapSize are discarded to prevent memory bloat.
//
// SECURITY NOTE: Size is checked BEFORE clearing because clear() sets len to 0.
// We preserve the original size to make the pooling decision.
func putKeysToUpperMap(m map[string]bool) {
	if m == nil {
		return
	}
	// Check size before clearing - after clear() len(m) will be 0
	size := len(m)

	// SECURITY: Use clear() builtin (Go 1.21+) for guaranteed complete clearing.
	// This is O(1) and prevents partial clears that could leave stale entries.
	clear(m)

	// Don't pool very large maps (check original size before clearing)
	// Use <= to include maps at exactly MaxPooledMapSize
	if size <= MaxPooledMapSize {
		keysToUpperPool.Put(m)
	}
}
