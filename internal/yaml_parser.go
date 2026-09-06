// Package internal provides YAML parsing utilities.
package internal

import (
	"fmt"
	"sync"
)

// ValueType represents the type of a YAML value.
type ValueType int

const (
	// ValueTypeScalar represents a scalar value (string, number, bool, null).
	ValueTypeScalar ValueType = iota
	// ValueTypeMap represents a map/object.
	ValueTypeMap
	// ValueTypeArray represents an array/list.
	ValueTypeArray
)

// Value represents a YAML value.
type Value struct {
	Type   ValueType
	Scalar string
	Map    map[string]*Value
	Array  []*Value
	Line   int
	Column int
	// Quoted reports that a ValueTypeScalar came from a quoted token.
	// YAML semantics: quoted scalars are always strings — no null/bool/number
	// coercion, and interior whitespace (including trailing) is significant.
	Quoted bool
}

// valuePool pools *Value nodes to reduce allocation pressure during YAML parsing.
var valuePool = sync.Pool{
	New: func() any {
		return &Value{}
	},
}

// NewScalarValue creates a new scalar value (from pool).
func NewScalarValue(s string, line, col int) *Value {
	v := valuePool.Get().(*Value)
	v.Type = ValueTypeScalar
	v.Scalar = s
	v.Line = line
	v.Column = col
	return v
}

// NewMapValue creates a new map value with pre-allocated capacity (from pool).
func NewMapValue(line, col int) *Value {
	return newMapValueCap(line, col, 8)
}

// newMapValueCap creates a pooled map value whose backing map is sized to
// hold at least size entries, avoiding incremental growth. Used for the
// document root where the entry count is estimable from the token count.
func newMapValueCap(line, col, size int) *Value {
	v := valuePool.Get().(*Value)
	v.Type = ValueTypeMap
	if v.Map == nil {
		v.Map = make(map[string]*Value, size)
	} else {
		// Reuse existing map after clearing
		clear(v.Map)
	}
	v.Line = line
	v.Column = col
	return v
}

// NewArrayValue creates a new array value with pre-allocated capacity (from pool).
func NewArrayValue(line, col int) *Value {
	v := valuePool.Get().(*Value)
	v.Type = ValueTypeArray
	if v.Array == nil {
		v.Array = make([]*Value, 0, 4)
	} else {
		v.Array = v.Array[:0]
	}
	v.Line = line
	v.Column = col
	return v
}

// Parser parses YAML tokens into a Value tree.
type Parser struct {
	tokens   []Token
	pos      int
	maxDepth int
}

// NewYAMLParser creates a new YAML parser.
func NewYAMLParser(tokens []Token, maxDepth int) *Parser {
	if maxDepth <= 0 {
		maxDepth = 10
	}
	return &Parser{
		tokens:   tokens,
		maxDepth: maxDepth,
	}
}

// Parse parses the tokens and returns the root value.
func (p *Parser) Parse() (*Value, error) {
	return p.parseDocument(0)
}

// parseDocument parses a YAML document.
func (p *Parser) parseDocument(depth int) (*Value, error) {
	if depth >= p.maxDepth {
		return nil, &YAMLError{
			Message: fmt.Sprintf("maximum nesting depth exceeded (%d)", p.maxDepth),
		}
	}

	// Skip document start markers
	p.skipDocumentStarts()

	// Skip leading newlines
	p.skipNewlines()

	if p.isEOF() {
		return NewMapValue(1, 1), nil
	}

	// Parse root level
	return p.parseMap(depth, 0)
}

// parseMap parses a YAML map.
func (p *Parser) parseMap(depth, expectedIndent int) (*Value, error) {
	var root *Value
	if depth == 0 {
		// Root document map: pre-size from the token count (~3 tokens per
		// entry: newline, key, value) so large flat documents don't pay for
		// repeated map growth. Nested maps keep the small default — depth 0
		// is reachable only from parseDocument.
		root = newMapValueCap(p.currentLine(), p.currentColumn(), len(p.tokens)/3)
	} else {
		root = NewMapValue(p.currentLine(), p.currentColumn())
	}

	for !p.isEOF() {
		// Skip newlines
		p.skipNewlines()

		if p.isEOF() {
			break
		}

		// Check for document start
		if p.current().Type == TokenDocumentStart {
			break
		}

		// Check for dedent (end of this map)
		if p.current().Type == TokenDedent {
			p.advance()
			break
		}

		// Check indentation
		currentIndent := p.current().Indent
		if currentIndent < expectedIndent {
			// End of current map level
			break
		}

		// Skip indents
		if p.current().Type == TokenIndent {
			p.advance()
			continue
		}

		// Parse key-value pair or array item
		if p.current().Type == TokenKey {
			key := p.current().Value
			p.advance()

			// Skip newlines after colon
			p.skipNewlines()

			var value *Value
			var err error

			if p.isEOF() {
				value = NewScalarValue("", p.currentLine(), p.currentColumn())
			} else if p.current().Type == TokenIndent {
				// Nested structure
				p.advance()
				value, err = p.parseNestedValue(depth+1, currentIndent+1)
			} else if p.current().Type == TokenValue {
				value = p.scalarFromToken()
			} else if p.current().Type == TokenComment {
				// Comment line(s) directly after the colon. A nested block
				// may still follow after the newline and any further comment
				// lines (see consumeCommentsAfterKey).
				if p.consumeCommentsAfterKey() {
					value, err = p.parseNestedValue(depth+1, currentIndent+1)
				} else {
					value = NewScalarValue("", p.currentLine(), p.currentColumn())
				}
			} else if p.current().Type == TokenDash {
				// Array as value
				value, err = p.parseArray(depth+1, currentIndent+1)
			} else if p.current().Type == TokenKey {
				// Inline nested map
				value, err = p.parseMap(depth+1, currentIndent+1)
			} else {
				value = NewScalarValue("", p.currentLine(), p.currentColumn())
			}

			if err != nil {
				return nil, err
			}

			root.Map[key] = value
		} else if p.current().Type == TokenDash {
			// This shouldn't happen at map level without a key
			break
		} else if p.current().Type == TokenComment {
			p.advance()
		} else {
			// Unknown token, skip
			p.advance()
		}
	}

	return root, nil
}

