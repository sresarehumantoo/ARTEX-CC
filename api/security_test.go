package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSameOriginWrites(t *testing.T) {
	h := sameOriginWrites(nil, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	tests := []struct {
		name    string
		method  string
		origin  string
		fetch   string
		allowed bool
	}{
		{"no browser headers", "POST", "", "", true},
		{"same origin", "POST", "http://localhost:9400", "same-origin", true},
		{"same origin without fetch metadata", "DELETE", "http://localhost:9400", "", true},
		{"origin host case", "PUT", "http://LOCALHOST:9400", "", true},
		{"user initiated", "POST", "", "none", true},
		{"other port", "POST", "http://localhost:3000", "", false},
		{"other host", "PATCH", "https://evil.example", "", false},
		{"null origin", "POST", "null", "", false},
		{"cross-site", "POST", "", "cross-site", false},
		{"same-site", "DELETE", "", "same-site", false},
		{"cross-site with matching origin", "POST", "http://localhost:9400", "cross-site", false},
		{"cross-site GET", "GET", "https://evil.example", "cross-site", true},
		{"cross-site HEAD", "HEAD", "https://evil.example", "cross-site", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(tt.method, "http://localhost:9400/api/hints", strings.NewReader("{}"))
			if tt.origin != "" {
				r.Header.Set("Origin", tt.origin)
			}
			if tt.fetch != "" {
				r.Header.Set("Sec-Fetch-Site", tt.fetch)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if got := w.Code != http.StatusForbidden; got != tt.allowed {
				t.Fatalf("allowed = %v (status %d), want %v", got, w.Code, tt.allowed)
			}
		})
	}
}

func TestSameOriginWritesAllowedHosts(t *testing.T) {
	h := sameOriginWrites([]string{"Artex.Example.com", "ops.example.com:8443"}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	tests := []struct {
		name    string
		host    string
		origin  string
		fetch   string
		allowed bool
	}{
		{"listed origin, Host rewritten to localhost", "localhost:9400", "https://artex.example.com", "same-origin", true},
		{"listed origin case", "localhost:9400", "https://ARTEX.example.com", "", true},
		{"listed host without port matches any port", "localhost:9400", "https://artex.example.com:8443", "", true},
		{"listed port matches", "localhost:9400", "https://ops.example.com:8443", "", true},
		{"listed port mismatch", "localhost:9400", "https://ops.example.com", "", false},
		{"listed port other port", "localhost:9400", "https://ops.example.com:9443", "", false},
		{"unlisted origin", "localhost:9400", "https://evil.example", "", false},
		{"listed suffix is not a match", "localhost:9400", "https://evil-artex.example.com", "", false},
		{"listed origin cross-site", "localhost:9400", "https://artex.example.com", "cross-site", false},
		{"listed origin same-site", "localhost:9400", "https://artex.example.com", "same-site", false},
		{"forwarded Host still matches itself", "artex.example.com", "https://artex.example.com", "same-origin", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "http://localhost:9400/api/hints", strings.NewReader("{}"))
			r.Host = tt.host
			r.Header.Set("Origin", tt.origin)
			if tt.fetch != "" {
				r.Header.Set("Sec-Fetch-Site", tt.fetch)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if got := w.Code != http.StatusForbidden; got != tt.allowed {
				t.Fatalf("allowed = %v (status %d), want %v", got, w.Code, tt.allowed)
			}
		})
	}
}

func TestSameOriginWritesIgnoresForwardedHost(t *testing.T) {
	for _, allowed := range [][]string{nil, {"artex.example.com"}} {
		h := sameOriginWrites(allowed, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}))
		r := httptest.NewRequest("POST", "http://localhost:9400/api/hints", strings.NewReader("{}"))
		r.Header.Set("Origin", "https://evil.example")
		r.Header.Set("X-Forwarded-Host", "evil.example")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != http.StatusForbidden {
			t.Fatalf("allowed %v: status %d, want 403", allowed, w.Code)
		}
	}
}

func TestHostAllowed(t *testing.T) {
	allowed := []string{"artex.example.com", "OPS.example.com:8443", "[::1]:9400"}
	for _, tt := range []struct {
		host string
		want bool
	}{
		{"artex.example.com", true},
		{"ARTEX.EXAMPLE.COM:443", true},
		{"ops.example.com:8443", true},
		{"ops.example.com", false},
		{"ops.example.com:443", false},
		{"[::1]:9400", true},
		{"[::1]:9401", false},
		{"example.com", false},
		{"", false},
	} {
		if got := HostAllowed(tt.host, allowed); got != tt.want {
			t.Errorf("HostAllowed(%q) = %v, want %v", tt.host, got, tt.want)
		}
	}
	if HostAllowed("artex.example.com", nil) {
		t.Error("empty list allowed a host")
	}
}

func TestHandlerAcceptsAllowedOriginWrite(t *testing.T) {
	h := New(nil, nil, 1, 1, WithDataDir(t.TempDir()), WithAllowedHosts([]string{"artex.example.com"})).Handler()
	r := httptest.NewRequest("POST", "http://localhost:9400/api/auth/init", strings.NewReader(`{"password":"correct horse battery"}`))
	r.Header.Set("Origin", "https://artex.example.com")
	r.Header.Set("Sec-Fetch-Site", "same-origin")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("allowed-origin write: %d %s, want 200", w.Code, w.Body)
	}
}

func TestHandlerRefusesCrossOriginWrite(t *testing.T) {
	h := New(nil, nil, 1, 1, WithDataDir(t.TempDir())).Handler()

	r := httptest.NewRequest("POST", "http://localhost:9400/api/auth/init", strings.NewReader(`{"password":"correct horse battery"}`))
	r.Header.Set("Content-Type", "text/plain")
	r.Header.Set("Origin", "http://localhost:3000")
	r.Header.Set("Sec-Fetch-Site", "same-site")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("cross-origin write: %d", w.Code)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("Access-Control-Allow-Origin = %q", got)
	}

	r = httptest.NewRequest("OPTIONS", "http://localhost:9400/api/hints", nil)
	r.Header.Set("Origin", "http://localhost:3000")
	r.Header.Set("Access-Control-Request-Method", "POST")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("preflight Access-Control-Allow-Origin = %q", got)
	}

	r = httptest.NewRequest("POST", "http://localhost:9400/api/auth/init", strings.NewReader(`{"password":"correct horse battery"}`))
	r.Header.Set("Origin", "http://localhost:9400")
	r.Header.Set("Sec-Fetch-Site", "same-origin")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code == http.StatusForbidden {
		t.Fatalf("same-origin write refused: %s", w.Body)
	}
}

func TestSecurityHeaders(t *testing.T) {
	h := SecurityHeaders(http.NotFoundHandler())
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/api/nope", nil))
	want := map[string]string{
		"X-Content-Type-Options":  "nosniff",
		"X-Frame-Options":         "DENY",
		"Content-Security-Policy": "frame-ancestors 'none'",
		"Referrer-Policy":         "no-referrer",
	}
	for k, v := range want {
		if got := w.Header().Get(k); got != v {
			t.Errorf("%s = %q, want %q", k, got, v)
		}
	}
}
