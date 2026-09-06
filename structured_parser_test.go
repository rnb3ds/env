package env

import (
	"bytes"
	"errors"
	"io"
	"strconv"
	"strings"
	"testing"
	"testing/iotest"
)

// This file exercises structuredParserConfig.validateResult (structured_parser.go),
// the shared validation gate used by both the JSON and YAML parsers. The function
// is otherwise only reached on happy paths by TestJSONParser_EdgeCases and
// TestYAMLParser_EdgeCases, leaving three of its four branches and the
// required-keys path uncovered. Each case below drives a real LoadFiles parse so
// the behavior is verified end-to-end through the public API.

// makeManyYAML returns a YAML document with n distinct scalar keys, used to
// exceed a small MaxVariables cap.
func makeManyYAML(n int) string {
	var sb strings.Builder
	for i := range n {
		sb.WriteString("KEY_")
		sb.WriteString(strconv.Itoa(i))
		sb.WriteString(": v\n")
	}
	return sb.String()
}

func TestStructuredParser_ValidateResult(t *testing.T) {
	tests := []struct {
		name      string
		filename  string
		content   string
		configFn  func(*Config)
		wantErr   bool
		wantField string // expected ValidationError.Field; "" skips the field check
	}{
		{
			name:      "yaml max variables exceeded",
			filename:  "many.yaml",
			content:   makeManyYAML(50),
			configFn:  func(c *Config) { c.MaxVariables = 10 },
			wantErr:   true,
			wantField: "variables",
		},
		{
			name:      "json invalid key character rejected",
			filename:  "badkey.json",
			content:   `{"KEY!": "v"}`,
			wantErr:   true,
			wantField: "key",
		},
		{
			name:      "yaml invalid key (space) rejected",
			filename:  "badkey.yaml",
			content:   "\"bad key\": v\n",
			wantErr:   true,
			wantField: "key",
		},
		{
			// A value longer than MaxValueLength exercises the ValidateValues ->
			// ValidateValue error path. (DefaultConfig already sets
			// ValidateValues=true; we only shrink the length cap.)
			name:      "json value exceeds MaxValueLength",
			filename:  "badval.json",
			content:   `{"K": "abcdefghij"}`,
			configFn:  func(c *Config) { c.MaxValueLength = 3 },
			wantErr:   true,
			wantField: "value",
		},
		{
			name:     "json required key missing",
			filename: "reqmissing.json",
			content:  `{"PRESENT": "v"}`,
			configFn: func(c *Config) { c.RequiredKeys = []string{"MISSING"} },
			wantErr:  true,
		},
		{
			name:     "json required key present succeeds",
			filename: "reqok.json",
			content:  `{"REQUIRED": "v"}`,
			configFn: func(c *Config) { c.RequiredKeys = []string{"REQUIRED"} },
			wantErr:  false,
		},
		{
			name:     "yaml happy path parses cleanly",
			filename: "ok.yaml",
			content:  "NAME: env\nPORT: \"8080\"\n",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			fs := newTestFileSystem()
			fs.files[tt.filename] = tt.content
			cfg.FileSystem = fs
			if tt.configFn != nil {
				tt.configFn(&cfg)
			}

			loader, err := New(cfg)
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}
			defer loader.Close()

			err = loader.LoadFiles(tt.filename)
			if tt.wantErr {
				if err == nil {
					t.Fatal("LoadFiles() error = nil, want a validation error")
				}
				if tt.wantField != "" {
					var ve *ValidationError
					if !errors.As(err, &ve) {
						t.Errorf("error is not a *ValidationError: %T (%v)", err, err)
					} else if ve.Field != tt.wantField {
						t.Errorf("ValidationError.Field = %q, want %q", ve.Field, tt.wantField)
					}
				}
				return
			}
			if err != nil {
				t.Errorf("LoadFiles() error = %v, want nil", err)
			}
		})
	}
}

