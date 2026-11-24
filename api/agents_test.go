package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func agentsDo(t *testing.T, srv *httptest.Server, method, path, body string) (int, []byte) {
	t.Helper()
	req, err := http.NewRequest(method, srv.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var buf strings.Builder
	b := make([]byte, 4096)
	for {
		n, err := res.Body.Read(b)
		buf.Write(b[:n])
		if err != nil {
			break
		}
	}
	return res.StatusCode, []byte(buf.String())
}

func seedAgentsHelper(t *testing.T, srv *httptest.Server) {
	t.Helper()
	agentsDo(t, srv, "GET", "/api/agents", "")
}

func TestAgentsGetExisting(t *testing.T) {
	a := New(&mockStore{}, nil, 1, 1, WithDataDir(t.TempDir()))
	srv := httptest.NewServer(a.Handler())
	defer srv.Close()

	seedAgentsHelper(t, srv)
	code, body := agentsDo(t, srv, "GET", "/api/agents/pentest-lead", "")
	var p AgentPreset
	if err := json.Unmarshal(body, &p); err != nil || code != 200 {
		t.Fatalf("get: code=%d err=%v", code, err)
	}
	if p.Name != "Pentest Lead" || p.Model != "opus" {
		t.Fatalf("agent = %+v", p)
	}
}

func TestAgentsCreateBadBody(t *testing.T) {
	a := New(&mockStore{}, nil, 1, 1, WithDataDir(t.TempDir()))
	srv := httptest.NewServer(a.Handler())
	defer srv.Close()

	if code, _ := agentsDo(t, srv, "POST", "/api/agents", "{invalid"); code != 400 {
		t.Fatalf("bad body: code=%d", code)
	}
}

func TestAgentsCreateEmptyName(t *testing.T) {
	a := New(&mockStore{}, nil, 1, 1, WithDataDir(t.TempDir()))
	srv := httptest.NewServer(a.Handler())
	defer srv.Close()

	if code, _ := agentsDo(t, srv, "POST", "/api/agents", `{}`); code != 400 {
		t.Fatalf("empty name: code=%d", code)
	}
	if code, _ := agentsDo(t, srv, "POST", "/api/agents", `{"name":"  "}`); code != 400 {
		t.Fatalf("whitespace name: code=%d", code)
	}
}

func TestAgentsCreateSymbolOnlyName(t *testing.T) {
	a := New(&mockStore{}, nil, 1, 1, WithDataDir(t.TempDir()))
	srv := httptest.NewServer(a.Handler())
	defer srv.Close()

	if code, _ := agentsDo(t, srv, "POST", "/api/agents", `{"name":"!!!"}`); code != 400 {
		t.Fatalf("symbol name: code=%d", code)
	}
}

func TestAgentsUpdateBadBody(t *testing.T) {
	a := New(&mockStore{}, nil, 1, 1, WithDataDir(t.TempDir()))
	srv := httptest.NewServer(a.Handler())
	defer srv.Close()

	seedAgentsHelper(t, srv)
	if code, _ := agentsDo(t, srv, "PATCH", "/api/agents/pentest-lead", "{invalid"); code != 400 {
		t.Fatalf("bad body: code=%d", code)
	}
}

func TestAgentsUpdateEmptyName(t *testing.T) {
	a := New(&mockStore{}, nil, 1, 1, WithDataDir(t.TempDir()))
	srv := httptest.NewServer(a.Handler())
	defer srv.Close()

	seedAgentsHelper(t, srv)
	if code, _ := agentsDo(t, srv, "PATCH", "/api/agents/pentest-lead", `{"name":"  "}`); code != 400 {
		t.Fatalf("empty name: code=%d", code)
	}
}

func TestAgentsUpdateBadModel(t *testing.T) {
	a := New(&mockStore{}, nil, 1, 1, WithDataDir(t.TempDir()))
	srv := httptest.NewServer(a.Handler())
	defer srv.Close()

	seedAgentsHelper(t, srv)
	if code, _ := agentsDo(t, srv, "PATCH", "/api/agents/pentest-lead", `{"model":"gpt"}`); code != 400 {
		t.Fatalf("bad model: code=%d", code)
	}
}

func TestAgentsUpdateNotFound(t *testing.T) {
	a := New(&mockStore{}, nil, 1, 1, WithDataDir(t.TempDir()))
	srv := httptest.NewServer(a.Handler())
	defer srv.Close()

	if code, _ := agentsDo(t, srv, "PATCH", "/api/agents/nonexistent", `{"description":"x"}`); code != 404 {
		t.Fatalf("not found: code=%d", code)
	}
}

func TestAgentsDeleteNotFound(t *testing.T) {
	a := New(&mockStore{}, nil, 1, 1, WithDataDir(t.TempDir()))
	srv := httptest.NewServer(a.Handler())
	defer srv.Close()

	if code, _ := agentsDo(t, srv, "DELETE", "/api/agents/nonexistent", ""); code != 404 {
		t.Fatalf("not found: code=%d", code)
	}
}

func TestAgentsCRUD(t *testing.T) {
	a := New(&mockStore{}, nil, 1, 1, WithDataDir(t.TempDir()))
	srv := httptest.NewServer(a.Handler())
	defer srv.Close()

	code, body := agentsDo(t, srv, "GET", "/api/agents", "")
	var list []AgentPreset
	if err := json.Unmarshal(body, &list); err != nil || code != 200 || len(list) != 3 {
		t.Fatalf("seed: code=%d len=%d err=%v", code, len(list), err)
	}
	if list[0].ID != "pentest-lead" {
		t.Fatalf("first id = %q", list[0].ID)
	}

	code, body = agentsDo(t, srv, "POST", "/api/agents", `{"name":"Recon Specialist","model":"haiku","allowed_tools":["Bash"," Bash ","Read"]}`)
	var created AgentPreset
	json.Unmarshal(body, &created)
	if code != 201 || created.ID != "recon-specialist-2" || len(created.AllowedTools) != 2 {
		t.Fatalf("create: code=%d %+v", code, created)
	}

	code, _ = agentsDo(t, srv, "POST", "/api/agents", `{"name":"x","model":"gpt"}`)
	if code != 400 {
		t.Fatalf("bad model: code=%d", code)
	}

	code, body = agentsDo(t, srv, "PATCH", "/api/agents/recon-specialist-2", `{"description":"d"}`)
	var updated AgentPreset
	json.Unmarshal(body, &updated)
	if code != 200 || updated.Description != "d" || updated.Model != "haiku" {
		t.Fatalf("patch: code=%d %+v", code, updated)
	}

	if code, _ = agentsDo(t, srv, "DELETE", "/api/agents/recon-specialist-2", ""); code != 204 {
		t.Fatalf("delete: code=%d", code)
	}
	if code, _ = agentsDo(t, srv, "GET", "/api/agents/recon-specialist-2", ""); code != 404 {
		t.Fatalf("get deleted: code=%d", code)
	}
	if code, _ = agentsDo(t, srv, "GET", "/api/agents/..%2Fx", ""); code != 404 {
		t.Fatalf("traversal: code=%d", code)
	}
}
