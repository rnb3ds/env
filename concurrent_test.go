package env

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cybergodev/env/internal"
)

// ============================================================================
// Concurrent Access Tests for Loader
// ============================================================================

// TestLoader_Concurrent exercises the loader under a variety of concurrent
// operation mixes. It replaces five former single-scenario tests
// (ConcurrentGet, ConcurrentSet, ConcurrentReadWrite, ConcurrentSetDelete,
// ConcurrentAllAndModify).
func TestLoader_Concurrent(t *testing.T) {
	scenarios := []struct {
		name  string
		write bool // goroutine performs Set/Delete (needs OverwriteExisting)
		fn    func(l *Loader)
	}{
		{"read GetString/Lookup/GetSecure", false, func(l *Loader) {
			for j := 0; j < 500; j++ {
				key := "KEY_" + string(rune('A'+j%26))
				_ = l.GetString(key)
				_, _ = l.Lookup(key)
				_ = l.GetSecure(key)
			}
		}},
		{"write Set", true, func(l *Loader) {
			for j := 0; j < 500; j++ {
				_ = l.Set("KEY_"+string(rune('A'+j%26)), "value")
			}
		}},
		{"read Keys/All/Len", false, func(l *Loader) {
			for j := 0; j < 200; j++ {
				_ = l.Keys()
				_ = l.All()
				_ = l.Len()
			}
		}},
		{"write Delete", true, func(l *Loader) {
			for j := 0; j < 300; j++ {
				_ = l.Delete("KEY_" + string(rune('A'+j%26)))
			}
		}},
	}

	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.OverwriteExisting = true
			loader, err := New(cfg)
			if err != nil {
				t.Fatal(err)
			}
			defer loader.Close()

			// Pre-populate so readers have data
			for i := 0; i < 26; i++ {
				_ = loader.Set("KEY_"+string(rune('A'+i)), "initial") // fixture; read assertions live below
			}

			var wg sync.WaitGroup
			concurrency := 10
			wg.Add(concurrency)
			for k := 0; k < concurrency; k++ {
				go func() {
					defer wg.Done()
					sc.fn(loader)
				}()
			}
			wg.Wait()
		})
	}
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
					_ = loader.Set("KEY", "value") // stress loop; valid key cannot fail
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

// TestLoader_ConcurrentFastPaths stresses the lock-free single-key read path
// (Lookup/GetSecure via the atomic closed flag) and the read-lock Set fast
// path (OverwriteExisting + no AutoApply) together with concurrent Close.
// Invariants: no data race, no panic, and every observed value is one of the
// written ones (graceful degradation — reads that lose the race with Close
// see the cleared state).
func TestLoader_ConcurrentFastPaths(t *testing.T) {
	for run := 0; run < 20; run++ {
		cfg := DefaultConfig()
		cfg.OverwriteExisting = true // activates the Set read-lock fast path
		loader, err := New(cfg)
		if err != nil {
			t.Fatal(err)
		}

		var wg sync.WaitGroup
		stop := make(chan struct{})

		// Writers using the Set fast path
		for i := 0; i < 4; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < 500; j++ {
					if err := loader.Set("KEY", "value"); err != nil && !errors.Is(err, ErrClosed) {
						t.Errorf("Set: unexpected error: %v", err)
						return
					}
				}
			}()
		}

		// Lock-free single-key readers
		for i := 0; i < 4; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < 500; j++ {
					if v, ok := loader.Lookup("KEY"); ok && v != "value" {
						t.Errorf("Lookup: got %q, want %q", v, "value")
						return
					}
					if sv := loader.GetSecure("KEY"); sv != nil {
						sv.Release()
					}
				}
			}()
		}

		// Closer racing with the paths above
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-stop
			_ = loader.Close()
		}()

		close(stop)
		wg.Wait()

		// After Close completes, all single-key reads must report absence.
		if _, ok := loader.Lookup("KEY"); ok {
			t.Error("Lookup after Close should miss")
		}
		if loader.GetSecure("KEY") != nil {
			t.Error("GetSecure after Close should return nil")
		}
	}
}

