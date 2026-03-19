package env

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/cybergodev/env/internal"
)

// ============================================================================
// Mock File System for Testing (local version for resource leak tests)
// ============================================================================

type mockFileSystem struct {
	mu      sync.RWMutex
	files   map[string][]byte
	env     map[string]string
	openErr error
	statErr error
}

func newMockFileSystem() *mockFileSystem {
	return &mockFileSystem{
		files: make(map[string][]byte),
		env:   make(map[string]string),
	}
}

func (m *mockFileSystem) AddFile(name string, content []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.files[name] = content
}

func (m *mockFileSystem) Open(name string) (File, error) {
	if m.openErr != nil {
		return nil, m.openErr
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	content, ok := m.files[name]
	if !ok {
		return nil, os.ErrNotExist
	}
	return &mockFile{reader: bytes.NewReader(content)}, nil
}

func (m *mockFileSystem) OpenFile(name string, flag int, perm os.FileMode) (File, error) {
	return m.Open(name)
}

func (m *mockFileSystem) Stat(name string) (os.FileInfo, error) {
	if m.statErr != nil {
		return nil, m.statErr
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	content, ok := m.files[name]
	if !ok {
		return nil, os.ErrNotExist
	}
	return &mockFileInfo{name: name, size: int64(len(content))}, nil
}

func (m *mockFileSystem) MkdirAll(path string, perm os.FileMode) error {
	return nil
}

func (m *mockFileSystem) Remove(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.files, name)
	return nil
}

func (m *mockFileSystem) Rename(oldpath, newpath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if content, ok := m.files[oldpath]; ok {
		m.files[newpath] = content
		delete(m.files, oldpath)
	}
	return nil
}

func (m *mockFileSystem) Getenv(key string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.env[key]
}

func (m *mockFileSystem) Setenv(key, value string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.env[key] = value
	return nil
}

func (m *mockFileSystem) Unsetenv(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.env, key)
	return nil
}

func (m *mockFileSystem) LookupEnv(key string) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	val, ok := m.env[key]
	return val, ok
}

type mockFile struct {
	reader *bytes.Reader
}

func (m *mockFile) Read(p []byte) (n int, err error) {
	return m.reader.Read(p)
}

func (m *mockFile) Write(p []byte) (n int, err error) {
	return 0, io.ErrUnexpectedEOF
}

func (m *mockFile) Close() error {
	return nil
}

func (m *mockFile) Stat() (os.FileInfo, error) {
	return &mockFileInfo{size: m.reader.Size()}, nil
}

func (m *mockFile) Sync() error {
	return nil
}

type mockFileInfo struct {
	name string
	size int64
}

func (m *mockFileInfo) Name() string       { return m.name }
func (m *mockFileInfo) Size() int64        { return m.size }
func (m *mockFileInfo) Mode() os.FileMode  { return 0644 }
func (m *mockFileInfo) ModTime() time.Time { return time.Time{} }
func (m *mockFileInfo) IsDir() bool        { return false }
func (m *mockFileInfo) Sys() interface{}   { return nil }

// ============================================================================
// Resource Leak Tests
// ============================================================================

