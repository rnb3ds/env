package internal

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"testing"
	"time"
)

func TestNewAuditor(t *testing.T) {
	// Test with nil handler
	a := NewAuditor(nil, nil, nil, false)
	if a == nil {
		t.Error("NewAuditor() should not return nil")
	}

	// Test with enabled
	a = NewAuditor(nil, nil, nil, true)
	if !a.enabled {
		t.Error("auditor should be enabled")
	}
}

func TestAuditorLogDisabled(t *testing.T) {
	a := NewAuditor(nil, nil, nil, false)

	err := a.Log(ActionSet, "KEY", "test", true)
	if err != nil {
		t.Errorf("Log() when disabled should return nil, got %v", err)
	}
}

func TestAuditorLogEnabled(t *testing.T) {
	// Create channel handler to capture events
	ch := make(chan Event, 1)
	handler := NewChannelHandler(ch)
	a := NewAuditor(handler, nil, nil, true)

	err := a.Log(ActionSet, "KEY", "test", true)
	if err != nil {
		t.Errorf("Log() error = %v", err)
		return
	}

	select {
	case event := <-ch:
		if event.Action != ActionSet {
			t.Errorf("event action = %v, want %v", event.Action, ActionSet)
		}
		if event.Key != "KEY" {
			t.Errorf("event key = %v, want KEY", event.Key)
		}
	case <-time.After(time.Second):
		t.Error("timeout waiting for event")
	}
}

func TestAuditorLogWithMasking(t *testing.T) {
	ch := make(chan Event, 1)
	handler := NewChannelHandler(ch)

	isSensitive := func(key string) bool {
		return strings.Contains(strings.ToUpper(key), "PASSWORD")
	}
	masker := func(key, value string) string {
		if isSensitive(key) {
			return "[MASKED]"
		}
		return value
	}

	a := NewAuditor(handler, isSensitive, masker, true)

	_ = a.Log(ActionSet, "PASSWORD", "test", true)

	select {
	case event := <-ch:
		if !event.Masked {
			t.Error("sensitive key should be marked as masked")
		}
	case <-time.After(time.Second):
		t.Error("timeout waiting for event")
	}
}

func TestAuditorLogWithFile(t *testing.T) {
	ch := make(chan Event, 1)
	handler := NewChannelHandler(ch)
	a := NewAuditor(handler, nil, nil, true)

	err := a.LogWithFile(ActionLoad, "", ".env", "loaded", true)
	if err != nil {
		t.Errorf("LogWithFile() error = %v", err)
		return
	}

	select {
	case event := <-ch:
		if event.File != ".env" {
			t.Errorf("event file = %v, want .env", event.File)
		}
	case <-time.After(time.Second):
		t.Error("timeout waiting for event")
	}
}

func TestAuditorLogWithDuration(t *testing.T) {
	ch := make(chan Event, 1)
	handler := NewChannelHandler(ch)
	a := NewAuditor(handler, nil, nil, true)

	duration := 100 * time.Millisecond
	err := a.LogWithDuration(ActionLoad, "", "test", true, duration)
	if err != nil {
		t.Errorf("LogWithDuration() error = %v", err)
		return
	}

	select {
	case event := <-ch:
		if event.Duration != duration.Nanoseconds() {
			t.Errorf("event duration = %v, want %v", event.Duration, duration.Nanoseconds())
		}
	case <-time.After(time.Second):
		t.Error("timeout waiting for event")
	}
}

func TestAuditorSetEnabled(t *testing.T) {
	a := NewAuditor(nil, nil, nil, false)

	a.SetEnabled(true)
	if !a.IsEnabled() {
		t.Error("IsEnabled() should return true after SetEnabled(true)")
	}

	a.SetEnabled(false)
	if a.IsEnabled() {
		t.Error("IsEnabled() should return false after SetEnabled(false)")
	}
}

func TestJSONHandler(t *testing.T) {
	var buf strings.Builder
	handler := NewJSONHandler(&buf)

	event := Event{
		Timestamp: time.Now(),
		Action:    ActionSet,
		Key:       "KEY",
		Reason:    "test",
		Success:   true,
	}

	err := handler.Log(event)
	if err != nil {
		t.Errorf("JSONHandler.Log() error = %v", err)
		return
	}

	// Verify output is valid JSON
	output := buf.String()
	var parsed Event
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &parsed); err != nil {
		t.Errorf("output is not valid JSON: %v\noutput: %s", err, output)
	}
}

