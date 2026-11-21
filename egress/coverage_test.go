package egress

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

var testOpen = Profile{Name: "test", RPS: 1000, Burst: 1000, MaxPerHost: 100, MaxGlobal: 100, CacheTTL: time.Minute, MaxRetries: 2, BaseDelay: 10 * time.Millisecond, MaxDelay: 2 * time.Second}

func taskCtx(id string) context.Context {
	return context.WithValue(context.Background(), taskCtxKey{}, id)
}

type taskCtxKey struct{}

func testTaskKey(ctx context.Context) string {
	if v, ok := ctx.Value(taskCtxKey{}).(string); ok && v != "" {
		return "task:" + v
	}
	return ""
}

func testServer(t *testing.T, delay time.Duration, h func(n int64, w http.ResponseWriter, r *http.Request)) *testSrv {
	t.Helper()
	s := &testSrv{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n, cur := s.hits.Add(1), s.inflight.Add(1)
		for p := s.peak.Load(); cur > p && !s.peak.CompareAndSwap(p, cur); p = s.peak.Load() {
		}
		time.Sleep(delay)
		s.inflight.Add(-1)
		if h != nil {
			h(n, w, r)
			return
		}
		_, _ = io.WriteString(w, "ok")
	}))
	t.Cleanup(s.Close)
	return s
}

type testSrv struct {
	*httptest.Server
	hits, inflight, peak atomic.Int64
}

