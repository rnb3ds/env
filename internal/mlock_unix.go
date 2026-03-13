//go:build unix || linux || darwin || freebsd || netbsd || openbsd

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
