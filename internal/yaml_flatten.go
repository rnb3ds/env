// Package internal provides YAML flattening utilities.
package internal

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// YAMLFlattenConfig holds configuration for YAML flattening.
type YAMLFlattenConfig struct {
	// KeyDelimiter is the delimiter for nested keys (default: "_").
	KeyDelimiter string
	// ArrayIndexFormat controls how array indices are formatted.
	// "underscore": KEY_0, KEY_1, etc.
	// "bracket": KEY[0], KEY[1], etc.
	ArrayIndexFormat string
	// NullAsEmpty converts null/~ values to empty strings (default: true).
	NullAsEmpty bool
	// NumberAsString converts numbers to strings (default: true).
	NumberAsString bool
	// BoolAsString converts booleans to strings (default: true).
	BoolAsString bool
	// MaxDepth limits the maximum nesting depth to prevent stack overflow.
	MaxDepth int
}

// FlattenYAML converts a YAML Value tree to a flat map of string key-value pairs.
// Keys are converted to uppercase with the configured delimiter.
func FlattenYAML(value *Value, cfg YAMLFlattenConfig) (map[string]string, error) {
	if value == nil {
		return make(map[string]string), nil
	}

	result := make(map[string]string)
	if err := flattenYAMLValue(value, "", cfg, result, 0); err != nil {
		return nil, err
	}

	return result, nil
}

// flattenYAMLValue recursively flattens a YAML value.
func flattenYAMLValue(value *Value, prefix string, cfg YAMLFlattenConfig, result map[string]string, depth int) error {
	// Check depth limit - use >= for strict enforcement
	if depth >= cfg.MaxDepth {
		return &YAMLError{
			Path:    prefix,
			Message: fmt.Sprintf("maximum nesting depth exceeded (%d)", cfg.MaxDepth),
		}
	}

	if value == nil {
		return nil
	}

	switch value.Type {
	case ValueTypeScalar:
		if prefix == "" {
			return nil
		}
		// Check for inline JSON array or object
		scalar := strings.TrimSpace(value.Scalar)
		if len(scalar) >= 2 {
			if (scalar[0] == '[' && scalar[len(scalar)-1] == ']') ||
				(scalar[0] == '{' && scalar[len(scalar)-1] == '}') {
				// Try to parse as inline JSON
				if err := flattenInlineJSON(scalar, prefix, cfg, result, depth); err == nil {
					return nil
				}
				// If parsing fails, fall through to treat as regular scalar
			}
		}
		result[prefix] = convertYAMLScalar(value.Scalar, cfg)

	case ValueTypeMap:
		if len(value.Map) == 0 && prefix != "" {
			// Empty map as empty string
			result[prefix] = ""
			return nil
		}
		for key, val := range value.Map {
			newPrefix := buildYAMLKey(prefix, key, cfg)
			if err := flattenYAMLValue(val, newPrefix, cfg, result, depth+1); err != nil {
				return err
			}
		}

	case ValueTypeArray:
		if len(value.Array) == 0 && prefix != "" {
			// Empty array as empty string
			result[prefix] = ""
			return nil
		}
		for i, val := range value.Array {
			newPrefix := buildYAMLArrayIndex(prefix, i, cfg)
			if err := flattenYAMLValue(val, newPrefix, cfg, result, depth+1); err != nil {
				return err
			}
		}
	}

	return nil
}

// convertYAMLScalar converts a YAML scalar to a string based on configuration.
func convertYAMLScalar(s string, cfg YAMLFlattenConfig) string {
	// Handle null
	lower := strings.ToLower(strings.TrimSpace(s))
	if lower == "null" || s == "~" || s == "" {
		if cfg.NullAsEmpty {
			return ""
		}
		return "null"
	}

	// Handle boolean
	if lower == "true" || lower == "false" {
		if cfg.BoolAsString {
			return lower
		}
		return s
	}

	// Handle numbers
	if cfg.NumberAsString {
		// Try integer
		if num, err := strconv.ParseInt(s, 10, 64); err == nil {
			return strconv.FormatInt(num, 10)
		}
		// Try float
		if num, err := strconv.ParseFloat(s, 64); err == nil {
			// Format as integer if it's a whole number
			if num == float64(int64(num)) {
				return strconv.FormatInt(int64(num), 10)
			}
			return strconv.FormatFloat(num, 'f', -1, 64)
		}
	}

	return s
}

