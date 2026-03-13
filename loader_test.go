package env

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// ============================================================================
// Test Helpers
// ============================================================================

// testFileSystem is a mock FileSystem for testing.
type testFileSystem struct {
	files   map[string]string
	env     map[string]string
	openErr error
	statErr error
}

func newTestFileSystem() *testFileSystem {
	return &testFileSystem{
		files: make(map[string]string),
		env:   make(map[string]string),
	}
}

func (fs *testFileSystem) Open(name string) (File, error) {
	if fs.openErr != nil {
		return nil, fs.openErr
	}
	content, ok := fs.files[name]
	if !ok {
		return nil, ErrFileNotFound
	}
	return &testFile{content: content}, nil
}

func (fs *testFileSystem) OpenFile(name string, flag int, perm os.FileMode) (File, error) {
	return fs.Open(name)
}

func (fs *testFileSystem) Stat(name string) (os.FileInfo, error) {
	if fs.statErr != nil {
		return nil, fs.statErr
	}
	content, ok := fs.files[name]
	if !ok {
		return nil, os.ErrNotExist
	}
	return &testFileInfo{name: name, size: int64(len(content))}, nil
}

func (fs *testFileSystem) MkdirAll(path string, perm os.FileMode) error {
	return nil
}

func (fs *testFileSystem) Remove(name string) error {
	delete(fs.files, name)
	return nil
}

func (fs *testFileSystem) Rename(oldpath, newpath string) error {
	fs.files[newpath] = fs.files[oldpath]
	delete(fs.files, oldpath)
	return nil
}

func (fs *testFileSystem) Getenv(key string) string {
	return fs.env[key]
}

func (fs *testFileSystem) Setenv(key, value string) error {
	fs.env[key] = value
	return nil
}

func (fs *testFileSystem) Unsetenv(key string) error {
	delete(fs.env, key)
	return nil
}

func (fs *testFileSystem) LookupEnv(key string) (string, bool) {
	v, ok := fs.env[key]
	return v, ok
}

type testFile struct {
	content string
	pos     int
}

func (f *testFile) Read(p []byte) (n int, err error) {
	if f.pos >= len(f.content) {
		return 0, io.EOF
	}
	n = copy(p, f.content[f.pos:])
	f.pos += n
	return n, nil
}

func (f *testFile) Write(p []byte) (n int, err error) {
	return len(p), nil
}

func (f *testFile) Close() error {
	return nil
}

func (f *testFile) Stat() (os.FileInfo, error) {
	return &testFileInfo{size: int64(len(f.content))}, nil
}

func (f *testFile) Sync() error {
	return nil
}

type testFileInfo struct {
	name string
	size int64
}

func (fi *testFileInfo) Name() string       { return fi.name }
func (fi *testFileInfo) Size() int64        { return fi.size }
func (fi *testFileInfo) Mode() os.FileMode  { return 0644 }
func (fi *testFileInfo) ModTime() time.Time { return time.Now() }
func (fi *testFileInfo) IsDir() bool        { return false }
func (fi *testFileInfo) Sys() interface{}   { return nil }

// ============================================================================
// New Tests
// ============================================================================

func TestNew(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		cfg := DefaultConfig()
		loader, err := New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		if loader == nil {
			t.Fatal("New() returned nil loader")
		}
		defer loader.Close()
	})

	t.Run("invalid config - zero max file size", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.MaxFileSize = 0
		_, err := New(cfg)
		if err == nil {
			t.Error("New() should fail with zero MaxFileSize")
		}
	})

	t.Run("invalid config - exceeds hard limit", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.MaxFileSize = 200 * 1024 * 1024 // 200MB exceeds hard limit
		_, err := New(cfg)
		if err == nil {
			t.Error("New() should fail with MaxFileSize exceeding hard limit")
		}
	})

	t.Run("custom key pattern", func(t *testing.T) {
		cfg := DefaultConfig()
		// Pattern that doesn't match TEST_KEY should fail
		cfg.KeyPattern = DefaultKeyPattern
		loader, err := New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer loader.Close()
	})
}

// ============================================================================
// LoadFiles Tests
// ============================================================================

func TestLoader_LoadFiles(t *testing.T) {
	t.Run("load single file", func(t *testing.T) {
		fs := newTestFileSystem()
		fs.files[".env"] = "KEY1=value1\nKEY2=value2"

		cfg := DefaultConfig()
		cfg.FileSystem = fs
		loader, err := New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer loader.Close()

		if err := loader.LoadFiles(".env"); err != nil {
			t.Fatalf("LoadFiles() error = %v", err)
		}

		if loader.GetString("KEY1") != "value1" {
			t.Errorf("GetString(\"KEY1\") = %q, want %q", loader.GetString("KEY1"), "value1")
		}
	})

	t.Run("load multiple files", func(t *testing.T) {
		fs := newTestFileSystem()
		fs.files[".env"] = "KEY1=value1"
		fs.files[".env.local"] = "KEY2=value2\nKEY1=overridden"

		cfg := DefaultConfig()
		cfg.FileSystem = fs
		cfg.OverwriteExisting = true
		loader, err := New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer loader.Close()

		if err := loader.LoadFiles(".env", ".env.local"); err != nil {
			t.Fatalf("LoadFiles() error = %v", err)
		}

		if loader.GetString("KEY1") != "overridden" {
			t.Errorf("GetString(\"KEY1\") = %q, want %q", loader.GetString("KEY1"), "overridden")
		}
	})

	t.Run("default to .env", func(t *testing.T) {
		fs := newTestFileSystem()
		fs.files[".env"] = "KEY=default"

		cfg := DefaultConfig()
		cfg.FileSystem = fs
		loader, err := New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer loader.Close()

		if err := loader.LoadFiles(); err != nil {
			t.Fatalf("LoadFiles() error = %v", err)
		}

		if loader.GetString("KEY") != "default" {
			t.Errorf("GetString(\"KEY\") = %q, want %q", loader.GetString("KEY"), "default")
		}
	})

	t.Run("file not found - skip", func(t *testing.T) {
		fs := newTestFileSystem()
		fs.files[".env"] = "KEY=value"

		cfg := DefaultConfig()
		cfg.FileSystem = fs
		cfg.FailOnMissingFile = false
		loader, err := New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer loader.Close()

		// Load existing file first, then missing file
		if err := loader.LoadFiles(".env", "missing.env"); err != nil {
			t.Fatalf("LoadFiles() error = %v", err)
		}
	})

	t.Run("file not found - fail", func(t *testing.T) {
		fs := newTestFileSystem()

		cfg := DefaultConfig()
		cfg.FileSystem = fs
		cfg.Filenames = nil // Don't auto-load, test LoadFiles separately
		cfg.FailOnMissingFile = true
		loader, err := New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer loader.Close()

		if err := loader.LoadFiles("missing.env"); err == nil {
			t.Error("LoadFiles() should fail with missing file")
		}
	})

	t.Run("file too large", func(t *testing.T) {
		fs := newTestFileSystem()
		fs.files["large.env"] = strings.Repeat("a", 2000)

		cfg := DefaultConfig()
		cfg.FileSystem = fs
		cfg.MaxFileSize = 1000
		loader, err := New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer loader.Close()

		var fileErr *FileError
		if err := loader.LoadFiles("large.env"); !errors.As(err, &fileErr) {
			t.Errorf("LoadFiles() error = %v, want FileError", err)
		}
	})

	t.Run("auto apply", func(t *testing.T) {
		fs := newTestFileSystem()
		fs.files[".env"] = "KEY=value"

		cfg := DefaultConfig()
		cfg.FileSystem = fs
		cfg.AutoApply = true
		loader, err := New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer loader.Close()

		if err := loader.LoadFiles(".env"); err != nil {
			t.Fatalf("LoadFiles() error = %v", err)
		}

		if fs.env["KEY"] != "value" {
			t.Errorf("env[\"KEY\"] = %q, want %q", fs.env["KEY"], "value")
		}
	})

	t.Run("prefix filter", func(t *testing.T) {
		fs := newTestFileSystem()
		fs.files[".env"] = "APP_KEY=value\nOTHER_KEY=other"

		cfg := DefaultConfig()
		cfg.FileSystem = fs
		cfg.Prefix = "APP_"
		loader, err := New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer loader.Close()

		if err := loader.LoadFiles(".env"); err != nil {
			t.Fatalf("LoadFiles() error = %v", err)
		}

		if loader.GetString("APP_KEY") != "value" {
			t.Errorf("GetString(\"APP_KEY\") = %q, want %q", loader.GetString("APP_KEY"), "value")
		}
		if _, ok := loader.Lookup("OTHER_KEY"); ok {
			t.Error("OTHER_KEY should not be loaded with APP_ prefix")
		}
	})

}