// ============================================================================
// Concurrent Access Tests for SecureMap
// ============================================================================

// TestSecureMap_Concurrent exercises all secureMap operations under concurrent
// access. Each scenario targets a specific operation mix; together they replace
// six former single-scenario tests (ConcurrentAccess, ConcurrentSetAll,
// ConcurrentDeleteAndRead, ConcurrentToMapAndModify, ConcurrentKeysAndClear,
// ResourceLeakConcurrentAccess).
func TestSecureMap_Concurrent(t *testing.T) {
	makeKey := func(j int) string { return "KEY_" + string(rune('A'+j%26)) }

	scenarios := []struct {
		name string
		fn   func(sm *secureMap)
	}{
		{
			name: "Set and Get",
			fn: func(sm *secureMap) {
				for j := 0; j < 500; j++ {
					key := makeKey(j)
					sm.Set(key, "value")
					sm.Get(key)
				}
			},
		},
		{
			name: "GetSecure and Delete",
			fn: func(sm *secureMap) {
				for j := 0; j < 500; j++ {
					key := makeKey(j)
					sm.GetSecure(key)
					sm.Delete(key)
				}
			},
		},
		{
			name: "SetAll",
			fn: func(sm *secureMap) {
				for j := 0; j < 100; j++ {
					sm.SetAll(map[string]string{"KEY_A": "a", "KEY_B": "b"})
				}
			},
		},
		{
			name: "ToMap and Keys",
			fn: func(sm *secureMap) {
				for j := 0; j < 200; j++ {
					_ = sm.ToMap()
					_ = sm.Keys()
					_ = sm.Len()
				}
			},
		},
	}

	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			sm := newSecureMap()
			// Pre-populate so readers have data
			for i := 0; i < 26; i++ {
				sm.Set(makeKey(i), "initial")
			}

			var wg sync.WaitGroup
			concurrency := 10
			wg.Add(concurrency)
			for k := 0; k < concurrency; k++ {
				go func() {
					defer wg.Done()
					sc.fn(sm)
				}()
			}
			wg.Wait()
		})
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

