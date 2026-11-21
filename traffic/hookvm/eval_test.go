package hookvm

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

const reqHookAddHeader = `package hook

func OnRequest(req *Request) error {
	req.SetHeader("X-Injected", "yes")
	return nil
}
`

const reqHookRewritePath = `package hook

func OnRequest(req *Request) error {
	req.Path = "/rewritten"
	return nil
}
`

const respHookSetStatus = `package hook

func OnResponse(req *Request, resp *Response) error {
	resp.Status = 418
	return nil
}
`

const respHookReadReq = `package hook

func OnResponse(req *Request, resp *Response) error {
	if req.Method == "GET" {
		resp.Body = []byte("was-get")
	}
	return nil
}
`

func compile(t *testing.T, phase, src string) *Program {
	t.Helper()
	p, err := Compile(phase, src)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	return p
}

func TestRunRequest_AddHeader(t *testing.T) {
	t.Parallel()
	p := compile(t, "request", reqHookAddHeader)
	req := &Request{
		Scheme:  "https",
		Host:    "example.com",
		Port:    "443",
		Method:  "GET",
		Path:    "/api",
		Headers: map[string][]string{},
	}
	if err := p.RunRequest(context.Background(), req, RunOptions{}); err != nil {
		t.Fatal(err)
	}
	if v := req.Headers["X-Injected"]; len(v) == 0 || v[0] != "yes" {
		t.Fatalf("expected X-Injected=yes, got %v", v)
	}
}

func TestRunRequest_RewritePath(t *testing.T) {
	t.Parallel()
	p := compile(t, "request", reqHookRewritePath)
	req := &Request{Method: "GET", Path: "/original", Headers: map[string][]string{}}
	if err := p.RunRequest(context.Background(), req, RunOptions{}); err != nil {
		t.Fatal(err)
	}
	if req.Path != "/rewritten" {
		t.Fatalf("path = %q, want /rewritten", req.Path)
	}
}

func TestRunRequest_NilHeaders(t *testing.T) {
	t.Parallel()
	p := compile(t, "request", reqHookAddHeader)
	req := &Request{Method: "GET", Path: "/"}
	if err := p.RunRequest(context.Background(), req, RunOptions{}); err != nil {
		t.Fatal(err)
	}
	if req.Headers == nil {
		t.Fatal("headers should be initialized")
	}
}

func TestRunRequest_EmptyBody(t *testing.T) {
	t.Parallel()
	src := `package hook

func OnRequest(req *Request) error {
	if len(req.Body) != 0 {
		return errors.New("expected empty body")
	}
	return nil
}
`
	p := compile(t, "request", src)
	req := &Request{Method: "GET", Path: "/", Body: nil}
	if err := p.RunRequest(context.Background(), req, RunOptions{}); err != nil {
		t.Fatal(err)
	}
}

func TestRunRequest_LargeBody(t *testing.T) {
	t.Parallel()
	src := `package hook

func OnRequest(req *Request) error {
	if len(req.Body) < 1000 {
		return errors.New("expected large body")
	}
	return nil
}
`
	p := compile(t, "request", src)
	req := &Request{Method: "POST", Path: "/", Body: make([]byte, 10000)}
	if err := p.RunRequest(context.Background(), req, RunOptions{}); err != nil {
		t.Fatal(err)
	}
}

func TestRunResponse_SetStatus(t *testing.T) {
	t.Parallel()
	p := compile(t, "response", respHookSetStatus)
	req := &Request{Method: "GET", Path: "/"}
	resp := &Response{Status: 200, Headers: map[string][]string{}, Body: []byte("ok")}
	if err := p.RunResponse(context.Background(), req, resp, RunOptions{}); err != nil {
		t.Fatal(err)
	}
	if resp.Status != 418 {
		t.Fatalf("status = %d, want 418", resp.Status)
	}
}

func TestRunResponse_ReadRequest(t *testing.T) {
	t.Parallel()
	p := compile(t, "response", respHookReadReq)
	req := &Request{Method: "GET", Path: "/"}
	resp := &Response{Status: 200, Body: []byte("original")}
	if err := p.RunResponse(context.Background(), req, resp, RunOptions{}); err != nil {
		t.Fatal(err)
	}
	if string(resp.Body) != "was-get" {
		t.Fatalf("body = %q, want was-get", resp.Body)
	}
}

