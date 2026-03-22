// Package internal provides path validation utilities for secure file access.
package internal

import (
	"path/filepath"
	"strings"
)

// PathValidatorConfig holds configuration for path validation.
type PathValidatorConfig struct {
	// MaskKey is an optional function to mask sensitive key names in errors.
	// If nil, a default masking function is used.
	MaskKey func(string) string
}

// PathValidator validates file paths for security.
// It checks for path traversal attempts, absolute paths, and other dangerous patterns.
type PathValidator struct {
	maskKey func(string) string
}

// NewPathValidator creates a new PathValidator with the specified configuration.
func NewPathValidator(cfg PathValidatorConfig) *PathValidator {
	maskKey := cfg.MaskKey
	if maskKey == nil {
		maskKey = DefaultMaskKey
	}
	return &PathValidator{maskKey: maskKey}
}

// Validate validates a file path for security.
// It checks for path traversal attempts and other potentially dangerous patterns.
//
// Security checks performed:
//   - Empty filename
//   - Null bytes (could bypass extension checks)
//   - URL encoding (could bypass path checks)
//   - UNC paths (Windows network paths)
//   - Unix absolute paths
//   - Windows drive letters
//   - Path traversal (..)
//   - Windows reserved device names
//   - Symlink escape attacks
func (v *PathValidator) Validate(filename string) error {
	if filename == "" {
		return &SecurityError{
			Action: "file_access",
			Reason: "empty filename",
		}
	}

	// SECURITY: Check for null bytes first (could be used to bypass extension checks)
	if strings.ContainsRune(filename, '\x00') {
		return &SecurityError{
			Action: "file_access",
			Reason: "null byte in path",
		}
	}

	// SECURITY: Check for URL encoding which could be used to bypass path checks
	// Examples: %2e%2e for .., %5c for \, etc.
	if strings.Contains(filename, "%") {
		return &SecurityError{
			Action: "file_access",
			Reason: "URL encoded path not allowed",
		}
	}

	// SECURITY: Check for UNC paths (Windows network paths)
	// These can be used to access files on network shares
	// Also block \\?\ prefix which can bypass path length limits
	if len(filename) >= 2 && filename[0] == '\\' && filename[1] == '\\' {
		return &SecurityError{
			Action: "file_access",
			Reason: "UNC path not allowed",
		}
	}

	// SECURITY: Check for forward-slash UNC paths (\\ translated to //)
	if len(filename) >= 2 && filename[0] == '/' && filename[1] == '/' {
		return &SecurityError{
			Action: "file_access",
			Reason: "network path not allowed",
		}
	}

	// SECURITY: Check for Unix-style absolute paths starting with /
	// Only allow relative paths for safety
	if len(filename) > 0 && filename[0] == '/' {
		return &SecurityError{
			Action: "file_access",
			Reason: "absolute path not allowed",
		}
	}

	// SECURITY: Check for Windows drive letters (C:, D:, etc.)
	// This prevents access to system files via absolute paths
	if len(filename) >= 2 && filename[1] == ':' {
		c := filename[0]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
			return &SecurityError{
				Action: "file_access",
				Reason: "absolute path with drive letter not allowed",
			}
		}
	}

	// Check for path traversal attempts using filepath.Clean
	// This is more precise than just checking for ".." in the raw string
	// filepath.Clean normalizes the path by resolving ".." and removing redundant separators
	cleanPath := filepath.Clean(filename)
	if strings.Contains(cleanPath, "..") {
		return &SecurityError{
			Action: "file_access",
			Reason: "path traversal detected",
			Key:    v.maskKey(filename),
		}
	}

	// SECURITY: On Windows, check for reserved device names
	// These names are reserved in any directory and cannot be used as filenames
	// See: https://learn.microsoft.com/en-us/windows/win32/fileio/naming-a-file
	if err := v.checkReservedDeviceNames(filename); err != nil {
		return err
	}

	// SECURITY: Resolve and validate symlinks to prevent symlink escape attacks
	// A symlink within the allowed directory could point outside the intended scope
	return v.validateSymlinks(cleanPath)
}

// checkReservedDeviceNames checks for Windows reserved device names.
func (v *PathValidator) checkReservedDeviceNames(filename string) error {
	if len(filename) < 3 {
		return nil
	}

	upper := strings.ToUpper(filename)

	// Check for CON, PRN, AUX, NUL
	reserved := []string{"CON", "PRN", "AUX", "NUL"}
	for _, r := range reserved {
		if upper == r || (len(upper) > 3 && upper[:3] == r && (upper[3] == '.' || upper[3] == ':')) {
			return &SecurityError{
				Action: "file_access",
				Reason: "reserved device name",
			}
		}
	}

	// Check COM and LPT ports (COM1-COM9, LPT1-LPT9)
	// These are 4-character names like "COM1", "LPT9", etc.
	if len(upper) >= 4 {
		prefix := upper[:3]
		if (prefix == "COM" || prefix == "LPT") && upper[3] >= '1' && upper[3] <= '9' {
			// Match: exactly 4 chars (e.g., "COM1") or followed by separator
			if len(upper) == 4 || (len(upper) > 4 && (upper[4] == '.' || upper[4] == ':')) {
				return &SecurityError{
					Action: "file_access",
					Reason: "reserved device name",
				}
			}
		}
	}

	// Check for pseudo-device names: CONIN$, CONOUT$, CLOCK$
	pseudoDevices := []string{"CONIN$", "CONOUT$", "CLOCK$"}
	for _, pd := range pseudoDevices {
		if upper == pd || strings.HasPrefix(upper, pd+".") || strings.HasPrefix(upper, pd+":") {
			return &SecurityError{
				Action: "file_access",
				Reason: "reserved pseudo-device name",
			}
		}
	}

	return nil
}

// validateSymlinks resolves and validates symlinks to prevent escape attacks.
func (v *PathValidator) validateSymlinks(cleanPath string) error {
	resolved, err := filepath.EvalSymlinks(cleanPath)
	if err != nil {
		// If the path doesn't exist yet (e.g., for write operations),
		// validate the parent directory instead
		dir := filepath.Dir(cleanPath)
		if dir != "." && dir != cleanPath {
			resolvedDir, dirErr := filepath.EvalSymlinks(dir)
			if dirErr != nil {
				// Path or parent doesn't exist - allow to proceed
				// The file operation will fail later if path is invalid
				return nil
			}
			// Validate resolved parent directory
			return v.validateResolvedPath(resolvedDir)
		}
		return nil
	}

	// Validate the resolved path is still within allowed bounds
	return v.validateResolvedPath(resolved)
}

// validateResolvedPath checks that a resolved (symlink-expanded) path is safe.
// It ensures the path is relative and doesn't escape to absolute locations.
func (v *PathValidator) validateResolvedPath(resolved string) error {
	// Check for absolute paths after symlink resolution
	if filepath.IsAbs(resolved) {
		return &SecurityError{
			Action: "file_access",
			Reason: "symlink resolves to absolute path",
		}
	}

	// Double-check for path traversal in resolved path
	cleanResolved := filepath.Clean(resolved)
	if strings.Contains(cleanResolved, "..") {
		return &SecurityError{
			Action: "file_access",
			Reason: "symlink escapes allowed directory",
		}
	}

	return nil
}

// DefaultPathValidator is the default path validator instance.
// It uses the default key masking function.
var DefaultPathValidator = NewPathValidator(PathValidatorConfig{})

// ValidateFilePath validates a file path using the default validator.
// This is a convenience function for backward compatibility.
func ValidateFilePath(filename string) error {
	return DefaultPathValidator.Validate(filename)
}
