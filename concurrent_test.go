package env

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
)

// ============================================================================
// Concurrent Access Tests for Loader
// ============================================================================

// TestLoader_ConcurrentGet tests concurrent read access to the loader.
func TestLoader_ConcurrentGet(t *testing.T) {
	cfg := DefaultConfig()
	loader, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer loader.Close()

	// Pre-populate with variables
	for i := 0; i < 100; i++ {
		loader.Set("KEY"+string(rune('A'+i%26)), "value")
	}

	var wg sync.WaitGroup
	errors := int64(0)
	iterations := 1000
	concurrency := 10

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				key := "KEY" + string(rune('A'+j%26))
				_ = loader.GetString(key)
				_, _ = loader.Lookup(key)
				_ = loader.GetSecure(key)
			}
		}(i)
	}

	wg.Wait()
	if atomic.LoadInt64(&errors) > 0 {
		t.Errorf("concurrent Get operations had %d errors", errors)
	}
}

// TestLoader_ConcurrentSet tests concurrent write access to the loader.
func TestLoader_ConcurrentSet(t *testing.T) {
	cfg := DefaultConfig()
	cfg.OverwriteExisting = true
	loader, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer loader.Close()

	var wg sync.WaitGroup
	iterations := 1000
	concurrency := 10

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				key := "KEY" + string(rune('A'+j%26))
				loader.Set(key, "value")
			}
		}(i)
	}

	wg.Wait()
}

// TestLoader_ConcurrentReadWrite tests concurrent read and write access.
func TestLoader_ConcurrentReadWrite(t *testing.T) {
	cfg := DefaultConfig()
	cfg.OverwriteExisting = true
	loader, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer loader.Close()

	// Pre-populate
	for i := 0; i < 50; i++ {
		loader.Set("KEY"+string(rune('A'+i%26)), "initial")
	}

	var wg sync.WaitGroup
	iterations := 500

	// Writers
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				key := "KEY" + string(rune('A'+j%26))
				loader.Set(key, "writer_value")
			}
		}(i)
	}

	// Readers
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				key := "KEY" + string(rune('A'+j%26))
				_ = loader.GetString(key)
				_ = loader.Keys()
				_ = loader.All()
				_ = loader.Len()
			}
		}(i)
	}

	// Deleters
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations/10; j++ {
				key := "KEY" + string(rune('A'+j%26))
				loader.Delete(key)
			}
		}(i)
	}

	wg.Wait()
}

// TestLoader_ConcurrentWithClose tests concurrent operations with Close.
func TestLoader_ConcurrentWithClose(t *testing.T) {
	for run := 0; run < 10; run++ {
		cfg := DefaultConfig()
		loader, err := New(cfg)
		if err != nil {
			t.Fatal(err)
		}

		var wg sync.WaitGroup
		iterations := 100
		closed := int64(0)

		// Operations
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < iterations; j++ {
					loader.Set("KEY", "value")
					_ = loader.GetString("KEY")
					_ = loader.Keys()
				}
			}()
		}

		// Closer
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				if atomic.CompareAndSwapInt64(&closed, 0, 1) {
					loader.Close()
				}
			}
		}()

		wg.Wait()
	}
}

// ============================================================================
// Concurrent Access Tests for SecureMap
// ============================================================================

// TestSecureMap_ConcurrentAccess tests concurrent access to secureMap.
func TestSecureMap_ConcurrentAccess(t *testing.T) {
	sm := newSecureMap()

	var wg sync.WaitGroup
	iterations := 1000
	concurrency := 10

	// Writers
	for i := 0; i < concurrency/2; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				key := "KEY" + string(rune('A'+j%26))
				sm.Set(key, "value")
			}
		}(i)
	}

	// Readers
	for i := 0; i < concurrency/2; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				key := "KEY" + string(rune('A'+j%26))
				sm.Get(key)
				sm.GetSecure(key)
			}
		}(i)
	}

	wg.Wait()
}