func TestRunResponse_RequestReadOnly(t *testing.T) {
	t.Parallel()
	src := `package hook

func OnResponse(req *Request, resp *Response) error {
	req.Path = "/hack"
	return nil
}
`
	p := compile(t, "response", src)
	req := &Request{Method: "GET", Path: "/original"}
	resp := &Response{Status: 200}
	err := p.RunResponse(context.Background(), req, resp, RunOptions{})
	if err == nil {
		t.Fatal("expected error mutating read-only request")
	}
	if !errors.Is(err, ErrForbiddenMutation) {
		t.Fatalf("got %v, want ErrForbiddenMutation", err)
	}
}

func TestRunResponse_RequestSetHeaderReadOnly(t *testing.T) {
	t.Parallel()
	src := `package hook

func OnResponse(req *Request, resp *Response) error {
	req.SetHeader("X-Bad", "val")
	return nil
}
`
	p := compile(t, "response", src)
	req := &Request{Method: "GET", Path: "/"}
	resp := &Response{Status: 200}
	err := p.RunResponse(context.Background(), req, resp, RunOptions{})
	if err == nil {
		t.Fatal("expected error calling SetHeader on read-only request")
	}
	if !errors.Is(err, ErrForbiddenMutation) {
		t.Fatalf("got %v, want ErrForbiddenMutation", err)
	}
}

func TestForbiddenMutation_SchemeHostPort(t *testing.T) {
	t.Parallel()
	for _, field := range []string{"Scheme", "Host", "Port"} {
		t.Run(field, func(t *testing.T) {
			src := `package hook

func OnRequest(req *Request) error {
	req.` + field + ` = "evil"
	return nil
}
`
			p := compile(t, "request", src)
			req := &Request{Scheme: "https", Host: "example.com", Port: "443", Method: "GET", Path: "/"}
			err := p.RunRequest(context.Background(), req, RunOptions{})
			if err == nil {
				t.Fatalf("expected error assigning %s", field)
			}
			if !errors.Is(err, ErrForbiddenMutation) {
				t.Fatalf("got %v, want ErrForbiddenMutation", err)
			}
		})
	}
}

func TestStepLimit(t *testing.T) {
	t.Parallel()
	src := `package hook

func OnRequest(req *Request) error {
	for i := 0; i < 999999999; i++ {
	}
	return nil
}
`
	p := compile(t, "request", src)
	req := &Request{Method: "GET", Path: "/"}
	err := p.RunRequest(context.Background(), req, RunOptions{MaxSteps: 500})
	if !errors.Is(err, ErrStepLimit) {
		t.Fatalf("got %v, want ErrStepLimit", err)
	}
}

func TestTimeout(t *testing.T) {
	t.Parallel()
	src := `package hook

func OnRequest(req *Request) error {
	for i := 0; i < 999999999; i++ {
	}
	return nil
}
`
	p := compile(t, "request", src)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()
	time.Sleep(2 * time.Millisecond)
	req := &Request{Method: "GET", Path: "/"}
	err := p.RunRequest(ctx, req, RunOptions{})
	if !errors.Is(err, ErrTimeout) && !errors.Is(err, ErrStepLimit) {
		t.Fatalf("expected timeout or step limit, got %v", err)
	}
}

func TestMemoryLimit(t *testing.T) {
	t.Parallel()
	src := `package hook

func OnRequest(req *Request) error {
	s := "a"
	for i := 0; i < 100; i++ {
		s = s + s
	}
	return nil
}
`
	p := compile(t, "request", src)
	req := &Request{Method: "GET", Path: "/"}
	err := p.RunRequest(context.Background(), req, RunOptions{MaxMemory: 1024})
	if !errors.Is(err, ErrMemoryLimit) {
		t.Fatalf("got %v, want ErrMemoryLimit", err)
	}
}

func TestCallDepthLimit(t *testing.T) {
	t.Parallel()
	src := `package hook

func recurse(n int) int {
	return recurse(n + 1)
}

func OnRequest(req *Request) error {
	recurse(0)
	return nil
}
`
	p := compile(t, "request", src)
	req := &Request{Method: "GET", Path: "/"}
	err := p.RunRequest(context.Background(), req, RunOptions{})
	if !errors.Is(err, ErrCallDepth) {
		t.Fatalf("got %v, want ErrCallDepth", err)
	}
}

func TestPanicRecovery(t *testing.T) {
	t.Parallel()
	src := `package hook

func OnRequest(req *Request) error {
	panic("boom")
	return nil
}
`
	p := compile(t, "request", src)
	req := &Request{Method: "GET", Path: "/"}
	err := p.RunRequest(context.Background(), req, RunOptions{})
	if err == nil {
		t.Fatal("expected error from panic")
	}
	var pe *PanicError
	if !errors.As(err, &pe) {
		t.Fatalf("expected PanicError, got %T: %v", err, err)
	}
}