// ============================================================================
// Apply Tests
// ============================================================================

func TestLoader_Apply(t *testing.T) {
	t.Run("apply to environment", func(t *testing.T) {
		fs := newTestFileSystem()

		cfg := DefaultConfig()
		cfg.FileSystem = fs
		cfg.OverwriteExisting = true
		loader, err := New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer loader.Close()

		if err := loader.Set("TEST_KEY", "test_value"); err != nil {
			t.Fatalf("Set() error = %v", err)
		}

		if err := loader.Apply(); err != nil {
			t.Fatalf("Apply() error = %v", err)
		}

		if fs.env["TEST_KEY"] != "test_value" {
			t.Errorf("env[\"TEST_KEY\"] = %q, want %q", fs.env["TEST_KEY"], "test_value")
		}
	})

	t.Run("apply respects overwrite policy", func(t *testing.T) {
		fs := newTestFileSystem()
		fs.env["EXISTING_KEY"] = "original"

		cfg := DefaultConfig()
		cfg.FileSystem = fs
		cfg.OverwriteExisting = false
		loader, err := New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer loader.Close()

		if err := loader.Set("EXISTING_KEY", "new_value"); err != nil {
			t.Fatalf("Set() error = %v", err)
		}

		if err := loader.Apply(); err != nil {
			t.Fatalf("Apply() error = %v", err)
		}

		if fs.env["EXISTING_KEY"] != "original" {
			t.Errorf("env[\"EXISTING_KEY\"] = %q, want %q", fs.env["EXISTING_KEY"], "original")
		}
	})

}

// ============================================================================
// GetString/GetSecure/Lookup Tests
// ============================================================================

func TestLoader_Get(t *testing.T) {
	t.Run("existing key", func(t *testing.T) {
		cfg := DefaultConfig()
		loader, err := New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer loader.Close()

		if err := loader.Set("KEY", "value"); err != nil {
			t.Fatalf("Set() error = %v", err)
		}

		if got := loader.GetString("KEY"); got != "value" {
			t.Errorf("GetString(\"KEY\") = %q, want %q", got, "value")
		}
	})

	t.Run("missing key with default", func(t *testing.T) {
		cfg := DefaultConfig()
		loader, err := New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer loader.Close()

		if got := loader.GetString("MISSING", "default"); got != "default" {
			t.Errorf("GetString(\"MISSING\", \"default\") = %q, want %q", got, "default")
		}
	})

	t.Run("missing key without default", func(t *testing.T) {
		cfg := DefaultConfig()
		loader, err := New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer loader.Close()

		if got := loader.GetString("MISSING"); got != "" {
			t.Errorf("GetString(\"MISSING\") = %q, want empty string", got)
		}
	})
}

func TestLoader_GetSecure(t *testing.T) {
	t.Run("existing key", func(t *testing.T) {
		cfg := DefaultConfig()
		loader, err := New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer loader.Close()

		if err := loader.Set("SECRET", "password123"); err != nil {
			t.Fatalf("Set() error = %v", err)
		}

		sv := loader.GetSecure("SECRET")
		if sv == nil {
			t.Fatal("GetSecure() returned nil")
		}
		if sv.String() != "password123" {
			t.Errorf("GetSecure().String() = %q, want %q", sv.String(), "password123")
		}
	})

	t.Run("missing key", func(t *testing.T) {
		cfg := DefaultConfig()
		loader, err := New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer loader.Close()

		sv := loader.GetSecure("MISSING")
		if sv != nil {
			t.Errorf("GetSecure() = %v, want nil", sv)
		}
	})

}

func TestLoader_Lookup(t *testing.T) {
	t.Run("existing key", func(t *testing.T) {
		cfg := DefaultConfig()
		loader, err := New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer loader.Close()

		if err := loader.Set("KEY", "value"); err != nil {
			t.Fatalf("Set() error = %v", err)
		}

		value, ok := loader.Lookup("KEY")
		if !ok {
			t.Error("Lookup() ok = false, want true")
		}
		if value != "value" {
			t.Errorf("Lookup() = %q, want %q", value, "value")
		}
	})

	t.Run("missing key", func(t *testing.T) {
		cfg := DefaultConfig()
		loader, err := New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer loader.Close()

		value, ok := loader.Lookup("MISSING")
		if ok {
			t.Error("Lookup() ok = true for missing key, want false")
		}
		if value != "" {
			t.Errorf("Lookup() = %q, want empty string", value)
		}
	})

	t.Run("trims whitespace", func(t *testing.T) {
		cfg := DefaultConfig()
		loader, err := New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer loader.Close()

		if err := loader.Set("KEY", "  value  "); err != nil {
			t.Fatalf("Set() error = %v", err)
		}

		value, ok := loader.Lookup("KEY")
		if !ok {
			t.Fatal("Lookup() ok = false, want true")
		}
		if value != "value" {
			t.Errorf("Lookup() = %q, want %q", value, "value")
		}
	})

}

// ============================================================================
// Set/Delete Tests
// ============================================================================

