package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func workspaceDoHelper(t *testing.T, dir string) func(string, string, string) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	a := New(nil, nil, 1, 1, WithDataDir(dir))
	a.registerWorkspaceRoutes(mux)
	return func(method, url, body string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(method, url, strings.NewReader(body)))
		return rec
	}
}

func TestWorkspace(t *testing.T) {
	dir := t.TempDir()
	do := workspaceDoHelper(t, dir)

	if c := do("PUT", "/api/workspace/file?path=a/b.txt", "hi").Code; c != 204 {
		t.Fatalf("put: %d", c)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "workspace", "a", "b.txt")); string(b) != "hi" {
		t.Fatalf("content %q", b)
	}
	if rec := do("GET", "/api/workspace/file?path=a/b.txt", ""); rec.Body.String() != "hi" || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/plain") {
		t.Fatalf("get: %q %s", rec.Body.String(), rec.Header().Get("Content-Type"))
	}
	if rec := do("GET", "/api/workspace?path=a", ""); !strings.Contains(rec.Body.String(), `"b.txt"`) {
		t.Fatalf("list: %s", rec.Body.String())
	}
	if c := do("POST", "/api/workspace/mkdir?path=new", "").Code; c != 201 {
		t.Fatalf("mkdir: %d", c)
	}
	for _, p := range []string{"../x", "a/../../x", "/etc/passwd"} {
		if c := do("PUT", "/api/workspace/file?path="+p, "x").Code; c != 400 {
			t.Fatalf("traversal %q: %d", p, c)
		}
	}
	if c := do("DELETE", "/api/workspace/file?path=", "").Code; c != 400 {
		t.Fatalf("delete root: %d", c)
	}
	if c := do("DELETE", "/api/workspace/file?path=a/b.txt", "").Code; c != 204 {
		t.Fatalf("delete: %d", c)
	}
	if c := do("GET", "/api/workspace/file?path=a/b.txt", "").Code; c != 404 {
		t.Fatalf("get missing: %d", c)
	}
}

func TestWorkspaceReadJSON(t *testing.T) {
	dir := t.TempDir()
	do := workspaceDoHelper(t, dir)
	do("PUT", "/api/workspace/file?path=data.json", `{"key":"val"}`)
	rec := do("GET", "/api/workspace/file?path=data.json", "")
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("content-type: want application/json, got %s", ct)
	}
}

func TestWorkspaceReadDownload(t *testing.T) {
	dir := t.TempDir()
	do := workspaceDoHelper(t, dir)
	do("PUT", "/api/workspace/file?path=report.txt", "data")
	rec := do("GET", "/api/workspace/file?path=report.txt&download=1", "")
	disp := rec.Header().Get("Content-Disposition")
	if !strings.Contains(disp, "report.txt") {
		t.Fatalf("content-disposition: %s", disp)
	}
}

func TestWorkspaceReadDirectory(t *testing.T) {
	dir := t.TempDir()
	do := workspaceDoHelper(t, dir)
	do("POST", "/api/workspace/mkdir?path=subdir", "")
	rec := do("GET", "/api/workspace/file?path=subdir", "")
	if rec.Code != 400 {
		t.Fatalf("read dir: want 400, got %d", rec.Code)
	}
}

