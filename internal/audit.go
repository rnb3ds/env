// Package internal provides audit logging functionality.
package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"
)

// Compile-time checks that handlers implement io.Closer.
var (
	_ io.Closer = (*JSONHandler)(nil)
	_ io.Closer = (*LogHandler)(nil)
	_ io.Closer = (*ChannelHandler)(nil)
	_ io.Closer = (*NopHandler)(nil)
	_ io.Closer = (*CloseableChannelHandler)(nil)
	_ io.Closer = (*Auditor)(nil)
)

// Action represents the type of action being audited.
type Action string

// Audit action constants.
const (
	ActionLoad       Action = "load"
	ActionParse      Action = "parse"
	ActionGet        Action = "get"
	ActionSet        Action = "set"
	ActionDelete     Action = "delete"
	ActionValidate   Action = "validate"
	ActionExpand     Action = "expand"
	ActionSecurity   Action = "security"
	ActionError      Action = "error"
	ActionFileAccess Action = "file_access"
)

// Event represents a single audit log entry.
type Event struct {
	Timestamp time.Time `json:"timestamp"`
	Action    Action    `json:"action"`
	Key       string    `json:"key,omitempty"`
	File      string    `json:"file,omitempty"`
	Reason    string    `json:"reason,omitempty"`
	Success   bool      `json:"success"`
	Masked    bool      `json:"masked,omitempty"`
	Details   string    `json:"details,omitempty"`
	Duration  int64     `json:"duration_ns,omitempty"`
}

// Handler defines the interface for audit log handlers.
type Handler interface {
	Log(event Event) error
	Close() error
}

// JSONHandler writes audit events as JSON to an io.Writer.
type JSONHandler struct {
	mu     sync.Mutex
	writer io.Writer
}

// NewJSONHandler creates a new JSONHandler.
func NewJSONHandler(w io.Writer) *JSONHandler {
	return &JSONHandler{writer: w}
}

// Log writes an audit event as JSON.
func (h *JSONHandler) Log(event Event) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal audit event: %w", err)
	}
	_, err = h.writer.Write(append(data, '\n'))
	return err
}

// Close implements Handler.
func (h *JSONHandler) Close() error {
	if closer, ok := h.writer.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

// LogHandler writes audit events using the standard log package.
type LogHandler struct {
	mu     sync.Mutex
	logger *log.Logger
}

// NewLogHandler creates a new LogHandler.
func NewLogHandler(logger *log.Logger) *LogHandler {
	if logger == nil {
		logger = log.New(os.Stderr, "[AUDIT] ", log.LstdFlags)
	}
	return &LogHandler{logger: logger}
}

// Log writes an audit event using the logger.
func (h *LogHandler) Log(event Event) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	var msg string
	if event.Key != "" {
		msg = fmt.Sprintf("action=%s key=%s success=%v reason=%q",
			event.Action, event.Key, event.Success, event.Reason)
	} else {
		msg = fmt.Sprintf("action=%s success=%v reason=%q",
			event.Action, event.Success, event.Reason)
	}
	if event.File != "" {
		msg += fmt.Sprintf(" file=%s", event.File)
	}
	if event.Duration > 0 {
		if event.Duration < 1e6 {
			msg += fmt.Sprintf(" duration=%dμs", event.Duration/1e3)
		} else {
			msg += fmt.Sprintf(" duration=%.2fms", float64(event.Duration)/1e6)
		}
	}
	h.logger.Println(msg)
	return nil
}

// Close implements Handler.
func (h *LogHandler) Close() error {
	return nil
}

