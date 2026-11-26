package traffic

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTemplatePath(t *testing.T) {
	t.Parallel()
	cases := []struct {
		path string
		want string
	}{
		{"", "/"},
		{"/", "/"},
		{"/api/users", "/api/users"},
		{"/api/users/42", "/api/users/{id}"},
		{"/api/users/550e8400-e29b-41d4-a716-446655440000", "/api/users/{uuid}"},
		{"/api/objects/abcdef0123456789", "/api/objects/{hex}"},
		{"/api/sessions/ABCDEFGHIJKLMNOPQRSTUVWXyz", "/api/sessions/{token}"},
		{"/api/42/items/550e8400-e29b-41d4-a716-446655440000/detail", "/api/{id}/items/{uuid}/detail"},
		{"/api/ab", "/api/ab"},
		{"/api/users/", "/api/users/"},
		{"/v1/org/99/team/1/member/7", "/v1/org/{id}/team/{id}/member/{id}"},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			got := templatePath(tc.path)
			if got != tc.want {
				t.Fatalf("templatePath(%q) = %q, want %q", tc.path, got, tc.want)
			}
		})
	}
}

func TestOpenAndRecordDetailed(t *testing.T) {
	t.Parallel()
	tr := openTestTraffic(t)

	seedExchange(t, tr,
		"rec-0001", "target.example.com", "POST", "/api/v1/login", "https://target.example.com/api/v1/login",
		200, "application/json", 42, 128,
		"POST /api/v1/login HTTP/1.1\nHost: target.example.com\n", []byte(`{"user":"test"}`),
		"HTTP 200\nContent-Type: application/json\n", []byte(`{"token":"abc"}`))

	results, err := tr.Search("target.example.com", 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	r := results[0]
	if r.ID != "rec-0001" {
		t.Fatalf("id = %q", r.ID)
	}
	if r.Method != "POST" {
		t.Fatalf("method = %q", r.Method)
	}
	if r.Status != 200 {
		t.Fatalf("status = %d", r.Status)
	}
}

func TestSearchPagination(t *testing.T) {
	t.Parallel()
	tr := openTestTraffic(t)

	for i := 0; i < 5; i++ {
		id := fmt.Sprintf("page-%04d", i)
		seedExchange(t, tr, id, "paged.test", "GET", "/", "https://paged.test/",
			200, "text/html", 0, 10, "GET / HTTP/1.1\n", nil, "HTTP 200\n", []byte("ok"))
	}

	page0, err := tr.Search("paged.test", 0, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(page0) != 2 {
		t.Fatalf("page 0: got %d, want 2", len(page0))
	}

	page1, err := tr.Search("paged.test", 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(page1) != 2 {
		t.Fatalf("page 1: got %d, want 2", len(page1))
	}

	page2, err := tr.Search("paged.test", 2, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(page2) != 1 {
		t.Fatalf("page 2: got %d, want 1", len(page2))
	}
}

func TestSearchSizeClamp(t *testing.T) {
	t.Parallel()
	tr := openTestTraffic(t)
	seedExchange(t, tr, "clamp-1", "clamp.test", "GET", "/", "https://clamp.test/",
		200, "text/html", 0, 2, "GET / HTTP/1.1\n", nil, "HTTP 200\n", []byte("x"))

	results, err := tr.Search("clamp.test", 0, -1)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("size -1: got %d, want 1", len(results))
	}
}

func TestDeleteHost(t *testing.T) {
	t.Parallel()
	tr := openTestTraffic(t)

	seedExchange(t, tr, "del-1", "delete-me.test", "GET", "/", "https://delete-me.test/",
		200, "text/html", 0, 5, "GET / HTTP/1.1\n", nil, "HTTP 200\n", []byte("gone"))
	seedExchange(t, tr, "del-2", "keep-me.test", "GET", "/", "https://keep-me.test/",
		200, "text/html", 0, 5, "GET / HTTP/1.1\n", nil, "HTTP 200\n", []byte("stay"))

	n, err := tr.DeleteHost("delete-me")
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("deleted %d, want 1", n)
	}

	count, err := tr.Count()
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("remaining %d, want 1", count)
	}
}

func TestDeleteHostsExact(t *testing.T) {
	t.Parallel()
	tr := openTestTraffic(t)

	seedExchange(t, tr, "exact-1", "api.example.com", "GET", "/", "https://api.example.com/",
		200, "text/html", 0, 5, "GET / HTTP/1.1\n", nil, "HTTP 200\n", []byte("a"))
	seedExchange(t, tr, "exact-2", "api.example.com.cn", "GET", "/", "https://api.example.com.cn/",
		200, "text/html", 0, 5, "GET / HTTP/1.1\n", nil, "HTTP 200\n", []byte("b"))

	n, err := tr.DeleteHostsExact([]string{"api.example.com"})
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("deleted %d, want 1", n)
	}

	results, err := tr.Search("api.example.com.cn", 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("remaining %d, want 1", len(results))
	}
}

func TestDeleteAll(t *testing.T) {
	t.Parallel()
	tr := openTestTraffic(t)

	for i := 0; i < 3; i++ {
		id := fmt.Sprintf("all-%04d", i)
		seedExchange(t, tr, id, "bulk.test", "GET", "/", "https://bulk.test/",
			200, "text/html", 0, 2, "GET / HTTP/1.1\n", nil, "HTTP 200\n", []byte("x"))
	}

	deleted, _, err := tr.DeleteAll()
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 3 {
		t.Fatalf("deleted %d, want 3", deleted)
	}

	count, err := tr.Count()
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("remaining %d, want 0", count)
	}
}

