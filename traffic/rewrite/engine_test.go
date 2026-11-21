package rewrite

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func mustURL(raw string) *url.URL {
	u, err := url.Parse(raw)
	if err != nil {
		panic(err)
	}
	return u
}

func compileSet(t *testing.T, rules ...Rule) *Set {
	t.Helper()
	s := Compile(rules, Options{})
	for id, msg := range s.LoadErrors() {
		t.Fatalf("rule %d failed to compile: %s", id, msg)
	}
	return s
}

func TestSetHeader_Request(t *testing.T) {
	t.Parallel()
	s := compileSet(t, Rule{
		ID: 1, Name: "add-header", Enabled: true, Phase: PhaseRequest, Priority: 0,
		Actions: []Action{{Type: ActionSetHeader, Name: "X-Injected", Value: "yes"}},
	})
	res := s.Apply(PhaseRequest, Input{
		Request: Request{
			Method: "GET",
			URL:    mustURL("https://example.com/api"),
			Header: http.Header{},
		},
	})
	if !res.Changed {
		t.Fatal("expected change")
	}
	if v := res.Request.Header.Get("X-Injected"); v != "yes" {
		t.Fatalf("got %q, want yes", v)
	}
}

func TestDeleteHeader_Request(t *testing.T) {
	t.Parallel()
	s := compileSet(t, Rule{
		ID: 1, Name: "del-header", Enabled: true, Phase: PhaseRequest, Priority: 0,
		Actions: []Action{{Type: ActionDeleteHeader, Name: "X-Remove"}},
	})
	res := s.Apply(PhaseRequest, Input{
		Request: Request{
			Method: "GET",
			URL:    mustURL("https://example.com/"),
			Header: http.Header{"X-Remove": {"val"}},
		},
	})
	if !res.Changed {
		t.Fatal("expected change")
	}
	if v := res.Request.Header.Get("X-Remove"); v != "" {
		t.Fatalf("header should be deleted, got %q", v)
	}
}

func TestReplaceHeader(t *testing.T) {
	t.Parallel()
	s := compileSet(t, Rule{
		ID: 1, Name: "replace-auth", Enabled: true, Phase: PhaseRequest, Priority: 0,
		Actions: []Action{{Type: ActionReplaceHeader, Name: "Authorization", Regex: "Bearer .*", Value: "Bearer NEW_TOKEN"}},
	})
	res := s.Apply(PhaseRequest, Input{
		Request: Request{
			Method: "GET",
			URL:    mustURL("https://api.example.com/v1"),
			Header: http.Header{"Authorization": {"Bearer OLD_TOKEN"}},
		},
	})
	if !res.Changed {
		t.Fatal("expected change")
	}
	if v := res.Request.Header.Get("Authorization"); v != "Bearer NEW_TOKEN" {
		t.Fatalf("got %q, want Bearer NEW_TOKEN", v)
	}
}

func TestSetBody_Request(t *testing.T) {
	t.Parallel()
	s := compileSet(t, Rule{
		ID: 1, Name: "set-body", Enabled: true, Phase: PhaseRequest, Priority: 0,
		Actions: []Action{{Type: ActionSetBody, Value: `{"replaced":true}`}},
	})
	res := s.Apply(PhaseRequest, Input{
		Request: Request{
			Method: "POST",
			URL:    mustURL("https://example.com/api"),
			Header: http.Header{"Content-Type": {"application/json"}},
			Body:   []byte(`{"original":true}`),
		},
	})
	if !res.Changed {
		t.Fatal("expected change")
	}
	if string(res.Request.Body) != `{"replaced":true}` {
		t.Fatalf("body = %q", res.Request.Body)
	}
}

func TestReplaceBody_Regex(t *testing.T) {
	t.Parallel()
	s := compileSet(t, Rule{
		ID: 1, Name: "replace-body", Enabled: true, Phase: PhaseRequest, Priority: 0,
		Actions: []Action{{Type: ActionReplaceBody, Regex: "password=[^&]+", Value: "password=REDACTED"}},
	})
	res := s.Apply(PhaseRequest, Input{
		Request: Request{
			Method: "POST",
			URL:    mustURL("https://example.com/login"),
			Header: http.Header{},
			Body:   []byte("user=admin&password=secret123&action=login"),
		},
	})
	if !res.Changed {
		t.Fatal("expected change")
	}
	got := string(res.Request.Body)
	if !strings.Contains(got, "password=REDACTED") {
		t.Fatalf("body = %q, want password=REDACTED", got)
	}
	if strings.Contains(got, "secret123") {
		t.Fatal("original password should be replaced")
	}
}

