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
	a := &API{files: NewFileStore(dir)}
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
	if b, _ := os.ReadFile(filepath.Join(dir, "a", "b.txt")); string(b) != "hi" {
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
