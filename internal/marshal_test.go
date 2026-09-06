package internal

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

type testStruct struct {
	Name    string        `env:"NAME"`
	Port    int           `env:"PORT"`
	Enabled bool          `env:"ENABLED"`
	Timeout time.Duration `env:"TIMEOUT"`
}

type nestedStruct struct {
	Config testStruct `env:"CONFIG"`
}

type taggedStruct struct {
	CustomName string `env:"MY_CUSTOM_NAME"`
	Ignored    string `env:"-"`
}

func TestStruct(t *testing.T) {
	tests := []struct {
		name    string
		input   interface{}
		prefix  string
		want    map[string]string
		wantErr bool
	}{
		{
			name: "simple struct",
			input: testStruct{
				Name:    "test",
				Port:    8080,
				Enabled: true,
				Timeout: 5 * time.Second,
			},
			want: map[string]string{
				"NAME":    "test",
				"PORT":    "8080",
				"ENABLED": "true",
				"TIMEOUT": "5s",
			},
			wantErr: false,
		},
		{
			name: "nested struct",
			input: nestedStruct{
				Config: testStruct{
					Name: "nested",
				},
			},
			want: map[string]string{
				"CONFIG_NAME": "nested",
			},
			wantErr: false,
		},
		{
			name: "with prefix",
			input: testStruct{
				Name: "test",
			},
			prefix: "APP",
			want: map[string]string{
				"APP_NAME": "test",
			},
			wantErr: false,
		},
		{
			name: "ignored field",
			input: taggedStruct{
				CustomName: "value",
				Ignored:    "should not appear",
			},
			want: map[string]string{
				"MY_CUSTOM_NAME": "value",
			},
			wantErr: false,
		},
		{
			name:    "nil input",
			input:   nil,
			want:    nil,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Struct(tt.input, tt.prefix)
			if (err != nil) != tt.wantErr {
				t.Errorf("Struct() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			for k, v := range tt.want {
				if result[k] != v {
					t.Errorf("Struct()[%q] = %q, want %q", k, result[k], v)
				}
			}
		})
	}
}

func TestStructInto(t *testing.T) {
	data := map[string]string{
		"NAME":    "test",
		"PORT":    "8080",
		"ENABLED": "true",
		"TIMEOUT": "5s",
	}

	var result testStruct
	err := StructInto(data, reflect.ValueOf(&result).Elem(), "")
	if err != nil {
		t.Errorf("StructInto() error = %v", err)
		return
	}

	if result.Name != "test" {
		t.Errorf("Name = %q, want %q", result.Name, "test")
	}
	if result.Port != 8080 {
		t.Errorf("Port = %d, want %d", result.Port, 8080)
	}
	if result.Enabled != true {
		t.Errorf("Enabled = %v, want %v", result.Enabled, true)
	}
	if result.Timeout != 5*time.Second {
		t.Errorf("Timeout = %v, want %v", result.Timeout, 5*time.Second)
	}
}

func TestStructIntoWithDefaults(t *testing.T) {
	type structWithDefaults struct {
		Name    string `env:"NAME" envDefault:"default_name"`
		Port    int    `env:"PORT" envDefault:"3000"`
		Missing string `env:"MISSING"` // No default
	}

	data := map[string]string{} // Empty data

	var result structWithDefaults
	err := StructInto(data, reflect.ValueOf(&result).Elem(), "")
	if err != nil {
		t.Errorf("StructInto() error = %v", err)
		return
	}

	if result.Name != "default_name" {
		t.Errorf("Name = %q, want default", result.Name)
	}
	if result.Port != 3000 {
		t.Errorf("Port = %d, want 3000", result.Port)
	}
	if result.Missing != "" {
		t.Errorf("Missing = %q, should be empty", result.Missing)
	}
}

