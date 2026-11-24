package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTruncateRunes(t *testing.T) {
	if got := truncateRunes("hello", 10); got != "hello" {
		t.Fatalf("short: %q", got)
	}
	if got := truncateRunes("hello", 3); got != "hel" {
		t.Fatalf("long: %q", got)
	}
	if got := truncateRunes("", 5); got != "" {
		t.Fatalf("empty: %q", got)
	}
}

func TestCommandsEndpoints(t *testing.T) {
	s := newTestChatStore(t)
	conv, err := s.Create(CreateConversationRequest{Title: "Recon"})
	if err != nil {
		t.Fatal(err)
	}
	events := []ChatEvent{
		{Kind: ChatKindToolUse, ToolName: "Bash", ToolID: "a", Input: json.RawMessage(`{"command":"nmap"}`), Time: "2024-01-01T00:00:01Z"},
		{Kind: ChatKindToolResult, ToolID: "a", Text: "open"},
		{Kind: ChatKindToolUse, ToolName: "mcp__artex__insert_asset", ToolID: "b", Time: "2024-01-01T00:00:02Z"},
		{Kind: ChatKindToolResult, ToolID: "b", Text: "boom", IsError: true},
		{Kind: ChatKindText, Text: "ignored"},
	}
	for _, ev := range events {
		if _, err := s.AppendEvent(conv.ID, ev); err != nil {
			t.Fatal(err)
		}
	}
	a := &API{chat: s}
	mux := http.NewServeMux()
	a.registerCommandsRoutes(mux)

	get := func(url string, v any) {
		t.Helper()
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, url, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: %d", url, rec.Code)
		}
		if err := json.Unmarshal(rec.Body.Bytes(), v); err != nil {
			t.Fatal(err)
		}
	}

	var all []CommandEntry
	get("/api/commands", &all)
	if len(all) != 2 || all[0].ToolName != "mcp__artex__insert_asset" || !all[0].IsError || all[0].OutputPreview != "boom" {
		t.Fatalf("unexpected entries: %+v", all)
	}
	if all[1].ConversationTitle != "Recon" || all[1].OutputPreview != "open" {
		t.Fatalf("unexpected pairing: %+v", all[1])
	}

	var errs, prefix, none []CommandEntry
	get("/api/commands?error=true", &errs)
	get("/api/commands?tool=mcp__", &prefix)
	get("/api/commands?conv=other", &none)
	if len(errs) != 1 || len(prefix) != 1 || len(none) != 0 {
		t.Fatalf("filters: %d %d %d", len(errs), len(prefix), len(none))
	}

	var stats CommandStats
	get("/api/commands/stats", &stats)
	if stats.Total != 2 || stats.ErrorCount != 1 || stats.ByTool["Bash"] != 1 || stats.ByConversation[conv.ID] != 2 {
		t.Fatalf("stats: %+v", stats)
	}

	var paged []CommandEntry
	get("/api/commands?limit=1&offset=0", &paged)
	if len(paged) != 1 {
		t.Fatalf("paged limit=1: %d", len(paged))
	}
	get("/api/commands?limit=1&offset=1", &paged)
	if len(paged) != 1 {
		t.Fatalf("paged offset=1: %d", len(paged))
	}
	get("/api/commands?offset=100", &paged)
	if len(paged) != 0 {
		t.Fatalf("paged offset=100: %d", len(paged))
	}
	get("/api/commands?error=false", &paged)
	if len(paged) != 1 || paged[0].IsError {
		t.Fatalf("error=false filter: %+v", paged)
	}
}
