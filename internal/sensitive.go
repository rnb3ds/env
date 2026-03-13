// Package internal provides sensitive key detection and masking utilities.
package internal

import (
	"fmt"
	"strings"
	"unicode"
)

// Patterns defines patterns that indicate sensitive data.
// This is the single source of truth for sensitive key detection.
// Keys containing these patterns (case-insensitive) will be masked in logs and errors.
var Patterns = []string{
	// Authentication & Authorization
	"PASSWORD",
	"SECRET",
	"TOKEN",
	"AUTH",
	"CREDENTIAL",
	"PASSPHRASE",
	"SESSION",
	"COOKIE",

	// API & Keys
	"API_KEY",
	"APIKEY",
	"ACCESS_KEY",
	"SECRET_KEY",
	"PRIVATE_KEY",
	"PUBLIC_KEY", // May contain paired private key context

	// Encryption & Security
	"PRIVATE",
	"ENCRYPTION_KEY",
	"ENCRYPT_KEY",
	"DECRYPT_KEY",
	"SIGNING_KEY",
	"SIGN_KEY",
	"VERIFY_KEY",

	// Financial & Personal (PII)
	"SSN",
	"SOCIAL_SECURITY",
	"CREDIT_CARD",
	"CARD_NUMBER",
	"CVV",
	"CVC",
	"CCV",
	"PAN", // Primary Account Number

	// Crypto & Blockchain
	"MNEMONIC",
	"SEED",
	"RECOVERY",
	"WALLET",
	"PRIVATE_ADDRESS",

	// Database & Infrastructure
	"CONNECTION_STRING",
	"CONN_STRING",
	"DATABASE_URL",
	"DB_PASSWORD",

	// Cloud & Services
	"AWS_SECRET",
	"AZURE_KEY",
	"GCP_KEY",
	"SERVICE_ACCOUNT",
}

// upperPatterns contains pre-uppercased patterns for faster matching.
// This avoids calling strings.ToUpper for each pattern during IsKey.
var upperPatterns = Patterns // Already uppercase

// sanitizePatterns contains lowercase patterns for sanitizing logs.
// These patterns are used to detect and mask sensitive key=value pairs.
var sanitizePatterns = []string{
	// Authentication
	"password=",
	"secret=",
	"token=",
	"auth=",
	"credential=",
	"passphrase=",
	"session=",
	"cookie=",

	// API & Keys
	"api_key=",
	"apikey=",
	"access_key=",
	"secret_key=",
	"private_key=",
	"public_key=",

	// Encryption
	"encrypt_key=",
	"decrypt_key=",
	"signing_key=",

	// Financial (PII)
	"ssn=",
	"credit_card=",
	"card_number=",
	"cvv=",
	"cvc=",

	// Crypto
	"mnemonic=",
	"seed=",
	"recovery=",
	"wallet=",

	// Database
	"connection_string=",
	"database_url=",
	"db_password=",
}

// IsKey determines if a key likely contains sensitive data.
// Uses byte-level comparison to avoid allocations from strings.ToUpper.
func IsKey(key string) bool {
	if len(key) == 0 {
		return false
	}

	// Check each pattern using zero-allocation case-insensitive matching
	for _, pattern := range upperPatterns {
		if containsIgnoreCase(key, pattern) {
			return true
		}
	}
	return false
}

// containsIgnoreCase checks if s contains pattern (case-insensitive, ASCII only).
// Pattern must be uppercase. This function avoids any heap allocations.
func containsIgnoreCase(s, pattern string) bool {
	patternLen := len(pattern)
	sLen := len(s)
	if patternLen > sLen || patternLen == 0 {
		return false
	}

	// Slide window through s
	for i := 0; i <= sLen-patternLen; i++ {
		match := true
		for j := 0; j < patternLen; j++ {
			c := s[i+j]
			// Convert to uppercase inline
			if c >= 'a' && c <= 'z' {
				c -= 32
			}
			if c != pattern[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// MaskValue masks a value based on its sensitivity.
// This is a utility function for general value masking.
func MaskValue(key, value string) string {
	if IsKey(key) {
		return fmt.Sprintf("[MASKED:%d chars]", len(value))
	}
	if len(value) <= 20 {
		return value
	}
	return value[:17] + "..."
}

// MaskKey masks a key name for logging purposes.
func MaskKey(key string) string {
	if len(key) <= 3 {
		return "***"
	}
	return key[:2] + "***"
}

// MaskInString masks potentially sensitive content in a string.
func MaskInString(s string) string {
	const maxLen = 50
	if len(s) > maxLen {
		return s[:maxLen-3] + "..."
	}
	return s
}

// SanitizeForLog removes potentially sensitive information from a string.
// It scans for patterns that might indicate sensitive data and masks them.
func SanitizeForLog(s string) string {
	result := s
	// Convert to lowercase once for pattern matching
	lowerResult := strings.ToLower(result)

	for _, pattern := range sanitizePatterns {
		idx := strings.Index(lowerResult, pattern)
		if idx != -1 {
			// Find the value after the pattern
			start := idx + len(pattern)
			end := start
			for end < len(result) && result[end] != ' ' && result[end] != '\n' && result[end] != '\t' {
				end++
			}
			if end > start {
				// Update both result and lowerResult to keep them in sync
				maskedPattern := "[MASKED]"
				result = result[:start] + maskedPattern + result[end:]
				lowerResult = lowerResult[:start] + strings.ToLower(maskedPattern) + lowerResult[end:]
			}
		}
	}

	// Remove control characters
	result = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return -1
		}
		return r
	}, result)

	return result
}
