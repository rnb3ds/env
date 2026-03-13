// Package internal provides security limits for the env package.
package internal

// Hard security limits that cannot be exceeded.
// These are absolute maximums for safety and are used across all packages.
const (
	// HardMaxFileSize is the absolute maximum file size (100 MB).
	HardMaxFileSize int64 = 100 * 1024 * 1024

	// HardMaxLineLength is the absolute maximum line length.
	HardMaxLineLength int = 64 * 1024

	// HardMaxKeyLength is the absolute maximum key length.
	HardMaxKeyLength int = 1024

	// HardMaxValueLength is the absolute maximum value length.
	HardMaxValueLength int = 1024 * 1024

	// HardMaxVariables is the absolute maximum variables per file.
	HardMaxVariables int = 10000

	// HardMaxExpansionDepth is the absolute maximum expansion depth.
	HardMaxExpansionDepth int = 20
)

// Pool size limits for sync.Pool objects.
// Objects larger than these limits are not returned to pools to prevent
// memory bloat from holding onto large allocations.
const (
	// MaxPooledBuilderSize is the maximum capacity for pooled strings.Builder objects.
	// Builders larger than this are discarded instead of pooled.
	MaxPooledBuilderSize = 16 * 1024 // 16KB

	// MaxPooledScannerBufferSize is the maximum capacity for pooled scanner buffers.
	// Buffers larger than this are discarded instead of pooled.
	MaxPooledScannerBufferSize = 256 * 1024 // 256KB

	// MaxPooledByteSliceSize is the maximum capacity for pooled byte slices.
	// Slices larger than this are discarded instead of pooled.
	MaxPooledByteSliceSize = 4096 // 4KB

	// MaxPooledMapSize is the maximum size for pooled map objects.
	// Maps with more entries than this are discarded instead of pooled.
	MaxPooledMapSize = 128
)