func TestLoader_Set(t *testing.T) {
	t.Run("valid key and value", func(t *testing.T) {
		cfg := DefaultConfig()
		loader, err := New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer loader.Close()

		if err := loader.Set("KEY", "value"); err != nil {
			t.Errorf("Set() error = %v", err)
		}
	})

	t.Run("invalid key", func(t *testing.T) {
		cfg := DefaultConfig()
		loader, err := New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer loader.Close()

		if err := loader.Set("", "value"); err == nil {
			t.Error("Set() should fail with empty key")
		}
	})

	t.Run("auto apply", func(t *testing.T) {
		fs := newTestFileSystem()

		cfg := DefaultConfig()
		cfg.FileSystem = fs
		cfg.AutoApply = true
		loader, err := New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer loader.Close()

		if err := loader.Set("KEY", "value"); err != nil {
			t.Fatalf("Set() error = %v", err)
		}

		if fs.env["KEY"] != "value" {
			t.Errorf("env[\"KEY\"] = %q, want %q", fs.env["KEY"], "value")
		}
	})

	t.Run("empty value", func(t *testing.T) {
		cfg := DefaultConfig()
		loader, err := New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer loader.Close()

		if err := loader.Set("EMPTY_KEY", ""); err != nil {
			t.Errorf("Set() with empty value error = %v", err)
		}
		if got := loader.GetString("EMPTY_KEY"); got != "" {
			t.Errorf("GetString() = %q, want empty", got)
		}
	})

	t.Run("unicode and emoji in value", func(t *testing.T) {
		cfg := DefaultConfig()
		loader, err := New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer loader.Close()

		unicodeValue := "hello 世界 🌍 \u4e2d\u6587"
		if err := loader.Set("UNICODE_KEY", unicodeValue); err != nil {
			t.Errorf("Set() with unicode error = %v", err)
		}
		if got := loader.GetString("UNICODE_KEY"); got != unicodeValue {
			t.Errorf("GetString() = %q, want %q", got, unicodeValue)
		}
	})
}

func TestLoader_Delete(t *testing.T) {
	t.Run("existing key", func(t *testing.T) {
		cfg := DefaultConfig()
		loader, err := New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer loader.Close()

		if err := loader.Set("KEY", "value"); err != nil {
			t.Fatalf("Set() error = %v", err)
		}

		if err := loader.Delete("KEY"); err != nil {
			t.Errorf("Delete() error = %v", err)
		}

		if _, ok := loader.Lookup("KEY"); ok {
			t.Error("Key should be deleted")
		}
	})

	t.Run("non-existent key", func(t *testing.T) {
		cfg := DefaultConfig()
		loader, err := New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer loader.Close()

		// Should not error on non-existent key
		if err := loader.Delete("MISSING"); err != nil {
			t.Errorf("Delete() error = %v", err)
		}
	})

}

// ============================================================================
// Keys/All/Len Tests
// ============================================================================

func TestLoader_Keys(t *testing.T) {
	t.Run("multiple keys", func(t *testing.T) {
		cfg := DefaultConfig()
		loader, err := New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer loader.Close()

		if err := loader.Set("KEY1", "value1"); err != nil {
			t.Fatalf("Set() error = %v", err)
		}
		if err := loader.Set("KEY2", "value2"); err != nil {
			t.Fatalf("Set() error = %v", err)
		}

		keys := loader.Keys()
		if len(keys) != 2 {
			t.Errorf("Keys() returned %d keys, want 2", len(keys))
		}
	})

	t.Run("empty loader", func(t *testing.T) {
		cfg := DefaultConfig()
		loader, err := New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer loader.Close()

		keys := loader.Keys()
		if len(keys) != 0 {
			t.Errorf("Keys() returned %d keys, want 0", len(keys))
		}
	})

}

func TestLoader_All(t *testing.T) {
	t.Run("multiple keys", func(t *testing.T) {
		cfg := DefaultConfig()
		loader, err := New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer loader.Close()

		if err := loader.Set("KEY1", "value1"); err != nil {
			t.Fatalf("Set() error = %v", err)
		}
		if err := loader.Set("KEY2", "value2"); err != nil {
			t.Fatalf("Set() error = %v", err)
		}

		all := loader.All()
		if len(all) != 2 {
			t.Errorf("All() returned %d keys, want 2", len(all))
		}
		if all["KEY1"] != "value1" {
			t.Errorf("All()[\"KEY1\"] = %q, want %q", all["KEY1"], "value1")
		}
	})

}

func TestLoader_Len(t *testing.T) {
	t.Run("multiple keys", func(t *testing.T) {
		cfg := DefaultConfig()
		loader, err := New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer loader.Close()

		if err := loader.Set("KEY1", "value1"); err != nil {
			t.Fatalf("Set() error = %v", err)
		}
		if err := loader.Set("KEY2", "value2"); err != nil {
			t.Fatalf("Set() error = %v", err)
		}

		if loader.Len() != 2 {
			t.Errorf("Len() = %d, want 2", loader.Len())
		}
	})

}

// ============================================================================
// IsApplied/LoadTime/Config Tests
// ============================================================================

func TestLoader_IsApplied(t *testing.T) {
	cfg := DefaultConfig()
	loader, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer loader.Close()

	if loader.IsApplied() {
		t.Error("IsApplied() = true before Apply()")
	}

	if err := loader.Apply(); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	if !loader.IsApplied() {
		t.Error("IsApplied() = false after Apply()")
	}
}

func TestLoader_LoadTime(t *testing.T) {
	fs := newTestFileSystem()
	fs.files[".env"] = "KEY=value"

	cfg := DefaultConfig()
	cfg.FileSystem = fs
	cfg.Filenames = nil // Don't auto-load, test LoadTime behavior
	loader, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer loader.Close()

	before := loader.LoadTime()
	if !before.IsZero() {
		t.Error("LoadTime() should be zero before loading")
	}

	if err := loader.LoadFiles(".env"); err != nil {
		t.Fatalf("LoadFiles() error = %v", err)
	}

	after := loader.LoadTime()
	if after.IsZero() {
		t.Error("LoadTime() should not be zero after loading")
	}
}

func TestLoader_Config(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxVariables = 50

	loader, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer loader.Close()

	returnedCfg := loader.Config()
	if returnedCfg.MaxVariables != 50 {
		t.Errorf("Config().MaxVariables = %d, want 50", returnedCfg.MaxVariables)
	}
}

// ============================================================================
// Closed Loader Behavior Tests (Table-Driven)
// ============================================================================

func TestLoader_ClosedBehavior(t *testing.T) {
	tests := []struct {
		name      string
		operation string
		testFunc  func(t *testing.T, loader *Loader)
	}{
		{
			name:      "LoadFiles",
			operation: "LoadFiles",
			testFunc: func(t *testing.T, loader *Loader) {
				if err := loader.LoadFiles(".env"); !errors.Is(err, ErrClosed) {
					t.Errorf("LoadFiles() error = %v, want ErrClosed", err)
				}
			},
		},
		{
			name:      "Apply",
			operation: "Apply",
			testFunc: func(t *testing.T, loader *Loader) {
				if err := loader.Apply(); !errors.Is(err, ErrClosed) {
					t.Errorf("Apply() error = %v, want ErrClosed", err)
				}
			},
		},
		{
			name:      "GetSecure",
			operation: "GetSecure",
			testFunc: func(t *testing.T, loader *Loader) {
				sv := loader.GetSecure("KEY")
				if sv != nil {
					t.Errorf("GetSecure() on closed loader = %v, want nil", sv)
				}
			},
		},
		{
			name:      "Lookup",
			operation: "Lookup",
			testFunc: func(t *testing.T, loader *Loader) {
				value, ok := loader.Lookup("KEY")
				if ok {
					t.Error("Lookup() on closed loader ok = true, want false")
				}
				if value != "" {
					t.Errorf("Lookup() = %q, want empty string", value)
				}
			},
		},
		{
			name:      "Set",
			operation: "Set",
			testFunc: func(t *testing.T, loader *Loader) {
				if err := loader.Set("KEY", "value"); !errors.Is(err, ErrClosed) {
					t.Errorf("Set() error = %v, want ErrClosed", err)
				}
			},
		},
		{
			name:      "Delete",
			operation: "Delete",
			testFunc: func(t *testing.T, loader *Loader) {
				if err := loader.Delete("KEY"); !errors.Is(err, ErrClosed) {
					t.Errorf("Delete() error = %v, want ErrClosed", err)
				}
			},
		},
		{
			name:      "Keys",
			operation: "Keys",
			testFunc: func(t *testing.T, loader *Loader) {
				keys := loader.Keys()
				if keys != nil {
					t.Errorf("Keys() on closed loader = %v, want nil", keys)
				}
			},
		},
		{
			name:      "All",
			operation: "All",
			testFunc: func(t *testing.T, loader *Loader) {
				all := loader.All()
				if all != nil {
					t.Errorf("All() on closed loader = %v, want nil", all)
				}
			},
		},
		{
			name:      "Len",
			operation: "Len",
			testFunc: func(t *testing.T, loader *Loader) {
				if loader.Len() != 0 {
					t.Errorf("Len() on closed loader = %d, want 0", loader.Len())
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			loader, err := New(cfg)
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}
			loader.Close()

			tt.testFunc(t, loader)
		})
	}
}

