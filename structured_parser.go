package env

import (
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/cybergodev/env/internal"
)

// readerBufferPool provides reusable read buffers for structured parsers.
// io.ReadAll starts at 512 bytes and doubles, which costs ~a dozen growth
// allocations for typical config files; starting from a pooled 64KB buffer
// removes that churn entirely.
var readerBufferPool = sync.Pool{
	New: func() any {
		buf := make([]byte, 0, 64*1024)
		return &buf
	},
}

// maxPooledReaderBufferSize caps the buffers kept in readerBufferPool so a
// single huge file cannot inflate every pooled buffer permanently.
const maxPooledReaderBufferSize = 1024 * 1024

// getReaderBuffer retrieves a pooled read buffer.
func getReaderBuffer() *[]byte {
	buf, ok := readerBufferPool.Get().(*[]byte)
	if !ok {
		b := make([]byte, 0, 64*1024)
		return &b
	}
	return buf
}

// putReaderBuffer clears and returns a read buffer to the pool.
// SECURITY: the buffer holds raw (unmasked) file contents, which may include
// secrets — clear it before pooling, mirroring putScannerBuffer.
func putReaderBuffer(buf *[]byte) {
	if buf == nil {
		return
	}
	clear(*buf)
	if cap(*buf) <= maxPooledReaderBufferSize {
		readerBufferPool.Put(buf)
	}
}

// readAllInto reads r fully into buf's capacity, growing it as needed, and
// returns the accumulated slice. It is io.ReadAll seeded with a pooled buffer.
// On error it returns the partially-filled buffer alongside the error: the
// bytes read so far live in the caller's pooled array and must be cleared
// before that buffer is reused (see putReaderBuffer).
func readAllInto(buf []byte, r io.Reader) ([]byte, error) {
	for {
		if len(buf) == cap(buf) {
			// Grow: double the capacity, starting from a sane floor.
			newCap := 2 * cap(buf)
			if newCap == 0 {
				newCap = 512
			}
			grown := make([]byte, len(buf), newCap)
			copy(grown, buf)
			buf = grown
		}
		n, err := r.Read(buf[len(buf):cap(buf)])
		buf = buf[:len(buf)+n]
		if err != nil {
			if err == io.EOF {
				return buf, nil
			}
			return buf, err
		}
	}
}

// structuredParserConfig holds common configuration for structured file parsers (JSON/YAML).
type structuredParserConfig struct {
	config    Config
	validator Validator
	auditor   FullAuditLogger
}

// structuredParseResult wraps common SecureReader + validation logic for JSON and YAML parsers.
// flattenFn receives raw bytes and returns the flattened key-value map.
func (c *structuredParserConfig) structuredParseResult(
	r io.Reader, filename, formatName string,
	flattenFn func(data []byte) (map[string]string, error),
) (map[string]string, error) {
	// Only record start time when audit is enabled — avoids time.Now() syscall
	// in the common (audit-disabled) case, consistent with the .env parser.
	var start time.Time
	if c.config.AuditEnabled {
		start = time.Now()
	}

	secureRd := internal.NewSecureReader(r, c.config.MaxFileSize, c.config.MaxLineLength)
	bufPtr := getReaderBuffer()
	data, err := readAllInto((*bufPtr)[:0], secureRd)
	// Track the (possibly grown, possibly partially-filled) slice back through
	// the pool pointer on BOTH paths, so the deferred putReaderBuffer clears
	// and pools the full buffer. On error, data holds the bytes read before
	// the failure — raw, unmasked file contents. The previous error path
	// pooled the untouched original pointer instead, whose zero length made
	// clear() a no-op and left those secrets resident in the pool's array.
	*bufPtr = data
	defer putReaderBuffer(bufPtr)
	if err != nil {
		if errors.Is(err, internal.ErrFileTooLarge) || errors.Is(err, internal.ErrLineTooLong) {
			_ = c.auditor.LogError(internal.ActionParse, "", "file exceeds size limit")
			return nil, &FileError{Path: filename, Op: "size_check", Err: err}
		}
		return nil, err
	}

	result, err := flattenFn(data)
	if err != nil {
		_ = c.auditor.LogError(internal.ActionParse, "", "invalid "+formatName)
		return nil, err
	}

	if err := c.validateResult(result, formatName); err != nil {
		return nil, err
	}

	if c.config.AuditEnabled {
		_ = c.auditor.LogWithDuration(internal.ActionParse, "", "parsed "+formatName+": "+filename, true, time.Since(start))
	}
	return result, nil
}

// keyPolicyValidator is an optional capability of validators that can check a
// key against the allowed-keys / forbidden-keys policy independently of the
// format-specific key pattern. It is defined here (consumer side) so existing
// external Validator implementations keep compiling; they simply opt out of
// the structured-format policy check.
type keyPolicyValidator interface {
	ValidateKeyPolicy(key string) error
}

// validateResult validates parsed key-value pairs from structured files (JSON/YAML).
func (c *structuredParserConfig) validateResult(result map[string]string, format string) error {
	if len(result) > c.config.MaxVariables {
		_ = c.auditor.LogError(internal.ActionParse, "", "maximum variables exceeded")
		return &ValidationError{
			Field:   "variables",
			Rule:    "max_variables",
			Message: fmt.Sprintf("exceeded maximum of %d variables", c.config.MaxVariables),
		}
	}

	// Structured formats use their own key pattern (IsValidJSONKey) but must
	// still enforce the allowed/forbidden key policy, mirroring the .env
	// parser's ValidateKey. Without this, a JSON/YAML file could set PATH or
	// LD_PRELOAD and have it applied to the process environment.
	policy, hasPolicy := c.validator.(keyPolicyValidator)

	for key, val := range result {
		// Enforce MaxKeyLength for structured formats, consistent with the
		// .env parser's ValidateKey which checks length first.
		if len(key) > c.config.MaxKeyLength {
			_ = c.auditor.LogError(internal.ActionParse, key, "key exceeds maximum length")
			return &ValidationError{
				Field:   "key",
				Value:   MaskSensitiveInString(key),
				Rule:    "max_length",
				Message: "key exceeds maximum length",
			}
		}
		if !internal.IsValidJSONKey(key) {
			_ = c.auditor.LogError(internal.ActionParse, key, "key does not match "+format+" key pattern")
			return &ValidationError{
				Field:   "key",
				Value:   MaskSensitiveInString(key),
				Rule:    "pattern",
				Message: "key does not match required pattern",
			}
		}
		if hasPolicy {
			if err := policy.ValidateKeyPolicy(key); err != nil {
				_ = c.auditor.LogError(internal.ActionParse, key, "key rejected by policy")
				return err
			}
		}
		if c.config.ValidateValues {
			if err := c.validator.ValidateValue(val); err != nil {
				_ = c.auditor.LogError(internal.ActionParse, key, err.Error())
				return err
			}
		}
	}

	return validateRequiredKeys(c.validator, c.auditor, result)
}

// validateRequiredKeys is the shared required-key validation logic used by
// both the .env parser and the structured (JSON/YAML) parsers. It skips the
// uppercase-key index entirely when no required keys are configured.
func validateRequiredKeys(validator Validator, auditor FullAuditLogger, result map[string]string) error {
	if !needsRequiredCheck(validator) {
		return nil
	}
	upperKeys := internal.KeysToUpperPooled(result)
	err := validator.ValidateRequired(upperKeys)
	internal.PutKeysToUpperMap(upperKeys)
	if err != nil {
		_ = auditor.LogError(internal.ActionValidate, "", err.Error())
		return err
	}
	return nil
}
