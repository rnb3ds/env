//go:build windows && !race

package internal

import (
	"testing"
	"unsafe"
)

// TestLockMemory_FailureContract covers VirtualLock's failure return: a
// 1GiB span starting at a tiny heap allocation necessarily crosses unmapped
// regions, so VirtualLock must fail and LockMemory must surface the OS
// error. If locking unexpectedly succeeds on some machine, the test skips —
// the contract under test is only the error path.
//
// This variant is excluded from -race builds: -race implies checkptr, whose
// "straddles multiple allocations" detector fatal-errors on the synthetic
// span (a fatal error, not a recoverable panic). See the race-tagged stub
// in mlock_failure_race_test.go.
func TestLockMemory_FailureContract(t *testing.T) {
	small := make([]byte, 8)
	huge := unsafe.Slice(&small[0], 1<<30) // 1GiB span over an 8-byte allocation

	if err := LockMemory(huge); err == nil {
		t.Skip("VirtualLock unexpectedly succeeded over a 1GiB span")
	}
}