func TestValueToString(t *testing.T) {
	tests := []struct {
		name    string
		input   interface{}
		want    string
		wantErr bool
	}{
		{"string", "hello", "hello", false},
		{"int", 42, "42", false},
		{"bool true", true, "true", false},
		{"bool false", false, "false", false},
		{"float", 3.14, "3.14", false},
		{"duration", 5 * time.Second, "5s", false},
		{"byte slice", []byte("bytes"), "bytes", false},
		{"string slice", []string{"a", "b"}, "a,b", false},
		{"int8", int8(8), "8", false},
		{"int16", int16(16), "16", false},
		{"int32", int32(32), "32", false},
		{"int64", int64(64), "64", false},
		{"uint", uint(10), "10", false},
		{"uint8", uint8(8), "8", false},
		{"uint16", uint16(16), "16", false},
		{"uint32", uint32(32), "32", false},
		{"uint64", uint64(64), "64", false},
		{"float32", float32(3.5), "3.5", false}, // exact-representable float32 value
		{"nil pointer", (*string)(nil), "", false},
		{"non-nil pointer dereferences", intptr(42), "42", false},
		{"invalid value (nil interface)", nil, "", false},
		{"TextMarshaler (time.Time)", time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC), "2026-01-02T03:04:05Z", false},
		{"json.Marshaler", json.RawMessage(`{"a":1}`), `{"a":1}`, false},
		{"empty string slice", []string{}, "", false},
		{"unsupported slice", []int{1, 2, 3}, "", true},
		{"unsupported map", map[string]string{"a": "b"}, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val := reflect.ValueOf(tt.input)
			result, err := valueToString(val)
			if (err != nil) != tt.wantErr {
				t.Errorf("valueToString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if result != tt.want {
				t.Errorf("valueToString() = %q, want %q", result, tt.want)
			}
		})
	}
}

// intptr returns a pointer to v; used to express pointer-typed expectations
// in the setFieldValue table.
func intptr(v int) *int { return &v }

