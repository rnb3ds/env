// Package env provides a high-security environment variable library for Go 1.24+.
// where security, concurrent access, and production-grade features are critical.
//
// Example usage:
//
//	cfg := env.DefaultConfig()
//	cfg.Filenames = []string{".env"}
//	cfg.AutoApply = true
//
//	loader, err := env.New(cfg) // Files are auto-loaded
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer loader.Close()
//
//	// GetString a value
//	value := loader.GetString("DATABASE_URL")
package env

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/cybergodev/env/internal"
)

// sliceElement is a type constraint for supported slice element types.
type sliceElement interface {
	string | int | int64 | uint | uint64 | bool | float64 | time.Duration
}

// Loader is the main type for loading and managing environment variables.
// It provides thread-safe access to environment variables with full
// security validation, audit logging, and error handling.
type Loader struct {
	config      Config
	factory     *ComponentFactory
	ownsFactory bool
	validator   Validator
	auditor     AuditLogger
	expander    VariableExpander
	parsers     map[FileFormat]EnvParser
	fs          FileSystem

	mu       sync.RWMutex
	vars     *secureMap
	applied  bool
	closed   bool
	loadTime time.Time
}

// Compile-time check that Loader implements EnvLoader.
var _ EnvLoader = (*Loader)(nil)

// New creates a new Loader with the given configuration.
// If cfg.Filenames is non-empty, files are automatically loaded.
// If cfg.AutoApply is true, loaded variables are also applied to os.Environ.
func New(cfg Config) (*Loader, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	// Use default file system if not specified
	fs := cfg.FileSystem
	if fs == nil {
		fs = DefaultFileSystem
	}

	// Build shared components once
	factory := cfg.buildComponentFactoryWithFS(fs)

	// Create parsers using registry
	parsers, err := createParsers(cfg, factory)
	if err != nil {
		if closeErr := factory.Close(); closeErr != nil {
			// Log close error via factory's auditor if available
			if aud := factory.Auditor(); aud != nil {
				_ = aud.LogError(internal.ActionError, "", "factory cleanup failed: "+closeErr.Error())
			}
		}
		return nil, err
	}

	loader := &Loader{
		config:      cfg,
		factory:     factory,
		ownsFactory: true,
		validator:   factory.Validator(),
		auditor:     factory.Auditor(),
		expander:    factory.internalExpander(),
		parsers:     parsers,
		fs:          fs,
		vars:        newSecureMap(),
	}

	// Auto-load files if Filenames is configured
	if len(cfg.Filenames) > 0 {
		start := time.Now()

		for _, filename := range cfg.Filenames {
			if err := loader.loadFileLocked(filename); err != nil {
				if errors.Is(err, ErrFileNotFound) && !cfg.FailOnMissingFile {
					_ = loader.auditor.LogWithFile(internal.ActionLoad, "", filename, "file not found, skipping", true)
					continue
				}
				if closeErr := loader.Close(); closeErr != nil {
					_ = loader.auditor.LogError(internal.ActionLoad, "", "cleanup failed: "+closeErr.Error())
				}
				return nil, err
			}
		}

		loader.loadTime = time.Now()
		_ = loader.auditor.LogWithDuration(internal.ActionLoad, "", "loaded files", true, time.Since(start))

		// Auto-apply if configured
		if cfg.AutoApply {
			if err := loader.applyLocked(); err != nil {
				if closeErr := loader.Close(); closeErr != nil {
					_ = loader.auditor.LogError(internal.ActionError, "", "cleanup failed: "+closeErr.Error())
				}
				return nil, err
			}
		}
	}

	return loader, nil
}