// buildYAMLKey constructs a key from prefix and key parts.
// Uses pooled strings.Builder for efficiency when concatenation is needed.
func buildYAMLKey(prefix, key string, cfg YAMLFlattenConfig) string {
	key = ToUpperASCII(key)
	if prefix == "" {
		return key
	}
	// Use pooled strings.Builder for efficient concatenation
	sb := GetBuilder()
	defer PutBuilder(sb)
	sb.Grow(len(prefix) + len(cfg.KeyDelimiter) + len(key))
	sb.WriteString(prefix)
	sb.WriteString(cfg.KeyDelimiter)
	sb.WriteString(key)
	return sb.String()
}

// buildYAMLArrayIndex constructs a key for array elements.
// Uses pooled strings.Builder and strconv.Itoa for efficient conversion.
func buildYAMLArrayIndex(prefix string, index int, cfg YAMLFlattenConfig) string {
	indexStr := strconv.Itoa(index)
	sb := GetBuilder()
	defer PutBuilder(sb)

	switch cfg.ArrayIndexFormat {
	case "bracket":
		// prefix[index] format
		sb.Grow(len(prefix) + 1 + len(indexStr) + 1)
		sb.WriteString(prefix)
		sb.WriteByte('[')
		sb.WriteString(indexStr)
		sb.WriteByte(']')
		return sb.String()
	default: // underscore
		// prefix_index format
		sb.Grow(len(prefix) + len(cfg.KeyDelimiter) + len(indexStr))
		sb.WriteString(prefix)
		sb.WriteString(cfg.KeyDelimiter)
		sb.WriteString(indexStr)
		return sb.String()
	}
}

// flattenInlineJSON parses and flattens inline JSON arrays or objects within YAML.
func flattenInlineJSON(jsonStr, prefix string, cfg YAMLFlattenConfig, result map[string]string, depth int) error {
	// Parse the inline JSON
	var raw interface{}
	if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
		return err
	}

	// Flatten the parsed value directly
	switch v := raw.(type) {
	case []interface{}:
		// Inline array
		for i, item := range v {
			itemPrefix := buildYAMLArrayIndex(prefix, i, cfg)
			if err := flattenInlineValue(item, itemPrefix, cfg, result, depth+1); err != nil {
				return err
			}
		}
	case map[string]interface{}:
		// Inline object
		for key, val := range v {
			keyPrefix := buildYAMLKey(prefix, key, cfg)
			if err := flattenInlineValue(val, keyPrefix, cfg, result, depth+1); err != nil {
				return err
			}
		}
	default:
		// Simple scalar value
		result[prefix] = fmt.Sprintf("%v", v)
	}

	return nil
}

// flattenInlineValue flattens a value from inline JSON.
func flattenInlineValue(value interface{}, prefix string, cfg YAMLFlattenConfig, result map[string]string, depth int) error {
	// Check depth limit - use >= for strict enforcement
	if depth >= cfg.MaxDepth {
		return &YAMLError{
			Path:    prefix,
			Message: fmt.Sprintf("maximum nesting depth exceeded (%d)", cfg.MaxDepth),
		}
	}

	switch v := value.(type) {
	case nil:
		if cfg.NullAsEmpty {
			result[prefix] = ""
		} else {
			result[prefix] = "null"
		}
	case bool:
		if cfg.BoolAsString {
			result[prefix] = strconv.FormatBool(v)
		} else {
			result[prefix] = fmt.Sprintf("%t", v)
		}
	case float64:
		if cfg.NumberAsString {
			if v == float64(int64(v)) {
				result[prefix] = strconv.FormatInt(int64(v), 10)
			} else {
				result[prefix] = strconv.FormatFloat(v, 'f', -1, 64)
			}
		} else {
			result[prefix] = fmt.Sprintf("%v", v)
		}
	case string:
		result[prefix] = v
	case []interface{}:
		for i, item := range v {
			itemPrefix := buildYAMLArrayIndex(prefix, i, cfg)
			if err := flattenInlineValue(item, itemPrefix, cfg, result, depth+1); err != nil {
				return err
			}
		}
	case map[string]interface{}:
		for key, val := range v {
			keyPrefix := buildYAMLKey(prefix, key, cfg)
			if err := flattenInlineValue(val, keyPrefix, cfg, result, depth+1); err != nil {
				return err
			}
		}
	default:
		result[prefix] = fmt.Sprintf("%v", v)
	}

	return nil
}