// ChannelHandler sends audit events to a channel.
//
// Channel Ownership: This handler does NOT own the channel. The caller is
// responsible for closing the channel when done. The handler's Close() method
// does nothing because closing a send-only channel (chan<-) would panic if
// the caller hasn't finished receiving.
//
// Blocking Behavior: This handler blocks if the channel buffer is full.
// Use a buffered channel if non-blocking behavior is required.
//
// Example:
//
//	ch := make(chan Event, 100)
//	handler := NewChannelHandler(ch)
//	// Start consumer goroutine
//	go func() {
//	    for event := range ch {
//	        process(event)
//	    }
//	}()
//	// ... use handler ...
//	handler.Close() // Does NOT close ch
//	close(ch)       // Caller must close the channel to signal EOF to receiver
type ChannelHandler struct {
	ch chan<- Event
}

// NewChannelHandler creates a new ChannelHandler that sends events to the
// provided channel. The caller retains ownership of the channel and must
// close it when finished to signal EOF to receivers.
func NewChannelHandler(ch chan<- Event) *ChannelHandler {
	return &ChannelHandler{ch: ch}
}

// Log sends an audit event to the channel.
// This method blocks if the channel is full.
func (h *ChannelHandler) Log(event Event) error {
	h.ch <- event
	return nil
}

// Close implements Handler.
// NOTE: This method does NOT close the underlying channel because the handler
// does not own it. The caller must close the channel separately.
func (h *ChannelHandler) Close() error {
	return nil
}

// CloseableChannelHandler sends audit events to a channel and owns the channel
// lifecycle. Unlike ChannelHandler, this handler creates its own channel and
// closes it when Close() is called.
//
// This is useful when you want the handler to manage the complete lifecycle
// of the channel, ensuring receivers are properly signaled when the handler
// is closed.
//
// Example:
//
//	handler := NewCloseableChannelHandler(100)
//	// Get the channel for receiving
//	ch := handler.Channel()
//	// Start consumer goroutine
//	go func() {
//	    for event := range ch {
//	        process(event)
//	    }
//	    fmt.Println("Channel closed, consumer exiting")
//	}()
//	// ... use handler ...
//	handler.Close() // Closes the channel, consumer goroutine exits gracefully
type CloseableChannelHandler struct {
	ch        chan Event
	done      chan struct{} // closed in Close() to unblock Log() without panic
	closeMu   sync.Mutex
	closeOnce sync.Once // ensures done/ch are closed exactly once
}

// NewCloseableChannelHandler creates a new CloseableChannelHandler with a
// buffered channel of the specified size. The handler owns the channel and
// will close it when Close() is called.
func NewCloseableChannelHandler(bufferSize int) *CloseableChannelHandler {
	if bufferSize < 0 {
		bufferSize = 0
	}
	return &CloseableChannelHandler{
		ch:   make(chan Event, bufferSize),
		done: make(chan struct{}),
	}
}

// Channel returns the underlying channel for receiving events.
// The returned channel will be closed when Close() is called.
func (h *CloseableChannelHandler) Channel() <-chan Event {
	return h.ch
}

// Log sends an audit event to the channel.
//
// For buffered channels, the send is non-blocking: if the buffer is full the
// event is dropped and an error is returned. For unbuffered channels (buffer
// size 0), the send blocks until a receiver is ready or the handler is closed.
//
// closeMu is held during the send to prevent Close() from closing the channel
// concurrently, which would cause a send-on-closed-channel panic. For
// unbuffered channels, Close() closes the done channel before acquiring
// closeMu so that a blocked send can be interrupted without a deadlock.
func (h *CloseableChannelHandler) Log(event Event) error {
	h.closeMu.Lock()
	defer h.closeMu.Unlock()

	select {
	case <-h.done:
		return fmt.Errorf("handler is closed")
	default:
	}

	if cap(h.ch) == 0 {
		// Unbuffered: block until the event is received or the handler closes.
		select {
		case h.ch <- event:
			return nil
		case <-h.done:
			return fmt.Errorf("handler is closed")
		}
	}

	// Buffered: non-blocking send, drop the event if the buffer is full.
	select {
	case h.ch <- event:
		return nil
	default:
		return fmt.Errorf("audit channel full, event dropped")
	}
}