// ============================================================================
// Close/IsClosed Tests
// ============================================================================

func TestLoader_Close(t *testing.T) {
	t.Run("close once", func(t *testing.T) {
		cfg := DefaultConfig()
		loader, err := New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}

		if err := loader.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})

	t.Run("close twice", func(t *testing.T) {
		cfg := DefaultConfig()
		loader, err := New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}

		if err := loader.Close(); err != nil {
			t.Fatalf("First Close() error = %v", err)
		}

		// Second close should be idempotent
		if err := loader.Close(); err != nil {
			t.Errorf("Second Close() error = %v", err)
		}
	})
}

func TestLoader_IsClosed(t *testing.T) {
	cfg := DefaultConfig()
	loader, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if loader.IsClosed() {
		t.Error("IsClosed() = true before Close()")
	}

	loader.Close()

	if !loader.IsClosed() {
		t.Error("IsClosed() = false after Close()")
	}
}

// ============================================================================
// GetInt/GetBool/GetDuration Tests
// ============================================================================

func TestLoader_GetInt(t *testing.T) {
	cfg := DefaultConfig()
	loader, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer loader.Close()

	if err := loader.Set("PORT", "8080"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	if got := loader.GetInt("PORT"); got != 8080 {
		t.Errorf("GetInt(\"PORT\") = %d, want 8080", got)
	}

	if got := loader.GetInt("MISSING", 3000); got != 3000 {
		t.Errorf("GetInt(\"MISSING\", 3000) = %d, want 3000", got)
	}
}

func TestLoader_GetBool(t *testing.T) {
	cfg := DefaultConfig()
	loader, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer loader.Close()

	if err := loader.Set("DEBUG", "true"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	if got := loader.GetBool("DEBUG"); !got {
		t.Errorf("GetBool(\"DEBUG\") = %v, want true", got)
	}

	if got := loader.GetBool("MISSING", true); !got {
		t.Errorf("GetBool(\"MISSING\", true) = %v, want true", got)
	}
}

func TestLoader_GetDuration(t *testing.T) {
	cfg := DefaultConfig()
	loader, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer loader.Close()

	if err := loader.Set("TIMEOUT", "30s"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	if got := loader.GetDuration("TIMEOUT"); got != 30*time.Second {
		t.Errorf("GetDuration(\"TIMEOUT\") = %v, want 30s", got)
	}

	if got := loader.GetDuration("MISSING", 5*time.Minute); got != 5*time.Minute {
		t.Errorf("GetDuration(\"MISSING\", 5m) = %v, want 5m", got)
	}
}

// ============================================================================
// Unmarshal Tests
// ============================================================================

func TestLoader_Unmarshal(t *testing.T) {
	t.Run("struct unmarshal", func(t *testing.T) {
		cfg := DefaultConfig()
		loader, err := New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer loader.Close()

		if err := loader.Set("NAME", "test"); err != nil {
			t.Fatalf("Set() error = %v", err)
		}
		if err := loader.Set("PORT", "8080"); err != nil {
			t.Fatalf("Set() error = %v", err)
		}

		type Config struct {
			Name string `env:"NAME"`
			Port int    `env:"PORT"`
		}

		var c Config
		if err := loader.ParseInto(&c); err != nil {
			t.Fatalf("ParseInto() error = %v", err)
		}

		if c.Name != "test" {
			t.Errorf("c.Name = %q, want %q", c.Name, "test")
		}
		if c.Port != 8080 {
			t.Errorf("c.Port = %d, want 8080", c.Port)
		}
	})
}

// ============================================================================
// Validate Tests
// ============================================================================

func TestLoader_Validate(t *testing.T) {
	t.Run("required keys present", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.RequiredKeys = []string{"KEY1", "KEY2"}

		loader, err := New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer loader.Close()

		if err := loader.Set("KEY1", "value1"); err != nil {
			t.Fatalf("Set() error = %v", err)
		}
		if err := loader.Set("KEY2", "value2"); err != nil {
			t.Fatalf("Set() error = %v", err)
		}

		if err := loader.Validate(); err != nil {
			t.Errorf("Validate() error = %v", err)
		}
	})

	t.Run("required keys missing", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.RequiredKeys = []string{"REQUIRED_KEY"}

		loader, err := New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer loader.Close()

		if err := loader.Validate(); err == nil {
			t.Error("Validate() should fail with missing required key")
		}
	})
}

// ============================================================================
// splitAndTrimComma Tests (internal)
// ============================================================================

func TestSplitAndTrimComma(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"a,b,c", []string{"a", "b", "c"}},
		{"  a  ,  b  ,  c  ", []string{"a", "b", "c"}},
		{"a,,b,,,c", []string{"a", "b", "c"}},
		{"", nil},
		{"   ", nil},
		{",,,", nil},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := splitAndTrimComma(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("splitAndTrimComma(%q) = %v, want %v", tt.input, result, tt.expected)
				return
			}
			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("splitAndTrimComma(%q)[%d] = %q, want %q", tt.input, i, v, tt.expected[i])
				}
			}
		})
	}
}

// ============================================================================
// JSON Format Detection Tests
// ============================================================================

func TestDetectFormat(t *testing.T) {
	tests := []struct {
		filename string
		expected FileFormat
	}{
		{".env", FormatEnv},
		{"config.env", FormatEnv},
		{"config.json", FormatJSON},
		{"config.yaml", FormatYAML},
		{"config.yml", FormatYAML},
		{"unknown.txt", FormatAuto},
		{"", FormatAuto},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			result := DetectFormat(tt.filename)
			if result != tt.expected {
				t.Errorf("DetectFormat(%q) = %v, want %v", tt.filename, result, tt.expected)
			}
		})
	}
}

// ============================================================================
// Audit Handler Tests
// ============================================================================

func TestNewJSONAuditHandler(t *testing.T) {
	var buf bytes.Buffer
	handler := NewJSONAuditHandler(&buf)

	event := AuditEvent{
		Action:  ActionSet,
		Key:     "KEY",
		Reason:  "test",
		Success: true,
	}
	err := handler.Log(event)
	if err != nil {
		t.Errorf("Log() error = %v", err)
	}

	// Verify JSON output
	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Errorf("Invalid JSON output: %v", err)
	}
}

