package env

import (
	"fmt"

	"github.com/cybergodev/env/internal"
)

// structuredParserConfig holds common configuration for structured file parsers (JSON/YAML).
// This is used internally to share validation logic between parsers.
type structuredParserConfig struct {
	config    Config
	validator Validator
	auditor   FullAuditLogger
}

// validateResult validates parsed key-value pairs from structured files (JSON/YAML).
// It checks:
//   - Maximum variable count
//   - Key pattern validity (using IsValidJSONKey for both JSON and YAML)
//   - Value validation (if enabled in config)
//   - Required keys presence
//
// Returns an error if validation fails, nil otherwise.
func (c *structuredParserConfig) validateResult(result map[string]string, format string) error {
	// Check result size against config
	if len(result) > c.config.MaxVariables {
		_ = c.auditor.LogError(internal.ActionParse, "", "maximum variables exceeded")
		return &ValidationError{
			Field:   "variables",
			Message: fmt.Sprintf("exceeded maximum of %d variables", c.config.MaxVariables),
		}
	}

	// Validate each key and value using fast byte-level validation
	for key, val := range result {
		// Use fast byte-level validation (allows @, -, ., etc.)
		// Note: Both JSON and YAML use the same key validation pattern
		if !internal.IsValidJSONKey(key) {
			_ = c.auditor.LogError(internal.ActionParse, key, "key does not match "+format+" key pattern")
			return &ValidationError{
				Field:   "key",
				Value:   MaskSensitiveInString(key),
				Rule:    "pattern",
				Message: "key does not match required pattern",
			}
		}
		if c.config.ValidateValues {
			if err := c.validator.ValidateValue(val); err != nil {
				_ = c.auditor.LogError(internal.ActionParse, key, err.Error())
				return err
			}
		}
	}

	// Validate required keys
	upperKeys := internal.KeysToUpperPooled(result)
	err := c.validator.ValidateRequired(upperKeys)
	internal.PutKeysToUpperMap(upperKeys)
	if err != nil {
		_ = c.auditor.LogError(internal.ActionValidate, "", err.Error())
		return err
	}

	return nil
}
