//go:build !unix && !linux && !darwin && !freebsd && !netbsd && !openbsd && !windows

package internal

import (
	"errors"
)

// ErrMemoryLockNotSupported is returned when memory locking is not supported
// on the current platform.
var ErrMemoryLockNotSupported = errors.New("memory locking not supported on this platform")

// MemLockSupported returns false on unsupported platforms.
func MemLockSupported() bool {
	return false
}

// LockMemory returns an error on unsupported platforms.
func LockMemory(_ []byte) error {
	return ErrMemoryLockNotSupported
}

// UnlockMemory is a no-op on unsupported platforms.
func UnlockMemory(_ []byte) {
	// No-op
}
