package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestTrafficSearchRegistered(t *testing.T) {
	t.Parallel()
	reg := NewRegistry()
	RegisterTrafficTools(reg, nil, &mockStore{}, 1)

	tool, ok := reg.Lookup("traffic_search")
	if !ok {
		t.Fatal("traffic_search not found")
	}
	if tool.Name != "traffic_search" {
		t.Errorf("expected name traffic_search, got %s", tool.Name)
	}
	if !tool.ReadOnly {
		t.Error("expected traffic_search to be read-only")
	}
}

func TestTrafficSearchMissingHost(t *testing.T) {
	t.Parallel()
	reg := NewRegistry()
	RegisterTrafficTools(reg, nil, &mockStore{}, 1)

	tool, _ := reg.Lookup("traffic_search")
	r, err := tool.Run(context.Background(), json.RawMessage(`{"host": ""}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for missing host")
	}
	if !strings.Contains(r.Content, "host is required") {
		t.Errorf("expected host required, got: %s", r.Content)
	}
}

func TestTrafficSearchInvalidJSON(t *testing.T) {
	t.Parallel()
	reg := NewRegistry()
	RegisterTrafficTools(reg, nil, &mockStore{}, 1)

	tool, _ := reg.Lookup("traffic_search")
	r, err := tool.Run(context.Background(), json.RawMessage(`not json`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestTrafficGetMissingID(t *testing.T) {
	t.Parallel()
	reg := NewRegistry()
	RegisterTrafficTools(reg, nil, &mockStore{}, 1)

	tool, ok := reg.Lookup("traffic_get")
	if !ok {
		t.Fatal("traffic_get not found")
	}

	r, err := tool.Run(context.Background(), json.RawMessage(`{"id": ""}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for missing id")
	}
	if !strings.Contains(r.Content, "id is required") {
		t.Errorf("expected id required, got: %s", r.Content)
	}
}

func TestTrafficGetInvalidJSON(t *testing.T) {
	t.Parallel()
	reg := NewRegistry()
	RegisterTrafficTools(reg, nil, &mockStore{}, 1)

	tool, _ := reg.Lookup("traffic_get")
	r, err := tool.Run(context.Background(), json.RawMessage(`{bad`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestTrafficBlobMissingHash(t *testing.T) {
	t.Parallel()
	reg := NewRegistry()
	RegisterTrafficTools(reg, nil, &mockStore{}, 1)

	tool, ok := reg.Lookup("traffic_blob")
	if !ok {
		t.Fatal("traffic_blob not found")
	}

	r, err := tool.Run(context.Background(), json.RawMessage(`{"hash": ""}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for missing hash")
	}
	if !strings.Contains(r.Content, "hash is required") {
		t.Errorf("expected hash required, got: %s", r.Content)
	}
}

func TestTrafficBlobInvalidJSON(t *testing.T) {
	t.Parallel()
	reg := NewRegistry()
	RegisterTrafficTools(reg, nil, &mockStore{}, 1)

	tool, _ := reg.Lookup("traffic_blob")
	r, err := tool.Run(context.Background(), json.RawMessage(`not json`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestTrafficMessagesMissingID(t *testing.T) {
	t.Parallel()
	reg := NewRegistry()
	RegisterTrafficTools(reg, nil, &mockStore{}, 1)

	tool, ok := reg.Lookup("traffic_messages")
	if !ok {
		t.Fatal("traffic_messages not found")
	}

	r, err := tool.Run(context.Background(), json.RawMessage(`{"id": ""}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for missing id")
	}
	if !strings.Contains(r.Content, "id is required") {
		t.Errorf("expected id required, got: %s", r.Content)
	}
}

func TestTrafficMessagesInvalidJSON(t *testing.T) {
	t.Parallel()
	reg := NewRegistry()
	RegisterTrafficTools(reg, nil, &mockStore{}, 1)

	tool, _ := reg.Lookup("traffic_messages")
	r, err := tool.Run(context.Background(), json.RawMessage(`{bad`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestTrafficReplayMissingID(t *testing.T) {
	t.Parallel()
	reg := NewRegistry()
	RegisterTrafficTools(reg, nil, &mockStore{}, 1)

	tool, ok := reg.Lookup("traffic_replay")
	if !ok {
		t.Fatal("traffic_replay not found")
	}

	r, err := tool.Run(context.Background(), json.RawMessage(`{"id": ""}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for missing id")
	}
	if !strings.Contains(r.Content, "id is required") {
		t.Errorf("expected id required, got: %s", r.Content)
	}
}

func TestTrafficReplayInvalidJSON(t *testing.T) {
	t.Parallel()
	reg := NewRegistry()
	RegisterTrafficTools(reg, nil, &mockStore{}, 1)

	tool, _ := reg.Lookup("traffic_replay")
	r, err := tool.Run(context.Background(), json.RawMessage(`not json`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestTrafficHARExportInvalidJSON(t *testing.T) {
	t.Parallel()
	reg := NewRegistry()
	RegisterTrafficTools(reg, nil, &mockStore{}, 1)

	tool, ok := reg.Lookup("traffic_har_export")
	if !ok {
		t.Fatal("traffic_har_export not found")
	}

	r, err := tool.Run(context.Background(), json.RawMessage(`{bad`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestTrafficTimelineMissingHost(t *testing.T) {
	t.Parallel()
	reg := NewRegistry()
	RegisterTrafficTools(reg, nil, &mockStore{}, 1)

	tool, ok := reg.Lookup("traffic_timeline")
	if !ok {
		t.Fatal("traffic_timeline not found")
	}

	r, err := tool.Run(context.Background(), json.RawMessage(`{"host": ""}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for missing host")
	}
	if !strings.Contains(r.Content, "host is required") {
		t.Errorf("expected host required, got: %s", r.Content)
	}
}

func TestTrafficTimelineInvalidJSON(t *testing.T) {
	t.Parallel()
	reg := NewRegistry()
	RegisterTrafficTools(reg, nil, &mockStore{}, 1)

	tool, _ := reg.Lookup("traffic_timeline")
	r, err := tool.Run(context.Background(), json.RawMessage(`not json`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestTrafficToolsAllRegistered(t *testing.T) {
	t.Parallel()
	reg := NewRegistry()
	RegisterTrafficTools(reg, nil, &mockStore{}, 1)

	expected := []string{
		"traffic_search", "traffic_get", "traffic_blob",
		"traffic_messages", "traffic_replay", "traffic_hosts",
		"traffic_har_export", "traffic_stats", "traffic_timeline",
		"traffic_patterns", "traffic_clear",
	}
	for _, name := range expected {
		if _, ok := reg.Lookup(name); !ok {
			t.Errorf("expected %s to be registered", name)
		}
	}
}

func TestTrafficClearMissingHost(t *testing.T) {
	t.Parallel()
	reg := NewRegistry()
	RegisterTrafficTools(reg, nil, &mockStore{}, 1)

	tool, ok := reg.Lookup("traffic_clear")
	if !ok {
		t.Fatal("traffic_clear not found")
	}

	r, err := tool.Run(context.Background(), json.RawMessage(`{"host": ""}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for missing host")
	}
	if !strings.Contains(r.Content, "host is required") {
		t.Errorf("expected host required, got: %s", r.Content)
	}
}

func TestTrafficClearInvalidJSON(t *testing.T) {
	t.Parallel()
	reg := NewRegistry()
	RegisterTrafficTools(reg, nil, &mockStore{}, 1)

	tool, _ := reg.Lookup("traffic_clear")
	r, err := tool.Run(context.Background(), json.RawMessage(`{bad`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParseRawRequest(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		raw        string
		wantMethod string
		wantURL    string
		wantHost   string
		wantBody   string
	}{
		{
			name:       "basic_get",
			raw:        "GET /api/users HTTP/1.1\nHost: example.com\nAccept: application/json\n\n",
			wantMethod: "GET",
			wantURL:    "https://example.com/api/users",
			wantHost:   "example.com",
			wantBody:   "",
		},
		{
			name:       "post_with_body",
			raw:        "POST /login HTTP/1.1\nHost: example.com\nContent-Type: application/json\n\n{\"user\":\"admin\"}",
			wantMethod: "POST",
			wantURL:    "https://example.com/login",
			wantHost:   "example.com",
			wantBody:   `{"user":"admin"}`,
		},
		{
			name:       "full_url",
			raw:        "GET https://example.com/path HTTP/1.1\nHost: example.com\n\n",
			wantMethod: "GET",
			wantURL:    "https://example.com/path",
			wantHost:   "example.com",
			wantBody:   "",
		},
		{
			name:       "no_host",
			raw:        "GET /path HTTP/1.1\n\n",
			wantMethod: "GET",
			wantURL:    "/path",
		},
		{
			name:       "empty",
			raw:        "",
			wantMethod: "",
			wantURL:    "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			method, reqURL, headers, body := parseRawRequest(tt.raw)
			if method != tt.wantMethod {
				t.Errorf("method = %q, want %q", method, tt.wantMethod)
			}
			if reqURL != tt.wantURL {
				t.Errorf("url = %q, want %q", reqURL, tt.wantURL)
			}
			if tt.wantHost != "" {
				if headers["Host"] != tt.wantHost {
					t.Errorf("host = %q, want %q", headers["Host"], tt.wantHost)
				}
			}
			if tt.wantBody != "" {
				if string(body) != tt.wantBody {
					t.Errorf("body = %q, want %q", string(body), tt.wantBody)
				}
			}
		})
	}
}

func TestClip(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		s    string
		max  int
		want string
	}{
		{"short", "hello", 10, "hello"},
		{"exact", "hello", 5, "hello"},
		{"truncated", "hello world", 5, "hello"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := clip(tt.s, tt.max)
			if !strings.HasPrefix(got, tt.want) {
				t.Errorf("clip(%q, %d) = %q, want prefix %q", tt.s, tt.max, got, tt.want)
			}
			if tt.name == "truncated" && !strings.Contains(got, "truncated") {
				t.Errorf("expected truncation marker, got: %s", got)
			}
		})
	}
}

func TestTrafficStatsRegistered(t *testing.T) {
	t.Parallel()
	reg := NewRegistry()
	RegisterTrafficTools(reg, nil, &mockStore{}, 1)

	tool, ok := reg.Lookup("traffic_stats")
	if !ok {
		t.Fatal("traffic_stats not found")
	}
	if !tool.ReadOnly {
		t.Error("expected traffic_stats to be read-only")
	}
}

func TestTrafficHostsRegistered(t *testing.T) {
	t.Parallel()
	reg := NewRegistry()
	RegisterTrafficTools(reg, nil, &mockStore{}, 1)

	tool, ok := reg.Lookup("traffic_hosts")
	if !ok {
		t.Fatal("traffic_hosts not found")
	}
	if !tool.ReadOnly {
		t.Error("expected traffic_hosts to be read-only")
	}
}