func TestSetPathQuery(t *testing.T) {
	t.Parallel()
	s := compileSet(t, Rule{
		ID: 1, Name: "rewrite-path", Enabled: true, Phase: PhaseRequest, Priority: 0,
		Actions: []Action{{Type: ActionSetPathQuery, Value: "/new/path?v=2"}},
	})
	res := s.Apply(PhaseRequest, Input{
		Request: Request{
			Method: "GET",
			URL:    mustURL("https://example.com/old/path?v=1"),
			Header: http.Header{},
		},
	})
	if !res.Changed {
		t.Fatal("expected change")
	}
	if got := res.Request.URL.RequestURI(); got != "/new/path?v=2" {
		t.Fatalf("URL = %q, want /new/path?v=2", got)
	}
}

func TestReplaceURL_Regex(t *testing.T) {
	t.Parallel()
	s := compileSet(t, Rule{
		ID: 1, Name: "replace-url", Enabled: true, Phase: PhaseRequest, Priority: 0,
		Actions: []Action{{Type: ActionReplaceURL, Regex: "/v1/", Value: "/v2/"}},
	})
	res := s.Apply(PhaseRequest, Input{
		Request: Request{
			Method: "GET",
			URL:    mustURL("https://api.example.com/v1/users"),
			Header: http.Header{},
		},
	})
	if !res.Changed {
		t.Fatal("expected change")
	}
	if got := res.Request.URL.Path; got != "/v2/users" {
		t.Fatalf("path = %q, want /v2/users", got)
	}
}

func TestSetMethod(t *testing.T) {
	t.Parallel()
	s := compileSet(t, Rule{
		ID: 1, Name: "to-post", Enabled: true, Phase: PhaseRequest, Priority: 0,
		Actions: []Action{{Type: ActionSetMethod, Value: "POST"}},
	})
	res := s.Apply(PhaseRequest, Input{
		Request: Request{
			Method: "GET",
			URL:    mustURL("https://example.com/"),
			Header: http.Header{},
		},
	})
	if !res.Changed {
		t.Fatal("expected change")
	}
	if res.Request.Method != "POST" {
		t.Fatalf("method = %q, want POST", res.Request.Method)
	}
}

func TestSetStatus_Response(t *testing.T) {
	t.Parallel()
	s := compileSet(t, Rule{
		ID: 1, Name: "force-200", Enabled: true, Phase: PhaseResponse, Priority: 0,
		Actions: []Action{{Type: ActionSetStatus, Value: "200"}},
	})
	res := s.Apply(PhaseResponse, Input{
		Request: Request{
			Method: "GET",
			URL:    mustURL("https://example.com/"),
			Header: http.Header{},
		},
		Response: Response{
			Status: 403,
			Header: http.Header{},
			Body:   []byte("forbidden"),
		},
	})
	if !res.Changed {
		t.Fatal("expected change")
	}
	if res.Response.Status != 200 {
		t.Fatalf("status = %d, want 200", res.Response.Status)
	}
}

func TestNoMatchingRules_NoChange(t *testing.T) {
	t.Parallel()
	s := compileSet(t, Rule{
		ID: 1, Name: "post-only", Enabled: true, Phase: PhaseRequest, Priority: 0,
		Matchers: Matchers{Methods: []string{"POST"}},
		Actions:  []Action{{Type: ActionSetHeader, Name: "X-Post", Value: "true"}},
	})
	res := s.Apply(PhaseRequest, Input{
		Request: Request{
			Method: "GET",
			URL:    mustURL("https://example.com/"),
			Header: http.Header{},
		},
	})
	if res.Changed {
		t.Fatal("did not expect change for unmatched rule")
	}
}

func TestMatcher_Methods(t *testing.T) {
	t.Parallel()
	s := compileSet(t, Rule{
		ID: 1, Name: "post-put", Enabled: true, Phase: PhaseRequest, Priority: 0,
		Matchers: Matchers{Methods: []string{"POST", "PUT"}},
		Actions:  []Action{{Type: ActionSetHeader, Name: "X-Match", Value: "yes"}},
	})

	for _, tc := range []struct {
		method string
		match  bool
	}{
		{"POST", true},
		{"PUT", true},
		{"GET", false},
		{"DELETE", false},
	} {
		t.Run(tc.method, func(t *testing.T) {
			res := s.Apply(PhaseRequest, Input{
				Request: Request{
					Method: tc.method,
					URL:    mustURL("https://example.com/"),
					Header: http.Header{},
				},
			})
			if res.Changed != tc.match {
				t.Fatalf("method %s: changed=%v, want %v", tc.method, res.Changed, tc.match)
			}
		})
	}
}

