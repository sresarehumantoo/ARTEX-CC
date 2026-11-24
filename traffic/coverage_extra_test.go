package traffic

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/SREsAreHumanToo/ARTEX-CC/traffic/hookvm"
	"github.com/SREsAreHumanToo/ARTEX-CC/traffic/rewrite"
)

func TestClassifyHookErr(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"panic", &hookvm.PanicError{Value: "boom"}, rewrite.ErrClassPanic},
		{"timeout", hookvm.ErrTimeout, "timeout"},
		{"step_limit", hookvm.ErrStepLimit, "step_limit"},
		{"memory_limit", hookvm.ErrMemoryLimit, "memory_limit"},
		{"forbidden_mutation", hookvm.ErrForbiddenMutation, "forbidden_mutation"},
		{"call_depth", hookvm.ErrCallDepth, "call_depth"},
		{"generic", errors.New("something"), "hook_error"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyHookErr(tc.err); got != tc.want {
				t.Fatalf("classifyHookErr(%v) = %q, want %q", tc.err, got, tc.want)
			}
		})
	}
}

func TestHookErrorTable(t *testing.T) {
	t.Parallel()
	var tbl hookErrorTable
	if got := tbl.get(1); got != nil {
		t.Fatal("expected nil for unrecorded id")
	}
	tbl.record(1, "timeout")
	got := tbl.get(1)
	if got == nil {
		t.Fatal("expected non-nil")
	}
	if got.Class != "timeout" || got.Count != 1 {
		t.Fatalf("got class=%q count=%d", got.Class, got.Count)
	}
	tbl.record(1, "panic")
	got = tbl.get(1)
	if got.Class != "panic" || got.Count != 2 {
		t.Fatalf("second: class=%q count=%d", got.Class, got.Count)
	}
	tbl.record(0, "ignored")
	if tbl.get(0) != nil {
		t.Fatal("id 0 should be ignored")
	}
}

func TestHookErrorTableNilReceiver(t *testing.T) {
	t.Parallel()
	var tbl *hookErrorTable
	tbl.record(1, "timeout")
	if got := tbl.get(1); got != nil {
		t.Fatal("expected nil on nil receiver")
	}
}

func TestHookTraceSkip(t *testing.T) {
	t.Parallel()
	h := &compiledHook{id: 42, version: 3, snapshot: `{"name":"test"}`, phase: rewrite.PhaseRequest}
	line := hookTraceSkip(h, rewrite.PhaseRequest, "stream")
	if line.Outcome != rewrite.OutcomeSkipped {
		t.Fatalf("outcome = %q", line.Outcome)
	}
	if line.HookID != 42 || line.HookVersion != 3 {
		t.Fatalf("hook id=%d ver=%d", line.HookID, line.HookVersion)
	}
	if line.Reason != "stream" {
		t.Fatalf("reason = %q", line.Reason)
	}
}

func TestHookTraceError(t *testing.T) {
	t.Parallel()
	h := &compiledHook{id: 7, version: 1, snapshot: `{"name":"test"}`, phase: rewrite.PhaseResponse}
	line := hookTraceError(h, rewrite.PhaseResponse, "timeout")
	if line.Outcome != rewrite.OutcomeError {
		t.Fatalf("outcome = %q", line.Outcome)
	}
	if line.Reason != "timeout" {
		t.Fatalf("reason = %q", line.Reason)
	}
}

func TestCloneHeaderMap(t *testing.T) {
	t.Parallel()
	rawKey := "content-type"
	orig := http.Header{rawKey: {"text/html"}, "X-Custom": {"a", "b"}}
	cloned := cloneHeaderMap(orig)
	if len(cloned) != 2 {
		t.Fatalf("got %d keys", len(cloned))
	}
	if v := cloned["Content-Type"]; len(v) != 1 || v[0] != "text/html" {
		t.Fatalf("Content-Type = %v", v)
	}
	if v := cloned["X-Custom"]; len(v) != 2 {
		t.Fatalf("X-Custom = %v", v)
	}
	cloned["Content-Type"][0] = "changed"
	if orig[rawKey][0] == "changed" { //nolint:staticcheck // orig is built with a non-canonical key on purpose
		t.Fatal("mutation leaked to original")
	}
}

