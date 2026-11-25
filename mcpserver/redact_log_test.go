package mcpserver

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
)

func captureDefaultLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &buf
}

func TestSessionRedactingLoggerMCPGoCallSites(t *testing.T) {
	sid := "mcp-session-0f8fad5b-d9cb-469f-a165-70867728950e"
	label := tools.OwnerLabel(sid)
	errWithID := fmt.Errorf("session not found: %s", sid)

	cases := []struct {
		name string
		log  func(l *slog.Logger)
	}{
		{"rejected GET", func(l *slog.Logger) {
			l.Info("Rejected GET request: streaming is disabled", "session", sid)
		}},
		{"sampling response", func(l *slog.Logger) {
			l.Info("Delivered sampling response", "session", sid, "request", int64(4))
		}},
		{"expired session sweep", func(l *slog.Logger) {
			l.Info("Sweeping expired session", "session", sid)
		}},
		{"CloseSessions", func(l *slog.Logger) {
			l.Warn("failed to terminate session during CloseSessions", "sessionID", sid, "err", errWithID)
		}},
		{"resume stream", func(l *slog.Logger) {
			l.Error("Failed to resume stream", "err", errWithID, "session", sid)
		}},
		{"purge events", func(l *slog.Logger) {
			l.Error("Failed to purge session events", "err", errors.New("store down"), "session", sid)
		}},
		{"ID in message", func(l *slog.Logger) {
			l.Error("panic in session " + sid)
		}},
		{"ID in With attrs and groups", func(l *slog.Logger) {
			l.With("peer", sid).WithGroup("g").Info("event", slog.Group("inner", "id", sid))
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			buf := captureDefaultLog(t)
			tc.log(newSessionRedactingLogger())
			out := buf.String()
			if strings.Contains(out, "mcp-session-") {
				t.Fatalf("raw session ID in log: %s", out)
			}
			if !strings.Contains(out, label) {
				t.Fatalf("label %s missing from log: %s", label, out)
			}
		})
	}
}

func TestSessionRedactingLoggerLabelsAnySessionAttr(t *testing.T) {
	buf := captureDefaultLog(t)
	newSessionRedactingLogger().Info("Rejected GET request: streaming is disabled", "session", "client-chosen")
	if out := buf.String(); strings.Contains(out, "client-chosen") || !strings.Contains(out, tools.OwnerLabel("client-chosen")) {
		t.Fatalf("session attribute not labeled: %s", out)
	}
}

func TestSessionRedactingLoggerPassesThrough(t *testing.T) {
	buf := captureDefaultLog(t)
	l := newSessionRedactingLogger()
	l.Error("Failed to write SSE event", "err", errors.New("broken pipe"), "stream", "abc")
	out := buf.String()
	for _, want := range []string{"level=ERROR", `msg="Failed to write SSE event"`, `err="broken pipe"`, "stream=abc"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in %s", want, out)
		}
	}

	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelWarn})))
	t.Cleanup(func() { slog.SetDefault(prev) })
	buf.Reset()
	l.Info("Sweeping expired session", "session", "x")
	if buf.Len() != 0 {
		t.Fatalf("info logged below the default handler's level: %s", buf)
	}
}

func TestSessionRedactingLoggerRedactsIdentityTokens(t *testing.T) {
	token := "ai1.eyJuIjoiYWxpY2UifQ.c2lnbmF0dXJlLWJ5dGVzLWhlcmU"
	buf := captureDefaultLog(t)
	l := newSessionRedactingLogger()
	l.Info("header X-Artex-Identity: "+token, "value", token, "err", fmt.Errorf("bad %s", token))
	l.With("h", "Bearer x "+token).WithGroup("g").Info("event", slog.Group("inner", "id", token))
	out := buf.String()
	if strings.Contains(out, "eyJuIjoiYWxpY2UifQ") || strings.Contains(out, "c2lnbmF0dXJl") {
		t.Fatalf("identity token in log: %s", out)
	}
	if strings.Count(out, redactedIdentityToken) != 5 {
		t.Fatalf("want 5 redactions in: %s", out)
	}
}
