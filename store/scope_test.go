package store

import (
	"context"
	"net"
	"strings"
	"testing"
)

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

func TestScopeCIDRContainment(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		cidr    string
		ip      string
		matches bool
	}{
		{"inside /24", "10.0.0.0/24", "10.0.0.5", true},
		{"first address", "10.0.0.0/24", "10.0.0.0", true},
		{"last address", "10.0.0.0/24", "10.0.0.255", true},
		{"outside /24", "10.0.0.0/24", "10.0.1.1", false},
		{"wide /16", "172.16.0.0/16", "172.16.255.254", true},
		{"outside /16", "172.16.0.0/16", "172.17.0.1", false},
		{"single host /32", "10.10.10.1/32", "10.10.10.1", true},
		{"miss /32", "10.10.10.1/32", "10.10.10.2", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := parseScopeCheckRule("cidr", "", tt.cidr)
			if r == nil {
				t.Fatal("expected non-nil rule")
			}
			ip := net.ParseIP(tt.ip)
			if ip == nil {
				t.Fatalf("bad test IP: %s", tt.ip)
			}
			got := r.net.Contains(ip)
			if got != tt.matches {
				t.Errorf("CIDR %s contains %s = %v, want %v", tt.cidr, tt.ip, got, tt.matches)
			}
		})
	}
}

func TestScopeDomainSuffixMatching(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		target  string
		rule    string
		matches bool
	}{
		{"exact root", "example.com", "example.com", true},
		{"subdomain of root", "sub.example.com", "example.com", true},
		{"deep subdomain", "a.b.c.example.com", "example.com", true},
		{"different domain", "other.com", "example.com", false},
		{"partial suffix", "notexample.com", "example.com", false},
		{"rule is subdomain", "example.com", "sub.example.com", false},
		{"exact subdomain match", "app.example.com", "app.example.com", true},
		{"sub of subdomain", "dev.app.example.com", "app.example.com", true},
		{"wildcard excludes apex", "example.com", "*.example.com", false},
		{"wildcard subdomain", "api.example.com", "*.example.com", true},
		{"wildcard deep subdomain", "a.b.example.com", "*.example.com", true},
		{"wildcard lookalike", "badexample.com", "*.example.com", false},
		{"wildcard lookalike subdomain", "api.badexample.com", "*.example.com", false},
		{"wildcard different domain", "example.org", "*.example.com", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesDomain(tt.target, tt.rule)
			if got != tt.matches {
				t.Errorf("matchesDomain(%q, %q) = %v, want %v", tt.target, tt.rule, got, tt.matches)
			}
		})
	}
}

func TestScopeURLTargetExtraction(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		target string
		want   string
	}{
		{"https with path", "https://example.com/path/to/resource", "example.com"},
		{"http with port and path", "http://example.com:8080/api/v1", "example.com"},
		{"https with query", "https://example.com/search?q=test&page=1", "example.com"},
		{"IP URL", "https://10.10.10.1:443/admin", "10.10.10.1"},
		{"bare domain", "example.com", "example.com"},
		{"bare IP", "192.168.1.1", "192.168.1.1"},
		{"host:port", "example.com:443", "example.com"},
		{"ipv6 URL", "https://[2001:db8::1]:8443/", "2001:db8::1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractScopeTarget(tt.target)
			if got != tt.want {
				t.Errorf("extractScopeTarget(%q) = %q, want %q", tt.target, got, tt.want)
			}
		})
	}
}

func TestScopeMixedRuleMatching(t *testing.T) {
	t.Parallel()

	rules := []scopeCheckRule{}

	domainRule := parseScopeCheckRule("root_domain", "example.com", "")
	if domainRule != nil {
		rules = append(rules, *domainRule)
	}
	cidrRule := parseScopeCheckRule("cidr", "", "10.0.0.0/8")
	if cidrRule != nil {
		rules = append(rules, *cidrRule)
	}
	subRule := parseScopeCheckRule("subdomain", "specific.other.com", "")
	if subRule != nil {
		rules = append(rules, *subRule)
	}

	tests := []struct {
		name    string
		target  string
		matches bool
	}{
		{"domain in scope", "sub.example.com", true},
		{"exact domain", "example.com", true},
		{"IP in CIDR", "10.10.10.5", true},
		{"IP outside CIDR", "192.168.1.1", false},
		{"specific subdomain", "specific.other.com", true},
		{"other subdomain not in scope", "admin.other.com", false},
		{"unrelated domain", "evil.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host := extractScopeTarget(tt.target)
			ip := net.ParseIP(host)
			matched := false
			for _, r := range rules {
				if ip != nil && r.net != nil {
					if r.net.Contains(ip) {
						matched = true
						break
					}
					continue
				}
				if ip == nil && r.domain != "" {
					if matchesDomain(host, r.domain) {
						matched = true
						break
					}
				}
			}
			if matched != tt.matches {
				t.Errorf("target %q: matched = %v, want %v", tt.target, matched, tt.matches)
			}
		})
	}
}

