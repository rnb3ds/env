package env

import (
	"io"
	"time"
)

// KeyValidator defines the interface for validating environment variable keys.
// Implementations can enforce naming conventions, security policies, and length limits.
type KeyValidator interface {
	// ValidateKey checks if the key is valid.
	// Returns an error if the key violates any validation rules.
	ValidateKey(key string) error
}

// ValueValidator defines the interface for validating environment variable values.
// Implementations can check for security issues like null bytes or control characters.
type ValueValidator interface {
	// ValidateValue checks if the value is valid.
	// Returns an error if the value violates any validation rules.
	ValidateValue(value string) error
}

// Validator combines key and value validation.
type Validator interface {
	KeyValidator
	ValueValidator
	// ValidateRequired checks that all required keys are present.
	ValidateRequired(keys map[string]bool) error
}

// AuditLogger defines the interface for audit logging.
// Implementations can log to different destinations (file, stdout, external services).
type AuditLogger interface {
	// Log records an audit event.
	Log(action AuditAction, key, reason string, success bool) error

	// LogError records an error event.
	LogError(action AuditAction, key, errMsg string) error

	// LogWithFile records an audit event with file information.
	LogWithFile(action AuditAction, key, file, reason string, success bool) error

	// LogWithDuration records an audit event with timing information.
	LogWithDuration(action AuditAction, key, reason string, success bool, duration time.Duration) error

	// Close closes the audit logger and releases resources.
	Close() error
}

// VariableExpander defines the interface for variable expansion.
// Implementations can support different expansion syntaxes ($VAR, ${VAR}, etc.).
type VariableExpander interface {
	// Expand performs variable expansion on the input string.
	// Returns the expanded string or an error if expansion fails.
	Expand(s string) (string, error)
}

// EnvParser defines the interface for parsing environment files.
type EnvParser interface {
	// Parse reads and parses environment variables from an io.Reader.
	// The filename parameter is used for error messages and audit logging.
	Parse(r io.Reader, filename string) (map[string]string, error)
}

// EnvStorage defines the interface for storing and retrieving environment variables.
type EnvStorage interface {
	// Get retrieves a value by key. Returns the value and whether it exists.
	Get(key string) (string, bool)

	// Set stores a value for a key.
	Set(key, value string)

	// Delete removes a key.
	Delete(key string)

	// Keys returns all keys in the storage.
	Keys() []string

	// Len returns the number of entries.
	Len() int

	// ToMap returns a copy of all values as a regular map.
	ToMap() map[string]string

	// Clear removes all entries.
	Clear()
}

// ============================================================================
// Fine-grained Interfaces (Interface Segregation Principle)
// ============================================================================

// EnvFileLoader handles loading environment variables from files and strings.
// Use this interface when you only need file loading capabilities.
type EnvFileLoader interface {
	// LoadFiles loads environment variables from multiple files.
	// If no filenames are provided, defaults to ".env".
	LoadFiles(filenames ...string) error
}

// EnvGetter handles reading environment variable values.
// Use this interface when you only need read access to variables.
type EnvGetter interface {
	// Get retrieves a value by key with optional default.
	GetString(key string, defaultValue ...string) string

	// Lookup retrieves a value by key and reports whether it exists.
	Lookup(key string) (string, bool)

	// Keys returns all keys.
	Keys() []string

	// All returns all environment variables as a map.
	All() map[string]string
}

// EnvSetter handles writing environment variable values.
// Use this interface when you only need write access to variables.
// Note: Set and Delete methods return error for validation failures,
// unlike EnvStorage.Set which is a simple storage operation.
type EnvSetter interface {
	// Set sets a value for a key with validation.
	Set(key, value string) error

	// Delete removes a key.
	Delete(key string) error
}

// EnvApplicator handles applying loaded variables to the process environment.
// Use this interface when you only need to apply variables to os.Environ.
type EnvApplicator interface {
	// Apply applies all loaded variables to the process environment.
	Apply() error
}

// EnvCloser handles lifecycle management.
// Use this interface when you only need to close and release resources.
type EnvCloser interface {
	// Close closes the loader and releases resources.
	Close() error
}

// EnvLoader defines the full interface for loading and managing environment variables.
// It combines all fine-grained interfaces for convenience.
//
// For new code, consider using the fine-grained interfaces (EnvFileLoader, EnvGetter,
// EnvSetter, EnvApplicator, EnvCloser) which follow the Interface Segregation Principle
// and allow for more precise dependency declarations.
type EnvLoader interface {
	EnvFileLoader
	EnvGetter
	EnvSetter
	EnvApplicator
	EnvCloser
}