func TestMatcher_Host(t *testing.T) {
	t.Parallel()
	s := compileSet(t, Rule{
		ID: 1, Name: "host-match", Enabled: true, Phase: PhaseRequest, Priority: 0,
		Matchers: Matchers{Host: "api.example.com"},
		Actions:  []Action{{Type: ActionSetHeader, Name: "X-Match", Value: "yes"}},
	})
	res := s.Apply(PhaseRequest, Input{
		Request: Request{Method: "GET", URL: mustURL("https://api.example.com/v1"), Header: http.Header{}},
	})
	if !res.Changed {
		t.Fatal("expected match for api.example.com")
	}
	res = s.Apply(PhaseRequest, Input{
		Request: Request{Method: "GET", URL: mustURL("https://other.example.com/v1"), Header: http.Header{}},
	})
	if res.Changed {
		t.Fatal("did not expect match for other.example.com")
	}
}

func TestMatcher_HostGlob(t *testing.T) {
	t.Parallel()
	s := compileSet(t, Rule{
		ID: 1, Name: "glob", Enabled: true, Phase: PhaseRequest, Priority: 0,
		Matchers: Matchers{Host: "*.example.com"},
		Actions:  []Action{{Type: ActionSetHeader, Name: "X-Match", Value: "yes"}},
	})
	res := s.Apply(PhaseRequest, Input{
		Request: Request{Method: "GET", URL: mustURL("https://sub.example.com/"), Header: http.Header{}},
	})
	if !res.Changed {
		t.Fatal("expected match for sub.example.com")
	}
	res = s.Apply(PhaseRequest, Input{
		Request: Request{Method: "GET", URL: mustURL("https://example.com/"), Header: http.Header{}},
	})
	if res.Changed {
		t.Fatal("*.example.com should not match example.com itself")
	}
}

func TestMatcher_URLRegex(t *testing.T) {
	t.Parallel()
	s := compileSet(t, Rule{
		ID: 1, Name: "url-re", Enabled: true, Phase: PhaseRequest, Priority: 0,
		Matchers: Matchers{URLRegex: `/api/v[0-9]+/`},
		Actions:  []Action{{Type: ActionSetHeader, Name: "X-Match", Value: "yes"}},
	})
	res := s.Apply(PhaseRequest, Input{
		Request: Request{Method: "GET", URL: mustURL("https://example.com/api/v2/users"), Header: http.Header{}},
	})
	if !res.Changed {
		t.Fatal("expected match")
	}
	res = s.Apply(PhaseRequest, Input{
		Request: Request{Method: "GET", URL: mustURL("https://example.com/static/main.js"), Header: http.Header{}},
	})
	if res.Changed {
		t.Fatal("did not expect match")
	}
}

func TestMatcher_HeaderPresence(t *testing.T) {
	t.Parallel()
	s := compileSet(t, Rule{
		ID: 1, Name: "has-auth", Enabled: true, Phase: PhaseRequest, Priority: 0,
		Matchers: Matchers{Headers: []HeaderMatcher{{Name: "Authorization"}}},
		Actions:  []Action{{Type: ActionSetHeader, Name: "X-Has-Auth", Value: "yes"}},
	})
	res := s.Apply(PhaseRequest, Input{
		Request: Request{Method: "GET", URL: mustURL("https://e.com/"), Header: http.Header{"Authorization": {"Bearer x"}}},
	})
	if !res.Changed {
		t.Fatal("expected match for present header")
	}
	res = s.Apply(PhaseRequest, Input{
		Request: Request{Method: "GET", URL: mustURL("https://e.com/"), Header: http.Header{}},
	})
	if res.Changed {
		t.Fatal("should not match absent header")
	}
}

func TestMatcher_HeaderValueRegex(t *testing.T) {
	t.Parallel()
	s := compileSet(t, Rule{
		ID: 1, Name: "json-ct", Enabled: true, Phase: PhaseRequest, Priority: 0,
		Matchers: Matchers{Headers: []HeaderMatcher{{Name: "Content-Type", ValueRegex: "application/json"}}},
		Actions:  []Action{{Type: ActionSetHeader, Name: "X-JSON", Value: "yes"}},
	})
	res := s.Apply(PhaseRequest, Input{
		Request: Request{
			Method: "POST",
			URL:    mustURL("https://e.com/"),
			Header: http.Header{"Content-Type": {"application/json; charset=utf-8"}},
		},
	})
	if !res.Changed {
		t.Fatal("expected match for json content type")
	}
}

func TestMatcher_BodyRegex(t *testing.T) {
	t.Parallel()
	s := compileSet(t, Rule{
		ID: 1, Name: "body-match", Enabled: true, Phase: PhaseRequest, Priority: 0,
		Matchers: Matchers{BodyRegex: `"admin"\s*:\s*true`},
		Actions:  []Action{{Type: ActionSetHeader, Name: "X-Admin", Value: "detected"}},
	})
	res := s.Apply(PhaseRequest, Input{
		Request: Request{
			Method: "POST",
			URL:    mustURL("https://e.com/"),
			Header: http.Header{},
			Body:   []byte(`{"admin": true, "user": "test"}`),
		},
	})
	if !res.Changed {
		t.Fatal("expected match for body regex")
	}
}

