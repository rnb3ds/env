package internal

import (
	"sync"
	"sync/atomic"
	"testing"
)

// ============================================================================
// Concurrent Access Tests for InternKey
// ============================================================================

func TestInternKey_ConcurrentAccess(t *testing.T) {
	var wg sync.WaitGroup
	iterations := 1000
	concurrency := 10

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				key := "TEST_KEY_" + string(rune('A'+j%26))
				result := InternKey(key)
				if result != key {
					t.Errorf("InternKey(%q) = %q, want %q", key, result, key)
				}
			}
		}(i)
	}

	wg.Wait()
}

func TestInternKey_ConcurrentWithClear(t *testing.T) {
	for run := 0; run < 10; run++ {
		ClearInternCache()

		var wg sync.WaitGroup
		iterations := 100
		cleared := int64(0)

		// Interners
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < iterations; j++ {
					InternKey("TEST_KEY")
				}
			}()
		}

		// Clearers
		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < iterations/10; j++ {
					if atomic.CompareAndSwapInt64(&cleared, 0, 1) {
						ClearInternCache()
						atomic.StoreInt64(&cleared, 0)
					}
				}
			}()
		}

		wg.Wait()
	}
}

// ============================================================================
// Concurrent Access Tests for Expander
// ============================================================================

func TestExpander_ConcurrentAccess(t *testing.T) {
	lookup := func(key string) (string, bool) {
		switch key {
		case "VAR1":
			return "value1", true
		case "VAR2":
			return "value2", true
		default:
			return "", false
		}
	}

	expander := NewExpander(ExpanderConfig{
		MaxDepth: 5,
		Lookup:   lookup,
		Mode:     ModeAll,
	})

	var wg sync.WaitGroup
	iterations := 1000
	concurrency := 10

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				_, err := expander.Expand("$VAR1 and $VAR2")
				if err != nil {
					t.Errorf("Expand() error = %v", err)
				}
			}
		}()
	}

	wg.Wait()
}

func TestDetectCycle_Concurrent(t *testing.T) {
	var wg sync.WaitGroup
	iterations := 100
	concurrency := 5

	vars := map[string]string{
		"VAR1": "$VAR2",
		"VAR2": "$VAR3",
		"VAR3": "value",
	}

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				DetectCycle(vars)
			}
		}()
	}

	wg.Wait()
}

// ============================================================================
// Concurrent Access Tests for Auditor
// ============================================================================

func TestAuditor_ConcurrentAccess(t *testing.T) {
	handler := NewNopHandler()
	auditor := NewAuditor(handler,
		func(key string) bool { return false },
		func(key, value string) string { return value },
		true,
	)

	var wg sync.WaitGroup
	iterations := 1000
	concurrency := 10

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				auditor.Log(ActionSet, "KEY", "test", true)
				auditor.LogError(ActionGet, "KEY", "error")
				_ = auditor.IsEnabled()
			}
		}()
	}

	wg.Wait()
}

func TestAuditor_ConcurrentWithEnable(t *testing.T) {
	for run := 0; run < 10; run++ {
		handler := NewNopHandler()
		auditor := NewAuditor(handler,
			func(key string) bool { return false },
			func(key, value string) string { return value },
			true,
		)

		var wg sync.WaitGroup
		iterations := 100

		// Loggers
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < iterations; j++ {
					auditor.Log(ActionSet, "KEY", "test", true)
					_ = auditor.IsEnabled()
				}
			}()
		}

		// Enablers/Disablers
		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < iterations/10; j++ {
					auditor.SetEnabled(j%2 == 0)
				}
			}()
		}

		wg.Wait()
	}
}

// ============================================================================
// Concurrent Access Tests for Validator
// ============================================================================

func TestValidator_ConcurrentAccess(t *testing.T) {
	validator := NewValidator(ValidatorConfig{
		MaxKeyLength:   256,
		MaxValueLength: 4096,
	})

	var wg sync.WaitGroup
	iterations := 1000
	concurrency := 10

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				_ = validator.ValidateKey("VALID_KEY")
				_ = validator.ValidateValue("valid value")
			}
		}()
	}

	wg.Wait()
}

// ============================================================================
// Concurrent Access Tests for Pools
// ============================================================================

func TestPool_ConcurrentAccess(t *testing.T) {
	var wg sync.WaitGroup
	iterations := 1000
	concurrency := 10

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				// Builder pool
				builder := GetBuilder()
				builder.WriteString("test")
				PutBuilder(builder)

				// Byte slice pool
				buf := GetByteSlice()
				*buf = append(*buf, "test"...)
				PutByteSlice(buf)
			}
		}()
	}

	wg.Wait()
}

// ============================================================================
// Concurrent Access Tests for LineParser
// ============================================================================

func TestLineParser_ConcurrentAccess(t *testing.T) {
	validator := NewValidator(ValidatorConfig{
		MaxKeyLength:   256,
		MaxValueLength: 4096,
	})
	auditor := NewAuditor(NewNopHandler(),
		func(key string) bool { return false },
		func(key, value string) string { return value },
		false,
	)
	expander := NewExpander(ExpanderConfig{
		MaxDepth: 5,
		Lookup:   func(key string) (string, bool) { return "", false },
		Mode:     ModeNone,
	})

	parser := NewLineParser(LineParserConfig{
		AllowExportPrefix: true,
		AllowYamlSyntax:   true,
		OverwriteExisting: true,
		MaxVariables:      1000,
		ExpandVariables:   false,
	}, validator, auditor, expander)

	var wg sync.WaitGroup
	iterations := 1000
	concurrency := 10

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				_, _, _ = parser.ParseLine("KEY=value")
				_, _, _ = parser.ParseLineBytes([]byte("KEY=value"))
			}
		}()
	}

	wg.Wait()
}

// ============================================================================
// Stress Tests
// ============================================================================

func TestStress_InternKey(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	var wg sync.WaitGroup
	iterations := 10000
	concurrency := 50

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				key := "STRESS_KEY_" + string(rune('A'+j%26))
				InternKey(key)
			}
		}(i)
	}

	wg.Wait()
}