// TestSecureMap_ConcurrentSetAll tests concurrent SetAll operations.
func TestSecureMap_ConcurrentSetAll(t *testing.T) {
	sm := newSecureMap()

	var wg sync.WaitGroup
	iterations := 100
	concurrency := 5

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				values := map[string]string{
					"KEY_A": "value_a",
					"KEY_B": "value_b",
					"KEY_C": "value_c",
				}
				sm.SetAll(values)
			}
		}(i)
	}

	wg.Wait()
}

// TestSecureMap_ConcurrentClear tests concurrent operations with Clear.
func TestSecureMap_ConcurrentClear(t *testing.T) {
	for run := 0; run < 10; run++ {
		sm := newSecureMap()

		// Pre-populate
		for i := 0; i < 100; i++ {
			sm.Set("KEY"+string(rune('A'+i%26)), "value")
		}

		var wg sync.WaitGroup
		iterations := 100
		cleared := int64(0)

		// Operations
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < iterations; j++ {
					sm.Set("KEY", "value")
					sm.Get("KEY")
					_ = sm.Keys()
					_ = sm.Len()
				}
			}()
		}

		// Clearer
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations/10; j++ {
				if atomic.CompareAndSwapInt64(&cleared, 0, 1) {
					sm.Clear()
					atomic.StoreInt64(&cleared, 0)
				}
			}
		}()

		wg.Wait()
	}
}

// ============================================================================
// Concurrent Access Tests for SecureValue
// ============================================================================

// TestSecureValue_ConcurrentAccess tests concurrent access to SecureValue.
func TestSecureValue_ConcurrentAccess(t *testing.T) {
	sv := NewSecureValue("test_value")

	var wg sync.WaitGroup
	iterations := 1000
	concurrency := 10

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				_ = sv.String()
				_ = sv.Bytes()
				_ = sv.Length()
				_ = sv.Masked()
				_ = sv.IsClosed()
			}
		}()
	}

	wg.Wait()
}

// TestSecureValue_ConcurrentWithClose tests concurrent access with Close.
func TestSecureValue_ConcurrentWithClose(t *testing.T) {
	for run := 0; run < 10; run++ {
		sv := NewSecureValue("test_value")

		var wg sync.WaitGroup
		iterations := 100
		closed := int64(0)

		// Readers
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < iterations; j++ {
					_ = sv.String()
					_ = sv.Bytes()
					_ = sv.Length()
					_ = sv.IsClosed()
				}
			}()
		}

		// Closer
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				if atomic.CompareAndSwapInt64(&closed, 0, 1) {
					sv.Close()
				}
			}
		}()

		wg.Wait()
	}
}

// ============================================================================
// Concurrent Access Tests for Parser Registry
// ============================================================================

// TestParserRegistry_ConcurrentAccess tests concurrent access to the parser registry.
func TestParserRegistry_ConcurrentAccess(t *testing.T) {
	var wg sync.WaitGroup
	iterations := 100
	concurrency := 10

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				cfg := DefaultConfig()
				factory := cfg.buildComponentFactory()
				_, err := createParsers(cfg, factory)
				if err != nil {
					t.Error(err)
				}
				factory.Close()
			}
		}()
	}

	wg.Wait()
}

// ============================================================================
// Concurrent Access Tests for ComponentFactory
// ============================================================================

// TestComponentFactory_ConcurrentAccess tests concurrent access to ComponentFactory.
func TestComponentFactory_ConcurrentAccess(t *testing.T) {
	cfg := DefaultConfig()
	factory := cfg.buildComponentFactory()
	defer factory.Close()

	var wg sync.WaitGroup
	iterations := 1000
	concurrency := 10

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				_ = factory.Validator()
				_ = factory.Auditor()
				_ = factory.internalExpander()
				_ = factory.IsClosed()
			}
		}()
	}

	wg.Wait()
}