func TestMatcher_Status(t *testing.T) {
	t.Parallel()
	s := compileSet(t, Rule{
		ID: 1, Name: "on-404", Enabled: true, Phase: PhaseResponse, Priority: 0,
		Matchers: Matchers{Status: []string{"404"}},
		Actions:  []Action{{Type: ActionSetBody, Value: "not found custom"}},
	})
	res := s.Apply(PhaseResponse, Input{
		Request:  Request{Method: "GET", URL: mustURL("https://e.com/"), Header: http.Header{}},
		Response: Response{Status: 404, Header: http.Header{}, Body: []byte("original")},
	})
	if !res.Changed {
		t.Fatal("expected change on 404")
	}
	if string(res.Response.Body) != "not found custom" {
		t.Fatalf("body = %q", res.Response.Body)
	}
	res = s.Apply(PhaseResponse, Input{
		Request:  Request{Method: "GET", URL: mustURL("https://e.com/"), Header: http.Header{}},
		Response: Response{Status: 200, Header: http.Header{}, Body: []byte("ok")},
	})
	if res.Changed {
		t.Fatal("should not change 200")
	}
}

func TestMatcher_StatusClass(t *testing.T) {
	t.Parallel()
	s := compileSet(t, Rule{
		ID: 1, Name: "on-4xx", Enabled: true, Phase: PhaseResponse, Priority: 0,
		Matchers: Matchers{Status: []string{"4xx"}},
		Actions:  []Action{{Type: ActionSetBody, Value: "error"}},
	})
	for _, code := range []int{400, 401, 403, 404, 499} {
		res := s.Apply(PhaseResponse, Input{
			Request:  Request{Method: "GET", URL: mustURL("https://e.com/"), Header: http.Header{}},
			Response: Response{Status: code, Header: http.Header{}, Body: []byte("x")},
		})
		if !res.Changed {
			t.Fatalf("expected match for %d", code)
		}
	}
	res := s.Apply(PhaseResponse, Input{
		Request:  Request{Method: "GET", URL: mustURL("https://e.com/"), Header: http.Header{}},
		Response: Response{Status: 200, Header: http.Header{}, Body: []byte("ok")},
	})
	if res.Changed {
		t.Fatal("should not match 200 for 4xx")
	}
}

func TestPriority_Ordering(t *testing.T) {
	t.Parallel()
	s := compileSet(t,
		Rule{
			ID: 2, Name: "prio-10", Enabled: true, Phase: PhaseRequest, Priority: 10,
			Actions: []Action{{Type: ActionSetHeader, Name: "X-Last", Value: "prio10"}},
		},
		Rule{
			ID: 1, Name: "prio-1", Enabled: true, Phase: PhaseRequest, Priority: 1,
			Actions: []Action{{Type: ActionSetHeader, Name: "X-Last", Value: "prio1"}},
		},
	)
	res := s.Apply(PhaseRequest, Input{
		Request: Request{Method: "GET", URL: mustURL("https://e.com/"), Header: http.Header{}},
	})
	if v := res.Request.Header.Get("X-Last"); v != "prio10" {
		t.Fatalf("got %q, want prio10 (highest priority applies last)", v)
	}
}

func TestDisabledRule_Skipped(t *testing.T) {
	t.Parallel()
	s := compileSet(t, Rule{
		ID: 1, Name: "disabled", Enabled: false, Phase: PhaseRequest, Priority: 0,
		Actions: []Action{{Type: ActionSetHeader, Name: "X-Bad", Value: "true"}},
	})
	res := s.Apply(PhaseRequest, Input{
		Request: Request{Method: "GET", URL: mustURL("https://e.com/"), Header: http.Header{}},
	})
	if res.Changed {
		t.Fatal("disabled rule should not apply")
	}
}

func TestNilSet(t *testing.T) {
	t.Parallel()
	var s *Set
	if s.Len(PhaseRequest) != 0 {
		t.Fatal("nil set should have length 0")
	}
}

func TestEmptyRules_NoChange(t *testing.T) {
	t.Parallel()
	s := compileSet(t)
	res := s.Apply(PhaseRequest, Input{
		Request: Request{Method: "GET", URL: mustURL("https://e.com/"), Header: http.Header{}},
	})
	if res.Changed {
		t.Fatal("no rules, no change")
	}
}

func TestBinaryBody_Skipped(t *testing.T) {
	t.Parallel()
	s := compileSet(t, Rule{
		ID: 1, Name: "body-rule", Enabled: true, Phase: PhaseRequest, Priority: 0,
		Actions: []Action{{Type: ActionReplaceBody, Regex: ".", Value: "x"}},
	})
	body := make([]byte, 100)
	body[0] = 0 // NUL in first 512 bytes = binary
	res := s.Apply(PhaseRequest, Input{
		Request: Request{Method: "POST", URL: mustURL("https://e.com/"), Header: http.Header{}, Body: body},
	})
	if res.Changed {
		t.Fatal("binary body should skip body rules")
	}
}

