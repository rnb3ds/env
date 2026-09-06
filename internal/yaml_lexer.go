// Package internal provides YAML lexing utilities.
package internal

import (
	"bytes"
	"fmt"
	"sync"
)

// lexerBufferPool provides a pool of reusable bytes.Buffer instances.
// This reduces allocations for frequent buffer operations in the YAML lexer.
var lexerBufferPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

// getLexerBuffer retrieves a bytes.Buffer from the pool.
// The buffer is reset before use.
func getLexerBuffer() *bytes.Buffer {
	buf, ok := lexerBufferPool.Get().(*bytes.Buffer)
	if !ok {
		// Fallback: create new buffer if pool returns unexpected type
		return new(bytes.Buffer)
	}
	buf.Reset()
	return buf
}

// putLexerBuffer returns a bytes.Buffer to the pool.
// Buffers with capacity exceeding 4096 bytes are discarded
// to prevent memory bloat.
func putLexerBuffer(buf *bytes.Buffer) {
	if buf == nil {
		return
	}
	// Don't pool very large buffers
	if buf.Cap() <= 4096 {
		lexerBufferPool.Put(buf)
	}
}

// TokenType represents the type of a YAML token.
type TokenType int

const (
	// TokenEOF marks the end of input.
	TokenEOF TokenType = iota
	// TokenNewline represents a newline character.
	TokenNewline
	// TokenDocumentStart represents a document separator (---).
	TokenDocumentStart
	// TokenKey represents a key in a key-value pair.
	TokenKey
	// TokenValue represents a scalar value.
	TokenValue
	// TokenDash represents the start of an array item.
	TokenDash
	// TokenColon represents the colon separator. It is never emitted by the
	// Lexer (scanKey consumes the colon inline); the constant is kept as a
	// synthetic "unknown token" fixture for parser tests.
	TokenColon
	// TokenComment represents a comment.
	TokenComment
	// TokenIndent represents increased indentation.
	TokenIndent
	// TokenDedent represents decreased indentation.
	TokenDedent
)

// Token represents a YAML token.
type Token struct {
	Type     TokenType
	Value    string
	Line     int
	Column   int
	Indent   int // Indentation level (number of spaces / 2)
	IsQuoted bool
}

// lexerPool pools Lexer structs to avoid per-parse allocation.
var lexerPool = sync.Pool{
	New: func() any {
		return &Lexer{
			indents: make([]int, 1, 8),
		}
	},
}

// Lexer tokenizes YAML input.
type Lexer struct {
	input    []byte
	pos      int
	line     int
	column   int
	indent   int     // Current indentation level
	indents  []int   // Stack of indentation levels
	tokens   []Token // Buffered tokens
	buffered bool    // Whether we have buffered tokens
	eof      bool    // Whether we've reached EOF
}

// newYAMLLexer creates a lexer over a byte slice without copying it.
// This is the hot-path entry used by ParseYAML: the input aliases the
// caller's read buffer, which stays valid for the duration of Tokenize
// because every token value is copied out as an independent string
// (see trimInputRange and scanQuotedString).
func newYAMLLexer(input []byte) *Lexer {
	l, ok := lexerPool.Get().(*Lexer)
	if !ok {
		l = &Lexer{
			indents: make([]int, 1, 8),
		}
	}
	l.input = input
	l.pos = 0
	l.line = 1
	l.column = 1
	l.indent = 0
	l.indents = l.indents[:1]
	l.indents[0] = 0
	l.tokens = nil
	l.buffered = false
	l.eof = false
	return l
}

// release returns the Lexer to the pool.
func (l *Lexer) release() {
	l.input = nil
	l.tokens = nil
	lexerPool.Put(l)
}

// NextToken returns the next token from the input.
func (l *Lexer) NextToken() (Token, error) {
	if l.buffered && len(l.tokens) > 0 {
		t := l.tokens[0]
		l.tokens = l.tokens[1:]
		if len(l.tokens) == 0 {
			l.buffered = false
		}
		return t, nil
	}

	if l.eof {
		return Token{Type: TokenEOF, Line: l.line, Column: l.column}, nil
	}

	return l.nextToken()
}

