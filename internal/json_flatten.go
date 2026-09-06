// Package internal provides JSON flattening utilities.
package internal

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// JSONFlattenConfig holds configuration for JSON flattening.
type JSONFlattenConfig struct {
	// KeyDelimiter is the delimiter for nested keys (default: "_").
	KeyDelimiter string
	// ArrayIndexFormat controls how array indices are formatted.
	// "underscore": KEY_0, KEY_1, etc.
	ArrayIndexFormat string
	// NullAsEmpty converts null values to empty strings (default: true).
	NullAsEmpty bool
	// NumberAsString converts numbers to strings (default: true).
	NumberAsString bool
	// BoolAsString converts booleans to strings (default: true).
	BoolAsString bool
	// MaxDepth limits the maximum nesting depth to prevent stack overflow.
	MaxDepth int
}

// FlattenJSON converts nested JSON data to a flat map of string key-value pairs.
// Keys are converted to uppercase with the configured delimiter.
func FlattenJSON(data []byte, cfg JSONFlattenConfig) (map[string]string, error) {
	if len(data) == 0 {
		return make(map[string]string), nil
	}

	// SECURITY: Pre-validate JSON nesting depth and node count BEFORE
	// json.Unmarshal so that MaxDepth and the node cap are enforced as
	// fail-fast boundaries. Without this, json.Unmarshal recursively decodes
	// the entire document into nested interface{} values before flattenValue
	// can enforce the limit, defeating its purpose and risking excessive
	// allocation (or goroutine-stack exhaustion on a document crafted
	// entirely of nested brackets at the size ceiling). The YAML inline
	// path (yaml_flatten.go) uses the same scanJSONLimits check.
	depthExceeded, nodesExceeded := scanJSONLimits(data, 0, cfg.MaxDepth, HardMaxJSONNodes)
	if depthExceeded {
		return nil, &JSONError{
			Message: fmt.Sprintf("maximum nesting depth exceeded (%d)", cfg.MaxDepth),
		}
	}
	if nodesExceeded {
		return nil, &JSONError{
			Message: fmt.Sprintf("maximum node count exceeded (%d)", HardMaxJSONNodes),
		}
	}

	// Parse JSON
	var raw interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, &JSONError{
			Path:    "",
			Message: "invalid JSON syntax",
			Err:     err,
		}
	}

	// Estimate capacity from input size to reduce map growth.
	// Average JSON key-value pair is ~30-50 bytes of raw JSON, producing one flat entry.
	estSize := len(data) / 40
	if estSize < 4 {
		estSize = 4
	}
	result := make(map[string]string, estSize)
	if err := flattenValue(raw, "", cfg, result, 0); err != nil {
		return nil, err
	}

	return result, nil
}

// scanJSONLimits pre-scans JSON bytes and reports whether the bracket
// nesting (starting at startDepth) exceeds maxDepth, or the structural node
// count (opening brackets, commas and colons — a proxy for the number of
// decoded values) exceeds maxNodes. String contents are skipped so brackets
// and commas inside JSON strings are not counted. Both counters are
// conservative (they may over-count on malformed input), which is safe — the
// goal is to reject oversized input before json.Unmarshal allocates a parse
// tree whose memory is disproportionate to the input byte size, not to
// compute exact counts.
//
// Called BEFORE json.Unmarshal so MaxDepth and the node cap act as fail-fast
// boundaries rather than checks that run only after a full recursive parse.
// Shared by the standalone JSON path (FlattenJSON) and the
// inline-JSON-in-YAML path (yaml_flatten.go).
func scanJSONLimits(data []byte, startDepth, maxDepth, maxNodes int) (depthExceeded, nodesExceeded bool) {
	nesting := 0
	nodes := 0
	for i := 0; i < len(data); i++ {
		switch data[i] {
		case '{', '[':
			nodes++
			if nesting++; startDepth+nesting > maxDepth {
				return true, false
			}
		case '}', ']':
			if nesting--; nesting < 0 {
				nesting = 0
			}
		case ',', ':':
			nodes++
		case '"':
			// Skip string body so brackets inside strings are not counted.
			i++
			for i < len(data) {
				if data[i] == '\\' {
					i++ // skip escaped char
				} else if data[i] == '"' {
					break
				}
				i++
			}
		}
		if nodes > maxNodes {
			return false, true
		}
	}
	return false, false
}

// flattenValue recursively flattens a JSON value.
func flattenValue(value interface{}, prefix string, cfg JSONFlattenConfig, result map[string]string, depth int) error {
	// Check depth limit - use >= for strict enforcement
	if depth >= cfg.MaxDepth {
		return &JSONError{
			Path:    prefix,
			Message: fmt.Sprintf("maximum nesting depth exceeded (%d)", cfg.MaxDepth),
		}
	}

	switch v := value.(type) {
	case nil:
		if prefix == "" {
			return nil
		}
		if cfg.NullAsEmpty {
			result[prefix] = ""
		} else {
			result[prefix] = "null"
		}

	case bool:
		if prefix == "" {
			return nil
		}
		// BoolAsString does not apply here: a decoded JSON bool has no raw
		// text to preserve, so both settings render the canonical form.
		result[prefix] = strconv.FormatBool(v)

	case float64:
		if prefix == "" {
			return nil
		}
		if cfg.NumberAsString {
			// Format as integer if it's a whole number
			if v == float64(int64(v)) {
				result[prefix] = strconv.FormatInt(int64(v), 10)
			} else {
				result[prefix] = strconv.FormatFloat(v, 'f', -1, 64)
			}
		} else {
			result[prefix] = fmt.Sprintf("%v", v)
		}

	case string:
		if prefix == "" {
			return nil
		}
		result[prefix] = v

	case map[string]interface{}:
		for key, val := range v {
			newPrefix := buildKey(prefix, key, cfg)
			if err := flattenValue(val, newPrefix, cfg, result, depth+1); err != nil {
				return err
			}
		}

	case []interface{}:
		for i := range v {
			newPrefix := buildArrayIndex(prefix, i, cfg)
			if err := flattenValue(v[i], newPrefix, cfg, result, depth+1); err != nil {
				return err
			}
		}

	default:
		return &JSONError{
			Path:    prefix,
			Message: fmt.Sprintf("unsupported JSON type: %T", value),
		}
	}

	return nil
}

// buildKey constructs a key from prefix and key parts.
// Delegates to the shared flattener key builder (see flatten_keys.go).
func buildKey(prefix, key string, cfg JSONFlattenConfig) string {
	return buildFlatKey(prefix, key, cfg.KeyDelimiter)
}

// buildArrayIndex constructs a key for array elements.
// Delegates to the shared flattener key builder (see flatten_keys.go).
func buildArrayIndex(prefix string, index int, cfg JSONFlattenConfig) string {
	return buildFlatArrayIndex(prefix, index, cfg.KeyDelimiter, cfg.ArrayIndexFormat)
}