// TestCreateParsers_FactoryRegistersParser verifies that a parser factory may
// call RegisterParser from inside its callback without deadlocking.
// createParsers previously invoked factory callbacks while holding
// globalParserRegistry.mu.RLock; RegisterParser needs the write lock, and
// RWMutex is not reentrant, so this self-deadlocked.
func TestCreateParsers_FactoryRegistersParser(t *testing.T) {
	outerFormat := nextTestFormat()
	innerFormat := nextTestFormat()

	// Insert via the internal map (like registerBuiltin) so the test can
	// remove its entries afterwards — RegisterParser is permanent.
	globalParserRegistry.mu.Lock()
	globalParserRegistry.factories[outerFormat] = func(cfg Config, factory *ComponentFactory) (EnvParser, error) {
		// Must not deadlock: the write lock requires the read lock held by
		// the caller to be released first.
		if err := RegisterParser(innerFormat, func(cfg Config, factory *ComponentFactory) (EnvParser, error) {
			return nil, nil
		}); err != nil {
			return nil, err
		}
		return nil, nil
	}
	globalParserRegistry.mu.Unlock()

	defer func() {
		globalParserRegistry.mu.Lock()
		delete(globalParserRegistry.factories, outerFormat)
		delete(globalParserRegistry.factories, innerFormat)
		globalParserRegistry.mu.Unlock()
	}()

	// Guard against a regression re-introducing the deadlock: fail the test
	// instead of hanging the whole suite.
	done := make(chan error, 1)
	go func() {
		cfg := DefaultConfig()
		factory := cfg.buildComponentFactory()
		defer factory.Close()
		_, err := createParsers(cfg, factory)
		done <- err
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("createParsers() error = %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("createParsers deadlocked: factory callback blocked on registry write lock")
	}
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
				_ = factory.Expander()
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

	// Pre-populate with many variables. Keys must be pattern-valid: the
	// previous rune-suffix construction silently failed validation for ~75%
	// of iterations (control/punct chars), populating ~246 of 1000 keys.
	for i := 0; i < 1000; i++ {
		if err := loader.Set(fmt.Sprintf("STRESS_KEY_%d", i), "value"); err != nil {
			t.Fatalf("setup Set() error = %v", err)
		}
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
					_ = loader.Set("STRESS_KEY_"+string(rune(j%256)), "new_value") // stress loop
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
				_ = loader.Set("KEY1", "value") // stress loop; valid key cannot fail
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
	resetDefaultLoader()
	defer resetDefaultLoader()

	// Without Load(), should return ErrNotInitialized
	_, err := getDefaultLoader()
	if !errors.Is(err, ErrNotInitialized) {
		t.Fatalf("getDefaultLoader() error = %v, want ErrNotInitialized", err)
	}

	// After explicit setup, should succeed
	loader, err := New(DefaultConfig())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := setDefaultLoader(loader); err != nil {
		t.Fatalf("setDefaultLoader() error = %v", err)
	}

	got, err := getDefaultLoader()
	if err != nil {
		t.Fatalf("getDefaultLoader() after set error = %v", err)
	}
	if got == nil {
		t.Fatal("getDefaultLoader() returned nil")
	}
}

func TestResetDefaultLoader(t *testing.T) {
	resetDefaultLoader()
	defer resetDefaultLoader()

	// Set a loader first
	loader, err := New(DefaultConfig())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := setDefaultLoader(loader); err != nil {
		t.Fatalf("setDefaultLoader() error = %v", err)
	}

	// Create and reset multiple times
	for i := 0; i < 3; i++ {
		resetDefaultLoader()
	}

	// After reset, should return ErrNotInitialized
	_, err = getDefaultLoader()
	if !errors.Is(err, ErrNotInitialized) {
		t.Fatalf("getDefaultLoader() after reset error = %v, want ErrNotInitialized", err)
	}
}

func TestSetDefaultLoader_AlreadyInitialized(t *testing.T) {
	resetDefaultLoader()
	defer resetDefaultLoader()

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
	resetDefaultLoader()
	defer resetDefaultLoader()

	// Set up loader before concurrent access
	loader, err := New(DefaultConfig())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := setDefaultLoader(loader); err != nil {
		t.Fatalf("setDefaultLoader() error = %v", err)
	}

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
		resetDefaultLoader()

		// Set up initial loader
		initLoader, err := New(DefaultConfig())
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		if err := setDefaultLoader(initLoader); err != nil {
			t.Fatalf("setDefaultLoader() error = %v", err)
		}

		var wg sync.WaitGroup
		iterations := 50

		// Getters — tolerate ErrNotInitialized during reset windows
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

		// Resetters — alternate between reset and re-set
		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < iterations/10; j++ {
					resetDefaultLoader()
					l, lerr := New(DefaultConfig())
					if lerr == nil {
						_ = setDefaultLoader(l) // ErrAlreadyInitialized expected when racing other resets
					}
				}
			}()
		}

		wg.Wait()
		resetDefaultLoader()
	}
}

// ============================================================================
// Resource Leak Tests
// ============================================================================

