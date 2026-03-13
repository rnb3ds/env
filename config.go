package env

import (
	"regexp"

	"github.com/cybergodev/env/internal"
)

// ============================================================================
// Configuration
// ============================================================================

// Config holds all configuration options for the Loader.
//
// Configuration is designed to be zero-value friendly. You can create
// a Config with only the fields you need:
//
//	cfg := env.Config{
//	    Filenames:         []string{".env"},
//	    OverwriteExisting: true,
//	}
//	loader, err := env.New(cfg)
//
// For sensible defaults, use DefaultConfig():
//
//	cfg := env.DefaultConfig()
//	cfg.Filenames = []string{".env"}
type Config struct {
	// === File Handling ===
	Filenames         []string // Files to load (default: [".env"])
	FailOnMissingFile bool     // Return error if file doesn't exist
	OverwriteExisting bool     // Overwrite existing environment variables
	AutoApply         bool     // Auto-apply to os.Environ

	// === Validation ===
	RequiredKeys  []string // Require these keys to be present
	AllowedKeys   []string // Only allow these keys (empty = all allowed)
	ForbiddenKeys []string // Always forbid these keys

	// === Variable Expansion ===
	ExpandVariables bool // Expand ${VAR} references

	// === Audit ===
	AuditEnabled bool
	AuditHandler AuditHandler

	// === Limits ===
	MaxFileSize    int64 // Maximum file size in bytes
	MaxVariables   int   // Maximum variables per file
	ValidateValues bool  // Validate values for security issues

	// === Security Options (flattened from SecurityConfig) ===
	KeyPattern        *regexp.Regexp // Pattern for valid keys (nil = default)
	AllowExportPrefix bool           // Allow "export KEY=value" syntax
	AllowYamlSyntax   bool           // Allow YAML-style values in .env

	// === Parsing Limits (flattened from ParsingConfig) ===
	MaxLineLength     int // Maximum line length
	MaxKeyLength      int // Maximum key length
	MaxValueLength    int // Maximum value length
	MaxExpansionDepth int // Maximum variable expansion depth

	// === JSON Parsing Options (flattened from ParsingConfig) ===
	JSONNullAsEmpty    bool // Convert null to empty string
	JSONNumberAsString bool // Convert numbers to strings
	JSONBoolAsString   bool // Convert booleans to strings
	JSONMaxDepth       int  // Maximum nesting depth

	// === YAML Parsing Options (flattened from ParsingConfig) ===
	YAMLNullAsEmpty    bool // Convert null/~ to empty string
	YAMLNumberAsString bool // Convert numbers to strings
	YAMLBoolAsString   bool // Convert booleans to strings
	YAMLMaxDepth       int  // Maximum nesting depth

	// === Advanced Options ===
	Prefix     string     // Only process vars with this prefix
	FileSystem FileSystem // Custom file system (for testing)
}

// Validate validates the configuration and returns an error if invalid.
func (c *Config) Validate() error {
	if err := validateConfigLimits(
		c.MaxFileSize,
		c.MaxLineLength,
		c.MaxKeyLength,
		c.MaxValueLength,
		c.MaxVariables,
		c.MaxExpansionDepth,
	); err != nil {
		return err
	}

	// Validate key pattern if provided
	// Test that the pattern can match a typical valid key
	if c.KeyPattern != nil {
		testKey := "TEST_KEY"
		if !c.KeyPattern.MatchString(testKey) {
			return newValidationError("KeyPattern", c.KeyPattern.String(), "valid_pattern",
				"key pattern must be able to match valid keys like TEST_KEY")
		}
	}

	return nil
}

// ============================================================================
// Configuration Factories
// ============================================================================

// DefaultConfig returns a Config with secure default values.
// These defaults are suitable for high-security environments.
func DefaultConfig() Config {
	return Config{
		// File handling
		Filenames:         []string{".env"},
		FailOnMissingFile: false,
		OverwriteExisting: false,
		AutoApply:         false,

		// Validation
		RequiredKeys:  nil,
		AllowedKeys:   nil,
		ForbiddenKeys: nil,

		// Variable expansion
		ExpandVariables: true,

		// Audit
		AuditEnabled: false,

		// Limits
		MaxFileSize:    DefaultMaxFileSize,
		MaxVariables:   DefaultMaxVariables,
		ValidateValues: true,

		// Security options
		KeyPattern:        DefaultKeyPattern,
		AllowExportPrefix: true,
		AllowYamlSyntax:   false,

		// Parsing limits
		MaxLineLength:     DefaultMaxLineLength,
		MaxKeyLength:      DefaultMaxKeyLength,
		MaxValueLength:    DefaultMaxValueLength,
		MaxExpansionDepth: DefaultMaxExpansionDepth,

		// JSON options
		JSONNullAsEmpty:    true,
		JSONNumberAsString: true,
		JSONBoolAsString:   true,
		JSONMaxDepth:       10,

		// YAML options
		YAMLNullAsEmpty:    true,
		YAMLNumberAsString: true,
		YAMLBoolAsString:   true,
		YAMLMaxDepth:       10,

		// Advanced options
		Prefix:     "",
		FileSystem: nil,
	}
}