// TestReadAllInto covers the buffer-growth and read-error paths of the
// structured-parser file reader directly.
func TestReadAllInto(t *testing.T) {
	t.Run("grows buffer beyond initial capacity", func(t *testing.T) {
		content := strings.Repeat("x", 2048)
		buf, err := readAllInto(make([]byte, 0, 64), strings.NewReader(content))
		if err != nil {
			t.Fatalf("readAllInto() error = %v", err)
		}
		if string(buf) != content {
			t.Errorf("readAllInto() read %d bytes, want %d", len(buf), len(content))
		}
	})

	t.Run("read error is returned", func(t *testing.T) {
		wantErr := errors.New("disk failure")
		_, err := readAllInto(make([]byte, 0, 64), io.MultiReader(strings.NewReader("partial"), iotest.ErrReader(wantErr)))
		if !errors.Is(err, wantErr) {
			t.Errorf("readAllInto() error = %v, want %v", err, wantErr)
		}
	})

	t.Run("empty reader returns empty buffer", func(t *testing.T) {
		buf, err := readAllInto(make([]byte, 0, 64), strings.NewReader(""))
		if err != nil {
			t.Fatalf("readAllInto() error = %v", err)
		}
		if len(buf) != 0 {
			t.Errorf("readAllInto() = %d bytes, want 0", len(buf))
		}
	})

	t.Run("zero-capacity buffer grows from the 512-byte floor", func(t *testing.T) {
		buf, err := readAllInto(make([]byte, 0), strings.NewReader("hello"))
		if err != nil {
			t.Fatalf("readAllInto() error = %v", err)
		}
		if string(buf) != "hello" {
			t.Errorf("readAllInto() = %q, want %q", buf, "hello")
		}
	})
}

// TestReaderBufferPool_Boundary covers the defensive paths of the structured
// parser's reader buffer pool: nil puts, oversized buffers refused entry, and
// the fallback allocation when the pool returns an unexpected type.
func TestReaderBufferPool_Boundary(t *testing.T) {
	t.Run("nil buffer is ignored", func(t *testing.T) {
		putReaderBuffer(nil) // must not panic
	})

	t.Run("oversized buffer is not pooled", func(t *testing.T) {
		big := make([]byte, 0, maxPooledReaderBufferSize+1)
		putReaderBuffer(&big)
		// No observable assertion: the contract is that the oversized buffer is
		// simply dropped, and the pool's buffers stay ≤ maxPooledReaderBufferSize.
	})

	t.Run("unexpected pool type falls back to a fresh buffer", func(t *testing.T) {
		readerBufferPool.Put(new(int)) // poison the pool with a foreign type
		buf := getReaderBuffer()
		if cap(*buf) != 64*1024 {
			t.Errorf("fallback buffer capacity = %d, want %d", cap(*buf), 64*1024)
		}
		putReaderBuffer(buf) // restore a well-typed buffer to the pool
	})
}