// Close implements Handler.
//
// Closes done first (via closeOnce, without holding closeMu) so that any Log()
// call blocked on an unbuffered send is interrupted immediately. Only then does
// it acquire closeMu to close the event channel, guaranteeing no send is in
// progress when the channel is closed. Safe to call multiple times.
func (h *CloseableChannelHandler) Close() error {
	h.closeOnce.Do(func() {
		// Signal shutdown. This unblocks any Log() waiting on an unbuffered
		// send before we contend on closeMu.
		close(h.done)

		// Wait for any in-flight Log() to finish its send, then close ch.
		h.closeMu.Lock()
		close(h.ch)
		h.closeMu.Unlock()
	})
	return nil
}

// IsClosed returns true if the handler has been closed.
func (h *CloseableChannelHandler) IsClosed() bool {
	select {
	case <-h.done:
		return true
	default:
		return false
	}
}

// NopHandler discards all audit events.
type NopHandler struct{}

// NewNopHandler creates a new NopHandler.
func NewNopHandler() *NopHandler {
	return &NopHandler{}
}

// Log does nothing.
func (h *NopHandler) Log(event Event) error {
	return nil
}

// Close does nothing.
func (h *NopHandler) Close() error {
	return nil
}

// IsSensitiveFunc is a function type that determines if a key is sensitive.
type IsSensitiveFunc func(key string) bool

// MaskerFunc is a function type that masks a key-value pair.
type MaskerFunc func(key, value string) string

// Auditor provides audit logging functionality.
type Auditor struct {
	handler     Handler
	masker      MaskerFunc
	isSensitive IsSensitiveFunc
	enabled     atomic.Bool
	mu          sync.RWMutex
}

// auditEventPool provides a pool of reusable Event structs.
// This reduces allocations for high-frequency audit logging.
var auditEventPool = sync.Pool{
	New: func() any {
		return &Event{}
	},
}

// getAuditEvent retrieves an Event from the pool.
func getAuditEvent() *Event {
	ev, ok := auditEventPool.Get().(*Event)
	if !ok {
		return &Event{}
	}
	return ev
}

// putAuditEvent returns an Event to the pool after resetting it.
func putAuditEvent(ev *Event) {
	if ev == nil {
		return
	}
	// Clear the event to allow GC to collect referenced strings
	*ev = Event{}
	auditEventPool.Put(ev)
}

// NewAuditor creates a new Auditor with the specified handler.
func NewAuditor(handler Handler, isSensitive IsSensitiveFunc, masker MaskerFunc, enabled bool) *Auditor {
	if handler == nil {
		handler = NewNopHandler()
	}
	if isSensitive == nil {
		isSensitive = func(key string) bool { return false }
	}
	if masker == nil {
		masker = func(key, value string) string { return value }
	}
	a := &Auditor{
		handler:     handler,
		masker:      masker,
		isSensitive: isSensitive,
	}
	a.enabled.Store(enabled)
	return a
}

// logEvent finalizes a pooled event (timestamp, key masking, sensitive flag)
// and hands it to the handler. The caller must hold a.mu.RLock and have
// re-checked a.enabled under that lock.
//
// NOTE: the RLock is held across handler.Log intentionally — it is
// load-bearing. It serializes logEvent against Close()/SetEnabled() (which
// take the write lock). Several handlers (e.g. JSONHandler) are NOT
// internally Close-safe: their Close() closes the writer without taking their
// own lock, so without this serialization a concurrent Auditor.Close() could
// close the writer mid-Write (a data race). Do not move handler.Log outside
// the lock without first making every Handler Close-safe.
func (a *Auditor) logEvent(event *Event, key string) error {
	event.Timestamp = time.Now()
	// Compute the sensitive flag from the original key before masking.
	event.Masked = key != "" && a.isSensitive(key)
	event.Key = a.maskKey(key)
	err := safeHandlerLog(a.handler, *event)
	putAuditEvent(event)
	return err
}

