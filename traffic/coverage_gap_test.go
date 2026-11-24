package traffic

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"testing"

	"github.com/SREsAreHumanToo/ARTEX-CC/traffic/rewrite"
)

func TestSplitLegacyEvidence(t *testing.T) {
	t.Run("normal_headers_and_body", func(t *testing.T) {
		input := "Host: example.com\r\nContent-Type: text/html\r\n\r\nbody content"
		head, body, err := splitLegacyEvidence(strings.NewReader(input))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(head, "Host: example.com") {
			t.Fatalf("head = %q, want Host header", head)
		}
		b, _ := io.ReadAll(body)
		if string(b) != "body content" {
			t.Fatalf("body = %q, want %q", b, "body content")
		}
	})

	t.Run("lf_only_separator", func(t *testing.T) {
		input := "Host: example.com\n\nbody"
		head, body, err := splitLegacyEvidence(strings.NewReader(input))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(head, "Host: example.com") {
			t.Fatalf("head = %q", head)
		}
		b, _ := io.ReadAll(body)
		if string(b) != "body" {
			t.Fatalf("body = %q", b)
		}
	})

	t.Run("headers_only_eof", func(t *testing.T) {
		input := "Host: example.com"
		head, body, err := splitLegacyEvidence(strings.NewReader(input))
		if err != nil {
			t.Fatal(err)
		}
		if head != "Host: example.com" {
			t.Fatalf("head = %q", head)
		}
		b, _ := io.ReadAll(body)
		if len(b) != 0 {
			t.Fatalf("body = %q, want empty", b)
		}
	})

	t.Run("oversized_header_rejected", func(t *testing.T) {
		huge := strings.Repeat("X", 1<<20+100)
		_, _, err := splitLegacyEvidence(strings.NewReader(huge))
		if err == nil || !strings.Contains(err.Error(), "too large") {
			t.Fatalf("err = %v, want 'too large'", err)
		}
	})
}

func TestSplitLegacyHTTP(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		wantHead string
		wantBody string
	}{
		{"crlf", "GET / HTTP/1.1\r\nHost: x\r\n\r\nbody", "GET / HTTP/1.1\r\nHost: x", "body"},
		{"lf_only", "GET / HTTP/1.1\nHost: x\n\nbody", "GET / HTTP/1.1\nHost: x", "body"},
		{"no_body", "GET / HTTP/1.1\r\nHost: x", "GET / HTTP/1.1\r\nHost: x", ""},
		{"empty", "", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			head, body := splitLegacyHTTP([]byte(tc.input))
			if head != tc.wantHead {
				t.Fatalf("head = %q, want %q", head, tc.wantHead)
			}
			if string(body) != tc.wantBody {
				t.Fatalf("body = %q, want %q", body, tc.wantBody)
			}
		})
	}
}

