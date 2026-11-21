package store

import "testing"

func TestMatchesDomain(t *testing.T) {
	t.Parallel()
	tests := []struct {
		target string
		rule   string
		want   bool
	}{
		{"example.com", "example.com", true},
		{"sub.example.com", "example.com", true},
		{"deep.sub.example.com", "example.com", true},
		{"other.com", "example.com", false},
		{"notexample.com", "example.com", false},
		{"EXAMPLE.COM", "example.com", true},
		{"sub.EXAMPLE.COM", "Example.Com", true},
		{"example.com", "sub.example.com", false},
	}
	for _, tt := range tests {
		got := matchesDomain(tt.target, tt.rule)
		if got != tt.want {
			t.Errorf("matchesDomain(%q, %q) = %v, want %v", tt.target, tt.rule, got, tt.want)
		}
	}
}

func TestExtractScopeTarget(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  string
	}{
		{"example.com", "example.com"},
		{"EXAMPLE.COM", "example.com"},
		{"https://example.com/path?q=1", "example.com"},
		{"http://10.10.10.1:8080/api", "10.10.10.1"},
		{"10.10.10.1", "10.10.10.1"},
		{"10.10.10.1:443", "10.10.10.1"},
		{"https://[::1]:8443/", "::1"},
		{"", ""},
		{"  ", ""},
	}
	for _, tt := range tests {
		got := extractScopeTarget(tt.input)
		if got != tt.want {
			t.Errorf("extractScopeTarget(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestIPToHostCIDR(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  string
	}{
		{"10.10.10.1", "10.10.10.1/32"},
		{"192.168.1.1", "192.168.1.1/32"},
		{"::1", "::1/128"},
		{"2001:db8::1", "2001:db8::1/128"},
		{"not-an-ip", ""},
		{"", ""},
	}
	for _, tt := range tests {
		got := ipToHostCIDR(tt.input)
		if got != tt.want {
			t.Errorf("ipToHostCIDR(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestStripHostPort(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  string
	}{
		{"example.com:443", "example.com"},
		{"10.10.10.1:8080", "10.10.10.1"},
		{"example.com", "example.com"},
		{"[::1]:443", "::1"},
		{"  example.com  ", "example.com"},
	}
	for _, tt := range tests {
		got := stripHostPort(tt.input)
		if got != tt.want {
			t.Errorf("stripHostPort(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestParseScopeCheckRule(t *testing.T) {
	t.Parallel()

	t.Run("root_domain", func(t *testing.T) {
		r := parseScopeCheckRule("root_domain", "example.com", "")
		if r == nil {
			t.Fatal("expected non-nil rule")
		}
		if r.domain != "example.com" {
			t.Errorf("domain = %q, want %q", r.domain, "example.com")
		}
		if r.net != nil {
			t.Error("expected nil net for domain rule")
		}
	})

	t.Run("subdomain", func(t *testing.T) {
		r := parseScopeCheckRule("subdomain", "app.example.com", "")
		if r == nil {
			t.Fatal("expected non-nil rule")
		}
		if r.domain != "app.example.com" {
			t.Errorf("domain = %q, want %q", r.domain, "app.example.com")
		}
	})

	t.Run("ip_cidr", func(t *testing.T) {
		r := parseScopeCheckRule("ip", "", "10.10.10.0/24")
		if r == nil {
			t.Fatal("expected non-nil rule")
		}
		if r.net == nil {
			t.Fatal("expected non-nil net")
		}
		if !r.net.Contains([]byte{10, 10, 10, 1}) {
			t.Error("expected CIDR to contain 10.10.10.1")
		}
	})

	t.Run("cidr", func(t *testing.T) {
		r := parseScopeCheckRule("cidr", "", "192.168.0.0/16")
		if r == nil {
			t.Fatal("expected non-nil rule")
		}
	})

	t.Run("empty_domain", func(t *testing.T) {
		r := parseScopeCheckRule("root_domain", "", "")
		if r != nil {
			t.Error("expected nil rule for empty domain")
		}
	})

	t.Run("unknown_kind", func(t *testing.T) {
		r := parseScopeCheckRule("keyword", "", "")
		if r != nil {
			t.Error("expected nil rule for keyword kind")
		}
	})
}
