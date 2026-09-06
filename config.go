package env

import (
	"regexp"
	"strconv"

	"github.com/cybergodev/env/internal"
)

// ============================================================================
// Configuration Types (Nested Structures)
// ============================================================================

// FileConfig controls file loading behavior.
type FileConfig struct {
	// Filenames lists the files to load. Defaults to [".env"].
	Filenames []string
	// FailOnMissingFile causes LoadFiles to return an error when a file does not exist.
	FailOnMissingFile bool
	// OverwriteExisting allows overwriting environment variables that are already set.
	OverwriteExisting bool
	// AutoApply calls Apply automatically after loading files in New.
	AutoApply bool
}

// ValidationConfig controls key and value validation.
type ValidationConfig struct {
	// RequiredKeys lists keys that must be present after loading. Validate returns ErrMissingRequired if any are missing.
	RequiredKeys []string
	// AllowedKeys restricts which keys are permitted. Empty means all keys are allowed.
	AllowedKeys []string
	// ForbiddenKeys lists keys that are always rejected. Set returns ErrForbiddenKey if any are present.
	ForbiddenKeys []string
	// KeyPattern is a custom regexp for validating key names. Nil uses DefaultKeyPattern.
	KeyPattern *regexp.Regexp
	// ValidateValues enables security validation of values (e.g., injection detection).
	ValidateValues bool
	// ValidateUTF8 enables validation that all values are valid UTF-8.
	ValidateUTF8 bool
}

// LimitsConfig controls size and count limits for parsing.
type LimitsConfig struct {
	// MaxFileSize is the maximum allowed file size in bytes.
	MaxFileSize int64
	// MaxVariables is the maximum number of variables per file.
	MaxVariables int
	// MaxLineLength is the maximum allowed line length in bytes.
	MaxLineLength int
	// MaxKeyLength is the maximum allowed key length in bytes.
	MaxKeyLength int
	// MaxValueLength is the maximum allowed value length in bytes.
	MaxValueLength int
	// MaxExpansionDepth limits recursive variable expansion depth.
	MaxExpansionDepth int
}

// JSONConfig controls JSON parsing behavior.
type JSONConfig struct {
	// JSONNullAsEmpty converts JSON null values to empty strings.
	JSONNullAsEmpty bool
	// JSONNumberAsString converts JSON numbers to strings instead of rejecting them.
	JSONNumberAsString bool
	// JSONBoolAsString converts JSON booleans to strings instead of rejecting them.
	JSONBoolAsString bool
	// JSONMaxDepth limits nesting depth for JSON objects. Must be between 1 and 100.
	JSONMaxDepth int
}

// YAMLConfig controls YAML parsing behavior.
type YAMLConfig struct {
	// YAMLNullAsEmpty converts YAML null/~ values to empty strings.
	YAMLNullAsEmpty bool
	// YAMLNumberAsString converts YAML numbers to strings instead of rejecting them.
	YAMLNumberAsString bool
	// YAMLBoolAsString converts YAML booleans to strings instead of rejecting them.
	YAMLBoolAsString bool
	// YAMLMaxDepth limits nesting depth for YAML mappings. Must be between 1 and 100.
	YAMLMaxDepth int
}

// ExpansionScope controls which variables are visible to ${VAR} and $VAR
// expansion in values.
type ExpansionScope int

const (
	// ExpansionFileThenProcess resolves references against the variables in
	// the parsed file first, then falls back to the process environment
	// (os.LookupEnv). This is the zero value and matches traditional dotenv
	// semantics.
	ExpansionFileThenProcess ExpansionScope = iota
	// ExpansionFileOnly restricts expansion to variables defined in the
	// parsed file(s) alone. References to variables that exist only in the
	// process environment expand to the empty string. Use this when
	// configuration files come from a less-trusted source: it prevents the
	// file from capturing unrelated process secrets (cloud credentials,
	// tokens) into values that are later logged or persisted (SEC-03).
	ExpansionFileOnly
)

