//go:build linux || darwin

// mlock/munlock via the syscall package. The tag list is exactly the
// platforms where stdlib syscall.Mlock/Munlock exist: the previous
// "unix || ..." form also pulled in the BSDs, Solaris and Illumos, where
// syscall.Mlock is undefined and the package failed to compile. Platforms
// without mlock take the no-op fallback in mlock_other.go.

package internal

import (
	"syscall"
)

// MemLockSupported returns true on Unix systems.
func MemLockSupported() bool {
	return true
}

// LockMemory locks the specified memory region using mlock.
// This prevents the memory from being swapped to disk.
func LockMemory(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	return syscall.Mlock(data)
}

// UnlockMemory unlocks the specified memory region using munlock.
func UnlockMemory(data []byte) {
	if len(data) == 0 {
		return
	}
	// Best effort: ignore error on unlock
	_ = syscall.Munlock(data)
}