func TestScopeEmptyRulesPermissive(t *testing.T) {
	t.Parallel()
	var rules []scopeCheckRule

	targets := []string{"anything.com", "10.10.10.1", "https://evil.com/hack"}
	for _, target := range targets {
		host := extractScopeTarget(target)
		if host == "" {
			t.Errorf("extractScopeTarget(%q) returned empty", target)
			continue
		}
		if len(rules) != 0 {
			t.Fatal("rules should be empty for this test")
		}
	}
}

func TestScopeIPSingleHostCIDR(t *testing.T) {
	t.Parallel()

	r := parseScopeCheckRule("ip", "", "10.10.10.42/32")
	if r == nil {
		t.Fatal("expected non-nil rule for /32 CIDR")
	}
	exact := net.ParseIP("10.10.10.42")
	neighbor := net.ParseIP("10.10.10.43")
	if !r.net.Contains(exact) {
		t.Error("/32 should contain the exact IP")
	}
	if r.net.Contains(neighbor) {
		t.Error("/32 should not contain a neighboring IP")
	}
}

func TestScopeInvalidCIDR(t *testing.T) {
	t.Parallel()
	r := parseScopeCheckRule("cidr", "", "not-a-cidr")
	if r != nil {
		t.Error("expected nil rule for invalid CIDR")
	}
}

func TestScopeDomainKindAlias(t *testing.T) {
	t.Parallel()
	r := parseScopeCheckRule("domain", "example.com", "")
	if r == nil {
		t.Fatal("expected non-nil rule for 'domain' kind")
	}
	if r.domain != "example.com" {
		t.Errorf("domain = %q, want %q", r.domain, "example.com")
	}
}

func TestAddTaskScopeValidation(t *testing.T) {
	t.Parallel()
	s := &Store{}
	ctx := context.Background()

	t.Run("zero task_id", func(t *testing.T) {
		err := s.AddTaskScope(ctx, 0, "keyword", "test", "", "")
		if err == nil || !strings.Contains(err.Error(), "task_id") {
			t.Errorf("want task_id error, got %v", err)
		}
	})

	t.Run("empty value", func(t *testing.T) {
		err := s.AddTaskScope(ctx, 1, "keyword", "", "", "")
		if err == nil || !strings.Contains(err.Error(), "value must not be empty") {
			t.Errorf("want empty value error, got %v", err)
		}
	})

	t.Run("unknown kind", func(t *testing.T) {
		err := s.AddTaskScope(ctx, 1, "bogus", "test", "", "")
		if err == nil || !strings.Contains(err.Error(), "unknown scope kind") {
			t.Errorf("want unknown kind error, got %v", err)
		}
	})

	t.Run("invalid IP", func(t *testing.T) {
		err := s.AddTaskScope(ctx, 1, "ip", "not-an-ip", "", "")
		if err == nil || !strings.Contains(err.Error(), "invalid IP") {
			t.Errorf("want invalid IP error, got %v", err)
		}
	})

	t.Run("invalid CIDR", func(t *testing.T) {
		err := s.AddTaskScope(ctx, 1, "cidr", "not-a-cidr", "", "")
		if err == nil || !strings.Contains(err.Error(), "invalid CIDR") {
			t.Errorf("want invalid CIDR error, got %v", err)
		}
	})

	t.Run("invalid subdomain dot only", func(t *testing.T) {
		err := s.AddTaskScope(ctx, 1, "subdomain", ".", "", "")
		if err == nil || !strings.Contains(err.Error(), "invalid subdomain") {
			t.Errorf("want invalid subdomain error, got %v", err)
		}
	})

	t.Run("wildcard root domain", func(t *testing.T) {
		err := s.AddTaskScope(ctx, 1, "root_domain", "*.example.com", "", "")
		if err == nil || !strings.Contains(err.Error(), "invalid root domain") {
			t.Errorf("want invalid root domain error, got %v", err)
		}
	})

	t.Run("invalid wildcard subdomain", func(t *testing.T) {
		for _, v := range []string{"*", "*.", "api.*.example.com", "*.*.example.com"} {
			err := s.AddTaskScope(ctx, 1, "subdomain", v, "", "")
			if err == nil || !strings.Contains(err.Error(), "invalid subdomain") {
				t.Errorf("%q: want invalid subdomain error, got %v", v, err)
			}
		}
	})

	t.Run("default source is agent", func(t *testing.T) {
		err := s.AddTaskScope(ctx, 1, "ip", "not-an-ip", "", "")
		if err == nil || !strings.Contains(err.Error(), "invalid IP") {
			t.Errorf("want invalid IP error, got %v", err)
		}
	})
}