// TestBufferedHandler_NoGoroutineLeak verifies that BufferedHandler's background
// goroutine is properly stopped when Close() is called.
func TestBufferedHandler_NoGoroutineLeak(t *testing.T) {
	// Get initial goroutine count
	initialGoroutines := runtime.NumGoroutine()

	// Create multiple buffered handlers with flush intervals
	// Each starts a background goroutine
	handlers := make([]*internal.BufferedHandler, 10)
	for i := 0; i < 10; i++ {
		handlers[i] = internal.NewBufferedHandler(internal.BufferedHandlerConfig{
			Handler:       internal.NewNopHandler(),
			BufferSize:    10,
			FlushInterval: 100 * time.Millisecond,
		})
	}

	// Give goroutines time to start
	time.Sleep(50 * time.Millisecond)

	// Verify goroutines were created
	afterCreate := runtime.NumGoroutine()
	if afterCreate <= initialGoroutines {
		t.Logf("Warning: expected goroutine increase, initial=%d, after=%d",
			initialGoroutines, afterCreate)
	}

	// Close all handlers
	for _, h := range handlers {
		if err := h.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	}

	// Give goroutines time to stop
	time.Sleep(100 * time.Millisecond)

	// Force garbage collection to clean up any pending finalizers
	runtime.GC()
	time.Sleep(50 * time.Millisecond)

	// Verify goroutines were cleaned up
	finalGoroutines := runtime.NumGoroutine()

	// Allow for some variance due to test framework goroutines
	// but there should be significant reduction
	leakedGoroutines := finalGoroutines - initialGoroutines

	t.Logf("Goroutine count: initial=%d, after_create=%d, final=%d, leaked=%d",
		initialGoroutines, afterCreate, finalGoroutines, leakedGoroutines)

	// We expect at most 2 extra goroutines from test infrastructure
	if leakedGoroutines > 2 {
		t.Errorf("Potential goroutine leak: %d goroutines not cleaned up", leakedGoroutines)
	}
}

// TestCloseableChannelHandler_ReceiverUnblocked verifies that receivers
// are properly unblocked when the handler is closed.
func TestCloseableChannelHandler_ReceiverUnblocked(t *testing.T) {
	handler := internal.NewCloseableChannelHandler(0) // Unbuffered

	receiverDone := make(chan struct{})
	go func() {
		defer close(receiverDone)
		ch := handler.Channel()
		// This will block until handler is closed
		for range ch {
			// Consume events
		}
	}()

	// Give receiver time to start blocking
	time.Sleep(50 * time.Millisecond)

	// Close the handler - this should unblock the receiver
	if err := handler.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}

	// Wait for receiver to finish
	select {
	case <-receiverDone:
		// Success - receiver was unblocked
	case <-time.After(time.Second):
		t.Error("receiver should have been unblocked by Close()")
	}
}

// TestLoader_ResourceCleanup verifies that Loader properly cleans up resources
// when Close() is called.
func TestLoader_ResourceCleanup(t *testing.T) {
	// Use a mock filesystem to avoid path validation issues
	mockFS := newMockFileSystem()
	mockFS.AddFile(".env", []byte("KEY1=value1\nKEY2=value2"))

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

// TestComponentFactory_CloseIdempotent verifies that ComponentFactory.Close()
// can be called multiple times safely.
func TestComponentFactory_CloseIdempotent(t *testing.T) {
	cfg := DefaultConfig()
	factory := cfg.buildComponentFactory()

	// Close multiple times
	for i := 0; i < 5; i++ {
		if err := factory.Close(); err != nil {
			t.Errorf("Close() #%d error = %v", i+1, err)
		}
	}

	if !factory.IsClosed() {
		t.Error("IsClosed() should return true")
	}
}

// TestBufferedHandler_FlushOnClose verifies that BufferedHandler flushes
// remaining events when closed.
func TestBufferedHandler_FlushOnClose(t *testing.T) {
	ch := make(chan internal.Event, 100)
	underlying := internal.NewChannelHandler(ch)

	handler := internal.NewBufferedHandler(internal.BufferedHandlerConfig{
		Handler:       underlying,
		BufferSize:    10,
		FlushInterval: 0, // Disable auto-flush
	})

	// Log events without flushing
	for i := 0; i < 5; i++ {
		_ = handler.Log(internal.Event{Action: internal.ActionSet})
	}

	// Close should flush remaining events
	if err := handler.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}

	// Count received events
	count := 0
	for {
		select {
		case <-ch:
			count++
		default:
			goto done
		}
	}
done:

	if count != 5 {
		t.Errorf("expected 5 events flushed on close, got %d", count)
	}
}

// TestMultipleLoader_NoResourceLeak verifies that creating and closing
// multiple Loaders doesn't accumulate resources.
func TestMultipleLoader_NoResourceLeak(t *testing.T) {
	// Get initial goroutine count
	initialGoroutines := runtime.NumGoroutine()

	// Use a mock filesystem to avoid path validation issues
	mockFS := newMockFileSystem()
	mockFS.AddFile(".env", []byte("KEY=value"))

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
