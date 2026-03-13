package env

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/cybergodev/env/internal"
)

// ============================================================================
// parser
// ============================================================================

// scannerBufferPool provides a pool of reusable byte slices for bufio.Scanner.
// This reduces allocations when parsing multiple files or strings.
// Each buffer is 64KB which is sufficient for most env files while being
// small enough to avoid excessive memory consumption. This size was chosen
// based on typical .env file sizes (rarely exceeding 64KB in practice).
var scannerBufferPool = sync.Pool{
	New: func() interface{} {
		buf := make([]byte, 64*1024)
		return &buf
	},
}

// parserPool provides a pool of reusable parsers to reduce allocations.
var parserPool = sync.Pool{
	New: func() interface{} {
		return &parser{}
	},
}

// getScannerBuffer retrieves a byte slice from the pool for use with bufio.Scanner.
// Returns a fallback buffer if the pool returns an unexpected type.
func getScannerBuffer() *[]byte {
	buf, ok := scannerBufferPool.Get().(*[]byte)
	if !ok {
		// Fallback: create new buffer if pool returns unexpected type
		b := make([]byte, 64*1024)
		return &b
	}
	return buf
}

// putScannerBuffer returns a byte slice to the pool.
func putScannerBuffer(buf *[]byte) {
	if buf == nil || cap(*buf) > internal.MaxPooledScannerBufferSize {
		// Don't pool very large buffers
		return
	}
	scannerBufferPool.Put(buf)
}

// parser handles secure parsing of environment files.
type parser struct {
	config      Config
	validator   Validator
	auditor     AuditLogger
	lineParser  *internal.LineParser
	factory     *ComponentFactory
	ownsFactory bool
}

// Compile-time check that parser implements EnvParser.
var _ EnvParser = (*parser)(nil)

// newParserWithFactory creates a new parser with a ComponentFactory.
// This is more efficient as it reuses the same Validator, Auditor, and Expander
// instances instead of creating new ones.
func newParserWithFactory(cfg Config, factory *ComponentFactory) (*parser, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	if factory == nil {
		return nil, fmt.Errorf("factory cannot be nil")
	}

	validator := factory.Validator()
	auditor := factory.Auditor()

	lineParserCfg := internal.LineParserConfig{
		AllowExportPrefix: cfg.AllowExportPrefix,
		AllowYamlSyntax:   cfg.AllowYamlSyntax,
		OverwriteExisting: cfg.OverwriteExisting,
		MaxVariables:      cfg.MaxVariables,
		ExpandVariables:   cfg.ExpandVariables,
	}

	return &parser{
		config:     cfg,
		validator:  validator,
		auditor:    auditor,
		lineParser: internal.NewLineParser(lineParserCfg, factory.internalValidator(), factory.internalAuditor(), factory.internalExpander()),
	}, nil
}

// Parse reads and parses environment variables from an io.Reader.
func (p *parser) Parse(r io.Reader, filename string) (map[string]string, error) {
	startTime := time.Now()

	// Wrap with secure reader
	secureRd := internal.NewSecureReader(r, p.config.MaxFileSize, p.config.MaxLineLength)
	scanner := bufio.NewScanner(secureRd)

	// Use pooled buffer for scanner to reduce allocations
	scannerBuf := getScannerBuffer()
	defer putScannerBuffer(scannerBuf)
	scanner.Buffer(*scannerBuf, cap(*scannerBuf))

	// Pre-allocate result map with optimal capacity
	// Estimate based on typical .env file density: ~1 variable per 2-3 lines
	// Use min of MaxVariables and a reasonable estimate
	initialCap := min(64, p.config.MaxVariables)
	result := make(map[string]string, initialCap)

	lineNum := 0
	var parseErr error

	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()

		// Fast path: skip empty lines and comments using byte-level operations
		start := 0
		end := len(line)
		for start < end && (line[start] == ' ' || line[start] == '\t') {
			start++
		}
		for end > start && (line[end-1] == ' ' || line[end-1] == '\t') {
			end--
		}
		if start == end {
			continue // Empty line
		}
		if line[start] == '#' {
			continue // Comment line
		}

		// Parse the line using byte-slice version to minimize allocations
		key, value, err := p.lineParser.ParseLineBytes(line)
		if err != nil {
			parseErr = newParseError(filename, lineNum, key, err)
			break
		}

		if key == "" {
			continue
		}

		// Check max variables
		if len(result) >= p.config.MaxVariables {
			_ = p.auditor.LogError(internal.ActionParse, "", "maximum variables exceeded")
			parseErr = &ValidationError{
				Field:   "variables",
				Message: fmt.Sprintf("exceeded maximum of %d variables", p.config.MaxVariables),
			}
			break
		}

		// Handle overwrites
		if _, exists := result[key]; exists && !p.config.OverwriteExisting {
			_ = p.auditor.Log(internal.ActionParse, key, "duplicate key skipped", false)
			continue
		}

		result[key] = value
	}

	if err := scanner.Err(); err != nil {
		if errors.Is(err, ErrFileTooLarge) || errors.Is(err, ErrLineTooLong) {
			_ = p.auditor.LogError(internal.ActionParse, "", err.Error())
			return nil, err
		}
		return nil, newParseError(filename, lineNum, "", err)
	}

	if parseErr != nil {
		return nil, parseErr
	}

	// Validate required keys (using pooled map to reduce allocations)
	upperKeys := internal.KeysToUpperPooled(result)
	err := p.validator.ValidateRequired(upperKeys)
	internal.PutKeysToUpperMap(upperKeys)
	if err != nil {
		_ = p.auditor.LogError(internal.ActionValidate, "", err.Error())
		return nil, err
	}

	// Expand variables if enabled
	if p.config.ExpandVariables {
		expanded, err := p.lineParser.ExpandAll(result)
		if err != nil {
			return nil, err
		}
		result = expanded
	}

	_ = p.auditor.LogWithDuration(internal.ActionParse, "", "parsed: "+filename, true, time.Since(startTime))
	return result, nil
}

