package store

import (
	"testing"
)

func TestCompanyNKey(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  string
	}{
		{"Example Corp", "example corp"},
		{"  Example   Corp  ", "example corp"},
		{"ACME", "acme"},
		{"", ""},
		{"  ", ""},
		{"Some  Multi   Space   Name", "some multi space name"},
	}
	for _, tt := range tests {
		got := companyNKey(tt.input)
		if got != tt.want {
			t.Errorf("companyNKey(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestClassifyScopeLine(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  string
	}{
		{"example.com", "domain"},
		{"sub.example.com", "domain"},
		{"10.10.10.1", "ip"},
		{"192.168.0.0/16", "cidr"},
		{"10.0.0.0/8", "cidr"},
		{"some keyword", "keyword"},
		{"singleword", "keyword"},
		{"::1", "ip"},
	}
	for _, tt := range tests {
		got := classifyScopeLine(tt.input)
		if got != tt.want {
			t.Errorf("classifyScopeLine(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestParseScopeLine(t *testing.T) {
	t.Parallel()

	t.Run("domain", func(t *testing.T) {
		r, err := ParseScopeLine("domain", "Example.COM.")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if r.Domain != "example.com" {
			t.Errorf("Domain = %q, want %q", r.Domain, "example.com")
		}
		if r.Kind != "domain" {
			t.Errorf("Kind = %q, want %q", r.Kind, "domain")
		}
	})

	t.Run("domain wildcard", func(t *testing.T) {
		r, err := ParseScopeLine("domain", "*.Example.com")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if r.Domain != "*.example.com" {
			t.Errorf("Domain = %q, want %q", r.Domain, "*.example.com")
		}
		for _, host := range []string{"api.example.com", "a.b.example.com"} {
			if !matchesDomain(host, r.Domain) {
				t.Errorf("matchesDomain(%q, %q) = false, want true", host, r.Domain)
			}
		}
		for _, host := range []string{"example.com", "notexample.com"} {
			if matchesDomain(host, r.Domain) {
				t.Errorf("matchesDomain(%q, %q) = true, want false", host, r.Domain)
			}
		}
	})

	t.Run("domain wildcard invalid", func(t *testing.T) {
		for _, v := range []string{"*.", "*..", "*", "*.*.example.com", "api.*.example.com", "*example.com"} {
			if _, err := ParseScopeLine("domain", v); err == nil {
				t.Errorf("ParseScopeLine(%q): expected error", v)
			}
		}
	})

	t.Run("domain invalid", func(t *testing.T) {
		_, err := ParseScopeLine("domain", "has spaces")
		if err == nil {
			t.Error("expected error for invalid domain")
		}
	})

	t.Run("domain empty", func(t *testing.T) {
		_, err := ParseScopeLine("domain", "")
		if err == nil {
			t.Error("expected error for empty value")
		}
	})

	t.Run("ip v4", func(t *testing.T) {
		r, err := ParseScopeLine("ip", "10.10.10.1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if r.Net != "10.10.10.1/32" {
			t.Errorf("Net = %q, want %q", r.Net, "10.10.10.1/32")
		}
	})

	t.Run("ip v6", func(t *testing.T) {
		r, err := ParseScopeLine("ip", "::1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if r.Net != "::1/128" {
			t.Errorf("Net = %q, want %q", r.Net, "::1/128")
		}
	})

	t.Run("ip invalid", func(t *testing.T) {
		_, err := ParseScopeLine("ip", "not-an-ip")
		if err == nil {
			t.Error("expected error for invalid IP")
		}
	})

	t.Run("cidr", func(t *testing.T) {
		r, err := ParseScopeLine("cidr", "10.0.0.0/8")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if r.Net != "10.0.0.0/8" {
			t.Errorf("Net = %q, want %q", r.Net, "10.0.0.0/8")
		}
	})

	t.Run("cidr invalid", func(t *testing.T) {
		_, err := ParseScopeLine("cidr", "not-a-cidr")
		if err == nil {
			t.Error("expected error for invalid CIDR")
		}
	})

	t.Run("keyword", func(t *testing.T) {
		r, err := ParseScopeLine("keyword", "some value")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if r.Value != "some value" {
			t.Errorf("Value = %q, want %q", r.Value, "some value")
		}
	})

	t.Run("icp", func(t *testing.T) {
		r, err := ParseScopeLine("icp", "icp-123")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if r.Value != "icp-123" {
			t.Errorf("Value = %q, want %q", r.Value, "icp-123")
		}
	})

	t.Run("unsupported kind", func(t *testing.T) {
		_, err := ParseScopeLine("unknown", "value")
		if err == nil {
			t.Error("expected error for unsupported kind")
		}
	})
}
