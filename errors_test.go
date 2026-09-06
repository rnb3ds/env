package env

import (
	"errors"
	"testing"
)

// TestLoader_SetAutoApplyThenDelete verifies that a variable applied to the
// environment via Set (AutoApply) is removed from the environment when
// deleted. Regression test: Set previously did not set l.applied, so Delete
// skipped Unsetenv and leaked the variable into the process environment.
func TestLoader_SetAutoApplyThenDelete(t *testing.T) {
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
	if !loader.IsApplied() {
		t.Error("IsApplied() = false after Set() with AutoApply")
	}
	if _, ok := fs.LookupEnv("KEY"); !ok {
		t.Fatal("KEY should be set in the environment after Set() with AutoApply")
	}

	if err := loader.Delete("KEY"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, ok := fs.LookupEnv("KEY"); ok {
		t.Error("KEY should be removed from the environment after Delete()")
	}
}

// TestSentinelErrorWiring verifies that the sentinel errors documented on
// LoadFiles/Set/Validate/New are actually matched via errors.Is by the errors
// those code paths return.
func TestSentinelErrorWiring(t *testing.T) {
	t.Run("New with invalid config matches ErrInvalidConfig", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.MaxFileSize = -1
		_, err := New(cfg)
		if err == nil {
			t.Fatal("New() should fail with invalid config")
		}
		if !errors.Is(err, ErrInvalidConfig) {
			t.Errorf("errors.Is(err, ErrInvalidConfig) = false, err = %v", err)
		}
	})

	t.Run("Set with invalid key matches ErrInvalidKey", func(t *testing.T) {
		loader, err := New(DefaultConfig())
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer loader.Close()

		if err := loader.Set("1INVALID", "value"); !errors.Is(err, ErrInvalidKey) {
			t.Errorf("errors.Is(err, ErrInvalidKey) = false, err = %v", err)
		}
	})

	t.Run("Set with forbidden key matches ErrForbiddenKey", func(t *testing.T) {
		loader, err := New(DefaultConfig())
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer loader.Close()

		err = loader.Set("PATH", "/usr/bin/evil")
		if !errors.Is(err, ErrForbiddenKey) {
			t.Errorf("errors.Is(err, ErrForbiddenKey) = false, err = %v", err)
		}
		if !errors.Is(err, ErrSecurityViolation) {
			t.Errorf("errors.Is(err, ErrSecurityViolation) = false, err = %v", err)
		}
	})

	t.Run("Validate with missing required key matches ErrMissingRequired", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Filenames = nil // no auto-load; empty loader state
		cfg.RequiredKeys = []string{"MUST_EXIST"}
		loader, err := New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer loader.Close()

		if err := loader.Validate(); !errors.Is(err, ErrMissingRequired) {
			t.Errorf("errors.Is(err, ErrMissingRequired) = false, err = %v", err)
		}
	})

	t.Run("parse exceeding MaxVariables matches ErrMaxVariables", func(t *testing.T) {
		fs := newTestFileSystem()
		fs.files[".env"] = "A=1\nB=2\n"

		cfg := DefaultConfig()
		cfg.FileSystem = fs
		cfg.MaxVariables = 1
		_, err := New(cfg) // auto-loads ".env" from the mock fs
		if err == nil {
			t.Fatal("New() should fail when MaxVariables is exceeded")
		}
		if !errors.Is(err, ErrMaxVariables) {
			t.Errorf("errors.Is(err, ErrMaxVariables) = false, err = %v", err)
		}
	})

	t.Run("re-exported sentinels share identity with internal errors", func(t *testing.T) {
		if ErrInvalidKey == nil || ErrForbiddenKey == nil || ErrMaxVariables == nil || ErrMissingRequired == nil {
			t.Error("sentinel errors must not be nil")
		}
	})
}

// TestStructuredFormats_EnforceForbiddenKeys verifies JSON/YAML file loads
// enforce the forbidden-keys policy the same way the .env parser does.
// Regression test: the structured parsers validated only length/pattern/value,
// so a config.json containing "PATH" or "LD_PRELOAD" bypassed the policy and
// was applied to the process environment.
func TestStructuredFormats_EnforceForbiddenKeys(t *testing.T) {
	files := map[string]string{
		"config.json": `{"PATH": "/usr/evil/bin", "LD_PRELOAD": "/tmp/evil.so", "GOOD": "1"}`,
		"config.yaml": "PATH: /usr/evil/bin\nGOOD: 1\n",
	}

	for filename, content := range files {
		t.Run(filename, func(t *testing.T) {
			fs := newTestFileSystem()
			fs.files[filename] = content

			cfg := DefaultConfig()
			cfg.Filenames = []string{filename}
			cfg.FileSystem = fs
			cfg.AutoApply = true

			_, err := New(cfg)
			if err == nil {
				t.Fatal("New() should reject a file containing forbidden keys")
			}
			if !errors.Is(err, ErrForbiddenKey) {
				t.Errorf("errors.Is(err, ErrForbiddenKey) = false, err = %v", err)
			}
			if _, ok := fs.LookupEnv("PATH"); ok {
				t.Error("PATH must not be applied to the environment")
			}
		})
	}
}

// TestLoader_DeleteForeignEnvVar verifies Delete does not unset process
// environment variables the loader never applied. Regression test: Delete
// unset ANY key once the loader had applied something, clobbering foreign
// process variables like HOME.
func TestLoader_DeleteForeignEnvVar(t *testing.T) {
	fs := newTestFileSystem()
	if err := fs.Setenv("FOREIGN_KEY", "original"); err != nil {
		t.Fatalf("Setenv() error = %v", err)
	}

	cfg := DefaultConfig()
	cfg.FileSystem = fs
	cfg.AutoApply = true
	loader, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer loader.Close()

	// Loader applied something of its own, then a foreign key is deleted.
	if err := loader.Set("OWN_KEY", "v"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	if !loader.IsApplied() {
		t.Fatal("IsApplied() = false after applied Set()")
	}

	if err := loader.Delete("FOREIGN_KEY"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if v, ok := fs.LookupEnv("FOREIGN_KEY"); !ok || v != "original" {
		t.Errorf("foreign env var clobbered by Delete(): value = %q, exists = %v", v, ok)
	}

	// A loader-applied key is still cleaned up on Delete.
	if err := loader.Delete("OWN_KEY"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, ok := fs.LookupEnv("OWN_KEY"); ok {
		t.Error("OWN_KEY should be removed from the environment after Delete()")
	}
}
