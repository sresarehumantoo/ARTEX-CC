package api

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func newSkillsTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	a := New(&mockStore{}, nil, 1, 1, WithDataDir(t.TempDir()))
	srv := httptest.NewServer(a.Handler())
	t.Cleanup(srv.Close)
	return srv
}

func TestSkillsCRUD(t *testing.T) {
	srv := newSkillsTestServer(t)

	code, body := agentsDo(t, srv, "GET", "/api/skills", "")
	var list []Skill
	if err := json.Unmarshal(body, &list); err != nil || code != 200 {
		t.Fatalf("list empty: code=%d err=%v body=%s", code, err, body)
	}
	if len(list) != 0 {
		t.Fatalf("want 0 skills, got %d", len(list))
	}

	code, body = agentsDo(t, srv, "PUT", "/api/skills/recon-scan", `{"description":"run nmap","content":"nmap -sV target"}`)
	var created Skill
	if err := json.Unmarshal(body, &created); err != nil || code != 200 {
		t.Fatalf("put: code=%d err=%v body=%s", code, err, body)
	}
	if created.Name != "recon-scan" || created.Description != "run nmap" || created.Content != "nmap -sV target" {
		t.Fatalf("put result: %+v", created)
	}
	if created.UpdatedAt.IsZero() {
		t.Fatal("updated_at not set")
	}

	code, body = agentsDo(t, srv, "PUT", "/api/skills/enum-dirs", `{"description":"enumerate dirs","content":"gobuster dir -u target"}`)
	if code != 200 {
		t.Fatalf("put second: code=%d body=%s", code, body)
	}

	code, body = agentsDo(t, srv, "GET", "/api/skills", "")
	json.Unmarshal(body, &list)
	if code != 200 || len(list) != 2 {
		t.Fatalf("list: code=%d len=%d", code, len(list))
	}
	if list[0].Name != "enum-dirs" || list[1].Name != "recon-scan" {
		t.Fatalf("sort order: %s, %s", list[0].Name, list[1].Name)
	}

	code, _ = agentsDo(t, srv, "DELETE", "/api/skills/recon-scan", "")
	if code != 204 {
		t.Fatalf("delete: code=%d", code)
	}

	code, body = agentsDo(t, srv, "GET", "/api/skills", "")
	json.Unmarshal(body, &list)
	if code != 200 || len(list) != 1 || list[0].Name != "enum-dirs" {
		t.Fatalf("list after delete: code=%d list=%+v", code, list)
	}
}

func TestSkillPutBadName(t *testing.T) {
	srv := newSkillsTestServer(t)
	for name, path := range map[string]string{
		"uppercase":  "/api/skills/BadName",
		"spaces":     "/api/skills/bad name",
		"underscore": "/api/skills/bad_name",
		"dot":        "/api/skills/bad.name",
	} {
		code, _ := agentsDo(t, srv, "PUT", path, `{"description":"d","content":"c"}`)
		if code != 400 {
			t.Errorf("%s: code=%d, want 400", name, code)
		}
	}
}

func TestSkillPutBadBody(t *testing.T) {
	srv := newSkillsTestServer(t)
	code, _ := agentsDo(t, srv, "PUT", "/api/skills/valid-name", "{invalid")
	if code != 400 {
		t.Fatalf("code=%d, want 400", code)
	}
}

func TestSkillDeleteNotFound(t *testing.T) {
	srv := newSkillsTestServer(t)
	code, _ := agentsDo(t, srv, "DELETE", "/api/skills/nonexistent", "")
	if code != 404 {
		t.Fatalf("code=%d, want 404", code)
	}
}

func TestSkillDeleteBadName(t *testing.T) {
	srv := newSkillsTestServer(t)
	code, _ := agentsDo(t, srv, "DELETE", "/api/skills/BAD", "")
	if code != 404 {
		t.Fatalf("code=%d, want 404", code)
	}
}

func TestSkillPutUpdate(t *testing.T) {
	srv := newSkillsTestServer(t)
	agentsDo(t, srv, "PUT", "/api/skills/my-skill", `{"description":"v1","content":"c1"}`)
	code, body := agentsDo(t, srv, "PUT", "/api/skills/my-skill", `{"description":"v2","content":"c2"}`)
	var s Skill
	json.Unmarshal(body, &s)
	if code != 200 || s.Description != "v2" || s.Content != "c2" {
		t.Fatalf("update: code=%d %+v", code, s)
	}
}