// LoadFiles loads environment variables from multiple files in order.
// If no filenames are provided, defaults to ".env".
// Files are loaded sequentially; later files can override values from earlier files.
//
// Example:
//
//	// Load default .env file
//	err := loader.LoadFiles()
//
//	// Load specific files
//	err := loader.LoadFiles(".env", ".env.local")
func (l *Loader) LoadFiles(filenames ...string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.closed {
		return ErrClosed
	}

	// Default to .env if no files specified
	if len(filenames) == 0 {
		filenames = []string{".env"}
	}

	start := time.Now()

	for _, filename := range filenames {
		if err := l.loadFileLocked(filename); err != nil {
			if errors.Is(err, ErrFileNotFound) && !l.config.FailOnMissingFile {
				_ = l.auditor.LogWithFile(internal.ActionLoad, "", filename, "file not found, skipping", true)
				continue
			}
			return err
		}
	}

	l.loadTime = time.Now()
	_ = l.auditor.LogWithDuration(internal.ActionLoad, "", "loaded files", true, time.Since(start))

	if l.config.AutoApply {
		return l.applyLocked()
	}

	return nil
}

// loadFileLocked loads a single file. Must be called with lock held.
func (l *Loader) loadFileLocked(filename string) error {
	start := time.Now()

	// SECURITY: Validate file path to prevent path traversal attacks
	if err := validateFilePath(filename); err != nil {
		_ = l.auditor.LogError(internal.ActionSecurity, "", "path validation failed: "+err.Error())
		return err
	}

	file, err := l.fs.Open(filename)
	if err != nil {
		if os.IsNotExist(err) || errors.Is(err, ErrFileNotFound) {
			return newFileError(filename, "open", ErrFileNotFound)
		}
		return newFileError(filename, "open", err)
	}
	defer file.Close()

	// Get file info for size check
	info, err := file.Stat()
	if err != nil {
		return newFileError(filename, "stat", err)
	}

	if info.Size() > l.config.MaxFileSize {
		return &FileError{
			Path:  filename,
			Op:    "size_check",
			Size:  info.Size(),
			Limit: l.config.MaxFileSize,
			Err:   ErrFileTooLarge,
		}
	}

	// Detect file format and select appropriate parser
	format := DetectFormat(filename)
	parser, ok := l.parsers[format]
	if !ok {
		// Fall back to dot-env parser for unknown formats or FormatAuto
		parser = l.parsers[FormatEnv]
	}

	vars, err := parser.Parse(file, filename)
	if err != nil {
		return err
	}

	// Fast path: if no prefix filter and overwrite is enabled, use SetAll directly
	if l.config.Prefix == "" && l.config.OverwriteExisting {
		l.vars.SetAll(vars)
		_ = l.auditor.LogWithDuration(internal.ActionFileAccess, "", "file loaded: "+filename, true, time.Since(start))
		return nil
	}

	// Filter and prepare variables for batch set
	toSet := make(map[string]string, len(vars))
	for key, value := range vars {
		// Check prefix if configured
		if l.config.Prefix != "" && !strings.HasPrefix(key, l.config.Prefix) {
			continue
		}

		// Check overwrite policy
		if _, exists := l.vars.Get(key); exists && !l.config.OverwriteExisting {
			_ = l.auditor.Log(internal.ActionSet, key, "skipped (no overwrite)", false)
			continue
		}

		toSet[key] = value
		_ = l.auditor.Log(internal.ActionSet, key, "loaded", true)
	}

	// Use batch set for better performance
	l.vars.SetAll(toSet)

	_ = l.auditor.LogWithDuration(internal.ActionFileAccess, "", "file loaded: "+filename, true, time.Since(start))
	return nil
}

// Apply applies all loaded variables to the process environment.
func (l *Loader) Apply() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.closed {
		return ErrClosed
	}

	return l.applyLocked()
}

