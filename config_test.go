package env

import (
	"errors"
	"regexp"
	"testing"
)

// ============================================================================
// DefaultConfig Tests
// ============================================================================

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	t.Run("file handling defaults", func(t *testing.T) {
		if len(cfg.Filenames) != 1 || cfg.Filenames[0] != ".env" {
			t.Errorf("Filenames = %v, want [.env]", cfg.Filenames)
		}
		if cfg.FailOnMissingFile {
			t.Error("FailOnMissingFile should be false")
		}
		if cfg.OverwriteExisting {
			t.Error("OverwriteExisting should be false")
		}
		if !cfg.AllowExportPrefix {
			t.Error("AllowExportPrefix should be true")
		}
		if cfg.AllowYamlSyntax {
			t.Error("AllowYamlSyntax should be false")
		}
	})

	t.Run("size limit defaults", func(t *testing.T) {
		if cfg.MaxFileSize != DefaultMaxFileSize {
			t.Errorf("MaxFileSize = %d, want %d", cfg.MaxFileSize, DefaultMaxFileSize)
		}
		if cfg.MaxLineLength != DefaultMaxLineLength {
			t.Errorf("MaxLineLength = %d, want %d", cfg.MaxLineLength, DefaultMaxLineLength)
		}
		if cfg.MaxKeyLength != DefaultMaxKeyLength {
			t.Errorf("MaxKeyLength = %d, want %d", cfg.MaxKeyLength, DefaultMaxKeyLength)
		}
		if cfg.MaxValueLength != DefaultMaxValueLength {
			t.Errorf("MaxValueLength = %d, want %d", cfg.MaxValueLength, DefaultMaxValueLength)
		}
		if cfg.MaxVariables != DefaultMaxVariables {
			t.Errorf("MaxVariables = %d, want %d", cfg.MaxVariables, DefaultMaxVariables)
		}
		if cfg.MaxExpansionDepth != DefaultMaxExpansionDepth {
			t.Errorf("MaxExpansionDepth = %d, want %d", cfg.MaxExpansionDepth, DefaultMaxExpansionDepth)
		}
	})

	t.Run("validation defaults", func(t *testing.T) {
		// KeyPattern is nil by default to enable fast byte-level validation
		// This provides ~10x performance improvement over regex validation
		if cfg.KeyPattern != nil {
			t.Error("KeyPattern should be nil for fast byte-level validation")
		}
		if len(cfg.AllowedKeys) != 0 {
			t.Errorf("AllowedKeys = %v, want empty", cfg.AllowedKeys)
		}
		if len(cfg.ForbiddenKeys) != 0 {
			t.Errorf("ForbiddenKeys = %v, want empty", cfg.ForbiddenKeys)
		}
		if len(cfg.RequiredKeys) != 0 {
			t.Errorf("RequiredKeys = %v, want empty", cfg.RequiredKeys)
		}
	})

	t.Run("security defaults", func(t *testing.T) {
		if !cfg.ValidateValues {
			t.Error("ValidateValues should be true")
		}
	})

	t.Run("expansion defaults", func(t *testing.T) {
		if !cfg.ExpandVariables {
			t.Error("ExpandVariables should be true")
		}
	})

	t.Run("audit defaults", func(t *testing.T) {
		if cfg.AuditEnabled {
			t.Error("AuditEnabled should be false")
		}
	})

	t.Run("JSON defaults", func(t *testing.T) {
		if !cfg.JSONNullAsEmpty {
			t.Error("JSONNullAsEmpty should be true")
		}
		if !cfg.JSONNumberAsString {
			t.Error("JSONNumberAsString should be true")
		}
		if !cfg.JSONBoolAsString {
			t.Error("JSONBoolAsString should be true")
		}
		if cfg.JSONMaxDepth != 10 {
			t.Errorf("JSONMaxDepth = %d, want 10", cfg.JSONMaxDepth)
		}
	})

	t.Run("YAML defaults", func(t *testing.T) {
		if !cfg.YAMLNullAsEmpty {
			t.Error("YAMLNullAsEmpty should be true")
		}
		if !cfg.YAMLNumberAsString {
			t.Error("YAMLNumberAsString should be true")
		}
		if !cfg.YAMLBoolAsString {
			t.Error("YAMLBoolAsString should be true")
		}
		if cfg.YAMLMaxDepth != 10 {
			t.Errorf("YAMLMaxDepth = %d, want 10", cfg.YAMLMaxDepth)
		}
	})
}