// consumeCommentsAfterKey advances past the comment lines that may follow a
// key's colon (e.g. "key: # comment"). It reports whether a nested block was
// found after the comments, in which case the block's opening Indent token
// has been consumed and the caller should parse the nested value.
//
// This is the single implementation of the comment/newline/nested sequencing
// rules for values after a key. parseMap's early skipNewlines (run right
// after the key) already consumes a bare "key:\n  nested" sequence, so this
// helper only fires for comments — the former TokenNewline branch in the
// value chain was unreachable after that skip and has been removed.
func (p *Parser) consumeCommentsAfterKey() bool {
	for p.current().Type == TokenComment {
		p.advance()
	}
	if p.current().Type != TokenNewline {
		return false
	}
	p.skipNewlines()
	for p.current().Type == TokenComment {
		p.advance()
		p.skipNewlines()
	}
	if p.current().Type == TokenIndent {
		p.advance()
		return true
	}
	return false
}

// parseNestedValue parses a nested value (could be map, array, or scalar).
func (p *Parser) parseNestedValue(depth, expectedIndent int) (*Value, error) {
	if depth >= p.maxDepth {
		return nil, &YAMLError{
			Message: fmt.Sprintf("maximum nesting depth exceeded (%d)", p.maxDepth),
		}
	}

	p.skipNewlines()

	if p.isEOF() {
		return NewScalarValue("", p.currentLine(), p.currentColumn()), nil
	}

	// Check if we have a nested structure
	if p.current().Type == TokenIndent {
		p.advance()
	}

	// Skip comments iteratively (not recursively to avoid stack overflow
	// on pathological input with many consecutive comments).
	// After each comment, re-skip newlines and indents since the recursive
	// version would re-enter the function and handle those.
	for p.current().Type == TokenComment {
		p.advance()
		p.skipNewlines()
		if p.current().Type == TokenIndent {
			p.advance()
		}
	}

	// Check what follows
	if p.current().Type == TokenKey {
		return p.parseMap(depth, expectedIndent)
	} else if p.current().Type == TokenDash {
		return p.parseArray(depth, expectedIndent)
	} else if p.current().Type == TokenValue {
		return p.scalarFromToken(), nil
	} else if p.current().Type == TokenDedent {
		// The nested block turned out to be empty (e.g. it held only
		// comments) and this Dedent closes it. Consume it here so the
		// caller resumes at the dedented level — leaving it unconsumed
		// made the next map up treat its own scope as ended and silently
		// dropped sibling keys after the comment block.
		p.advance()
		return NewScalarValue("", p.currentLine(), p.currentColumn()), nil
	}

	return NewScalarValue("", p.currentLine(), p.currentColumn()), nil
}

