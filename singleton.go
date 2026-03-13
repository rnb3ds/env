package env

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
)

// ErrAlreadyInitialized is returned when attempting to set a default loader
// when one has already been initialized.
var ErrAlreadyInitialized = errors.New("default loader already initialized")

// ============================================================================
// Default Loader Singleton
// ============================================================================

var (
	defaultLoader atomic.Pointer[Loader]
	defaultMu     sync.Mutex
)

// getDefaultLoader returns the default loader, creating it if necessary.
// This function is thread-safe using atomic.Pointer and mutex for initialization.
// Unlike traditional singletons, it allows retry after initialization errors.
func getDefaultLoader() (*Loader, error) {
	// Fast path: check if already initialized
	if loader := defaultLoader.Load(); loader != nil {
		return loader, nil
	}

	// Slow path: acquire lock for initialization
	defaultMu.Lock()
	defer defaultMu.Unlock()

	// Double-check after acquiring lock
	if loader := defaultLoader.Load(); loader != nil {
		return loader, nil
	}

	loader, err := New(DefaultConfig())
	if err != nil {
		// Don't cache errors - allow retry on next call
		return nil, err
	}

	defaultLoader.Store(loader)
	return loader, nil
}

// ResetDefaultLoader resets the default loader singleton.
// This function is intended for use in tests to ensure isolation between test cases.
// It is safe for concurrent use.
//
// Example:
//
//	func TestSomething(t *testing.T) {
//	    env.ResetDefaultLoader()
//	    defer env.ResetDefaultLoader()
//	    // ... test code
//	}
func ResetDefaultLoader() {
	defaultMu.Lock()
	defer defaultMu.Unlock()

	// Use atomic Swap to ensure no window where nil is stored but Close not called.
	// This eliminates the race condition between Store(nil) and Close().
	oldLoader := defaultLoader.Swap(nil)
	if oldLoader == nil {
		return
	}

	// Close the old loader after atomically clearing the reference.
	// Any concurrent getDefaultLoader() call will either see the old loader
	// (before Swap) or nil (after Swap), but never a closed loader.
	if err := oldLoader.Close(); err != nil {
		// Log to stderr as fallback - we don't have access to auditor here
		// Production safety: don't panic on cleanup errors
		fmt.Fprintf(os.Stderr, "[env] warning: failed to close default loader in ResetDefaultLoader: %v\n", err)
	}
}

// setDefaultLoader sets the given loader as the default loader.
// This is an internal function used by LoadWithConfig.
// Returns ErrAlreadyInitialized if already initialized.
func setDefaultLoader(loader *Loader) error {
	defaultMu.Lock()
	defer defaultMu.Unlock()

	if defaultLoader.Load() != nil {
		return ErrAlreadyInitialized
	}

	defaultLoader.Store(loader)
	return nil
}
