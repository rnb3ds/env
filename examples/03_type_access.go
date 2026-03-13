//go:build examples

// Package main demonstrates typed access methods for environment variables.
// The library provides convenient getters for common types.
package main

import (
	"fmt"
	"log"
	"time"

	"github.com/cybergodev/env"
)

func main() {
	// Load configuration from JSON file
	if err := env.Load("examples/data/config.json"); err != nil {
		log.Fatalf("Failed to load: %v", err)
	}

	fmt.Println("=== String Access ===")
	demonstrateStringAccess()

	fmt.Println("\n=== Integer Access ===")
	demonstrateIntAccess()

	fmt.Println("\n=== Boolean Access ===")
	demonstrateBoolAccess()

	fmt.Println("\n=== Duration Access ===")
	demonstrateDurationAccess()

	fmt.Println("\n=== Slice Access ===")
	demonstrateSliceAccess()

	fmt.Println("\n=== Lookup and Existence ===")
	demonstrateLookup()
}

func demonstrateStringAccess() {
	// Simple string get
	name := env.GetString("app.name")
	fmt.Printf("app.name: %q\n", name)

	// With default value
	envType := env.GetString("config.env", "development")
	fmt.Printf("config.env: %q\n", envType)

	// Nested path access (dot notation)
	dbHost := env.GetString("db.host", "localhost")
	fmt.Printf("db.host: %q\n", dbHost)
}

func demonstrateIntAccess() {
	// Integer with default
	port := env.GetInt("app.port", 9090)
	fmt.Printf("app.port: %d\n", port)

	// Nested integer
	maxConn := env.GetInt("db.max_connections", 10)
	fmt.Printf("db.max_connections: %d\n", maxConn)

	// Missing key returns 0 or default
	missing := env.GetInt("nonexistent", 42)
	fmt.Printf("nonexistent (with default): %d\n", missing)
}

func demonstrateBoolAccess() {
	// Boolean values
	debug := env.GetBool("app.debug", false)
	fmt.Printf("app.debug: %v\n", debug)

	// Feature flags
	cacheEnabled := env.GetBool("cache.enabled", false)
	fmt.Printf("cache.enabled: %v\n", cacheEnabled)

	rateLimit := env.GetBool("features.rate_limit", true)
	fmt.Printf("features.rate_limit: %v\n", rateLimit)
}

func demonstrateDurationAccess() {
	// Duration parsing
	timeout := env.GetDuration("db.timeout", 10*time.Second)
	fmt.Printf("db.timeout: %v\n", timeout)

	// Cache TTL
	ttl := env.GetDuration("cache.ttl", 5*time.Minute)
	fmt.Printf("cache.ttl: %v\n", ttl)

	// With default for missing
	missing := env.GetDuration("missing.duration", 1*time.Hour)
	fmt.Printf("missing.duration (default): %v\n", missing)
}

func demonstrateSliceAccess() {
	// Indexed access (arrays in JSON/YAML)
	host0 := env.GetString("cache.hosts.0")
	fmt.Printf("cache.hosts.0: %q\n", host0)

	// String slice from array
	hosts := env.GetSlice[string]("cache.hosts")
	fmt.Printf("cache.hosts: %v\n", hosts)

	// Integer slice
	ports := env.GetSlice("ports", []int{8080, 8081})
	fmt.Printf("ports (default): %v\n", ports)
}

func demonstrateLookup() {
	// Check existence and get value
	if value, exists := env.Lookup("app.port"); exists {
		fmt.Printf("app.port exists: %v\n", value)
	}

	if value, exists := env.Lookup("db.password"); exists {
		fmt.Printf("db.password exists: %v\n", value)
	}

	// Missing key
	if _, exists := env.Lookup("nonexistent.key"); !exists {
		fmt.Println("nonexistent.key does not exist")
	}
}