// parseArray parses a YAML array.
func (p *Parser) parseArray(depth, expectedIndent int) (*Value, error) {
	if depth >= p.maxDepth {
		return nil, &YAMLError{
			Message: fmt.Sprintf("maximum nesting depth exceeded (%d)", p.maxDepth),
		}
	}

	root := NewArrayValue(p.currentLine(), p.currentColumn())

	for !p.isEOF() {
		p.skipNewlines()

		if p.isEOF() {
			break
		}

		// Check for document start
		if p.current().Type == TokenDocumentStart {
			break
		}

		// Check for dedent
		if p.current().Type == TokenDedent {
			p.advance()
			break
		}

		// Check indentation
		currentIndent := p.current().Indent
		if currentIndent < expectedIndent {
			break
		}

		// Skip indents
		if p.current().Type == TokenIndent {
			p.advance()
			continue
		}

		// Check for dash (array item)
		if p.current().Type == TokenDash {
			p.advance()

			var value *Value
			var err error

			if p.isEOF() {
				value = NewScalarValue("", p.currentLine(), p.currentColumn())
			} else if p.current().Type == TokenNewline {
				// Multi-line array item
				p.advance()
				p.skipNewlines()
				if p.current().Type == TokenIndent {
					p.advance()
					value, err = p.parseNestedValue(depth+1, currentIndent+1)
				} else {
					value = NewScalarValue("", p.currentLine(), p.currentColumn())
				}
			} else if p.current().Type == TokenIndent {
				p.advance()
				value, err = p.parseNestedValue(depth+1, currentIndent+1)
			} else if p.current().Type == TokenValue {
				value = p.scalarFromToken()
			} else if p.current().Type == TokenKey {
				// Map as array item (KEY at same indent level as DASH)
				value, err = p.parseMap(depth+1, currentIndent)
			} else if p.current().Type == TokenDash {
				// Nested array. In "- - a" the second dash sits at the SAME
				// indent level as the first (no Indent token separates them),
				// so the nested array starts at currentIndent — the previous
				// currentIndent+1 made the nested array's own first dash fail
				// its indent guard, yielding an empty item and letting the
				// scalars spill into the outer array.
				value, err = p.parseArray(depth+1, currentIndent)
			} else {
				value = NewScalarValue("", p.currentLine(), p.currentColumn())
			}

			if err != nil {
				return nil, err
			}

			root.Array = append(root.Array, value)
		} else if p.current().Type == TokenComment {
			p.advance()
		} else {
			// Not an array item, end of array
			break
		}
	}

	return root, nil
}

// eofToken is the shared end-of-input sentinel returned by current().
// It is read-only: parsers never mutate tokens.
var eofToken = Token{Type: TokenEOF}

// scalarFromToken creates a scalar Value from the current TokenValue token,
// preserving the token's quoting flag (a quoted scalar must never be
// type-coerced downstream — see Value.Quoted), and advances past it.
func (p *Parser) scalarFromToken() *Value {
	tok := p.current()
	v := NewScalarValue(tok.Value, tok.Line, tok.Column)
	v.Quoted = tok.IsQuoted
	p.advance()
	return v
}

// current returns a pointer to the current token.
// The tokens slice is never mutated during parsing, so the pointer stays
// valid until the next advance(). Returning a pointer avoids copying the
// 56-byte Token struct on every check — current() is called several times
// per token in the parse loops, making those copies a measured hotspot.
func (p *Parser) current() *Token {
	if p.pos >= len(p.tokens) {
		return &eofToken
	}
	return &p.tokens[p.pos]
}

// advance moves to the next token.
func (p *Parser) advance() {
	if p.pos < len(p.tokens) {
		p.pos++
	}
}

// isEOF checks if we've reached the end of tokens.
func (p *Parser) isEOF() bool {
	return p.pos >= len(p.tokens) || p.tokens[p.pos].Type == TokenEOF
}

// currentLine returns the current line number.
func (p *Parser) currentLine() int {
	if p.isEOF() {
		return 0
	}
	return p.tokens[p.pos].Line
}

// currentColumn returns the current column number.
func (p *Parser) currentColumn() int {
	if p.isEOF() {
		return 0
	}
	return p.tokens[p.pos].Column
}

// skipNewlines skips newline tokens.
func (p *Parser) skipNewlines() {
	for !p.isEOF() && p.current().Type == TokenNewline {
		p.advance()
	}
}

// skipDocumentStarts skips document start markers.
func (p *Parser) skipDocumentStarts() {
	for !p.isEOF() && p.current().Type == TokenDocumentStart {
		p.advance()
	}
}

// ReleaseValue recursively returns a Value tree to the pool.
// After calling this, the Value and all its children must not be accessed.
func ReleaseValue(v *Value) {
	if v == nil {
		return
	}
	switch v.Type {
	case ValueTypeMap:
		for key, child := range v.Map {
			ReleaseValue(child)
			delete(v.Map, key)
		}
		v.Map = nil
	case ValueTypeArray:
		for _, child := range v.Array {
			ReleaseValue(child)
		}
		v.Array = nil
	}
	v.Scalar = ""
	v.Quoted = false
	v.Type = ValueTypeScalar
	valuePool.Put(v)
}

// ParseYAML parses YAML input and returns a Value tree.
// The caller MUST call ReleaseValue on the returned tree when done
// to return nodes to the pool and prevent memory growth.
func ParseYAML(data []byte, maxDepth int) (*Value, error) {
	// Lex directly over the caller's buffer (no full-file copy): every token
	// value is copied out as an independent string during tokenization, and
	// the Value tree holds only those copies, so aliasing data is safe for
	// the duration of this call.
	lexer := newYAMLLexer(data)
	// Seed tokenization with a pooled token slice — the largest per-parse
	// allocation (see tokenSlicePool).
	tokensPtr := getTokenSlice()
	tokens, err := lexer.tokenizeInto(*tokensPtr)
	*tokensPtr = tokens // tokenizeInto may reallocate; pool the final slice
	lexer.release()
	if err != nil {
		putTokenSlice(tokensPtr)
		return nil, err
	}

	parser := NewYAMLParser(tokens, maxDepth)
	value, perr := parser.Parse()
	putTokenSlice(tokensPtr)
	if perr != nil {
		return nil, perr
	}
	return value, nil
}