// ============================================================================
// DevelopmentConfig Tests
// ============================================================================

func TestDevelopmentConfig(t *testing.T) {
	cfg := DevelopmentConfig()

	t.Run("development-specific settings", func(t *testing.T) {
		if cfg.FailOnMissingFile {
			t.Error("FailOnMissingFile should be false")
		}
		if !cfg.OverwriteExisting {
			t.Error("OverwriteExisting should be true")
		}
		if !cfg.AllowYamlSyntax {
			t.Error("AllowYamlSyntax should be true")
		}
		if cfg.MaxFileSize != 10*1024*1024 {
			t.Errorf("MaxFileSize = %d, want 10MB", cfg.MaxFileSize)
		}
		if cfg.MaxVariables != 500 {
			t.Errorf("MaxVariables = %d, want 500", cfg.MaxVariables)
		}
		// SECURITY: ValidateValues should remain true even in development
		// to prevent injection attacks during development
		if !cfg.ValidateValues {
			t.Error("ValidateValues should be true for security")
		}
	})
}

// ============================================================================
// TestingConfig Tests
// ============================================================================

func TestTestingConfig(t *testing.T) {
	cfg := TestingConfig()

	t.Run("testing-specific settings", func(t *testing.T) {
		if cfg.FailOnMissingFile {
			t.Error("FailOnMissingFile should be false")
		}
		if !cfg.OverwriteExisting {
			t.Error("OverwriteExisting should be true")
		}
		if cfg.MaxFileSize != 64*1024 {
			t.Errorf("MaxFileSize = %d, want 64KB", cfg.MaxFileSize)
		}
		if cfg.MaxVariables != 50 {
			t.Errorf("MaxVariables = %d, want 50", cfg.MaxVariables)
		}
		if cfg.AuditEnabled {
			t.Error("AuditEnabled should be false")
		}
	})
}

// ============================================================================
// ProductionConfig Tests
// ============================================================================

func TestProductionConfig(t *testing.T) {
	cfg := ProductionConfig()

	t.Run("production-specific settings", func(t *testing.T) {
		if !cfg.FailOnMissingFile {
			t.Error("FailOnMissingFile should be true")
		}
		if cfg.OverwriteExisting {
			t.Error("OverwriteExisting should be false")
		}
		if !cfg.AuditEnabled {
			t.Error("AuditEnabled should be true")
		}
		if !cfg.ValidateValues {
			t.Error("ValidateValues should be true")
		}
		if cfg.MaxFileSize != 64*1024 {
			t.Errorf("MaxFileSize = %d, want 64KB", cfg.MaxFileSize)
		}
		if cfg.MaxVariables != 50 {
			t.Errorf("MaxVariables = %d, want 50", cfg.MaxVariables)
		}
	})
}

// ============================================================================
// Config.Validate Tests
// ============================================================================

func TestConfig_Validate(t *testing.T) {
	t.Run("valid default config", func(t *testing.T) {
		cfg := DefaultConfig()
		if err := cfg.Validate(); err != nil {
			t.Errorf("Validate() error = %v", err)
		}
	})

	t.Run("invalid key pattern", func(t *testing.T) {
		cfg := DefaultConfig()
		// Pattern that doesn't match TEST_KEY
		cfg.KeyPattern = regexp.MustCompile(`^[a-z]+$`)
		if err := cfg.Validate(); err == nil {
			t.Error("Validate() should fail with invalid key pattern")
		}
	})
}

// ============================================================================
// KeyPattern Edge Case Tests
// ============================================================================