func TestContentEncoding_Skipped(t *testing.T) {
	t.Parallel()
	s := compileSet(t, Rule{
		ID: 1, Name: "body-rule", Enabled: true, Phase: PhaseRequest, Priority: 0,
		Actions: []Action{{Type: ActionReplaceBody, Regex: "x", Value: "y"}},
	})
	res := s.Apply(PhaseRequest, Input{
		Request: Request{
			Method: "POST",
			URL:    mustURL("https://e.com/"),
			Header: http.Header{"Content-Encoding": {"gzip"}},
			Body:   []byte("xxx"),
		},
	})
	if res.Changed {
		t.Fatal("content-encoded body should be skipped")
	}
}

func TestStream_ResponseBodySkipped(t *testing.T) {
	t.Parallel()
	s := compileSet(t, Rule{
		ID: 1, Name: "resp-body", Enabled: true, Phase: PhaseResponse, Priority: 0,
		Actions: []Action{{Type: ActionSetBody, Value: "new"}},
	})
	res := s.Apply(PhaseResponse, Input{
		Request:  Request{Method: "GET", URL: mustURL("https://e.com/"), Header: http.Header{}},
		Response: Response{Status: 200, Header: http.Header{}, Body: []byte("old")},
		Stream:   true,
	})
	if res.Changed {
		t.Fatal("stream should skip body rules")
	}
}

func TestActionOrder_HeaderBeforeBody(t *testing.T) {
	t.Parallel()
	s := compileSet(t, Rule{
		ID: 1, Name: "both", Enabled: true, Phase: PhaseRequest, Priority: 0,
		Actions: []Action{
			{Type: ActionSetBody, Value: "new-body"},
			{Type: ActionSetHeader, Name: "X-Order", Value: "first"},
		},
	})
	res := s.Apply(PhaseRequest, Input{
		Request: Request{
			Method: "POST",
			URL:    mustURL("https://e.com/"),
			Header: http.Header{},
			Body:   []byte("old"),
		},
	})
	if !res.Changed {
		t.Fatal("expected change")
	}
	if v := res.Request.Header.Get("X-Order"); v != "first" {
		t.Fatalf("got %q, want first", v)
	}
	if string(res.Request.Body) != "new-body" {
		t.Fatalf("body = %q, want new-body", res.Request.Body)
	}
}

func TestBodyRewriteClearsContentLength(t *testing.T) {
	t.Parallel()
	s := compileSet(t, Rule{
		ID: 1, Name: "set-body", Enabled: true, Phase: PhaseRequest, Priority: 0,
		Actions: []Action{{Type: ActionSetBody, Value: "different"}},
	})
	res := s.Apply(PhaseRequest, Input{
		Request: Request{
			Method: "POST",
			URL:    mustURL("https://e.com/"),
			Header: http.Header{"Content-Length": {"100"}, "Etag": {`"abc"`}},
			Body:   []byte("original"),
		},
	})
	if !res.Changed {
		t.Fatal("expected change")
	}
	if v := res.Request.Header.Get("Content-Length"); v != "" {
		t.Fatalf("Content-Length should be cleared, got %q", v)
	}
	if v := res.Request.Header.Get("Etag"); v != "" {
		t.Fatalf("Etag should be cleared, got %q", v)
	}
}

func TestNoChange_SameValue(t *testing.T) {
	t.Parallel()
	s := compileSet(t, Rule{
		ID: 1, Name: "same", Enabled: true, Phase: PhaseRequest, Priority: 0,
		Actions: []Action{{Type: ActionSetHeader, Name: "X-Keep", Value: "val"}},
	})
	res := s.Apply(PhaseRequest, Input{
		Request: Request{
			Method: "GET",
			URL:    mustURL("https://e.com/"),
			Header: http.Header{"X-Keep": {"val"}},
		},
	})
	if res.Changed {
		t.Fatal("setting same value should not mark as changed")
	}
}

func TestValidate_ForbiddenHeader(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"Host", "Content-Length", "Transfer-Encoding", "Connection", "Proxy-Auth"} {
		t.Run(name, func(t *testing.T) {
			err := Validate(Rule{
				ID: 1, Name: "bad", Enabled: true, Phase: PhaseRequest,
				Actions: []Action{{Type: ActionSetHeader, Name: name, Value: "x"}},
			})
			if err == nil {
				t.Fatalf("expected error for forbidden header %s", name)
			}
		})
	}
}

