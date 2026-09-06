// Package env provides a high-security environment variable library for Go 1.25+.
// It is designed for applications where security, concurrent access, and production-grade
// features are critical.
//
// The library supports multiple file formats (.env, JSON, YAML), secure memory handling
// for sensitive values, comprehensive audit logging, and extensive validation.
//
// # Two Usage Modes
//
// The library provides two complementary usage patterns:
//
// ## Global Mode (Simple)
//
// Use package-level functions after calling Load(). Best for simple applications:
//
//	err := env.Load(".env")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	port := env.GetInt("PORT", 8080)
//	host := env.GetString("DATABASE_HOST", "localhost")
//
// ## Instance Mode (Advanced)
//
// Create isolated Loader instances with New(). Best for tests and fine-grained control:
//
//	cfg := env.DefaultConfig()
//	cfg.Filenames = []string{".env"}
//	loader, err := env.New(cfg)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer loader.Close()
//	value := loader.GetString("DATABASE_URL")
//
// # When to Use Which Mode
//
// Use Global Mode when:
//   - Simple application with single configuration
//   - Want automatic apply to os.Environ (Load does this by default)
//   - Quick scripts, prototypes, or small services
//
// Use Instance Mode when:
//   - Writing tests that need isolation
//   - Multiple configurations in same process
//   - Need control over when variables are applied to os.Environ
//   - Want explicit lifecycle management with Close()
//
// # Secure Value Handling
//
// For sensitive values like API keys and passwords, use SecureValue:
//
//	sv := loader.GetSecure("API_KEY")
//	if sv != nil {
//	    defer sv.Release()
//	    data := sv.Bytes()
//	    // use data securely
//	    env.ClearBytes(data)
//	}
//
// # Struct Mapping
//
// Map environment variables to structs using tags:
//
//	type Config struct {
//	    Host string `env:"DB_HOST" envDefault:"localhost"`
//	    Port int    `env:"DB_PORT" envDefault:"5432"`
//	}
//
//	var cfg Config
//	if err := env.ParseInto(&cfg); err != nil {
//	    log.Fatal(err)
//	}
//
// # Environment Presets
//
// The library provides preset configurations for different environments:
//   - DefaultConfig: Secure defaults for general use
//   - DevelopmentConfig: Relaxed limits, overwrite enabled
//   - TestingConfig: Isolated, compact limits
//   - ProductionConfig: Strict limits, audit enabled
//
// # File Format Support
//
// Supported file formats:
//   - .env: Standard dotenv format with KEY=value pairs
//   - .json: JSON files with nested structure (flattened with underscores)
//   - .yaml/.yml: YAML files with nested structure (flattened with underscores)
//
// # Thread Safety
//
// All Loader methods are safe for concurrent use. The library uses sharded
// locking for optimal performance in high-concurrency scenarios.
//
// # Error Types
//
// The library defines several error types for precise error handling:
//   - ErrFileNotFound: File does not exist
//   - ErrFileTooLarge: File exceeds size limit
//   - ErrInvalidKey: Key format validation failed
//   - ErrSecurityViolation: Security policy violation
//   - ErrClosed: Loader has been closed
//   - ParseError: Parsing failure with file/line info
//   - ValidationError: Configuration or value validation failure
//   - SecurityError: Security-related error
//   - FileError: File operation error
//   - ExpansionError: Variable expansion error
//   - JSONError: JSON parsing error
//   - YAMLError: YAML parsing error
//   - MarshalError: Marshaling/unmarshaling error
package env

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cybergodev/env/internal"
)

// Loader is the main type for loading and managing environment variables.
// It provides thread-safe access to environment variables with full
// security validation, audit logging, and error handling.
type Loader struct {
	config      Config
	factory     *ComponentFactory
	ownsFactory bool
	parsers     map[FileFormat]EnvParser
	fs          FileSystem

	mu   sync.RWMutex
	vars *secureMap
	// closedFast mirrors closed as an atomic so single-key reads (Lookup,
	// GetSecure) can check loader state without taking l.mu — under high
	// concurrency the RWMutex reader count itself becomes the bottleneck.
	// It is written exactly once, under l.mu, in Close.
	closedFast  atomic.Bool
	applied     bool
	appliedKeys map[string]struct{} // process-env keys this loader actually set
	closed      bool
	loadTime    time.Time
}

