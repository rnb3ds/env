package env

import (
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/cybergodev/env/internal"
)

// jsonParser handles parsing of JSON configuration files.
type jsonParser struct {
	config      Config
	validator   Validator
	auditor     AuditLogger
	flatten     internal.JSONFlattenConfig
	factory     *ComponentFactory
	ownsFactory bool
}

// Compile-time check that jsonParser implements EnvParser.
var _ EnvParser = (*jsonParser)(nil)

// newJSONParserWithFactory creates a new jsonParser with a ComponentFactory.
func newJSONParserWithFactory(cfg Config, factory *ComponentFactory) (*jsonParser, error) {
	maxDepth := cfg.JSONMaxDepth
	if maxDepth <= 0 {
		maxDepth = 10 // Default depth
	}

	flattenCfg := internal.JSONFlattenConfig{
		KeyDelimiter:     "_",          // Always use underscore for storage keys
		ArrayIndexFormat: "underscore", // Always use underscore format (KEY_0, KEY_1)
		NullAsEmpty:      cfg.JSONNullAsEmpty,
		NumberAsString:   cfg.JSONNumberAsString,
		BoolAsString:     cfg.JSONBoolAsString,
		MaxDepth:         maxDepth,
	}
	return &jsonParser{
		config:      cfg,
		validator:   factory.Validator(),
		auditor:     factory.Auditor(),
		flatten:     flattenCfg,
		factory:     factory,
		ownsFactory: false, // factory lifecycle managed by caller
	}, nil
}

// Parse reads and parses JSON content from an io.Reader.
func (p *jsonParser) Parse(r io.Reader, filename string) (map[string]string, error) {
	start := time.Now()

	// Wrap with secure reader
	secureRd := internal.NewSecureReader(r, p.config.MaxFileSize, 0)
	data, err := io.ReadAll(secureRd)
	if err != nil {
		if errors.Is(err, internal.ErrFileTooLarge) || errors.Is(err, internal.ErrLineTooLong) {
			_ = p.auditor.LogError(internal.ActionParse, "", "file exceeds size limit")
			return nil, &JSONError{
				Path:    filename,
				Message: "file exceeds size limit",
				Err:     err,
			}
		}
		return nil, err
	}

	// Flatten JSON to environment variables
	result, err := internal.FlattenJSON(data, p.flatten)
	if err != nil {
		_ = p.auditor.LogError(internal.ActionParse, "", "invalid JSON syntax")
		return nil, err
	}

	// Check result size against config
	if len(result) > p.config.MaxVariables {
		_ = p.auditor.LogError(internal.ActionParse, "", "maximum variables exceeded")
		return nil, &ValidationError{
			Field:   "variables",
			Message: fmt.Sprintf("exceeded maximum of %d variables", p.config.MaxVariables),
		}
	}

	// Validate each key and value using fast byte-level validation
	for key, val := range result {
		// Use fast byte-level validation (allows @, -, ., [] etc.)
		if !internal.IsValidJSONKey(key) {
			_ = p.auditor.LogError(internal.ActionParse, key, "key does not match JSON key pattern")
			return nil, &ValidationError{
				Field:   "key",
				Value:   MaskSensitiveInString(key),
				Rule:    "pattern",
				Message: "key does not match required pattern",
			}
		}
		if p.config.ValidateValues {
			if err := p.validator.ValidateValue(val); err != nil {
				_ = p.auditor.LogError(internal.ActionParse, key, err.Error())
				return nil, err
			}
		}
	}

	// Validate required keys
	upperKeys := internal.KeysToUpperPooled(result)
	err = p.validator.ValidateRequired(upperKeys)
	internal.PutKeysToUpperMap(upperKeys)
	if err != nil {
		_ = p.auditor.LogError(internal.ActionValidate, "", err.Error())
		return nil, err
	}

	_ = p.auditor.LogWithDuration(internal.ActionParse, "", "parsed JSON: "+filename, true, time.Since(start))
	return result, nil
}

// Close releases resources held by the parser.
// If the parser owns its ComponentFactory, it will also close the factory.
func (p *jsonParser) Close() error {
	if p.ownsFactory && p.factory != nil {
		return p.factory.Close()
	}
	return nil
}