func TestValidate_EmptyName(t *testing.T) {
	t.Parallel()
	err := Validate(Rule{
		ID: 1, Name: "", Enabled: true, Phase: PhaseRequest,
		Actions: []Action{{Type: ActionSetHeader, Name: "X-Ok", Value: "v"}},
	})
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestValidate_NoActions(t *testing.T) {
	t.Parallel()
	err := Validate(Rule{
		ID: 1, Name: "empty", Enabled: true, Phase: PhaseRequest,
	})
	if err == nil {
		t.Fatal("expected error for no actions")
	}
}

func TestValidate_InvalidPhase(t *testing.T) {
	t.Parallel()
	err := Validate(Rule{
		ID: 1, Name: "bad", Enabled: true, Phase: "invalid",
		Actions: []Action{{Type: ActionSetHeader, Name: "X-Ok", Value: "v"}},
	})
	if err == nil {
		t.Fatal("expected error for invalid phase")
	}
}

func TestValidate_RequestActionInResponsePhase(t *testing.T) {
	t.Parallel()
	err := Validate(Rule{
		ID: 1, Name: "wrong-phase", Enabled: true, Phase: PhaseResponse,
		Actions: []Action{{Type: ActionSetMethod, Value: "POST"}},
	})
	if err == nil {
		t.Fatal("expected error for request-phase action in response phase")
	}
}

func TestValidate_ResponseMatcherInRequestPhase(t *testing.T) {
	t.Parallel()
	err := Validate(Rule{
		ID: 1, Name: "bad-matcher", Enabled: true, Phase: PhaseRequest,
		Matchers: Matchers{Status: []string{"404"}},
		Actions:  []Action{{Type: ActionSetHeader, Name: "X-Ok", Value: "v"}},
	})
	if err == nil {
		t.Fatal("expected error for response matcher in request phase")
	}
}

func TestValidate_InvalidSetMethod(t *testing.T) {
	t.Parallel()
	err := Validate(Rule{
		ID: 1, Name: "bad-method", Enabled: true, Phase: PhaseRequest,
		Actions: []Action{{Type: ActionSetMethod, Value: "CONNECT"}},
	})
	if err == nil {
		t.Fatal("expected error for CONNECT method")
	}
}

func TestValidate_InvalidSetPathQuery(t *testing.T) {
	t.Parallel()
	err := Validate(Rule{
		ID: 1, Name: "bad-path", Enabled: true, Phase: PhaseRequest,
		Actions: []Action{{Type: ActionSetPathQuery, Value: "no-leading-slash"}},
	})
	if err == nil {
		t.Fatal("expected error for path without leading /")
	}
}

func TestValidate_HeaderValueCRLF(t *testing.T) {
	t.Parallel()
	err := Validate(Rule{
		ID: 1, Name: "crlf", Enabled: true, Phase: PhaseRequest,
		Actions: []Action{{Type: ActionSetHeader, Name: "X-Bad", Value: "val\r\nX-Injected: oops"}},
	})
	if err == nil {
		t.Fatal("expected error for CRLF in header value")
	}
}

func TestValidate_SetStatusRange(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		value string
		ok    bool
	}{
		{"200", true},
		{"404", true},
		{"599", true},
		{"100", false},
		{"204", false},
		{"304", false},
		{"600", false},
		{"abc", false},
	} {
		t.Run(tc.value, func(t *testing.T) {
			err := Validate(Rule{
				ID: 1, Name: "status", Enabled: true, Phase: PhaseResponse,
				Actions: []Action{{Type: ActionSetStatus, Value: tc.value}},
			})
			if tc.ok && err != nil {
				t.Fatalf("unexpected error for %s: %v", tc.value, err)
			}
			if !tc.ok && err == nil {
				t.Fatalf("expected error for %s", tc.value)
			}
		})
	}
}

func TestSensitiveAction_TaintedField(t *testing.T) {
	t.Parallel()
	s := compileSet(t, Rule{
		ID: 1, Name: "sensitive", Enabled: true, Phase: PhaseRequest, Priority: 0,
		Actions: []Action{{Type: ActionSetHeader, Name: "X-Secret", Value: "token123", Sensitive: true}},
	})
	res := s.Apply(PhaseRequest, Input{
		Request: Request{Method: "GET", URL: mustURL("https://e.com/"), Header: http.Header{}},
	})
	if !res.Changed {
		t.Fatal("expected change")
	}
	if res.Tainted == nil || !res.Tainted["header:X-Secret"] {
		t.Fatal("expected header:X-Secret to be tainted")
	}
}

func TestTrace_AppliedLine(t *testing.T) {
	t.Parallel()
	s := compileSet(t, Rule{
		ID: 1, Name: "traced", Enabled: true, Phase: PhaseRequest, Priority: 0,
		Actions: []Action{{Type: ActionSetHeader, Name: "X-Traced", Value: "yes"}},
	})
	res := s.Apply(PhaseRequest, Input{
		Request: Request{Method: "GET", URL: mustURL("https://e.com/"), Header: http.Header{}},
	})
	if len(res.Lines) == 0 {
		t.Fatal("expected at least one trace line")
	}
	found := false
	for _, l := range res.Lines {
		if l.Outcome == OutcomeApplied && l.Action == ActionSetHeader {
			found = true
		}
	}
	if !found {
		t.Fatal("expected an applied set_header line")
	}
}