// nextToken reads and returns the next token.
func (l *Lexer) nextToken() (Token, error) {
	// Skip whitespace except newlines
	l.skipSpaces()

	if l.pos >= len(l.input) {
		l.eof = true
		// Emit any pending dedents
		if len(l.indents) > 1 {
			l.indents = l.indents[:len(l.indents)-1]
			return Token{Type: TokenDedent, Line: l.line, Column: l.column, Indent: l.indents[len(l.indents)-1]}, nil
		}
		return Token{Type: TokenEOF, Line: l.line, Column: l.column}, nil
	}

	ch := l.input[l.pos]

	// Handle newlines
	if ch == '\n' {
		return l.handleNewline()
	}

	// Handle carriage return (CRLF or CR only)
	if ch == '\r' {
		// Don't skip here - let handleNewline do it
		return l.handleNewline()
	}

	// Handle document separator
	if ch == '-' && l.isDocumentStart() {
		return l.scanDocumentStart()
	}

	// Handle comment
	if ch == '#' {
		return l.scanComment()
	}

	// Handle array item
	if ch == '-' && l.isDashStart() {
		return l.scanDash()
	}

	// Handle key-value pair
	if l.isKeyStart() {
		return l.scanKey()
	}

	// Handle value (scalar)
	return l.scanValue()
}

// skipSpaces skips space and tab characters.
func (l *Lexer) skipSpaces() {
	for l.pos < len(l.input) {
		ch := l.input[l.pos]
		if ch == ' ' || ch == '\t' {
			l.pos++
			l.column++
		} else {
			break
		}
	}
}

// handleNewline processes a newline and handles indentation changes.
func (l *Lexer) handleNewline() (Token, error) {
	startLine := l.line
	l.line++
	l.column = 1

	// Skip the newline character(s)
	if l.pos < len(l.input) && l.input[l.pos] == '\r' {
		l.pos++
		if l.pos < len(l.input) && l.input[l.pos] == '\n' {
			l.pos++
		}
	} else if l.pos < len(l.input) && l.input[l.pos] == '\n' {
		l.pos++
	}

	// Count indentation of next line
	newIndent := 0
	for l.pos < len(l.input) {
		ch := l.input[l.pos]
		if ch == ' ' {
			newIndent++
			l.pos++
		} else if ch == '\t' {
			// Treat tab as 2 spaces
			newIndent += 2
			l.pos++
		} else if ch == '\n' || ch == '\r' {
			// Empty line, continue to next
			if ch == '\r' && l.pos+1 < len(l.input) && l.input[l.pos+1] == '\n' {
				l.pos++
			}
			l.pos++
			l.line++
			l.column = 1
			newIndent = 0
		} else {
			break
		}
	}

	// Normalize indentation to levels (every 2 spaces = 1 level)
	newLevel := newIndent / 2
	currentLevel := l.indents[len(l.indents)-1]

	if newLevel > currentLevel {
		// SECURITY: Limit indent stack depth to prevent unbounded memory growth
		// from malicious YAML with monotonically increasing indentation.
		const maxIndentDepth = 256
		if len(l.indents) >= maxIndentDepth {
			return Token{}, &YAMLError{
				Line:    startLine,
				Column:  l.column,
				Message: fmt.Sprintf("maximum indent depth exceeded (%d)", maxIndentDepth),
			}
		}
		// Indentation increased
		l.indents = append(l.indents, newLevel)
		l.indent = newLevel
		// Buffer newline and indent
		l.tokens = append(l.tokens, Token{Type: TokenIndent, Line: l.line, Column: 1, Indent: newLevel})
		l.buffered = true
		return Token{Type: TokenNewline, Line: startLine, Column: l.column}, nil
	} else if newLevel < currentLevel {
		// Indentation decreased - may need multiple dedents
		// Save the levels we're dedenting from before modifying the stack
		var dedentLevels []int
		for len(l.indents) > 1 && l.indents[len(l.indents)-1] > newLevel {
			dedentLevels = append(dedentLevels, l.indents[len(l.indents)-1])
			l.indents = l.indents[:len(l.indents)-1]
		}
		l.indent = l.indents[len(l.indents)-1]

		// Buffer dedents with correct levels (in reverse order)
		for i := len(dedentLevels) - 1; i >= 0; i-- {
			l.tokens = append(l.tokens, Token{Type: TokenDedent, Line: l.line, Column: 1, Indent: dedentLevels[i]})
		}
		if len(dedentLevels) > 0 {
			l.buffered = true
		}
		return Token{Type: TokenNewline, Line: startLine, Column: l.column}, nil
	}

	return Token{Type: TokenNewline, Line: startLine, Column: l.column}, nil
}

