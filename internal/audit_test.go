package internal

import (
	"bytes"
	"encoding/json"
	"log"
	"os"
	"strings"
	"sync"
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
	if !a.enabled.Load() {
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
	tests := []struct {
		name      string
		event     Event
		wantParts []string // substrings expected in the log line
		skipParts []string // substrings that must NOT appear
	}{
		{
			name:      "event with key",
			event:     Event{Timestamp: time.Now(), Action: ActionSet, Key: "KEY", Reason: "test", Success: true},
			wantParts: []string{"action=set", "key=KEY", "success=true", `reason="test"`},
			skipParts: []string{"file=", "duration="},
		},
		{
			name:      "event without key omits key field",
			event:     Event{Timestamp: time.Now(), Action: ActionLoad, Reason: "nofile", Success: false},
			wantParts: []string{"action=load", "success=false", `reason="nofile"`},
			skipParts: []string{"key="},
		},
		{
			name:      "event with file appends file field",
			event:     Event{Timestamp: time.Now(), Action: ActionLoad, Key: "KEY", File: ".env", Success: true},
			wantParts: []string{"file=.env"},
		},
		{
			name:      "sub-millisecond duration formatted in microseconds",
			event:     Event{Timestamp: time.Now(), Action: ActionLoad, Key: "KEY", Success: true, Duration: 1500},
			wantParts: []string{"duration=1μs"},
		},
		{
			name:      "millisecond duration formatted in ms",
			event:     Event{Timestamp: time.Now(), Action: ActionLoad, Key: "KEY", Success: true, Duration: 2500000},
			wantParts: []string{"duration=2.50ms"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			handler := NewLogHandler(log.New(&buf, "[TEST] ", 0))

			if err := handler.Log(tt.event); err != nil {
				t.Fatalf("LogHandler.Log() error = %v", err)
			}

			output := buf.String()
			for _, want := range tt.wantParts {
				if !strings.Contains(output, want) {
					t.Errorf("log output missing %q: %s", want, output)
				}
			}
			for _, skip := range tt.skipParts {
				if strings.Contains(output, skip) {
					t.Errorf("log output should not contain %q: %s", skip, output)
				}
			}
		})
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

// panicTestHandler panics on Log to verify recover protection when the
// auditor invokes a user-supplied handler.
type panicTestHandler struct{}

func (h *panicTestHandler) Log(event Event) error {
	panic("simulated handler panic")
}

func (h *panicTestHandler) Close() error {
	return nil
}

// closePanicTestHandler panics on Close to verify recover protection when the
// auditor or buffered handler closes a user-supplied handler.
type closePanicTestHandler struct{}

func (h *closePanicTestHandler) Log(event Event) error {
	return nil
}

func (h *closePanicTestHandler) Close() error {
	panic("simulated close panic")
}

// TestAuditor_LogHandlerPanicRecovery verifies that a panic in a user-supplied
// handler during a foreground Auditor.Log call is recovered and returned as an
// error instead of crashing the process.
func TestAuditor_LogHandlerPanicRecovery(t *testing.T) {
	a := NewAuditor(&panicTestHandler{}, nil, nil, true)

	err := a.Log(ActionSet, "KEY", "test", true)
	if err == nil {
		t.Fatal("Log() with panicking handler should return an error")
	}
	if !strings.Contains(err.Error(), "audit handler panicked") {
		t.Errorf("Log() error = %q, want substring %q", err.Error(), "audit handler panicked")
	}

	// The auditor must remain usable after recovering.
	if err := a.Log(ActionSet, "KEY", "test", true); err == nil {
		t.Error("subsequent Log() should still report the panic as an error")
	}
	if err := a.Close(); err != nil {
		t.Errorf("Close() error = %v, want nil", err)
	}
}

// TestAuditor_CloseHandlerPanicRecovery verifies that a panic in a
// user-supplied handler during Auditor.Close is recovered and returned as an
// error instead of crashing the process.
func TestAuditor_CloseHandlerPanicRecovery(t *testing.T) {
	a := NewAuditor(&closePanicTestHandler{}, nil, nil, true)

	err := a.Close()
	if err == nil {
		t.Fatal("Close() with panicking handler should return an error")
	}
	if !strings.Contains(err.Error(), "audit handler panicked during Close") {
		t.Errorf("Close() error = %q, want substring %q", err.Error(), "audit handler panicked during Close")
	}
}

// ============================================================================
// CloseableChannelHandler Tests
// ============================================================================

func TestCloseableChannelHandler_Basic(t *testing.T) {
	handler := NewCloseableChannelHandler(10)
	defer handler.Close()

	ch := handler.Channel()
	if ch == nil {
		t.Error("Channel() should not return nil")
	}

	event := Event{
		Timestamp: time.Now(),
		Action:    ActionSet,
		Key:       "KEY",
	}

	err := handler.Log(event)
	if err != nil {
		t.Errorf("Log() error = %v", err)
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

func TestCloseableChannelHandler_CloseChannel(t *testing.T) {
	handler := NewCloseableChannelHandler(10)
	ch := handler.Channel()

	// Log an event
	_ = handler.Log(Event{Action: ActionSet})

	// Close should close the channel
	if err := handler.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}

	// Drain the buffered event first
	<-ch

	// Channel should be closed now
	_, ok := <-ch
	if ok {
		t.Error("channel should be closed")
	}
}

func TestCloseableChannelHandler_ReceiverUnblocks(t *testing.T) {
	handler := NewCloseableChannelHandler(0) // Unbuffered

	receiverDone := make(chan struct{})
	go func() {
		ch := handler.Channel()
		// This will block until handler is closed
		for range ch {
			// Consume events
		}
		close(receiverDone)
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

func TestCloseableChannelHandler_LogAfterClose(t *testing.T) {
	handler := NewCloseableChannelHandler(10)

	// Close first
	if err := handler.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}

	// Log after close should return error
	err := handler.Log(Event{Action: ActionSet})
	if err == nil {
		t.Error("Log() after Close() should return error")
	}
}

func TestCloseableChannelHandler_IdempotentClose(t *testing.T) {
	handler := NewCloseableChannelHandler(10)

	// Close multiple times should not error
	for i := 0; i < 3; i++ {
		if err := handler.Close(); err != nil {
			t.Errorf("Close() #%d error = %v", i+1, err)
		}
	}

	if !handler.IsClosed() {
		t.Error("IsClosed() should return true")
	}
}

func TestCloseableChannelHandler_NegativeBufferSize(t *testing.T) {
	// Negative buffer size should be treated as 0
	handler := NewCloseableChannelHandler(-1)
	defer handler.Close()

	// Should still work (unbuffered)
	ch := handler.Channel()
	go func() {
		_ = handler.Log(Event{Action: ActionSet})
	}()

	select {
	case <-ch:
		// Success
	case <-time.After(time.Second):
		t.Error("should have received event")
	}
}

func TestCloseableChannelHandler_Concurrent(t *testing.T) {
	handler := NewCloseableChannelHandler(100)
	defer handler.Close()

	ch := handler.Channel()
	received := make(chan int, 100)

	// Start receiver
	go func() {
		for range ch {
			received <- 1
		}
	}()

	// Concurrent senders
	var done sync.WaitGroup
	for i := 0; i < 10; i++ {
		done.Add(1)
		go func() {
			defer done.Done()
			for j := 0; j < 10; j++ {
				_ = handler.Log(Event{Action: ActionSet})
			}
		}()
	}

	done.Wait()

	// Wait for all events to be received
	timeout := time.After(time.Second)
	count := 0
	for count < 100 {
		select {
		case <-received:
			count++
		case <-timeout:
			t.Errorf("timeout waiting for events, received %d/100", count)
			return
		}
	}
}

// TestAuditEventPool_Boundary covers the audit event pool's defensive paths:
// nil puts are ignored, and a pool poisoned with an unexpected type falls
// back to a fresh Event.
func TestAuditEventPool_Boundary(t *testing.T) {
	t.Run("nil event is ignored", func(t *testing.T) {
		putAuditEvent(nil) // must not panic
	})

	t.Run("unexpected pool type falls back to a fresh event", func(t *testing.T) {
		auditEventPool.Put(new(int)) // poison the pool with a foreign type
		ev := getAuditEvent()
		if ev == nil {
			t.Fatal("getAuditEvent() = nil, want a fresh fallback Event")
		}
		putAuditEvent(ev) // restore a well-typed event to the pool
	})
}

// TestCloseableChannelHandler_IsClosedStates covers both states of the
// closed flag: false while open, true after Close.
func TestCloseableChannelHandler_IsClosedStates(t *testing.T) {
	handler := NewCloseableChannelHandler(1)
	defer handler.Close()

	if handler.IsClosed() {
		t.Error("IsClosed() = true before Close(), want false")
	}
	if err := handler.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if !handler.IsClosed() {
		t.Error("IsClosed() = false after Close(), want true")
	}
}
