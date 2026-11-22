package store

import (
	"context"
	"testing"

	"github.com/SREsAreHumanToo/cc-artex/tools"
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