func TestHARExport(t *testing.T) {
	t.Parallel()
	tr := openTestTraffic(t)

	seedExchange(t, tr, "har-1", "har.test", "GET", "/api/data", "https://har.test/api/data",
		200, "application/json", 20, 50,
		"GET /api/data HTTP/1.1\nHost: har.test\n", []byte(`{"q":"search"}`),
		"HTTP/1.1 200 OK\nContent-Type: application/json\n", []byte(`{"results":[]}`))

	doc, meta, err := tr.ExportHAR(PageQuery{Host: "har.test", RespMin: -1, RespMax: -1}, HARExportOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if meta.Exported != 1 {
		t.Fatalf("exported %d, want 1", meta.Exported)
	}
	if doc.Log.Version != HARVersion {
		t.Fatalf("HAR version = %q", doc.Log.Version)
	}
	if len(doc.Log.Entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(doc.Log.Entries))
	}
	entry := doc.Log.Entries[0]
	if entry.ArtexExchangeID != "har-1" {
		t.Fatalf("artex id = %q", entry.ArtexExchangeID)
	}
	if entry.Response.Status != 200 {
		t.Fatalf("status = %d", entry.Response.Status)
	}
}

func TestHARExportTruncation(t *testing.T) {
	t.Parallel()
	tr := openTestTraffic(t)

	for i := 0; i < 5; i++ {
		id := fmt.Sprintf("hartrunc-%04d", i)
		seedExchange(t, tr, id, "hartrunc.test", "GET", "/", "https://hartrunc.test/",
			200, "text/html", 0, 10, "GET / HTTP/1.1\n", nil, "HTTP/1.1 200 OK\n", []byte("hello"))
	}

	_, meta, err := tr.ExportHAR(PageQuery{Host: "hartrunc.test", RespMin: -1, RespMax: -1}, HARExportOptions{MaxEntries: 3})
	if err != nil {
		t.Fatal(err)
	}
	if meta.Exported != 3 {
		t.Fatalf("exported %d, want 3", meta.Exported)
	}
	if !meta.Truncated || meta.TruncatedReason != "max_entries" {
		t.Fatalf("truncated=%v reason=%q", meta.Truncated, meta.TruncatedReason)
	}
}

