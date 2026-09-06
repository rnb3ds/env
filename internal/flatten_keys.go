// Package internal provides the shared flat-key constructors used by the
// JSON and YAML flatteners. Both flatten nested documents into flat maps
// using the same "PREFIX<delim>KEY" and "PREFIX<delim|bracket>INDEX"
// conventions; keeping the constructors in one place prevents the two
// formats' key layouts from drifting apart.
package internal

import "strconv"

// buildFlatKey joins prefix and an uppercased key with the delimiter.
// Uses direct concatenation for the common case (short keys) to avoid pool
// overhead; longer keys go through a pooled builder.
func buildFlatKey(prefix, key, delimiter string) string {
	key = ToUpperASCII(key)
	if prefix == "" {
		return key
	}
	totalLen := len(prefix) + len(delimiter) + len(key)
	if totalLen <= 64 {
		return prefix + delimiter + key
	}
	sb := GetBuilder()
	defer PutBuilder(sb)
	sb.Grow(totalLen)
	sb.WriteString(prefix)
	sb.WriteString(delimiter)
	sb.WriteString(key)
	return sb.String()
}

// buildFlatArrayIndex constructs a key for array elements. indexFormat
// "bracket" renders "prefix[index]"; anything else (the "underscore"
// default) renders "prefix<delimiter>index".
func buildFlatArrayIndex(prefix string, index int, delimiter, indexFormat string) string {
	indexStr := strconv.Itoa(index)

	switch indexFormat {
	case "bracket":
		totalLen := len(prefix) + 1 + len(indexStr) + 1
		if totalLen <= 64 {
			return prefix + "[" + indexStr + "]"
		}
		sb := GetBuilder()
		defer PutBuilder(sb)
		sb.Grow(totalLen)
		sb.WriteString(prefix)
		sb.WriteByte('[')
		sb.WriteString(indexStr)
		sb.WriteByte(']')
		return sb.String()
	default:
		totalLen := len(prefix) + len(delimiter) + len(indexStr)
		if totalLen <= 64 {
			return prefix + delimiter + indexStr
		}
		sb := GetBuilder()
		defer PutBuilder(sb)
		sb.Grow(totalLen)
		sb.WriteString(prefix)
		sb.WriteString(delimiter)
		sb.WriteString(indexStr)
		return sb.String()
	}
}
