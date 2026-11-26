package main

import (
	"context"
	"net"
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

func TestLoopbackHostOnly(t *testing.T) {
	h := loopbackHostOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	loopback := &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 9400}
	loopback6 := &net.TCPAddr{IP: net.IPv6loopback, Port: 9400}
	remote := &net.TCPAddr{IP: net.IPv4(172, 17, 0, 2), Port: 9400}
	for _, tt := range []struct {
		local   net.Addr
		host    string
		allowed bool
	}{
		{loopback, "localhost:9400", true},
		{loopback, "LOCALHOST", true},
		{loopback, "127.0.0.1:9400", true},
		{loopback, "127.0.0.2", true},
		{loopback, "[::1]:9400", true},
		{loopback6, "[::1]", true},
		{loopback, "evil.example:9400", false},
		{loopback6, "evil.example", false},
		{loopback, "192.168.1.20:9400", false},
		{loopback, "", false},
		{remote, "artex.example.com", true},
		{remote, "evil.example:9400", true},
		{remote, "localhost:9400", true},
		{nil, "evil.example", true},
	} {
		r := httptest.NewRequest("GET", "/api/findings", nil)
		r.Host = tt.host
		if tt.local != nil {
			r = r.WithContext(context.WithValue(r.Context(), http.LocalAddrContextKey, tt.local))
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if got := w.Code == http.StatusNoContent; got != tt.allowed {
			t.Errorf("local %v Host %q: status %d, want allowed=%v", tt.local, tt.host, w.Code, tt.allowed)
		}
	}
}
