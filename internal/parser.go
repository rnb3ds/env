// Package internal provides internal line parsing utilities.
package internal

import (
	"bytes"
	"strings"
)

// LineParserConfig holds the configuration needed for parsing.
type LineParserConfig struct {
	AllowExportPrefix bool
	AllowYamlSyntax   bool
	OverwriteExisting bool
	MaxVariables      int
	ExpandVariables   bool
}

// LineParser handles parsing of individual lines.
type LineParser struct {
	config    LineParserConfig
	validator *Validator
	auditor   *Auditor
	expander  *Expander
}

// NewLineParser creates a new LineParser.
func NewLineParser(cfg LineParserConfig, v *Validator, a *Auditor, e *Expander) *LineParser {
	return &LineParser{
		config:    cfg,
		validator: v,
		auditor:   a,
		expander:  e,
	}
}

// ParseLine parses a single line and returns the key and value.
func (p *LineParser) ParseLine(line string) (string, string, error) {
	// Handle export prefix - use direct slice comparison for speed
	if p.config.AllowExportPrefix && len(line) > 7 && line[:7] == "export " {
		line = line[7:]
	}

	// Find the separator
	sepIdx := strings.IndexAny(line, "=:")
	if sepIdx == -1 {
		return "", "", nil // No assignment on this line
	}

	// Trim key using byte-level operations (faster than strings.TrimSpace)
	keyStart := 0
	keyEnd := sepIdx
	for keyStart < keyEnd && (line[keyStart] == ' ' || line[keyStart] == '\t') {
		keyStart++
	}
	for keyEnd > keyStart && (line[keyEnd-1] == ' ' || line[keyEnd-1] == '\t') {
		keyEnd--
	}
	key := line[keyStart:keyEnd]

	// Validate key
	if err := p.validator.ValidateKey(key); err != nil {
		return "", "", err
	}

	value := line[sepIdx+1:]

	// Parse the value
	parsedValue, err := p.ParseValue(value)
	if err != nil {
		return "", "", err
	}

	// Validate value
	if err := p.validator.ValidateValue(parsedValue); err != nil {
		return "", "", err
	}

	return key, parsedValue, nil
}

// ParseLineBytes parses a single line from a byte slice and returns the key and value.
// This version avoids string allocation by working directly with bytes.
func (p *LineParser) ParseLineBytes(line []byte) (string, string, error) {
	// Fast path for empty or very short lines
	if len(line) == 0 {
		return "", "", nil
	}

	// Handle export prefix - use byte-level comparison to avoid string allocation
	if p.config.AllowExportPrefix && len(line) > 7 {
		// Check for "export " prefix (7 characters: e-x-p-o-r-t-space)
		if line[0] == 'e' && line[1] == 'x' && line[2] == 'p' &&
			line[3] == 'o' && line[4] == 'r' && line[5] == 't' && line[6] == ' ' {
			line = line[7:]
		}
	}

	// Find the separator using byte-level search
	sepIdx := -1
	for i := 0; i < len(line); i++ {
		if line[i] == '=' || line[i] == ':' {
			sepIdx = i
			break
		}
	}
	if sepIdx == -1 {
		return "", "", nil // No assignment on this line
	}

	// Trim key using byte-level operations
	keyStart := 0
	keyEnd := sepIdx
	for keyStart < keyEnd && (line[keyStart] == ' ' || line[keyStart] == '\t') {
		keyStart++
	}
	for keyEnd > keyStart && (line[keyEnd-1] == ' ' || line[keyEnd-1] == '\t') {
		keyEnd--
	}

	// SECURITY: Must copy the key string because the scanner buffer will be
	// overwritten on the next Scan() call. Using bytesToString here would cause
	// the interned key to point to invalid memory after the buffer is reused.
	// The small allocation cost is necessary for memory safety.
	key := InternKey(string(line[keyStart:keyEnd]))

	// Validate key
	if err := p.validator.ValidateKey(key); err != nil {
		return "", "", err
	}

	// Parse the value
	value := line[sepIdx+1:]
	parsedValue, err := p.ParseValueBytes(value)
	if err != nil {
		return "", "", err
	}

	// Validate value
	if err := p.validator.ValidateValue(parsedValue); err != nil {
		return "", "", err
	}

	return key, parsedValue, nil
}