func TestKeyPattern_EdgeCases(t *testing.T) {
	// Test the default key pattern behavior
	t.Run("nil pattern allows standard keys", func(t *testing.T) {
		cfg := DefaultConfig()
		// KeyPattern is nil by default for fast byte-level validation
		if cfg.KeyPattern != nil {
			t.Error("KeyPattern should be nil by default")
		}

		loader, err := New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer loader.Close()

		// Standard keys should work
		if err := loader.Set("TEST_KEY", "value"); err != nil {
			t.Errorf("Set() error = %v", err)
		}
		if err := loader.Set("API_KEY_123", "value"); err != nil {
			t.Errorf("Set() error = %v", err)
		}
	})

	t.Run("custom pattern matches valid keys", func(t *testing.T) {
		cfg := DefaultConfig()
		// Pattern that matches uppercase with underscores and numbers (and TEST_KEY)
		cfg.KeyPattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)

		loader, err := New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer loader.Close()

		// Keys matching the pattern should work
		if err := loader.Set("VALID_KEY", "value"); err != nil {
			t.Errorf("Set() error = %v", err)
		}
	})

	t.Run("pattern must match TEST_KEY during validation", func(t *testing.T) {
		cfg := DefaultConfig()
		// Pattern that only allows lowercase - this will fail validation
		// because it can't match TEST_KEY
		cfg.KeyPattern = regexp.MustCompile(`^[a-z]+$`)

		_, err := New(cfg)
		if err == nil {
			t.Error("New() should fail with pattern that doesn't match TEST_KEY")
		}
	})

	t.Run("pattern must not allow numeric start", func(t *testing.T) {
		cfg := DefaultConfig()
		// Pattern that allows keys starting with numbers
		// This should fail validation because it allows numeric-start keys
		cfg.KeyPattern = regexp.MustCompile(`^[A-Z0-9][A-Z0-9_]*$`)

		_, err := New(cfg)
		if err == nil {
			t.Error("New() should fail with pattern that allows numeric-start keys")
		}
	})
}

// ============================================================================
// validateConfigLimits Tests
// ============================================================================

