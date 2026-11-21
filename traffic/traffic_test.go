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
	db := tr.DB()
	_, err := db.Exec(
		`INSERT INTO exchanges(id,ts,host,method,url_template,url,status,content_type,req_len,resp_len,path)
		VALUES(?,?,?,?,?,?,?,?,?,?,?)`,
		id, time.Now().Unix(), host, method, tmpl, rawURL, status, ct, reqLen, respLen, "")
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
