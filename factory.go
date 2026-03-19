package env

import (
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cybergodev/env/internal"
)

// auditorAdapter wraps internal.Auditor to implement AuditLogger interface.
type auditorAdapter struct {
	auditor *internal.Auditor
}

// newAuditorAdapter creates a new AuditLogger from an internal.Auditor.
func newAuditorAdapter(a *internal.Auditor) AuditLogger {
	if a == nil {
		return nil
	}
	return &auditorAdapter{auditor: a}
}

// Log implements AuditLogger.
func (a *auditorAdapter) Log(action AuditAction, key, reason string, success bool) error {
	return a.auditor.Log(internal.Action(action), key, reason, success)
}

// LogError implements AuditLogger.
func (a *auditorAdapter) LogError(action AuditAction, key, errMsg string) error {
	return a.auditor.LogError(internal.Action(action), key, errMsg)
}

// LogWithFile implements AuditLogger.
func (a *auditorAdapter) LogWithFile(action AuditAction, key, file, reason string, success bool) error {
	return a.auditor.LogWithFile(internal.Action(action), key, file, reason, success)
}

// LogWithDuration implements AuditLogger.
func (a *auditorAdapter) LogWithDuration(action AuditAction, key, reason string, success bool, duration time.Duration) error {
	return a.auditor.LogWithDuration(internal.Action(action), key, reason, success, duration)
}

// Close implements AuditLogger.
func (a *auditorAdapter) Close() error {
	// Check both a and a.auditor for nil safety
	// newAuditorAdapter guarantees non-nil auditor, but be defensive
	if a == nil || a.auditor == nil {
		return nil
	}
	return a.auditor.Close()
}

// ComponentFactory creates and manages shared components used by Loader and Parser.
// It provides a clean lifecycle for validator, auditor, and expander instances.
// ComponentFactory is safe for concurrent use.
type ComponentFactory struct {
	validator *internal.Validator
	auditor   *internal.Auditor
	expander  *internal.Expander
	closed    atomic.Bool
	mu        sync.Mutex // Protects auditor.Close()
}

// Compile-time check that ComponentFactory implements io.Closer.
var _ io.Closer = (*ComponentFactory)(nil)

// Validator returns the validator component.
func (f *ComponentFactory) Validator() Validator {
	return f.validator
}

// Auditor returns the audit logger component.
func (f *ComponentFactory) Auditor() AuditLogger {
	return newAuditorAdapter(f.auditor)
}

// Close releases resources held by the factory.
// After calling Close, the factory should not be used.
// Safe to call multiple times; subsequent calls return nil.
// This method is safe for concurrent use.
func (f *ComponentFactory) Close() error {
	// Use CompareAndSwap for atomic transition from open to closed state.
	// This eliminates the need for double-check locking pattern.
	if !f.closed.CompareAndSwap(false, true) {
		return nil // Already closed
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if f.auditor != nil {
		return f.auditor.Close()
	}
	return nil
}

// IsClosed returns true if the factory has been closed.
// This method is safe for concurrent use.
func (f *ComponentFactory) IsClosed() bool {
	return f.closed.Load()
}

// internalValidator returns the concrete validator for internal use.
// This method is used internally by parsers that need access to the concrete type.
func (f *ComponentFactory) internalValidator() *internal.Validator {
	return f.validator
}

// internalAuditor returns the concrete auditor for internal use.
// This method is used internally by parsers that need access to the concrete type.
func (f *ComponentFactory) internalAuditor() *internal.Auditor {
	return f.auditor
}

// internalExpander returns the concrete expander for internal use.
// This method is used internally by parsers that need access to the concrete type.
func (f *ComponentFactory) internalExpander() *internal.Expander {
	return f.expander
}

// buildComponentFactory creates a new ComponentFactory from the configuration.
// It uses the default OS file system for environment variable lookup.
func (c *Config) buildComponentFactory() *ComponentFactory {
	return c.buildComponentFactoryWithFS(DefaultFileSystem)
}

// buildComponentFactoryWithFS creates a new ComponentFactory from the configuration
// using the provided FileSystem for environment variable lookup.
// If fs is nil, DefaultFileSystem is used.
func (c *Config) buildComponentFactoryWithFS(fs FileSystem) *ComponentFactory {
	// Use default file system if not provided
	if fs == nil {
		fs = DefaultFileSystem
	}

	handler := c.AuditHandler
	if handler == nil {
		handler = internal.DefaultHandler()
	}

	lookup := func(key string) (string, bool) {
		return fs.LookupEnv(key)
	}

	// Start with default forbidden keys
	forbiddenKeys := make([]string, 0, len(DefaultForbiddenKeys))
	for k := range DefaultForbiddenKeys {
		forbiddenKeys = append(forbiddenKeys, k)
	}
	// Add custom forbidden keys
	forbiddenKeys = append(forbiddenKeys, c.ForbiddenKeys...)

	return &ComponentFactory{
		validator: internal.NewValidator(internal.ValidatorConfig{
			KeyPattern:     c.KeyPattern,
			AllowedKeys:    c.AllowedKeys,
			ForbiddenKeys:  forbiddenKeys,
			RequiredKeys:   c.RequiredKeys,
			MaxKeyLength:   c.MaxKeyLength,
			MaxValueLength: c.MaxValueLength,
			IsSensitive:    IsSensitiveKey,
			MaskKey:        MaskKey,
			MaskSensitive:  MaskSensitiveInString,
		}),
		auditor: internal.NewAuditor(handler, IsSensitiveKey, MaskValue, c.AuditEnabled),
		expander: internal.NewExpander(internal.ExpanderConfig{
			MaxDepth:   c.MaxExpansionDepth,
			Lookup:     lookup,
			Mode:       internal.ModeAll,
			KeyPattern: c.KeyPattern,
		}),
	}
}