// isDocumentStart checks if current position is a document separator.
func (l *Lexer) isDocumentStart() bool {
	if l.pos+2 >= len(l.input) {
		return false
	}
	// Check for ---
	if l.input[l.pos] == '-' && l.input[l.pos+1] == '-' && l.input[l.pos+2] == '-' {
		// Must be at start of line or after whitespace
		if l.column == 1 || (l.pos > 0 && (l.input[l.pos-1] == ' ' || l.input[l.pos-1] == '\t')) {
			// Check what follows
			if l.pos+3 >= len(l.input) {
				return true
			}
			next := l.input[l.pos+3]
			return next == ' ' || next == '\t' || next == '\n' || next == '\r'
		}
	}
	return false
}

// scanDocumentStart scans a document separator.
func (l *Lexer) scanDocumentStart() (Token, error) {
	startCol := l.column
	l.pos += 3
	l.column += 3
	return Token{Type: TokenDocumentStart, Line: l.line, Column: startCol}, nil
}

// isDashStart checks if dash is start of array item.
func (l *Lexer) isDashStart() bool {
	if l.pos+1 >= len(l.input) {
		return true // Last character dash
	}
	next := l.input[l.pos+1]
	return next == ' ' || next == '\t' || next == '\n' || next == '\r'
}

// scanDash scans an array item marker.
func (l *Lexer) scanDash() (Token, error) {
	startCol := l.column
	l.pos++
	l.column++
	return Token{Type: TokenDash, Line: l.line, Column: startCol, Indent: l.indent}, nil
}

// isKeyStart checks if we're at the start of a key.
func (l *Lexer) isKeyStart() bool {
	// Look for a colon before newline
	for i := l.pos; i < len(l.input); i++ {
		ch := l.input[i]
		if ch == ':' {
			// Check if colon is followed by space, newline, or end
			if i+1 >= len(l.input) {
				return true
			}
			next := l.input[i+1]
			return next == ' ' || next == '\t' || next == '\n' || next == '\r'
		}
		if ch == '\n' || ch == '\r' {
			return false
		}
		// Quoted key
		if ch == '"' || ch == '\'' {
			// Find end of quote
			quote := ch
			i++
			for i < len(l.input) && l.input[i] != quote {
				if l.input[i] == '\\' {
					i++
				}
				i++
			}
		}
	}
	return false
}

// scanKey scans a key token.
func (l *Lexer) scanKey() (Token, error) {
	startLine := l.line
	startCol := l.column

	// Handle quoted keys
	if l.pos < len(l.input) {
		ch := l.input[l.pos]
		if ch == '"' || ch == '\'' {
			str, err := l.scanQuotedString(ch)
			if err != nil {
				return Token{}, err
			}
			// Skip colon and whitespace
			l.skipSpaces()
			if l.pos < len(l.input) && l.input[l.pos] == ':' {
				l.pos++
				l.column++
			}
			return Token{Type: TokenKey, Value: str, Line: startLine, Column: startCol, Indent: l.indent, IsQuoted: true}, nil
		}
	}

	// Unquoted key: scan by index and slice the input once instead of copying
	// byte-by-byte through a pooled buffer. The string() conversion makes an
	// exact-size copy so the token value stays independent of l.input.
	contentStart := l.pos
	contentEnd := l.pos
	for l.pos < len(l.input) {
		ch := l.input[l.pos]
		if ch == ':' {
			// Check if colon is followed by space, newline, or end
			if l.pos+1 >= len(l.input) {
				contentEnd = l.pos
				l.pos++
				break
			}
			next := l.input[l.pos+1]
			if next == ' ' || next == '\t' || next == '\n' || next == '\r' {
				contentEnd = l.pos
				l.pos++
				l.column++
				break
			}
		}
		if ch == '\n' || ch == '\r' {
			contentEnd = l.pos
			break
		}
		if ch == '#' && isCommentStart(l.input, l.pos) {
			// Same whitespace rule as scanValue: '#' after whitespace ends the
			// key and starts a comment; '#' mid-token stays part of the key and
			// is then rejected by key validation as an explicit error instead
			// of silently truncating the key.
			contentEnd = l.pos
			break
		}
		l.pos++
		l.column++
		contentEnd = l.pos
	}

	return Token{Type: TokenKey, Value: trimInputRange(l.input, contentStart, contentEnd), Line: startLine, Column: startCol, Indent: l.indent}, nil
}