// TestComponentFactory_ConcurrentClose tests concurrent Close operations.
func TestComponentFactory_ConcurrentClose(t *testing.T) {
	for run := 0; run < 10; run++ {
		cfg := DefaultConfig()
		factory := cfg.buildComponentFactory()

		var wg sync.WaitGroup
		iterations := 100

		// Users
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < iterations; j++ {
					_ = factory.Validator()
					_ = factory.Auditor()
					_ = factory.IsClosed()
				}
			}()
		}

		// Closers
		for i := 0; i < 3; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < iterations/10; j++ {
					factory.Close()
				}
			}()
		}

		wg.Wait()
	}
}

// ============================================================================
// Stress Tests
// ============================================================================

// TestStress_HighConcurrency tests the loader under high concurrency.
func TestStress_HighConcurrency(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	cfg := DefaultConfig()
	cfg.OverwriteExisting = true
	loader, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer loader.Close()

	// Pre-populate with many variables
	for i := 0; i < 1000; i++ {
		loader.Set("STRESS_KEY_"+string(rune(i%256)), "value")
	}

	var wg sync.WaitGroup
	iterations := 10000
	concurrency := 50

	// Mixed operations
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				switch j % 5 {
				case 0:
					loader.Set("STRESS_KEY_"+string(rune(j%256)), "new_value")
				case 1:
					_ = loader.GetString("STRESS_KEY_" + string(rune(j%256)))
				case 2:
					_ = loader.Keys()
				case 3:
					_ = loader.Len()
				case 4:
					_, _ = loader.Lookup("STRESS_KEY_" + string(rune(j%256)))
				}
			}
		}(i)
	}

	wg.Wait()
}

// ============================================================================
// Concurrent Apply and Validate Tests
// ============================================================================

func TestLoader_ConcurrentApplyValidate(t *testing.T) {
	cfg := DefaultConfig()
	cfg.RequiredKeys = []string{"KEY1", "KEY2"}
	loader, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer loader.Close()

	var wg sync.WaitGroup
	iterations := 100

	// Concurrent Apply
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				loader.Set("KEY1", "value")
				_ = loader.Apply()
			}
		}(i)
	}

	// Concurrent Validate
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				_ = loader.Validate()
			}
		}()
	}

	wg.Wait()
}

// ============================================================================
// Edge Case Concurrency Tests
// ============================================================================

// TestSecureMap_ConcurrentDeleteAndRead tests concurrent delete while reading.
func TestSecureMap_ConcurrentDeleteAndRead(t *testing.T) {
	for run := 0; run < 5; run++ {
		sm := newSecureMap()

		// Pre-populate with many keys
		for i := 0; i < 100; i++ {
			sm.Set("KEY_"+string(rune('A'+i%26)), "value")
		}

		var wg sync.WaitGroup
		iterations := 500

		// Deleters
		for i := 0; i < 3; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < iterations; j++ {
					key := "KEY_" + string(rune('A'+j%26))
					sm.Delete(key)
				}
			}(i)
		}

		// Readers
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < iterations; j++ {
					key := "KEY_" + string(rune('A'+j%26))
					sm.Get(key)
					sm.GetSecure(key)
				}
			}(i)
		}

		// Writers
		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < iterations; j++ {
					key := "KEY_" + string(rune('A'+j%26))
					sm.Set(key, "new_value")
				}
			}(i)
		}

		wg.Wait()
	}
}

// TestSecureMap_ConcurrentToMapAndModify tests concurrent ToMap with modifications.
func TestSecureMap_ConcurrentToMapAndModify(t *testing.T) {
	sm := newSecureMap()

	// Pre-populate
	for i := 0; i < 50; i++ {
		sm.Set("KEY_"+string(rune('A'+i%26)), "value")
	}

	var wg sync.WaitGroup
	iterations := 200

	// ToMap callers
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				m := sm.ToMap()
				_ = len(m)
			}
		}()
	}

	// Modifiers
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				key := "KEY_" + string(rune('A'+j%26))
				sm.Set(key, "modified")
				sm.Delete(key)
			}
		}(i)
	}

	wg.Wait()
}