// Compile-time check that Loader implements EnvLoader.
var _ EnvLoader = (*Loader)(nil)

// Compile-time check that Loader implements io.Closer.
var _ io.Closer = (*Loader)(nil)

// enterRead acquires the read lock and validates loader state.
// Returns ErrClosed if the loader is nil or closed.
// Caller must call exitRead() when done.
func (l *Loader) enterRead() error {
	if l == nil {
		return ErrClosed
	}
	l.mu.RLock()
	if l.closed {
		l.mu.RUnlock()
		return ErrClosed
	}
	return nil
}

// exitRead releases the read lock.
func (l *Loader) exitRead() { l.mu.RUnlock() }

// enterWrite acquires the write lock and validates loader state.
// Returns ErrClosed if the loader is nil or closed.
// Caller must call exitWrite() when done.
func (l *Loader) enterWrite() error {
	if l == nil {
		return ErrClosed
	}
	l.mu.Lock()
	if l.closed {
		l.mu.Unlock()
		return ErrClosed
	}
	return nil
}

// exitWrite releases the write lock.
func (l *Loader) exitWrite() { l.mu.Unlock() }

// cleanupOnError closes the loader if cleanup is true, logging any close error.
// Returns the original error regardless of cleanup outcome.
func (l *Loader) cleanupOnError(err error, cleanup bool) error {
	if !cleanup {
		return err
	}
	if closeErr := l.Close(); closeErr != nil {
		_ = l.factory.Auditor().LogError(internal.ActionError, "", "cleanup failed: "+closeErr.Error())
	}
	return err
}

// New creates a new Loader with the given configuration.
//
// BEHAVIOR:
//   - Does NOT set the package-level default loader
//   - Does NOT auto-apply to os.Environ (unless cfg.AutoApply = true)
//   - Can be called multiple times to create independent instances
//   - Requires explicit lifecycle management: defer loader.Close()
//
// If no configuration is provided or a zero-value Config is passed,
// DefaultConfig() is used automatically.
//
// FOR SIMPLE USE CASES: Use Load() instead, which sets up
// the package-level default and applies to os.Environ automatically.
//
// WHEN TO USE New():
//   - Multiple loaders in one application (different configs/files)
//   - Testing with isolated environment state
//   - When you need explicit control over when variables are applied
//
// Example:
//
//	// Use default configuration
//	loader, err := env.New()
//
//	// Use custom configuration
//	cfg := env.DefaultConfig()
//	cfg.Filenames = []string{".env.production"}
//	cfg.AutoApply = true
//	loader, err := env.New(cfg)
func New(cfg ...Config) (*Loader, error) {
	var c Config
	if len(cfg) > 0 {
		c = cfg[0]
		// If zero-value Config is passed, use defaults
		if c.IsZero() {
			c = DefaultConfig()
		}
	} else {
		c = DefaultConfig()
	}

	if err := c.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidConfig, err)
	}

	// Use default file system if not specified
	fs := c.FileSystem
	if fs == nil {
		fs = DefaultFileSystem
	}

	// Build shared components once
	factory := c.buildComponentFactoryWithFS(fs)

	// Create parsers using registry
	parsers, err := createParsers(c, factory)
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
		config:      c,
		factory:     factory,
		ownsFactory: true,
		parsers:     parsers,
		fs:          fs,
		vars:        newSecureMap(),
	}

	// Auto-load files if Filenames is configured
	if len(c.Filenames) > 0 {
		if err := loader.loadFilesInternal(c.Filenames, true); err != nil {
			return nil, err
		}
	}

	return loader, nil
}

