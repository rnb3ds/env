// Package internal provides test utilities for pools.

package internal

import (
	"testing"
)

// ============================================================================
// Lexer Buffer Pool Tests
// ============================================================================

func TestLexerBufferPool_BasicUsage(t *testing.T) {
	buf := getLexerBuffer()
	if buf == nil {
		t.Fatal("getLexerBuffer() returned nil")
	}

	// Buffer should be usable
	buf.WriteString("test data")
	if buf.Len() != 9 {
		t.Errorf("Buffer length = %d, want 9", buf.Len())
	}

	// Return to pool
	putLexerBuffer(buf)

	// Get another buffer - should be reset
	buf2 := getLexerBuffer()
	if buf2.Len() != 0 {
		t.Errorf("Pooled buffer length = %d, want 0", buf2.Len())
	}
	putLexerBuffer(buf2)
}

func TestLexerBufferPool_NilSafety(t *testing.T) {
	// Should not panic
	putLexerBuffer(nil)
}

func TestLexerBufferPool_LargeBuffer(t *testing.T) {
	// Create a buffer that exceeds max size
	buf := getLexerBuffer()
	largeData := make([]byte, 4097)
	buf.Write(largeData)

	if buf.Len() != 4097 {
		t.Errorf("Buffer length = %d, want 4097", buf.Len())
	}

	// Put back to pool - should be discarded
	putLexerBuffer(buf)

	// Next get should still work
	buf2 := getLexerBuffer()
	if buf2 == nil {
		t.Error("getLexerBuffer() returned nil after discarding large buffer")
	}
	putLexerBuffer(buf2)
}

func TestLexerBufferPool_ConcurrentAccess(t *testing.T) {
	// Test concurrent access to the lexer buffer pool
	done := make(chan bool, 100)

	for i := 0; i < 100; i++ {
		go func() {
			for j := 0; j < 10; j++ {
				buf := getLexerBuffer()
				buf.WriteString("concurrent test")
				putLexerBuffer(buf)
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 100; i++ {
		<-done
	}
}

func TestLexerBufferPool_NoLeakOnMultipleReturns(t *testing.T) {
	// Test that returning the buffer multiple times doesn't cause issues
	buf := getLexerBuffer()
	buf.WriteString("test1")

	// Return the same buffer multiple times (should be safe)
	putLexerBuffer(buf)
	putLexerBuffer(buf) // Second return should be safe
	putLexerBuffer(buf)

	// Get a new buffer - should work fine
	buf2 := getLexerBuffer()
	if buf2 == nil {
		t.Error("getLexerBuffer() returned nil after multiple returns")
	}
	buf2.WriteString("test2")
	putLexerBuffer(buf2)
}