func doReq(t *testing.T, g *Governor, ctx context.Context, method, u string) (int, string) {
	t.Helper()
	req, _ := http.NewRequestWithContext(ctx, method, u, nil)
	resp, err := (&http.Client{Transport: g.Transport("test", nil)}).Do(req)
	if err != nil {
		t.Errorf("%s %s: %v", method, u, err)
		return 0, ""
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func runParallel(n int, f func(i int)) {
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() { defer wg.Done(); f(i) }()
	}
	wg.Wait()
}

func parseURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func saveProfile(t *testing.T) {
	t.Helper()
	prev := CurrentProfile()
	t.Cleanup(func() { SetProfile(prev) })
}

// --- ProfileByName ---

func TestProfileByNameControlCharInUA(t *testing.T) {
	t.Setenv("ARTEX_TARGET_USER_AGENT", "bad\x01agent")
	p, ok := ProfileByName("quiet")
	if !ok || p.Name != "quiet" {
		t.Fatalf("ProfileByName(quiet) = %+v %v", p, ok)
	}
	if p.UserAgent != "" {
		t.Fatalf("control-char UA should be ignored, got %q", p.UserAgent)
	}
}

func TestProfileByNameUnknown(t *testing.T) {
	if _, ok := ProfileByName(""); ok {
		t.Fatal(`ProfileByName("") accepted`)
	}
	if _, ok := ProfileByName("loud"); ok {
		t.Fatal(`ProfileByName("loud") accepted`)
	}
}

func TestProfileFromEnvUnknown(t *testing.T) {
	t.Setenv("ARTEX_NOISE_PROFILE", "nope")
	p := ProfileFromEnv()
	if p.Name != "off" || p.RPS != 0 {
		t.Fatalf("unknown profile: %+v, want off", p)
	}
}

// --- acquire context cancellation ---

func TestAcquireContextCancelledOnHostSlot(t *testing.T) {
	g := New(Profile{Name: "test", RPS: 100, Burst: 100, MaxPerHost: 1, MaxGlobal: 10})
	release, err := g.acquire(context.Background(), "a.example")
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = g.acquire(ctx, "a.example")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestAcquireContextCancelledOnGlobalSlot(t *testing.T) {
	g := New(Profile{Name: "test", RPS: 1000, Burst: 1000, MaxPerHost: 100, MaxGlobal: 1, Jitter: 0})
	release, err := g.acquire(context.Background(), "a.example")
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err = g.acquire(ctx, "b.example")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected DeadlineExceeded waiting for global slot, got %v", err)
	}
}

// --- backoff ---

func TestBackoffBaseDelayOverflow(t *testing.T) {
	g := New(Profile{Name: "test", RPS: 10, Burst: 10, MaxPerHost: 2, MaxGlobal: 10,
		BaseDelay: time.Second, MaxDelay: 5 * time.Second, MaxRetries: 3})
	d, ok := g.backoff(http.Header{}, 60)
	if !ok || d != 5*time.Second {
		t.Fatalf("backoff at high attempt: %s %v, want MaxDelay (5s) true", d, ok)
	}
}

func TestBackoffHTTPDate(t *testing.T) {
	g := New(testOpen)
	h := http.Header{"Retry-After": {time.Now().Add(2 * time.Second).UTC().Format(http.TimeFormat)}}
	d, ok := g.backoff(h, 0)
	if !ok || d <= 0 || d > 2*time.Second {
		t.Fatalf("HTTP-date Retry-After: %s, %v", d, ok)
	}
}

func TestBackoffOverflowRetryAfter(t *testing.T) {
	g := New(testOpen)
	d, ok := g.backoff(http.Header{"Retry-After": {"9223372036854775807"}}, 0)
	if ok || d != 0 {
		t.Fatalf("overflow Retry-After: %s %v, want refuse", d, ok)
	}
}

// --- report / Snapshot ---

func TestReportSkipsIdleHosts(t *testing.T) {
	g := New(testOpen)
	g.host("active.example")
	g.host("idle.example")
	g.mu.Lock()
	g.hosts["active.example"].sent = 5
	g.mu.Unlock()
	lines := g.report()
	if len(lines) != 1 {
		t.Fatalf("report lines = %d, want 1 (idle host skipped)", len(lines))
	}
	if !strings.Contains(lines[0], "active.example") {
		t.Fatalf("report line = %q, want active.example", lines[0])
	}
}

func TestSnapshotSkipsAllZeroHost(t *testing.T) {
	g := New(testOpen)
	g.host("zero.example")
	if s := g.Snapshot(); len(s) != 0 {
		t.Fatalf("Snapshot with all-zero host: %+v, want empty", s)
	}
}

func TestSnapshotSortOrder(t *testing.T) {
	g := New(testOpen)
	for _, host := range []string{"c.example", "a.example", "b.example"} {
		h := g.host(host)
		g.mu.Lock()
		h.sent = 1
		g.mu.Unlock()
	}
	s := g.Snapshot()
	if len(s) != 3 || s[0].Host != "a.example" || s[1].Host != "b.example" || s[2].Host != "c.example" {
		t.Fatalf("Snapshot not sorted: %+v", s)
	}
}

func TestSnapshotDoesNotResetCounters(t *testing.T) {
	g := New(testOpen)
	srv := testServer(t, 0, nil)
	tr := g.Transport("ns", nil)
	for range 3 {
		req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
		resp, err := tr.RoundTrip(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
	}
	want := HostNoise{Host: HostKey(parseURL(t, srv.URL)), RPM: 3, PeakRPM: 3, PeakInflight: 1}
	for range 2 {
		if s := g.Snapshot(); len(s) != 1 || s[0] != want {
			t.Fatalf("snapshot = %+v, want [%+v]", s, want)
		}
	}
	g.report()
	if s := g.Snapshot(); len(s) != 1 || s[0].RPM != 0 || s[0].PeakRPM != 3 {
		t.Fatalf("snapshot after report = %+v, want rpm 0 and peak 3", s)
	}
}

// --- share ---

func TestShareContextCancelledWaiting(t *testing.T) {
	g := New(testOpen)
	g.flights = map[string]*flight{}
	slow := &flight{done: make(chan struct{})}
	g.flights["key"] = slow

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := g.share(ctx, "h.example", "key", func() (any, time.Duration, error) {
		return "v", 0, nil
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("share waiting on flight: %v, want DeadlineExceeded", err)
	}
	close(slow.done)
}

func TestShareFollowerRetriesOnCancelledLeader(t *testing.T) {
	g := New(testOpen)
	g.flights = map[string]*flight{}

	f := &flight{done: make(chan struct{}), err: context.Canceled}
	close(f.done)
	g.flights["key"] = f

	called := false
	v, err := g.share(context.Background(), "h.example", "key", func() (any, time.Duration, error) {
		called = true
		return "retry-result", 0, nil
	})
	if !called {
		t.Fatal("follower should re-run fn when leader was cancelled")
	}
	if err != nil || v != "retry-result" {
		t.Fatalf("share follower retry: v=%v err=%v", v, err)
	}
}

func TestShareFollowerRetriesOnNotShared(t *testing.T) {
	g := New(testOpen)
	g.flights = map[string]*flight{}

	f := &flight{done: make(chan struct{}), err: errNotShared}
	close(f.done)
	g.flights["key2"] = f

	called := false
	v, err := g.share(context.Background(), "h.example", "key2", func() (any, time.Duration, error) {
		called = true
		return "my-result", 0, nil
	})
	if !called {
		t.Fatal("follower should re-run fn when leader returned errNotShared")
	}
	if err != nil || v != "my-result" {
		t.Fatalf("share follower retry on errNotShared: v=%v err=%v", v, err)
	}
}

// --- getTaskKey ---

func TestGetTaskKeyNilFunc(t *testing.T) {
	g := New(testOpen)
	if k := g.getTaskKey(context.Background()); k != "" {
		t.Fatalf("getTaskKey with nil func = %q, want empty", k)
	}
}

func TestGetTaskKeyWithFunc(t *testing.T) {
	g := New(testOpen, WithTaskKey(testTaskKey))
	ctx := taskCtx("abc")
	if k := g.getTaskKey(ctx); k != "task:abc" {
		t.Fatalf("getTaskKey = %q, want task:abc", k)
	}
}

// --- Do ---

func TestDoAcquireError(t *testing.T) {
	g := New(Profile{Name: "test", RPS: 100, Burst: 100, MaxPerHost: 1, MaxGlobal: 1})
	release, err := g.acquire(context.Background(), "a.example")
	if err != nil {
		t.Fatal(err)
	}
	defer release()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	u := parseURL(t, "http://a.example/")
	_, err = Do(g, ctx, u, func() (int, error) {
		t.Fatal("fn should not be called")
		return 0, nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Do with cancelled context: %v, want context.Canceled", err)
	}
}

// --- transport errors ---

type failRT struct{ err error }

func (f *failRT) RoundTrip(*http.Request) (*http.Response, error) { return nil, f.err }

func TestSendBaseTransportError(t *testing.T) {
	g := New(testOpen)
	base := &failRT{err: errors.New("connection refused")}
	tr := g.Transport("test", base)
	req := httptest.NewRequest(http.MethodGet, "http://a.example/", nil)
	_, err := tr.RoundTrip(req)
	if err == nil || !strings.Contains(err.Error(), "connection refused") {
		t.Fatalf("send with failing base: %v", err)
	}
}

func TestRoundTripShareError(t *testing.T) {
	g := New(testOpen, WithTaskKey(testTaskKey))
	base := &failRT{err: errors.New("dial failed")}
	tr := g.Transport("test", base)
	ctx := taskCtx("t1")
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://a.example/path", nil)
	_, err := tr.RoundTrip(req)
	if err == nil || !strings.Contains(err.Error(), "dial failed") {
		t.Fatalf("RoundTrip with GET+task and failing transport: %v", err)
	}
}

func TestSendAcquireError(t *testing.T) {
	g := New(Profile{Name: "test", RPS: 100, Burst: 100, MaxPerHost: 1, MaxGlobal: 1})
	tr := g.Transport("test", nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "http://a.example/", nil)
	_, err := tr.RoundTrip(req)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("send acquire with cancelled context: %v", err)
	}
}

// --- 429/503 not cached ---

func TestCachedResponseNotCachedOn429(t *testing.T) {
	p := testOpen
	p.CacheTTL = time.Minute
	p.MaxRetries = 0
	g := New(p, WithTaskKey(testTaskKey))
	var calls int
	srv := testServer(t, 0, func(_ int64, w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusTooManyRequests)
	})
	ctx := taskCtx("t1")
	doReq(t, g, ctx, "GET", srv.URL+"/rate")
	doReq(t, g, ctx, "GET", srv.URL+"/rate")
	if calls != 2 {
		t.Fatalf("429 response cached: calls = %d, want 2", calls)
	}
}

func TestCachedResponseNotCachedOn500(t *testing.T) {
	p := testOpen
	p.CacheTTL = time.Minute
	p.MaxRetries = 0
	g := New(p, WithTaskKey(testTaskKey))
	var calls int
	srv := testServer(t, 0, func(_ int64, w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusInternalServerError)
	})
	ctx := taskCtx("t1")
	doReq(t, g, ctx, "GET", srv.URL+"/err")
	doReq(t, g, ctx, "GET", srv.URL+"/err")
	if calls != 2 {
		t.Fatalf("500 response cached: calls = %d, want 2", calls)
	}
}

// --- HEAD/OPTIONS retries ---

func TestHeadAndOptionsRetried(t *testing.T) {
	p := testOpen
	p.MaxRetries = 2
	g := New(p)
	var headHits, optHits int
	srv := testServer(t, 0, func(_ int64, w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodHead:
			headHits++
			if headHits <= 1 {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
		case http.MethodOptions:
			optHits++
			if optHits <= 1 {
				w.WriteHeader(http.StatusTooManyRequests)
				return
			}
		}
	})
	code, _ := doReq(t, g, context.Background(), "HEAD", srv.URL+"/head")
	if code != 200 || headHits != 2 {
		t.Fatalf("HEAD retry: code %d, hits %d, want 200 and 2", code, headHits)
	}
	code, _ = doReq(t, g, context.Background(), "OPTIONS", srv.URL+"/opts")
	if code != 200 || optHits != 2 {
		t.Fatalf("OPTIONS retry: code %d, hits %d, want 200 and 2", code, optHits)
	}
}

// --- aggressive profile ---

func TestAggressiveProfile(t *testing.T) {
	p := Profiles["aggressive"]
	g := New(p)
	srv := testServer(t, 10*time.Millisecond, nil)
	n := p.Burst + 2
	start := time.Now()
	runParallel(n, func(int) { doReq(t, g, context.Background(), "GET", srv.URL) })
	d := time.Since(start)
	if srv.hits.Load() != int64(n) || srv.peak.Load() > int64(p.MaxPerHost) {
		t.Errorf("aggressive: hits %d, peak %d; want %d, <= %d", srv.hits.Load(), srv.peak.Load(), n, p.MaxPerHost)
	}
	if d > 5*time.Second {
		t.Errorf("aggressive: took %s, want < 5s", d)
	}
}

// --- global concurrency ---

func TestGlobalConcurrencyCap(t *testing.T) {
	p := Profile{Name: "test", RPS: 1000, Burst: 1000, MaxPerHost: 100, MaxGlobal: 2, Jitter: 0}
	g := New(p)
	var globalPeak atomic.Int64
	var globalCur atomic.Int64
	srv := testServer(t, 50*time.Millisecond, func(_ int64, w http.ResponseWriter, _ *http.Request) {
		cur := globalCur.Add(1)
		for {
			pk := globalPeak.Load()
			if cur <= pk || globalPeak.CompareAndSwap(pk, cur) {
				break
			}
		}
		time.Sleep(30 * time.Millisecond)
		globalCur.Add(-1)
	})
	runParallel(6, func(i int) {
		doReq(t, g, context.Background(), "GET", srv.URL+fmt.Sprintf("/g%d", i))
	})
	if pk := globalPeak.Load(); pk > int64(p.MaxGlobal) {
		t.Fatalf("global peak = %d, want <= %d", pk, p.MaxGlobal)
	}
}

// --- large response not shared ---

func TestLargeResponseNotSharedReleasesSlots(t *testing.T) {
	p := testOpen
	p.MaxPerHost = 1
	g := New(p, WithTaskKey(testTaskKey))
	big := strings.Repeat("x", 3<<20)
	srv := testServer(t, 0, func(_ int64, w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, big) })
	for i := range 3 {
		ctx, cancel := context.WithTimeout(taskCtx("t1"), 5*time.Second)
		if _, body := doReq(t, g, ctx, "GET", srv.URL); len(body) != len(big) {
			t.Fatalf("GET %d: body %d bytes, want %d", i+1, len(body), len(big))
		}
		cancel()
	}
	if srv.hits.Load() != 3 {
		t.Fatalf("hits %d, want 3", srv.hits.Load())
	}
}

