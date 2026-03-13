package env

import (
	"regexp"

	"github.com/cybergodev/env/internal"
)

// ============================================================================
// Security Limits
// ============================================================================

// Default security limits for high-security configurations.
// These values are intentionally conservative to prevent various attacks.
const (
	// DefaultMaxFileSize is the maximum allowed file size (2 MB).
	DefaultMaxFileSize int64 = 2 * 1024 * 1024

	// DefaultMaxLineLength is the maximum allowed line length.
	DefaultMaxLineLength int = 1024

	// DefaultMaxKeyLength is the maximum allowed key length.
	DefaultMaxKeyLength int = 64

	// DefaultMaxValueLength is the maximum allowed value length.
	DefaultMaxValueLength int = 4096

	// DefaultMaxVariables is the maximum number of variables per file.
	DefaultMaxVariables int = 500

	// DefaultMaxExpansionDepth is the maximum variable expansion depth.
	DefaultMaxExpansionDepth int = 5
)

// Hard limits that cannot be exceeded even with custom configuration.
// These are re-exported from internal/limits for public API compatibility.
const (
	// HardMaxFileSize is the absolute maximum file size (100 MB).
	HardMaxFileSize = internal.HardMaxFileSize

	// HardMaxLineLength is the absolute maximum line length.
	HardMaxLineLength = internal.HardMaxLineLength

	// HardMaxKeyLength is the absolute maximum key length.
	HardMaxKeyLength = internal.HardMaxKeyLength

	// HardMaxValueLength is the absolute maximum value length.
	HardMaxValueLength = internal.HardMaxValueLength

	// HardMaxVariables is the absolute maximum variables per file.
	HardMaxVariables = internal.HardMaxVariables

	// HardMaxExpansionDepth is the absolute maximum expansion depth.
	HardMaxExpansionDepth = internal.HardMaxExpansionDepth
)

// ============================================================================
// Default Patterns
// ============================================================================

// DefaultKeyPattern is the default pattern for valid keys.
// Set to nil to use fast byte-level validation (isValidDefaultKey) instead of regex.
// This provides ~10x performance improvement for key validation.
// The regex pattern ^[A-Za-z][A-Za-z0-9_]*$ is still available for reference:
//
//	var defaultKeyPatternRegex = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*$`)
var DefaultKeyPattern *regexp.Regexp = nil

// DefaultForbiddenKeys contains keys that could affect system behavior.
var DefaultForbiddenKeys = map[string]bool{
	"PATH":                  true,
	"LD_PRELOAD":            true,
	"LD_LIBRARY_PATH":       true,
	"LD_DEBUG":              true,
	"LD_AUDIT":              true,
	"LD_PRELOAD_32":         true,
	"LD_PRELOAD_64":         true,
	"LD_LIBRARY_PATH_32":    true,
	"LD_LIBRARY_PATH_64":    true,
	"DYLD_INSERT_LIBRARIES": true,
	"DYLD_LIBRARY_PATH":     true,
	"IFS":                   true,
	"SHELL":                 true,
	"ENV":                   true,
	"BASH_ENV":              true,
	"PERL5OPT":              true,
	"PYTHONPATH":            true,
	"RUBYLIB":               true,
	"NODE_PATH":             true,
}