func TestSetFieldValue(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		target  interface{}
		want    interface{}
		wantErr bool
	}{
		{"string", "hello", new(string), "hello", false},
		{"int", "42", new(int), 42, false},
		{"bool", "true", new(bool), true, false},
		{"float", "3.14", new(float64), 3.14, false},
		{"empty value for non-string leaves zero value", "", new(int), 0, false},
		{"pointer to int", "42", new(*int), intptr(42), false},
		{"uint types", "42", new(uint), uint(42), false},
		{"float32 type", "3.14", new(float32), float32(3.14), false},
		{"invalid int", "not_a_number", new(int), 0, true},
		{"invalid bool", "not_a_bool", new(bool), false, true},
		{"unsupported type", "value", new(map[string]string), nil, true},
		// Narrow-width fields must reject out-of-range values instead of
		// silently truncating them (300 → 44 in int8, 256 → 0 in uint8).
		{"int8 overflow", "300", new(int8), 0, true},
		{"int16 overflow", "40000", new(int16), 0, true},
		{"uint8 overflow", "256", new(uint8), 0, true},
		{"uint16 overflow", "70000", new(uint16), 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val := reflect.ValueOf(tt.target).Elem()
			err := setFieldValue(val, tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("setFieldValue() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				got := val.Interface()
				if !reflect.DeepEqual(got, tt.want) {
					t.Errorf("setFieldValue() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestSetSliceValue(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		target  interface{}
		want    interface{}
		wantErr bool
	}{
		{"string slice", "a,b,c", new([]string), []string{"a", "b", "c"}, false},
		{"int slice", "1,2,3", new([]int), []int{1, 2, 3}, false},
		{"bool slice", "true,false", new([]bool), []bool{true, false}, false},
		{"int slice with spaces", " 1 , 2 , 3 ", new([]int), []int{1, 2, 3}, false},
		{"uint slice", "1,2,3", new([]uint), []uint{1, 2, 3}, false},
		{"float slice", "1.1,2.2,3.3", new([]float64), []float64{1.1, 2.2, 3.3}, false},
		// Boundary: empty input yields an empty (possibly nil) slice; the
		// resulting value is unspecified beyond being empty, so only the
		// error is asserted.
		{"empty", "", new([]string), nil, false},
		{"invalid int slice element", "1,not_a_number,3", new([]int), nil, true},
		{"invalid bool slice element", "true,not_a_bool", new([]bool), nil, true},
		{"unsupported slice element type", "value", new([]map[string]string), nil, true},
		// Slice elements follow the same bit-width rule as scalar fields.
		{"int8 slice element overflow", "300", new([]int8), nil, true},
		{"uint8 slice element overflow", "256", new([]uint8), nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val := reflect.ValueOf(tt.target).Elem()
			err := setSliceValue(val, tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("setSliceValue() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && tt.value != "" {
				got := val.Interface()
				if !reflect.DeepEqual(got, tt.want) {
					t.Errorf("setSliceValue() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

// ============================================================================
// Struct Edge Cases Tests
// ============================================================================

func TestStruct_EdgeCases(t *testing.T) {
	t.Run("nil pointer to struct", func(t *testing.T) {
		var s *testStruct
		result, err := Struct(s, "")
		if err != nil {
			t.Errorf("Struct(nil pointer) error = %v", err)
		}
		if result != nil {
			t.Errorf("Struct(nil pointer) = %v, want nil", result)
		}
	})

	t.Run("non-struct input", func(t *testing.T) {
		_, err := Struct("not a struct", "")
		if err == nil {
			t.Error("Struct(string) should return error")
		}
	})

	t.Run("pointer to nested struct", func(t *testing.T) {
		type outer struct {
			Inner *testStruct `env:"INNER"`
		}
		s := outer{
			Inner: &testStruct{
				Name: "nested",
			},
		}
		result, err := Struct(s, "")
		if err != nil {
			t.Errorf("Struct() error = %v", err)
		}
		if result["INNER_NAME"] != "nested" {
			t.Errorf("result[\"INNER_NAME\"] = %q, want \"nested\"", result["INNER_NAME"])
		}
	})

	t.Run("empty struct", func(t *testing.T) {
		type empty struct{}
		result, err := Struct(empty{}, "")
		if err != nil {
			t.Errorf("Struct(empty) error = %v", err)
		}
		if len(result) != 0 {
			t.Errorf("Struct(empty) = %v, want empty map", result)
		}
	})

	t.Run("unexported field ignored", func(t *testing.T) {
		type withUnexported struct {
			Exported   string `env:"EXPORTED"`
			unexported string // should be ignored
		}
		s := withUnexported{
			Exported:   "value",
			unexported: "secret",
		}
		result, err := Struct(s, "")
		if err != nil {
			t.Errorf("Struct() error = %v", err)
		}
		if len(result) != 1 || result["EXPORTED"] != "value" {
			t.Errorf("Struct() = %v, want only EXPORTED", result)
		}
	})
}

// ============================================================================
// StructInto Edge Cases Tests
// ============================================================================

func TestStructInto_EdgeCases(t *testing.T) {
	t.Run("nested struct", func(t *testing.T) {
		type inner struct {
			Value string `env:"VALUE"`
		}
		type outer struct {
			Inner inner `env:"INNER"`
		}

		data := map[string]string{
			"INNER_VALUE": "nested_value",
		}

		var result outer
		err := StructInto(data, reflect.ValueOf(&result).Elem(), "")
		if err != nil {
			t.Errorf("StructInto() error = %v", err)
		}
		if result.Inner.Value != "nested_value" {
			t.Errorf("result.Inner.Value = %q, want \"nested_value\"", result.Inner.Value)
		}
	})

	t.Run("pointer to nested struct", func(t *testing.T) {
		type inner struct {
			Value string `env:"VALUE"`
		}
		type outer struct {
			Inner *inner `env:"INNER"`
		}

		data := map[string]string{
			"INNER_VALUE": "pointer_value",
		}

		var result outer
		err := StructInto(data, reflect.ValueOf(&result).Elem(), "")
		if err != nil {
			t.Errorf("StructInto() error = %v", err)
		}
		if result.Inner == nil || result.Inner.Value != "pointer_value" {
			t.Errorf("result.Inner.Value = %v, want \"pointer_value\"", result.Inner)
		}
	})

	t.Run("env tag dash skips field", func(t *testing.T) {
		type withSkip struct {
			Value  string `env:"VALUE"`
			SkipMe string `env:"-"`
		}

		data := map[string]string{
			"VALUE":  "kept",
			"SKIPME": "should_not_be_set",
		}

		var result withSkip
		err := StructInto(data, reflect.ValueOf(&result).Elem(), "")
		if err != nil {
			t.Errorf("StructInto() error = %v", err)
		}
		if result.Value != "kept" {
			t.Errorf("result.Value = %q, want \"kept\"", result.Value)
		}
		if result.SkipMe != "" {
			t.Errorf("result.SkipMe = %q, should be empty (skipped)", result.SkipMe)
		}
	})

	t.Run("unexported field skipped", func(t *testing.T) {
		type withUnexported struct {
			Exported   string `env:"EXPORTED"`
			unexported string
		}

		data := map[string]string{
			"EXPORTED":   "value",
			"UNEXPORTED": "should_be_ignored",
		}

		var result withUnexported
		err := StructInto(data, reflect.ValueOf(&result).Elem(), "")
		if err != nil {
			t.Errorf("StructInto() error = %v", err)
		}
		if result.Exported != "value" {
			t.Errorf("result.Exported = %q, want \"value\"", result.Exported)
		}
		// unexported field should remain empty
		if result.unexported != "" {
			t.Errorf("result.unexported = %q, should be empty", result.unexported)
		}
	})
}

// ============================================================================
// Lowercase and Dot-Notation Tag Tests
// ============================================================================

func TestStructInto_LowercaseTag(t *testing.T) {
	t.Run("lowercase tag matches uppercase key", func(t *testing.T) {
		type Config struct {
			APIKey string `env:"deepseek_key"`
		}

		data := map[string]string{"DEEPSEEK_KEY": "sk-123"}
		var result Config
		err := StructInto(data, reflect.ValueOf(&result).Elem(), "")
		if err != nil {
			t.Errorf("StructInto() error = %v", err)
		}
		if result.APIKey != "sk-123" {
			t.Errorf("APIKey = %q, want %q", result.APIKey, "sk-123")
		}
	})

	t.Run("lowercase tag with prefix", func(t *testing.T) {
		type Config struct {
			Host string `env:"host"`
		}

		data := map[string]string{"APP_HOST": "localhost"}
		var result Config
		err := StructInto(data, reflect.ValueOf(&result).Elem(), "app")
		if err != nil {
			t.Errorf("StructInto() error = %v", err)
		}
		if result.Host != "localhost" {
			t.Errorf("Host = %q, want %q", result.Host, "localhost")
		}
	})

	t.Run("mixed case tag", func(t *testing.T) {
		type Config struct {
			Key string `env:"DeepSeek_Key"`
		}

		data := map[string]string{"DEEPSEEK_KEY": "val"}
		var result Config
		err := StructInto(data, reflect.ValueOf(&result).Elem(), "")
		if err != nil {
			t.Errorf("StructInto() error = %v", err)
		}
		if result.Key != "val" {
			t.Errorf("Key = %q, want %q", result.Key, "val")
		}
	})
}

func TestStructInto_DotNotationTag(t *testing.T) {
	t.Run("dot-notation tag resolves to flattened key", func(t *testing.T) {
		type Config struct {
			Host string `env:"database.host"`
		}

		data := map[string]string{"DATABASE_HOST": "localhost"}
		var result Config
		err := StructInto(data, reflect.ValueOf(&result).Elem(), "")
		if err != nil {
			t.Errorf("StructInto() error = %v", err)
		}
		if result.Host != "localhost" {
			t.Errorf("Host = %q, want %q", result.Host, "localhost")
		}
	})

	t.Run("multi-level dot-notation", func(t *testing.T) {
		type Config struct {
			Port string `env:"server.http.port"`
		}

		data := map[string]string{"SERVER_HTTP_PORT": "8080"}
		var result Config
		err := StructInto(data, reflect.ValueOf(&result).Elem(), "")
		if err != nil {
			t.Errorf("StructInto() error = %v", err)
		}
		if result.Port != "8080" {
			t.Errorf("Port = %q, want %q", result.Port, "8080")
		}
	})

	t.Run("dot-notation with envDefault", func(t *testing.T) {
		type Config struct {
			Host string `env:"database.host" envDefault:"127.0.0.1"`
		}

		data := map[string]string{}
		var result Config
		err := StructInto(data, reflect.ValueOf(&result).Elem(), "")
		if err != nil {
			t.Errorf("StructInto() error = %v", err)
		}
		if result.Host != "127.0.0.1" {
			t.Errorf("Host = %q, want %q", result.Host, "127.0.0.1")
		}
	})

	t.Run("dot-notation takes precedence over default", func(t *testing.T) {
		type Config struct {
			Host string `env:"database.host" envDefault:"127.0.0.1"`
		}

		data := map[string]string{"DATABASE_HOST": "production.db"}
		var result Config
		err := StructInto(data, reflect.ValueOf(&result).Elem(), "")
		if err != nil {
			t.Errorf("StructInto() error = %v", err)
		}
		if result.Host != "production.db" {
			t.Errorf("Host = %q, want %q", result.Host, "production.db")
		}
	})
}

// TestStructIntoNestedPrefixSymmetry pins the marshal/unmarshal prefix
// symmetry: nested structs map to PARENT_CHILD keys in both directions,
// matching the JSON/YAML flatteners. Regression: StructInto previously passed
// no prefix for untagged nested structs (looking up HOST instead of
// INNER_HOST), so flattened nested values were silently dropped.
func TestStructIntoNestedPrefixSymmetry(t *testing.T) {
	type Inner struct {
		Host string
	}
	type Outer struct {
		Inner Inner
		Name  string
	}

	envMap, err := Struct(Outer{Inner: Inner{Host: "db.example.com"}, Name: "app"}, "")
	if err != nil {
		t.Fatalf("Struct() error = %v", err)
	}
	if envMap["INNER_HOST"] != "db.example.com" || envMap["NAME"] != "app" {
		t.Fatalf("Struct() = %v, want INNER_HOST=db.example.com NAME=app", envMap)
	}

	var out Outer
	if err := StructInto(envMap, reflect.ValueOf(&out).Elem(), ""); err != nil {
		t.Fatalf("StructInto() error = %v", err)
	}
	if out.Inner.Host != "db.example.com" {
		t.Errorf("out.Inner.Host = %q, want %q (nested value was silently dropped)", out.Inner.Host, "db.example.com")
	}
	if out.Name != "app" {
		t.Errorf("out.Name = %q, want %q", out.Name, "app")
	}
}

// TestStructIntoMultiLevelTaggedPrefixAccumulates covers tagged multi-level
// nesting: marshal emits M_I_HOST, so unmarshal must accumulate prefixes and
// look up the same key (regression: it previously looked up I_HOST).
func TestStructIntoMultiLevelTaggedPrefixAccumulates(t *testing.T) {
	type Leaf struct {
		Host string `env:"HOST"`
	}
	type Mid struct {
		Leaf Leaf `env:"I"`
	}
	type Top struct {
		Mid Mid `env:"M"`
	}

	envMap, err := Struct(Top{Mid: Mid{Leaf: Leaf{Host: "h"}}}, "")
	if err != nil {
		t.Fatalf("Struct() error = %v", err)
	}
	if envMap["M_I_HOST"] != "h" {
		t.Fatalf("Struct() = %v, want M_I_HOST=h", envMap)
	}

	var out Top
	if err := StructInto(envMap, reflect.ValueOf(&out).Elem(), ""); err != nil {
		t.Fatalf("StructInto() error = %v", err)
	}
	if out.Mid.Leaf.Host != "h" {
		t.Errorf("out.Mid.Leaf.Host = %q, want %q", out.Mid.Leaf.Host, "h")
	}
}

// ============================================================================
// Error Propagation & Rare Branches
// ============================================================================

type marshalBadField struct {
	Ch chan int
}

type marshalNestedBad struct {
	Inner marshalBadField
}

type marshalPtrNestedBad struct {
	Inner *marshalBadField
}

// TestMarshalStruct_UnsupportedKindErrors pins the error wrap chain: an
// unsupported field kind (chan) errors at the top level naming the field,
// and through nested structs and struct pointers the inner error propagates
// unchanged to the caller.
func TestMarshalStruct_UnsupportedKindErrors(t *testing.T) {
	t.Run("top-level field", func(t *testing.T) {
		_, err := Struct(marshalBadField{Ch: make(chan int, 1)}, "")
		if err == nil {
			t.Fatal("Struct(chan field) error = nil, want error")
		}
		if !strings.Contains(err.Error(), "Ch") {
			t.Errorf("error = %v, want it to name field Ch", err)
		}
	})

	t.Run("nested struct", func(t *testing.T) {
		_, err := Struct(marshalNestedBad{Inner: marshalBadField{Ch: make(chan int, 1)}}, "")
		if err == nil {
			t.Error("Struct(nested chan field) error = nil, want error")
		}
	})

	t.Run("pointer to nested struct", func(t *testing.T) {
		_, err := Struct(marshalPtrNestedBad{Inner: &marshalBadField{Ch: make(chan int, 1)}}, "")
		if err == nil {
			t.Error("Struct(ptr-nested chan field) error = nil, want error")
		}
	})
}

// TestMarshalStruct_TopLevelPointer covers Struct invoked with a non-nil
// struct pointer (the val.Elem() dereference path).
func TestMarshalStruct_TopLevelPointer(t *testing.T) {
	type ptrTarget struct {
		Host string `env:"HOST"`
	}
	got, err := Struct(&ptrTarget{Host: "h1"}, "")
	if err != nil {
		t.Fatalf("Struct(&struct) error = %v", err)
	}
	if got["HOST"] != "h1" {
		t.Errorf("result[HOST] = %q, want %q", got["HOST"], "h1")
	}
}

// TestStructInto_InlineDefaultTag covers the "KEY,envDefault:value" inline
// tag syntax: the key is everything before the comma; the inline default
// applies when the data map lacks the key.
func TestStructInto_InlineDefaultTag(t *testing.T) {
	type inlineDefault struct {
		Mode string `env:"MODE,envDefault:prod"`
	}

	t.Run("data value wins over inline default", func(t *testing.T) {
		var s inlineDefault
		if err := StructInto(map[string]string{"MODE": "dev"}, reflect.ValueOf(&s).Elem(), ""); err != nil {
			t.Fatalf("StructInto error = %v", err)
		}
		if s.Mode != "dev" {
			t.Errorf("Mode = %q, want %q", s.Mode, "dev")
		}
	})

	t.Run("inline default applies", func(t *testing.T) {
		var s inlineDefault
		if err := StructInto(map[string]string{}, reflect.ValueOf(&s).Elem(), ""); err != nil {
			t.Fatalf("StructInto error = %v", err)
		}
		if s.Mode != "prod" {
			t.Errorf("Mode = %q, want %q", s.Mode, "prod")
		}
	})
}

// TestStructInto_NestedErrorPropagation pins that parse failures inside
// nested structs — plain and pointer — propagate to the outer StructInto
// call.
func TestStructInto_NestedErrorPropagation(t *testing.T) {
	type inner struct {
		N int
	}
	type plain struct {
		In inner `env:"IN"`
	}
	type ptr struct {
		In *inner `env:"IN"`
	}

	t.Run("nested struct field", func(t *testing.T) {
		var s plain
		if err := StructInto(map[string]string{"IN_N": "abc"}, reflect.ValueOf(&s).Elem(), ""); err == nil {
			t.Error("StructInto(nested bad int) error = nil, want error")
		}
	})

	t.Run("pointer to nested struct field", func(t *testing.T) {
		var s ptr
		if err := StructInto(map[string]string{"IN_N": "abc"}, reflect.ValueOf(&s).Elem(), ""); err == nil {
			t.Error("StructInto(ptr-nested bad int) error = nil, want error")
		}
	})
}

// errTextMarshaler and errJSONMarshaler are deliberately string-kind (not
// structs): a struct field of struct kind is recursed into by marshalStruct
// and never reaches valueToString's marshaler checks.
type errTextMarshaler string

func (errTextMarshaler) MarshalText() ([]byte, error) { return nil, errors.New("text boom") }

type errJSONMarshaler string

func (errJSONMarshaler) MarshalJSON() ([]byte, error) { return nil, errors.New("json boom") }

// TestMarshalStruct_MarshalerErrors covers TextMarshaler and json.Marshaler
// implementations that fail: the error must surface instead of being
// swallowed.
func TestMarshalStruct_MarshalerErrors(t *testing.T) {
	t.Run("TextMarshaler error", func(t *testing.T) {
		type host struct {
			F errTextMarshaler
		}
		if _, err := Struct(host{}, ""); err == nil {
			t.Error("Struct(failing TextMarshaler) error = nil, want error")
		}
	})

	t.Run("json.Marshaler error", func(t *testing.T) {
		type host struct {
			F errJSONMarshaler
		}
		if _, err := Struct(host{}, ""); err == nil {
			t.Error("Struct(failing json.Marshaler) error = nil, want error")
		}
	})
}

type textUnmarshalTarget struct {
	got string
}

func (t *textUnmarshalTarget) UnmarshalText(b []byte) error {
	t.got = string(b)
	return nil
}

// TestSetFieldValue_RarePaths covers the TextUnmarshaler delegation, the
// time.Duration and float parse errors, slice passthrough to setSliceValue,
// and the float-slice parse error.
func TestSetFieldValue_RarePaths(t *testing.T) {
	t.Run("TextUnmarshaler field", func(t *testing.T) {
		var target textUnmarshalTarget
		field := reflect.ValueOf(&target).Elem()
		if err := setFieldValue(field, "hello"); err != nil {
			t.Fatalf("setFieldValue(TextUnmarshaler) error = %v", err)
		}
		if target.got != "hello" {
			t.Errorf("target.got = %q, want %q", target.got, "hello")
		}
	})

	t.Run("time.Duration parse error", func(t *testing.T) {
		var d time.Duration
		if err := setFieldValue(reflect.ValueOf(&d).Elem(), "not-a-duration"); err == nil {
			t.Error("setFieldValue(bad duration) error = nil, want error")
		}
	})

	t.Run("float parse error", func(t *testing.T) {
		var f float64
		if err := setFieldValue(reflect.ValueOf(&f).Elem(), "abc"); err == nil {
			t.Error("setFieldValue(bad float) error = nil, want error")
		}
	})

	t.Run("slice passthrough", func(t *testing.T) {
		var s []string
		field := reflect.ValueOf(&s).Elem()
		if err := setFieldValue(field, "a,b"); err != nil {
			t.Fatalf("setFieldValue(slice) error = %v", err)
		}
		if field.Len() != 2 || field.Index(0).String() != "a" || field.Index(1).String() != "b" {
			t.Errorf("slice = %v, want [a b]", s)
		}
	})

	t.Run("float slice parse error", func(t *testing.T) {
		var fs []float64
		if err := setFieldValue(reflect.ValueOf(&fs).Elem(), "1.5,x"); err == nil {
			t.Error("setFieldValue(bad float slice) error = nil, want error")
		}
	})
}

// TestBuildPrefixedKey_LongKeys covers the fallback concatenation branch
// taken when prefix+key exceeds the 64-byte stack buffer.
func TestBuildPrefixedKey_LongKeys(t *testing.T) {
	prefix := strings.Repeat("P", 40)
	key := strings.Repeat("K", 40)
	if got := buildPrefixedKey(prefix, key); got != prefix+"_"+key {
		t.Errorf("buildPrefixedKey(long) = %q, want %q_%q", got, prefix, key)
	}
}