// safeHandlerLog invokes handler.Log with panic protection.
//
// The handler is user-supplied; if it panics, the panic is recovered and
// returned as an error instead of crashing the process. Every call site —
// including the foreground paths (Load/Parse, which call the auditor
// mid-operation) — routes handler.Log through here.
func safeHandlerLog(h Handler, event Event) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("audit handler panicked: %v\n%s", r, debug.Stack())
		}
	}()
	return h.Log(event)
}

// safeHandlerClose invokes handler.Close with panic protection.
// See safeHandlerLog for rationale.
func safeHandlerClose(h Handler) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("audit handler panicked during Close: %v\n%s", r, debug.Stack())
		}
	}()
	return h.Close()
}

// acquireLogEvent validates the enabled state and returns a pooled event the
// caller can populate, or nil when auditing is disabled.
// On success the caller holds a.mu.RLock and MUST call releaseLogEvent.
func (a *Auditor) acquireLogEvent() (*Event, bool) {
	// Fast path: atomic read avoids lock acquisition when auditing is disabled.
	if !a.enabled.Load() {
		return nil, false
	}
	a.mu.RLock()
	// Re-check under lock: ensures we don't log after a concurrent
	// SetEnabled(false) or Close().
	if !a.enabled.Load() {
		a.mu.RUnlock()
		return nil, false
	}
	return getAuditEvent(), true
}

// releaseLogEvent releases the read lock acquired by acquireLogEvent.
func (a *Auditor) releaseLogEvent() {
	a.mu.RUnlock()
}

// Log records an audit event.
func (a *Auditor) Log(action Action, key, reason string, success bool) error {
	event, ok := a.acquireLogEvent()
	if !ok {
		return nil
	}
	defer a.releaseLogEvent()
	event.Action = action
	event.Reason = reason
	event.Success = success
	return a.logEvent(event, key)
}

// LogWithFile records an audit event with file information.
func (a *Auditor) LogWithFile(action Action, key, file, reason string, success bool) error {
	event, ok := a.acquireLogEvent()
	if !ok {
		return nil
	}
	defer a.releaseLogEvent()
	event.Action = action
	event.File = file
	event.Reason = reason
	event.Success = success
	return a.logEvent(event, key)
}

// LogWithDuration records an audit event with timing information.
func (a *Auditor) LogWithDuration(action Action, key, reason string, success bool, duration time.Duration) error {
	event, ok := a.acquireLogEvent()
	if !ok {
		return nil
	}
	defer a.releaseLogEvent()
	event.Action = action
	event.Reason = reason
	event.Success = success
	event.Duration = duration.Nanoseconds()
	return a.logEvent(event, key)
}

// LogError records an error event.
func (a *Auditor) LogError(action Action, key, errMsg string) error {
	return a.Log(action, key, errMsg, false)
}

// SetEnabled enables or disables audit logging.
func (a *Auditor) SetEnabled(enabled bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.enabled.Store(enabled)
}

// IsEnabled returns whether audit logging is enabled.
func (a *Auditor) IsEnabled() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.enabled.Load()
}

// Close closes the underlying handler and disables further logging.
// Subsequent Log* calls become no-ops instead of writing to a handler
// whose resources (e.g. an underlying file) may already be released.
// SetEnabled(true) can re-enable logging on a closed auditor if the
// handler remains usable.
func (a *Auditor) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.enabled.Store(false)
	return safeHandlerClose(a.handler)
}

// maskKey masks a key for audit logging.
func (a *Auditor) maskKey(key string) string {
	if key == "" {
		return ""
	}
	if a.isSensitive(key) {
		return a.masker(key, key)
	}
	return key
}

// DefaultHandler returns the default audit handler (writes to stderr).
func DefaultHandler() Handler {
	return NewLogHandler(nil)
}