// --- dedup and cache ---

func TestDedupeAndCache(t *testing.T) {
	p := testOpen
	p.CacheTTL = 200 * time.Millisecond
	g := New(p, WithTaskKey(testTaskKey))
	srv := testServer(t, 50*time.Millisecond, nil)
	runParallel(5, func(int) {
		if _, body := doReq(t, g, taskCtx("t1"), "GET", srv.URL); body != "ok" {
			t.Errorf("body = %q", body)
		}
	})
	doReq(t, g, taskCtx("t1"), "GET", srv.URL) // cached
	if srv.hits.Load() != 1 {
		t.Fatalf("coalesced and cached GETs: hits %d, want 1", srv.hits.Load())
	}
	doReq(t, g, taskCtx("t2"), "GET", srv.URL)        // another task
	doReq(t, g, context.Background(), "GET", srv.URL) // no task key func match
	time.Sleep(250 * time.Millisecond)
	doReq(t, g, taskCtx("t1"), "GET", srv.URL) // expired
	if srv.hits.Load() != 4 {
		t.Fatalf("other task, no task, expired: hits %d, want 4", srv.hits.Load())
	}
}

// --- UserAgent ---

func TestUserAgentSet(t *testing.T) {
	var got atomic.Value
	srv := testServer(t, 0, func(_ int64, _ http.ResponseWriter, r *http.Request) { got.Store(r.UserAgent()) })
	p := testOpen
	doReq(t, New(p), context.Background(), "GET", srv.URL)
	if ua := got.Load().(string); !strings.HasPrefix(ua, "Go-http-client/") {
		t.Fatalf("unset: User-Agent = %q, want the Go default", ua)
	}
	p.UserAgent = "engagement-ua"
	doReq(t, New(p), context.Background(), "GET", srv.URL)
	if ua := got.Load().(string); ua != "engagement-ua" {
		t.Fatalf("set: User-Agent = %q", ua)
	}
}