func TestHARExportEmpty(t *testing.T) {
	t.Parallel()
	tr := openTestTraffic(t)
	_, meta, err := tr.ExportHAR(PageQuery{Host: "nonexistent.test", RespMin: -1, RespMax: -1}, HARExportOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if meta.Exported != 0 {
		t.Fatalf("exported %d", meta.Exported)
	}
}

func TestHARExportNilTraffic(t *testing.T) {
	t.Parallel()
	var tr *Traffic
	_, _, err := tr.ExportHAR(PageQuery{RespMin: -1, RespMax: -1}, HARExportOptions{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestHARImportRoundtrip(t *testing.T) {
	t.Parallel()
	tr := openTestTraffic(t)

	har := HAR{
		Log: &HARLog{
			Version: "1.2",
			Creator: HARCreator{Name: "test", Version: "1.0"},
			Entries: []HAREntry{{
				StartedDateTime: time.Now().UTC().Format(time.RFC3339),
				Request: HARRequest{
					Method: "GET", URL: "https://import.test/api/data", HTTPVersion: "HTTP/1.1",
					Cookies: []HARCookie{}, Headers: []HARHeader{{Name: "Host", Value: "import.test"}},
					QueryString: []HARParam{}, HeadersSize: -1,
				},
				Response: HARResponse{
					Status: 200, StatusText: "OK", HTTPVersion: "HTTP/1.1",
					Cookies: []HARCookie{}, Headers: []HARHeader{{Name: "Content-Type", Value: "text/html"}},
					Content:     HARContent{Size: 4, MimeType: "text/html", Text: "test"},
					RedirectURL: "", HeadersSize: -1, BodySize: 4,
				},
				Cache: HARCache{}, Timings: HARTimings{Send: -1, Wait: -1, Receive: -1},
			}},
		},
	}
	raw, _ := json.Marshal(har)

	result, err := tr.ImportHAR(raw, HARImportOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Imported != 1 {
		t.Fatalf("imported %d", result.Imported)
	}

	results, err := tr.Search("import.test", 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results", len(results))
	}
}

func TestHARImportIdempotent(t *testing.T) {
	t.Parallel()
	tr := openTestTraffic(t)

	har := HAR{
		Log: &HARLog{
			Version: "1.2",
			Creator: HARCreator{Name: "test", Version: "1.0"},
			Entries: []HAREntry{{
				StartedDateTime: "2024-01-15T10:30:00Z",
				Request: HARRequest{
					Method: "GET", URL: "https://idempotent.test/page", HTTPVersion: "HTTP/1.1",
					Cookies: []HARCookie{}, Headers: []HARHeader{},
					QueryString: []HARParam{}, HeadersSize: -1,
				},
				Response: HARResponse{
					Status: 200, HTTPVersion: "HTTP/1.1",
					Cookies: []HARCookie{}, Headers: []HARHeader{},
					Content:     HARContent{Size: 2, MimeType: "text/plain", Text: "ok"},
					RedirectURL: "", HeadersSize: -1, BodySize: 2,
				},
				Cache: HARCache{}, Timings: HARTimings{Send: -1, Wait: -1, Receive: -1},
			}},
		},
	}
	raw, _ := json.Marshal(har)

	r1, _ := tr.ImportHAR(raw, HARImportOptions{})
	if r1.Imported != 1 {
		t.Fatalf("first: %d", r1.Imported)
	}
	r2, _ := tr.ImportHAR(raw, HARImportOptions{})
	if r2.Imported != 0 || r2.Skipped != 1 {
		t.Fatalf("second: imported=%d skipped=%d", r2.Imported, r2.Skipped)
	}
}

func TestHARImportInvalidDocument(t *testing.T) {
	t.Parallel()
	tr := openTestTraffic(t)

	for _, raw := range []string{
		"",
		"not json",
		`{"other":"data"}`,
		`{"log":{"version":"1.1","entries":[]}}`,
	} {
		if _, err := tr.ImportHAR([]byte(raw), HARImportOptions{}); err == nil {
			t.Fatalf("expected error for %q", raw)
		}
	}
}

func TestHARImportNilTraffic(t *testing.T) {
	t.Parallel()
	var tr *Traffic
	_, err := tr.ImportHAR([]byte(`{"log":{"version":"1.2","entries":[]}}`), HARImportOptions{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSpillSmallBodyInline(t *testing.T) {
	t.Parallel()
	tr := openTestTraffic(t)
	body := []byte("small body")
	sb := tr.spill(body, "text/plain")
	if sb.hash != "" {
		t.Fatal("small body should not spill")
	}
	if !bytes.Equal(sb.inline, body) {
		t.Fatalf("inline = %q", sb.inline)
	}
}

func TestSpillEmptyBody(t *testing.T) {
	t.Parallel()
	tr := openTestTraffic(t)
	for _, body := range [][]byte{nil, {}} {
		sb := tr.spill(body, "text/plain")
		if sb.hash != "" || sb.inline != nil || sb.index != "" {
			t.Fatal("empty body should produce empty storedBody")
		}
	}
}

func TestSpillLargeBodyToBlob(t *testing.T) {
	t.Parallel()
	tr := openTestTraffic(t)
	body := bytes.Repeat([]byte("A"), maxInlineBody+1)
	sb := tr.spill(body, "text/plain")
	if sb.hash == "" {
		t.Fatal("large body should spill")
	}
	if len(sb.inline) > blobPreview+100 {
		t.Fatalf("inline preview too large: %d", len(sb.inline))
	}
	if sb.index == "" {
		t.Fatal("text body should still be indexed")
	}
}

func TestSpillBinaryBody(t *testing.T) {
	t.Parallel()
	tr := openTestTraffic(t)
	body := append([]byte{0x89, 0x50, 0x4E, 0x47}, bytes.Repeat([]byte{0x00}, maxInlineBody+1)...)
	sb := tr.spill(body, "image/png")
	if sb.hash == "" {
		t.Fatal("large binary should spill")
	}
	if sb.index != "" {
		t.Fatal("binary should not be indexed")
	}
	if !strings.Contains(string(sb.inline), "[binary") {
		t.Fatalf("inline = %q", sb.inline)
	}
}

func TestIsBinaryBody(t *testing.T) {
	t.Parallel()
	cases := []struct {
		ct   string
		body []byte
		want bool
	}{
		{"text/html", []byte("<html>"), false},
		{"application/json", []byte(`{}`), false},
		{"image/png", []byte{0x89}, true},
		{"application/zip", []byte("PK"), true},
		{"application/pdf", []byte("%PDF"), true},
		{"", []byte("hello"), false},
		{"", append([]byte("ab"), 0x00), true},
		{"text/html; charset=utf-8", []byte("<p>"), false},
		{"audio/mpeg", []byte{0xFF, 0xFB}, true},
		{"application/wasm", []byte{0x00, 0x61}, true},
	}
	for _, tc := range cases {
		t.Run(tc.ct, func(t *testing.T) {
			if got := isBinaryBody(tc.ct, tc.body); got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestBinaryTag(t *testing.T) {
	t.Parallel()
	tag := binaryTag("image/png", []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A})
	for _, want := range []string{"image/png", "6 bytes", "magic=89504e47"} {
		if !strings.Contains(tag, want) {
			t.Fatalf("tag %q missing %q", tag, want)
		}
	}

	tag2 := binaryTag("", []byte{0xFF})
	if !strings.Contains(tag2, "application/octet-stream") {
		t.Fatalf("empty ct: %q", tag2)
	}
}

func TestClipBytes(t *testing.T) {
	t.Parallel()
	if got := clipBytes([]byte("hello world"), 5); string(got) != "hello" {
		t.Fatalf("got %q", got)
	}
	if got := clipBytes([]byte("hi"), 100); string(got) != "hi" {
		t.Fatalf("got %q", got)
	}
}

func TestTruncateUTF8(t *testing.T) {
	t.Parallel()
	if got := truncateUTF8([]byte("abc"), 10); got != "abc" {
		t.Fatalf("short = %q", got)
	}
	if got := truncateUTF8([]byte("abcdef"), 3); got != "abc" {
		t.Fatalf("cut = %q", got)
	}
}

func TestNullIfEmpty(t *testing.T) {
	t.Parallel()
	if nullIfEmpty("") != nil {
		t.Fatal("empty should be nil")
	}
	if nullIfEmpty("abc") != "abc" {
		t.Fatal("non-empty should pass through")
	}
}

func TestHeaderLines(t *testing.T) {
	t.Parallel()
	got := headerLines(map[string][]string{"Content-Type": {"application/json"}})
	if !strings.Contains(got, "Content-Type: application/json") {
		t.Fatalf("got %q", got)
	}
}

func TestSanitize(t *testing.T) {
	t.Parallel()
	cases := []struct{ in, want string }{
		{"example.com", "example.com"},
		{"foo/bar\\baz", "foo_bar_baz"},
		{"a:b?c*d", "a_b_c_d"},
		{"", "root"},
		{"_", "root"},
		{strings.Repeat("x", 200), strings.Repeat("x", 120)},
	}
	for _, tc := range cases {
		if got := sanitize(tc.in); got != tc.want {
			t.Fatalf("sanitize(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestHostOnly(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ in, want string }{
		{"example.com:443", "example.com"},
		{"example.com", "example.com"},
		{"[::1]:8080", "::1"},
		{"127.0.0.1:80", "127.0.0.1"},
	} {
		if got := hostOnly(tc.in); got != tc.want {
			t.Fatalf("hostOnly(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestProxyAddr(t *testing.T) {
	t.Parallel()
	tr := &Traffic{addr: ":8788"}
	if got := tr.ProxyAddr(); got != "http://127.0.0.1:8788" {
		t.Fatalf("got %q", got)
	}
	tr2 := &Traffic{addr: "10.0.0.1:8788"}
	if got := tr2.ProxyAddr(); got != "http://10.0.0.1:8788" {
		t.Fatalf("got %q", got)
	}
}

func TestProxyAddrResolvesPortZero(t *testing.T) {
	t.Parallel()
	tr := openTestTraffic(t)
	u, err := url.Parse(tr.ProxyAddr())
	if err != nil {
		t.Fatal(err)
	}
	if u.Port() == "0" {
		t.Fatalf("ProxyAddr = %q, want the bound port", tr.ProxyAddr())
	}
	go tr.Start()
	t.Cleanup(func() { tr.Shutdown(context.Background()) })
	deadline := time.Now().Add(5 * time.Second)
	for {
		c, err := net.Dial("tcp", u.Host)
		if err == nil {
			c.Close()
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("dial %s: %v", u.Host, err)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestValidateProxyURL(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		raw     string
		wantErr bool
	}{
		{"http://proxy.test:8080", false},
		{"https://proxy.test:443", false},
		{"socks5://proxy.test:1080", false},
		{"http://user:pass@proxy.test:8080", false},
		{"proxy.test:8080", true},
		{"ftp://proxy.test:8080", true},
		{"http://", true},
		{"", true},
	} {
		_, err := validateProxyURL(tc.raw)
		if (err != nil) != tc.wantErr {
			t.Fatalf("validateProxyURL(%q) err=%v, wantErr=%v", tc.raw, err, tc.wantErr)
		}
	}
}

func TestSetUpstreamProxy(t *testing.T) {
	t.Parallel()
	tr := openTestTraffic(t)

	if err := tr.SetUpstreamProxy("http://proxy.test:8080"); err != nil {
		t.Fatal(err)
	}
	if u := tr.upstream.Load(); u == nil || u.Host != "proxy.test:8080" {
		t.Fatalf("upstream = %v", u)
	}
	if err := tr.SetUpstreamProxy(""); err != nil {
		t.Fatal(err)
	}
	if tr.upstream.Load() != nil {
		t.Fatal("expected nil after clear")
	}
	if err := tr.SetUpstreamProxy("not-a-url"); err == nil {
		t.Fatal("expected error")
	}
}

func TestPage(t *testing.T) {
	t.Parallel()
	tr := openTestTraffic(t)

	seedExchange(t, tr, "pg-1", "page.test", "GET", "/api/{id}", "https://page.test/api/42",
		200, "application/json", 0, 100,
		"GET /api/42 HTTP/1.1\n", nil, "HTTP/1.1 200 OK\n", bytes.Repeat([]byte("x"), 100))
	seedExchange(t, tr, "pg-2", "page.test", "POST", "/api/items", "https://page.test/api/items",
		201, "application/json", 50, 20,
		"POST /api/items HTTP/1.1\n", []byte(`{"name":"a"}`), "HTTP/1.1 201 Created\n", []byte(`{"id":1}`))
	seedExchange(t, tr, "pg-3", "other.test", "GET", "/", "https://other.test/",
		404, "text/html", 0, 10,
		"GET / HTTP/1.1\n", nil, "HTTP/1.1 404 Not Found\n", []byte("not found"))

	pq := func(overrides PageQuery) PageQuery {
		overrides.RespMin = -1
		overrides.RespMax = -1
		return overrides
	}
	t.Run("host", func(t *testing.T) {
		rows, total, err := tr.Page(pq(PageQuery{Host: "page"}), 0, 10)
		if err != nil {
			t.Fatal(err)
		}
		if total != 2 || len(rows) != 2 {
			t.Fatalf("total=%d rows=%d", total, len(rows))
		}
	})
	t.Run("method", func(t *testing.T) {
		rows, _, err := tr.Page(pq(PageQuery{Method: "POST"}), 0, 10)
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) != 1 || rows[0].Method != "POST" {
			t.Fatalf("rows=%v", rows)
		}
	})
	t.Run("status_exact", func(t *testing.T) {
		rows, _, err := tr.Page(pq(PageQuery{Status: "404"}), 0, 10)
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) != 1 {
			t.Fatalf("rows=%d", len(rows))
		}
	})
	t.Run("status_class", func(t *testing.T) {
		rows, _, err := tr.Page(pq(PageQuery{Status: "2xx"}), 0, 10)
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) != 2 {
			t.Fatalf("rows=%d", len(rows))
		}
	})
	t.Run("path", func(t *testing.T) {
		rows, _, err := tr.Page(pq(PageQuery{Path: "{id}"}), 0, 10)
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) != 1 {
			t.Fatalf("rows=%d", len(rows))
		}
	})
	t.Run("sort_status_asc", func(t *testing.T) {
		rows, _, err := tr.Page(pq(PageQuery{Sort: "status", Order: "asc"}), 0, 10)
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) >= 2 && rows[0].Status > rows[1].Status {
			t.Fatal("wrong sort order")
		}
	})
	t.Run("resp_size_min", func(t *testing.T) {
		rows, _, err := tr.Page(PageQuery{RespMin: 50, RespMax: -1}, 0, 10)
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) != 1 {
			t.Fatalf("rows=%d", len(rows))
		}
	})
}

func TestStats(t *testing.T) {
	t.Parallel()
	tr := openTestTraffic(t)

	s, err := tr.Stats()
	if err != nil {
		t.Fatal(err)
	}
	if s.TotalExchanges != 0 {
		t.Fatalf("empty stats: total=%d", s.TotalExchanges)
	}

	seedExchange(t, tr, "stat-1", "stats.test", "GET", "/", "https://stats.test/",
		200, "text/html", 0, 100, "GET / HTTP/1.1\n", nil, "HTTP/1.1 200 OK\n", bytes.Repeat([]byte("x"), 100))
	seedExchange(t, tr, "stat-2", "stats.test", "POST", "/api", "https://stats.test/api",
		500, "application/json", 50, 200, "POST /api HTTP/1.1\n", []byte("data"), "HTTP/1.1 500 Internal Server Error\n", bytes.Repeat([]byte("y"), 200))
	seedExchange(t, tr, "stat-3", "stats.test", "GET", "/odd", "https://stats.test/odd",
		200, "text/plain", 0, 101, "GET /odd HTTP/1.1\n", nil, "HTTP/1.1 200 OK\n", bytes.Repeat([]byte("z"), 101))

	s, err = tr.Stats()
	if err != nil {
		t.Fatal(err)
	}
	if s.TotalExchanges != 3 {
		t.Fatalf("total=%d", s.TotalExchanges)
	}
	if s.UniqueHosts != 1 {
		t.Fatalf("hosts=%d", s.UniqueHosts)
	}
	if s.StatusCodes["2xx"] != 2 || s.StatusCodes["5xx"] != 1 {
		t.Fatalf("status codes=%v", s.StatusCodes)
	}
	if s.AvgRespSize != 133 {
		t.Fatalf("avg resp size=%d, want 133", s.AvgRespSize)
	}

	if ok, err := tr.Exists("stat-1"); err != nil || !ok {
		t.Fatalf("Exists(stat-1) = %v, %v; want true", ok, err)
	}
	if ok, err := tr.Exists("missing"); err != nil || ok {
		t.Fatalf("Exists(missing) = %v, %v; want false", ok, err)
	}
}

func TestTimeline(t *testing.T) {
	t.Parallel()
	tr := openTestTraffic(t)

	now := time.Now().Unix()
	db := tr.DB()
	for i := 0; i < 3; i++ {
		id := fmt.Sprintf("tl-%04d", i)
		db.Exec(`INSERT INTO exchanges(id,ts,host,method,url_template,url,status,content_type,req_len,resp_len,path) VALUES(?,?,?,?,?,?,?,?,?,?,?)`,
			id, now, "timeline.test", "GET", "/", "https://timeline.test/", 200, "text/html", 0, 1, "")
	}

	for _, bucket := range []string{"minute", "hour", "day"} {
		buckets, err := tr.Timeline("timeline.test", bucket)
		if err != nil {
			t.Fatalf("bucket %q: %v", bucket, err)
		}
		total := 0
		for _, b := range buckets {
			total += b.Count
		}
		if total != 3 {
			t.Fatalf("bucket %q: total=%d, want 3", bucket, total)
		}
	}
}

func TestNormalizeSearchHost(t *testing.T) {
	t.Parallel()
	cases := []struct {
		raw, host, port string
		err             bool
	}{
		{"example.com", "example.com", "", false},
		{"example.com:8443", "example.com", "8443", false},
		{"https://example.com:8443/path", "example.com", "8443", false},
		{"https://example.com/path", "example.com", "", false},
		{"[::1]", "::1", "", false},
		{"[::1]:8080", "::1", "8080", false},
		{"", "", "", true},
		{"example.com:99999", "", "", true},
		{"EXAMPLE.COM", "example.com", "", false},
	}
	for _, tc := range cases {
		host, port, err := normalizeSearchHost(tc.raw)
		if (err != nil) != tc.err {
			t.Fatalf("normalizeSearchHost(%q) err=%v", tc.raw, err)
		}
		if err == nil && (host != tc.host || port != tc.port) {
			t.Fatalf("normalizeSearchHost(%q) = (%q,%q)", tc.raw, host, port)
		}
	}
}

func TestStatusFilter(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		s  string
		ok bool
	}{
		{"", false},
		{"404", true},
		{"4xx", true},
		{"2xx", true},
		{"abc", false},
	} {
		_, _, ok := statusFilter(tc.s)
		if ok != tc.ok {
			t.Fatalf("statusFilter(%q) ok=%v", tc.s, ok)
		}
	}
}

func TestRewriteFilterSQL(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		s  string
		ok bool
	}{
		{"rewritten", true}, {"true", true}, {"1", true},
		{"original", true}, {"false", true}, {"0", true},
		{"all", false}, {"", false}, {"unknown", false},
	} {
		_, ok := rewriteFilterSQL(tc.s)
		if ok != tc.ok {
			t.Fatalf("rewriteFilterSQL(%q) ok=%v", tc.s, ok)
		}
	}
}

func TestFtsQuote(t *testing.T) {
	t.Parallel()
	if got := ftsQuote("hello"); got != `"hello"` {
		t.Fatalf("got %q", got)
	}
	if got := ftsQuote(`he"lo`); got != `"he""lo"` {
		t.Fatalf("got %q", got)
	}
}

func TestClip(t *testing.T) {
	t.Parallel()
	if got := clip("abc", 10); got != "abc" {
		t.Fatal("short should pass through")
	}
	got := clip(strings.Repeat("x", 100), 10)
	if !strings.Contains(got, "truncated") {
		t.Fatal("long should be truncated")
	}
}

func TestRecordExternal(t *testing.T) {
	t.Parallel()
	tr := openTestTraffic(t)

	err := tr.RecordExternal(ExternalExchange{
		Method: "GET", URL: "https://ext.test/api/v1/data", Status: 200,
		RequestHeaders:  map[string]string{"host": "ext.test"},
		ResponseHeaders: map[string]string{"content-type": "application/json"},
		ResponseBody:    `{"ok":true}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	results, err := tr.Search("ext.test", 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d", len(results))
	}
}

func TestRecordExternalInvalid(t *testing.T) {
	t.Parallel()
	tr := openTestTraffic(t)
	for _, ex := range []ExternalExchange{
		{Method: "GET", URL: ""},
		{Method: "", URL: "https://x.test/"},
		{Method: "GET POST", URL: "https://x.test/"},
		{Method: "GET", URL: "https://x.test/", RequestHeaders: map[string]string{"bad header": "v"}},
		{Method: "GET", URL: "https://x.test/", ResponseHeaders: map[string]string{"x": "a\nb"}},
	} {
		if err := tr.RecordExternal(ex); err == nil {
			t.Fatalf("expected error for %+v", ex)
		}
	}
}

func TestBlobRoundtrip(t *testing.T) {
	t.Parallel()
	tr := openTestTraffic(t)

	body := bytes.Repeat([]byte("Z"), maxInlineBody+100)
	sb := tr.spill(body, "text/plain")
	if sb.hash == "" {
		t.Fatal("expected blob hash")
	}
	f, size, err := tr.Blob(sb.hash)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if size != int64(len(body)) {
		t.Fatalf("size=%d, want %d", size, len(body))
	}
}

func TestBlobRange(t *testing.T) {
	t.Parallel()
	tr := openTestTraffic(t)
	body := bytes.Repeat([]byte("ABCDEFGH"), maxInlineBody/8+1)
	sb := tr.spill(body, "text/plain")
	data, total, err := tr.BlobRange(sb.hash, 0, 8)
	if err != nil {
		t.Fatal(err)
	}
	if total != int64(len(body)) || len(data) != 8 {
		t.Fatalf("total=%d data=%d", total, len(data))
	}
}

func TestBlobInvalidHash(t *testing.T) {
	t.Parallel()
	tr := openTestTraffic(t)
	_, _, err := tr.Blob("not-valid")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestClampBlobRange(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name             string
		offset, length   int64
		size             int64
		wantOff, wantLen int64
		wantEmpty        bool
	}{
		{"normal", 0, 10, 100, 0, 10, false},
		{"past_end", 100, 10, 100, 100, 0, true},
		{"neg_offset", -5, 10, 100, 0, 10, false},
		{"zero_size", 0, 10, 0, 0, 0, true},
		{"length_exceeds", 90, 20, 100, 90, 10, false},
		{"neg_length", 0, -1, 100, 0, 100, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			off, n, empty := clampBlobRange(tc.offset, tc.length, tc.size)
			if off != tc.wantOff || n != tc.wantLen || empty != tc.wantEmpty {
				t.Fatalf("got (%d,%d,%v), want (%d,%d,%v)", off, n, empty, tc.wantOff, tc.wantLen, tc.wantEmpty)
			}
		})
	}
}

func TestValidHeaderName(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		s    string
		want bool
	}{
		{"Content-Type", true},
		{"", false},
		{"has space", false},
		{"ok-token!", true},
	} {
		if got := validHeaderName(tc.s); got != tc.want {
			t.Fatalf("validHeaderName(%q) = %v", tc.s, got)
		}
	}
}

func TestProxyCausedErr(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		msg  string
		want bool
	}{
		{"HEAD request with body", true},
		{"http2 frame error", true},
		{"protocol_error on stream", true},
		{"malformed response", true},
		{"connection refused", false},
	} {
		if got := proxyCausedErr(fmt.Errorf("%s", tc.msg)); got != tc.want {
			t.Fatalf("proxyCausedErr(%q) = %v", tc.msg, got)
		}
	}
}

func TestDecodeHARBody(t *testing.T) {
	t.Parallel()
	got, err := decodeHARBody("hello", "")
	if err != nil || string(got) != "hello" {
		t.Fatalf("plain: %q %v", got, err)
	}
	got, err = decodeHARBody("aGVsbG8=", "base64")
	if err != nil || string(got) != "hello" {
		t.Fatalf("base64: %q %v", got, err)
	}
	got, err = decodeHARBody("", "")
	if err != nil || got != nil {
		t.Fatalf("empty: %v %v", got, err)
	}
	if _, err := decodeHARBody("data", "gzip"); err == nil {
		t.Fatal("expected error for unsupported encoding")
	}
}

func TestEncodeHARBody(t *testing.T) {
	t.Parallel()
	text, enc := encodeHARBody([]byte("hello"))
	if enc != "" || text != "hello" {
		t.Fatalf("text: %q %q", text, enc)
	}
	text, enc = encodeHARBody([]byte{0xFF, 0x00})
	if enc != "base64" || text == "" {
		t.Fatalf("binary: %q %q", text, enc)
	}
}

func TestSanitizeHARProto(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ in, want string }{
		{"HTTP/1.1", "HTTP/1.1"},
		{"", "HTTP/1.1"},
		{"not-http", "HTTP/1.1"},
	} {
		if got := sanitizeHARProto(tc.in); got != tc.want {
			t.Fatalf("sanitizeHARProto(%q) = %q", tc.in, got)
		}
	}
}

func TestRedactURLUserinfo(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ in, want string }{
		{"https://example.com/path", "https://example.com/path"},
		{"https://user:pass@example.com/path", "https://example.com/path"},
		{"", ""},
	} {
		if got := redactURLUserinfo(tc.in); got != tc.want {
			t.Fatalf("redactURLUserinfo(%q) = %q", tc.in, got)
		}
	}
}

func TestValidArtexExchangeID(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		id   string
		want bool
	}{
		{"1234-0001", true},
		{"har-abcdef", true},
		{"", false},
		{"has space", false},
		{strings.Repeat("a", maxArtexIDLen+1), false},
	} {
		if got := validArtexExchangeID(tc.id); got != tc.want {
			t.Fatalf("validArtexExchangeID(%q) = %v", tc.id, got)
		}
	}
}

func TestFormatHARHeaders(t *testing.T) {
	t.Parallel()
	headers := []HARHeader{
		{Name: "Content-Type", Value: "text/html"},
		{Name: "Cookie", Value: "session=abc"},
	}
	_, ct, hasCookie, err := formatHARHeaders(headers, "Cookie")
	if err != nil || ct != "text/html" || !hasCookie {
		t.Fatalf("ct=%q hasCookie=%v err=%v", ct, hasCookie, err)
	}
	if _, _, _, err := formatHARHeaders([]HARHeader{{Name: "", Value: "v"}}, ""); err == nil {
		t.Fatal("expected error for empty name")
	}
	if _, _, _, err := formatHARHeaders([]HARHeader{{Name: "X", Value: "a\nb"}}, ""); err == nil {
		t.Fatal("expected error for newline")
	}
}

func TestFormatHARRequestCookies(t *testing.T) {
	t.Parallel()
	got, err := formatHARRequestCookies([]HARCookie{{Name: "s", Value: "abc"}, {Name: "t", Value: "xyz"}})
	if err != nil || !strings.HasPrefix(got, "Cookie: ") {
		t.Fatalf("got %q err %v", got, err)
	}
	if _, err := formatHARRequestCookies([]HARCookie{{Name: "", Value: "v"}}); err == nil {
		t.Fatal("expected error")
	}
}

func TestEncodeHARFormParams(t *testing.T) {
	t.Parallel()
	got := encodeHARFormParams([]HARParam{{Name: "user", Value: "admin"}, {Name: "pass", Value: "s3cr3t"}})
	if string(got) != "user=admin&pass=s3cr3t" {
		t.Fatalf("got %q", got)
	}
}

func TestParseStoredRequestHead(t *testing.T) {
	t.Parallel()
	proto, headers := parseStoredRequestHead("GET /api HTTP/1.1\nHost: example.com\nContent-Type: application/json\n")
	if proto != "HTTP/1.1" || len(headers) != 2 {
		t.Fatalf("proto=%q headers=%v", proto, headers)
	}
}

func TestParseStoredResponseHead(t *testing.T) {
	t.Parallel()
	proto, statusText, headers := parseStoredResponseHead("HTTP/1.1 200 OK\nContent-Type: text/html\n")
	if proto != "HTTP/1.1" || statusText != "OK" || len(headers) != 1 {
		t.Fatalf("proto=%q statusText=%q headers=%v", proto, statusText, headers)
	}
}

func TestPathWithin(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		root, cand string
		want       bool
	}{
		{"/a/b", "/a/b/c", true},
		{"/a/b", "/a/b", true},
		{"/a/b", "/a/c", false},
	} {
		if got := pathWithin(tc.root, tc.cand); got != tc.want {
			t.Fatalf("pathWithin(%q,%q) = %v", tc.root, tc.cand, got)
		}
	}
}

func TestQueryWithHostPort(t *testing.T) {
	t.Parallel()
	tr := openTestTraffic(t)

	seedExchange(t, tr, "qhp-1", "10.10.10.1", "GET", "/", "https://10.10.10.1:8443/",
		200, "text/html", 0, 2, "GET / HTTP/1.1\n", nil, "HTTP/1.1 200 OK\n", []byte("x"))
	seedExchange(t, tr, "qhp-2", "10.10.10.1", "GET", "/", "https://10.10.10.1:9443/",
		200, "text/html", 0, 2, "GET / HTTP/1.1\n", nil, "HTTP/1.1 200 OK\n", []byte("y"))

	results, err := tr.query("10.10.10.1:8443", "", "", 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].ID != "qhp-1" {
		t.Fatalf("got %d results, want 1 for port 8443", len(results))
	}
}

func TestQueryLimitClamp(t *testing.T) {
	t.Parallel()
	tr := openTestTraffic(t)
	for i := 0; i < 15; i++ {
		id := fmt.Sprintf("qlc-%04d", i)
		seedExchange(t, tr, id, "qlc.test", "GET", "/", "https://qlc.test/",
			200, "text/html", 0, 1, "GET / HTTP/1.1\n", nil, "HTTP/1.1 200 OK\n", []byte("x"))
	}

	results, _ := tr.query("qlc.test", "", "", 0, 0)
	if len(results) != 3 {
		t.Fatalf("default limit: got %d, want 3", len(results))
	}
	results, _ = tr.query("qlc.test", "", "", 0, 100)
	if len(results) != 10 {
		t.Fatalf("max limit: got %d, want 10", len(results))
	}
}

func TestQueryContainsFilter(t *testing.T) {
	t.Parallel()
	tr := openTestTraffic(t)
	seedExchange(t, tr, "qcf-1", "filter.test", "GET", "/api/v1/users", "https://filter.test/api/v1/users",
		200, "application/json", 0, 2, "GET /api/v1/users HTTP/1.1\n", nil, "HTTP/1.1 200 OK\n", []byte("[]"))
	seedExchange(t, tr, "qcf-2", "filter.test", "GET", "/api/v1/items", "https://filter.test/api/v1/items",
		200, "application/json", 0, 2, "GET /api/v1/items HTTP/1.1\n", nil, "HTTP/1.1 200 OK\n", []byte("[]"))

	results, err := tr.query("filter.test", "users", "", 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d", len(results))
	}
}

func TestAssembleRaw(t *testing.T) {
	t.Parallel()
	got := assembleRaw("HTTP 200\n", []byte("body"), sql.NullString{}, 4)
	if !strings.Contains(got, "body") {
		t.Fatal("missing body")
	}
	got = assembleRaw("HTTP 200\n", []byte("preview"), sql.NullString{Valid: true, String: "abc123"}, 1000)
	if !strings.Contains(got, "@blob sha256:abc123") || !strings.Contains(got, "len=1000") {
		t.Fatalf("got %q", got)
	}
}

func TestIndexBytes(t *testing.T) {
	t.Parallel()
	tr := openTestTraffic(t)
	if size := tr.indexBytes(); size <= 0 {
		t.Fatalf("size=%d", size)
	}
}

func TestAllHostTrees(t *testing.T) {
	t.Parallel()
	tr := openTestTraffic(t)
	os.MkdirAll(filepath.Join(tr.dir, "example.com"), 0o755)
	os.MkdirAll(filepath.Join(tr.dir, "other.test"), 0o755)
	dirs, err := tr.allHostTrees()
	if err != nil {
		t.Fatal(err)
	}
	if len(dirs) != 2 {
		t.Fatalf("got %d dirs", len(dirs))
	}
}

func TestFindHeaderValue(t *testing.T) {
	t.Parallel()
	headers := []HARHeader{{Name: "Content-Type", Value: "text/html"}}
	if got := findHeaderValue(headers, "content-type"); got != "text/html" {
		t.Fatalf("got %q", got)
	}
	if got := findHeaderValue(headers, "Missing"); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestAppendParsedHeaders(t *testing.T) {
	t.Parallel()
	got := appendParsedHeaders(nil, []string{"Content-Type: text/html", "X: v", "", "nocolon"})
	if len(got) != 2 {
		t.Fatalf("got %d headers", len(got))
	}
}

func TestTamperHooksNilTraffic(t *testing.T) {
	t.Parallel()
	var tr *Traffic
	tr.SetTamperHooks(nil)
	if tr.TamperHooks() != nil {
		t.Fatal("expected nil")
	}
}

func TestRecoverHostDeleteStagesNilTraffic(t *testing.T) {
	t.Parallel()
	var tr *Traffic
	if err := tr.RecoverHostDeleteStages(nil); err != nil {
		t.Fatal(err)
	}
}

func TestHostDeleteStageNil(t *testing.T) {
	t.Parallel()
	var s *HostDeleteStage
	if s.Deleted() != 0 {
		t.Fatal("nil Deleted should be 0")
	}
	if err := s.Rollback(); err != nil {
		t.Fatal(err)
	}
	if err := s.Commit(); err != nil {
		t.Fatal(err)
	}
}
