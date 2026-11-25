package mcpserver_test

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/SREsAreHumanToo/ARTEX-CC/mcpserver"
	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
	"github.com/mark3labs/mcp-go/server"
)

// Not parallel: it swaps the default slog logger.
func captureLog(t *testing.T) *syncBuffer {
	t.Helper()
	buf := &syncBuffer{}
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return buf
}

func assertRedacted(t *testing.T, out, sid string) {
	t.Helper()
	if strings.Contains(out, sid) || strings.Contains(out, "mcp-session-") {
		t.Fatalf("raw session ID in mcp-go log: %s", out)
	}
	if !strings.Contains(out, tools.OwnerLabel(sid)) {
		t.Fatalf("label for session missing from mcp-go log: %s", out)
	}
}

func TestStreamableHTTPRejectedGETLogRedactsSessionID(t *testing.T) {
	buf := captureLog(t)
	srv := mcpserver.New(testRegistry())
	ts := httptest.NewServer(srv.StreamableHTTP(server.WithDisableStreaming(true)))
	t.Cleanup(ts.Close)
	sid := initSession(t, strings.TrimPrefix(ts.URL, "http://"))

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/mcp", nil)
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Mcp-Session-Id", sid)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("GET status = %d, want 405", resp.StatusCode)
	}

	out := buf.String()
	if !strings.Contains(out, "Rejected GET request") {
		t.Fatalf("mcp-go did not log the rejected GET: %s", out)
	}
	assertRedacted(t, out, sid)
}

type refusingSessions struct {
	server.InsecureStatefulSessionIdManager
}

func (*refusingSessions) Terminate(sessionID string) (bool, error) {
	return false, fmt.Errorf("terminate %s refused", sessionID)
}

func TestStreamableHTTPCloseSessionsLogRedactsSessionID(t *testing.T) {
	buf := captureLog(t)
	srv := mcpserver.New(testRegistry())
	streamable := srv.StreamableHTTP(server.WithSessionIdManager(&refusingSessions{}))
	ts := httptest.NewServer(streamable)
	t.Cleanup(ts.Close)
	addr := strings.TrimPrefix(ts.URL, "http://")
	sid := initSession(t, addr)
	doRPC(t, addr, sid, 2, "tools/list", map[string]any{})

	streamable.CloseSessions(context.Background())

	out := buf.String()
	if !strings.Contains(out, "failed to terminate session during CloseSessions") {
		t.Fatalf("mcp-go did not log the CloseSessions failure: %s", out)
	}
	assertRedacted(t, out, sid)
}