// TestSecureMap_ConcurrentKeysAndClear tests concurrent Keys() with Clear().
func TestSecureMap_ConcurrentKeysAndClear(t *testing.T) {
	for run := 0; run < 5; run++ {
		sm := newSecureMap()

		// Pre-populate
		for i := 0; i < 100; i++ {
			sm.Set("KEY_"+string(rune('A'+i%26)), "value")
		}

		var wg sync.WaitGroup
		iterations := 100
		cleared := int64(0)

		// Keys callers
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < iterations; j++ {
					keys := sm.Keys()
					_ = keys
				}
			}()
		}

		// Clear callers
		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < iterations/10; j++ {
					if atomic.CompareAndSwapInt64(&cleared, 0, 1) {
						sm.Clear()
						atomic.StoreInt64(&cleared, 0)
					}
				}
			}()
		}

		wg.Wait()
	}
}

// TestSecureValue_ConcurrentReleaseAndRead tests concurrent Release with read operations.
func TestSecureValue_ConcurrentReleaseAndRead(t *testing.T) {
	for run := 0; run < 10; run++ {
		sv := NewSecureValue("test_value_for_concurrent_release")

		var wg sync.WaitGroup
		iterations := 100
		released := int64(0)

		// Readers
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < iterations; j++ {
					_ = sv.String()
					_ = sv.Bytes()
					_ = sv.Length()
					_ = sv.IsClosed()
					_ = sv.Masked()
				}
			}()
		}

		// Releaser
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				if atomic.CompareAndSwapInt64(&released, 0, 1) {
					sv.Release()
				}
			}
		}()

		wg.Wait()
	}
}

// TestSecureValue_PoolReuseConcurrency tests pool reuse under concurrent access.
func TestSecureValue_PoolReuseConcurrency(t *testing.T) {
	var wg sync.WaitGroup
	iterations := 500
	concurrency := 20

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				sv := NewSecureValue("concurrent_pool_test")
				_ = sv.String()
				_ = sv.Length()
				sv.Release()
			}
		}(i)
	}

	wg.Wait()
}

// TestLoader_ConcurrentSetDelete tests concurrent Set and Delete operations.
func TestLoader_ConcurrentSetDelete(t *testing.T) {
	cfg := DefaultConfig()
	cfg.OverwriteExisting = true
	loader, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer loader.Close()

	var wg sync.WaitGroup
	iterations := 500

	// Setters
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				key := "KEY_" + string(rune('A'+j%26))
				loader.Set(key, "value")
			}
		}(i)
	}

	// Deleters
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				key := "KEY_" + string(rune('A'+j%26))
				loader.Delete(key)
			}
		}(i)
	}

	// Readers
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				key := "KEY_" + string(rune('A'+j%26))
				_ = loader.GetString(key)
				_, _ = loader.Lookup(key)
			}
		}(i)
	}

	wg.Wait()
}

// TestLoader_ConcurrentAllAndModify tests concurrent All() with modifications.
func TestLoader_ConcurrentAllAndModify(t *testing.T) {
	cfg := DefaultConfig()
	cfg.OverwriteExisting = true
	loader, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer loader.Close()

	// Pre-populate
	for i := 0; i < 50; i++ {
		loader.Set("KEY_"+string(rune('A'+i%26)), "initial")
	}

	var wg sync.WaitGroup
	iterations := 200

	// All() callers
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				all := loader.All()
				_ = len(all)
			}
		}()
	}

	// Modifiers
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				key := "KEY_" + string(rune('A'+j%26))
				loader.Set(key, "modified")
			}
		}(i)
	}

	wg.Wait()
}