// trimInputRange trims ASCII spaces and tabs (and, defensively, CR/LF) from
// both ends of input[start:end] and returns an exact-size copy.
// It replaces the previous buffered WriteByte round-trip: the scanned region
// can never contain a newline (the scanners stop there), so the same trim
// semantics apply with one allocation and no buffer churn.
//
// SECURITY/RETENTION: the string() copy is deliberate — returning the
// substring input[start:end] would be zero-copy but would keep the entire
// input alive for as long as any token value (and any result-map entry built
// from it) is referenced.
func trimInputRange(input []byte, start, end int) string {
	for start < end {
		c := input[start]
		if c != ' ' && c != '\t' && c != '\n' && c != '\r' {
			break
		}
		start++
	}
	for end > start {
		c := input[end-1]
		if c != ' ' && c != '\t' && c != '\n' && c != '\r' {
			break
		}
		end--
	}
	return string(input[start:end])
}

// isCommentStart reports whether the '#' at input[pos] begins a YAML comment.
// Per the YAML spec a comment starts at the beginning of a line or where '#'
// is preceded by whitespace; '#' anywhere else is plain-scalar data. pos == 0
// counts as a comment start (beginning of input/line).
func isCommentStart(input []byte, pos int) bool {
	if pos == 0 {
		return true
	}
	switch input[pos-1] {
	case ' ', '\t', '\n', '\r':
		return true
	}
	return false
}

// scanValue scans a scalar value.
func (l *Lexer) scanValue() (Token, error) {
	startLine := l.line
	startCol := l.column

	// Skip leading spaces
	l.skipSpaces()

	if l.pos >= len(l.input) {
		return Token{Type: TokenEOF, Line: l.line, Column: l.column}, nil
	}

	// Handle quoted strings
	ch := l.input[l.pos]
	if ch == '"' || ch == '\'' {
		str, err := l.scanQuotedString(ch)
		if err != nil {
			return Token{}, err
		}
		return Token{Type: TokenValue, Value: str, Line: startLine, Column: startCol, Indent: l.indent, IsQuoted: true}, nil
	}

	// Unquoted value: scan by index and slice the input once instead of
	// copying byte-by-byte through a pooled buffer (see trimInputRange).
	contentStart := l.pos
	contentEnd := l.pos
	for l.pos < len(l.input) {
		ch := l.input[l.pos]
		if ch == '\n' || ch == '\r' {
			contentEnd = l.pos
			break
		}
		if ch == '#' && isCommentStart(l.input, l.pos) {
			// YAML starts a comment only where '#' is preceded by whitespace
			// (or begins the value). '#' inside a plain scalar — pa#ss,
			// http://host/p#frag — is data, not a comment marker.
			contentEnd = l.pos
			break
		}
		if ch == ':' {
			// Check if this looks like a nested key (colon followed by space/newline)
			if l.pos+1 < len(l.input) {
				next := l.input[l.pos+1]
				if next == ' ' || next == '\t' || next == '\n' || next == '\r' {
					// This is a key, stop here
					contentEnd = l.pos
					break
				}
			}
		}
		l.pos++
		l.column++
		contentEnd = l.pos
	}

	return Token{Type: TokenValue, Value: trimInputRange(l.input, contentStart, contentEnd), Line: startLine, Column: startCol, Indent: l.indent}, nil
}

// scanQuotedString scans a quoted string.
func (l *Lexer) scanQuotedString(quote byte) (string, error) {
	l.pos++ // Skip opening quote
	l.column++

	buf := getLexerBuffer()
	defer putLexerBuffer(buf)
	for l.pos < len(l.input) {
		ch := l.input[l.pos]
		if ch == quote {
			l.pos++
			l.column++
			return buf.String(), nil
		}
		if ch == '\\' {
			l.pos++
			l.column++
			if l.pos >= len(l.input) {
				return "", &YAMLError{
					Line:    l.line,
					Column:  l.column,
					Message: "unexpected end of input in escape sequence",
				}
			}
			escaped := l.input[l.pos]
			switch escaped {
			case 'n':
				buf.WriteByte('\n')
			case 't':
				buf.WriteByte('\t')
			case 'r':
				buf.WriteByte('\r')
			case '\\':
				buf.WriteByte('\\')
			case '"':
				buf.WriteByte('"')
			case '\'':
				buf.WriteByte('\'')
			// SECURITY: \0 escape is explicitly rejected at the lexer level for defense in depth.
			// Null bytes can cause:
			// - Log injection (log entries truncated at null byte)
			// - String truncation vulnerabilities in C interop
			// - Bypass of security controls that don't expect nulls
			// This provides early rejection before value validation runs.
			case '0':
				return "", &YAMLError{
					Line:    l.line,
					Column:  l.column,
					Message: "null byte escape (\\0) is not allowed in YAML values",
				}
			default:
				buf.WriteByte(escaped)
			}
			l.pos++
			l.column++
			continue
		}
		if ch == '\n' || ch == '\r' {
			return "", &YAMLError{
				Line:    l.line,
				Column:  l.column,
				Message: "unterminated string",
			}
		}
		buf.WriteByte(ch)
		l.pos++
		l.column++
	}

	return "", &YAMLError{
		Line:    l.line,
		Column:  l.column,
		Message: "unterminated string",
	}
}

