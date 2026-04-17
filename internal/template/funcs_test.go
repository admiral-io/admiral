package template

import (
	"strings"
	"testing"
)

func TestToJSON(t *testing.T) {
	tests := []struct {
		input any
		want  string
	}{
		{"hello", `"hello"`},
		{42, `42`},
		{true, `true`},
		{[]string{"a", "b"}, `["a","b"]`},
		{map[string]any{"k": "v"}, `{"k":"v"}`},
		{nil, `null`},
	}
	for _, tt := range tests {
		got, err := toJSON(tt.input)
		if err != nil {
			t.Errorf("toJSON(%v) error: %v", tt.input, err)
			continue
		}
		if got != tt.want {
			t.Errorf("toJSON(%v) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestFromJSON(t *testing.T) {
	got, err := fromJSON(`{"a": 1}`)
	if err != nil {
		t.Fatalf("fromJSON error: %v", err)
	}
	m, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", got)
	}
	if m["a"] != float64(1) {
		t.Fatalf("expected a=1, got %v", m["a"])
	}
}

func TestFromJSON_Invalid(t *testing.T) {
	_, err := fromJSON(`not json`)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestDefault(t *testing.T) {
	tests := []struct {
		fallback any
		val      any
		want     any
	}{
		{"fb", "actual", "actual"},
		{"fb", "", "fb"},
		{"fb", nil, "fb"},
		{42, 0, 42},
		{"fb", false, "fb"},
	}
	for _, tt := range tests {
		got := dflt(tt.fallback, tt.val)
		if got != tt.want {
			t.Errorf("dflt(%v, %v) = %v, want %v", tt.fallback, tt.val, got, tt.want)
		}
	}
}

func TestRequired(t *testing.T) {
	got, err := required("msg", "value")
	if err != nil {
		t.Fatalf("required with value: %v", err)
	}
	if got != "value" {
		t.Fatalf("expected value, got %v", got)
	}

	_, err = required("field is required", "")
	if err == nil {
		t.Fatal("expected error for empty value")
	}
	if !strings.Contains(err.Error(), "field is required") {
		t.Fatalf("wrong error: %v", err)
	}
}

func TestURLHelpers(t *testing.T) {
	raw := "postgres://db.example.com:5432/mydb?sslmode=require"

	scheme, err := urlScheme(raw)
	if err != nil {
		t.Fatal(err)
	}
	if scheme != "postgres" {
		t.Errorf("scheme = %q, want postgres", scheme)
	}

	host, err := urlHost(raw)
	if err != nil {
		t.Fatal(err)
	}
	if host != "db.example.com" {
		t.Errorf("host = %q, want db.example.com", host)
	}

	port, err := urlPort(raw)
	if err != nil {
		t.Fatal(err)
	}
	if port != "5432" {
		t.Errorf("port = %q, want 5432", port)
	}

	path, err := urlPath(raw)
	if err != nil {
		t.Fatal(err)
	}
	if path != "/mydb" {
		t.Errorf("path = %q, want /mydb", path)
	}
}

func TestJoin(t *testing.T) {
	// []string
	got, err := join(",", []string{"a", "b", "c"})
	if err != nil {
		t.Fatal(err)
	}
	if got != "a,b,c" {
		t.Errorf("join string = %q", got)
	}

	// []any (from JSON unmarshalling)
	got, err = join("-", []any{"x", "y", float64(1)})
	if err != nil {
		t.Fatal(err)
	}
	if got != "x-y-1" {
		t.Errorf("join any = %q", got)
	}

	// Wrong type
	_, err = join(",", "not a slice")
	if err == nil {
		t.Fatal("expected error for non-slice")
	}
}

func TestSafeIndex(t *testing.T) {
	m := map[string]any{
		"nested": map[string]any{
			"deep": "value",
		},
	}

	got := safeIndex(m, "nested", "deep")
	if got != "value" {
		t.Errorf("safeIndex nested.deep = %v, want value", got)
	}

	got = safeIndex(m, "missing")
	if got != nil {
		t.Errorf("safeIndex missing = %v, want nil", got)
	}

	got = safeIndex(m, "nested", "missing")
	if got != nil {
		t.Errorf("safeIndex nested.missing = %v, want nil", got)
	}

	got = safeIndex("not a map", "key")
	if got != nil {
		t.Errorf("safeIndex non-map = %v, want nil", got)
	}
}

func TestReplace(t *testing.T) {
	got := replace("world", "Go", "hello world")
	if got != "hello Go" {
		t.Errorf("replace = %q, want 'hello Go'", got)
	}
}

func TestIsEmpty(t *testing.T) {
	tests := []struct {
		val  any
		want bool
	}{
		{nil, true},
		{"", true},
		{"x", false},
		{0, true},
		{1, false},
		{false, true},
		{true, false},
		{[]any{}, true},
		{[]any{"a"}, false},
		{map[string]any{}, true},
		{map[string]any{"k": "v"}, false},
	}
	for _, tt := range tests {
		got := isEmpty(tt.val)
		if got != tt.want {
			t.Errorf("isEmpty(%v) = %v, want %v", tt.val, got, tt.want)
		}
	}
}
