package api

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
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

func TestHandleLogs_OK(t *testing.T) {
	a := newTestAPI(&mockStore{})
	h := a.Handler()

	slog.Info("test log entry for handler")

	req := httptest.NewRequest(http.MethodGet, "/api/logs", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", rec.Code)
	}
	var entries []LogEntry
	json.NewDecoder(rec.Body).Decode(&entries)
	if len(entries) == 0 {
		t.Fatal("expected at least one log entry")
	}
}

func TestHandleLogs_AfterParam(t *testing.T) {
	a := newTestAPI(&mockStore{})
	h := a.Handler()
	a.logs.add(LogEntry{Level: "info", Message: "first"})
	a.logs.add(LogEntry{Level: "info", Message: "second"})

	req := httptest.NewRequest(http.MethodGet, "/api/logs?after=1", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", rec.Code)
	}
	var entries []LogEntry
	json.NewDecoder(rec.Body).Decode(&entries)
	for _, e := range entries {
		if e.Seq <= 1 {
			t.Fatalf("got entry with seq %d, expected only after 1", e.Seq)
		}
	}
}

func TestHandleLogs_LevelFilter(t *testing.T) {
	a := newTestAPI(&mockStore{})
	h := a.Handler()
	a.logs.add(LogEntry{Level: "info", Message: "info msg"})
	a.logs.add(LogEntry{Level: "error", Message: "error msg"})

	req := httptest.NewRequest(http.MethodGet, "/api/logs?level=error", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	var entries []LogEntry
	json.NewDecoder(rec.Body).Decode(&entries)
	for _, e := range entries {
		if e.Level != "error" {
			t.Fatalf("got level %q, want only error", e.Level)
		}
	}
}

func TestHandleSystem_OK(t *testing.T) {
	a := newTestAPI(&mockStore{})
	h := a.Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/system", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", rec.Code)
	}
	body := decodeRecorder(t, rec)
	if body["go_version"] == nil || body["os"] == nil || body["arch"] == nil {
		t.Fatalf("missing fields: %v", body)
	}
}

func TestHandleSystem_WithDataDir(t *testing.T) {
	dir := t.TempDir()
	a := New(&mockStore{}, nil, 1, 1, WithDataDir(dir))
	h := a.Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/system", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	body := decodeRecorder(t, rec)
	if body["data_dir"] != dir {
		t.Fatalf("data_dir: want %s, got %v", dir, body["data_dir"])
	}
}

func TestHandleLogStream_OK(t *testing.T) {
	a := newTestAPI(&mockStore{})
	srv := httptest.NewServer(a.Handler())
	defer srv.Close()

	a.logs.add(LogEntry{Level: "info", Message: "pre-existing"})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, _ := http.NewRequest("GET", srv.URL+"/api/logs/stream?after=0", nil)
	resp, err := http.DefaultClient.Do(req.WithContext(ctx))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("content type = %s", ct)
	}

	go func() {
		time.Sleep(200 * time.Millisecond)
		a.logs.add(LogEntry{Level: "warn", Message: "live event"})
	}()

	var lines []string
	sc := bufio.NewScanner(resp.Body)
	for sc.Scan() {
		line := sc.Text()
		if line != "" {
			lines = append(lines, line)
		}
		if strings.Contains(strings.Join(lines, "|"), "live event") {
			break
		}
	}
	got := strings.Join(lines, "|")
	if !strings.Contains(got, "pre-existing") {
		t.Fatalf("missing pre-existing in stream: %s", got)
	}
	if !strings.Contains(got, "live event") {
		t.Fatalf("missing live event in stream: %s", got)
	}
}

func TestLevelRankDebug(t *testing.T) {
	if r := levelRank("debug"); r != 0 {
		t.Fatalf("debug rank: want 0, got %d", r)
	}
	if r := levelRank("warn"); r != 2 {
		t.Fatalf("warn rank: want 2, got %d", r)
	}
}

func TestLevelNameDebug(t *testing.T) {
	if n := levelName(slog.LevelDebug); n != "debug" {
		t.Fatalf("want debug, got %s", n)
	}
	if n := levelName(slog.LevelWarn); n != "warn" {
		t.Fatalf("want warn, got %s", n)
	}
}

func TestAddAttr_ErrorValue(t *testing.T) {
	dst := map[string]any{}
	a := slog.Any("err", &bytes.Buffer{})
	addAttr(dst, a)
	if dst["err"] == nil {
		t.Fatal("expected non-nil")
	}
}

func TestAddAttr_NilAndBool(t *testing.T) {
	dst := map[string]any{}
	addAttr(dst, slog.Bool("flag", true))
	addAttr(dst, slog.Any("none", nil))
	if dst["flag"] != true {
		t.Fatalf("flag: %v", dst["flag"])
	}
}

func TestAddAttr_ErrorType(t *testing.T) {
	dst := map[string]any{}
	addAttr(dst, slog.Any("e", errors.New("oops")))
	if dst["e"] != "oops" {
		t.Fatalf("e: %v", dst["e"])
	}
}

func TestAddAttr_GroupEmpty(t *testing.T) {
	dst := map[string]any{}
	addAttr(dst, slog.Group("", slog.String("k", "v")))
	if dst["k"] != "v" {
		t.Fatalf("flat group: %v", dst)
	}
}

func TestHandleLogStream_LastEventID(t *testing.T) {
	a := newTestAPI(&mockStore{})
	srv := httptest.NewServer(a.Handler())
	defer srv.Close()

	a.logs.add(LogEntry{Level: "info", Message: "first"})
	a.logs.add(LogEntry{Level: "info", Message: "second"})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, _ := http.NewRequest("GET", srv.URL+"/api/logs/stream", nil)
	req.Header.Set("Last-Event-ID", "1")
	resp, err := http.DefaultClient.Do(req.WithContext(ctx))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	go func() {
		time.Sleep(200 * time.Millisecond)
		cancel()
	}()

	var lines []string
	sc := bufio.NewScanner(resp.Body)
	for sc.Scan() {
		line := sc.Text()
		if strings.Contains(line, "second") {
			lines = append(lines, line)
			break
		}
	}
	if len(lines) == 0 {
		t.Fatal("expected to receive 'second' event via Last-Event-ID backfill")
	}
}

func TestHandleLogStream_LevelFilter(t *testing.T) {
	a := newTestAPI(&mockStore{})
	srv := httptest.NewServer(a.Handler())
	defer srv.Close()

	a.logs.add(LogEntry{Level: "info", Message: "info msg"})
	a.logs.add(LogEntry{Level: "error", Message: "error msg"})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, _ := http.NewRequest("GET", srv.URL+"/api/logs/stream?after=0&level=error", nil)
	resp, err := http.DefaultClient.Do(req.WithContext(ctx))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	go func() {
		time.Sleep(200 * time.Millisecond)
		cancel()
	}()

	var found bool
	sc := bufio.NewScanner(resp.Body)
	for sc.Scan() {
		line := sc.Text()
		if strings.Contains(line, "info msg") {
			t.Fatal("info msg should be filtered out")
		}
		if strings.Contains(line, "error msg") {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected error msg in stream")
	}
}