// LoadFiles loads environment variables from multiple files in order.
// If no filenames are provided, defaults to ".env".
// Files are loaded sequentially. A later file only overrides values from an
// earlier one when OverwriteExisting is true; with the default (false) the
// first file to set a key wins.
//
// Returns:
//   - ErrClosed: if the loader is nil or has been closed
//   - ErrFileNotFound: if a file does not exist and FailOnMissingFile is true
//   - ParseError: if a file contains invalid syntax
//   - SecurityError: if a file path fails security validation
//   - ErrFileTooLarge: if a file exceeds MaxFileSize
//
// Example:
//
//	// Load default .env file
//	err := loader.LoadFiles()
//
//	// Load specific files
//	err := loader.LoadFiles(".env", ".env.local")
func (l *Loader) LoadFiles(filenames ...string) error {
	if err := l.enterWrite(); err != nil {
		return err
	}
	defer l.exitWrite()

	// Default to .env if no files specified
	if len(filenames) == 0 {
		filenames = []string{".env"}
	}

	return l.loadFilesInternal(filenames, false)
}

// loadFilesInternal is the shared implementation for file loading.
// Thread safety: caller MUST hold l.mu (write lock). New() callers are exempt
// because the loader is not yet shared, but LoadFiles() acquires the lock first.
// This method modifies l.loadTime and l.applied, which require write-lock protection.
//
// The cleanupOnErr parameter determines whether to close the loader on error (used during initialization).
func (l *Loader) loadFilesInternal(filenames []string, cleanupOnErr bool) error {
	var start time.Time
	auditEnabled := l.config.AuditEnabled
	if auditEnabled {
		start = time.Now()
	}

	for _, filename := range filenames {
		if err := l.loadFileLocked(filename); err != nil {
			if errors.Is(err, ErrFileNotFound) && !l.config.FailOnMissingFile {
				_ = l.factory.Auditor().LogWithFile(internal.ActionLoad, "", filename, "file not found, skipping", true)
				continue
			}
			return l.cleanupOnError(err, cleanupOnErr)
		}
	}

	l.loadTime = time.Now()
	if auditEnabled {
		_ = l.factory.Auditor().LogWithDuration(internal.ActionLoad, "", "loaded files", true, time.Since(start))
	}

	if l.config.AutoApply {
		if err := l.applyLocked(); err != nil {
			return l.cleanupOnError(err, cleanupOnErr)
		}
	}

	return nil
}