// applyLocked applies variables to the environment. Must be called with lock held.
func (l *Loader) applyLocked() error {
	keys := l.vars.Keys()
	for _, key := range keys {
		value, ok := l.vars.Get(key)
		if !ok {
			continue
		}

		// Check for existing value
		if existing := l.fs.Getenv(key); existing != "" && !l.config.OverwriteExisting {
			_ = l.auditor.Log(internal.ActionSet, key, "skipped (existing)", false)
			continue
		}

		if err := l.fs.Setenv(key, value); err != nil {
			_ = l.auditor.LogError(internal.ActionSet, key, err.Error())
			return fmt.Errorf("failed to set %s: %w", MaskKey(key), err)
		}

		_ = l.auditor.Log(internal.ActionSet, key, "applied", true)
	}

	l.applied = true
	return nil
}

// GetString retrieves a value by key with optional default.
// If the key is not found and no default is provided, returns empty string.
// Supports dot-notation path resolution for nested keys (e.g., "database.host" -> "DATABASE_HOST").
//
// Example:
//
//	value := loader.GetString("KEY")           // Returns "" if not found
//	value := loader.GetString("KEY", "default") // Returns "default" if not found
func (l *Loader) GetString(key string, defaultValue ...string) string {
	value, ok := l.Lookup(key)
	if !ok {
		if len(defaultValue) > 0 {
			return defaultValue[0]
		}
		return ""
	}
	return value
}

// GetSecure retrieves a SecureValue by key.
func (l *Loader) GetSecure(key string) *SecureValue {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if l.closed {
		return nil
	}

	return l.vars.GetSecure(key)
}

// Lookup retrieves a value by key and reports whether it exists.
// Supports dot-notation path resolution for nested keys (e.g., "database.host" -> "DATABASE_HOST").
// For indexed access (e.g., "service.cors.origins.0"), falls back to comma-separated values
// if indexed key is not found.
// Returns the value with leading and trailing whitespace trimmed.
func (l *Loader) Lookup(key string) (string, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if l.closed {
		return "", false
	}

	// Fast path for simple keys (no dots) - skip ResolvePath allocation
	// This is the most common case and avoids creating candidate slices
	if strings.IndexByte(key, '.') == -1 {
		// Try exact key first
		if value, ok := l.vars.Get(key); ok {
			return internal.TrimSpace(value), true
		}
		// Try uppercase version if different
		upper := internal.ToUpperASCII(key)
		if upper != key {
			if value, ok := l.vars.Get(upper); ok {
				return internal.TrimSpace(value), true
			}
		}
		return "", false
	}

	// Slow path for dot-notation keys - use path resolver
	candidates := internal.ResolvePath(key)

	// Try each candidate in priority order
	for _, candidate := range candidates {
		if value, ok := l.vars.Get(candidate); ok {
			return internal.TrimSpace(value), true
		}
	}

	// Fallback to comma-separated values for indexed access
	if basePath, index, hasIndex := internal.ExtractNumericIndex(key); hasIndex {
		baseCandidates := internal.ResolvePath(basePath)
		for _, baseCandidate := range baseCandidates {
			if value, ok := l.vars.Get(baseCandidate); ok {
				parts := splitAndTrimComma(value)
				if index >= 0 && index < len(parts) {
					return parts[index], true
				}
			}
		}
	}

	return "", false
}

