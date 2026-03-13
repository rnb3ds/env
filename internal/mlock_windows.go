package internal

import (
	"syscall"
	"unsafe"
)

var (
	kernel32          = syscall.NewLazyDLL("kernel32.dll")
	procVirtualLock   = kernel32.NewProc("VirtualLock")
	procVirtualUnlock = kernel32.NewProc("VirtualUnlock")
)

// MemLockSupported returns true on Windows systems.
func MemLockSupported() bool {
	return true
}

// LockMemory locks the specified memory region using VirtualLock.
// This prevents the memory from being swapped to disk.
// Note: On Windows, VirtualLock requires SE_LOCK_MEMORY_NAME privilege
// for locking more than 30 pages. For smaller allocations, it should work
// without special privileges.
func LockMemory(data []byte) error {
	if len(data) == 0 {
		return nil
	}

	// VirtualLock parameters
	// LPVOID lpAddress - pointer to the region to lock
	// SIZE_T dwSize - size of the region in bytes
	ret, _, err := procVirtualLock.Call(
		uintptr(unsafe.Pointer(&data[0])),
		uintptr(len(data)),
	)
	if ret == 0 {
		return err
	}
	return nil
}

// UnlockMemory unlocks the specified memory region using VirtualUnlock.
func UnlockMemory(data []byte) {
	if len(data) == 0 {
		return
	}

	// Best effort: ignore error on unlock
	// VirtualUnlock returns TRUE on success
	_, _, _ = procVirtualUnlock.Call(
		uintptr(unsafe.Pointer(&data[0])),
		uintptr(len(data)),
	)
}
