package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkspace(t *testing.T) {
	dir := t.TempDir()
	mux := http.NewServeMux()
	a := &API{files: NewFileStore(dir)}
	a.registerWorkspaceRoutes(mux)

	do := func(method, url, body string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(method, url, strings.NewReader(body)))
		return rec
	}

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
