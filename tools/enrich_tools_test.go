package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

type enrichMockStore struct {
	mockStore
}

func TestEnrichServiceInvalidScheme(t *testing.T) {
	t.Parallel()
	ms := &enrichMockStore{}
	reg := NewRegistry()
	RegisterEnrichTools(reg, ms, 1)

	tool, ok := reg.Lookup("enrich_service")
	if !ok {
		t.Fatal("enrich_service not found")
	}

	r, err := tool.Run(context.Background(), json.RawMessage(`{"host":"example.com","port":80,"scheme":"ftp"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for invalid scheme")
	}
	if !strings.Contains(r.Content, "scheme must be http or https") {
		t.Errorf("expected scheme validation error, got: %s", r.Content)
	}
}

func TestEnrichServiceEmptySchemeAllowed(t *testing.T) {
	t.Parallel()
	ms := &enrichMockStore{}
	reg := NewRegistry()
	RegisterEnrichTools(reg, ms, 1)

	tool, _ := reg.Lookup("enrich_service")
	r, err := tool.Run(context.Background(), json.RawMessage(`{"host":"127.0.0.1","port":99999}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for out-of-range port")
	}
}

func TestEnrichServiceMalformedHost(t *testing.T) {
	t.Parallel()
	ms := &enrichMockStore{}
	reg := NewRegistry()
	RegisterEnrichTools(reg, ms, 1)

	tool, _ := reg.Lookup("enrich_service")
	r, err := tool.Run(context.Background(), json.RawMessage(`{"host":"","port":80}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for empty host")
	}
	if !strings.Contains(r.Content, "host is required") {
		t.Errorf("expected 'host is required', got: %s", r.Content)
	}
}

func TestEnrichServiceValidSchemes(t *testing.T) {
	t.Parallel()
	ms := &enrichMockStore{}

	for _, scheme := range []string{"http", "https", "HTTP", "HTTPS"} {
		t.Run(scheme, func(t *testing.T) {
			reg := NewRegistry()
			RegisterEnrichTools(reg, ms, 1)
			tool, _ := reg.Lookup("enrich_service")

			input := `{"host":"127.0.0.1","port":80,"scheme":"` + scheme + `"}`
			r, err := tool.Run(context.Background(), json.RawMessage(input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if r.IsError && strings.Contains(r.Content, "scheme must be") {
				t.Errorf("valid scheme %q rejected: %s", scheme, r.Content)
			}
		})
	}
}

func TestEnrichServiceBadSchemes(t *testing.T) {
	t.Parallel()
	ms := &enrichMockStore{}

	for _, scheme := range []string{"ftp", "gopher", "file", "javascript"} {
		t.Run(scheme, func(t *testing.T) {
			reg := NewRegistry()
			RegisterEnrichTools(reg, ms, 1)
			tool, _ := reg.Lookup("enrich_service")

			input := `{"host":"example.com","port":443,"scheme":"` + scheme + `"}`
			r, err := tool.Run(context.Background(), json.RawMessage(input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !r.IsError {
				t.Errorf("expected error for scheme %q", scheme)
			}
			if !strings.Contains(r.Content, "scheme must be http or https") {
				t.Errorf("expected scheme validation message, got: %s", r.Content)
			}
		})
	}
}
