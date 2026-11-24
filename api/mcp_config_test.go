package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/SREsAreHumanToo/cc-artex/tools"
)

func TestMCPServersLifecycle(t *testing.T) {
	a := New(nil, nil, 1, 1, WithDataDir(t.TempDir()), WithToolList([]tools.Tool{{Name: "insert_assets", ReadOnly: false}}))
	h := a.Handler()

	do := func(method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	if rec := do("POST", "/api/mcp/servers", `{"name":"x","type":"stdio"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("missing command: got %d", rec.Code)
	}
	if rec := do("POST", "/api/mcp/servers", `{"name":"cc-artex","type":"http","url":"http://a"}`); rec.Code != http.StatusConflict {
		t.Fatalf("reserved name: got %d", rec.Code)
	}
	if rec := do("POST", "/api/mcp/servers", `{"name":"fs","type":"stdio","command":"npx","env":{"K":"v"}}`); rec.Code != http.StatusCreated {
		t.Fatalf("add: got %d %s", rec.Code, rec.Body)
	}

	var list []mcpServerView
	if err := json.Unmarshal(do("GET", "/api/mcp/servers", "").Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || !list[0].BuiltIn || list[1].Name != "fs" || len(list[1].EnvKeys) != 1 {
		t.Fatalf("unexpected list: %+v", list)
	}

	if rec := do("DELETE", "/api/mcp/servers/cc-artex", ""); rec.Code != http.StatusBadRequest {
		t.Fatalf("delete built-in: got %d", rec.Code)
	}
	if rec := do("DELETE", "/api/mcp/servers/fs", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("delete: got %d", rec.Code)
	}

	var ti []ToolInfo
	if err := json.Unmarshal(do("GET", "/api/tools", "").Body.Bytes(), &ti); err != nil {
		t.Fatal(err)
	}
	if len(ti) != 1 || ti[0].Category != "assets" {
		t.Fatalf("unexpected tools: %+v", ti)
	}
}