// loadFileLocked loads a single file.
// Thread safety: caller MUST hold l.mu (write lock). While individual secureMap
// operations are internally synchronized, this method modifies loader state
// (via loadFilesInternal → l.loadTime, l.applied) that requires write-lock protection.
//
// SECURITY - Defense-in-Depth for TOCTOU:
// There is a theoretical Time-Of-Check-Time-Of-Use window between Open() and Stat()
// where the file could be replaced or modified. This is mitigated by:
//  1. SecureReader: The parser wraps the file with SecureReader which enforces
//     hard limits during reading, providing secondary enforcement of size constraints.
//  2. Hard Limits: Even if the file grows between Stat() and Read(), SecureReader
//     will stop reading at hardMaxFileSize (100MB), preventing memory exhaustion.
//  3. Validation: All parsed content is validated for size, format, and safety
//     regardless of the initial Stat() results.
func (l *Loader) loadFileLocked(filename string) error {
	// Only record start time when audit is enabled — avoids time.Now() syscall
	// and string concatenation in the common (audit-disabled) case.
	var start time.Time
	auditEnabled := l.config.AuditEnabled
	if auditEnabled {
		start = time.Now()
	}

	// SECURITY: Validate file path to prevent path traversal attacks
	if err := validateFilePath(filename); err != nil {
		_ = l.factory.Auditor().LogError(internal.ActionSecurity, "", "path validation failed: "+err.Error())
		return err
	}

	file, err := l.fs.Open(filename)
	if err != nil {
		if os.IsNotExist(err) || errors.Is(err, ErrFileNotFound) {
			return newFileError(filename, "open", ErrFileNotFound)
		}
		return newFileError(filename, "open", err)
	}
	defer file.Close() // read-only handle; close error is not actionable

	// Get file info for size check
	// Note: This provides a fast-path check for obviously oversized files.
	// SecureReader provides defense-in-depth for files that grow after this check.
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

	if parser == nil {
		return newFileError(filename, "parse", fmt.Errorf("no parser registered for format %v", format))
	}

	vars, err := parser.Parse(file, filename)
	if err != nil {
		return err
	}

	// Fast path: no prefix filter → apply the whole file directly.
	// SetAll overwrites; SetAllIfAbsent skips existing keys without building
	// an intermediate filtered map or per-key Get() allocations.
	if l.config.Prefix == "" {
		if l.config.AuditEnabled {
			for key := range vars {
				if !l.config.OverwriteExisting && l.vars.Has(key) {
					_ = l.factory.Auditor().Log(internal.ActionSet, key, "skipped (no overwrite)", false)
				} else {
					_ = l.factory.Auditor().Log(internal.ActionSet, key, "loaded", true)
				}
			}
		}
		if l.config.OverwriteExisting {
			l.vars.SetAll(vars)
		} else {
			l.vars.SetAllIfAbsent(vars)
		}
		if auditEnabled {
			_ = l.factory.Auditor().LogWithDuration(internal.ActionFileAccess, "", "file loaded: "+filename, true, time.Since(start))
		}
		return nil
	}

	// Slow path: prefix filtering needed.
	// Pre-compute uppercase prefix once outside the loop.
	toSet := make(map[string]string, len(vars))
	upperPrefix := internal.ToUpperASCII(l.config.Prefix)

	for key, value := range vars {
		// Check prefix if configured (case-insensitive, zero-allocation)
		if !internal.HasUpperPrefix(key, upperPrefix) {
			continue
		}

		// Check overwrite policy using Has (no string allocation)
		if !l.config.OverwriteExisting && l.vars.Has(key) {
			if l.config.AuditEnabled {
				_ = l.factory.Auditor().Log(internal.ActionSet, key, "skipped (no overwrite)", false)
			}
			continue
		}

		toSet[key] = value
		if l.config.AuditEnabled {
			_ = l.factory.Auditor().Log(internal.ActionSet, key, "loaded", true)
		}
	}

	// Use batch set for better performance
	l.vars.SetAll(toSet)

	if auditEnabled {
		_ = l.factory.Auditor().LogWithDuration(internal.ActionFileAccess, "", "file loaded: "+filename, true, time.Since(start))
	}
	return nil
}

// Apply applies all loaded variables to the process environment.
// Only sets variables that do not already exist unless OverwriteExisting is true.
//
// Returns:
//   - ErrClosed: if the loader is nil or has been closed
//   - Wrapped os errors: if setting an environment variable fails
func (l *Loader) Apply() error {
	if err := l.enterWrite(); err != nil {
		return err
	}
	defer l.exitWrite()

	return l.applyLocked()
}

// applyLocked applies variables to the environment.
// Thread safety: caller MUST hold l.mu (write lock). This method sets l.applied
// and l.appliedKeys, which require write-lock protection.
func (l *Loader) applyLocked() error {
	for key, value := range l.vars.ToMap() {
		// Check for existing value using LookupEnv to distinguish between
		// "not set" and "empty string". This correctly handles the case where
		// an environment variable is explicitly set to empty string.
		if _, exists := l.fs.LookupEnv(key); exists && !l.config.OverwriteExisting {
			_ = l.factory.Auditor().Log(internal.ActionSet, key, "skipped (existing)", false)
			continue
		}

		if err := l.fs.Setenv(key, value); err != nil {
			_ = l.factory.Auditor().LogError(internal.ActionSet, key, err.Error())
			return fmt.Errorf("failed to set %s: %w", MaskKey(key), err)
		}
		l.markAppliedLocked(key)

		_ = l.factory.Auditor().Log(internal.ActionSet, key, "applied", true)
	}

	l.applied = true
	return nil
}

