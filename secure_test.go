package env

import (
	"testing"
)

// ============================================================================
// SecureValue Tests
// ============================================================================

func TestNewSecureValue(t *testing.T) {
	sv := NewSecureValue("test")
	if sv == nil {
		t.Fatal("NewSecureValue() returned nil")
	}
	if sv.String() != "test" {
		t.Errorf("String() = %q, want %q", sv.String(), "test")
	}
}

func TestSecureValue_Empty(t *testing.T) {
	sv := NewSecureValue("")
	if sv.String() != "" {
		t.Errorf("String() = %q, want empty string", sv.String())
	}
}

func TestSecureValue_Bytes(t *testing.T) {
	sv := NewSecureValue("test")
	bytes := sv.Bytes()
	if string(bytes) != "test" {
		t.Errorf("Bytes() = %q, want %q", string(bytes), "test")
	}
}

func TestSecureValue_Length(t *testing.T) {
	sv := NewSecureValue("test")
	if sv.Length() != 4 {
		t.Errorf("Length() = %d, want 4", sv.Length())
	}

	svEmpty := NewSecureValue("")
	if svEmpty.Length() != 0 {
		t.Errorf("Length() = %d, want 0", svEmpty.Length())
	}
}

func TestSecureValue_Close(t *testing.T) {
	sv := NewSecureValue("test")

	if err := sv.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}

	if !sv.IsClosed() {
		t.Error("IsClosed() = false after Close()")
	}

	// Second close should be idempotent
	if err := sv.Close(); err != nil {
		t.Errorf("Second Close() error = %v", err)
	}
}

func TestSecureValue_Release(t *testing.T) {
	sv := NewSecureValue("test")

	sv.Release()

	if !sv.IsClosed() {
		t.Error("IsClosed() = false after Release()")
	}
}

func TestSecureValue_Masked(t *testing.T) {
	t.Run("with value", func(t *testing.T) {
		sv := NewSecureValue("test")
		if sv.Masked() != "[SECURE:4 bytes]" {
			t.Errorf("Masked() = %q, want %q", sv.Masked(), "[SECURE:4 bytes]")
		}
	})

	t.Run("closed", func(t *testing.T) {
		sv := NewSecureValue("test")
		sv.Close()
		if sv.Masked() != "[CLOSED]" {
			t.Errorf("Masked() = %q, want %q", sv.Masked(), "[CLOSED]")
		}
	})

	t.Run("empty", func(t *testing.T) {
		sv := NewSecureValue("")
		if sv.Masked() != "[SECURE:0 bytes]" {
			t.Errorf("Masked() = %q, want %q", sv.Masked(), "[SECURE:0 bytes]")
		}
	})
}

func TestSecureValuePool(t *testing.T) {
	// Create multiple SecureValues and release them back to pool
	for i := 0; i < 10; i++ {
		sv := NewSecureValue("test")
		sv.Release()
	}

	// Create new ones - should potentially reuse from pool
	for i := 0; i < 5; i++ {
		newSv := NewSecureValue("new")
		if newSv.String() != "new" {
			t.Errorf("New SecureValue from pool = %q, want %q", newSv.String(), "new")
		}
	}
}

// TestSecureValue_ResetStateConsistency tests the fix for C1:
// The reset() method should properly manage state transitions
// with data being cleared before new value is set.
func TestSecureValue_ResetStateConsistency(t *testing.T) {
	t.Run("empty_value_is_valid", func(t *testing.T) {
		sv := NewSecureValue("")
		// Empty value is a valid value, not closed
		if sv.IsClosed() {
			t.Error("Empty SecureValue should not be closed")
		}
		if sv.String() != "" {
			t.Errorf("Empty SecureValue String() = %q, want empty", sv.String())
		}
	})

	t.Run("non_empty_value_is_not_closed", func(t *testing.T) {
		sv := NewSecureValue("test")
		if sv.IsClosed() {
			t.Error("Non-empty SecureValue should not be closed")
		}
	})

	t.Run("reuse_from_pool_with_empty", func(t *testing.T) {
		// Create and release a value
		sv := NewSecureValue("initial")
		sv.Release()

		// Reuse with empty value
		sv2 := NewSecureValue("")
		// Empty is a valid value
		if sv2.IsClosed() {
			t.Error("Reused SecureValue with empty value should not be closed")
		}
	})

	t.Run("reuse_from_pool_preserves_state", func(t *testing.T) {
		// Create and release a value
		sv := NewSecureValue("initial")
		sv.Release()

		// Reuse with new value
		sv2 := NewSecureValue("newvalue")
		if sv2.String() != "newvalue" {
			t.Errorf("Reused SecureValue = %q, want %q", sv2.String(), "newvalue")
		}
		if sv2.IsClosed() {
			t.Error("Reused SecureValue with value should not be closed")
		}
	})
}