func TestLogHandler(t *testing.T) {
	logger := log.New(os.Stdout, "[TEST] ", 0)
	handler := NewLogHandler(logger)

	event := Event{
		Timestamp: time.Now(),
		Action:    ActionSet,
		Key:       "KEY",
		Reason:    "test",
		Success:   true,
	}

	err := handler.Log(event)
	if err != nil {
		t.Errorf("LogHandler.Log() error = %v", err)
	}
}

func TestLogHandlerNilLogger(t *testing.T) {
	handler := NewLogHandler(nil)
	if handler == nil {
		t.Error("NewLogHandler(nil) should not return nil")
	}
}

func TestChannelHandler(t *testing.T) {
	ch := make(chan Event, 1)
	handler := NewChannelHandler(ch)

	event := Event{
		Timestamp: time.Now(),
		Action:    ActionSet,
		Key:       "KEY",
	}

	err := handler.Log(event)
	if err != nil {
		t.Errorf("ChannelHandler.Log() error = %v", err)
		return
	}

	select {
	case received := <-ch:
		if received.Action != ActionSet {
			t.Errorf("received action = %v, want %v", received.Action, ActionSet)
		}
	case <-time.After(time.Second):
		t.Error("timeout waiting for event")
	}
}

func TestNopHandler(t *testing.T) {
	handler := NewNopHandler()

	event := Event{Action: ActionSet}

	err := handler.Log(event)
	if err != nil {
		t.Errorf("NopHandler.Log() error = %v", err)
	}

	err = handler.Close()
	if err != nil {
		t.Errorf("NopHandler.Close() error = %v", err)
	}
}

func TestDefaultHandler(t *testing.T) {
	handler := DefaultHandler()
	if handler == nil {
		t.Error("DefaultHandler() should not return nil")
	}
}

// ============================================================================
// Close Method Tests
// ============================================================================

func TestJSONHandler_Close(t *testing.T) {
	t.Run("with closer", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "audit*.json")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		defer os.Remove(tmpFile.Name())

		handler := NewJSONHandler(tmpFile)
		if err := handler.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})

	t.Run("without closer", func(t *testing.T) {
		var buf strings.Builder
		handler := NewJSONHandler(&buf)
		// Should not error when underlying writer has no Close
		if err := handler.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})
}

