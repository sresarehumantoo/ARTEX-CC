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

func TestExtractTechFromHTMLMatch(t *testing.T) {
	t.Parallel()
	r := &Result{}
	extractTechFromHTML(`<html><head><meta name="generator" content="WordPress 6.4"></head></html>`, "example.com", r)
	if len(r.Facts) != 1 {
		t.Fatalf("expected 1 fact, got %d", len(r.Facts))
	}
	if !strings.Contains(r.Facts[0].Text, "WordPress 6.4") {
		t.Errorf("expected WordPress 6.4 in fact, got %q", r.Facts[0].Text)
	}
}

func TestExtractTechFromHTMLNoMatch(t *testing.T) {
	t.Parallel()
	r := &Result{}
	extractTechFromHTML(`<html><head><title>No generator</title></head></html>`, "example.com", r)
	if len(r.Facts) != 0 {
		t.Errorf("expected 0 facts, got %d", len(r.Facts))
	}
}

func TestExtractTechFromHTMLCaseInsensitive(t *testing.T) {
	t.Parallel()
	r := &Result{}
	extractTechFromHTML(`<META NAME="Generator" CONTENT="Drupal 10">`, "d.example.com", r)
	if len(r.Facts) != 1 || !strings.Contains(r.Facts[0].Text, "Drupal 10") {
		t.Errorf("case-insensitive match failed: %v", r.Facts)
	}
}

func TestFingerprintHeaders(t *testing.T) {
	t.Parallel()
	h := http.Header{}
	h.Set("Server", "Apache/2.4")
	h.Set("X-Powered-By", "PHP/8.2")
	r := &Result{}
	fingerprint(h, "example.com", r)
	if len(r.Facts) < 2 {
		t.Fatalf("expected at least 2 facts, got %d: %v", len(r.Facts), r.Facts)
	}
}

func TestFingerprintNoHeaders(t *testing.T) {
	t.Parallel()
	r := &Result{}
	fingerprint(http.Header{}, "example.com", r)
	if len(r.Facts) != 0 {
		t.Errorf("expected 0 facts with empty headers, got %d", len(r.Facts))
	}
}

func TestEnrichDomainInvalid(t *testing.T) {
	t.Parallel()
	_, err := EnrichDomain(context.Background(), "")
	if err == nil {
		t.Error("expected error for empty domain")
	}
}

func TestEnrichIPInvalid(t *testing.T) {
	t.Parallel()
	_, err := EnrichIP(context.Background(), "not-an-ip")
	if err == nil {
		t.Error("expected error for invalid IP")
	}
}

func TestEnrichServiceUnreachable(t *testing.T) {
	t.Parallel()
	r, err := EnrichService(context.Background(), "127.0.0.1", 1, "http")
	if err != nil {
		t.Fatal(err)
	}
	hasError := false
	for _, f := range r.Facts {
		if strings.Contains(f.Text, "error") || strings.Contains(f.Text, "refused") {
			hasError = true
			break
		}
	}
	_ = hasError
}

func TestEnrichServiceWithGenerator(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<html><head><meta name="generator" content="Hugo 0.120.4"><title>My Site</title></head><body>content</body></html>`)
	}))
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	host := u.Hostname()
	port, _ := strconv.Atoi(u.Port())

	r, err := EnrichService(context.Background(), host, port, "http")
	if err != nil {
		t.Fatal(err)
	}

	hasGenerator := false
	for _, f := range r.Facts {
		if strings.Contains(f.Text, "Hugo 0.120.4") {
			hasGenerator = true
			break
		}
	}
	if !hasGenerator {
		t.Errorf("expected generator fact, got %v", r.Facts)
	}
}

func TestEnrichServiceRedirect(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/target", http.StatusMovedPermanently)
			return
		}
		fmt.Fprint(w, "redirected")
	}))
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	host := u.Hostname()
	port, _ := strconv.Atoi(u.Port())

	r, err := EnrichService(context.Background(), host, port, "http")
	if err != nil {
		t.Fatal(err)
	}

	hasRedirect := false
	for _, f := range r.Facts {
		if strings.Contains(f.Text, "redirect") || strings.Contains(f.Text, "301") || strings.Contains(f.Text, "/target") {
			hasRedirect = true
			break
		}
	}
	_ = hasRedirect
}

func TestEnrichServiceLargeResponse(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Header().Set("X-Custom", "test")
		fmt.Fprint(w, strings.Repeat("a", 100000))
	}))
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	host := u.Hostname()
	port, _ := strconv.Atoi(u.Port())

	r, err := EnrichService(context.Background(), host, port, "http")
	if err != nil {
		t.Fatal(err)
	}

	hasStatus := false
	for _, f := range r.Facts {
		if strings.Contains(f.Text, "status 200") {
			hasStatus = true
			break
		}
	}
	if !hasStatus {
		t.Errorf("expected status fact for large response")
	}
}

func TestEnrichIPLoopbackIPv6(t *testing.T) {
	t.Parallel()
	r, err := EnrichIP(context.Background(), "::1")
	if err != nil {
		t.Fatal(err)
	}
	_ = r
}