func TestWorkspaceListRoot(t *testing.T) {
	dir := t.TempDir()
	do := workspaceDoHelper(t, dir)
	do("PUT", "/api/workspace/file?path=top.txt", "x")
	rec := do("GET", "/api/workspace", "")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "top.txt") {
		t.Fatalf("list root: code=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestWorkspaceListMissing(t *testing.T) {
	dir := t.TempDir()
	do := workspaceDoHelper(t, dir)
	rec := do("GET", "/api/workspace?path=nonexistent", "")
	if rec.Code != 404 {
		t.Fatalf("list missing: want 404, got %d", rec.Code)
	}
}

func TestWorkspaceDeleteMissing(t *testing.T) {
	dir := t.TempDir()
	do := workspaceDoHelper(t, dir)
	rec := do("DELETE", "/api/workspace/file?path=nope.txt", "")
	if rec.Code != 404 {
		t.Fatalf("delete missing: want 404, got %d", rec.Code)
	}
}

func TestWorkspaceMkdirMissingPath(t *testing.T) {
	dir := t.TempDir()
	do := workspaceDoHelper(t, dir)
	rec := do("POST", "/api/workspace/mkdir?path=", "")
	if rec.Code != 400 {
		t.Fatalf("mkdir no path: want 400, got %d", rec.Code)
	}
}

func TestWorkspaceReadMissingPath(t *testing.T) {
	dir := t.TempDir()
	do := workspaceDoHelper(t, dir)
	rec := do("GET", "/api/workspace/file?path=", "")
	if rec.Code != 400 {
		t.Fatalf("read no path: want 400, got %d", rec.Code)
	}
}

func TestWorkspaceWriteMissingPath(t *testing.T) {
	dir := t.TempDir()
	do := workspaceDoHelper(t, dir)
	rec := do("PUT", "/api/workspace/file?path=", "data")
	if rec.Code != 400 {
		t.Fatalf("write no path: want 400, got %d", rec.Code)
	}
}

func TestWorkspaceRootedUnderDataDir(t *testing.T) {
	dir := t.TempDir()
	secrets := map[string]string{
		"auth.json":                    `{"password_hash":"x"}`,
		"identity.key":                 "key",
		"traffic/_ca/mitmproxy-ca.pem": "pem",
	}
	for name, body := range secrets {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	do := workspaceDoHelper(t, dir)

	rec := do("GET", "/api/workspace", "")
	if rec.Code != 200 || rec.Body.String() != "[]\n" {
		t.Fatalf("list root: %d %q", rec.Code, rec.Body.String())
	}
	info, err := os.Stat(filepath.Join(dir, "workspace"))
	if err != nil || info.Mode().Perm() != 0o700 {
		t.Fatalf("workspace dir: %v %v", info, err)
	}

	for _, p := range []string{"auth.json", "identity.key", "traffic/_ca/mitmproxy-ca.pem"} {
		if c := do("GET", "/api/workspace/file?path="+p, "").Code; c != 404 {
			t.Errorf("read %s: %d", p, c)
		}
		if c := do("DELETE", "/api/workspace/file?path="+p, "").Code; c != 404 {
			t.Errorf("delete %s: %d", p, c)
		}
	}
	if c := do("GET", "/api/workspace?path=traffic/_ca", "").Code; c != 404 {
		t.Errorf("list traffic/_ca: %d", c)
	}
	for _, p := range []string{"../auth.json", "../identity.key", "../traffic/_ca/mitmproxy-ca.pem", "a/../../auth.json"} {
		for _, m := range []string{"GET", "PUT", "DELETE"} {
			if c := do(m, "/api/workspace/file?path="+p, "x").Code; c != 400 {
				t.Errorf("%s %s: %d", m, p, c)
			}
		}
		if c := do("POST", "/api/workspace/mkdir?path="+p, "").Code; c != 400 {
			t.Errorf("mkdir %s: %d", p, c)
		}
	}
	if c := do("GET", "/api/workspace?path=..", "").Code; c != 400 {
		t.Errorf("list ..: %d", c)
	}

	if c := do("PUT", "/api/workspace/file?path=auth.json", "mine").Code; c != 204 {
		t.Fatalf("put workspace auth.json: %d", c)
	}
	for name, body := range secrets {
		if b, _ := os.ReadFile(filepath.Join(dir, filepath.FromSlash(name))); string(b) != body {
			t.Errorf("%s changed: %q", name, b)
		}
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "workspace", "auth.json")); string(b) != "mine" {
		t.Fatalf("workspace auth.json: %q", b)
	}
}

func TestWorkspaceSymlinkEscape(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "auth.json"), []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	do := workspaceDoHelper(t, dir)
	do("POST", "/api/workspace/mkdir?path=sub", "")
	ws := filepath.Join(dir, "workspace")
	if err := os.Symlink(filepath.Join(dir, "auth.json"), filepath.Join(ws, "link.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(dir, filepath.Join(ws, "out")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../../auth.json", filepath.Join(ws, "sub", "rel.json")); err != nil {
		t.Fatal(err)
	}

	for _, p := range []string{"link.json", "out/auth.json", "sub/rel.json"} {
		if rec := do("GET", "/api/workspace/file?path="+p, ""); rec.Code != 400 || strings.Contains(rec.Body.String(), "secret") {
			t.Errorf("read %s: %d %q", p, rec.Code, rec.Body.String())
		}
		if c := do("PUT", "/api/workspace/file?path="+p, "pwned").Code; c != 400 {
			t.Errorf("write %s: %d", p, c)
		}
	}
	if c := do("DELETE", "/api/workspace/file?path=out/auth.json", "").Code; c != 400 {
		t.Errorf("delete through symlink: %d", c)
	}
	if c := do("GET", "/api/workspace?path=out", "").Code; c != 400 {
		t.Errorf("list through symlink: %d", c)
	}
	if c := do("POST", "/api/workspace/mkdir?path=out/x", "").Code; c != 400 {
		t.Errorf("mkdir through symlink: %d", c)
	}
	if c := do("DELETE", "/api/workspace/file?path=link.json", "").Code; c != 204 {
		t.Errorf("delete link: %d", c)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "auth.json")); string(b) != "secret" {
		t.Fatalf("auth.json changed: %q", b)
	}
	if _, err := os.Stat(filepath.Join(dir, "x")); !os.IsNotExist(err) {
		t.Fatalf("mkdir escaped: %v", err)
	}
}

func TestWorkspaceSymlinkInside(t *testing.T) {
	dir := t.TempDir()
	do := workspaceDoHelper(t, dir)
	do("PUT", "/api/workspace/file?path=notes/a.txt", "inside")
	if err := os.Symlink("notes/a.txt", filepath.Join(dir, "workspace", "alias.txt")); err != nil {
		t.Fatal(err)
	}
	if rec := do("GET", "/api/workspace/file?path=alias.txt", ""); rec.Code != 200 || rec.Body.String() != "inside" {
		t.Fatalf("read inside link: %d %q", rec.Code, rec.Body.String())
	}
}