// markAppliedLocked records that this loader set key in the process
// environment. Thread safety: caller MUST hold l.mu (write lock).
func (l *Loader) markAppliedLocked(key string) {
	if l.appliedKeys == nil {
		l.appliedKeys = make(map[string]struct{})
	}
	l.appliedKeys[key] = struct{}{}
}

// wasAppliedLocked reports whether this loader set key in the process
// environment. Thread safety: caller MUST hold l.mu (either lock).
func (l *Loader) wasAppliedLocked(key string) bool {
	_, ok := l.appliedKeys[key]
	return ok
}

// validateSetKV validates a Set key/value pair and reports failures to the
// auditor. Shared by both lock paths of Set.
func (l *Loader) validateSetKV(key, value string) error {
	// Validate key
	if err := l.factory.Validator().ValidateKey(key); err != nil {
		_ = l.factory.Auditor().LogError(internal.ActionSet, key, err.Error())
		return err
	}

	// Validate value
	if l.config.ValidateValues {
		if err := l.factory.Validator().ValidateValue(value); err != nil {
			_ = l.factory.Auditor().LogError(internal.ActionSet, key, err.Error())
			return err
		}
	}
	return nil
}

// Set sets a value for a key.
// If OverwriteExisting is false and the key already exists, the call is silently skipped.
//
// Returns:
//   - ErrClosed: if the loader is nil or has been closed
//   - ErrInvalidKey: if the key fails validation
//   - ErrForbiddenKey: if the key is rejected by the forbidden-keys or allowed-keys policy
//   - ErrInvalidValue: if ValidateValues is true and the value fails validation
func (l *Loader) Set(key, value string) error {
	// Fast path: with OverwriteExisting there is no Has→Set check-then-act to
	// serialize, and without AutoApply, Set touches no loader-level state
	// (applied/appliedKeys/loadTime) — only l.vars, which synchronizes
	// internally. A read lock therefore suffices, keeping concurrent Set
	// calls from serializing on the loader mutex; Close() still takes the
	// write lock, so it fully excludes this path.
	// l.config is immutable after New, so reading it here needs no lock.
	if l != nil && l.config.OverwriteExisting && !l.config.AutoApply {
		if err := l.enterRead(); err != nil {
			return err
		}
		defer l.exitRead()

		if err := l.validateSetKV(key, value); err != nil {
			return err
		}
		l.vars.Set(key, value)
		_ = l.factory.Auditor().Log(internal.ActionSet, key, "set", true)
		return nil
	}

	if err := l.enterWrite(); err != nil {
		return err
	}
	defer l.exitWrite()

	if err := l.validateSetKV(key, value); err != nil {
		return err
	}

	// Check overwrite policy
	// Use Has (no allocation) instead of Get (allocates string copy).
	if !l.config.OverwriteExisting {
		if l.vars.Has(key) {
			_ = l.factory.Auditor().Log(internal.ActionSet, key, "skipped (no overwrite)", false)
			return nil
		}
	}

	l.vars.Set(key, value)
	_ = l.factory.Auditor().Log(internal.ActionSet, key, "set", true)

	// Apply to environment if auto-apply is enabled
	if l.config.AutoApply {
		if err := l.fs.Setenv(key, value); err != nil {
			return fmt.Errorf("failed to set environment: %w", err)
		}
		// Record the key so Delete() knows to unset it from the process
		// environment, and mark the loader as applied for IsApplied().
		l.markAppliedLocked(key)
		l.applied = true
	}

	return nil
}

