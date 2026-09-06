package env

// Regression tests for the SEC-001 security audit fixes (2026-09-06),
// root-package surface. Internal-package counterparts live in
// internal/sec_regression_test.go.

import (
	"errors"
	"os"
	"regexp"
	"strings"
	"testing"
)

// TestSEC01_UnmarshalIntoDoesNotLeakValue pins the public contract: a
// conversion failure during struct mapping must not embed the raw value in
// the error message (values are frequently secrets).
func TestSEC01_UnmarshalIntoDoesNotLeakValue(t *testing.T) {
	const secret = "s3cr3t-PIN-9876"
	type Cfg struct {
		Pin int `env:"DB_PASSWORD"`
	}
	err := UnmarshalInto(map[string]string{"DB_PASSWORD": secret}, &Cfg{})
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), secret) {
		t.Errorf("raw value leaked: %v", err)
	}
	if !IsMarshalError(err) {
		t.Errorf("want MarshalError, got %T: %v", err, err)
	}
	var ve *ValidationError
	if errors.As(err, &ve) {
		t.Error("must not be a ValidationError carrying the value in .Value")
	}
}

// TestSEC02_UnmarshalMapYAMLTokenCap exercises the YAML token cap through
// the public UnmarshalMap path.
func TestSEC02_UnmarshalMapYAMLTokenCap(t *testing.T) {
	input := strings.Repeat("a\n", 100001) // 200002 tokens > HardMaxYAMLTokens
	_, err := UnmarshalMap(input, FormatYAML)
	if err == nil {
		t.Fatal("expected token cap error")
	}
	if !strings.Contains(err.Error(), "token count") {
		t.Errorf("error should mention token count: %v", err)
	}
}

// TestSEC04_MaskSensitiveInStringMasksPatterns pins the exported masking
// helper: key=value secret pairs are masked, not merely truncated.
func TestSEC04_MaskSensitiveInStringMasksPatterns(t *testing.T) {
	if got := MaskSensitiveInString("password=hunter2"); got != "password=[MASKED]" {
		t.Errorf("MaskSensitiveInString = %q, want password=[MASKED]", got)
	}
}

// TestSEC03_ExpansionFileOnly verifies the ExpansionFileOnly scope:
// file-local references expand normally, while references that would fall
// back to the process environment expand to empty.
func TestSEC03_ExpansionFileOnly(t *testing.T) {
	t.Setenv("SECRET_PROCESS_TOKEN", "topsecret-xyz") // auto-restored after the test

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldWd) }() // best-effort restore

	content := "FOO=file-local\nLOCAL_REF=${FOO}\nLEAK=${SECRET_PROCESS_TOKEN}\n"
	if err := os.WriteFile(".env.sec03", []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := DefaultConfig()
	cfg.Filenames = []string{".env.sec03"}
	cfg.ExpansionScope = ExpansionFileOnly
	loader, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer loader.Close()

	if v, _ := loader.Lookup("LOCAL_REF"); v != "file-local" {
		t.Errorf("LOCAL_REF = %q, want %q: file-local expansion must keep working", v, "file-local")
	}
	if v, _ := loader.Lookup("LEAK"); v != "" {
		t.Errorf("LEAK = %q, want empty: file-only scope must not read the process environment", v)
	}
}

// TestSEC03_ExpansionFileThenProcessDefault pins the default scope:
// expansion still falls back to the process environment (dotenv semantics)
// unless the user opts into ExpansionFileOnly.
func TestSEC03_ExpansionFileThenProcessDefault(t *testing.T) {
	t.Setenv("SEC03_PROCESS_VAR", "from-process")

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldWd) }() // best-effort restore

	if err := os.WriteFile(".env.sec03", []byte("REF=${SEC03_PROCESS_VAR}\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := DefaultConfig()
	cfg.Filenames = []string{".env.sec03"}
	loader, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer loader.Close()

	if v, _ := loader.Lookup("REF"); v != "from-process" {
		t.Errorf("REF = %q, want %q (default scope keeps dotenv semantics)", v, "from-process")
	}
}

// TestSEC06_KeyPatternRejectsInjectionEnablers verifies custom key patterns
// that accept separator/newline characters fail Config validation — such
// keys previously enabled .env round-trip injection via Marshal.
func TestSEC06_KeyPatternRejectsInjectionEnablers(t *testing.T) {
	for _, pattern := range []*regexp.Regexp{
		regexp.MustCompile(`^[A-Za-z](?s:.)*$`), // allows newline, '=', ':'
		regexp.MustCompile(`^[\w=]+$`),          // allows '='
		regexp.MustCompile(`^[\w:]+$`),          // allows ':'
	} {
		cfg := DefaultConfig()
		cfg.KeyPattern = pattern
		if err := cfg.Validate(); err == nil {
			t.Errorf("pattern %q should be rejected", pattern)
		}
	}

	// A safe custom pattern is still accepted.
	cfg := DefaultConfig()
	cfg.KeyPattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]+$`)
	if err := cfg.Validate(); err != nil {
		t.Errorf("safe pattern rejected: %v", err)
	}
}

// TestSEC06_MarshalRejectsInjectionKeys verifies Marshal refuses to emit
// keys that would re-parse as different lines (the injection primitive).
func TestSEC06_MarshalRejectsInjectionKeys(t *testing.T) {
	if _, err := Marshal(map[string]string{"A\nB=INJECTED": "1"}); err == nil {
		t.Error("Marshal(.env) accepted a key with an embedded newline")
	}
	if _, err := Marshal(map[string]string{"A\nB": "1"}, FormatYAML); err == nil {
		t.Error("Marshal(YAML) accepted a key with an embedded newline")
	}
	if _, err := Marshal(map[string]string{"GOOD_KEY": "v"}); err != nil {
		t.Errorf("Marshal rejected a safe key: %v", err)
	}
}

// TestSEC03_FileOnlyScopeNotTreatedAsZeroConfig pins the IsZero fix: a Config
// whose only non-default field is ExpansionScope used to be detected as a
// zero-value Config and silently replaced by DefaultConfig() inside New(),
// dropping the file-only expansion restriction without any error.
func TestSEC03_FileOnlyScopeNotTreatedAsZeroConfig(t *testing.T) {
	cfg := Config{ParsingConfig: ParsingConfig{ExpansionScope: ExpansionFileOnly}}
	if cfg.IsZero() {
		t.Fatal("Config with only ExpansionScope set must not be detected as zero — " +
			"New() would silently replace it with DefaultConfig() and drop the SEC-03 restriction")
	}

	// A partially-initialized Config now fails validation loudly, like any
	// other partially-filled Config, instead of being swapped for the default.
	if _, err := New(cfg); err == nil {
		t.Fatal("expected validation error for partially-initialized config, got nil")
	}
}