// ParsingConfig controls general parsing behavior.
type ParsingConfig struct {
	// AllowExportPrefix allows the "export KEY=value" shell syntax in .env files.
	AllowExportPrefix bool
	// AllowYamlSyntax enables YAML-style values (lists, maps) in .env files.
	AllowYamlSyntax bool
	// ExpandVariables enables ${VAR} and $VAR expansion in values.
	ExpandVariables bool
	// ExpansionScope restricts which variables ${VAR} references can resolve
	// from. The default (ExpansionFileThenProcess) resolves file-local
	// variables first and then falls back to the process environment.
	// Set to ExpansionFileOnly to prevent an untrusted configuration file
	// from reading unrelated process environment variables (SEC-03).
	ExpansionScope ExpansionScope
}

// ComponentConfig holds custom component implementations and advanced options.
type ComponentConfig struct {
	// CustomValidator replaces the built-in key/value validator. Nil uses the default.
	CustomValidator Validator
	// CustomExpander replaces the built-in variable expander. Nil uses the default.
	CustomExpander VariableExpander
	// CustomAuditor replaces the built-in audit logger. Nil uses the default.
	CustomAuditor AuditLogger
	// FileSystem overrides file system operations. Nil uses OSFileSystem. Useful for testing.
	FileSystem FileSystem
	// AuditHandler receives audit events when AuditEnabled is true.
	AuditHandler AuditHandler
	// AuditEnabled activates audit logging for all loader operations.
	AuditEnabled bool
	// Prefix restricts processing to environment variables that start with this prefix.
	Prefix string
}

// ============================================================================
// Configuration
// ============================================================================

// Config holds all configuration options for the Loader.
//
// Configuration is organized into nested structures for better organization
// while maintaining backward compatibility through Go's struct embedding.
// You can access fields either way:
//
//	// Old way (still works via field promotion):
//	cfg.Filenames = []string{".env"}
//	cfg.MaxFileSize = 1024
//
//	// New way (recommended for clarity):
//	cfg.FileConfig.Filenames = []string{".env"}
//	cfg.LimitsConfig.MaxFileSize = 1024
//
// For sensible defaults, use DefaultConfig():
//
//	cfg := env.DefaultConfig()
//	cfg.FileConfig.Filenames = []string{".env"}
type Config struct {
	// FileConfig controls file loading behavior.
	FileConfig
	// ValidationConfig controls key and value validation rules.
	ValidationConfig
	// LimitsConfig controls size and count limits for parsing.
	LimitsConfig
	// JSONConfig controls JSON file parsing options.
	JSONConfig
	// YAMLConfig controls YAML file parsing options.
	YAMLConfig
	// ParsingConfig controls general parsing behavior for .env files.
	ParsingConfig
	// ComponentConfig holds custom components and advanced options.
	ComponentConfig
}

// Validate validates the configuration and returns an error if invalid.
// This method performs all configuration validation by delegating to focused sub-methods.
func (c *Config) Validate() error {
	if err := c.validateLimits(); err != nil {
		return err
	}
	if err := c.validateFormatConfig(); err != nil {
		return err
	}
	if err := c.validateAdvancedOptions(); err != nil {
		return err
	}
	return nil
}

// validateLimits validates size and count limits.
func (c *Config) validateLimits() error {
	return validateConfigLimits(
		c.MaxFileSize,
		c.MaxLineLength,
		c.MaxKeyLength,
		c.MaxValueLength,
		c.MaxVariables,
		c.MaxExpansionDepth,
	)
}

// validateFormatConfig validates JSON and YAML format configuration.
func (c *Config) validateFormatConfig() error {
	if c.JSONMaxDepth <= 0 || c.JSONMaxDepth > 100 {
		return newValidationError("JSONMaxDepth", "", "range", "must be between 1 and 100")
	}
	if c.YAMLMaxDepth <= 0 || c.YAMLMaxDepth > 100 {
		return newValidationError("YAMLMaxDepth", "", "range", "must be between 1 and 100")
	}
	return nil
}

// validateAdvancedOptions validates advanced options like custom key patterns.
func (c *Config) validateAdvancedOptions() error {
	if c.ExpansionScope != ExpansionFileThenProcess && c.ExpansionScope != ExpansionFileOnly {
		return newValidationError("ExpansionScope", strconv.Itoa(int(c.ExpansionScope)), "enum",
			"must be ExpansionFileThenProcess or ExpansionFileOnly")
	}
	if c.KeyPattern != nil {
		return validateKeyPattern(c.KeyPattern)
	}
	return nil
}