// splitAndTrimComma splits by comma and trims whitespace.
func splitAndTrimComma(s string) []string {
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := internal.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// Set sets a value for a key.
func (l *Loader) Set(key, value string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.closed {
		return ErrClosed
	}

	// Validate key
	if err := l.validator.ValidateKey(key); err != nil {
		_ = l.auditor.LogError(internal.ActionSet, key, err.Error())
		return err
	}

	// Validate value
	if l.config.ValidateValues {
		if err := l.validator.ValidateValue(value); err != nil {
			_ = l.auditor.LogError(internal.ActionSet, key, err.Error())
			return err
		}
	}

	// Check overwrite policy
	if _, exists := l.vars.Get(key); exists && !l.config.OverwriteExisting {
		_ = l.auditor.Log(internal.ActionSet, key, "skipped (no overwrite)", false)
		return nil
	}

	l.vars.Set(key, value)
	_ = l.auditor.Log(internal.ActionSet, key, "set", true)

	// Apply to environment if auto-apply is enabled
	if l.config.AutoApply {
		if err := l.fs.Setenv(key, value); err != nil {
			return fmt.Errorf("failed to set environment: %w", err)
		}
	}

	return nil
}

// Delete removes a key.
func (l *Loader) Delete(key string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.closed {
		return ErrClosed
	}

	l.vars.Delete(key)
	_ = l.auditor.Log(internal.ActionDelete, key, "deleted", true)

	// Remove from environment if applied
	if l.applied {
		if err := l.fs.Unsetenv(key); err != nil {
			_ = l.auditor.LogError(internal.ActionDelete, key, err.Error())
		}
	}

	return nil
}

// Keys returns all keys.
func (l *Loader) Keys() []string {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if l.closed {
		return nil
	}

	return l.vars.Keys()
}

// All returns all environment variables as a map.
func (l *Loader) All() map[string]string {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if l.closed {
		return nil
	}

	return l.vars.ToMap()
}

// Len returns the number of loaded variables.
func (l *Loader) Len() int {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if l.closed {
		return 0
	}

	return l.vars.Len()
}

// IsApplied returns true if the variables have been applied to os.Environ.
func (l *Loader) IsApplied() bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.applied
}

// LoadTime returns the time when variables were last loaded.
func (l *Loader) LoadTime() time.Time {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.loadTime
}

// Close closes the loader and securely clears all stored values.
// If the loader owns its ComponentFactory, it will also close the factory.
func (l *Loader) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.closed {
		return nil
	}

	l.vars.Clear()

	// Only close the factory if we own it
	// This prevents double-closing when the factory is shared
	if l.ownsFactory && l.factory != nil {
		if err := l.factory.Close(); err != nil {
			return err
		}
	}

	l.closed = true
	return nil
}

// IsClosed returns true if the loader has been closed.
func (l *Loader) IsClosed() bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.closed
}

// getWithDefault is a generic helper for retrieving values with optional defaults.
// It handles the common pattern of looking up a key, parsing it, and returning
// a default value if the key is not found or parsing fails.
// Parse failures are logged to the auditor for debugging purposes.
func getWithDefault[T any](loader *Loader, key string, parse func(string) (T, error), defaultValue ...T) T {
	var zero T
	value, ok := loader.Lookup(key)
	if !ok {
		if len(defaultValue) > 0 {
			return defaultValue[0]
		}
		return zero
	}
	result, err := parse(value)
	if err != nil {
		// Log parse failure for debugging
		_ = loader.auditor.LogError(internal.ActionGet, key, fmt.Sprintf("parse failed: %v", err))
		if len(defaultValue) > 0 {
			return defaultValue[0]
		}
		return zero
	}
	return result
}

// Config returns the loader's configuration.
// Note: The returned Config should be treated as read-only.
// Modifying the Security.KeyPattern, AllowedKeys, ForbiddenKeys, or RequiredKeys
// fields may affect the loader's behavior. For a safe mutable copy,
// manually copy the necessary fields.
func (l *Loader) Config() Config {
	return l.config
}

// GetInt retrieves an integer value with optional default.
// If the key is not found and no default is provided, returns 0.
//
// Example:
//
//	port := loader.GetInt("PORT")           // Returns 0 if not found
//	port := loader.GetInt("PORT", 8080)     // Returns 8080 if not found
func (l *Loader) GetInt(key string, defaultValue ...int64) int64 {
	return getWithDefault(l, key, func(s string) (int64, error) {
		return parseInt(s, 64)
	}, defaultValue...)
}

