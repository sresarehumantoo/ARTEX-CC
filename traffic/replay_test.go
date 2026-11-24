package traffic

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestReplayNonceLifecycle(t *testing.T) {
	t.Parallel()

	nonce, err := registerReplayNonce()
	if err != nil {
		t.Fatal(err)
	}
	if nonce == "" {
		t.Fatal("nonce should not be empty")
	}
	replayNonces.mu.Lock()
	count := len(replayNonces.live)
	replayNonces.mu.Unlock()
	if count < 1 {
		t.Fatal("count should be >= 1 after register")
	}

	if !consumeReplayNonce(nonce) {
		t.Fatal("first consume should succeed")
	}
	if consumeReplayNonce(nonce) {
		t.Fatal("second consume should fail")
	}
}

func TestConsumeReplayNonceEmpty(t *testing.T) {
	t.Parallel()
	if consumeReplayNonce("") {
		t.Fatal("empty string should return false")
	}
}

func TestDeleteReplayNonce(t *testing.T) {
	t.Parallel()
	nonce, err := registerReplayNonce()
	if err != nil {
		t.Fatal(err)
	}
	replayNonces.mu.Lock()
	before := len(replayNonces.live)
	replayNonces.mu.Unlock()
	deleteReplayNonce(nonce)
	replayNonces.mu.Lock()
	after := len(replayNonces.live)
	replayNonces.mu.Unlock()
	if after >= before {
		t.Fatalf("count should decrease: before=%d after=%d", before, after)
	}
	if consumeReplayNonce(nonce) {
		t.Fatal("consume after delete should fail")
	}
}

func TestReplayInvalidWrapsError(t *testing.T) {
	t.Parallel()
	err := replayInvalid("bad %s", "thing")
	if !errors.Is(err, ErrReplayInvalid) {
		t.Fatalf("expected ErrReplayInvalid, got %v", err)
	}
	if !strings.Contains(err.Error(), "bad thing") {
		t.Fatalf("message = %q", err.Error())
	}
}

func TestReplayValidation(t *testing.T) {
	t.Parallel()
	tr := openTestTraffic(t)
	ctx := context.Background()

	cases := []struct {
		name    string
		req     ReplayRequest
		wantErr string
	}{
		{"empty_method", ReplayRequest{Method: "", URL: "https://x.test/"}, "method must be set"},
		{"connect_method", ReplayRequest{Method: "CONNECT", URL: "https://x.test/"}, "CONNECT"},
		{"invalid_method_token", ReplayRequest{Method: "GET POST", URL: "https://x.test/"}, "not a valid token"},
		{"ftp_scheme", ReplayRequest{Method: "GET", URL: "ftp://x.test/"}, "http or https"},
		{"no_host", ReplayRequest{Method: "GET", URL: "http://"}, "no host"},
		{"userinfo", ReplayRequest{Method: "GET", URL: "https://user:pass@x.test/"}, "user info"},
		{"body_too_large", ReplayRequest{Method: "POST", URL: "https://x.test/", Body: make([]byte, maxReplayBody+1)}, "too large"},
		{"too_many_headers", ReplayRequest{Method: "GET", URL: "https://x.test/", Headers: makeHeaders(maxReplayHeaders + 1)}, "over the limit"},
		{"upgrade_header", ReplayRequest{Method: "GET", URL: "https://x.test/", Headers: []ReplayHeader{{Name: "Upgrade", Value: "websocket"}}}, "Upgrade"},
		{"host_mismatch", ReplayRequest{Method: "GET", URL: "https://x.test/", Headers: []ReplayHeader{{Name: "Host", Value: "y.test"}}}, "differs from the URL host"},
		{"invalid_header_name", ReplayRequest{Method: "GET", URL: "https://x.test/", Headers: []ReplayHeader{{Name: "Bad Header", Value: "v"}}}, "invalid header name"},
		{"header_newline", ReplayRequest{Method: "GET", URL: "https://x.test/", Headers: []ReplayHeader{{Name: "X-Ok", Value: "a\nb"}}}, "line break"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tr.Replay(ctx, tc.req)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("err = %v, want substring %q", err, tc.wantErr)
			}
		})
	}
}

func makeHeaders(n int) []ReplayHeader {
	out := make([]ReplayHeader, n)
	for i := range out {
		out[i] = ReplayHeader{Name: "X-Test", Value: "v"}
	}
	return out
}

func TestReplayDroppedKeys(t *testing.T) {
	t.Parallel()
	for _, key := range []string{
		"Content-Length", "Transfer-Encoding", "Connection",
		"Proxy-Connection", "Proxy-Authorization",
		"Keep-Alive", "Te", "Trailer", "Expect",
	} {
		if !replayDropped[key] {
			t.Errorf("replayDropped missing %q", key)
		}
	}
}

func TestReplayHostMatchOK(t *testing.T) {
	t.Parallel()
	tr := openTestTraffic(t)
	ctx := context.Background()

	// Host matching the URL host should pass the Host validation and fail later
	// (at replayClient) since there is no real proxy.
	_, err := tr.Replay(ctx, ReplayRequest{
		Method:  "GET",
		URL:     "https://example.test:443/",
		Headers: []ReplayHeader{{Name: "Host", Value: "example.test:443"}},
	})
	// The error should NOT be about Host mismatch; it should be about the proxy.
	if err != nil && strings.Contains(err.Error(), "differs from the URL host") {
		t.Fatalf("matching Host should not cause mismatch error: %v", err)
	}
}
