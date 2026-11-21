package egress

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestPassthroughProfile(t *testing.T) {
	t.Parallel()
	g := New(Profile{Name: "off"})
	if !g.off() {
		t.Fatal("off profile should report off()")
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "ok")
	}))
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	val, err := Do(g, context.Background(), u, func() (string, error) {
		return "pass", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if val != "pass" {
		t.Errorf("got %q, want %q", val, "pass")
	}
}

func TestConcurrencyLimit(t *testing.T) {
	t.Parallel()
	g := New(Profile{
		Name:       "test",
		RPS:        1000,
		Burst:      100,
		MaxPerHost: 2,
		MaxGlobal:  10,
	})

	var peak atomic.Int32
	var current atomic.Int32
	var wg sync.WaitGroup

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c := current.Add(1)
		for {
			p := peak.Load()
			if c <= p || peak.CompareAndSwap(p, c) {
				break
			}
		}
		time.Sleep(50 * time.Millisecond)
		current.Add(-1)
		fmt.Fprint(w, "ok")
	}))
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	tr := g.Transport("test", nil)
	client := &http.Client{Transport: tr}

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req, _ := http.NewRequest("GET", srv.URL, nil)
			resp, err := client.Do(req)
			if err != nil {
				t.Error(err)
				return
			}
			resp.Body.Close()
		}()
	}
	wg.Wait()

	_ = u
	if p := peak.Load(); p > 2 {
		t.Errorf("peak concurrency %d exceeded MaxPerHost 2", p)
	}
}

func TestHostBucketIsolation(t *testing.T) {
	t.Parallel()
	g := New(Profile{
		Name:       "test",
		RPS:        1000,
		Burst:      100,
		MaxPerHost: 1,
		MaxGlobal:  10,
	})

	srv1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		fmt.Fprint(w, "1")
	}))
	defer srv1.Close()

	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		fmt.Fprint(w, "2")
	}))
	defer srv2.Close()

	tr := g.Transport("test", nil)
	client := &http.Client{Transport: tr}

	start := time.Now()
	var wg sync.WaitGroup
	for _, srvURL := range []string{srv1.URL, srv2.URL} {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()
			req, _ := http.NewRequest("GET", url, nil)
			resp, err := client.Do(req)
			if err != nil {
				t.Error(err)
				return
			}
			resp.Body.Close()
		}(srvURL)
	}
	wg.Wait()
	elapsed := time.Since(start)

	if elapsed > 500*time.Millisecond {
		t.Errorf("requests to different hosts took %v; expected them to run concurrently", elapsed)
	}
}

func TestBackoffOn429(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if n <= 2 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		fmt.Fprint(w, "ok")
	}))
	defer srv.Close()

	g := New(Profile{
		Name:       "test",
		RPS:        1000,
		Burst:      100,
		MaxPerHost: 5,
		MaxGlobal:  10,
		MaxRetries: 3,
		BaseDelay:  10 * time.Millisecond,
		MaxDelay:   100 * time.Millisecond,
	})

	tr := g.Transport("test", nil)
	client := &http.Client{Transport: tr}

	req, _ := http.NewRequest("GET", srv.URL, nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200 after retries", resp.StatusCode)
	}
	if c := calls.Load(); c < 3 {
		t.Errorf("expected at least 3 calls (2 retries + success); got %d", c)
	}
}

func TestRetryAfterHeader(t *testing.T) {
	t.Parallel()
	g := New(Profile{
		Name:       "test",
		RPS:        1000,
		Burst:      100,
		MaxPerHost: 5,
		MaxGlobal:  10,
		MaxRetries: 2,
		BaseDelay:  time.Second,
		MaxDelay:   10 * time.Second,
	})

	h := http.Header{}
	h.Set("Retry-After", "2")
	d, ok := g.backoff(h, 0)
	if !ok {
		t.Fatal("backoff returned not ok for Retry-After: 2")
	}
	if d != 2*time.Second {
		t.Errorf("backoff = %v, want 2s", d)
	}

	h.Set("Retry-After", "999")
	_, ok = g.backoff(h, 0)
	if ok {
		t.Error("backoff should return not ok when Retry-After exceeds MaxDelay")
	}
}