// GetBool retrieves a boolean value with optional default.
// If the key is not found and no default is provided, returns false.
//
// Example:
//
//	debug := loader.GetBool("DEBUG")           // Returns false if not found
//	debug := loader.GetBool("DEBUG", true)     // Returns true if not found
func (l *Loader) GetBool(key string, defaultValue ...bool) bool {
	return getWithDefault(l, key, parseBool, defaultValue...)
}

// GetDuration retrieves a duration value with optional default.
// If the key is not found and no default is provided, returns 0.
//
// Example:
//
//	timeout := loader.GetDuration("TIMEOUT")                  // Returns 0 if not found
//	timeout := loader.GetDuration("TIMEOUT", 30*time.Second) // Returns 30s if not found
func (l *Loader) GetDuration(key string, defaultValue ...time.Duration) time.Duration {
	return getWithDefault(l, key, parseDuration, defaultValue...)
}

// GetSliceFrom retrieves a slice of values from a loader by iterating through indexed keys.
// If the key is not found and no default is provided, returns nil.
// Supports dot-notation path resolution for nested keys.
//
// Indexed keys are searched in format: KEY_0, KEY_1, KEY_2, etc.
// Also supports comma-separated values as fallback for .env files.
//
// Note: This is a generic function (not a method) because Go does not support
// type parameters on methods. Use this when you have a Loader instance.
//
// Example:
//
//	ports := env.GetSliceFrom[int](loader, "PORTS")           // Returns []int{8080, 8081} from PORTS_0, PORTS_1
//	hosts := env.GetSliceFrom[string](loader, "HOSTS", []string{"localhost"}) // With default
func GetSliceFrom[T sliceElement](loader *Loader, key string, defaultValue ...[]T) []T {
	if loader == nil {
		if len(defaultValue) > 0 {
			return defaultValue[0]
		}
		return nil
	}

	loader.mu.RLock()
	defer loader.mu.RUnlock()

	if loader.closed {
		if len(defaultValue) > 0 {
			return defaultValue[0]
		}
		return nil
	}

	// GetString candidate keys from path resolver (handles dot-notation)
	candidates := internal.ResolvePath(key)

	// Try each candidate in priority order
	for _, baseKey := range candidates {
		result := getSliceFromIndexedKeys[T](loader, baseKey, defaultValue)
		if len(result) > 0 {
			return result
		}
	}

	// No indexed keys found, return default or nil
	if len(defaultValue) > 0 {
		return defaultValue[0]
	}
	return nil
}

// getSliceFromIndexedKeys tries to get a slice from indexed keys for a specific base key.
func getSliceFromIndexedKeys[T sliceElement](loader *Loader, baseKey string, defaultValue [][]T) []T {
	// Collect values from indexed keys: KEY_0, KEY_1, KEY_2, ...
	var result []T
	for i := 0; ; i++ {
		indexedKey := fmt.Sprintf("%s_%d", baseKey, i)
		value, ok := loader.vars.Get(indexedKey)
		if !ok {
			break
		}

		parsed, err := parseSliceElement[T](value)
		if err != nil {
			if len(defaultValue) > 0 {
				return defaultValue[0]
			}
			return nil
		}
		result = append(result, parsed)
	}

	// If no indexed keys found, try comma-separated value
	if len(result) == 0 {
		if value, ok := loader.vars.Get(baseKey); ok {
			return parseCommaSeparated[T](value, defaultValue...)
		}
	}

	return result
}

// ParseInto populates a struct from loaded environment variables.
// Struct fields can be tagged with `env:"KEY"` to specify the env variable name.
// Optional `envDefault:"value"` sets a default if the key is not found.
func (l *Loader) ParseInto(v interface{}) error {
	return UnmarshalInto(l.All(), v)
}

// Validate validates the loaded environment against required and allowed keys.
func (l *Loader) Validate() error {
	return l.validator.ValidateRequired(l.keysToUpper())
}

// keysToUpper returns all keys as uppercase for comparison.
func (l *Loader) keysToUpper() map[string]bool {
	keys := l.Keys()
	result := make(map[string]bool, len(keys))
	for _, k := range keys {
		result[internal.ToUpperASCII(k)] = true
	}
	return result
}