func TestPipeline_MultipleRules(t *testing.T) {
	t.Parallel()
	s := compileSet(t,
		Rule{
			ID: 1, Name: "first", Enabled: true, Phase: PhaseRequest, Priority: 1,
			Actions: []Action{{Type: ActionSetHeader, Name: "X-Step", Value: "1"}},
		},
		Rule{
			ID: 2, Name: "second", Enabled: true, Phase: PhaseRequest, Priority: 2,
			Actions: []Action{{Type: ActionSetHeader, Name: "X-Step", Value: "2"}},
		},
	)
	res := s.Apply(PhaseRequest, Input{
		Request: Request{Method: "GET", URL: mustURL("https://e.com/"), Header: http.Header{}},
	})
	if v := res.Request.Header.Get("X-Step"); v != "2" {
		t.Fatalf("pipeline: got %q, want 2 (second rule overwrites first)", v)
	}
}

func TestUpgrade_Skipped(t *testing.T) {
	t.Parallel()
	s := compileSet(t, Rule{
		ID: 1, Name: "ws", Enabled: true, Phase: PhaseRequest, Priority: 0,
		Actions: []Action{{Type: ActionSetHeader, Name: "X-Modified", Value: "yes"}},
	})
	res := s.Apply(PhaseRequest, Input{
		Request: Request{
			Method: "GET",
			URL:    mustURL("https://e.com/ws"),
			Header: http.Header{"Upgrade": {"websocket"}},
		},
	})
	if res.Changed {
		t.Fatal("websocket upgrade should not be rewritten")
	}
}

func TestConnectionHeader_Skipped(t *testing.T) {
	t.Parallel()
	s := compileSet(t, Rule{
		ID: 1, Name: "conn", Enabled: true, Phase: PhaseRequest, Priority: 0,
		Actions: []Action{{Type: ActionSetHeader, Name: "X-Custom", Value: "val"}},
	})
	res := s.Apply(PhaseRequest, Input{
		Request: Request{
			Method: "GET",
			URL:    mustURL("https://e.com/"),
			Header: http.Header{"Connection": {"X-Custom"}},
		},
	})
	if res.Changed {
		t.Fatal("rule writing a Connection-listed header should be skipped")
	}
}

func TestLargeBody_Skipped(t *testing.T) {
	t.Parallel()
	s := compileSet(t, Rule{
		ID: 1, Name: "body-rule", Enabled: true, Phase: PhaseRequest, Priority: 0,
		Matchers: Matchers{BodyRegex: "x"},
		Actions:  []Action{{Type: ActionSetHeader, Name: "X-Tag", Value: "yes"}},
	})
	body := make([]byte, MaxBodySize+1)
	for i := range body {
		body[i] = 'x'
	}
	res := s.Apply(PhaseRequest, Input{
		Request: Request{Method: "POST", URL: mustURL("https://e.com/"), Header: http.Header{}, Body: body},
	})
	if res.Changed {
		t.Fatal("rule with body matcher should skip large bodies")
	}
}

func TestReplaceBody_NoMatch(t *testing.T) {
	t.Parallel()
	s := compileSet(t, Rule{
		ID: 1, Name: "no-match", Enabled: true, Phase: PhaseRequest, Priority: 0,
		Actions: []Action{{Type: ActionReplaceBody, Regex: "NOMATCH", Value: "x"}},
	})
	res := s.Apply(PhaseRequest, Input{
		Request: Request{
			Method: "POST",
			URL:    mustURL("https://e.com/"),
			Header: http.Header{},
			Body:   []byte("hello world"),
		},
	})
	if res.Changed {
		t.Fatal("replace with no match should not change")
	}
}

func TestResponse_RespHeader(t *testing.T) {
	t.Parallel()
	s := compileSet(t, Rule{
		ID: 1, Name: "resp-hdr", Enabled: true, Phase: PhaseResponse, Priority: 0,
		Actions: []Action{{Type: ActionSetHeader, Name: "X-Added", Value: "resp-val"}},
	})
	res := s.Apply(PhaseResponse, Input{
		Request:  Request{Method: "GET", URL: mustURL("https://e.com/"), Header: http.Header{}},
		Response: Response{Status: 200, Header: http.Header{}, Body: []byte("ok")},
	})
	if !res.Changed {
		t.Fatal("expected change")
	}
	if v := res.Response.Header.Get("X-Added"); v != "resp-val" {
		t.Fatalf("got %q, want resp-val", v)
	}
}

