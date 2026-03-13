//go:build examples

// Package main demonstrates SecureValue for handling sensitive data.
// SecureValue automatically zeros memory when closed or garbage collected.
package main

import (
	"fmt"
	"log"

	"github.com/cybergodev/env"
)

func main() {
	fmt.Println("=== SecureValue Basics ===")
	demonstrateBasics()

	fmt.Println("\n=== SecureValue from Loader ===")
	demonstrateFromLoader()

	fmt.Println("\n=== Lifecycle: Close vs Release ===")
	demonstrateLifecycle()
}

func demonstrateBasics() {
	// Create a SecureValue from a sensitive string
	password := env.NewSecureValue("super_secret_password_123")

	// Access the value (creates a copy)
	fmt.Printf("Value: %s\n", password.String())

	// Get masked representation (safe for logging)
	fmt.Printf("Masked: %s\n", password.Masked())

	// Get length without exposing the value
	fmt.Printf("Length: %d bytes\n", password.Length())

	// Clean up when done (zeros the memory)
	password.Close()
}

func demonstrateFromLoader() {
	cfg := env.DefaultConfig()
	cfg.Filenames = []string{"examples/data/config.env"}

	loader, err := env.New(cfg)
	if err != nil {
		log.Fatalf("Failed to create loader: %v", err)
	}
	defer loader.Close()

	// Get sensitive values as SecureValue
	securePassword := loader.GetSecure("DB_PASSWORD")
	if securePassword != nil {
		fmt.Printf("DB_PASSWORD: %s\n", securePassword.Masked())
		securePassword.Close()
	}

	secureAPIKey := loader.GetSecure("API_KEY")
	if secureAPIKey != nil {
		fmt.Printf("API_KEY: %s\n", secureAPIKey.Masked())
		secureAPIKey.Close()
	}
}

func demonstrateLifecycle() {
	// Close() zeros memory but doesn't return to pool
	secret := env.NewSecureValue("temporary_secret")
	fmt.Printf("Before close: %s\n", secret.Masked())
	secret.Close()
	fmt.Printf("After close: %s\n", secret.Masked())

	// Release() zeros memory AND returns to pool (more efficient for frequent use)
	secret2 := env.NewSecureValue("another_secret")
	bytes := secret2.Bytes()
	fmt.Printf("\nBytes length: %d\n", len(bytes))

	// Clear external copies when done
	env.ClearBytes(bytes)

	// Release returns the object to the pool for reuse
	secret2.Release()
}