// --- HostKey ---

func TestHostKey(t *testing.T) {
	for in, want := range map[string]string{
		"http://Example.COM:80/x": "example.com",
		"https://a.test:8443/":    "a.test:8443",
		"https://b.test:443/":     "b.test",
		"http://c.test/":          "c.test",
	} {
		if u, _ := url.Parse(in); HostKey(u) != want {
			t.Errorf("HostKey(%s) = %q, want %q", in, HostKey(u), want)
		}
	}
}

// --- jitter ---

func TestJitterBounded(t *testing.T) {
	for name, want := range map[string]time.Duration{"quiet": 250 * time.Millisecond, "balanced": 40 * time.Millisecond, "aggressive": 5 * time.Millisecond} {
		g := New(Profiles[name])
		if b := g.jitterBound(); b != want {
			t.Fatalf("%s: bound %s, want %s", name, b, want)
		}
		var nonzero bool
		for range 1000 {
			d := g.jitter()
			if d < 0 || d > want {
				t.Fatalf("%s: jitter %s outside [0, %s]", name, d, want)
			}
			nonzero = nonzero || d > 0
		}
		if !nonzero {
			t.Fatalf("%s: jitter always 0", name)
		}
	}
}

// --- SetProfile / Current ---

func TestSetProfileConcurrent(t *testing.T) {
	saveProfile(t)
	names := []string{"off", "quiet", "balanced", "aggressive"}
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range 200 {
				p, _ := ProfileByName(names[(i+j)%len(names)])
				SetProfile(p)
				if g := Current(); g == nil || g.Profile().Name == "" {
					t.Errorf("Current() = %v during swaps", g)
					return
				}
			}
		}()
	}
	wg.Wait()
}