func TestResponse_ReplaceBody(t *testing.T) {
	t.Parallel()
	s := compileSet(t, Rule{
		ID: 1, Name: "resp-body", Enabled: true, Phase: PhaseResponse, Priority: 0,
		Actions: []Action{{Type: ActionReplaceBody, Regex: "secret", Value: "REDACTED"}},
	})
	res := s.Apply(PhaseResponse, Input{
		Request:  Request{Method: "GET", URL: mustURL("https://e.com/"), Header: http.Header{}},
		Response: Response{Status: 200, Header: http.Header{}, Body: []byte("the secret is here")},
	})
	if !res.Changed {
		t.Fatal("expected change")
	}
	got := string(res.Response.Body)
	if !strings.Contains(got, "REDACTED") {
		t.Fatalf("body = %q, expected REDACTED", got)
	}
}

func TestInputNotMutated(t *testing.T) {
	t.Parallel()
	s := compileSet(t, Rule{
		ID: 1, Name: "mut-check", Enabled: true, Phase: PhaseRequest, Priority: 0,
		Actions: []Action{{Type: ActionSetHeader, Name: "X-New", Value: "val"}},
	})
	origHeader := http.Header{"X-Original": {"keep"}}
	in := Input{
		Request: Request{Method: "GET", URL: mustURL("https://e.com/"), Header: origHeader},
	}
	s.Apply(PhaseRequest, in)
	if origHeader.Get("X-New") != "" {
		t.Fatal("original header should not be mutated")
	}
}

func TestResponse_RespHeaderMatcher(t *testing.T) {
	t.Parallel()
	s := compileSet(t, Rule{
		ID: 1, Name: "resp-hdr-match", Enabled: true, Phase: PhaseResponse, Priority: 0,
		Matchers: Matchers{RespHeaders: []HeaderMatcher{{Name: "X-Custom", ValueRegex: "target"}}},
		Actions:  []Action{{Type: ActionSetBody, Value: "matched"}},
	})
	res := s.Apply(PhaseResponse, Input{
		Request:  Request{Method: "GET", URL: mustURL("https://e.com/"), Header: http.Header{}},
		Response: Response{Status: 200, Header: http.Header{"X-Custom": {"target-value"}}, Body: []byte("original")},
	})
	if !res.Changed {
		t.Fatal("expected match for resp header value regex")
	}
	res = s.Apply(PhaseResponse, Input{
		Request:  Request{Method: "GET", URL: mustURL("https://e.com/"), Header: http.Header{}},
		Response: Response{Status: 200, Header: http.Header{"X-Custom": {"other"}}, Body: []byte("original")},
	})
	if res.Changed {
		t.Fatal("should not match non-matching resp header value")
	}
}

func TestErrorLog(t *testing.T) {
	t.Parallel()
	el := &ErrorLog{}
	el.record(1, ErrClassPanic)
	le, ok := el.Get(1)
	if !ok {
		t.Fatal("expected entry for rule 1")
	}
	if le.Class != ErrClassPanic {
		t.Fatalf("class = %q, want %q", le.Class, ErrClassPanic)
	}
	if le.Count != 1 {
		t.Fatalf("count = %d, want 1", le.Count)
	}
	el.record(1, ErrClassPanic)
	le, _ = el.Get(1)
	if le.Count != 2 {
		t.Fatalf("count = %d, want 2", le.Count)
	}
	el.Forget(1)
	_, ok = el.Get(1)
	if ok {
		t.Fatal("expected entry to be forgotten")
	}
}

func TestLoadErrors(t *testing.T) {
	t.Parallel()
	s := Compile([]Rule{
		{ID: 1, Name: "good", Enabled: true, Phase: PhaseRequest, Actions: []Action{{Type: ActionSetHeader, Name: "X-Ok", Value: "v"}}},
		{ID: 2, Name: "", Enabled: true, Phase: PhaseRequest, Actions: []Action{{Type: ActionSetHeader, Name: "X-Ok", Value: "v"}}},
	}, Options{})
	errs := s.LoadErrors()
	if _, ok := errs[2]; !ok {
		t.Fatal("expected load error for rule 2 (empty name)")
	}
	if s.Len(PhaseRequest) != 1 {
		t.Fatalf("expected 1 valid rule, got %d", s.Len(PhaseRequest))
	}
}

func response(status int, header http.Header, body string) Response {
	if header == nil {
		header = http.Header{}
	}
	return Response{Status: status, Header: header, Body: []byte(body)}
}

func traceLineString(l TraceLine) string {
	return fmt.Sprintf("%d:%s:%s:%s:%s", l.RuleID, l.Action, l.Field, l.Outcome, l.Reason)
}

func wantLines(t *testing.T, got []TraceLine, want ...string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("trace lines: got %d, want %d\n  got=%v", len(got), len(want), got)
	}
	for i, w := range want {
		if s := traceLineString(got[i]); s != w {
			t.Errorf("line %d: got %q, want %q", i, s, w)
		}
	}
}