// TestSecureValue_ConcurrentWithMemoryLock tests concurrent operations with memory locking enabled.
func TestSecureValue_ConcurrentWithMemoryLock(t *testing.T) {
	// Save original state
	originalEnabled := IsMemoryLockEnabled()
	defer SetMemoryLockEnabled(originalEnabled)

	// Test with memory locking enabled
	SetMemoryLockEnabled(true)

	var wg sync.WaitGroup
	iterations := 200
	concurrency := 10

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				sv := NewSecureValue("concurrent_memory_lock_test")
				_ = sv.String()
				_ = sv.IsMemoryLocked()
				_ = sv.MemoryLockError()
				sv.Release()
			}
		}(i)
	}

	wg.Wait()
}

// Singleton Tests
// ============================================================================

func TestGetDefaultLoader(t *testing.T) {
	ResetDefaultLoader()
	defer ResetDefaultLoader()

	loader, err := getDefaultLoader()
	if err != nil {
		t.Fatalf("getDefaultLoader() error = %v", err)
	}
	if loader == nil {
		t.Fatal("getDefaultLoader() returned nil")
	}
}

func TestResetDefaultLoader(t *testing.T) {
	ResetDefaultLoader()
	defer ResetDefaultLoader()

	// Create and reset multiple times
	for i := 0; i < 3; i++ {
		ResetDefaultLoader()
	}

	// Check that reset cleans up properly
	loader, err := getDefaultLoader()
	if err != nil {
		t.Fatalf("getDefaultLoader() after reset error = %v", err)
	}
	if loader == nil {
		t.Fatal("getDefaultLoader() returned nil")
	}
}

func TestSetDefaultLoader(t *testing.T) {
	ResetDefaultLoader()
	defer ResetDefaultLoader()

	loader, err := New(DefaultConfig())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := setDefaultLoader(loader); err != nil {
		t.Fatalf("setDefaultLoader() error = %v", err)
	}
}

func TestSetDefaultLoader_AlreadyInitialized(t *testing.T) {
	ResetDefaultLoader()
	defer ResetDefaultLoader()

	// First initialization should succeed
	loader, err := New(DefaultConfig())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := setDefaultLoader(loader); err != nil {
		t.Fatalf("setDefaultLoader() error = %v", err)
	}

	// Second initialization should fail
	loader2, err := New(DefaultConfig())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := setDefaultLoader(loader2); err == nil {
		t.Error("setDefaultLoader() should fail with ErrAlreadyInitialized")
	} else if !errors.Is(err, ErrAlreadyInitialized) {
		t.Errorf("setDefaultLoader() error = %v, want ErrAlreadyInitialized", err)
	}
}

// ============================================================================
// Concurrent Access Tests for Singleton
// ============================================================================

// TestSingleton_ConcurrentAccess tests concurrent access to the default loader.
func TestSingleton_ConcurrentAccess(t *testing.T) {
	ResetDefaultLoader()
	defer ResetDefaultLoader()

	var wg sync.WaitGroup
	iterations := 100
	concurrency := 10
	successCount := int64(0)

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				loader, err := getDefaultLoader()
				if err == nil && loader != nil {
					atomic.AddInt64(&successCount, 1)
				}
			}
		}()
	}

	wg.Wait()

	expected := int64(concurrency * iterations)
	if atomic.LoadInt64(&successCount) != expected {
		t.Errorf("successCount = %d, want %d", successCount, expected)
	}
}

// TestSingleton_ConcurrentReset tests concurrent access with reset.
func TestSingleton_ConcurrentReset(t *testing.T) {
	for run := 0; run < 10; run++ {
		ResetDefaultLoader()

		var wg sync.WaitGroup
		iterations := 50

		// Getters
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < iterations; j++ {
					loader, _ := getDefaultLoader()
					if loader != nil {
						_ = loader.GetString("KEY")
					}
				}
			}()
		}

		// Resetters
		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < iterations/10; j++ {
					ResetDefaultLoader()
				}
			}()
		}

		wg.Wait()
		ResetDefaultLoader()
	}
}
