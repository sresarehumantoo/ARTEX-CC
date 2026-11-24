package api

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestLogTeeCapturesAndForwards(t *testing.T) {
	var out bytes.Buffer
	buf := NewLogBuffer(NewHub())
	l := slog.New(&logTee{next: slog.NewTextHandler(&out, nil), buf: buf}).With("req", 7).WithGroup("g")

	l.Info("hello", "k", "v")
	l.Debug("hidden")
	l.Error("boom", "err", "x")

	if !strings.Contains(out.String(), "hello") {
		t.Fatalf("underlying handler did not receive record: %q", out.String())
	}
	got := buf.Entries(0, "")
	if len(got) != 2 {
		t.Fatalf("entries = %d, want 2 (debug disabled by base handler)", len(got))
	}
	if got[0].Level != "info" || got[0].Attrs["req"] != int64(7) {
		t.Fatalf("unexpected first entry: %+v", got[0])
	}
	if g, _ := got[0].Attrs["g"].(map[string]any); g["k"] != "v" {
		t.Fatalf("group attr missing: %+v", got[0].Attrs)
	}
	if e := buf.Entries(0, "error"); len(e) != 1 || e[0].Message != "boom" {
		t.Fatalf("level filter failed: %+v", e)
	}
	if e := buf.Entries(got[0].Seq, ""); len(e) != 1 {
		t.Fatalf("after filter failed: %+v", e)
	}
}

func TestLogBufferWraps(t *testing.T) {
	buf := NewLogBuffer(NewHub())
	for i := 0; i < logBufferSize+10; i++ {
		buf.add(LogEntry{Level: "info"})
	}
	got := buf.Entries(0, "")
	if len(got) != logBufferSize || got[0].Seq != 11 || got[len(got)-1].Seq != logBufferSize+10 {
		t.Fatalf("wrap order wrong: len=%d first=%d last=%d", len(got), got[0].Seq, got[len(got)-1].Seq)
	}
}