// ParseValue parses a value handling quotes and escapes.
func (p *LineParser) ParseValue(value string) (string, error) {
	// Fast byte-level trim for leading/trailing whitespace
	start := 0
	end := len(value)
	for start < end && (value[start] == ' ' || value[start] == '\t') {
		start++
	}
	for end > start && (value[end-1] == ' ' || value[end-1] == '\t') {
		end--
	}
	value = value[start:end]

	if len(value) == 0 {
		return "", nil
	}

	// Handle quoted values
	switch value[0] {
	case '"':
		return ParseDoubleQuoted(value)
	case '\'':
		return ParseSingleQuoted(value)
	}

	// Handle YAML-style values if enabled
	if p.config.AllowYamlSyntax {
		if unquoted, ok := TryParseYamlValue(value); ok {
			return unquoted, nil
		}
	}

	// Unquoted value - remove inline comments
	if idx := strings.Index(value, " #"); idx != -1 {
		value = value[:idx]
		// Trim trailing whitespace after comment removal
		end = len(value)
		for end > 0 && (value[end-1] == ' ' || value[end-1] == '\t') {
			end--
		}
		value = value[:end]
	}

	return value, nil
}

// ParseValueBytes parses a value from a byte slice handling quotes and escapes.
// This version avoids string allocation until the final result.
func (p *LineParser) ParseValueBytes(value []byte) (string, error) {
	// Fast path for empty value
	if len(value) == 0 {
		return "", nil
	}

	// Fast byte-level trim for leading/trailing whitespace
	start := 0
	end := len(value)
	for start < end && (value[start] == ' ' || value[start] == '\t') {
		start++
	}
	for end > start && (value[end-1] == ' ' || value[end-1] == '\t') {
		end--
	}
	value = value[start:end]

	if len(value) == 0 {
		return "", nil
	}

	// Handle quoted values
	switch value[0] {
	case '"':
		return ParseDoubleQuotedBytes(value)
	case '\'':
		return ParseSingleQuotedBytes(value)
	}

	// Handle YAML-style values if enabled
	if p.config.AllowYamlSyntax {
		if unquoted, ok := TryParseYamlValueBytes(value); ok {
			return unquoted, nil
		}
	}

	// Unquoted value - remove inline comments using byte-level search
	// Find " #" pattern (need at least 2 characters for this pattern)
	commentIdx := -1
	if len(value) >= 2 {
		for i := 0; i < len(value)-1; i++ {
			if value[i] == ' ' && value[i+1] == '#' {
				commentIdx = i
				break
			}
		}
	}
	if commentIdx != -1 {
		value = value[:commentIdx]
		// Trim trailing whitespace after comment removal
		end = len(value)
		for end > 0 && (value[end-1] == ' ' || value[end-1] == '\t') {
			end--
		}
		value = value[:end]
	}

	// SECURITY: Must copy the value string because it's a slice of the scanner buffer.
	// The scanner reuses its buffer on each Scan() call, so using bytesToString here
	// would cause the returned string to be corrupted when the next line is scanned.
	// The small allocation cost is necessary for memory safety.
	return string(value), nil
}

// ParseDoubleQuoted handles double-quoted values with escape sequences.
func ParseDoubleQuoted(value string) (string, error) {
	if len(value) < 2 || value[len(value)-1] != '"' {
		return "", ErrInvalidValue
	}

	// Extract content between quotes
	content := value[1 : len(value)-1]

	// Fast path: no escape sequences
	if strings.IndexByte(content, '\\') == -1 {
		return content, nil
	}

	// Use pooled builder for escaped content
	result := GetBuilder()
	defer PutBuilder(result)
	result.Grow(len(content))

	// Optimized escape processing with lookup table
	for i := 0; i < len(content); {
		c := content[i]
		if c == '\\' && i+1 < len(content) {
			escaped := content[i+1]
			// Use lookup table for O(1) escape translation
			if translated := escapeTable[escaped]; translated != 0 {
				result.WriteByte(translated)
			} else {
				result.WriteByte(escaped)
			}
			i += 2
		} else {
			result.WriteByte(c)
			i++
		}
	}

	return result.String(), nil
}

