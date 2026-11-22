package traffic

import (
	"strings"
	"testing"
	"time"
)

func openTestTraffic(t *testing.T) *Traffic {
	t.Helper()
	dir := t.TempDir()
	tr, err := Open(dir, "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { tr.Close() })
	return tr
}

func seedExchange(t *testing.T, tr *Traffic, id, host, method, tmpl, rawURL string, status int, ct string, reqLen, respLen int, reqHead string, reqBody []byte, respHead string, respBody []byte) {
	t.Helper()
	seedExchangeAt(t, tr, id, host, method, tmpl, rawURL, status, ct, reqLen, respLen, reqHead, reqBody, respHead, respBody, time.Now().Unix())
}

func seedExchangeAt(t *testing.T, tr *Traffic, id, host, method, tmpl, rawURL string, status int, ct string, reqLen, respLen int, reqHead string, reqBody []byte, respHead string, respBody []byte, ts int64) {
	t.Helper()
	db := tr.DB()
	_, err := db.Exec(
		`INSERT INTO exchanges(id,ts,host,method,url_template,url,status,content_type,req_len,resp_len,path)
		VALUES(?,?,?,?,?,?,?,?,?,?,?)`,
		id, ts, host, method, tmpl, rawURL, status, ct, reqLen, respLen, "")
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(
		`INSERT INTO exchange_bodies(id,req_head,req_body,resp_head,resp_body)
		VALUES(?,?,?,?,?)`,
		id, reqHead, reqBody, respHead, respBody)
	if err != nil {
		t.Fatal(err)
	}
}

func TestTrafficStoreRoundtrip(t *testing.T) {
	t.Parallel()
	tr := openTestTraffic(t)

	seedExchange(t, tr,
		"test-0001", "example.com", "GET", "/api/{id}", "https://example.com/api/123",
		200, "application/json", 100, 500,
		"GET /api/123 HTTP/1.1\nHost: example.com\n", []byte(`{"q":"test"}`),
		"HTTP 200\nContent-Type: application/json\n", []byte(`{"result":"ok"}`))

	t.Run("Search", func(t *testing.T) {
		results, err := tr.Search("example.com", 0, 10)
		if err != nil {
			t.Fatal(err)
		}
		if len(results) != 1 {
			t.Fatalf("got %d results, want 1", len(results))
		}
		if results[0].Host != "example.com" {
			t.Fatalf("got host %q, want %q", results[0].Host, "example.com")
		}
		if results[0].Method != "GET" {
			t.Fatalf("got method %q, want %q", results[0].Method, "GET")
		}
		if results[0].Status != 200 {
			t.Fatalf("got status %d, want 200", results[0].Status)
		}
	})

	t.Run("Count", func(t *testing.T) {
		n, err := tr.Count()
		if err != nil {
			t.Fatal(err)
		}
		if n != 1 {
			t.Fatalf("got count %d, want 1", n)
		}
	})

	t.Run("Get", func(t *testing.T) {
		req, resp, err := tr.Get("test-0001")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(req, "GET /api/123") {
			t.Fatalf("request missing expected content: %s", req)
		}
		if !strings.Contains(resp, "result") {
			t.Fatalf("response missing expected content: %s", resp)
		}
	})

	t.Run("Hosts", func(t *testing.T) {
		hosts, err := tr.Hosts()
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, h := range hosts {
			if h.Host == "example.com" {
				found = true
				if h.Count != 1 {
					t.Fatalf("got count %d for example.com, want 1", h.Count)
				}
			}
		}
		if !found {
			t.Fatal("example.com not found in hosts")
		}
	})
}

func TestTrafficSearchEmpty(t *testing.T) {
	t.Parallel()
	tr := openTestTraffic(t)

	results, err := tr.Search("nonexistent.invalid", 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Fatalf("got %d results, want 0", len(results))
	}
}

func TestTrafficGetNotFound(t *testing.T) {
	t.Parallel()
	tr := openTestTraffic(t)

	_, _, err := tr.Get("does-not-exist")
	if err == nil {
		t.Fatal("expected error for nonexistent exchange")
	}
}

func TestTrafficMultipleHosts(t *testing.T) {
	t.Parallel()
	tr := openTestTraffic(t)

	seedExchange(t, tr,
		"a-0001", "alpha.test", "GET", "/", "https://alpha.test/",
		200, "text/html", 0, 100,
		"GET / HTTP/1.1\nHost: alpha.test\n", nil,
		"HTTP 200\n", []byte("<html></html>"))

	seedExchange(t, tr,
		"b-0001", "beta.test", "POST", "/login", "https://beta.test/login",
		302, "text/html", 50, 0,
		"POST /login HTTP/1.1\nHost: beta.test\n", []byte("user=admin"),
		"HTTP 302\n", nil)

	seedExchange(t, tr,
		"b-0002", "beta.test", "GET", "/dashboard", "https://beta.test/dashboard",
		200, "text/html", 0, 200,
		"GET /dashboard HTTP/1.1\nHost: beta.test\n", nil,
		"HTTP 200\n", []byte("<html>dashboard</html>"))

	t.Run("CountTotal", func(t *testing.T) {
		n, err := tr.Count()
		if err != nil {
			t.Fatal(err)
		}
		if n != 3 {
			t.Fatalf("got count %d, want 3", n)
		}
	})

	t.Run("SearchByHost", func(t *testing.T) {
		results, err := tr.Search("beta.test", 0, 10)
		if err != nil {
			t.Fatal(err)
		}
		if len(results) != 2 {
			t.Fatalf("got %d results for beta.test, want 2", len(results))
		}
	})

	t.Run("HostCounts", func(t *testing.T) {
		hosts, err := tr.Hosts()
		if err != nil {
			t.Fatal(err)
		}
		counts := map[string]int{}
		for _, h := range hosts {
			counts[h.Host] = h.Count
		}
		if counts["alpha.test"] != 1 {
			t.Fatalf("alpha.test count = %d, want 1", counts["alpha.test"])
		}
		if counts["beta.test"] != 2 {
			t.Fatalf("beta.test count = %d, want 2", counts["beta.test"])
		}
	})
}

func TestPatternsTopPaths(t *testing.T) {
	t.Parallel()
	tr := openTestTraffic(t)
	now := time.Now().Unix()
	for i := 0; i < 5; i++ {
		id := "tp-" + strings.Repeat("0", 3) + string(rune('1'+i))
		seedExchangeAt(t, tr, id, "example.com", "GET", "/api/users", "https://example.com/api/users", 200, "application/json", 0, 100, "", nil, "HTTP 200\n", nil, now-int64(i))
	}
	for i := 0; i < 2; i++ {
		id := "tp2-" + string(rune('1'+i))
		seedExchangeAt(t, tr, id, "example.com", "POST", "/api/login", "https://example.com/api/login", 200, "application/json", 50, 0, "", nil, "HTTP 200\n", nil, now-int64(i))
	}
	p, err := tr.Patterns("")
	if err != nil {
		t.Fatal(err)
	}
	if len(p.TopPaths) == 0 {
		t.Fatal("expected at least one top path")
	}
	if p.TopPaths[0].Path != "/api/users" {
		t.Errorf("top path = %q, want /api/users", p.TopPaths[0].Path)
	}
	if p.TopPaths[0].Count != 5 {
		t.Errorf("top path count = %d, want 5", p.TopPaths[0].Count)
	}
}

func TestPatternsErrorHosts(t *testing.T) {
	t.Parallel()
	tr := openTestTraffic(t)
	now := time.Now().Unix()
	seedExchangeAt(t, tr, "e-01", "err.test", "GET", "/missing", "https://err.test/missing", 404, "text/html", 0, 100, "", nil, "HTTP 404\n", nil, now)
	seedExchangeAt(t, tr, "e-02", "err.test", "GET", "/broken", "https://err.test/broken", 500, "text/html", 0, 100, "", nil, "HTTP 500\n", nil, now)
	seedExchangeAt(t, tr, "e-03", "err.test", "GET", "/ok", "https://err.test/ok", 200, "text/html", 0, 100, "", nil, "HTTP 200\n", nil, now)
	seedExchangeAt(t, tr, "e-04", "clean.test", "GET", "/", "https://clean.test/", 200, "text/html", 0, 100, "", nil, "HTTP 200\n", nil, now)
	p, err := tr.Patterns("")
	if err != nil {
		t.Fatal(err)
	}
	if len(p.ErrorHosts) != 1 {
		t.Fatalf("got %d error hosts, want 1", len(p.ErrorHosts))
	}
	eh := p.ErrorHosts[0]
	if eh.Host != "err.test" {
		t.Errorf("error host = %q, want err.test", eh.Host)
	}
	if eh.Codes4x != 1 {
		t.Errorf("4xx count = %d, want 1", eh.Codes4x)
	}
	if eh.Codes5x != 1 {
		t.Errorf("5xx count = %d, want 1", eh.Codes5x)
	}
}

func TestPatternsNewEndpoints(t *testing.T) {
	t.Parallel()
	tr := openTestTraffic(t)
	seedExchangeAt(t, tr, "old-01", "app.test", "GET", "/old-path", "https://app.test/old-path", 200, "text/html", 0, 100, "", nil, "HTTP 200\n", nil, 1000)
	seedExchangeAt(t, tr, "old-02", "app.test", "GET", "/old-path", "https://app.test/old-path", 200, "text/html", 0, 100, "", nil, "HTTP 200\n", nil, 1100)
	seedExchangeAt(t, tr, "new-01", "app.test", "GET", "/new-endpoint", "https://app.test/new-endpoint", 200, "text/html", 0, 100, "", nil, "HTTP 200\n", nil, 2000)
	seedExchangeAt(t, tr, "new-02", "app.test", "GET", "/new-endpoint", "https://app.test/new-endpoint", 200, "text/html", 0, 100, "", nil, "HTTP 200\n", nil, 2010)
	p, err := tr.Patterns("")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, ne := range p.NewEndpoints {
		if ne.Path == "/new-endpoint" {
			found = true
			if ne.Count != 2 {
				t.Errorf("new endpoint count = %d, want 2", ne.Count)
			}
		}
	}
	if !found {
		t.Error("/new-endpoint not found in new endpoints")
	}
}

func TestPatternsEmpty(t *testing.T) {
	t.Parallel()
	tr := openTestTraffic(t)
	p, err := tr.Patterns("")
	if err != nil {
		t.Fatal(err)
	}
	if len(p.TopPaths) != 0 {
		t.Errorf("expected no top paths, got %d", len(p.TopPaths))
	}
	if len(p.ErrorHosts) != 0 {
		t.Errorf("expected no error hosts, got %d", len(p.ErrorHosts))
	}
	if len(p.NewEndpoints) != 0 {
		t.Errorf("expected no new endpoints, got %d", len(p.NewEndpoints))
	}
}
