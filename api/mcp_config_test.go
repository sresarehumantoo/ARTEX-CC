package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
)

func TestMCPServersAddHTTP(t *testing.T) {
	a := New(nil, nil, 1, 1, WithDataDir(t.TempDir()))
	h := a.Handler()

	do := func(method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	rec := do("POST", "/api/mcp/servers", `{"name":"my-svc","type":"http","url":"http://localhost:8080/mcp","headers":{"Authorization":"Bearer tok"}}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("add http: got %d %s", rec.Code, rec.Body)
	}
	var list []mcpServerView
	json.Unmarshal(do("GET", "/api/mcp/servers", "").Body.Bytes(), &list)
	found := false
	for _, s := range list {
		if s.Name == "my-svc" {
			found = true
			if s.Type != "http" {
				t.Fatalf("type: want http, got %s", s.Type)
			}
		}
	}
	if !found {
		t.Fatal("added server not in list")
	}
}

func TestMCPServersBadBody(t *testing.T) {
	a := New(nil, nil, 1, 1, WithDataDir(t.TempDir()))
	h := a.Handler()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/mcp/servers", strings.NewReader("{invalid"))
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad body: got %d", rec.Code)
	}
}

func TestMCPServersDeleteNotFound(t *testing.T) {
	a := New(nil, nil, 1, 1, WithDataDir(t.TempDir()))
	h := a.Handler()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "/api/mcp/servers/nonexistent", nil)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("delete not found: got %d", rec.Code)
	}
}

func TestMCPServersBadType(t *testing.T) {
	a := New(nil, nil, 1, 1, WithDataDir(t.TempDir()))
	h := a.Handler()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/mcp/servers", strings.NewReader(`{"name":"x","type":"grpc"}`))
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad type: got %d", rec.Code)
	}
}

func TestMCPServersHTTPMissingURL(t *testing.T) {
	a := New(nil, nil, 1, 1, WithDataDir(t.TempDir()))
	h := a.Handler()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/mcp/servers", strings.NewReader(`{"name":"x","type":"http"}`))
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("missing url: got %d", rec.Code)
	}
}

func TestMCPServersBadName(t *testing.T) {
	a := New(nil, nil, 1, 1, WithDataDir(t.TempDir()))
	h := a.Handler()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/mcp/servers", strings.NewReader(`{"name":"!!!","type":"stdio","command":"x"}`))
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad name: got %d", rec.Code)
	}
}

func TestMCPServersDuplicate(t *testing.T) {
	a := New(nil, nil, 1, 1, WithDataDir(t.TempDir()))
	h := a.Handler()
	do := func(body string) int {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/api/mcp/servers", strings.NewReader(body))
		h.ServeHTTP(rec, req)
		return rec.Code
	}
	if code := do(`{"name":"dup","type":"stdio","command":"x"}`); code != http.StatusCreated {
		t.Fatalf("first add: %d", code)
	}
	if code := do(`{"name":"dup","type":"stdio","command":"y"}`); code != http.StatusConflict {
		t.Fatalf("duplicate: %d", code)
	}
}

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
	if rec := do("POST", "/api/mcp/servers", `{"name":"artex","type":"http","url":"http://a"}`); rec.Code != http.StatusConflict {
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

	if rec := do("DELETE", "/api/mcp/servers/artex", ""); rec.Code != http.StatusBadRequest {
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