// TestStructuredParseResult_Boundaries drives the remaining structured-parse
// branches directly: generic (non-limit) read errors passed through unwrapped,
// MaxKeyLength enforcement, and the audit-enabled duration log on success.
func TestStructuredParseResult_Boundaries(t *testing.T) {
	t.Run("generic read error is returned as-is", func(t *testing.T) {
		wantErr := errors.New("device not ready")
		c := &structuredParserConfig{config: DefaultConfig(), auditor: &mockFullAuditLogger{}}

		_, err := c.structuredParseResult(&partialReader{data: []byte("{"), err: wantErr}, "f.json", "JSON",
			func([]byte) (map[string]string, error) { return nil, nil })
		if !errors.Is(err, wantErr) {
			t.Errorf("structuredParseResult() error = %v, want the raw %v", err, wantErr)
		}
	})

	t.Run("key exceeding MaxKeyLength is rejected", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.MaxKeyLength = 4
		c := &structuredParserConfig{config: cfg, auditor: &mockFullAuditLogger{}}

		_, err := c.structuredParseResult(strings.NewReader(`{}`), "f.json", "JSON",
			func([]byte) (map[string]string, error) { return map[string]string{"TOOLONGKEY": "v"}, nil })
		var ve *ValidationError
		if !errors.As(err, &ve) || ve.Rule != "max_length" {
			t.Errorf("structuredParseResult() error = %v, want ValidationError rule max_length", err)
		}
	})

	t.Run("audit enabled logs parse duration on success", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.AuditEnabled = true
		factory := cfg.buildComponentFactory()
		defer factory.Close()
		auditor := &mockFullAuditLogger{}
		c := &structuredParserConfig{config: cfg, validator: factory.Validator(), auditor: auditor}

		result, err := c.structuredParseResult(strings.NewReader(`{"K":"v"}`), "f.json", "JSON",
			func(data []byte) (map[string]string, error) { return map[string]string{"K": "v"}, nil })
		if err != nil {
			t.Fatalf("structuredParseResult() error = %v", err)
		}
		if result["K"] != "v" {
			t.Errorf("result[K] = %q, want %q", result["K"], "v")
		}
		found := false
		for _, entry := range auditor.logs {
			if entry == "LogWithDuration" {
				found = true
			}
		}
		if !found {
			t.Errorf("audit log entries = %v, want a LogWithDuration entry", auditor.logs)
		}
	})
}

// partialReader yields the given bytes once, then fails with a custom error.
type partialReader struct {
	data []byte
	err  error
	done bool
}

func (r *partialReader) Read(p []byte) (int, error) {
	if r.done {
		return 0, r.err
	}
	r.done = true
	n := copy(p, r.data)
	return n, r.err
}

// TestReadAllIntoReturnsPartialBufferOnError pins the readAllInto contract the
// structured-parse error path relies on: on error it must return the
// partially-filled buffer, not nil, so the caller can clear it before pooling.
// Regression: the error path in structuredParseResult previously pooled the
// untouched original pointer, whose zero length made clear() a no-op and left
// raw file contents resident in readerBufferPool.
func TestReadAllIntoReturnsPartialBufferOnError(t *testing.T) {
	secret := []byte("PASSWORD=supersecret-value")
	wantErr := errors.New("read failed")
	buf := make([]byte, 0, 64)

	data, err := readAllInto(buf, &partialReader{data: secret, err: wantErr})
	if !errors.Is(err, wantErr) {
		t.Fatalf("readAllInto() error = %v, want %v", err, wantErr)
	}
	if data == nil {
		t.Fatal("readAllInto() returned nil buffer on error; caller cannot clear pooled memory")
	}
	if string(data) != string(secret) {
		t.Errorf("partial data = %q, want %q", data, secret)
	}
}

// TestStructuredParseErrorPathClearsPooledBuffer verifies end to end that a
// failing structured parse leaves no raw file contents in the reader buffer
// pool: after the error, the next pooled buffer contains only zeroes.
func TestStructuredParseErrorPathClearsPooledBuffer(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxFileSize = 32
	cfg.MaxLineLength = 1024

	// Content longer than MaxFileSize so the read fails mid-file with
	// ErrFileTooLarge after ~32 bytes of secret-bearing content.
	oversized := []byte("PASSWORD=supersecret-payload-that-exceeds-the-limit")
	c := &structuredParserConfig{
		config:  cfg,
		auditor: &mockFullAuditLogger{},
	}
	flattenStub := func([]byte) (map[string]string, error) { return nil, nil }

	if _, err := c.structuredParseResult(bytes.NewReader(oversized), "test.json", "JSON", flattenStub); err == nil {
		t.Fatal("expected ErrFileTooLarge from oversized input")
	}

	// The buffer just returned to the pool must be fully zeroed.
	next := getReaderBuffer()
	for i, b := range *next {
		if b != 0 {
			t.Fatalf("pooled buffer dirty at byte %d after read error; raw file contents would persist in the pool", i)
		}
	}
}