// validateKeyPattern validates a custom key pattern.
func validateKeyPattern(pattern *regexp.Regexp) error {
	// Test that the pattern can match a typical valid key
	testKey := "TEST_KEY"
	if !pattern.MatchString(testKey) {
		return newValidationError("KeyPattern", pattern.String(), "valid_pattern",
			"key pattern must be able to match valid keys like TEST_KEY")
	}

	// Test that the pattern does not match empty strings
	if pattern.MatchString("") {
		return newValidationError("KeyPattern", pattern.String(), "reject_empty",
			"key pattern must not match empty strings")
	}

	// Test that the pattern does not match keys starting with numbers
	// (standard env var convention: must start with letter)
	if pattern.MatchString("123_INVALID") {
		return newValidationError("KeyPattern", pattern.String(), "reject_numeric_start",
			"key pattern must not match keys starting with numbers")
	}

	// SECURITY (SEC-06): a pattern that also matches keys containing
	// separators or control characters lets such keys pass Set(). The .env
	// emitter cannot represent them, previously producing output that
	// re-parses as different (attacker-chosen) lines — round-trip injection.
	// Reject patterns that match any of these probes.
	for _, probe := range []string{"A=B", "A:B", "A\nB", "A\rB", "A\x00B"} {
		if pattern.MatchString(probe) {
			return newValidationError("KeyPattern", pattern.String(), "reject_unsafe_chars",
				"key pattern must not match keys containing '=', ':', newline, or control characters")
		}
	}

	return nil
}

// IsZero returns true if the Config appears to be uninitialized (all fields zero).
// This is useful to determine if DefaultConfig() should be applied.
//
// Note: A partially-initialized Config may not be detected as zero.
// Always start from DefaultConfig() for custom configurations:
//
//	cfg := env.DefaultConfig()
//	cfg.Filenames = []string{".env.production"}
func (c *Config) IsZero() bool {
	// Check numeric limits (non-zero defaults in DefaultConfig)
	if c.MaxFileSize != 0 || c.MaxVariables != 0 ||
		c.MaxLineLength != 0 || c.MaxKeyLength != 0 ||
		c.MaxValueLength != 0 || c.MaxExpansionDepth != 0 ||
		c.JSONMaxDepth != 0 || c.YAMLMaxDepth != 0 {
		return false
	}

	// Check boolean fields (any non-zero value means initialized)
	if c.ValidateValues || c.ValidateUTF8 || c.JSONNullAsEmpty ||
		c.JSONNumberAsString || c.JSONBoolAsString ||
		c.YAMLNullAsEmpty || c.YAMLNumberAsString ||
		c.YAMLBoolAsString || c.AllowExportPrefix || c.ExpandVariables ||
		c.OverwriteExisting || c.AutoApply || c.FailOnMissingFile ||
		c.AllowYamlSyntax || c.AuditEnabled {
		return false
	}

	// ExpansionScope is an enum, not a bool: any value other than the zero
	// value (ExpansionFileThenProcess) means initialized. Without this check
	// a Config whose only non-default field is the scope was mistaken for
	// zero and silently replaced by DefaultConfig() in New(), dropping the
	// SEC-03 file-only expansion restriction.
	if c.ExpansionScope != ExpansionFileThenProcess {
		return false
	}

	// Check pointer/interface fields (non-nil means initialized)
	if c.KeyPattern != nil || c.FileSystem != nil ||
		c.CustomValidator != nil || c.CustomExpander != nil ||
		c.CustomAuditor != nil || c.AuditHandler != nil {
		return false
	}

	// Check string fields (non-empty means partially initialized)
	if c.Prefix != "" {
		return false
	}

	// Check slices (non-nil means partially initialized)
	if c.Filenames != nil || c.RequiredKeys != nil ||
		c.AllowedKeys != nil || c.ForbiddenKeys != nil {
		return false
	}

	return true
}

// ============================================================================
// Configuration Factories
// ============================================================================

// defaultStructuredMaxDepth is the default nesting-depth limit for JSON and
// YAML documents. It backs DefaultConfig (JSONMaxDepth/YAMLMaxDepth),
// UnmarshalMap's structured paths, and the structured parsers' defensive
// fallback when constructed with a non-positive depth.
const defaultStructuredMaxDepth = 10