func TestSetProfileInFlightFinishesOnOld(t *testing.T) {
	saveProfile(t)
	old := SetProfile(testOpen)
	release, err := old.acquire(t.Context(), "a.example")
	if err != nil {
		t.Fatal(err)
	}
	fresh := SetProfile(Profiles["balanced"])
	if Current() != fresh || len(fresh.Snapshot()) != 0 {
		t.Fatalf("new governor: current=%p fresh=%p snapshot=%v", Current(), fresh, fresh.Snapshot())
	}
	if s := old.Snapshot(); len(s) != 1 || s[0].Inflight != 1 {
		t.Fatalf("old governor before release: %+v", s)
	}
	release()
	if s := old.Snapshot(); len(s) != 1 || s[0].Inflight != 0 || s[0].RPM != 1 || s[0].PeakInflight != 1 {
		t.Fatalf("old governor after release: %+v", s)
	}
}

// --- Dynamic ---

func TestDynamicFollowsSetProfile(t *testing.T) {
	saveProfile(t)
	srv := testServer(t, 0, nil)
	SetProfile(Profile{Name: "off"})
	client := &http.Client{Transport: Dynamic("ns", nil)}
	get := func() {
		t.Helper()
		resp, err := client.Get(srv.URL)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
	}
	get()
	g := SetProfile(testOpen)
	get()
	get()
	if s := g.Snapshot(); len(s) != 1 || s[0].RPM != 2 {
		t.Fatalf("snapshot after switch: %+v", s)
	}
}

