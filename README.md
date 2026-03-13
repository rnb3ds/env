# Env - High-Performance Go Environment Variable Library

[![Go Reference](https://pkg.go.dev/badge/github.com/cybergodev/env.svg)](https://pkg.go.dev/github.com/cybergodev/env)
[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)
[![Security Policy](https://img.shields.io/badge/security-policy-blue.svg)](docs/SECURITY.md)
[![Thread Safe](https://img.shields.io/badge/thread%20safe-%E2%9C%93-brightgreen.svg)](docs/CONCURRENCY_SAFETY.md)

**[📖 中文文档](README_zh-CN.md)**

---

## Overview

**Env** is a production-ready, zero-dependency, thread-safe Go library for environment variable management, focusing on **security**, **concurrency**, and **developer experience**. It supports `.env`, `.json`, and `.yaml` formats with automatic type conversion and secure memory handling.

---

## Core Features

| Feature | Description |
|:--------|:------------|
| 🚀 **One-Line Setup** | `env.Load(".env")` loads and applies to `os.Environ` |
| 🔒 **Type Safety** | `GetString`, `GetInt`, `GetBool`, `GetDuration`, `GetSlice[T]` |
| 📁 **Multi-Format Support** | Auto-detect `.env`, `.json`, `.yaml` files |
| ⚡ **Thread Safety** | Sharded storage (8 shards) + RWMutex, optimized for high concurrency |
| 🛡️ **Memory Safety** | `SecureValue` auto-zeroes sensitive data, supports memory pooling |
| 🔄 **Variable Expansion** | Full support for `${VAR}` syntax with default values |
| 📝 **Audit Logging** | Built-in JSON/Log/Channel handlers for compliance requirements |
| 🧪 **Testing Support** | Isolated loaders for test isolation |
| 📦 **Zero Dependencies** | Standard library only |

---

## Installation

```bash
go get github.com/cybergodev/env
```

**Requirements:** Go 1.24+

---

## Quick Start

### Step 1: Create a `.env` file

```env
# Application configuration
APP_NAME=myapp
APP_PORT=8080
DEBUG=true

# Database configuration
DB_HOST=localhost
DB_PORT=5432
DB_PASSWORD=secret123

# Timeout settings
TIMEOUT=30s
```

### Step 2: Use in Go code

```go
package main

import (
    "fmt"
    "log"
    "time"

    "github.com/cybergodev/env"
)

func main() {
    // One-line load and apply
    if err := env.Load(".env"); err != nil {
        log.Printf("Warning: %v", err) // Missing file doesn't cause error by default
    }

    // Type-safe access (with defaults)
    port    := env.GetInt("APP_PORT", 8080)
    debug   := env.GetBool("DEBUG", false)
    timeout := env.GetDuration("TIMEOUT", 30*time.Second)
    hosts   := env.GetSlice[string]("HOSTS", []string{"localhost"})

    fmt.Printf("Server: %s:%d\n", env.GetString("APP_NAME", "unknown"), port)
    fmt.Printf("Debug: %v, Timeout: %v\n", debug, timeout)
    fmt.Printf("Hosts: %v\n", hosts)
}
```

---

## Multi-Format Support

### .env Files

```env
# Comments start with #
DATABASE_URL=postgres://localhost:5432/db
PORT=8080
DEBUG=true

# Quotes are optional
MESSAGE="Hello World"
SINGLE='Single quotes work too'
```

### JSON (Auto-flattened)

```json
{
    "database": { "host": "localhost", "port": 5432 },
    "ports": [8080, 8081]
}
```

**Access:**
```go
env.GetString("database.host")    // "localhost" (dot notation)
env.GetInt("database.port")       // 5432
env.GetSlice[int]("ports")        // [8080, 8081]
// Also works: DATABASE_HOST, DATABASE_PORT
```

### YAML (Auto-flattened)

```yaml
database:
  host: localhost
  port: 5432
ports: [8080, 8081]
```

**Access:** Same as JSON - use dot notation or uppercase underscore format.

---

## Usage Guide

### Basic Operations

```go
// Load multiple files (later files override earlier ones)
env.Load(".env", "config.json", ".env.local")

// Check existence
value, exists := env.Lookup("KEY")
if !exists {
    // Handle missing key
}

// CRUD operations
env.Set("KEY", "value")    // Set value
env.Delete("KEY")          // Delete key
keys := env.Keys()         // Get all keys
all := env.All()           // Get all variables
count := env.Len()         // Variable count
```

### Type Access

```go
// String (with default)
name := env.GetString("APP_NAME", "default-app")

// Integer (returns int64)
port := env.GetInt("PORT", 8080)

// Boolean
debug := env.GetBool("DEBUG", false)

// Duration
timeout := env.GetDuration("TIMEOUT", 30*time.Second)

// Generic slice: string, int, int64, uint, uint64, bool, float64, time.Duration
hosts := env.GetSlice[string]("HOSTS", []string{"localhost"})
ports := env.GetSlice[int]("PORTS", []int{8080})
delays := env.GetSlice[time.Duration]("DELAYS", nil)
```

**Supported Boolean Values:**

- **True**: `true`, `1`, `yes`, `on`, `enabled`
- **False**: `false`, `0`, `no`, `off`, `disabled`

### Struct Mapping

```go
type Config struct {
    Port    int           `env:"PORT" envDefault:"8080"`
    Debug   bool          `env:"DEBUG" envDefault:"false"`
    Timeout time.Duration `env:"TIMEOUT"`
    Origins []string      `env:"CORS_ORIGINS"`
}

var cfg Config
if err := env.Load(".env"); err != nil {
    log.Fatal(err)
}
if err := env.ParseInto(&cfg); err != nil {
    log.Fatal(err)
}
```

### Serialization/Deserialization

> Supported formats: `FormatEnv`, `FormatJSON`, `FormatYAML`, `FormatAuto`

```go
// Map to format string
data := map[string]string{"PORT": "8080", "DEBUG": "true"}

envString, _  := env.Marshal(data)                      // .env (default)
jsonString, _ := env.Marshal(data, env.FormatJSON)      // JSON
yamlString, _ := env.Marshal(data, env.FormatYAML)      // YAML

// Parse string to Map
m, _ := env.UnmarshalMap("PORT=8080\nDEBUG=true")           // .env format
m, _ := env.UnmarshalMap(`{"port": 8080}`, env.FormatJSON)  // JSON
m, _ := env.UnmarshalMap(yamlString, env.FormatAuto)        // Auto-detect

// Struct <-> Map conversion
m, _ := env.MarshalStruct(&config)          // Struct to map
env.UnmarshalInto(m, &config)               // Map to struct

// String directly to struct
env.UnmarshalStruct("PORT=8080\nDEBUG=true", &config, env.FormatEnv)
```

### Variable Expansion

`.env` files fully support `${VAR}` syntax:

```env
HOST=localhost
PORT=8080

# Variable reference
URL=${HOST}:${PORT}                    # → "localhost:8080"

# Default value if unset or empty
TIMEOUT=${TIMEOUT:-30s}

# Default value only if unset (preserves empty string)
NAME=${NAME-default}

# Combined expansion
FULL_URL=https://${HOST}:${PORT:-443}
```

### Loader API (Fine-grained Control)

```go
cfg := env.ProductionConfig()
cfg.Filenames = []string{"/etc/app/.env"}

loader, err := env.New(cfg)
if err != nil {
    log.Fatal(err)
}
defer loader.Close()

// Load additional files
loader.LoadFiles("override.env")

// Apply to os.Environ
loader.Apply()

// Access values
port := loader.GetInt("PORT", 8080)
```

### Secure Value Handling

Use `SecureValue` for sensitive data like passwords, API keys, and tokens:

```go
// Get SecureValue
sv := env.GetSecure("API_KEY")
if sv != nil {
    // Safe logging
    fmt.Println(sv.Masked())       // [SECURE:32 bytes]

    // Access actual value
    value := sv.String()

    // Get bytes (caller must clean up)
    data := sv.Bytes()
    defer env.ClearBytes(data)     // Manual zeroing

    // Cleanup
    sv.Close()                     // or sv.Release() to return to pool
}

// Create SecureValue directly
secret := env.NewSecureValue("super_secret")
defer secret.Release()
```

### Audit Logging

```go
cfg := env.ProductionConfig()
cfg.AuditEnabled = true
cfg.AuditHandler = env.NewJSONAuditHandler(os.Stdout)

loader, _ := env.New(cfg)
// Output: {"action":"set","key":"API_KEY","success":true,"timestamp":"..."}
```

**Built-in Handlers:**

```go
env.NewJSONAuditHandler(w)      // JSON format → io.Writer
env.NewLogAuditHandler(logger)  // Standard log.Logger
env.NewChannelAuditHandler(ch)  // Channel (external processing)
env.NewNopAuditHandler()        // No-op (discard)
```

### Testing Support

```go
func TestConfig(t *testing.T) {
    // Create isolated loader (doesn't affect global state)
    cfg := env.TestingConfig()
    cfg.Filenames = []string{".env.test"}

    loader, err := env.New(cfg)
    if err != nil {
        t.Fatal(err)
    }
    defer loader.Close()

    port := loader.GetInt("PORT", 8080)
    // Test your code...
}

// Reset default loader between tests
func TestMain(m *testing.M) {
    env.ResetDefaultLoader()
    os.Exit(m.Run())
}
```

---

## API Reference

### Package Functions

| Function | Description |
|:---------|:------------|
| `Load(files...)` | Load files and apply to `os.Environ` |
| `GetString(key, def...)` | Get string value |
| `GetInt(key, def...)` | Get `int64` value |
| `GetBool(key, def...)` | Get boolean value |
| `GetDuration(key, def...)` | Get duration value |
| `GetSlice[T](key, def...)` | Get generic slice |
| `GetSliceFrom[T](loader, key, def...)` | Get slice from specific loader |
| `Lookup(key)` | Get value + existence check |
| `Set(key, value)` | Set value (returns error) |
| `Delete(key)` | Delete key (returns error) |
| `Keys()` | Get all keys |
| `All()` | Get all variables as map |
| `Len()` | Get variable count |
| `GetSecure(key)` | Get `SecureValue` for sensitive data |
| `Validate()` | Validate required keys |
| `ParseInto(&struct)` | Populate struct from env vars |
| `Marshal(data, format?)` | Convert map/struct to format string |
| `UnmarshalMap(string, format?)` | Parse format string to map |
| `UnmarshalStruct(string, &struct, format?)` | Parse string to struct |
| `UnmarshalInto(map, &struct)` | Populate struct from map |
| `MarshalStruct(struct)` | Convert struct to map |
| `New(cfg)` | Create new loader with config |
| `NewSecureValue(string)` | Create SecureValue from string |
| `ResetDefaultLoader()` | Reset singleton (for testing) |
| `ClearBytes([]byte)` | Securely zero byte slice |

### Utility Functions

| Function | Description |
|:---------|:------------|
| `IsSensitiveKey(key)` | Check if key contains sensitive pattern |
| `MaskValue(key, value)` | Mask value based on key sensitivity |
| `MaskKey(key)` | Mask key name for logging |
| `MaskSensitiveInString(s)` | Mask sensitive content in string |
| `SanitizeForLog(s)` | Remove sensitive info for logging |
| `DetectFormat(filename)` | Detect file format by extension |

### Loader Methods

| Method | Description |
|:-------|:------------|
| `LoadFiles(files...)` | Load files into loader |
| `Apply()` | Apply to `os.Environ` |
| `Validate()` | Validate required keys |
| `Close()` | Close and cleanup resources |
| `IsApplied()` | Check if applied to os.Environ |
| `IsClosed()` | Check if closed |
| `LoadTime()` | Get last load time |
| `Config()` | Get loader configuration |

### SecureValue Methods

| Method | Description |
|:-------|:------------|
| `String()` | Get string value |
| `Bytes()` | Get byte slice (copy - caller must clean up) |
| `Length()` | Get value length |
| `Masked()` | Get masked representation for logging |
| `Close()` | Zero memory, don't return to pool |
| `Release()` | Zero memory and return to pool |
| `IsClosed()` | Check if closed |

---

## Configuration Options

### Preset Configurations

```go
env.DefaultConfig()     // Safe defaults
env.DevelopmentConfig() // Relaxed limits + allow override
env.TestingConfig()     // Tight config + isolated testing
env.ProductionConfig()  // Strict security + audit
```

### Configuration Comparison

| Setting | Default | Development | Testing | Production |
|---------|---------|-------------|---------|------------|
| `FailOnMissingFile` | false | false | false | **true** |
| `OverwriteExisting` | false | **true** | **true** | false |
| `ValidateValues` | **true** | **true** | **true** | **true** |
| `AuditEnabled` | false | false | false | **true** |
| `MaxFileSize` | 2 MB | 10 MB | 64 KB | 64 KB |
| `MaxVariables` | 500 | 500 | 50 | 50 |

### Full Configuration Options

```go
cfg := env.DefaultConfig()

// === File Handling ===
cfg.Filenames         = []string{".env"}
cfg.FailOnMissingFile = false
cfg.OverwriteExisting = true
cfg.AutoApply         = true

// === Validation ===
cfg.RequiredKeys   = []string{"DB_URL"}
cfg.AllowedKeys    = []string{"PORT", "DEBUG"}  // Empty = allow all
cfg.ForbiddenKeys  = []string{"PATH"}           // Block dangerous keys

// === Security Limits ===
cfg.MaxFileSize    = 2 << 20   // 2 MB
cfg.MaxVariables   = 500
cfg.ValidateValues = true

// === Parsing Options ===
cfg.MaxLineLength     = 1024
cfg.MaxKeyLength      = 64
cfg.MaxValueLength    = 4096
cfg.MaxExpansionDepth = 5
cfg.JSONNullAsEmpty   = true
cfg.YAMLNullAsEmpty   = true

// === Security Settings ===
cfg.KeyPattern        = nil  // nil = fast validation (recommended)
cfg.AllowExportPrefix = false

// === Advanced Options ===
cfg.Prefix     = "APP_"      // Only load keys with prefix
cfg.FileSystem = nil         // nil = OS filesystem

// === Audit Logging ===
cfg.AuditEnabled = true
cfg.AuditHandler = env.NewJSONAuditHandler(os.Stdout)
```

### Default Limits

| Setting | Default | Hard Limit |
|---------|---------|------------|
| MaxFileSize | 2 MB | 100 MB |
| MaxLineLength | 1,024 chars | 64 KB |
| MaxKeyLength | 64 chars | 1,024 chars |
| MaxValueLength | 4,096 chars | 1 MB |
| MaxVariables | 500 | 10,000 |
| MaxExpansionDepth | 5 | 20 |

---

## Security Features

| Feature | Description |
|:--------|:------------|
| **Key/Value Validation** | Block invalid formats and dangerous patterns |
| **Forbidden Keys** | Prevent overwriting `PATH`, `LD_PRELOAD`, `DYLD_*`, etc. |
| **Size Limits** | File size, line length, variable count limits |
| **Expansion Depth** | Prevent exponential expansion attacks |
| **Sensitive Data Masking** | Auto-detect and mask passwords, tokens, keys |
| **Secure Memory** | `SecureValue` zeroes memory on GC/cleanup |
| **Path Traversal Protection** | Block `..`, absolute paths, UNC paths |

### Sensitive Key Patterns

Automatically detected patterns (case-insensitive):

```
*password*, *secret*, *key*, *token*, *credential*,
*api_key*, *private*, *auth*, *session*, *access*
```

```go
// Sensitive key detection
env.IsSensitiveKey("API_SECRET")  // true
env.IsSensitiveKey("HOST")        // false

// Value masking
env.MaskValue("API_KEY", "secret123")  // "***"

// Key masking for logging
env.MaskKey("DB_PASSWORD")  // "DB_***"

// Safe log handling
safe := env.SanitizeForLog(userInput)
```

---

## Performance

| Metric | Value |
|:-------|:------|
| **Sharded Concurrency** | 8 shards for parallel access |
| **Memory Pooling** | Reusable SecureValue, Builder, Scanner pools |
| **Zero Allocations** | Fast path for simple key lookups |
| **Benchmarks** | Run `go test -bench=. -benchmem` |

---

## Examples

See the [examples](examples) directory for complete example code:

| Example | Description |
|:--------|:------------|
| [01_quickstart.go](examples/01_quickstart.go) | Basic usage |
| [02_loader_config.go](examples/02_loader_config.go) | Configuration options |
| [03_type_access.go](examples/03_type_access.go) | Type conversion |
| [04_struct_mapping.go](examples/04_struct_mapping.go) | Struct population |
| [05_secure_values.go](examples/05_secure_values.go) | Secure handling |
| [06_audit_logging.go](examples/06_audit_logging.go) | Audit logging |
| [07_marshal_unmarshal.go](examples/07_marshal_unmarshal.go) | Serialization |

---

## License

MIT License - See [LICENSE](LICENSE) file for details.

---

If this project helps you, please give it a Star! ⭐