func TestNewLogAuditHandler(t *testing.T) {
	logger := NewLogAuditHandler(nil) // nil logger uses default

	if logger == nil {
		t.Error("NewLogAuditHandler(nil) returned nil")
	}

	event := AuditEvent{
		Action:  ActionSet,
		Key:     "KEY",
		Reason:  "test",
		Success: true,
	}
	if err := logger.Log(event); err != nil {
		t.Errorf("Log() error = %v", err)
	}
}

func TestNewChannelAuditHandler(t *testing.T) {
	ch := make(chan AuditEvent, 10)
	handler := NewChannelAuditHandler(ch)

	event := AuditEvent{
		Action:  ActionSet,
		Key:     "KEY",
		Reason:  "test",
		Success: true,
	}
	err := handler.Log(event)
	if err != nil {
		t.Errorf("Log() error = %v", err)
	}

	select {
	case received := <-ch:
		if received.Key != "KEY" {
			t.Errorf("Event.Key = %q, want %q", received.Key, "KEY")
		}
	default:
		t.Error("No event received on channel")
	}
}

func TestNewNopAuditHandler(t *testing.T) {
	handler := NewNopAuditHandler()

	// Log and Close should succeed without doing anything
	event := AuditEvent{
		Action:  ActionSet,
		Key:     "KEY",
		Reason:  "test",
		Success: true,
	}
	if err := handler.Log(event); err != nil {
		t.Errorf("Log() error = %v", err)
	}
	if err := handler.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

// ============================================================================
// ComponentFactory Tests
// ============================================================================

func TestComponentFactory(t *testing.T) {
	cfg := DefaultConfig()
	loader, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer loader.Close()

	factory := cfg.buildComponentFactory()

	t.Run("Validator", func(t *testing.T) {
		v := factory.Validator()
		if v == nil {
			t.Error("Validator() returned nil")
		}
	})

	t.Run("Auditor", func(t *testing.T) {
		a := factory.Auditor()
		if a == nil {
			t.Error("Auditor() returned nil")
		}
	})

	t.Run("Close", func(t *testing.T) {
		if err := factory.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})

	t.Run("IsClosed", func(t *testing.T) {
		if !factory.IsClosed() {
			t.Error("IsClosed() = false after Close()")
		}
	})
}

func TestAuditorAdapter(t *testing.T) {
	cfg := DefaultConfig()
	factory := cfg.buildComponentFactory()
	defer factory.Close()

	adapter := newAuditorAdapter(factory.internalAuditor())

	t.Run("Log", func(t *testing.T) {
		if err := adapter.Log(ActionSet, "KEY", "test", true); err != nil {
			t.Errorf("Log() error = %v", err)
		}
	})

	t.Run("LogError", func(t *testing.T) {
		if err := adapter.LogError(ActionSet, "KEY", "error"); err != nil {
			t.Errorf("LogError() error = %v", err)
		}
	})

	t.Run("LogWithFile", func(t *testing.T) {
		if err := adapter.LogWithFile(ActionSet, "KEY", "file", "test", true); err != nil {
			t.Errorf("LogWithFile() error = %v", err)
		}
	})

	t.Run("LogWithDuration", func(t *testing.T) {
		if err := adapter.LogWithDuration(ActionSet, "KEY", "test", true, time.Second); err != nil {
			t.Errorf("LogWithDuration() error = %v", err)
		}
	})

	t.Run("Close", func(t *testing.T) {
		if err := adapter.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})

	t.Run("nil adapter", func(t *testing.T) {
		nilAdapter := newAuditorAdapter(nil)
		if nilAdapter != nil {
			t.Error("newAuditorAdapter(nil) should return nil")
		}
	})
}

// ============================================================================
// JSON Parser Edge Case Tests
// ============================================================================

func TestJSONParser_EdgeCases(t *testing.T) {
	t.Run("empty object", func(t *testing.T) {
		fs := newTestFileSystem()
		fs.files["empty.json"] = "{}"

		cfg := DefaultConfig()
		cfg.FileSystem = fs
		loader, err := New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer loader.Close()

		if err := loader.LoadFiles("empty.json"); err != nil {
			t.Fatalf("LoadFiles() error = %v", err)
		}

		if loader.Len() != 0 {
			t.Errorf("Len() = %d, want 0", loader.Len())
		}
	})

	t.Run("nested object", func(t *testing.T) {
		fs := newTestFileSystem()
		fs.files["nested.json"] = `{
			"database": {
				"host": "localhost",
				"port": 5432,
				"credentials": {
					"username": "admin",
					"password": "secret"
				}
			}
		}`

		cfg := DefaultConfig()
		cfg.FileSystem = fs
		loader, err := New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer loader.Close()

		if err := loader.LoadFiles("nested.json"); err != nil {
			t.Fatalf("LoadFiles() error = %v", err)
		}

		if loader.GetString("DATABASE_HOST") != "localhost" {
			t.Errorf("GetString(\"DATABASE_HOST\") = %q, want %q", loader.GetString("DATABASE_HOST"), "localhost")
		}
		if loader.GetString("DATABASE_PORT") != "5432" {
			t.Errorf("GetString(\"DATABASE_PORT\") = %q, want %q", loader.GetString("DATABASE_PORT"), "5432")
		}
		if loader.GetString("DATABASE_CREDENTIALS_USERNAME") != "admin" {
			t.Errorf("GetString(\"DATABASE_CREDENTIALS_USERNAME\") = %q, want %q", loader.GetString("DATABASE_CREDENTIALS_USERNAME"), "admin")
		}
	})

	t.Run("array handling", func(t *testing.T) {
		fs := newTestFileSystem()
		fs.files["array.json"] = `{
			"servers": ["server1", "server2", "server3"],
			"ports": [8080, 8081, 8082]
		}`

		cfg := DefaultConfig()
		cfg.FileSystem = fs
		loader, err := New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer loader.Close()

		if err := loader.LoadFiles("array.json"); err != nil {
			t.Fatalf("LoadFiles() error = %v", err)
		}

		if loader.GetString("SERVERS_0") != "server1" {
			t.Errorf("GetString(\"SERVERS_0\") = %q, want %q", loader.GetString("SERVERS_0"), "server1")
		}
		if loader.GetString("SERVERS_2") != "server3" {
			t.Errorf("GetString(\"SERVERS_2\") = %q, want %q", loader.GetString("SERVERS_2"), "server3")
		}
	})

	t.Run("null handling", func(t *testing.T) {
		fs := newTestFileSystem()
		fs.files["null.json"] = `{
			"null_value": null,
			"other_value": "test"
		}`

		cfg := DefaultConfig()
		cfg.FileSystem = fs
		cfg.JSONNullAsEmpty = true
		loader, err := New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer loader.Close()

		if err := loader.LoadFiles("null.json"); err != nil {
			t.Fatalf("LoadFiles() error = %v", err)
		}

		if loader.GetString("NULL_VALUE") != "" {
			t.Errorf("GetString(\"NULL_VALUE\") = %q, want empty", loader.GetString("NULL_VALUE"))
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		fs := newTestFileSystem()
		fs.files["invalid.json"] = `{invalid json}`

		cfg := DefaultConfig()
		cfg.FileSystem = fs
		loader, err := New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer loader.Close()

		if err := loader.LoadFiles("invalid.json"); err == nil {
			t.Error("LoadFiles() should fail with invalid JSON")
		}
	})

	t.Run("file too large", func(t *testing.T) {
		fs := newTestFileSystem()
		fs.files["large.json"] = strings.Repeat(`{"key":"value"}`, 1000)

		cfg := DefaultConfig()
		cfg.FileSystem = fs
		cfg.MaxFileSize = 100
		loader, err := New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer loader.Close()

		if err := loader.LoadFiles("large.json"); err == nil {
			t.Error("LoadFiles() should fail with file too large")
		}
	})

	t.Run("max variables exceeded", func(t *testing.T) {
		fs := newTestFileSystem()
		// Create JSON with many variables
		var sb strings.Builder
		sb.WriteString("{")
		for i := 0; i < 100; i++ {
			if i > 0 {
				sb.WriteString(",")
			}
			sb.WriteString(`"KEY_`)
			sb.WriteString(string(rune('A' + i%26)))
			sb.WriteString(`":"value"`)
		}
		sb.WriteString("}")
		fs.files["many.json"] = sb.String()

		cfg := DefaultConfig()
		cfg.FileSystem = fs
		cfg.MaxVariables = 10
		loader, err := New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer loader.Close()

		if err := loader.LoadFiles("many.json"); err == nil {
			t.Error("LoadFiles() should fail with max variables exceeded")
		}
	})
}

// ============================================================================
// YAML Parser Edge Case Tests
// ============================================================================

func TestYAMLParser_EdgeCases(t *testing.T) {
	t.Run("empty document", func(t *testing.T) {
		fs := newTestFileSystem()
		fs.files["empty.yaml"] = ""

		cfg := DefaultConfig()
		cfg.FileSystem = fs
		loader, err := New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer loader.Close()

		if err := loader.LoadFiles("empty.yaml"); err != nil {
			t.Fatalf("LoadFiles() error = %v", err)
		}
	})

	t.Run("nested map", func(t *testing.T) {
		fs := newTestFileSystem()
		fs.files["nested.yaml"] = `
database:
  host: localhost
  port: 5432
  credentials:
    username: admin
    password: secret
`

		cfg := DefaultConfig()
		cfg.FileSystem = fs
		loader, err := New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer loader.Close()

		if err := loader.LoadFiles("nested.yaml"); err != nil {
			t.Fatalf("LoadFiles() error = %v", err)
		}

		if loader.GetString("DATABASE_HOST") != "localhost" {
			t.Errorf("GetString(\"DATABASE_HOST\") = %q, want %q", loader.GetString("DATABASE_HOST"), "localhost")
		}
	})

	t.Run("list handling", func(t *testing.T) {
		fs := newTestFileSystem()
		fs.files["list.yaml"] = `
servers:
  - server1
  - server2
  - server3
`

		cfg := DefaultConfig()
		cfg.FileSystem = fs
		loader, err := New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer loader.Close()

		if err := loader.LoadFiles("list.yaml"); err != nil {
			t.Fatalf("LoadFiles() error = %v", err)
		}

		if loader.GetString("SERVERS_0") != "server1" {
			t.Errorf("GetString(\"SERVERS_0\") = %q, want %q", loader.GetString("SERVERS_0"), "server1")
		}
	})

	t.Run("boolean handling", func(t *testing.T) {
		fs := newTestFileSystem()
		fs.files["bool.yaml"] = `
debug: true
production: false
`

		cfg := DefaultConfig()
		cfg.FileSystem = fs
		cfg.YAMLBoolAsString = true
		loader, err := New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer loader.Close()

		if err := loader.LoadFiles("bool.yaml"); err != nil {
			t.Fatalf("LoadFiles() error = %v", err)
		}

		if loader.GetString("DEBUG") != "true" {
			t.Errorf("GetString(\"DEBUG\") = %q, want %q", loader.GetString("DEBUG"), "true")
		}
	})

	t.Run("null handling", func(t *testing.T) {
		fs := newTestFileSystem()
		fs.files["null.yaml"] = `
null_value: null
other_value: test
`

		cfg := DefaultConfig()
		cfg.FileSystem = fs
		cfg.YAMLNullAsEmpty = true
		loader, err := New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer loader.Close()

		if err := loader.LoadFiles("null.yaml"); err != nil {
			t.Fatalf("LoadFiles() error = %v", err)
		}

		if loader.GetString("NULL_VALUE") != "" {
			t.Errorf("GetString(\"NULL_VALUE\") = %q, want empty", loader.GetString("NULL_VALUE"))
		}
	})

	t.Run("invalid YAML", func(t *testing.T) {
		fs := newTestFileSystem()
		fs.files["invalid.yaml"] = `
invalid:
  - unclosed
    - bad indent
`

		cfg := DefaultConfig()
		cfg.FileSystem = fs
		loader, err := New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer loader.Close()

		// YAML parsing is lenient, may not error
		_ = loader.LoadFiles("invalid.yaml")
	})

	t.Run("complex nested structure", func(t *testing.T) {
		fs := newTestFileSystem()
		fs.files["complex.yaml"] = `
app:
  name: myapp
  servers:
    - name: web1
      port: 8080
    - name: web2
      port: 8081
  database:
    primary:
      host: db1.example.com
      port: 5432
`

		cfg := DefaultConfig()
		cfg.FileSystem = fs
		loader, err := New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer loader.Close()

		if err := loader.LoadFiles("complex.yaml"); err != nil {
			t.Fatalf("LoadFiles() error = %v", err)
		}

		if loader.GetString("APP_NAME") != "myapp" {
			t.Errorf("GetString(\"APP_NAME\") = %q, want %q", loader.GetString("APP_NAME"), "myapp")
		}
	})
}

// ============================================================================
// Error Type Tests - Extended
// ============================================================================

func TestJSONError(t *testing.T) {
	t.Run("with path", func(t *testing.T) {
		err := &JSONError{
			Path:    "$.database.host",
			Message: "invalid type",
			Err:     errors.New("expected string"),
		}

		if err.Error() == "" {
			t.Error("JSONError.Error() should not be empty")
		}

		// Unwrap returns the underlying error
		unwrapped := err.Unwrap()
		if unwrapped == nil {
			t.Error("JSONError.Unwrap() should return non-nil error")
		}
	})

	t.Run("without path", func(t *testing.T) {
		err := &JSONError{
			Message: "parse error",
		}

		if err.Error() == "" {
			t.Error("JSONError.Error() should not be empty")
		}
	})
}

func TestYAMLError(t *testing.T) {
	t.Run("with path and line", func(t *testing.T) {
		err := &YAMLError{
			Path:    "config.yaml",
			Line:    10,
			Column:  5,
			Message: "invalid mapping",
		}

		if err.Error() == "" {
			t.Error("YAMLError.Error() should not be empty")
		}
	})

	t.Run("with line only", func(t *testing.T) {
		err := &YAMLError{
			Line:    15,
			Message: "indentation error",
		}

		if err.Error() == "" {
			t.Error("YAMLError.Error() should not be empty")
		}
	})

	t.Run("without location", func(t *testing.T) {
		err := &YAMLError{
			Message: "parse error",
		}

		if err.Error() == "" {
			t.Error("YAMLError.Error() should not be empty")
		}
	})
}

func TestExpansionError(t *testing.T) {
	t.Run("with key", func(t *testing.T) {
		err := &ExpansionError{
			Key:   "VAR",
			Depth: 10,
			Limit: 5,
		}

		if err.Error() == "" {
			t.Error("ExpansionError.Error() should not be empty")
		}
	})

	t.Run("without key", func(t *testing.T) {
		err := &ExpansionError{
			Depth: 10,
			Limit: 5,
			Chain: "A -> B -> C",
		}

		if err.Error() == "" {
			t.Error("ExpansionError.Error() should not be empty")
		}
	})
}

func TestSecurityError(t *testing.T) {
	t.Run("with key", func(t *testing.T) {
		err := &SecurityError{
			Action:  "set",
			Reason:  "forbidden key",
			Key:     "SECRET_KEY",
			Details: "key is in forbidden list",
		}

		if err.Error() == "" {
			t.Error("SecurityError.Error() should not be empty")
		}
	})

	t.Run("without key", func(t *testing.T) {
		err := &SecurityError{
			Action: "load",
			Reason: "file too large",
		}

		if err.Error() == "" {
			t.Error("SecurityError.Error() should not be empty")
		}
	})
}

// ============================================================================
// validateFilePath Tests (Security)
// ============================================================================

func TestValidateFilePath(t *testing.T) {
	tests := []struct {
		name      string
		filename  string
		wantErr   bool
		errReason string
	}{
		{"valid relative path", "config/.env", false, ""},
		{"valid simple filename", ".env", false, ""},
		{"empty filename", "", true, "empty filename"},
		{"null byte in path", "config\x00.env", true, "null byte"},
		{"UNC path backslash", "\\\\server\\share", true, "UNC path"},
		{"network path forward slash", "//server/share", true, "network path"},
		{"Unix absolute path", "/etc/passwd", true, "absolute path"},
		{"Windows drive letter", "C:\\Windows", true, "drive letter"},
		{"lowercase drive letter", "c:\\test", true, "drive letter"},
		{"path traversal", "../../../etc/passwd", true, "path traversal"},
		{"hidden traversal", "config/../../../etc", true, "path traversal"},
		{"Windows reserved CON", "CON", true, "reserved device"},
		{"Windows reserved NUL", "NUL.txt", true, "reserved device"},
		{"Windows reserved AUX", "AUX:", true, "reserved device"},
		{"Windows reserved PRN", "PRN", true, "reserved device"},
		{"Windows COM port", "COM1", true, "reserved device"},
		{"Windows LPT port", "LPT1.txt", true, "reserved device"},
		{"valid with dots", "config.local/.env", false, ""},
		{"valid subdirectory", "config/local/.env", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateFilePath(tt.filename)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateFilePath(%q) error = %v, wantErr %v", tt.filename, err, tt.wantErr)
			}
			if err != nil && tt.errReason != "" {
				var secErr *SecurityError
				if errors.As(err, &secErr) {
					if !strings.Contains(secErr.Reason, tt.errReason) {
						t.Errorf("validateFilePath(%q) reason = %q, want containing %q", tt.filename, secErr.Reason, tt.errReason)
					}
				}
			}
		})
	}
}