func TestCompile_ForbiddenImport(t *testing.T) {
	t.Parallel()
	src := `package hook

import "os"

func OnRequest(req *Request) error {
	os.Exit(1)
	return nil
}
`
	_, err := Compile("request", src)
	if err == nil {
		t.Fatal("expected error for forbidden import")
	}
	if !strings.Contains(err.Error(), "forbidden import") {
		t.Fatalf("got %v, expected forbidden import error", err)
	}
}

func TestCompile_ForbiddenGoroutine(t *testing.T) {
	t.Parallel()
	src := `package hook

func OnRequest(req *Request) error {
	go func() {}()
	return nil
}
`
	_, err := Compile("request", src)
	if err == nil {
		t.Fatal("expected error for goroutine")
	}
	if !strings.Contains(err.Error(), "goroutine") {
		t.Fatalf("got %v, expected goroutine error", err)
	}
}

func TestCompile_ForbiddenDefer(t *testing.T) {
	t.Parallel()
	src := `package hook

func OnRequest(req *Request) error {
	defer func() {}()
	return nil
}
`
	_, err := Compile("request", src)
	if err == nil {
		t.Fatal("expected error for defer")
	}
}

func TestCompile_ForbiddenChannel(t *testing.T) {
	t.Parallel()
	src := `package hook

func OnRequest(req *Request) error {
	var ch chan int
	_ = ch
	return nil
}
`
	_, err := Compile("request", src)
	if err == nil {
		t.Fatal("expected error for channel type")
	}
}

func TestCompile_ForbiddenRecover(t *testing.T) {
	t.Parallel()
	src := `package hook

func OnRequest(req *Request) error {
	recover()
	return nil
}
`
	_, err := Compile("request", src)
	if err == nil {
		t.Fatal("expected error for recover")
	}
}

func TestCompile_ForbiddenAddressOf(t *testing.T) {
	t.Parallel()
	src := `package hook

func OnRequest(req *Request) error {
	x := 1
	_ = &x
	return nil
}
`
	_, err := Compile("request", src)
	if err == nil {
		t.Fatal("expected error for address-of")
	}
}

func TestCompile_ForbiddenInit(t *testing.T) {
	t.Parallel()
	src := `package hook

func init() {}

func OnRequest(req *Request) error {
	return nil
}
`
	_, err := Compile("request", src)
	if err == nil {
		t.Fatal("expected error for init()")
	}
}

func TestCompile_ForbiddenPackageLevelVar(t *testing.T) {
	t.Parallel()
	src := `package hook

var state = 0

func OnRequest(req *Request) error {
	return nil
}
`
	_, err := Compile("request", src)
	if err == nil {
		t.Fatal("expected error for package-level var")
	}
}

func TestCompile_InvalidPhase(t *testing.T) {
	t.Parallel()
	_, err := Compile("invalid", `package hook
func OnRequest(req *Request) error { return nil }
`)
	if err == nil {
		t.Fatal("expected error for invalid phase")
	}
}

func TestCompile_EmptySource(t *testing.T) {
	t.Parallel()
	_, err := Compile("request", "")
	if err == nil {
		t.Fatal("expected error for empty source")
	}
}

func TestCompile_WrongPackageName(t *testing.T) {
	t.Parallel()
	_, err := Compile("request", `package main
func OnRequest(req *Request) error { return nil }
`)
	if err == nil {
		t.Fatal("expected error for wrong package name")
	}
}

func TestCompile_MissingEntry(t *testing.T) {
	t.Parallel()
	_, err := Compile("request", `package hook
func Helper() error { return nil }
`)
	if err == nil {
		t.Fatal("expected error for missing entry function")
	}
}

func TestCompile_WrongParamCount(t *testing.T) {
	t.Parallel()
	_, err := Compile("request", `package hook
func OnRequest() error { return nil }
`)
	if err == nil {
		t.Fatal("expected error for wrong param count")
	}
}

func TestRunRequest_WrongPhase(t *testing.T) {
	t.Parallel()
	p := compile(t, "response", respHookSetStatus)
	req := &Request{Method: "GET", Path: "/"}
	err := p.RunRequest(context.Background(), req, RunOptions{})
	if err == nil {
		t.Fatal("expected error running response hook as request hook")
	}
}

func TestRunResponse_WrongPhase(t *testing.T) {
	t.Parallel()
	p := compile(t, "request", reqHookAddHeader)
	req := &Request{Method: "GET", Path: "/"}
	resp := &Response{Status: 200}
	err := p.RunResponse(context.Background(), req, resp, RunOptions{})
	if err == nil {
		t.Fatal("expected error running request hook as response hook")
	}
}