// DevelopmentConfig returns a Config optimized for development environments.
// This configuration prioritizes developer experience and flexibility:
//   - FailOnMissingFile: false (graceful handling of missing .env files)
//   - OverwriteExisting: true (easy iteration during development)
//   - AllowYamlSyntax: true (supports YAML-style values)
//   - Relaxed size limits (10MB files, 500 variables)
//   - Value validation ENABLED for security (prevents injection attacks)
//
// Example:
//
//	cfg := env.DevelopmentConfig()
//	cfg.Filenames = []string{".env.development"}
//	loader, err := env.New(cfg)
func DevelopmentConfig() Config {
	cfg := DefaultConfig()
	cfg.FailOnMissingFile = false
	cfg.OverwriteExisting = true
	cfg.AllowYamlSyntax = true
	cfg.MaxFileSize = 10 * 1024 * 1024
	cfg.MaxVariables = 500
	// ValidateValues remains true for security - never disable value validation
	return cfg
}

// TestingConfig returns a Config optimized for testing environments.
// This configuration is designed for isolated, repeatable tests:
//   - FailOnMissingFile: false (tests may not have .env files)
//   - OverwriteExisting: true (test isolation)
//   - Compact size limits (test files are typically small)
//   - No audit logging (reduces test noise)
//
// Example:
//
//	func TestSomething(t *testing.T) {
//	    cfg := env.TestingConfig()
//	    cfg.Filenames = []string{".env.test"}
//	    loader, err := env.NewIsolated(cfg)
//	    if err != nil {
//	        t.Fatal(err)
//	    }
//	    defer loader.Close()
//	}
func TestingConfig() Config {
	cfg := DefaultConfig()
	cfg.FailOnMissingFile = false
	cfg.OverwriteExisting = true
	cfg.MaxFileSize = 64 * 1024 // 64KB
	cfg.MaxVariables = 50
	cfg.AuditEnabled = false
	return cfg
}

// ProductionConfig returns a Config optimized for production environments.
// This configuration provides maximum security for production deployments:
//   - FailOnMissingFile: true (fail fast on configuration errors)
//   - AuditEnabled: true (compliance and security monitoring)
//   - Strict size limits (64KB files, 50 variables)
//   - Value validation enabled
//
// Example:
//
//	cfg := env.ProductionConfig()
//	cfg.Filenames = []string{"/etc/app/.env"}
//	cfg.AuditHandler = env.NewJSONAuditHandler(os.Stdout)
//	loader, err := env.New(cfg)
func ProductionConfig() Config {
	cfg := DefaultConfig()
	cfg.FailOnMissingFile = true
	cfg.OverwriteExisting = false
	cfg.AuditEnabled = true
	cfg.ValidateValues = true
	cfg.MaxFileSize = 64 * 1024 // 64KB
	cfg.MaxVariables = 50
	return cfg
}

// ============================================================================
// Validation Helpers
// ============================================================================

// validateConfigLimits validates that configuration limits are within acceptable ranges.
func validateConfigLimits(maxSize int64, maxLineLen, maxKeyLen, maxValLen, maxVars, maxDepth int) error {
	if maxSize <= 0 {
		return newValidationError("MaxFileSize", "", "positive", "must be positive")
	}
	if maxSize > internal.HardMaxFileSize {
		return newValidationError("MaxFileSize", "", "hard_limit", "exceeds hard limit")
	}
	if maxLineLen <= 0 {
		return newValidationError("MaxLineLength", "", "positive", "must be positive")
	}
	if maxLineLen > internal.HardMaxLineLength {
		return newValidationError("MaxLineLength", "", "hard_limit", "exceeds hard limit")
	}
	if maxKeyLen <= 0 {
		return newValidationError("MaxKeyLength", "", "positive", "must be positive")
	}
	if maxKeyLen > internal.HardMaxKeyLength {
		return newValidationError("MaxKeyLength", "", "hard_limit", "exceeds hard limit")
	}
	if maxValLen <= 0 {
		return newValidationError("MaxValueLength", "", "positive", "must be positive")
	}
	if maxValLen > internal.HardMaxValueLength {
		return newValidationError("MaxValueLength", "", "hard_limit", "exceeds hard limit")
	}
	if maxVars <= 0 {
		return newValidationError("MaxVariables", "", "positive", "must be positive")
	}
	if maxVars > internal.HardMaxVariables {
		return newValidationError("MaxVariables", "", "hard_limit", "exceeds hard limit")
	}
	if maxDepth <= 0 {
		return newValidationError("MaxExpansionDepth", "", "positive", "must be positive")
	}
	if maxDepth > internal.HardMaxExpansionDepth {
		return newValidationError("MaxExpansionDepth", "", "hard_limit", "exceeds hard limit")
	}
	return nil
}

// newValidationError creates a new ValidationError.
func newValidationError(field, value, rule, message string) *ValidationError {
	return &ValidationError{
		Field:   field,
		Value:   MaskSensitiveInString(value),
		Rule:    rule,
		Message: message,
	}
}