func TestValidateConfigLimits(t *testing.T) {
	tests := []struct {
		name        string
		maxSize     int64
		maxLineLen  int
		maxKeyLen   int
		maxValLen   int
		maxVars     int
		maxDepth    int
		wantErr     bool
		errContains string
	}{
		{
			name:       "valid limits",
			maxSize:    DefaultMaxFileSize,
			maxLineLen: DefaultMaxLineLength,
			maxKeyLen:  DefaultMaxKeyLength,
			maxValLen:  DefaultMaxValueLength,
			maxVars:    DefaultMaxVariables,
			maxDepth:   DefaultMaxExpansionDepth,
			wantErr:    false,
		},
		{
			name:        "zero max file size",
			maxSize:     0,
			maxLineLen:  DefaultMaxLineLength,
			maxKeyLen:   DefaultMaxKeyLength,
			maxValLen:   DefaultMaxValueLength,
			maxVars:     DefaultMaxVariables,
			maxDepth:    DefaultMaxExpansionDepth,
			wantErr:     true,
			errContains: "MaxFileSize",
		},
		{
			name:        "negative max file size",
			maxSize:     -1,
			maxLineLen:  DefaultMaxLineLength,
			maxKeyLen:   DefaultMaxKeyLength,
			maxValLen:   DefaultMaxValueLength,
			maxVars:     DefaultMaxVariables,
			maxDepth:    DefaultMaxExpansionDepth,
			wantErr:     true,
			errContains: "MaxFileSize",
		},
		{
			name:        "exceeds hard max file size",
			maxSize:     200 * 1024 * 1024,
			maxLineLen:  DefaultMaxLineLength,
			maxKeyLen:   DefaultMaxKeyLength,
			maxValLen:   DefaultMaxValueLength,
			maxVars:     DefaultMaxVariables,
			maxDepth:    DefaultMaxExpansionDepth,
			wantErr:     true,
			errContains: "MaxFileSize",
		},
		{
			name:        "zero max line length",
			maxSize:     DefaultMaxFileSize,
			maxLineLen:  0,
			maxKeyLen:   DefaultMaxKeyLength,
			maxValLen:   DefaultMaxValueLength,
			maxVars:     DefaultMaxVariables,
			maxDepth:    DefaultMaxExpansionDepth,
			wantErr:     true,
			errContains: "MaxLineLength",
		},
		{
			name:        "exceeds hard max line length",
			maxSize:     DefaultMaxFileSize,
			maxLineLen:  100 * 1024,
			maxKeyLen:   DefaultMaxKeyLength,
			maxValLen:   DefaultMaxValueLength,
			maxVars:     DefaultMaxVariables,
			maxDepth:    DefaultMaxExpansionDepth,
			wantErr:     true,
			errContains: "MaxLineLength",
		},
		{
			name:        "zero max key length",
			maxSize:     DefaultMaxFileSize,
			maxLineLen:  DefaultMaxLineLength,
			maxKeyLen:   0,
			maxValLen:   DefaultMaxValueLength,
			maxVars:     DefaultMaxVariables,
			maxDepth:    DefaultMaxExpansionDepth,
			wantErr:     true,
			errContains: "MaxKeyLength",
		},
		{
			name:        "exceeds hard max key length",
			maxSize:     DefaultMaxFileSize,
			maxLineLen:  DefaultMaxLineLength,
			maxKeyLen:   1025,
			maxValLen:   DefaultMaxValueLength,
			maxVars:     DefaultMaxVariables,
			maxDepth:    DefaultMaxExpansionDepth,
			wantErr:     true,
			errContains: "MaxKeyLength",
		},
		{
			name:        "zero max value length",
			maxSize:     DefaultMaxFileSize,
			maxLineLen:  DefaultMaxLineLength,
			maxKeyLen:   DefaultMaxKeyLength,
			maxValLen:   0,
			maxVars:     DefaultMaxVariables,
			maxDepth:    DefaultMaxExpansionDepth,
			wantErr:     true,
			errContains: "MaxValueLength",
		},
		{
			name:        "exceeds hard max value length",
			maxSize:     DefaultMaxFileSize,
			maxLineLen:  DefaultMaxLineLength,
			maxKeyLen:   DefaultMaxKeyLength,
			maxValLen:   1024*1024 + 1,
			maxVars:     DefaultMaxVariables,
			maxDepth:    DefaultMaxExpansionDepth,
			wantErr:     true,
			errContains: "MaxValueLength",
		},
		{
			name:        "zero max variables",
			maxSize:     DefaultMaxFileSize,
			maxLineLen:  DefaultMaxLineLength,
			maxKeyLen:   DefaultMaxKeyLength,
			maxValLen:   DefaultMaxValueLength,
			maxVars:     0,
			maxDepth:    DefaultMaxExpansionDepth,
			wantErr:     true,
			errContains: "MaxVariables",
		},
		{
			name:        "exceeds hard max variables",
			maxSize:     DefaultMaxFileSize,
			maxLineLen:  DefaultMaxLineLength,
			maxKeyLen:   DefaultMaxKeyLength,
			maxValLen:   DefaultMaxValueLength,
			maxVars:     10001,
			maxDepth:    DefaultMaxExpansionDepth,
			wantErr:     true,
			errContains: "MaxVariables",
		},
		{
			name:        "zero max expansion depth",
			maxSize:     DefaultMaxFileSize,
			maxLineLen:  DefaultMaxLineLength,
			maxKeyLen:   DefaultMaxKeyLength,
			maxValLen:   DefaultMaxValueLength,
			maxVars:     DefaultMaxVariables,
			maxDepth:    0,
			wantErr:     true,
			errContains: "MaxExpansionDepth",
		},
		{
			name:        "exceeds hard max expansion depth",
			maxSize:     DefaultMaxFileSize,
			maxLineLen:  DefaultMaxLineLength,
			maxKeyLen:   DefaultMaxKeyLength,
			maxValLen:   DefaultMaxValueLength,
			maxVars:     DefaultMaxVariables,
			maxDepth:    50,
			wantErr:     true,
			errContains: "MaxExpansionDepth",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConfigLimits(tt.maxSize, tt.maxLineLen, tt.maxKeyLen, tt.maxValLen, tt.maxVars, tt.maxDepth)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateConfigLimits() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil {
				var valErr *ValidationError
				if !errors.As(err, &valErr) {
					t.Errorf("validateConfigLimits() error type = %T, want *ValidationError", err)
				} else if tt.errContains != "" && valErr.Field != tt.errContains {
					t.Errorf("validateConfigLimits() error field = %s, want %s", valErr.Field, tt.errContains)
				}
			}
		})
	}
}

// ============================================================================
// newValidationError Tests
// ============================================================================