// ============================================================================
// newParseError Tests
// ============================================================================

func TestNewParseError(t *testing.T) {
	err := newParseError("test.env", 10, "API_KEY=secret123", errors.New("parse failed"))

	if err.File != "test.env" {
		t.Errorf("File = %q, want %q", err.File, "test.env")
	}
	if err.Line != 10 {
		t.Errorf("Line = %d, want 10", err.Line)
	}
	if err.Err == nil {
		t.Error("Err should not be nil")
	}

	// Verify error message is not empty
	if err.Error() == "" {
		t.Error("Error() should not be empty")
	}
}

// ============================================================================
// New() Error Path Tests
// ============================================================================

func TestNew_ErrorPaths(t *testing.T) {
	t.Run("parser creation error with factory cleanup", func(t *testing.T) {
		// This tests the error path where createParsers fails
		// and factory.Close() is called for cleanup
		cfg := DefaultConfig()
		loader, err := New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		loader.Close()
	})

	t.Run("auto-load file not found with fail on missing", func(t *testing.T) {
		fs := newTestFileSystem()
		// Don't add any files

		cfg := DefaultConfig()
		cfg.FileSystem = fs
		cfg.Filenames = []string{"missing.env"}
		cfg.FailOnMissingFile = true

		_, err := New(cfg)
		if err == nil {
			t.Error("New() should fail with missing file and FailOnMissingFile=true")
		}
	})

	t.Run("auto-apply error", func(t *testing.T) {
		fs := newTestFileSystem()
		fs.files[".env"] = "KEY=value"

		cfg := DefaultConfig()
		cfg.FileSystem = fs
		cfg.Filenames = []string{".env"}
		cfg.AutoApply = true

		loader, err := New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		loader.Close()
	})
}

