//go:build examples

// Package main demonstrates the quick start usage of the env library.
// This is the simplest way to load and access environment variables.
package main

import (
	"fmt"
	"log"
	"time"

	"github.com/cybergodev/env"
)

func main() {
	// Initialize environment variables from a .env file.
	// This is a one-liner that loads and applies variables to os.Environ.
	if err := env.Load("examples/data/config.env"); err != nil {
		log.Fatalf("Failed to load env: %v", err)
	}

	// Get string value with default
	appName := env.GetString("APP_NAME", "unknown")
	fmt.Printf("APP_NAME: %s\n", appName)

	// Get integer value with default
	port := env.GetInt("APP_PORT", 9090)
	fmt.Printf("APP_PORT: %d\n", port)

	// Get boolean value
	debug := env.GetBool("DEBUG", false)
	fmt.Printf("DEBUG: %v\n", debug)

	// Get duration value
	timeout := env.GetDuration("DB_TIMEOUT", 10*time.Second)
	fmt.Printf("DB_TIMEOUT: %v\n", timeout)

	// Check if a key exists
	if value, exists := env.Lookup("DB_PASSWORD"); exists {
		fmt.Printf("DB_PASSWORD: [HIDDEN - %d chars]\n", len(value))
	}

	// Set a new value at runtime
	if err := env.Set("RUNTIME_VAR", "set_at_runtime"); err != nil {
		log.Printf("Failed to set: %v", err)
	}
	fmt.Printf("RUNTIME_VAR: %s\n", env.GetString("RUNTIME_VAR"))
}
