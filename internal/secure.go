// Package internal provides secure io.Reader implementations with security limits.
package internal

import (
	"bytes"
	"io"
	"math"
)

// SecureReader wraps an io.Reader with security limits.
// It enforces maximum size and line length constraints.
type SecureReader struct {
	reader     io.Reader
	maxSize    int64
	maxLineLen int
	totalRead  int64
	lineRead   int
	err        error
}

// NewSecureReader creates a new SecureReader with the specified limits.
func NewSecureReader(r io.Reader, maxSize int64, maxLineLen int) *SecureReader {
	if maxSize > HardMaxFileSize {
		maxSize = HardMaxFileSize
	}
	if maxLineLen > HardMaxLineLength {
		maxLineLen = HardMaxLineLength
	}
	return &SecureReader{
		reader:     r,
		maxSize:    maxSize,
		maxLineLen: maxLineLen,
	}
}

// Read implements io.Reader with security checks.
func (r *SecureReader) Read(p []byte) (n int, err error) {
	// Return any previous error
	if r.err != nil {
		return 0, r.err
	}

	// Check total size limit before reading
	if r.totalRead >= r.maxSize {
		r.err = ErrFileTooLarge
		return 0, r.err
	}

	// Limit the read to not exceed max size
	maxRead := r.maxSize - r.totalRead
	if int64(len(p)) > maxRead {
		// Safe type conversion: maxRead is bounded by HardMaxFileSize (100MB)
		// which fits within int32/int64 on all platforms
		if maxRead > math.MaxInt {
			maxRead = math.MaxInt
		}
		p = p[:int(maxRead)]
	}

	n, err = r.reader.Read(p)
	r.totalRead += int64(n)

	// Check line length limits (skip if maxLineLen is 0, meaning no limit)
	// Optimized: use bytes.IndexByte for SIMD-accelerated newline search
	if r.maxLineLen > 0 && n > 0 {
		lineRead := r.lineRead
		maxLineLen := r.maxLineLen

		// Process data in segments between newlines using IndexByte
		// IndexByte is SIMD-optimized on most platforms
		remaining := p[:n]
		offset := 0

		for {
			newlineIdx := bytes.IndexByte(remaining, '\n')
			if newlineIdx == -1 {
				// No newline found - check remaining segment
				if lineRead+len(remaining) > maxLineLen {
					r.lineRead = lineRead + len(remaining)
					r.err = ErrLineTooLong
					return n, r.err
				}
				lineRead += len(remaining)
				break
			}

			// Check segment before newline
			if lineRead+newlineIdx > maxLineLen {
				r.lineRead = lineRead + newlineIdx
				r.err = ErrLineTooLong
				return n, r.err
			}

			// Reset line counter after newline and advance
			lineRead = 0
			offset += newlineIdx + 1
			remaining = remaining[newlineIdx+1:]
		}
		r.lineRead = lineRead
	}

	// Check if we've reached the limit and there's more data
	// Use the returned error from the underlying reader to detect EOF
	if r.totalRead >= r.maxSize && err == nil {
		r.err = ErrFileTooLarge
		return n, r.err
	}

	return n, err
}