func TestStdlib_StringsContains(t *testing.T) {
	t.Parallel()
	src := `package hook

import "strings"

func OnRequest(req *Request) error {
	if strings.Contains(req.Path, "admin") {
		req.SetHeader("X-Admin", "true")
	}
	return nil
}
`
	p := compile(t, "request", src)
	req := &Request{Method: "GET", Path: "/admin/panel", Headers: map[string][]string{}}
	if err := p.RunRequest(context.Background(), req, RunOptions{}); err != nil {
		t.Fatal(err)
	}
	if v := req.Headers["X-Admin"]; len(v) == 0 || v[0] != "true" {
		t.Fatalf("expected X-Admin=true, got %v", v)
	}
}

func TestStdlib_JSON(t *testing.T) {
	t.Parallel()
	src := `package hook

import "encoding/json"

func OnRequest(req *Request) error {
	data, err := json.Decode(req.Body)
	if err != nil {
		return err
	}
	m, ok := data.(map[string]any)
	if !ok {
		return errors.New("expected object")
	}
	_ = m
	return nil
}
`
	p := compile(t, "request", src)
	req := &Request{Method: "POST", Path: "/", Body: []byte(`{"key":"value"}`)}
	if err := p.RunRequest(context.Background(), req, RunOptions{}); err != nil {
		t.Fatal(err)
	}
}

func TestStdlib_Base64(t *testing.T) {
	t.Parallel()
	src := `package hook

import (
	"encoding/base64"
	"strconv"
)

func OnRequest(req *Request) error {
	encoded := base64.EncodeToString(req.Body)
	req.SetHeader("X-B64-Len", strconv.Itoa(int64(len(encoded))))
	return nil
}
`
	p := compile(t, "request", src)
	req := &Request{Method: "POST", Path: "/", Body: []byte("hello"), Headers: map[string][]string{}}
	if err := p.RunRequest(context.Background(), req, RunOptions{}); err != nil {
		t.Fatal(err)
	}
}

func TestRequestMethods_GetHeader(t *testing.T) {
	t.Parallel()
	src := `package hook

func OnRequest(req *Request) error {
	v := req.GetHeader("Content-Type")
	if v == "" {
		req.SetHeader("X-Had-CT", "no")
	} else {
		req.SetHeader("X-Had-CT", "yes")
	}
	return nil
}
`
	p := compile(t, "request", src)
	req := &Request{
		Method:  "GET",
		Path:    "/",
		Headers: map[string][]string{"Content-Type": {"application/json"}},
	}
	if err := p.RunRequest(context.Background(), req, RunOptions{}); err != nil {
		t.Fatal(err)
	}
	if v := req.Headers["X-Had-Ct"]; len(v) == 0 || v[0] != "yes" {
		t.Fatalf("expected X-Had-Ct=yes, got %v", v)
	}
}

func TestRequestMethods_AddHeader(t *testing.T) {
	t.Parallel()
	src := `package hook

func OnRequest(req *Request) error {
	req.AddHeader("X-Multi", "a")
	req.AddHeader("X-Multi", "b")
	return nil
}
`
	p := compile(t, "request", src)
	req := &Request{Method: "GET", Path: "/", Headers: map[string][]string{}}
	if err := p.RunRequest(context.Background(), req, RunOptions{}); err != nil {
		t.Fatal(err)
	}
	v := req.Headers["X-Multi"]
	if len(v) != 2 || v[0] != "a" || v[1] != "b" {
		t.Fatalf("expected [a b], got %v", v)
	}
}