// scanComment scans a comment.
func (l *Lexer) scanComment() (Token, error) {
	startLine := l.line
	startCol := l.column
	contentStart := l.pos + 1 // skip '#'
	l.pos++
	l.column++

	contentEnd := l.pos
	for l.pos < len(l.input) {
		ch := l.input[l.pos]
		if ch == '\n' || ch == '\r' {
			contentEnd = l.pos
			break
		}
		l.pos++
		l.column++
		contentEnd = l.pos
	}

	return Token{Type: TokenComment, Value: trimInputRange(l.input, contentStart, contentEnd), Line: startLine, Column: startCol, Indent: l.indent}, nil
}

// tokenSlicePool reuses the token slices produced by tokenization.
// A 100-variable document yields ~300 tokens ≈ 17KB — by far the largest
// single allocation of a YAML parse — so pooling it removes most of the
// per-parse allocation churn. The pool stores *[]Token because Tokenize may
// reallocate while appending; the caller writes the final slice back through
// the pointer before returning it (same pattern as the reader buffers in the
// root package's structured_parser.go).
var tokenSlicePool = sync.Pool{
	New: func() any {
		s := make([]Token, 0, 256)
		return &s
	},
}

// getTokenSlice retrieves a token slice from the pool.
func getTokenSlice() *[]Token {
	p, ok := tokenSlicePool.Get().(*[]Token)
	if !ok {
		s := make([]Token, 0, 256)
		return &s
	}
	return p
}

// putTokenSlice returns a token slice to the pool.
// It drops references to token values (which may hold file contents) so the
// strings can be collected, and discards slices whose capacity exceeds
// MaxPooledYAMLTokenCount to prevent memory bloat.
func putTokenSlice(p *[]Token) {
	if p == nil || *p == nil {
		return
	}
	clear(*p) // drop Value string references
	if cap(*p) > MaxPooledYAMLTokenCount {
		return // too large, let GC reclaim it
	}
	*p = (*p)[:0]
	tokenSlicePool.Put(p)
}

// tokenizeInto tokenizes into the provided slice, reusing its capacity and
// growing it as needed, and returns the (possibly reallocated) slice. On
// error the partially-filled slice is returned alongside the error so pooled
// callers can return it to the pool. Callers that do not pool the token slice
// pass nil.
//
// Pre-allocates for efficiency: a typical line yields ~3 tokens (newline,
// key, value), while single-line flow-style input is better estimated by
// length. Taking the max of both heuristics lands within one growth step of
// the exact count and avoids reallocating the 56-byte-per-token slice
// mid-parse.
func (l *Lexer) tokenizeInto(tokens []Token) ([]Token, error) {
	// Line-count estimate (3 tokens per line + slack), floored by the
	// length-based estimate for inputs with few or no newlines.
	estimatedTokens := bytes.Count(l.input, []byte{'\n'})*3 + 8
	if byLength := len(l.input)/20 + 10; byLength > estimatedTokens {
		estimatedTokens = byLength
	}
	if estimatedTokens > 1024 {
		estimatedTokens = 1024
	}
	if cap(tokens) < estimatedTokens {
		tokens = make([]Token, 0, estimatedTokens)
	}
	tokens = tokens[:0]

	for {
		tok, err := l.NextToken()
		if err != nil {
			return tokens, err
		}
		// SECURITY (SEC-02): fail fast once the token count reaches the hard
		// cap. Every token is ~56 bytes, so tokenizing a size-legal file made
		// of tiny lines would otherwise amplify the input into gigabytes of
		// memory before the parser's MaxVariables check ever runs.
		if len(tokens) >= HardMaxYAMLTokens {
			return tokens, &YAMLError{
				Line:    l.line,
				Column:  l.column,
				Message: fmt.Sprintf("maximum token count exceeded (%d)", HardMaxYAMLTokens),
			}
		}
		tokens = append(tokens, tok)
		if tok.Type == TokenEOF {
			break
		}
	}
	return tokens, nil
}
