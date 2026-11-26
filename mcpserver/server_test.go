package mcpserver_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"

	"github.com/SREsAreHumanToo/ARTEX-CC/mcpserver"
	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
)

func echoTool() tools.Tool {
	return tools.Tool{
		Name:        "echo",
		Description: "echoes input",
		Schema:      map[string]any{"type": "object", "properties": map[string]any{"msg": map[string]any{"type": "string"}}},
		Run: func(_ context.Context, in json.RawMessage) (tools.Result, error) {
			return tools.Result{Content: string(in)}, nil
		},
	}
}

func pingTool() tools.Tool {
	return tools.Tool{
		Name:        "ping",
		Description: "returns pong",
		Schema:      map[string]any{"type": "object", "properties": map[string]any{}},
		Run: func(_ context.Context, _ json.RawMessage) (tools.Result, error) {
			return tools.Result{Content: "pong"}, nil
		},
	}
}

func startTestServer(t *testing.T, reg *tools.Registry) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	srv := mcpserver.New(reg)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go srv.Serve(ctx, ln)

	return ln.Addr().String()
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func doRPC(t *testing.T, addr, sessionID string, id int, method string, params any) (*http.Response, rpcResponse) {
	t.Helper()
	body := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
	}
	if params != nil {
		body["params"] = params
	}
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest("POST", "http://"+addr+"/mcp", bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if sessionID != "" {
		req.Header.Set("Mcp-Session-Id", sessionID)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	var rr rpcResponse
	if err := json.Unmarshal(data, &rr); err != nil {
		t.Fatalf("unmarshal RPC response: %v\nraw: %s", err, data)
	}
	return resp, rr
}

func initSession(t *testing.T, addr string) string {
	t.Helper()
	body := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2025-03-26",
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "test", "version": "1.0"},
		},
	}
	raw, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "http://"+addr+"/mcp", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	io.ReadAll(resp.Body)

	sid := resp.Header.Get("Mcp-Session-Id")
	if sid == "" {
		t.Fatal("initialize did not return Mcp-Session-Id header")
	}
	return sid
}

func testRegistry() *tools.Registry {
	reg := tools.NewRegistry()
	reg.Register(echoTool())
	reg.Register(pingTool())
	return reg
}

func TestToolsList(t *testing.T) {
	t.Parallel()
	addr := startTestServer(t, testRegistry())
	sid := initSession(t, addr)

	_, rr := doRPC(t, addr, sid, 2, "tools/list", map[string]any{})
	if rr.Error != nil {
		t.Fatalf("tools/list returned error: %s", rr.Error.Message)
	}
	if rr.Result == nil {
		t.Fatal("tools/list returned nil result")
	}

	var result struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(rr.Result, &result); err != nil {
		t.Fatalf("unmarshal tools result: %v", err)
	}

	names := map[string]bool{}
	for _, tool := range result.Tools {
		names[tool.Name] = true
	}
	for _, want := range []string{"echo", "ping"} {
		if !names[want] {
			t.Errorf("tools/list missing tool %q; got %v", want, names)
		}
	}
}

func TestToolsCall(t *testing.T) {
	t.Parallel()
	addr := startTestServer(t, testRegistry())
	sid := initSession(t, addr)

	_, rr := doRPC(t, addr, sid, 3, "tools/call", map[string]any{
		"name":      "echo",
		"arguments": map[string]any{"msg": "hello"},
	})
	if rr.Error != nil {
		t.Fatalf("tools/call returned error: %s", rr.Error.Message)
	}
	if rr.Result == nil {
		t.Fatal("tools/call returned nil result")
	}

	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(rr.Result, &result); err != nil {
		t.Fatalf("unmarshal call result: %v", err)
	}
	if len(result.Content) == 0 {
		t.Fatal("tools/call returned empty content")
	}
	if !strings.Contains(result.Content[0].Text, "hello") {
		t.Errorf("tools/call content = %q, want substring %q", result.Content[0].Text, "hello")
	}
}

func TestToolsCallNotFound(t *testing.T) {
	t.Parallel()
	addr := startTestServer(t, testRegistry())
	sid := initSession(t, addr)

	_, rr := doRPC(t, addr, sid, 4, "tools/call", map[string]any{
		"name":      "nonexistent_tool",
		"arguments": map[string]any{},
	})
	if rr.Error == nil {
		t.Fatal("expected error for nonexistent tool, got success")
	}
	if !strings.Contains(rr.Error.Message, "not found") {
		t.Errorf("error message = %q, want substring %q", rr.Error.Message, "not found")
	}
}