// reset resets the parser with new configuration and factory.
// This allows reusing a pooled parser instance.
// The ownsFactory parameter determines whether the parser owns the factory
// lifecycle and should close it when Close() is called.
func (p *parser) reset(cfg Config, factory *ComponentFactory, ownsFactory bool) error {
	if err := cfg.Validate(); err != nil {
		return err
	}

	if factory == nil {
		return fmt.Errorf("factory cannot be nil")
	}

	validator := factory.Validator()
	auditor := factory.Auditor()

	lineParserCfg := internal.LineParserConfig{
		AllowExportPrefix: cfg.AllowExportPrefix,
		AllowYamlSyntax:   cfg.AllowYamlSyntax,
		OverwriteExisting: cfg.OverwriteExisting,
		MaxVariables:      cfg.MaxVariables,
		ExpandVariables:   cfg.ExpandVariables,
	}

	p.config = cfg
	p.validator = validator
	p.auditor = auditor
	p.factory = factory
	p.ownsFactory = ownsFactory
	p.lineParser = internal.NewLineParser(lineParserCfg, factory.internalValidator(), factory.internalAuditor(), factory.internalExpander())

	return nil
}

// Close releases resources held by the parser.
// If the parser owns its ComponentFactory, it will also close the factory.
func (p *parser) Close() error {
	if p.ownsFactory && p.factory != nil {
		return p.factory.Close()
	}
	return nil
}

// clear resets all parser fields to their zero values.
// This ensures the parser is in a clean state before being returned to the pool.
func (p *parser) clear() {
	p.config = Config{}
	p.validator = nil
	p.auditor = nil
	p.lineParser = nil
	p.factory = nil
	p.ownsFactory = false
}

// parseString parses environment variables from a string.
// This is an internal function used by Unmarshal.
func parseString(s string) (map[string]string, error) {
	cfg := DefaultConfig()

	// Build factory - will be owned and closed by the parser
	factory := cfg.buildComponentFactory()

	// GetString parser from pool
	p, ok := parserPool.Get().(*parser)
	if !ok {
		p = &parser{}
	}

	// Reset parser with new config - parser owns the factory lifecycle
	if err := p.reset(cfg, factory, true); err != nil {
		// Reset failed - clean up factory and parser separately.
		// The parser state is unchanged since reset validates before modifying.
		_ = factory.Close()
		p.clear()
		parserPool.Put(p)
		return nil, err
	}

	// Use defer to ensure cleanup even if Parse panics
	defer func() {
		// Close the parser which will close the factory since ownsFactory=true
		if err := p.Close(); err != nil {
			// Log cleanup error but don't panic - production safety is paramount
			_ = p.auditor.LogError(internal.ActionError, "", "parser cleanup failed: "+err.Error())
		}
		p.clear()
		parserPool.Put(p)
	}()

	// Parse and return result
	result, err := p.Parse(strings.NewReader(s), "")
	return result, err
}
