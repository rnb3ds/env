package env

import (
	"io"
	"log"

	"github.com/cybergodev/env/internal"
)

// ============================================================================
// Audit Types (re-exported from internal/audit for backward compatibility)
// ============================================================================

// AuditAction represents the type of action being audited.
// Use these constants with AuditLogger.Log() to record security-relevant events.
type AuditAction = internal.Action

// Audit constants for common actions.
// These are used with AuditLogger methods to categorize audit events:
//   - ActionLoad: File loading operations
//   - ActionParse: Parsing operations for env, JSON, YAML files
//   - ActionGet: Variable retrieval operations
//   - ActionSet: Variable assignment operations
//   - ActionDelete: Variable deletion operations
//   - ActionValidate: Validation operations
//   - ActionExpand: Variable expansion operations
//   - ActionSecurity: Security-related events (path validation, forbidden keys)
//   - ActionError: Error conditions
//   - ActionFileAccess: File system access operations
const (
	ActionLoad       AuditAction = internal.ActionLoad
	ActionParse      AuditAction = internal.ActionParse
	ActionGet        AuditAction = internal.ActionGet
	ActionSet        AuditAction = internal.ActionSet
	ActionDelete     AuditAction = internal.ActionDelete
	ActionValidate   AuditAction = internal.ActionValidate
	ActionExpand     AuditAction = internal.ActionExpand
	ActionSecurity   AuditAction = internal.ActionSecurity
	ActionError      AuditAction = internal.ActionError
	ActionFileAccess AuditAction = internal.ActionFileAccess
)

// AuditEvent represents a single audit log entry.
type AuditEvent = internal.Event

// AuditHandler defines the interface for audit log handlers.
type AuditHandler = internal.Handler

// JSONAuditHandler writes audit events as JSON to an io.Writer.
type JSONAuditHandler = internal.JSONHandler

// NewJSONAuditHandler creates a new JSONAuditHandler.
func NewJSONAuditHandler(w io.Writer) *JSONAuditHandler {
	return internal.NewJSONHandler(w)
}

// LogAuditHandler writes audit events using the standard log package.
type LogAuditHandler = internal.LogHandler

// NewLogAuditHandler creates a new LogAuditHandler.
func NewLogAuditHandler(logger *log.Logger) *LogAuditHandler {
	return internal.NewLogHandler(logger)
}

// ChannelAuditHandler sends audit events to a channel.
type ChannelAuditHandler = internal.ChannelHandler

// NewChannelAuditHandler creates a new ChannelAuditHandler.
func NewChannelAuditHandler(ch chan<- AuditEvent) *ChannelAuditHandler {
	return internal.NewChannelHandler(ch)
}

// NopAuditHandler discards all audit events.
type NopAuditHandler = internal.NopHandler

// NewNopAuditHandler creates a new NopAuditHandler.
func NewNopAuditHandler() *NopAuditHandler {
	return internal.NewNopHandler()
}