// ============================================================================
// secureMap Tests
// ============================================================================

func TestSecureMap_Basic(t *testing.T) {
	sm := newSecureMap()

	// Test Set
	sm.Set("KEY1", "value1")
	sm.Set("KEY2", "value2")

	// Test GetString
	if v, ok := sm.Get("KEY1"); !ok || v != "value1" {
		t.Errorf("GetString(\"KEY1\") = (%q, %v), want (\"value1\", true)", v, ok)
	}

	// Test missing key
	if _, ok := sm.Get("MISSING"); ok {
		t.Error("GetString(\"MISSING\") should return false for missing key")
	}
}

func TestSecureMap_SetAll(t *testing.T) {
	sm := newSecureMap()

	values := map[string]string{
		"KEY1": "value1",
		"KEY2": "value2",
		"KEY3": "value3",
	}

	sm.SetAll(values)

	if sm.Len() != 3 {
		t.Errorf("Len() = %d, want 3", sm.Len())
	}
}

func TestSecureMap_Delete(t *testing.T) {
	sm := newSecureMap()
	sm.Set("KEY1", "value1")
	sm.Set("KEY2", "value2")

	sm.Delete("KEY1")

	if _, ok := sm.Get("KEY1"); ok {
		t.Error("KEY1 should be deleted")
	}
	if sm.Len() != 1 {
		t.Errorf("Len() = %d, want 1", sm.Len())
	}
}

func TestSecureMap_Clear(t *testing.T) {
	sm := newSecureMap()
	sm.Set("KEY1", "value1")
	sm.Set("KEY2", "value2")

	sm.Clear()

	if sm.Len() != 0 {
		t.Errorf("Len() = %d, want 0", sm.Len())
	}
}

func TestSecureMap_Keys(t *testing.T) {
	sm := newSecureMap()
	sm.Set("KEY1", "value1")
	sm.Set("KEY2", "value2")

	keys := sm.Keys()
	if len(keys) != 2 {
		t.Errorf("Keys() returned %d keys, want 2", len(keys))
	}
}

func TestSecureMap_ToMap(t *testing.T) {
	sm := newSecureMap()
	sm.Set("KEY1", "value1")
	sm.Set("KEY2", "value2")

	m := sm.ToMap()
	if len(m) != 2 {
		t.Errorf("ToMap() returned %d keys, want 2", len(m))
	}
	if m["KEY1"] != "value1" {
		t.Errorf("ToMap()[\"KEY1\"] = %q, want %q", m["KEY1"], "value1")
	}
}

func TestSecureMap_GetSecure(t *testing.T) {
	sm := newSecureMap()
	sm.Set("KEY1", "value1")

	sv := sm.GetSecure("KEY1")
	if sv == nil {
		t.Fatal("GetSecure() returned nil")
	}
	if sv.String() != "value1" {
		t.Errorf("GetSecure().String() = %q, want %q", sv.String(), "value1")
	}

	// Test missing key
	if sm.GetSecure("MISSING") != nil {
		t.Error("GetSecure(\"MISSING\") should return nil")
	}
}

// ============================================================================
// ClearBytes Tests
// ============================================================================

func TestClearBytes(t *testing.T) {
	data := []byte("sensitive data")
	ClearBytes(data)

	for _, b := range data {
		if b != 0 {
			t.Error("ClearBytes() did not zero the data")
			return
		}
	}
}

// ============================================================================
// Memory Lock Tests
// ============================================================================

func TestMemoryLock_Config(t *testing.T) {
	// Save original state
	originalEnabled := IsMemoryLockEnabled()
	originalStrict := IsMemoryLockStrict()
	defer func() {
		SetMemoryLockEnabled(originalEnabled)
		SetMemoryLockStrict(originalStrict)
	}()

	t.Run("enable_disable", func(t *testing.T) {
		SetMemoryLockEnabled(true)
		if !IsMemoryLockEnabled() {
			t.Error("IsMemoryLockEnabled() = false after enabling")
		}

		SetMemoryLockEnabled(false)
		if IsMemoryLockEnabled() {
			t.Error("IsMemoryLockEnabled() = true after disabling")
		}
	})

	t.Run("strict_mode", func(t *testing.T) {
		SetMemoryLockStrict(true)
		if !IsMemoryLockStrict() {
			t.Error("IsMemoryLockStrict() = false after enabling strict mode")
		}

		SetMemoryLockStrict(false)
		if IsMemoryLockStrict() {
			t.Error("IsMemoryLockStrict() = true after disabling strict mode")
		}
	})
}