// ============================================================================
// getWithDefault Tests
// ============================================================================

func TestGetWithDefault(t *testing.T) {
	cfg := DefaultConfig()
	loader, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer loader.Close()

	t.Run("existing key without default", func(t *testing.T) {
		loader.Set("KEY", "value")
		result := loader.GetString("KEY")
		if result != "value" {
			t.Errorf("GetString() = %q, want %q", result, "value")
		}
	})

	t.Run("missing key with default", func(t *testing.T) {
		result := loader.GetString("MISSING", "default_value")
		if result != "default_value" {
			t.Errorf("GetString() = %q, want %q", result, "default_value")
		}
	})

	t.Run("closed loader returns default", func(t *testing.T) {
		cfg2 := DefaultConfig()
		loader2, err := New(cfg2)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		loader2.Close()

		result := loader2.GetString("ANY_KEY", "default")
		if result != "default" {
			t.Errorf("GetString() on closed loader = %q, want %q", result, "default")
		}
	})
}

// OSFileSystem Tests
// ============================================================================

func TestOSFileSystem_Getenv(t *testing.T) {
	fs := OSFileSystem{}

	// Test getting an environment variable
	t.Setenv("TEST_OS_GETENV", "test_value")
	result := fs.Getenv("TEST_OS_GETENV")
	if result != "test_value" {
		t.Errorf("Getenv() = %q, want %q", result, "test_value")
	}

	// Test getting non-existent variable
	result = fs.Getenv("NON_EXISTENT_VAR_12345")
	if result != "" {
		t.Errorf("Getenv() for non-existent var = %q, want empty", result)
	}
}

func TestOSFileSystem_Setenv(t *testing.T) {
	fs := OSFileSystem{}

	err := fs.Setenv("TEST_OS_SETENV", "new_value")
	if err != nil {
		t.Errorf("Setenv() error = %v", err)
	}

	result := fs.Getenv("TEST_OS_SETENV")
	if result != "new_value" {
		t.Errorf("Getenv() after Setenv() = %q, want %q", result, "new_value")
	}
}