func TestNewValidationError(t *testing.T) {
	err := newValidationError("TestField", "test_value", "test_rule", "test message")

	if err.Field != "TestField" {
		t.Errorf("Field = %q, want %q", err.Field, "TestField")
	}
	if err.Rule != "test_rule" {
		t.Errorf("Rule = %q, want %q", err.Rule, "test_rule")
	}
	if err.Message != "test message" {
		t.Errorf("Message = %q, want %q", err.Message, "test message")
	}
}

// ============================================================================
// Config with Custom FileSystem Tests
// ============================================================================

func TestConfig_WithCustomFileSystem(t *testing.T) {
	fs := newTestFileSystem()
	fs.files[".env"] = "KEY=value"

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

	if loader.GetString("KEY") != "value" {
		t.Errorf("GetString(\"KEY\") = %q, want %q", loader.GetString("KEY"), "value")
	}
}

// ============================================================================
// Config with AuditHandler Tests
// ============================================================================

func TestConfig_WithAuditHandler(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AuditEnabled = true
	cfg.AuditHandler = NewNopAuditHandler()

	loader, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer loader.Close()

	// Should not error with audit handler configured
	if err := loader.Set("KEY", "value"); err != nil {
		t.Errorf("Set() error = %v", err)
	}
}

// ============================================================================
// Config with AllowedKeys Tests
// ============================================================================

func TestConfig_WithAllowedKeys(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AllowedKeys = []string{"ALLOWED_KEY"}

	loader, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer loader.Close()

	t.Run("allowed key", func(t *testing.T) {
		if err := loader.Set("ALLOWED_KEY", "value"); err != nil {
			t.Errorf("Set() allowed key error = %v", err)
		}
	})

	t.Run("non-allowed key", func(t *testing.T) {
		if err := loader.Set("NOT_ALLOWED", "value"); err == nil {
			t.Error("Set() should fail with non-allowed key")
		}
	})
}

// ============================================================================
// Config with ForbiddenKeys Tests
// ============================================================================

func TestConfig_WithForbiddenKeys(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ForbiddenKeys = []string{"FORBIDDEN_KEY"}
	cfg.OverwriteExisting = true

	loader, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer loader.Close()

	if err := loader.Set("FORBIDDEN_KEY", "value"); err == nil {
		t.Error("Set() should fail with forbidden key")
	}
}

// ============================================================================
// Config with RequiredKeys Tests
// ============================================================================

func TestConfig_WithRequiredKeys(t *testing.T) {
	cfg := DefaultConfig()
	cfg.RequiredKeys = []string{"REQUIRED_KEY"}

	loader, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer loader.Close()

	t.Run("validate without required key", func(t *testing.T) {
		if err := loader.Validate(); err == nil {
			t.Error("Validate() should fail without required key")
		}
	})

	t.Run("validate with required key", func(t *testing.T) {
		if err := loader.Set("REQUIRED_KEY", "value"); err != nil {
			t.Fatalf("Set() error = %v", err)
		}
		if err := loader.Validate(); err != nil {
			t.Errorf("Validate() error = %v", err)
		}
	})
}

// ============================================================================
// Config OverwriteExisting Tests
// ============================================================================

func TestConfig_OverwriteExisting(t *testing.T) {
	t.Run("overwrite disabled", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.OverwriteExisting = false

		loader, err := New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer loader.Close()

		if err := loader.Set("KEY", "value1"); err != nil {
			t.Fatalf("Set() error = %v", err)
		}
		if err := loader.Set("KEY", "value2"); err != nil {
			t.Fatalf("Set() error = %v", err)
		}

		// Value should not change
		if loader.GetString("KEY") != "value1" {
			t.Errorf("GetString(\"KEY\") = %q, want %q", loader.GetString("KEY"), "value1")
		}
	})

	t.Run("overwrite enabled", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.OverwriteExisting = true

		loader, err := New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer loader.Close()

		if err := loader.Set("KEY", "value1"); err != nil {
			t.Fatalf("Set() error = %v", err)
		}
		if err := loader.Set("KEY", "value2"); err != nil {
			t.Fatalf("Set() error = %v", err)
		}

		// Value should change
		if loader.GetString("KEY") != "value2" {
			t.Errorf("GetString(\"KEY\") = %q, want %q", loader.GetString("KEY"), "value2")
		}
	})
}