func TestMemoryLock_SecureValue(t *testing.T) {
	// Save original state
	originalEnabled := IsMemoryLockEnabled()
	defer SetMemoryLockEnabled(originalEnabled)

	t.Run("disabled_by_default", func(t *testing.T) {
		SetMemoryLockEnabled(false)
		sv := NewSecureValue("test")
		defer sv.Release()

		// Should not be locked when disabled
		if sv.IsMemoryLocked() {
			t.Error("IsMemoryLocked() = true when memory locking is disabled")
		}
	})

	t.Run("enabled_creates_locked_value", func(t *testing.T) {
		SetMemoryLockEnabled(true)
		sv := NewSecureValue("sensitive-data")
		defer sv.Release()

		// On systems with sufficient privileges, the value should be locked
		// We check that the function doesn't panic and returns a valid value
		if sv.String() != "sensitive-data" {
			t.Errorf("String() = %q, want %q", sv.String(), "sensitive-data")
		}

		// Check if there was a lock error (expected on systems without privileges)
		// This is informational, not a failure
		if err := sv.MemoryLockError(); err != nil {
			t.Logf("Memory lock error (expected on systems without privileges): %v", err)
		}
	})

	t.Run("release_unlocks_memory", func(t *testing.T) {
		SetMemoryLockEnabled(true)
		sv := NewSecureValue("test")
		sv.Release()

		// After release, should not report as locked
		if sv.IsMemoryLocked() {
			t.Error("IsMemoryLocked() = true after Release()")
		}
	})

	t.Run("close_unlocks_memory", func(t *testing.T) {
		SetMemoryLockEnabled(true)
		sv := NewSecureValue("test")
		sv.Close()

		// After close, should not report as locked
		if sv.IsMemoryLocked() {
			t.Error("IsMemoryLocked() = true after Close()")
		}
	})
}

func TestNewSecureValueStrict(t *testing.T) {
	// Save original state
	originalEnabled := IsMemoryLockEnabled()
	originalStrict := IsMemoryLockStrict()
	defer func() {
		SetMemoryLockEnabled(originalEnabled)
		SetMemoryLockStrict(originalStrict)
	}()

	t.Run("returns_error_on_lock_failure_in_strict_mode", func(t *testing.T) {
		SetMemoryLockEnabled(true)
		SetMemoryLockStrict(true)

		sv, err := NewSecureValueStrict("sensitive-data")
		defer sv.Release()

		// On systems without privileges, this may return an error
		// On systems with privileges, this should succeed
		if err != nil {
			t.Logf("NewSecureValueStrict() returned error (expected on systems without privileges): %v", err)
		}

		// The SecureValue should still be usable regardless of lock status
		if sv.String() != "sensitive-data" {
			t.Errorf("String() = %q, want %q", sv.String(), "sensitive-data")
		}
	})

	t.Run("disabled_locking_no_error", func(t *testing.T) {
		SetMemoryLockEnabled(false)

		sv, err := NewSecureValueStrict("sensitive-data")
		defer sv.Release()

		// When locking is disabled, no error should be returned
		if err != nil {
			t.Errorf("NewSecureValueStrict() returned error when locking disabled: %v", err)
		}

		if sv.String() != "sensitive-data" {
			t.Errorf("String() = %q, want %q", sv.String(), "sensitive-data")
		}
	})
}

func TestSecureValue_MemoryLockError(t *testing.T) {
	// Save original state
	originalEnabled := IsMemoryLockEnabled()
	defer SetMemoryLockEnabled(originalEnabled)

	t.Run("no_error_when_disabled", func(t *testing.T) {
		SetMemoryLockEnabled(false)
		sv := NewSecureValue("test")
		defer sv.Release()

		if err := sv.MemoryLockError(); err != nil {
			t.Errorf("MemoryLockError() = %v, want nil when disabled", err)
		}
	})
}

func TestIsMemoryLockSupported(t *testing.T) {
	// This test just verifies the function doesn't panic
	supported := IsMemoryLockSupported()
	t.Logf("IsMemoryLockSupported() = %v", supported)
}