// Delete removes a key from the loaded environment.
// If the key was applied to the process environment by this loader
// (via Apply, AutoApply, or Set with AutoApply), it is also unset there.
// Keys the loader never applied are not touched in the process environment.
//
// Returns:
//   - ErrClosed: if the loader is nil or has been closed
func (l *Loader) Delete(key string) error {
	if err := l.enterWrite(); err != nil {
		return err
	}
	defer l.exitWrite()

	l.vars.Delete(key)
	_ = l.factory.Auditor().Log(internal.ActionDelete, key, "deleted", true)

	// Remove from the process environment only if this loader put it there.
	// Per-key tracking matters: unsetting on l.applied alone would remove
	// foreign process variables (HOME, TERM, ...) the loader never owned.
	if l.wasAppliedLocked(key) {
		if err := l.fs.Unsetenv(key); err != nil {
			_ = l.factory.Auditor().LogError(internal.ActionDelete, key, err.Error())
		} else {
			delete(l.appliedKeys, key)
		}
	}

	return nil
}

// Close closes the loader and securely clears all stored values.
// If the loader owns its ComponentFactory, it will also close the factory.
//
// Note: variables this loader previously applied to the process environment
// (via Apply, AutoApply, or Set with AutoApply) are NOT unset by Close —
// only the in-memory copies are cleared. Use Delete for per-key removal
// from the process environment before closing.
func (l *Loader) Close() error {
	if err := l.enterWrite(); err != nil {
		return nil // Close on nil/closed is not an error
	}
	defer l.exitWrite()

	l.vars.Clear()

	// Mark the loader closed before closing the owned factory. This guarantees
	// the loader is never left half-open (vars cleared but closed=false) if
	// factory.Close fails; subsequent calls observe a closed loader either way.
	// closedFast publishes the same state to lock-free single-key readers.
	l.closed = true
	l.closedFast.Store(true)

	// Only close the factory if we own it.
	// This prevents double-closing when the factory is shared.
	if l.ownsFactory && l.factory != nil {
		if err := l.factory.Close(); err != nil {
			return err
		}
	}

	return nil
}

// IsClosed returns true if the loader has been closed.
func (l *Loader) IsClosed() bool {
	if l == nil {
		return true
	}
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.closed
}

// ParseInto populates a struct from loaded environment variables.
// Struct fields can be tagged with `env:"KEY"` to specify the env variable name.
// Optional `envDefault:"value"` sets a default if the key is not found.
//
// Returns:
//   - ErrClosed: if the loader is nil or has been closed
//   - MarshalError: if struct tag parsing or type conversion fails
func (l *Loader) ParseInto(v any) error {
	// Hold the read lock across the snapshot to prevent a TOCTOU race where
	// another goroutine closes the loader between the state check and All().
	if err := l.enterRead(); err != nil {
		return err
	}
	defer l.exitRead()
	return UnmarshalInto(l.vars.ToMap(), v)
}

// Validate validates the loaded environment against required keys.
// Checks that all RequiredKeys are present. ForbiddenKeys are enforced at
// Set/parse time (see Set), not by this method.
//
// Returns:
//   - ErrClosed: if the loader is nil or has been closed
//   - ErrMissingRequired: if a required key is not present
func (l *Loader) Validate() error {
	// Hold the read lock across the snapshot to prevent a TOCTOU race where
	// another goroutine closes the loader between the state check and Keys().
	if err := l.enterRead(); err != nil {
		return err
	}
	defer l.exitRead()
	return l.factory.Validator().ValidateRequired(l.keysToUpper())
}

// keysToUpper returns all keys as uppercase for comparison.
// Caller must already hold the read lock (l.mu) — accesses l.vars directly
// to avoid the nested RLock that would occur through l.Keys().
func (l *Loader) keysToUpper() map[string]bool {
	keys := l.vars.Keys()
	result := make(map[string]bool, len(keys))
	for _, k := range keys {
		result[internal.ToUpperASCII(k)] = true
	}
	return result
}

// validateFilePath validates a file path for security.
// It checks for path traversal attempts and other potentially dangerous patterns.
// Note: This function delegates to internal.PathValidator for the actual validation.
func validateFilePath(filename string) error {
	return internal.ValidateFilePath(filename)
}