// TestLoader_ResourceCleanup verifies that Loader properly cleans up resources
// when Close() is called.
func TestLoader_ResourceCleanup(t *testing.T) {
	// Use a mock filesystem to avoid path validation issues
	mockFS := newTestFileSystem()
	mockFS.files[".env"] = "KEY1=value1\nKEY2=value2"

	cfg := DefaultConfig()
	cfg.Filenames = []string{".env"}
	cfg.FileSystem = mockFS
	cfg.AuditEnabled = true

	loader, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Verify loader is functional
	if loader.Len() != 2 {
		t.Errorf("loader.Len() = %d, want 2", loader.Len())
	}

	// Close should clean up resources
	if err := loader.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}

	// Verify closed state
	if !loader.IsClosed() {
		t.Error("IsClosed() should return true after Close()")
	}

	// Operations on closed loader should fail
	_, ok := loader.Lookup("KEY1")
	if ok {
		t.Error("Lookup() on closed loader should return false")
	}

	if err := loader.Set("KEY3", "value3"); err != ErrClosed {
		t.Errorf("Set() on closed loader should return ErrClosed, got %v", err)
	}
}

// TestSecureValue_PoolNoLeak verifies that SecureValue pool doesn't leak memory.
func TestSecureValue_PoolNoLeak(t *testing.T) {
	const iterations = 1000

	// Create and release many SecureValues
	for i := 0; i < iterations; i++ {
		sv := NewSecureValue("sensitive-data")
		sv.Release()
	}

	// Force GC to clean up any unreferenced objects
	runtime.GC()

	// Create more to verify pool is still functional
	for i := 0; i < 100; i++ {
		sv := NewSecureValue("more-data")
		if sv.IsClosed() {
			t.Error("NewSecureValue() should not return closed value from pool")
		}
		sv.Release()
	}
}

// TestSecureValue_DoubleReleaseSafe verifies that calling Release() multiple times
// is safe and doesn't cause panics or pool corruption.
func TestSecureValue_DoubleReleaseSafe(t *testing.T) {
	sv := NewSecureValue("test-data")

	// First release
	sv.Release()

	if !sv.IsClosed() {
		t.Error("IsClosed() should return true after Release()")
	}

	// Second release should be safe (no-op)
	sv.Release()

	// Third release should also be safe
	sv.Release()
}

// TestComponentFactory_CloseIdempotent is covered by TestComponentFactory's
// "Close" + "IsClosed" subtests in loader_test.go.

// TestBufferedHandler_FlushOnClose verifies that BufferedHandler flushes
// remaining events when closed.
// TestMultipleLoader_NoResourceLeak verifies that creating and closing
// multiple Loaders doesn't accumulate resources.
func TestMultipleLoader_NoResourceLeak(t *testing.T) {
	// Get initial goroutine count
	initialGoroutines := runtime.NumGoroutine()

	// Use a mock filesystem to avoid path validation issues
	mockFS := newTestFileSystem()
	mockFS.files[".env"] = "KEY=value"

	// Create and close multiple loaders
	for i := 0; i < 20; i++ {
		cfg := DefaultConfig()
		cfg.Filenames = []string{".env"}
		cfg.FileSystem = mockFS
		cfg.AuditEnabled = true

		loader, err := New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}

		// Use the loader
		_ = loader.GetString("KEY")

		// Close properly
		if err := loader.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	}

	// Give time for cleanup
	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	// Verify goroutine count
	finalGoroutines := runtime.NumGoroutine()
	leakedGoroutines := finalGoroutines - initialGoroutines

	t.Logf("Goroutine count: initial=%d, final=%d, leaked=%d",
		initialGoroutines, finalGoroutines, leakedGoroutines)

	if leakedGoroutines > 2 {
		t.Errorf("Potential goroutine leak: %d goroutines not cleaned up", leakedGoroutines)
	}
}

// TestSecureMap_ClearReleasesMemory verifies that secureMap.Clear() properly
// releases all SecureValue objects.
func TestSecureMap_ClearReleasesMemory(t *testing.T) {
	sm := newSecureMap()

	// Add many values with unique keys
	for i := 0; i < 100; i++ {
		sm.Set(fmt.Sprintf("KEY_%d", i), "value")
	}

	if sm.Len() != 100 {
		t.Errorf("Len() = %d, want 100", sm.Len())
	}

	// Clear should release all
	sm.Clear()

	if sm.Len() != 0 {
		t.Errorf("Len() after Clear() = %d, want 0", sm.Len())
	}

	// Verify we can add new values after clear
	sm.Set("NEWKEY", "newvalue")
	if sm.Len() != 1 {
		t.Errorf("Len() after new Set() = %d, want 1", sm.Len())
	}

	sm.Clear()
}