func TestValidateHookDef(t *testing.T) {
	validReqSrc := `package hook
func OnRequest(req *Request) error { return nil }`
	validRespSrc := `package hook
func OnResponse(req *Request, resp *Response) error { return nil }`

	t.Run("valid_request_hook", func(t *testing.T) {
		err := validateHookDef(HookDef{
			Name:   "test-hook",
			Phase:  rewrite.PhaseRequest,
			Source: validReqSrc,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("valid_response_hook", func(t *testing.T) {
		err := validateHookDef(HookDef{
			Name:   "resp-hook",
			Phase:  rewrite.PhaseResponse,
			Source: validRespSrc,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("empty_name", func(t *testing.T) {
		err := validateHookDef(HookDef{
			Name:   "",
			Phase:  rewrite.PhaseRequest,
			Source: validReqSrc,
		})
		if err == nil || !strings.Contains(err.Error(), "name") {
			t.Fatalf("err = %v, want name error", err)
		}
	})

	t.Run("whitespace_name", func(t *testing.T) {
		err := validateHookDef(HookDef{
			Name:   "   ",
			Phase:  rewrite.PhaseRequest,
			Source: validReqSrc,
		})
		if err == nil {
			t.Fatal("expected error for whitespace-only name")
		}
	})

	t.Run("invalid_phase", func(t *testing.T) {
		err := validateHookDef(HookDef{
			Name:   "bad-phase",
			Phase:  rewrite.Phase("invalid"),
			Source: validReqSrc,
		})
		if err == nil || !strings.Contains(err.Error(), "phase") {
			t.Fatalf("err = %v, want phase error", err)
		}
	})

	t.Run("invalid_source", func(t *testing.T) {
		err := validateHookDef(HookDef{
			Name:   "bad-src",
			Phase:  rewrite.PhaseRequest,
			Source: "not valid go",
		})
		if err == nil {
			t.Fatal("expected error for bad source")
		}
	})
}

func TestDryRunHook(t *testing.T) {
	src := `package hook
func OnRequest(req *Request) error {
	req.SetHeader("X-Dry", "yes")
	return nil
}`
	def := HookDef{
		ID:     42,
		Name:   "dry-test",
		Phase:  rewrite.PhaseRequest,
		Source: src,
	}
	u, _ := url.Parse("https://api.example.test/v1/test")
	in := rewrite.Input{
		Request: rewrite.Request{
			Method: "GET",
			URL:    u,
			Header: http.Header{},
		},
		Response: rewrite.Response{Status: 200, Header: http.Header{}},
	}

	res, err := dryRunHook(def, in, []byte("trace-key-32-bytes-long-0000000"))
	if err != nil {
		t.Fatalf("dryRunHook: %v", err)
	}
	if res.Request.Header.Get("X-Dry") != "yes" {
		t.Fatalf("header not set: %v", res.Request.Header)
	}

	t.Run("invalid_def_returns_error", func(t *testing.T) {
		bad := HookDef{Name: "", Phase: rewrite.PhaseRequest, Source: src}
		_, err := dryRunHook(bad, in, nil)
		if err == nil {
			t.Fatal("expected error for empty name")
		}
	})

	t.Run("response_phase", func(t *testing.T) {
		respSrc := `package hook
func OnResponse(req *Request, resp *Response) error {
	resp.SetHeader("X-DryResp", "done")
	return nil
}`
		def := HookDef{ID: 43, Name: "dry-resp", Phase: rewrite.PhaseResponse, Source: respSrc}
		res, err := dryRunHook(def, in, nil)
		if err != nil {
			t.Fatalf("dryRunHook response: %v", err)
		}
		if res.Response.Header.Get("X-DryResp") != "done" {
			t.Fatalf("response header not set: %v", res.Response.Header)
		}
	})
}

func TestHookSetLenNilReceiver(t *testing.T) {
	var nilHS *HookSet
	if nilHS.Len(rewrite.PhaseRequest) != 0 {
		t.Fatal("nil HookSet Len should be 0")
	}
	if nilHS.Len(rewrite.PhaseResponse) != 0 {
		t.Fatal("nil HookSet Len should be 0")
	}
}

func TestHookSetLastErrorNilReceiver(t *testing.T) {
	var nilHS *HookSet
	if nilHS.LastError(1) != nil {
		t.Fatal("nil HookSet LastError should be nil")
	}
}

func TestRecordErrorMethods(t *testing.T) {
	inner := errors.New("dial tcp: connection refused")
	e := &recordError{step: "connect", err: inner}
	if e.Error() != "connect: dial tcp: connection refused" {
		t.Fatalf("Error() = %q", e.Error())
	}
	if e.Unwrap() != inner {
		t.Fatal("Unwrap did not return inner error")
	}
}

func TestMapDialErrorToSOCKS5ReplyExtended(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want byte
	}{
		{"dns_error", &net.DNSError{Err: "no such host", Name: "x"}, socks5RepHostUnreachable},
		{"conn_refused", syscall.ECONNREFUSED, socks5RepConnectionRefused},
		{"net_unreachable", syscall.ENETUNREACH, socks5RepNetworkUnreachable},
		{"host_unreachable", syscall.EHOSTUNREACH, socks5RepHostUnreachable},
		{"deadline_exceeded", context.DeadlineExceeded, socks5RepTTLExpired},
		{"string_conn_refused", errors.New("connection refused by remote"), socks5RepConnectionRefused},
		{"string_net_unreachable", errors.New("network is unreachable"), socks5RepNetworkUnreachable},
		{"string_no_route", errors.New("no route to host"), socks5RepHostUnreachable},
		{"string_no_such_host", errors.New("no such host"), socks5RepHostUnreachable},
		{"string_host_unreachable", errors.New("host is unreachable"), socks5RepHostUnreachable},
		{"generic", errors.New("something else entirely"), socks5RepGeneralFailure},
		{"wrapped_conn_refused", fmt.Errorf("dial: %w", syscall.ECONNREFUSED), socks5RepConnectionRefused},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := mapDialErrorToSOCKS5Reply(tc.err)
			if got != tc.want {
				t.Fatalf("got 0x%02x, want 0x%02x", got, tc.want)
			}
		})
	}
}

func TestIsLoopbackRemoteExtended(t *testing.T) {
	cases := []struct {
		name string
		addr net.Addr
		want bool
	}{
		{"tcp_loopback4", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 80}, true},
		{"tcp_loopback6", &net.TCPAddr{IP: net.IPv6loopback, Port: 80}, true},
		{"tcp_nonloopback", &net.TCPAddr{IP: net.IPv4(10, 0, 0, 1), Port: 80}, false},
		{"tcp_nil_ip", &net.TCPAddr{Port: 80}, false},
		{"nil_addr", nil, false},
		{"udp_loopback", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 80}, true},
		{"udp_nonloopback", &net.UDPAddr{IP: net.IPv4(192, 168, 1, 1), Port: 80}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := isLoopbackRemote(tc.addr)
			if got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestCompileHooksDisabledSkipped(t *testing.T) {
	src := `package hook
func OnRequest(req *Request) error { return nil }`
	hs, errs := compileHooks([]HookDef{
		{ID: 1, Name: "enabled", Enabled: true, Phase: rewrite.PhaseRequest, Source: src},
		{ID: 2, Name: "disabled", Enabled: false, Phase: rewrite.PhaseRequest, Source: src},
	}, nil)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if hs.Len(rewrite.PhaseRequest) != 1 {
		t.Fatalf("compiled = %d, want 1 (disabled should be skipped)", hs.Len(rewrite.PhaseRequest))
	}
}

func TestCompileHooksInvalidReportsError(t *testing.T) {
	_, errs := compileHooks([]HookDef{
		{ID: 1, Name: "bad", Enabled: true, Phase: rewrite.PhaseRequest, Source: "not go"},
	}, nil)
	if len(errs) != 1 {
		t.Fatalf("expected 1 load error, got %d", len(errs))
	}
	if _, ok := errs[1]; !ok {
		t.Fatalf("expected error for ID 1, got %v", errs)
	}
}

func TestCompileHooksPriorityOrder(t *testing.T) {
	src := `package hook
func OnRequest(req *Request) error { return nil }`
	hs, errs := compileHooks([]HookDef{
		{ID: 3, Name: "low-pri", Enabled: true, Phase: rewrite.PhaseRequest, Priority: 10, Source: src},
		{ID: 1, Name: "high-pri", Enabled: true, Phase: rewrite.PhaseRequest, Priority: 1, Source: src},
		{ID: 2, Name: "mid-pri", Enabled: true, Phase: rewrite.PhaseRequest, Priority: 5, Source: src},
	}, nil)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if hs.Len(rewrite.PhaseRequest) != 3 {
		t.Fatalf("compiled = %d, want 3", hs.Len(rewrite.PhaseRequest))
	}
}
