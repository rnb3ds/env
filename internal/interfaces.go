// Package internal provides internal interfaces for dependency injection.
package internal

// KeyValidator validates environment variable keys.
type KeyValidator interface {
	ValidateKey(key string) error
}

// ValueValidator validates environment variable values.
type ValueValidator interface {
	ValidateValue(value string) error
}

// VariableExpander performs variable expansion.
type VariableExpander interface {
	Expand(s string) (string, error)
}

// AuditLogger records audit events.
type AuditLogger interface {
	LogError(action Action, key, errMsg string) error
}

// Dependencies combines all parser dependencies.
type Dependencies interface {
	KeyValidator
	ValueValidator
	VariableExpander
	AuditLogger
}

// Type aliases for backward compatibility with existing internal code.
type (
	LineKeyValidator       = KeyValidator
	LineValueValidator     = ValueValidator
	LineExpander           = VariableExpander
	LineAuditLogger        = AuditLogger
	LineParserDependencies = Dependencies
)
