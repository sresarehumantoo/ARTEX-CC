package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/SREsAreHumanToo/ARTEX-CC/api"
	"github.com/SREsAreHumanToo/ARTEX-CC/mcpserver"
	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
)

func TestWebHandler(t *testing.T) {
	mcp := bearerAuth("s3cret", mcpserver.New(tools.NewRegistry()).StreamableHTTP())
	restAPI := api.New(nil, nil, 1, 1, api.WithDataDir(t.TempDir())).Handler()
	srv := httptest.NewServer(webHandler(mcp, restAPI))
	t.Cleanup(srv.Close)
	addr := strings.TrimPrefix(srv.URL, "http://")

	if got := initializeStatus(t, addr, "Bearer s3cret"); got != http.StatusOK {
		t.Errorf("mcp with bearer token: %d", got)
	}

	post := func(origin, fetchSite string) *http.Response {
		req, _ := http.NewRequest("POST", srv.URL+"/api/auth/init", strings.NewReader(`{"password":"correct horse battery"}`))
		req.Header.Set("Content-Type", "text/plain")
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		if fetchSite != "" {
			req.Header.Set("Sec-Fetch-Site", fetchSite)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp
	}
	if resp := post("http://localhost:1", "same-site"); resp.StatusCode != http.StatusForbidden {
		t.Errorf("cross-origin api write: %d", resp.StatusCode)
	}
	if resp := post(srv.URL, "same-origin"); resp.StatusCode == http.StatusForbidden {
		t.Errorf("same-origin api write refused")
	}

	for _, path := range []string{"/", "/api/auth/status", "/mcp"} {
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		for k, v := range map[string]string{
			"X-Content-Type-Options":  "nosniff",
			"X-Frame-Options":         "DENY",
			"Content-Security-Policy": "frame-ancestors 'none'",
			"Referrer-Policy":         "no-referrer",
		} {
			if got := resp.Header.Get(k); got != v {
				t.Errorf("%s %s = %q, want %q", path, k, got, v)
			}
		}
		if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "" {
			t.Errorf("%s Access-Control-Allow-Origin = %q", path, got)
		}
	}
}
