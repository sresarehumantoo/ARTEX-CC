package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
)

func TestToolCategory(t *testing.T) {
	for input, want := range map[string]string{
		"Bash":                         "system",
		"Read":                         "system",
		"add_finding":                  "findings",
		"insert_asset":                 "assets",
		"mcp__artex__add_finding":      "findings",
		"mcp__artex__insert_asset":     "assets",
		"mcp__artex__traffic_snapshot": "traffic",
		"mcp__artex__evidence_capture": "evidence",
		"record_coverage":              "coverage",
		"save_credential":              "credentials",
		"graph_overview":               "graph",
		"insert_node":                  "graph",
		"ingest_data":                  "enrichment",
		"log_activity":                 "activity",
		"create_engagement":            "engagement",
		"something_random":             "other",
	} {
		got := toolCategory(input)
		if got != want {
			t.Errorf("toolCategory(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestHandleListTools_Empty(t *testing.T) {
	a := newTestAPI(&mockStore{})
	h := a.Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/tools", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", rec.Code)
	}
	var list []ToolInfo
	json.NewDecoder(rec.Body).Decode(&list)
	if len(list) != 0 {
		t.Fatalf("want 0 tools, got %d", len(list))
	}
}

func TestHandleListTools_WithTools(t *testing.T) {
	toolList := []tools.Tool{
		{Name: "add_finding", Description: "add a finding"},
		{Name: "Bash", Description: "run bash"},
	}
	a := New(&mockStore{}, nil, 1, 1, WithToolList(toolList))
	h := a.Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/tools", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	var list []ToolInfo
	json.NewDecoder(rec.Body).Decode(&list)
	if len(list) != 2 {
		t.Fatalf("want 2 tools, got %d", len(list))
	}
	if list[0].Category != "findings" || list[1].Category != "system" {
		t.Fatalf("categories: %s, %s", list[0].Category, list[1].Category)
	}
}
