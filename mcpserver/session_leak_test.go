package mcpserver_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/SREsAreHumanToo/ARTEX-CC/mcpserver"
	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func TestListSessionsCannotBeReplayedToStealClaim(t *testing.T) {
	t.Parallel()
	st := &claimStore{
		nodes:  map[int64]*tools.Node{10: {ID: 10, Kind: tools.KindIntent, State: "open"}},
		claims: map[int64]tools.IntentClaim{},
	}
	reg := tools.NewRegistry()
	tools.RegisterIntentTools(reg, st, 1)
	srv := mcpserver.New(reg)
	ts := httptest.NewServer(server.NewStreamableHTTPServer(srv.MCPServer()))
	t.Cleanup(ts.Close)
	addr := strings.TrimPrefix(ts.URL, "http://")

	call := func(sid, tool string, args map[string]any) (string, bool) {
		_, rr := doRPC(t, addr, sid, 2, "tools/call", map[string]any{"name": tool, "arguments": args})
		if rr.Error != nil {
			return rr.Error.Message, true
		}
		var res mcp.CallToolResult
		if json.Unmarshal(rr.Result, &res) != nil || len(res.Content) == 0 {
			return "", true
		}
		text, _ := mcp.AsTextContent(res.Content[0])
		return text.Text, res.IsError
	}
	a, b := initSession(t, addr), initSession(t, addr)

	if out, isErr := call(a, "start_intent", map[string]any{"node_id": 10}); isErr {
		t.Fatalf("session A claim = %q", out)
	}
	listing, isErr := call(b, "list_sessions", map[string]any{})
	if isErr {
		t.Fatalf("list_sessions = %q", listing)
	}
	if strings.Contains(listing, a) {
		t.Fatalf("list_sessions exposes session A's ID to B: %s", listing)
	}

	var payload struct {
		Sessions []struct {
			Label string `json:"label"`
		} `json:"sessions"`
	}
	if err := json.Unmarshal([]byte(listing), &payload); err != nil {
		t.Fatal(err)
	}
	for _, s := range payload.Sessions {
		body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 3, "method": "tools/call",
			"params": map[string]any{"name": "complete_intent", "arguments": map[string]any{"node_id": 10}}})
		req, _ := http.NewRequest(http.MethodPost, ts.URL+"/mcp", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		req.Header.Set("Mcp-Session-Id", s.Label)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			t.Errorf("label %q was accepted as a session ID", s.Label)
		}
	}
	if got := st.owner(10); got != a {
		t.Errorf("claim owner = %q, want session A", got)
	}
}

type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// Not parallel: it swaps the default slog logger.
func TestSessionDebugLogsOmitRawID(t *testing.T) {
	var buf syncBuffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	srv := mcpserver.New(tools.NewRegistry())
	ts := httptest.NewServer(server.NewStreamableHTTPServer(srv.MCPServer()))
	t.Cleanup(ts.Close)
	addr := strings.TrimPrefix(ts.URL, "http://")

	sid := initSession(t, addr)
	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/mcp", nil)
	req.Header.Set("Mcp-Session-Id", sid)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	logs := buf.String()
	label := tools.OwnerLabel(sid)
	if !strings.Contains(logs, `msg="session registered" session=`+label) {
		t.Errorf("registration log lacks the session label %s:\n%s", label, logs)
	}
	if !strings.Contains(logs, `msg="session unregistered" session=`+label) {
		t.Errorf("unregistration log lacks the session label %s:\n%s", label, logs)
	}
	if strings.Contains(logs, sid) {
		t.Errorf("logs contain the raw session ID %s:\n%s", sid, logs)
	}
}
