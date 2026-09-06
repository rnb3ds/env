//go:build windows

package internal

import (
	"testing"
)

// TestMemLockSupported_Windows pins the platform contract: memory locking is
// declared as supported on Windows (VirtualLock).
func TestMemLockSupported_Windows(t *testing.T) {
	if !MemLockSupported() {
		t.Error("MemLockSupported() = false on Windows, want true")
	}
}

// TestLockMemory_EmptySlice pins the zero-length contract: locking and
// unlocking an empty slice must be immediate no-ops, not VirtualLock calls
// with a nil/zero pointer.
func TestLockMemory_EmptySlice(t *testing.T) {
	if err := LockMemory(nil); err != nil {
		t.Errorf("LockMemory(nil) error = %v, want nil", err)
	}
	UnlockMemory(nil) // must not panic
}

// TestLockUnlockRoundTrip exercises a small page-sized lock/unlock cycle.
// VirtualLock may legitimately fail (working-set limits, group policy), so
// the lock result is tolerated; the contract under test is that both calls
// return without panicking and that unlocking a locked region succeeds.
func TestLockUnlockRoundTrip(t *testing.T) {
	buf := make([]byte, 4096)

	err := LockMemory(buf)
	if err != nil {
		t.Skipf("VirtualLock denied by the OS (acceptable): %v", err)
	}

	UnlockMemory(buf) // best-effort; must not panic
}