func TestLogHandler_Close(t *testing.T) {
	handler := NewLogHandler(nil)
	if err := handler.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

func TestChannelHandler_Close(t *testing.T) {
	ch := make(chan Event, 1)
	handler := NewChannelHandler(ch)

	if err := handler.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

func TestAuditor_Close(t *testing.T) {
	t.Run("with handler", func(t *testing.T) {
		ch := make(chan Event, 10)
		handler := NewChannelHandler(ch)
		a := NewAuditor(handler, nil, nil, true)

		if err := a.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})

	t.Run("without handler", func(t *testing.T) {
		a := NewAuditor(nil, nil, nil, true)
		if err := a.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})
}

func TestAuditor_LogSecurity(t *testing.T) {
	t.Run("logs security event", func(t *testing.T) {
		ch := make(chan Event, 1)
		handler := NewChannelHandler(ch)
		a := NewAuditor(handler, nil, nil, true)

		err := a.LogSecurity("SENSITIVE_KEY", "forbidden key access")
		if err != nil {
			t.Errorf("LogSecurity() error = %v", err)
			return
		}

		select {
		case event := <-ch:
			if event.Action != ActionSecurity {
				t.Errorf("event action = %v, want %v", event.Action, ActionSecurity)
			}
			if event.Success {
				t.Error("security event should have success=false")
			}
			if event.Key != "SENSITIVE_KEY" {
				t.Errorf("event key = %v, want SENSITIVE_KEY", event.Key)
			}
		case <-time.After(time.Second):
			t.Error("timeout waiting for event")
		}
	})

	t.Run("disabled auditor", func(t *testing.T) {
		a := NewAuditor(nil, nil, nil, false)
		err := a.LogSecurity("KEY", "test")
		if err != nil {
			t.Errorf("LogSecurity() when disabled should return nil, got %v", err)
		}
	})
}

// ============================================================================
// BufferedHandler Tests
// ============================================================================

func TestBufferedHandler_Basic(t *testing.T) {
	// Create a channel to collect events
	ch := make(chan Event, 100)
	underlying := NewChannelHandler(ch)

	handler := NewBufferedHandler(BufferedHandlerConfig{
		Handler:       underlying,
		BufferSize:    10,
		FlushInterval: 0, // Disable auto-flush for this test
	})
	defer handler.Close()

	// Log some events
	for i := 0; i < 5; i++ {
		err := handler.Log(Event{Action: ActionSet, Key: "KEY"})
		if err != nil {
			t.Errorf("Log() error = %v", err)
		}
	}

	// Buffer should contain 5 events, not flushed yet
	if len(ch) != 0 {
		t.Errorf("expected 0 events in channel, got %d", len(ch))
	}

	// Manual flush
	if err := handler.Flush(); err != nil {
		t.Errorf("Flush() error = %v", err)
	}

	// Now we should have 5 events
	if len(ch) != 5 {
		t.Errorf("expected 5 events in channel, got %d", len(ch))
	}
}

func TestBufferedHandler_AutoFlushOnFull(t *testing.T) {
	ch := make(chan Event, 100)
	underlying := NewChannelHandler(ch)

	handler := NewBufferedHandler(BufferedHandlerConfig{
		Handler:       underlying,
		BufferSize:    5,
		FlushInterval: 0, // Disable auto-flush
	})
	defer handler.Close()

	// Log exactly BufferSize events
	for i := 0; i < 5; i++ {
		_ = handler.Log(Event{Action: ActionSet})
	}

	// Buffer should auto-flush when full
	if len(ch) != 5 {
		t.Errorf("expected 5 events after auto-flush, got %d", len(ch))
	}
}

func TestBufferedHandler_CloseFlushes(t *testing.T) {
	ch := make(chan Event, 100)
	underlying := NewChannelHandler(ch)

	handler := NewBufferedHandler(BufferedHandlerConfig{
		Handler:       underlying,
		BufferSize:    10,
		FlushInterval: 0, // Disable auto-flush
	})

	// Log some events
	for i := 0; i < 3; i++ {
		_ = handler.Log(Event{Action: ActionSet})
	}

	// Close should flush remaining events
	if err := handler.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}

	// All events should be flushed
	if len(ch) != 3 {
		t.Errorf("expected 3 events after close, got %d", len(ch))
	}

	// Second close should be idempotent
	if err := handler.Close(); err != nil {
		t.Errorf("Second Close() error = %v", err)
	}
}

func TestBufferedHandler_TimeBasedFlush(t *testing.T) {
	ch := make(chan Event, 100)
	underlying := NewChannelHandler(ch)

	handler := NewBufferedHandler(BufferedHandlerConfig{
		Handler:       underlying,
		BufferSize:    100, // Large buffer so time-based flush happens first
		FlushInterval: 50 * time.Millisecond,
	})
	defer handler.Close()

	// Log an event
	_ = handler.Log(Event{Action: ActionSet})

	// Event should not be flushed immediately
	if len(ch) != 0 {
		t.Errorf("expected 0 events immediately, got %d", len(ch))
	}

	// Wait for time-based flush
	time.Sleep(100 * time.Millisecond)

	// Event should be flushed now
	if len(ch) != 1 {
		t.Errorf("expected 1 event after time-based flush, got %d", len(ch))
	}
}

func TestBufferedHandler_BufferLen(t *testing.T) {
	underlying := NewNopHandler()
	handler := NewBufferedHandler(BufferedHandlerConfig{
		Handler:       underlying,
		BufferSize:    10,
		FlushInterval: 0,
	})
	defer handler.Close()

	if handler.BufferLen() != 0 {
		t.Errorf("initial BufferLen() = %d, want 0", handler.BufferLen())
	}

	_ = handler.Log(Event{Action: ActionSet})
	_ = handler.Log(Event{Action: ActionSet})

	if handler.BufferLen() != 2 {
		t.Errorf("BufferLen() = %d, want 2", handler.BufferLen())
	}

	_ = handler.Flush()

	if handler.BufferLen() != 0 {
		t.Errorf("BufferLen() after flush = %d, want 0", handler.BufferLen())
	}
}

func TestBufferedHandler_IsFull(t *testing.T) {
	underlying := NewNopHandler()
	handler := NewBufferedHandler(BufferedHandlerConfig{
		Handler:       underlying,
		BufferSize:    3,
		FlushInterval: 0,
	})
	defer handler.Close()

	if handler.IsFull() {
		t.Error("IsFull() = true for empty buffer")
	}

	_ = handler.Log(Event{Action: ActionSet})
	_ = handler.Log(Event{Action: ActionSet})

	if handler.IsFull() {
		t.Error("IsFull() = true for buffer with 2/3 events")
	}

	// This should trigger auto-flush
	_ = handler.Log(Event{Action: ActionSet})

	// After auto-flush, buffer should be empty
	if handler.IsFull() {
		t.Error("IsFull() = true after auto-flush")
	}
}

func TestBufferedHandler_RequestFlush(t *testing.T) {
	ch := make(chan Event, 100)
	underlying := NewChannelHandler(ch)

	handler := NewBufferedHandler(BufferedHandlerConfig{
		Handler:       underlying,
		BufferSize:    100,
		FlushInterval: 10 * time.Minute, // Long interval, rely on RequestFlush
	})
	defer handler.Close()

	_ = handler.Log(Event{Action: ActionSet})

	// Request flush
	handler.RequestFlush()

	// Wait for flush to complete
	time.Sleep(50 * time.Millisecond)

	// Event should be flushed
	if len(ch) != 1 {
		t.Errorf("expected 1 event after RequestFlush, got %d", len(ch))
	}
}

func TestBufferedHandler_OnError(t *testing.T) {
	// Create a handler that always errors
	errorHandler := &errorTestHandler{}

	var capturedError error
	handler := NewBufferedHandler(BufferedHandlerConfig{
		Handler:    errorHandler,
		BufferSize: 10,
		OnError: func(err error) {
			capturedError = err
		},
	})
	defer handler.Close()

	_ = handler.Log(Event{Action: ActionSet})
	_ = handler.Flush()

	if capturedError == nil {
		t.Error("OnError should have been called")
	}
}

// errorTestHandler always returns an error on Log
type errorTestHandler struct{}

func (h *errorTestHandler) Log(event Event) error {
	return fmt.Errorf("test error")
}

func (h *errorTestHandler) Close() error {
	return nil
}

func TestBufferedHandler_Defaults(t *testing.T) {
	// Test with minimal config
	handler := NewBufferedHandler(BufferedHandlerConfig{
		Handler: NewNopHandler(),
	})
	defer handler.Close()

	// Should use default buffer size
	if handler.size != DefaultBufferSize {
		t.Errorf("buffer size = %d, want %d", handler.size, DefaultBufferSize)
	}

	// Should use default flush interval
	if handler.interval != DefaultFlushInterval {
		t.Errorf("flush interval = %v, want %v", handler.interval, DefaultFlushInterval)
	}
}

func TestBufferedHandler_NilHandler(t *testing.T) {
	// Should not panic with nil handler
	handler := NewBufferedHandler(BufferedHandlerConfig{})
	defer handler.Close()

	// Should use NopHandler
	err := handler.Log(Event{Action: ActionSet})
	if err != nil {
		t.Errorf("Log() error = %v", err)
	}
}

func TestBufferedHandler_LogAfterClose(t *testing.T) {
	handler := NewBufferedHandler(BufferedHandlerConfig{
		Handler:       NewNopHandler(),
		FlushInterval: 0,
	})

	_ = handler.Close()

	// Should return error after close
	err := handler.Log(Event{Action: ActionSet})
	if err == nil {
		t.Error("Log() after Close() should return error")
	}
}

func TestBufferedHandler_Concurrent(t *testing.T) {
	ch := make(chan Event, 1000)
	underlying := NewChannelHandler(ch)

	handler := NewBufferedHandler(BufferedHandlerConfig{
		Handler:       underlying,
		BufferSize:    50,
		FlushInterval: 0,
	})
	defer handler.Close()

	// Concurrent logging
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 10; j++ {
				_ = handler.Log(Event{Action: ActionSet})
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Final flush
	_ = handler.Flush()

	// Should have 100 events
	if len(ch) != 100 {
		t.Errorf("expected 100 events, got %d", len(ch))
	}
}