// validateFilePath validates a file path for security.
// It checks for path traversal attempts and other potentially dangerous patterns.
func validateFilePath(filename string) error {
	if filename == "" {
		return &SecurityError{
			Action: "file_access",
			Reason: "empty filename",
		}
	}

	// SECURITY: Check for null bytes first (could be used to bypass extension checks)
	if strings.ContainsRune(filename, '\x00') {
		return &SecurityError{
			Action: "file_access",
			Reason: "null byte in path",
		}
	}

	// SECURITY: Check for UNC paths (Windows network paths)
	// These can be used to access files on network shares
	// Also block \\?\ prefix which can bypass path length limits
	if len(filename) >= 2 && filename[0] == '\\' && filename[1] == '\\' {
		return &SecurityError{
			Action: "file_access",
			Reason: "UNC path not allowed",
		}
	}

	// SECURITY: Check for forward-slash UNC paths (\\ translated to //)
	if len(filename) >= 2 && filename[0] == '/' && filename[1] == '/' {
		return &SecurityError{
			Action: "file_access",
			Reason: "network path not allowed",
		}
	}

	// SECURITY: Check for Unix-style absolute paths starting with /
	// Only allow relative paths for safety
	if len(filename) > 0 && filename[0] == '/' {
		return &SecurityError{
			Action: "file_access",
			Reason: "absolute path not allowed",
		}
	}

	// SECURITY: Check for Windows drive letters (C:, D:, etc.)
	// This prevents access to system files via absolute paths
	if len(filename) >= 2 && filename[1] == ':' {
		c := filename[0]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
			return &SecurityError{
				Action: "file_access",
				Reason: "absolute path with drive letter not allowed",
			}
		}
	}

	// Check for path traversal attempts using filepath.Clean
	// This is more precise than just checking for ".." in the raw string
	//    filepath.Clean normalizes the path by resolving ".." and removing redundant separators
	cleanPath := filepath.Clean(filename)
	if strings.Contains(cleanPath, "..") {
		return &SecurityError{
			Action: "file_access",
			Reason: "path traversal detected",
			Key:    MaskKey(filename),
		}
	}

	// On Windows, check for reserved device names
	// These names are reserved in any directory and cannot be used as filenames
	// See: https://learn.microsoft.com/en-us/windows/win32/fileio/naming-a-file
	if len(filename) >= 3 {
		upper := strings.ToUpper(filename)
		// Check for CON, PRN, AUX, NUL, COM1-9, LPT1-9
		reserved := []string{"CON", "PRN", "AUX", "NUL"}
		for _, r := range reserved {
			if upper == r || (len(upper) > 3 && upper[:3] == r && (upper[3] == '.' || upper[3] == ':')) {
				return &SecurityError{
					Action: "file_access",
					Reason: "reserved device name",
				}
			}
		}
		// Check COM and LPT ports (COM1-COM9, LPT1-LPT9)
		if len(upper) >= 4 {
			prefix := upper[:3]
			if (prefix == "COM" || prefix == "LPT") && upper[3] >= '1' && upper[3] <= '9' {
				if len(upper) == 4 || upper[4] == '.' || upper[4] == ':' {
					return &SecurityError{
						Action: "file_access",
						Reason: "reserved device name",
					}
				}
			}
		}
		// Check for pseudo-device names: CONIN$, CONOUT$, CLOCK$
		pseudoDevices := []string{"CONIN$", "CONOUT$", "CLOCK$"}
		for _, pd := range pseudoDevices {
			if upper == pd || strings.HasPrefix(upper, pd+".") || strings.HasPrefix(upper, pd+":") {
				return &SecurityError{
					Action: "file_access",
					Reason: "reserved pseudo-device name",
				}
			}
		}
	}

	return nil
}
