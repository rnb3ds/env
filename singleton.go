package env

import (
	"errors"
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
// Returns any error that occurred while closing the old loader.
// A nil return value indicates either no loader was set or it was closed successfully.
//
// Design: The function atomically swaps the loader to nil while holding the lock,
// then closes the old loader outside the lock. This design choice:
//   - Ensures only one reset can happen at a time (mutex protected)
//   - Allows new loaders to be created immediately after the swap
//   - Avoids blocking concurrent operations during potentially slow Close()
//   - The old and new loaders are completely independent
//
// Concurrent getDefaultLoader() calls during reset will:
//   - Either see the old loader (before swap) - safe to use
//   - Or see nil and create a new loader (after swap) - safe to use
//
// They will never receive a closed loader.
//
// Example:
//
//	func TestSomething(t *testing.T) {
//	    if err := env.ResetDefaultLoader(); err != nil {
//	        t.Logf("warning: failed to reset loader: %v", err)
//	    }
//	    defer env.ResetDefaultLoader()
//	    // ... test code
//	}
func ResetDefaultLoader() error {
	defaultMu.Lock()
	// Atomically swap to nil while holding the lock.
	// This ensures only one reset can happen at a time.
	oldLoader := defaultLoader.Swap(nil)
	defaultMu.Unlock()

	// Close outside the lock to avoid blocking concurrent operations.
	// The old loader is no longer accessible via defaultLoader,
	// so this is safe even if new loaders are created concurrently.
	// The old and new loaders are independent - closing one doesn't affect the other.
	if oldLoader != nil {
		return oldLoader.Close()
	}
	return nil
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