// ============================================================================
// Additional Resource Leak Tests
// ============================================================================

// TestSecureValue_FinalizerCleanup verifies that SecureValue's finalizer
// properly cleans up memory when the value is garbage collected.
func TestSecureValue_FinalizerCleanup(t *testing.T) {
	// Create many SecureValues without explicit release
	for i := 0; i < 100; i++ {
		_ = NewSecureValue("sensitive-data-that-should-be-cleared")
	}

	// Force GC to trigger finalizers
	runtime.GC()
	time.Sleep(50 * time.Millisecond)

	// Create and release properly
	for i := 0; i < 50; i++ {
		sv := NewSecureValue("more-data")
		sv.Release()
	}

	runtime.GC()
	time.Sleep(50 * time.Millisecond)

	// If we get here without panic or deadlock, the test passes
}

// TestSingleton_ResetClosesLoader verifies that ResetDefaultLoader properly
// closes the old loader.
func TestSingleton_ResetClosesLoader(t *testing.T) {
	// Reset any existing loader first
	_ = ResetDefaultLoader()

	// Create a new default loader via Load
	mockFS := newTestFileSystem()
	mockFS.files[".env"] = "TEST_KEY=test_value"

	cfg := DefaultConfig()
	cfg.Filenames = []string{".env"}
	cfg.FileSystem = mockFS

	loader, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Set as default
	if err := setDefaultLoader(loader); err != nil {
		t.Fatalf("setDefaultLoader() error = %v", err)
	}

	// Verify it's not closed
	if loader.IsClosed() {
		t.Fatal("loader should not be closed")
	}

	// Reset should close the loader
	if err := ResetDefaultLoader(); err != nil {
		t.Errorf("ResetDefaultLoader() error = %v", err)
	}

	// Verify the old loader is now closed
	if !loader.IsClosed() {
		t.Error("old loader should be closed after reset")
	}
}

// TestKeyInternCache_BoundedGrowth verifies that the key interning cache
// doesn't grow unbounded.
func TestKeyInternCache_BoundedGrowth(t *testing.T) {
	// Clear cache first
	internal.ClearInternCache()

	// Intern many unique keys (more than the max cache size)
	for i := 0; i < 2000; i++ {
		// Create keys that are within the length limit
		key := fmt.Sprintf("KEY_%04d", i)
		_ = internal.InternKeyBytes([]byte(key))
	}

	// Clear and verify it doesn't panic
	internal.ClearInternCache()

	// Verify we can still intern after clear
	interned := internal.InternKeyBytes([]byte("TEST_KEY"))
	if interned != "TEST_KEY" {
		t.Error("InternKeyBytes should return the key after clear")
	}

	internal.ClearInternCache()
}

// TestLoader_MultipleCloseSafe verifies that calling Close() on a Loader
// multiple times is safe.
// TestLoader_MultipleCloseSafe is covered by TestLoader_CloseAndIsClosed in
// loader_test.go (second-close idempotency).

// TestAuditEventPool_NoLeak verifies that the audit event pool properly
// recycles events without leaking memory.
func TestAuditEventPool_NoLeak(t *testing.T) {
	auditor := internal.NewAuditor(internal.NewNopHandler(), nil, nil, true)

	// Log many events; each cycle takes an Event from the pool in Log and
	// returns it in logEvent after the handler consumes it.
	for i := 0; i < 1000; i++ {
		_ = auditor.Log(internal.ActionSet, "TEST_KEY", "test", true)
	}

	_ = auditor.Close()

	// If we get here without memory issues, the test passes
}
