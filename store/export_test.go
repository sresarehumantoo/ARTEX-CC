package store

import (
	"encoding/json"
	"testing"
)

func TestExportMapString(t *testing.T) {
	t.Parallel()
	m := map[string]any{"name": "test", "count": 42}

	if got := exportMapString(m, "name"); got != "test" {
		t.Errorf("exportMapString(name) = %q, want %q", got, "test")
	}
	if got := exportMapString(m, "missing"); got != "" {
		t.Errorf("exportMapString(missing) = %q, want %q", got, "")
	}
	if got := exportMapString(m, "count"); got != "" {
		t.Errorf("exportMapString(count) = %q, want %q (non-string value)", got, "")
	}
}

func TestExportMapFloat64(t *testing.T) {
	t.Parallel()
	m := map[string]any{
		"float":   42.5,
		"int":     10,
		"int64":   int64(20),
		"jnum":    json.Number("30.5"),
		"str":     "not-a-number",
		"missing": nil,
	}

	if got := exportMapFloat64(m, "float"); got != 42.5 {
		t.Errorf("float = %v, want 42.5", got)
	}
	if got := exportMapFloat64(m, "int"); got != 10 {
		t.Errorf("int = %v, want 10", got)
	}
	if got := exportMapFloat64(m, "int64"); got != 20 {
		t.Errorf("int64 = %v, want 20", got)
	}
	if got := exportMapFloat64(m, "jnum"); got != 30.5 {
		t.Errorf("jnum = %v, want 30.5", got)
	}
	if got := exportMapFloat64(m, "str"); got != 0 {
		t.Errorf("str = %v, want 0", got)
	}
	if got := exportMapFloat64(m, "nonexistent"); got != 0 {
		t.Errorf("nonexistent = %v, want 0", got)
	}
}

func TestExportMapInt64(t *testing.T) {
	t.Parallel()
	m := map[string]any{"id": float64(42)}
	if got := exportMapInt64(m, "id"); got != 42 {
		t.Errorf("exportMapInt64(id) = %d, want 42", got)
	}
	if got := exportMapInt64(m, "missing"); got != 0 {
		t.Errorf("exportMapInt64(missing) = %d, want 0", got)
	}
}

func TestExportToInt64(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input any
		want  int64
	}{
		{"float64", float64(42), 42},
		{"int", int(10), 10},
		{"int64", int64(20), 20},
		{"json.Number", json.Number("30"), 30},
		{"string", "not-a-number", 0},
		{"nil", nil, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := exportToInt64(tt.input)
			if got != tt.want {
				t.Errorf("exportToInt64(%v) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestExportToInt64Slice(t *testing.T) {
	t.Parallel()

	t.Run("[]any", func(t *testing.T) {
		input := []any{float64(1), float64(2), float64(3)}
		got := exportToInt64Slice(input)
		want := []int64{1, 2, 3}
		if !int64SliceEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("[]any with zero", func(t *testing.T) {
		input := []any{float64(1), float64(0), float64(3)}
		got := exportToInt64Slice(input)
		want := []int64{1, 3}
		if !int64SliceEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("[]int64", func(t *testing.T) {
		input := []int64{10, 20}
		got := exportToInt64Slice(input)
		if !int64SliceEqual(got, input) {
			t.Errorf("got %v, want %v", got, input)
		}
	})

	t.Run("[]float64", func(t *testing.T) {
		input := []float64{1.0, 2.0, 3.0}
		got := exportToInt64Slice(input)
		want := []int64{1, 2, 3}
		if !int64SliceEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("unsupported type", func(t *testing.T) {
		got := exportToInt64Slice("not a slice")
		if got != nil {
			t.Errorf("got %v, want nil", got)
		}
	})

	t.Run("nil", func(t *testing.T) {
		got := exportToInt64Slice(nil)
		if got != nil {
			t.Errorf("got %v, want nil", got)
		}
	})
}

func TestExportToStringSlice(t *testing.T) {
	t.Parallel()

	t.Run("[]any", func(t *testing.T) {
		input := []any{"a", "b", "c"}
		got := exportToStringSlice(input)
		want := []string{"a", "b", "c"}
		if !sliceEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("[]any with non-string", func(t *testing.T) {
		input := []any{"a", 42, "c"}
		got := exportToStringSlice(input)
		want := []string{"a", "c"}
		if !sliceEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("[]string", func(t *testing.T) {
		input := []string{"x", "y"}
		got := exportToStringSlice(input)
		if !sliceEqual(got, input) {
			t.Errorf("got %v, want %v", got, input)
		}
	})

	t.Run("unsupported type", func(t *testing.T) {
		got := exportToStringSlice(42)
		if got != nil {
			t.Errorf("got %v, want nil", got)
		}
	})
}

func TestExportMapToAssetInput(t *testing.T) {
	t.Parallel()
	m := map[string]any{
		"type":         "service",
		"domain":       "example.com",
		"ip":           "10.10.10.1",
		"url":          "https://example.com",
		"method":       "GET",
		"bundle_id":    "com.example",
		"app_name":     "Example",
		"service_name": "web",
		"service_type": "http",
		"port":         float64(443),
		"technologies": []any{"nginx", "php"},
		"extra":        map[string]any{"custom": "val"},
	}

	input := exportMapToAssetInput(m)
	if input.Type != "service" {
		t.Errorf("Type = %q, want %q", input.Type, "service")
	}
	if input.Domain != "example.com" {
		t.Errorf("Domain = %q, want %q", input.Domain, "example.com")
	}
	if input.IP != "10.10.10.1" {
		t.Errorf("IP = %q, want %q", input.IP, "10.10.10.1")
	}
	if input.URL != "https://example.com" {
		t.Errorf("URL = %q, want %q", input.URL, "https://example.com")
	}
	if input.Method != "GET" {
		t.Errorf("Method = %q, want %q", input.Method, "GET")
	}
	if input.BundleID != "com.example" {
		t.Errorf("BundleID = %q, want %q", input.BundleID, "com.example")
	}
	if input.AppName != "Example" {
		t.Errorf("AppName = %q, want %q", input.AppName, "Example")
	}
	if input.ServiceName != "web" {
		t.Errorf("ServiceName = %q, want %q", input.ServiceName, "web")
	}
	if input.ServiceType != "http" {
		t.Errorf("ServiceType = %q, want %q", input.ServiceType, "http")
	}
	if input.Port == nil || *input.Port != 443 {
		t.Errorf("Port = %v, want 443", input.Port)
	}
	if len(input.Technologies) != 2 || input.Technologies[0] != "nginx" || input.Technologies[1] != "php" {
		t.Errorf("Technologies = %v, want [nginx, php]", input.Technologies)
	}
	if input.Extra == nil {
		t.Error("Extra should not be nil")
	}
}

func TestExportMapToAssetInputMinimal(t *testing.T) {
	t.Parallel()
	m := map[string]any{"type": "ip", "ip": "1.2.3.4"}
	input := exportMapToAssetInput(m)
	if input.Type != "ip" {
		t.Errorf("Type = %q, want %q", input.Type, "ip")
	}
	if input.IP != "1.2.3.4" {
		t.Errorf("IP = %q, want %q", input.IP, "1.2.3.4")
	}
	if input.Port != nil {
		t.Errorf("Port = %v, want nil", input.Port)
	}
	if input.Extra != nil {
		t.Errorf("Extra = %v, want nil", input.Extra)
	}
}
