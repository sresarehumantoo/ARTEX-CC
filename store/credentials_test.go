package store

import (
	"context"
	"testing"

	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
)

func TestSaveCredentialValidation(t *testing.T) {
	t.Parallel()
	s := &Store{}
	ctx := context.Background()

	t.Run("empty domain", func(t *testing.T) {
		_, err := s.SaveCredential(ctx, 1, tools.CredentialInput{
			Username: "admin", Password: "pass",
		})
		if err != errCredentialInvalid {
			t.Errorf("want errCredentialInvalid, got %v", err)
		}
	})

	t.Run("empty username", func(t *testing.T) {
		_, err := s.SaveCredential(ctx, 1, tools.CredentialInput{
			Domain: "example.com", Password: "pass",
		})
		if err != errCredentialInvalid {
			t.Errorf("want errCredentialInvalid, got %v", err)
		}
	})

	t.Run("whitespace only domain", func(t *testing.T) {
		_, err := s.SaveCredential(ctx, 1, tools.CredentialInput{
			Domain: "   ", Username: "admin", Password: "pass",
		})
		if err != errCredentialInvalid {
			t.Errorf("want errCredentialInvalid, got %v", err)
		}
	})

	t.Run("whitespace only username", func(t *testing.T) {
		_, err := s.SaveCredential(ctx, 1, tools.CredentialInput{
			Domain: "example.com", Username: "  ", Password: "pass",
		})
		if err != errCredentialInvalid {
			t.Errorf("want errCredentialInvalid, got %v", err)
		}
	})

	t.Run("empty password with agent source", func(t *testing.T) {
		_, err := s.SaveCredential(ctx, 1, tools.CredentialInput{
			Domain: "example.com", Username: "admin", Source: "agent",
		})
		if err == nil {
			t.Error("expected error for empty password with agent source")
		}
	})

	t.Run("empty password with default source", func(t *testing.T) {
		_, err := s.SaveCredential(ctx, 1, tools.CredentialInput{
			Domain: "example.com", Username: "admin",
		})
		if err == nil {
			t.Error("expected error for empty password with default agent source")
		}
	})
}

func TestAssetURLCredential(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in                 string
		domain, user, pass string
		ok                 bool
	}{
		{"https://admin:s3cret@Example.com/login", "example.com", "admin", "s3cret", true},
		{"https://ad%40min:p%3Ass%2F@example.com:8443/", "example.com", "ad@min", "p:ss/", true},
		{"http://admin@[2001:db8::1]:80/x", "2001:db8::1", "admin", "", true},
		{"http://u:p@host/%zz", "host", "u", "p", true},
		{"http://u:p@host?q=a@b", "host", "u", "p", true},
		{"https://:pass@example.com/", "example.com", "", "pass", false},
		{"https://user:pa/ss@Example.com/", "example.com", "user", "pa/ss", true},
		{"https://user:p#ss@example.com/", "example.com", "user", "p#ss", true},
		{"http://u:p@host:abc/", "host", "u", "p", true},
		{"//u:p@host/x", "host", "u", "p", true},
		{"https://u:p@BÜCHER.de/", "bücher.de", "u", "p", true},
		{"http://u:100%@host/", "host", "u", "100%", true},
		{"http://host:8080/path@x", "", "", "", false},
		{"https://host/users/@me", "", "", "", false},
		{"https://example.com/a@b", "", "", "", false},
		{"http://u:p@exa mple.com/a", "", "", "", false},
		{"example.com/path", "", "", "", false},
		{"", "", "", "", false},
	}
	for _, tt := range tests {
		domain, user, pass, ok := assetURLCredential(tt.in)
		if ok != tt.ok || ok && (domain != tt.domain || user != tt.user || pass != tt.pass) {
			t.Errorf("assetURLCredential(%q) = %q, %q, %q, %v, want %q, %q, %q, %v",
				tt.in, domain, user, pass, ok, tt.domain, tt.user, tt.pass, tt.ok)
		}
	}
}