func TestRequestMethods_DelHeader(t *testing.T) {
	t.Parallel()
	src := `package hook

func OnRequest(req *Request) error {
	req.DelHeader("X-Remove-Me")
	return nil
}
`
	p := compile(t, "request", src)
	req := &Request{
		Method:  "GET",
		Path:    "/",
		Headers: map[string][]string{"X-Remove-Me": {"val"}},
	}
	if err := p.RunRequest(context.Background(), req, RunOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, ok := req.Headers["X-Remove-Me"]; ok {
		t.Fatal("header should have been deleted")
	}
}

func TestResponse_DelHeaderReadOnly(t *testing.T) {
	t.Parallel()
	src := `package hook

func OnResponse(req *Request, resp *Response) error {
	req.DelHeader("X-Remove")
	return nil
}
`
	p := compile(t, "response", src)
	req := &Request{Method: "GET", Path: "/", Headers: map[string][]string{"X-Remove": {"v"}}}
	resp := &Response{Status: 200}
	err := p.RunResponse(context.Background(), req, resp, RunOptions{})
	if !errors.Is(err, ErrForbiddenMutation) {
		t.Fatalf("got %v, want ErrForbiddenMutation", err)
	}
}

func TestResponse_ModifyRespHeaders(t *testing.T) {
	t.Parallel()
	src := `package hook

func OnResponse(req *Request, resp *Response) error {
	resp.SetHeader("X-Added", "yes")
	resp.DelHeader("X-Old")
	return nil
}
`
	p := compile(t, "response", src)
	req := &Request{Method: "GET", Path: "/"}
	resp := &Response{Status: 200, Headers: map[string][]string{"X-Old": {"v"}}}
	if err := p.RunResponse(context.Background(), req, resp, RunOptions{}); err != nil {
		t.Fatal(err)
	}
	if v := resp.Headers["X-Added"]; len(v) == 0 || v[0] != "yes" {
		t.Fatalf("expected X-Added=yes, got %v", v)
	}
	if _, ok := resp.Headers["X-Old"]; ok {
		t.Fatal("X-Old should have been deleted")
	}
}

func TestResponse_ModifyBody(t *testing.T) {
	t.Parallel()
	src := `package hook

func OnResponse(req *Request, resp *Response) error {
	resp.Body = []byte("replaced")
	return nil
}
`
	p := compile(t, "response", src)
	req := &Request{Method: "GET", Path: "/"}
	resp := &Response{Status: 200, Body: []byte("original")}
	if err := p.RunResponse(context.Background(), req, resp, RunOptions{}); err != nil {
		t.Fatal(err)
	}
	if string(resp.Body) != "replaced" {
		t.Fatalf("body = %q, want replaced", resp.Body)
	}
}

func TestConst(t *testing.T) {
	t.Parallel()
	src := `package hook

const marker = "CONST_VAL"

func OnRequest(req *Request) error {
	req.SetHeader("X-Marker", marker)
	return nil
}
`
	p := compile(t, "request", src)
	req := &Request{Method: "GET", Path: "/", Headers: map[string][]string{}}
	if err := p.RunRequest(context.Background(), req, RunOptions{}); err != nil {
		t.Fatal(err)
	}
	if v := req.Headers["X-Marker"]; len(v) == 0 || v[0] != "CONST_VAL" {
		t.Fatalf("got %v, want CONST_VAL", v)
	}
}

func TestHelperFunction(t *testing.T) {
	t.Parallel()
	src := `package hook

func addTag(req *Request, tag string) {
	req.AddHeader("X-Tag", tag)
}

func OnRequest(req *Request) error {
	addTag(req, "one")
	addTag(req, "two")
	return nil
}
`
	p := compile(t, "request", src)
	req := &Request{Method: "GET", Path: "/", Headers: map[string][]string{}}
	if err := p.RunRequest(context.Background(), req, RunOptions{}); err != nil {
		t.Fatal(err)
	}
	v := req.Headers["X-Tag"]
	if len(v) != 2 {
		t.Fatalf("expected 2 tags, got %v", v)
	}
}

func TestReturnError(t *testing.T) {
	t.Parallel()
	src := `package hook

import "errors"

func OnRequest(req *Request) error {
	return errors.New("intentional failure")
}
`
	p := compile(t, "request", src)
	req := &Request{Method: "GET", Path: "/"}
	err := p.RunRequest(context.Background(), req, RunOptions{})
	if err == nil {
		t.Fatal("expected error return from hook")
	}
	if !strings.Contains(err.Error(), "intentional failure") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNilContext(t *testing.T) {
	t.Parallel()
	p := compile(t, "request", reqHookAddHeader)
	req := &Request{Method: "GET", Path: "/", Headers: map[string][]string{}}
	//nolint:staticcheck // intentional nil context
	if err := p.RunRequest(nil, req, RunOptions{}); err != nil {
		t.Fatal(err)
	}
}

func TestResponse_NilHeaders(t *testing.T) {
	t.Parallel()
	src := `package hook

func OnResponse(req *Request, resp *Response) error {
	resp.SetHeader("X-Test", "val")
	return nil
}
`
	p := compile(t, "response", src)
	req := &Request{Method: "GET", Path: "/"}
	resp := &Response{Status: 200}
	if err := p.RunResponse(context.Background(), req, resp, RunOptions{}); err != nil {
		t.Fatal(err)
	}
	if resp.Headers == nil {
		t.Fatal("resp headers should be initialized")
	}
}
