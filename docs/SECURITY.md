# Security Policy

[![Security](https://img.shields.io/badge/Security-Hardened-green.svg)]()
[![Audit](https://img.shields.io/badge/Audit-Enabled-blue.svg)]()

## Reporting a Vulnerability

**Do NOT report security vulnerabilities through public GitHub issues.**

Instead, please report them via:

- **Email**: cybergodev@gmail.com
- **GitHub Security Advisory**: [Report a vulnerability](https://github.com/cybergodev/env/security/advisories/new)

---

## Security Features

This library is designed for **high-security environments** with the following built-in protections:

### Input Validation

| Protection | Description |
|------------|-------------|
| **Key Pattern Validation** | Only allows `^[A-Za-z][A-Za-z0-9_]*$` by default |
| **Value Content Validation** | Blocks null bytes and control characters |
| **Key Length Limits** | Default: 64 chars, Hard limit: 1024 chars |
| **Value Length Limits** | Default: 4096 chars, Hard limit: 1MB |

### File Security

| Protection | Description |
|------------|-------------|
| **File Size Limits** | Default: 2MB, Hard limit: 100MB |
| **Line Length Limits** | Default: 1024 chars, Hard limit: 64KB |
| **Variable Count Limits** | Default: 500, Hard limit: 10,000 |
| **Format Detection** | Automatic detection by extension |

### Forbidden Keys

The following system-critical keys are blocked by default:

```
PATH, LD_PRELOAD, LD_LIBRARY_PATH, LD_DEBUG, LD_AUDIT
DYLD_INSERT_LIBRARIES, DYLD_LIBRARY_PATH
IFS, SHELL, ENV, BASH_ENV
PERL5OPT, PYTHONPATH, RUBYLIB, NODE_PATH
```

### Memory Security

| Feature | Description |
|---------|-------------|
| **SecureValue** | Automatic memory zeroing on GC/cleanup |
| **Sensitive Data Masking** | Logs never expose passwords, tokens, keys |
| **Memory Pool Clearing** | Pooled objects are cleared before reuse |
| **Finalizer Cleanup** | GC triggers secure memory erasure |

### Variable Expansion Security

| Protection | Description |
|------------|-------------|
| **Depth Limiting** | Default: 5 levels, Hard limit: 20 levels |
| **Cycle Detection** | Prevents infinite expansion loops |
| **Key Validation** | Only valid keys are expanded |

### Concurrency Safety

- All operations are **thread-safe**
- Sharded storage design (8 shards) for reduced lock contention
- Atomic operations for fast-path checks
- 100+ race detection test runs pass

---

## Configuration

### Production-Ready Configuration

```go
cfg := env.ProductionConfig()
cfg.Filenames = []string{"/etc/app/.env"}
cfg.AuditEnabled = true
cfg.AuditHandler = env.NewJSONAuditHandler(auditFile)
cfg.RequiredKeys = []string{"DATABASE_URL", "API_KEY"}
```

### Security Settings

```go
cfg := env.DefaultConfig()

// Key validation
cfg.Security.KeyPattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]+$`)
cfg.ValidateValues = true

// Key restrictions
cfg.AllowedKeys = []string{"APP_PORT", "APP_HOST"}
cfg.ForbiddenKeys = []string{"PATH", "HOME"}

// Limits
cfg.MaxFileSize = 64 * 1024      // 64KB
cfg.MaxVariables = 50
cfg.Parsing.MaxKeyLength = 32
cfg.Parsing.MaxValueLength = 1024
```

---

## Secure Value Handling

### Basic Usage

```go
sv := env.GetSecure("API_KEY")
if sv != nil {
    // Safe for logging
    log.Println(sv.Masked()) // [SECURE:32 bytes]

    // Access the value
    value := sv.String()

    // Or get bytes (caller must clear)
    data := sv.Bytes()
    defer env.ClearBytes(data)

    // Cleanup when done
    sv.Close()
}
```

### Automatic Cleanup

```go
sv := env.NewSecureValue("secret")
// When sv is garbage collected, memory is automatically zeroed
// Or explicitly:
sv.Close() // Zero memory immediately
```

---

## Audit Logging

### Enable Auditing

```go
cfg := env.ProductionConfig()
cfg.AuditEnabled = true
cfg.AuditHandler = env.NewJSONAuditHandler(os.Stdout)
```

### Audit Output Format

```json
{
    "timestamp": "2026-03-11T10:30:00Z",
    "action": "set",
    "key": "AP***",
    "message": "loaded",
    "success": true
}
```

### Built-in Handlers

| Handler | Use Case |
|---------|----------|
| `JSONAuditHandler` | Structured logs, SIEM integration |
| `LogAuditHandler` | Standard Go log package |
| `ChannelAuditHandler` | Custom async processing |
| `NopAuditHandler` | Disabled auditing |

---

## Sensitive Data Detection

Keys containing these patterns (case-insensitive) are automatically masked:

```
PASSWORD, SECRET, TOKEN, API_KEY, APIKEY
PRIVATE, CREDENTIAL, AUTH, ACCESS_KEY
SECRET_KEY, PRIVATE_KEY, PASSPHRASE
SESSION, COOKIE
```

### Example

```go
// Key: DATABASE_PASSWORD
// Logged as: DA*** = [MASKED:16 chars]
```

---

## Security Best Practices

### Do

- Use `ProductionConfig()` in production environments
- Enable audit logging for compliance
- Use `SecureValue` for sensitive data
- Set `RequiredKeys` to fail fast on missing configuration
- Use `FailOnMissingFile: true` in production

### Don't

- Never log raw environment values
- Never store `SecureValue` pointers longer than needed
- Never disable `ValidateValues` in production
- Never allow untrusted input as file paths

---

## Security Checklist

| Item | Status |
|------|--------|
| Input validation on all keys and values | Implemented |
| File size and line length limits | Implemented |
| Forbidden keys blocking | Implemented |
| Memory zeroing for sensitive data | Implemented |
| Audit logging support | Implemented |
| Thread-safe operations | Implemented |
| Race condition prevention | Verified (100+ test runs) |
| Sensitive data masking in logs/errors | Implemented |
| Variable expansion cycle detection | Implemented |
| Hard limits to prevent DoS | Implemented |

---

## License

This project is licensed under the MIT License - see [LICENSE](../LICENSE) for details.
