//go:build windows && race

package internal

import (
	"testing"
)

// TestLockMemory_FailureContract is a -race stub: the real failure-path
// test (mlock_failure_norace_test.go) cannot run under -race because the
// implied checkptr detector fatal-errors on its synthetic 1GiB span.
func TestLockMemory_FailureContract(t *testing.T) {
	t.Skip("checkptr (implied by -race) rejects the synthetic over-allocation span")
}