// ParseSingleQuoted handles single-quoted values (no escape processing).
func ParseSingleQuoted(value string) (string, error) {
	if len(value) < 2 || value[len(value)-1] != '\'' {
		return "", ErrInvalidValue
	}

	// Single quotes don't process escapes
	return value[1 : len(value)-1], nil
}

// escapeTable provides O(1) lookup for escape sequence translation.
// Index by escaped character byte value, returns the translated character.
// 0 means pass through the character as-is.
var escapeTable = [256]byte{
	'n': '\n', 'r': '\r', 't': '\t',
	'\\': '\\', '"': '"', '$': '$',
}

// ParseDoubleQuotedBytes handles double-quoted values from a byte slice with escape sequences.
func ParseDoubleQuotedBytes(value []byte) (string, error) {
	if len(value) < 2 || value[len(value)-1] != '"' {
		return "", ErrInvalidValue
	}

	// Extract content between quotes
	content := value[1 : len(value)-1]

	// Fast path: no escape sequences - must copy because content is a slice
	// of the scanner buffer which will be reused on the next Scan() call.
	// Using bytesToString here would cause data corruption.
	if bytes.IndexByte(content, '\\') == -1 {
		return string(content), nil
	}

	// Use pooled byte slice for escaped content (more efficient than strings.Builder)
	buf := GetByteSlice()
	defer PutByteSlice(buf)

	// Ensure capacity - allocate exactly what we need, no more
	if cap(*buf) < len(content) {
		*buf = make([]byte, 0, len(content))
	}
	*buf = (*buf)[:0]

	// Optimized escape processing with lookup table
	for i := 0; i < len(content); {
		c := content[i]
		if c == '\\' && i+1 < len(content) {
			escaped := content[i+1]
			// Use lookup table for O(1) escape translation
			if translated := escapeTable[escaped]; translated != 0 {
				*buf = append(*buf, translated)
			} else {
				*buf = append(*buf, escaped)
			}
			i += 2
		} else {
			*buf = append(*buf, c)
			i++
		}
	}

	// SECURITY: Must copy the result because buf is a pooled buffer that will be
	// returned to the pool and potentially reused by other goroutines.
	// Using bytesToString here would cause data corruption when the buffer is reused.
	return string(*buf), nil
}

// ParseSingleQuotedBytes handles single-quoted values from a byte slice (no escape processing).
func ParseSingleQuotedBytes(value []byte) (string, error) {
	if len(value) < 2 || value[len(value)-1] != '\'' {
		return "", ErrInvalidValue
	}

	// SECURITY: Must copy the value because it's a slice of the scanner buffer.
	// Using bytesToString here would cause data corruption when the scanner buffer is reused.
	return string(value[1 : len(value)-1]), nil
}

// TryParseYamlValueBytes attempts to parse YAML-style values from a byte slice.
func TryParseYamlValueBytes(value []byte) (string, bool) {
	// Check for YAML boolean/null values using byte-level comparison
	if len(value) == 4 {
		if (value[0] == 't' || value[0] == 'T') && (value[1] == 'r' || value[1] == 'R') &&
			(value[2] == 'u' || value[2] == 'U') && (value[3] == 'e' || value[3] == 'E') {
			return string(value), true
		}
		if (value[0] == 'n' || value[0] == 'N') && (value[1] == 'u' || value[1] == 'U') &&
			(value[2] == 'l' || value[2] == 'L') && (value[3] == 'l' || value[3] == 'L') {
			return "", true
		}
	}
	if len(value) == 5 {
		if (value[0] == 'f' || value[0] == 'F') && (value[1] == 'a' || value[1] == 'A') &&
			(value[2] == 'l' || value[2] == 'L') && (value[3] == 's' || value[3] == 'S') &&
			(value[4] == 'e' || value[4] == 'E') {
			return string(value), true
		}
	}
	if len(value) == 1 && value[0] == '~' {
		return "", true
	}

	// Check for YAML numbers
	if IsYamlNumberBytes(value) {
		return string(value), true
	}

	return "", false
}