// --- pace.go ---

func TestMarkProxyURL(t *testing.T) {
	for in, want := range map[string]string{
		"http://127.0.0.1:8788":            "http://artex-pace:artex-pace@127.0.0.1:8788",
		"http://u:p@127.0.0.1:8788":        "http://u:p@127.0.0.1:8788",
		"":                                 "",
		"127.0.0.1:8788":                   "127.0.0.1:8788",
		"socks5://proxy.example.test:1080": "socks5://artex-pace:artex-pace@proxy.example.test:1080",
	} {
		if got := MarkProxyURL(in); got != want {
			t.Errorf("MarkProxyURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestIsPaceMarker(t *testing.T) {
	enc := func(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }
	for v, want := range map[string]bool{
		"Basic " + enc("artex-pace:artex-pace"):  true,
		"basic " + enc("artex-pace:x"):           true,
		"Basic " + enc("user:pass"):              false,
		"Bearer " + enc("artex-pace:artex-pace"): false,
		"Basic !!!":                              false,
		"Basic":                                  false,
		"":                                       false,
	} {
		if got := IsPaceMarker(v); got != want {
			t.Errorf("IsPaceMarker(%q) = %v, want %v", v, got, want)
		}
	}
}

func TestWait(t *testing.T) {
	if err := New(Profile{Name: "off"}).Wait(t.Context(), "a.example"); err != nil {
		t.Fatalf("off Wait: %v", err)
	}
	off := New(Profile{Name: "off"})
	_ = off.Wait(t.Context(), "a.example")
	if s := off.Snapshot(); len(s) != 0 {
		t.Fatalf("off counted: %v", s)
	}

	g := New(Profile{Name: "test", RPS: 10, Burst: 1, MaxPerHost: 1, MaxGlobal: 1})
	start := time.Now()
	for range 2 {
		if err := g.Wait(t.Context(), "a.example"); err != nil {
			t.Fatal(err)
		}
	}
	if d := time.Since(start); d < 80*time.Millisecond {
		t.Fatalf("two Waits at 10 req/s took %v", d)
	}
	if s := g.Snapshot(); len(s) != 1 || s[0].RPM != 2 || s[0].Inflight != 0 || s[0].PeakInflight != 0 {
		t.Fatalf("snapshot after Wait = %+v, want rpm 2 and no in-flight slot", s)
	}

	slow := New(Profile{Name: "test", RPS: 0.5, Burst: 1, MaxPerHost: 1, MaxGlobal: 1})
	_ = slow.Wait(t.Context(), "b.example")
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	if err := slow.Wait(ctx, "b.example"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("bounded Wait: %v, want DeadlineExceeded", err)
	}
	if s := slow.Snapshot(); len(s) != 1 || s[0].RPM != 2 {
		t.Fatalf("snapshot after a timed-out Wait = %+v, want rpm 2", s)
	}

	saveProfile(t)
	SetProfile(Profile{Name: "off"})
	if err := Wait(t.Context(), "c.example"); err != nil || len(Current().Snapshot()) != 0 {
		t.Fatalf("package Wait under off: err=%v snapshot=%v", err, Current().Snapshot())
	}
}

// --- off is strict passthrough ---

func TestOffIsStrictPassthrough(t *testing.T) {
	g := New(Profile{Name: "off"})
	if g.Transport("test", http.DefaultTransport) != http.DefaultTransport {
		t.Fatal("off: Transport must return the base transport itself")
	}
	srv := testServer(t, 50*time.Millisecond, nil)
	start := time.Now()
	runParallel(20, func(int) { doReq(t, g, context.Background(), "GET", srv.URL) })
	if d := time.Since(start); srv.hits.Load() != 20 || srv.peak.Load() < 10 || d > time.Second {
		t.Fatalf("off: hits %d, peak %d, %s; want 20 parallel hits, no delay", srv.hits.Load(), srv.peak.Load(), d)
	}
	var calls atomic.Int64
	u, _ := url.Parse(srv.URL)
	for range 2 {
		_, _ = Do(g, context.Background(), u, func() (int, error) { calls.Add(1); return 0, nil })
	}
	if calls.Load() != 2 || len(g.report()) != 0 {
		t.Fatalf("off: Do calls = %d, report %v; want 2 calls, no counter", calls.Load(), g.report())
	}
}

func TestDoSuccess(t *testing.T) {
	g := New(testOpen)
	u := parseURL(t, "http://a.example/")
	val, err := Do(g, context.Background(), u, func() (string, error) {
		return "result", nil
	})
	if err != nil || val != "result" {
		t.Fatalf("Do success: val=%q err=%v", val, err)
	}
}

// --- WithTaskKey option ---

func TestWithTaskKeyOption(t *testing.T) {
	fn := func(ctx context.Context) string { return "fixed" }
	g := New(testOpen, WithTaskKey(fn))
	if g.taskKey == nil {
		t.Fatal("WithTaskKey did not set taskKey")
	}
	if k := g.getTaskKey(context.Background()); k != "fixed" {
		t.Fatalf("getTaskKey = %q, want 'fixed'", k)
	}
}

// --- namespace isolation ---

func TestNamespaceIsolation(t *testing.T) {
	p := testOpen
	p.CacheTTL = time.Minute
	g := New(p, WithTaskKey(testTaskKey))
	srv := testServer(t, 0, nil)
	for _, ns := range []string{"tool-a", "tool-b", "tool-a"} {
		req, _ := http.NewRequestWithContext(taskCtx("t9"), "GET", srv.URL+"/ns", nil)
		resp, err := (&http.Client{Transport: g.Transport(ns, nil)}).Do(req)
		if err == nil {
			resp.Body.Close()
		}
	}
	if srv.hits.Load() != 2 {
		t.Fatalf("namespaces: hits %d, want 2 (one per namespace, then a cache hit)", srv.hits.Load())
	}
}