// DefaultConfig returns a Config with secure default values.
// These defaults are suitable for high-security environments.
func DefaultConfig() Config {
	return Config{
		FileConfig: FileConfig{
			Filenames:         []string{".env"},
			FailOnMissingFile: false,
			OverwriteExisting: false,
			AutoApply:         false,
		},
		ValidationConfig: ValidationConfig{
			RequiredKeys:   nil,
			AllowedKeys:    nil,
			ForbiddenKeys:  nil,
			KeyPattern:     DefaultKeyPattern,
			ValidateValues: true,
		},
		LimitsConfig: LimitsConfig{
			MaxFileSize:       DefaultMaxFileSize,
			MaxVariables:      DefaultMaxVariables,
			MaxLineLength:     DefaultMaxLineLength,
			MaxKeyLength:      DefaultMaxKeyLength,
			MaxValueLength:    DefaultMaxValueLength,
			MaxExpansionDepth: DefaultMaxExpansionDepth,
		},
		JSONConfig: JSONConfig{
			JSONNullAsEmpty:    true,
			JSONNumberAsString: true,
			JSONBoolAsString:   true,
			JSONMaxDepth:       defaultStructuredMaxDepth,
		},
		YAMLConfig: YAMLConfig{
			YAMLNullAsEmpty:    true,
			YAMLNumberAsString: true,
			YAMLBoolAsString:   true,
			YAMLMaxDepth:       defaultStructuredMaxDepth,
		},
		ParsingConfig: ParsingConfig{
			AllowExportPrefix: true,
			AllowYamlSyntax:   false,
			ExpandVariables:   true,
		},
		ComponentConfig: ComponentConfig{
			CustomValidator: nil,
			CustomExpander:  nil,
			CustomAuditor:   nil,
			FileSystem:      nil,
			AuditHandler:    nil,
			AuditEnabled:    false,
			Prefix:          "",
		},
	}
}

// DevelopmentConfig returns a Config optimized for development environments.
// This configuration prioritizes developer experience and flexibility:
//   - FailOnMissingFile: false (graceful handling of missing .env files)
//   - OverwriteExisting: true (easy iteration during development)
//   - AllowYamlSyntax: true (supports YAML-style values)
//   - Relaxed file-size limit (10MB; variable count keeps the default 500)
//   - Value validation ENABLED for security (prevents injection attacks)
//
// Example:
//
//	cfg := env.DevelopmentConfig()
//	cfg.FileConfig.Filenames = []string{".env.development"}
//	loader, err := env.New(cfg)
func DevelopmentConfig() Config {
	cfg := DefaultConfig()
	cfg.FileConfig.FailOnMissingFile = false
	cfg.FileConfig.OverwriteExisting = true
	cfg.ParsingConfig.AllowYamlSyntax = true
	cfg.LimitsConfig.MaxFileSize = 10 * 1024 * 1024
	cfg.LimitsConfig.MaxVariables = 500
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
//	    cfg.FileConfig.Filenames = []string{".env.test"}
//	    loader, err := env.New(cfg)
//	    if err != nil {
//	        t.Fatal(err)
//	    }
//	    defer loader.Close()
//	}
func TestingConfig() Config {
	cfg := DefaultConfig()
	cfg.FileConfig.FailOnMissingFile = false
	cfg.FileConfig.OverwriteExisting = true
	cfg.LimitsConfig.MaxFileSize = 64 * 1024 // 64KB
	cfg.LimitsConfig.MaxVariables = 50
	cfg.ComponentConfig.AuditEnabled = false
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
//	cfg.FileConfig.Filenames = []string{"app.env"}
//	cfg.ComponentConfig.AuditHandler = env.NewJSONAuditHandler(os.Stdout)
//	loader, err := env.New(cfg)
func ProductionConfig() Config {
	cfg := DefaultConfig()
	cfg.FileConfig.FailOnMissingFile = true
	cfg.FileConfig.OverwriteExisting = false
	cfg.ComponentConfig.AuditEnabled = true
	cfg.ValidationConfig.ValidateValues = true
	cfg.LimitsConfig.MaxFileSize = 64 * 1024 // 64KB
	cfg.LimitsConfig.MaxVariables = 50
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