// IsYamlNumberBytes checks if a byte slice is a valid YAML number.
func IsYamlNumberBytes(s []byte) bool {
	if len(s) == 0 {
		return false
	}

	// Simple check for integer/float patterns
	hasDigit := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= '0' && c <= '9' {
			hasDigit = true
		} else if c != '-' && c != '+' && c != '.' && c != 'e' && c != 'E' {
			return false
		}
	}
	return hasDigit
}

// TryParseYamlValue attempts to parse YAML-style values.
func TryParseYamlValue(value string) (string, bool) {
	// Check for YAML boolean/null values
	switch strings.ToLower(value) {
	case "true", "false":
		return value, true
	case "null", "~":
		return "", true
	}

	// Check for YAML numbers
	if IsYamlNumber(value) {
		return value, true
	}

	return "", false
}

// IsYamlNumber checks if a string is a valid YAML number.
func IsYamlNumber(s string) bool {
	if s == "" {
		return false
	}

	// Simple check for integer/float patterns
	hasDigit := false
	for _, c := range s {
		if c >= '0' && c <= '9' {
			hasDigit = true
		} else if c != '-' && c != '+' && c != '.' && c != 'e' && c != 'E' {
			return false
		}
	}
	return hasDigit
}

// ExpandAll expands all variables in the map.
// Returns the original map if no expansion is needed, avoiding unnecessary allocations.
func (p *LineParser) ExpandAll(vars map[string]string) (map[string]string, error) {
	// Fast path: first check if any values need expansion
	// This avoids expensive cycle detection when no expansion is needed
	needsExpansion := false
	for _, value := range vars {
		if strings.IndexByte(value, '$') != -1 {
			needsExpansion = true
			break
		}
	}

	// No expansion needed - return original map without copying
	if !needsExpansion {
		return vars, nil
	}

	// Check for cycles only when expansion is needed
	if cycle, found := DetectCycle(vars); found {
		p.auditor.LogError(ActionExpand, cycle, "cycle detected")
		return nil, &ExpansionError{
			Key:   cycle,
			Chain: "cycle detected",
		}
	}

	// Pre-allocate result map with exact size
	result := make(map[string]string, len(vars))

	// Expand all values
	for key, value := range vars {
		expanded, err := p.expander.Expand(value)
		if err != nil {
			p.auditor.LogError(ActionExpand, key, err.Error())
			return nil, err
		}
		// Store expanded value (may be same as original)
		result[key] = expanded
	}

	return result, nil
}

// keysToUpperImpl is the shared implementation for converting map keys to uppercase.
// The result map is provided by the caller to allow both pooled and non-pooled variants.
func keysToUpperImpl(m map[string]string, result map[string]bool) {
	for k := range m {
		if k == "" {
			continue
		}
		result[ToUpperASCII(k)] = true
	}
}

// KeysToUpper converts map keys to uppercase for comparison.
// This function is optimized to minimize allocations.
// The caller owns the returned map and does not need to return it to a pool.
func KeysToUpper(m map[string]string) map[string]bool {
	result := make(map[string]bool, len(m))
	keysToUpperImpl(m, result)
	return result
}

// KeysToUpperPooled converts map keys to uppercase using a pooled map.
// The returned map MUST be returned to the pool using PutKeysToUpperMap after use.
// This reduces allocations when the map is only needed temporarily.
//
// Example:
//
//	upperKeys := KeysToUpperPooled(vars)
//	defer PutKeysToUpperMap(upperKeys)
//	// use upperKeys...
func KeysToUpperPooled(m map[string]string) map[string]bool {
	result := getKeysToUpperMap()
	keysToUpperImpl(m, result)
	return result
}

// PutKeysToUpperMap returns a pooled map obtained from KeysToUpperPooled.
// It is safe to call with nil.
func PutKeysToUpperMap(m map[string]bool) {
	putKeysToUpperMap(m)
}
