package env

import (
	"fmt"
	"io"
	"sync"
)

// ParserFactory creates an EnvParser from a Config and ComponentFactory.
// Custom parser implementations should implement this function signature
// to register their parsers with the library.
type ParserFactory func(cfg Config, factory *ComponentFactory) (EnvParser, error)

// parserRegistry holds registered parser factories.
type parserRegistry struct {
	mu        sync.RWMutex
	factories map[FileFormat]ParserFactory
}

// globalParserRegistry is the global registry for parser factories.
var globalParserRegistry = &parserRegistry{
	factories: make(map[FileFormat]ParserFactory),
}

// RegisterParser registers a custom parser factory for a specific file format.
// Returns an error if a factory is already registered for the format.
// Built-in formats (DotEnv, JSON, YAML) cannot be overridden for security.
//
// Example:
//
//	err := env.RegisterParser(env.FormatEnv, func(cfg env.Config, factory *env.ComponentFactory) (env.EnvParser, error) {
//	    return &MyCustomParser{validator: factory.Validator()}, nil
//	})
func RegisterParser(format FileFormat, factory ParserFactory) error {
	globalParserRegistry.mu.Lock()
	defer globalParserRegistry.mu.Unlock()

	// Prevent overriding built-in parsers for security
	if format == FormatEnv || format == FormatJSON || format == FormatYAML {
		return fmt.Errorf("cannot override built-in parser for format: %s", format.String())
	}

	if _, exists := globalParserRegistry.factories[format]; exists {
		return fmt.Errorf("parser already registered for format: %s", format.String())
	}

	globalParserRegistry.factories[format] = factory
	return nil
}

// createParsers creates all registered parsers for the given configuration.
// Returns a map of parsers keyed by file format.
func createParsers(cfg Config, factory *ComponentFactory) (map[FileFormat]EnvParser, error) {
	globalParserRegistry.mu.RLock()
	defer globalParserRegistry.mu.RUnlock()

	parsers := make(map[FileFormat]EnvParser, len(globalParserRegistry.factories))

	for format, parserFactory := range globalParserRegistry.factories {
		parser, err := parserFactory(cfg, factory)
		if err != nil {
			// Clean up already created parsers to prevent resource leak
			// Use type assertion since EnvParser interface doesn't include Close
			for _, p := range parsers {
				if closer, ok := p.(io.Closer); ok {
					_ = closer.Close()
				}
			}
			return nil, fmt.Errorf("failed to create %s parser: %w", format.String(), err)
		}
		parsers[format] = parser
	}

	return parsers, nil
}

// init registers the built-in parsers.
// Built-in parsers are registered directly to the factories map to bypass
// the security check in RegisterParser that prevents overriding built-in formats.
func init() {
	// Register dot-env parser
	globalParserRegistry.factories[FormatEnv] = func(cfg Config, factory *ComponentFactory) (EnvParser, error) {
		return newParserWithFactory(cfg, factory)
	}

	// Register JSON parser
	globalParserRegistry.factories[FormatJSON] = func(cfg Config, factory *ComponentFactory) (EnvParser, error) {
		return newJSONParserWithFactory(cfg, factory)
	}

	// Register YAML parser
	globalParserRegistry.factories[FormatYAML] = func(cfg Config, factory *ComponentFactory) (EnvParser, error) {
		return newYAMLParserWithFactory(cfg, factory)
	}
}
