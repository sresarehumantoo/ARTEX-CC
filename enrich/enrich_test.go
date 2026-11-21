package enrich

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

func TestEnrichDomainLocalhost(t *testing.T) {
	t.Parallel()
	r, err := EnrichDomain(context.Background(), "localhost")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range r.Facts {
		if f.Source == "dns" && strings.Contains(f.Text, "127.0.0.1") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected A record 127.0.0.1 in facts; got %v", r.Facts)
	}
}

func TestEnrichIPLoopback(t *testing.T) {
	t.Parallel()
	r, err := EnrichIP(context.Background(), "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range r.Facts {
		if f.Source == "reverse-dns" && strings.Contains(f.Text, "PTR") {
			found = true
			break
		}
	}
	if !found {
		t.Logf("no PTR record for 127.0.0.1 (may depend on host config); facts: %v", r.Facts)
	}
}

func TestEnrichServiceHTTP(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, "<html><head><title>Test Page</title></head><body>hello</body></html>")
	}))
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	host := u.Hostname()
	port, _ := strconv.Atoi(u.Port())

	r, err := EnrichService(context.Background(), host, port, "http")
	if err != nil {
		t.Fatal(err)
	}

	var hasStatus, hasTitle bool
	for _, f := range r.Facts {
		if strings.Contains(f.Text, "status 200") {
			hasStatus = true
		}
		if strings.Contains(f.Text, "title: Test Page") {
			hasTitle = true
		}
	}
	if !hasStatus {
		t.Errorf("expected status 200 fact; got %v", r.Facts)
	}
	if !hasTitle {
		t.Errorf("expected title fact; got %v", r.Facts)
	}
}

func TestEnrichServiceHTTPS(t *testing.T) {
	t.Parallel()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "ok")
	}))
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	host := u.Hostname()
	port, _ := strconv.Atoi(u.Port())

	r, err := EnrichService(context.Background(), host, port, "https")
	if err != nil {
		t.Fatal(err)
	}

	var hasTLSCN, hasTLSIssuer bool
	for _, f := range r.Facts {
		if f.Source == "tls" && strings.HasPrefix(f.Text, "TLS CN=") {
			hasTLSCN = true
		}
		if f.Source == "tls" && strings.HasPrefix(f.Text, "TLS issuer=") {
			hasTLSIssuer = true
		}
	}
	if !hasTLSCN {
		t.Errorf("expected TLS CN fact; got %v", r.Facts)
	}
	if !hasTLSIssuer {
		t.Errorf("expected TLS issuer fact; got %v", r.Facts)
	}
}

func TestTechFingerprinting(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Server", "nginx/1.25.3")
		w.Header().Set("X-Powered-By", "Express")
		fmt.Fprint(w, "ok")
	}))
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	host := u.Hostname()
	port, _ := strconv.Atoi(u.Port())

	r, err := EnrichService(context.Background(), host, port, "http")
	if err != nil {
		t.Fatal(err)
	}

	var hasServer, hasPoweredBy bool
	for _, f := range r.Facts {
		if strings.Contains(f.Text, "header Server: nginx/1.25.3") {
			hasServer = true
		}
		if strings.Contains(f.Text, "header X-Powered-By: Express") {
			hasPoweredBy = true
		}
	}
	if !hasServer {
		t.Errorf("expected Server header fact; got %v", r.Facts)
	}
	if !hasPoweredBy {
		t.Errorf("expected X-Powered-By header fact; got %v", r.Facts)
	}
}