func TestCloneHeaderMapEmpty(t *testing.T) {
	t.Parallel()
	cloned := cloneHeaderMap(nil)
	if len(cloned) != 0 {
		t.Fatal("expected empty")
	}
}

func TestFormatHARResponseCookies(t *testing.T) {
	t.Parallel()
	t.Run("basic", func(t *testing.T) {
		got, err := formatHARResponseCookies([]HARCookie{
			{Name: "session", Value: "abc"},
		})
		if err != nil {
			t.Fatal(err)
		}
		if got != "Set-Cookie: session=abc\n" {
			t.Fatalf("got %q", got)
		}
	})
	t.Run("full", func(t *testing.T) {
		got, err := formatHARResponseCookies([]HARCookie{
			{Name: "id", Value: "xyz", Path: "/", Domain: ".example.com",
				Expires: "Thu, 01 Jan 2026 00:00:00 GMT", Secure: true, HTTPOnly: true},
		})
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{"Set-Cookie: id=xyz", "Path=/", "Domain=.example.com", "Expires=", "Secure", "HttpOnly"} {
			if !contains(got, want) {
				t.Fatalf("missing %q in %q", want, got)
			}
		}
	})
	t.Run("empty_name", func(t *testing.T) {
		_, err := formatHARResponseCookies([]HARCookie{{Name: "", Value: "v"}})
		if err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("invalid_name", func(t *testing.T) {
		_, err := formatHARResponseCookies([]HARCookie{{Name: "bad;name", Value: "v"}})
		if err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("empty_list", func(t *testing.T) {
		got, err := formatHARResponseCookies(nil)
		if err != nil || got != "" {
			t.Fatalf("got %q err %v", got, err)
		}
	})
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestRewriteStats(t *testing.T) {
	t.Parallel()
	tr := openTestTraffic(t)
	stats := tr.RewriteStats()
	if stats.LargeResponses != 0 || stats.CapacityRefusals != 0 {
		t.Fatalf("unexpected non-zero stats: %+v", stats)
	}
}

func TestRewriteStatsNilSnaps(t *testing.T) {
	t.Parallel()
	tr := &Traffic{}
	stats := tr.RewriteStats()
	if stats.LargeResponses != 0 {
		t.Fatal("expected zero")
	}
}

func TestHookBodyIneligible(t *testing.T) {
	t.Parallel()
	req := rewrite.Request{
		Method: "GET",
		Header: http.Header{},
		Body:   []byte("hello"),
	}
	resp := rewrite.Response{
		Status: 200,
		Header: http.Header{},
		Body:   []byte("world"),
	}
	in := rewrite.Input{
		Request:  req,
		Response: resp,
		IsBinary: func(ct string, b []byte) bool { return false },
	}

	t.Run("request_ok", func(t *testing.T) {
		if r := hookBodyIneligible(rewrite.PhaseRequest, in, req, resp); r != "" {
			t.Fatalf("got %q", r)
		}
	})
	t.Run("response_ok", func(t *testing.T) {
		if r := hookBodyIneligible(rewrite.PhaseResponse, in, req, resp); r != "" {
			t.Fatalf("got %q", r)
		}
	})
	t.Run("response_stream", func(t *testing.T) {
		sin := in
		sin.Stream = true
		if r := hookBodyIneligible(rewrite.PhaseResponse, sin, req, resp); r != rewrite.ReasonStream {
			t.Fatalf("got %q", r)
		}
	})
	t.Run("response_head_method", func(t *testing.T) {
		headReq := req
		headReq.Method = "HEAD"
		if r := hookBodyIneligible(rewrite.PhaseResponse, in, headReq, resp); r != rewrite.ReasonNoBodyAllowed {
			t.Fatalf("got %q", r)
		}
	})
	t.Run("request_binary", func(t *testing.T) {
		bin := in
		bin.IsBinary = func(string, []byte) bool { return true }
		if r := hookBodyIneligible(rewrite.PhaseRequest, bin, req, resp); r != rewrite.ReasonBinary {
			t.Fatalf("got %q", r)
		}
	})
	t.Run("request_content_encoding", func(t *testing.T) {
		ceReq := req
		ceReq.Header = http.Header{"Content-Encoding": {"gzip"}}
		if r := hookBodyIneligible(rewrite.PhaseRequest, in, ceReq, resp); r != rewrite.ReasonContentEncoding {
			t.Fatalf("got %q", r)
		}
	})
	t.Run("request_too_large", func(t *testing.T) {
		bigReq := req
		bigReq.Body = make([]byte, rewrite.MaxBodySize+1)
		if r := hookBodyIneligible(rewrite.PhaseRequest, in, bigReq, resp); r != rewrite.ReasonBodyTooLarge {
			t.Fatalf("got %q", r)
		}
	})
	t.Run("response_204", func(t *testing.T) {
		noBody := resp
		noBody.Status = 204
		if r := hookBodyIneligible(rewrite.PhaseResponse, in, req, noBody); r != rewrite.ReasonNoBodyAllowed {
			t.Fatalf("got %q", r)
		}
	})
}

func TestSnapTableConvertLocked(t *testing.T) {
	t.Parallel()
	st := newSnapTable()
	st.maxEntries = 2
	st.maxBytes = 100000
	st.ttl = 0

	done1 := make(chan struct{})
	e1 := &reqSnap{done: done1, cost: 10, rewritten: true,
		lines: []rewrite.TraceLine{{Phase: rewrite.PhaseRequest, RuleID: 5, Outcome: rewrite.OutcomeApplied}}}
	st.mu.Lock()
	st.slots[flowKey{1}] = e1
	e1.created = time.Now().Add(-2 * time.Minute)
	st.entries++
	st.bytes += e1.cost
	st.mu.Unlock()

	done2 := make(chan struct{})
	e2 := &reqSnap{done: done2, cost: 10}
	st.mu.Lock()
	st.slots[flowKey{2}] = e2
	e2.created = time.Now()
	st.entries++
	st.bytes += e2.cost
	st.mu.Unlock()

	st.mu.Lock()
	ok := st.convertLocked(10)
	st.mu.Unlock()
	if !ok {
		t.Fatal("convertLocked should succeed")
	}
	st.mu.Lock()
	entries, tombs := st.entries, st.tombs
	st.mu.Unlock()
	if entries != 1 || tombs != 1 {
		t.Fatalf("entries=%d tombs=%d", entries, tombs)
	}
}

func TestSnapTableEvictFinishedTombs(t *testing.T) {
	t.Parallel()
	st := newSnapTable()
	done := make(chan struct{})
	close(done)
	st.mu.Lock()
	st.slots[flowKey{1}] = &reqSnap{done: done, tomb: true, reason: reasonSnapshotCapacity}
	st.tombs++
	st.mu.Unlock()

	st.mu.Lock()
	st.evictFinishedTombsLocked()
	st.mu.Unlock()

	st.mu.Lock()
	tombs := st.tombs
	st.mu.Unlock()
	if tombs != 0 {
		t.Fatalf("tombs=%d after eviction", tombs)
	}
}

func TestToHeaderMap(t *testing.T) {
	t.Parallel()
	flat := map[string]string{"content-type": "text/html", "x-custom": "val"}
	m := toHeaderMap(flat)
	if len(m) != 2 {
		t.Fatalf("got %d", len(m))
	}
	if v := m["content-type"]; len(v) != 1 || v[0] != "text/html" {
		t.Fatalf("content-type = %v", v)
	}
}

func TestValidateExternalHeaders(t *testing.T) {
	t.Parallel()
	if err := validateExternalHeaders(map[string]string{"Content-Type": "text/html"}); err != nil {
		t.Fatal(err)
	}
	if err := validateExternalHeaders(map[string]string{"bad header": "val"}); err == nil {
		t.Fatal("expected error for invalid name")
	}
	if err := validateExternalHeaders(map[string]string{"X": "a\nb"}); err == nil {
		t.Fatal("expected error for newline")
	}
	if err := validateExternalHeaders(nil); err != nil {
		t.Fatal(err)
	}
}
