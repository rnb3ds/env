//go:build !linux && !darwin && !windows

// No-op memory locking fallback for every platform without stdlib
// syscall.Mlock — the BSDs, Solaris, Illumos, AIX, Plan 9, WASM, etc.
// (Windows uses mlock_windows.go; Linux and Darwin use mlock_unix.go.)
// The previous "!unix && ..." form wrongly excluded the BSDs and Solaris
// from this fallback, leaving those platforms with no implementation.

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