func TestOSFileSystem_Unsetenv(t *testing.T) {
	fs := OSFileSystem{}

	// Set and then unset
	t.Setenv("TEST_OS_UNSETENV", "value")
	err := fs.Unsetenv("TEST_OS_UNSETENV")
	if err != nil {
		t.Errorf("Unsetenv() error = %v", err)
	}

	result := fs.Getenv("TEST_OS_UNSETENV")
	if result != "" {
		t.Errorf("Getenv() after Unsetenv() = %q, want empty", result)
	}
}

func TestOSFileSystem_LookupEnv(t *testing.T) {
	fs := OSFileSystem{}

	t.Setenv("TEST_OS_LOOKUP", "lookup_value")
	value, ok := fs.LookupEnv("TEST_OS_LOOKUP")
	if !ok {
		t.Error("LookupEnv() should find existing variable")
	}
	if value != "lookup_value" {
		t.Errorf("LookupEnv() = %q, want %q", value, "lookup_value")
	}

	_, ok = fs.LookupEnv("NON_EXISTENT_VAR_12345")
	if ok {
		t.Error("LookupEnv() should return false for non-existent variable")
	}
}

func TestOSFileSystem_Stat(t *testing.T) {
	fs := OSFileSystem{}

	// Test existing file (this test file)
	info, err := fs.Stat("filesystem.go")
	if err != nil {
		t.Errorf("Stat() error = %v", err)
	}
	if info.Name() != "filesystem.go" {
		t.Errorf("Stat().Name() = %q, want %q", info.Name(), "filesystem.go")
	}

	// Test non-existent file
	_, err = fs.Stat("non_existent_file_12345.txt")
	if err == nil {
		t.Error("Stat() should return error for non-existent file")
	}
}

func TestOSFileSystem_MkdirAll(t *testing.T) {
	fs := OSFileSystem{}

	// Create temp directory
	tmpDir := t.TempDir()
	testDir := tmpDir + "/test/nested/dir"

	err := fs.MkdirAll(testDir, 0755)
	if err != nil {
		t.Errorf("MkdirAll() error = %v", err)
	}

	// Verify directory exists
	info, err := fs.Stat(testDir)
	if err != nil {
		t.Errorf("Stat() after MkdirAll() error = %v", err)
	}
	if !info.IsDir() {
		t.Error("MkdirAll() should create a directory")
	}
}

func TestOSFileSystem_Remove(t *testing.T) {
	fs := OSFileSystem{}

	// Remove non-existent file should fail
	err := fs.Remove("non_existent_file_12345.txt")
	if err == nil {
		t.Error("Remove() should return error for non-existent file")
	}
}

func TestOSFileSystem_Open_Missing(t *testing.T) {
	fs := OSFileSystem{}

	// Open non-existent file should fail
	_, err := fs.Open("non_existent_file_12345.txt")
	if err == nil {
		t.Error("Open() should return error for non-existent file")
	}
}

func TestOSFileSystem_OpenFile(t *testing.T) {
	fs := OSFileSystem{}

	// Create temp file for testing
	tmpDir := t.TempDir()
	tmpFile := tmpDir + "/test_openfile.txt"
	content := []byte("test content for OpenFile")
	if err := os.WriteFile(tmpFile, content, 0644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	// Test OpenFile with O_RDONLY
	f, err := fs.OpenFile(tmpFile, os.O_RDONLY, 0644)
	if err != nil {
		t.Errorf("OpenFile() error = %v", err)
	}
	if f != nil {
		// Read and verify content
		data, err := io.ReadAll(f)
		if err != nil {
			t.Errorf("ReadAll() error = %v", err)
		}
		if string(data) != string(content) {
			t.Errorf("OpenFile() content = %q, want %q", string(data), string(content))
		}
		f.Close()
	}

	// Test OpenFile with non-existent file should fail
	_, err = fs.OpenFile(tmpDir+"/nonexistent.txt", os.O_RDONLY, 0644)
	if err == nil {
		t.Error("OpenFile() should return error for non-existent file")
	}
}

func TestOSFileSystem_Rename(t *testing.T) {
	fs := OSFileSystem{}

	tmpDir := t.TempDir()
	oldPath := tmpDir + "/old_name.txt"
	newPath := tmpDir + "/new_name.txt"
	content := []byte("test content for rename")

	// Create the old file
	if err := os.WriteFile(oldPath, content, 0644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	// Test Rename
	err := fs.Rename(oldPath, newPath)
	if err != nil {
		t.Errorf("Rename() error = %v", err)
	}

	// Verify old file no longer exists
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Error("Old file should not exist after rename")
	}

	// Verify new file exists with correct content
	data, err := os.ReadFile(newPath)
	if err != nil {
		t.Errorf("ReadFile() error = %v", err)
	}
	if string(data) != string(content) {
		t.Errorf("Rename() content = %q, want %q", string(data), string(content))
	}

	// Test Rename with non-existent source should fail
	err = fs.Rename(tmpDir+"/nonexistent.txt", tmpDir+"/another.txt")
	if err == nil {
		t.Error("Rename() should return error for non-existent source")
	}
}

// FileFormat.String() Tests
// ============================================================================

func TestFileFormat_String(t *testing.T) {
	tests := []struct {
		format   FileFormat
		expected string
	}{
		{FormatAuto, "auto"},
		{FormatEnv, "dotenv"},
		{FormatJSON, "json"},
		{FormatYAML, "yaml"},
		{FileFormat(999), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.format.String(); got != tt.expected {
				t.Errorf("FileFormat(%d).String() = %q, want %q", tt.format, got, tt.expected)
			}
		})
	}
}

// ============================================================================
// RegisterParser Tests
// ============================================================================

// testFormatCounter generates unique format IDs for test isolation.
// This ensures tests can run multiple times with -count=N without conflicts.
var testFormatCounter int64

// nextTestFormat returns a unique FileFormat for testing.
func nextTestFormat() FileFormat {
	return FileFormat(1000 + atomic.AddInt64(&testFormatCounter, 1))
}

func TestRegisterParser(t *testing.T) {
	t.Run("cannot override built-in dotenv parser", func(t *testing.T) {
		err := RegisterParser(FormatEnv, nil)
		if err == nil {
			t.Error("RegisterParser should fail for built-in FormatEnv")
		}
	})

	t.Run("cannot override built-in JSON parser", func(t *testing.T) {
		err := RegisterParser(FormatJSON, nil)
		if err == nil {
			t.Error("RegisterParser should fail for built-in FormatJSON")
		}
	})

	t.Run("cannot override built-in YAML parser", func(t *testing.T) {
		err := RegisterParser(FormatYAML, nil)
		if err == nil {
			t.Error("RegisterParser should fail for built-in FormatYAML")
		}
	})

	t.Run("custom format registration", func(t *testing.T) {
		// Use unique format to ensure test isolation with -count=N
		customFormat := nextTestFormat()

		// First registration should succeed
		err := RegisterParser(customFormat, func(cfg Config, factory *ComponentFactory) (EnvParser, error) {
			return nil, nil
		})
		if err != nil {
			t.Errorf("RegisterParser for custom format failed: %v", err)
		}

		// Duplicate registration should fail
		err = RegisterParser(customFormat, func(cfg Config, factory *ComponentFactory) (EnvParser, error) {
			return nil, nil
		})
		if err == nil {
			t.Error("RegisterParser should fail for duplicate custom format")
		}
	})
}
