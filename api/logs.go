package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	logBufferSize = 1000
	logsTopic     = "logs"
)

type LogEntry struct {
	Seq     int            `json:"seq"`
	Level   string         `json:"level"`
	Time    string         `json:"time"`
	Message string         `json:"message"`
	Attrs   map[string]any `json:"attrs"`
}

type LogBuffer struct {
	mu      sync.Mutex
	entries []LogEntry
	next    int
	seq     int
	hub     *Hub
}

func NewLogBuffer(hub *Hub) *LogBuffer {
	return &LogBuffer{entries: make([]LogEntry, 0, logBufferSize), hub: hub}
}

func (b *LogBuffer) add(e LogEntry) {
	b.mu.Lock()
	b.seq++
	e.Seq = b.seq
	if len(b.entries) < logBufferSize {
		b.entries = append(b.entries, e)
	} else {
		b.entries[b.next] = e
		b.next = (b.next + 1) % logBufferSize
	}
	b.mu.Unlock()

	if data, err := json.Marshal(e); err == nil {
		b.hub.Publish(logsTopic, data)
	}
}

func (b *LogBuffer) Entries(after int, minLevel string) []LogEntry {
	min := levelRank(minLevel)
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]LogEntry, 0, len(b.entries))
	for i := range b.entries {
		e := b.entries[(b.next+i)%len(b.entries)]
		if e.Seq > after && levelRank(e.Level) >= min {
			out = append(out, e)
		}
	}
	return out
}

func levelRank(level string) int {
	switch strings.ToLower(level) {
	case "info":
		return 1
	case "warn":
		return 2
	case "error":
		return 3
	}
	return 0
}

func levelName(l slog.Level) string {
	switch {
	case l >= slog.LevelError:
		return "error"
	case l >= slog.LevelWarn:
		return "warn"
	case l >= slog.LevelInfo:
		return "info"
	}
	return "debug"
}

type logTee struct {
	next   slog.Handler
	buf    *LogBuffer
	attrs  []slog.Attr
	groups []string
}

func (t *logTee) Enabled(ctx context.Context, l slog.Level) bool {
	return t.next.Enabled(ctx, l)
}

func (t *logTee) Handle(ctx context.Context, r slog.Record) error {
	attrs := map[string]any{}
	for _, a := range t.attrs {
		addAttr(attrs, a)
	}
	r.Attrs(func(a slog.Attr) bool {
		addAttr(attrs, nested(t.groups, a))
		return true
	})
	ts := r.Time
	if ts.IsZero() {
		ts = time.Now()
	}
	t.buf.add(LogEntry{
		Level:   levelName(r.Level),
		Time:    ts.Format(time.RFC3339Nano),
		Message: r.Message,
		Attrs:   attrs,
	})
	return t.next.Handle(ctx, r)
}

func (t *logTee) WithAttrs(attrs []slog.Attr) slog.Handler {
	c := *t
	c.next = t.next.WithAttrs(attrs)
	nestedAttrs := make([]slog.Attr, len(attrs))
	for i, a := range attrs {
		nestedAttrs[i] = nested(t.groups, a)
	}
	c.attrs = append(append([]slog.Attr(nil), t.attrs...), nestedAttrs...)
	return &c
}

func (t *logTee) WithGroup(name string) slog.Handler {
	c := *t
	c.next = t.next.WithGroup(name)
	c.groups = append(append([]string(nil), t.groups...), name)
	return &c
}

func nested(groups []string, a slog.Attr) slog.Attr {
	for i := len(groups) - 1; i >= 0; i-- {
		a = slog.Group(groups[i], a)
	}
	return a
}

func addAttr(dst map[string]any, a slog.Attr) {
	v := a.Value.Resolve()
	if v.Kind() == slog.KindGroup {
		if a.Key == "" {
			for _, g := range v.Group() {
				addAttr(dst, g)
			}
			return
		}
		sub, _ := dst[a.Key].(map[string]any)
		if sub == nil {
			sub = map[string]any{}
		}
		for _, g := range v.Group() {
			addAttr(sub, g)
		}
		dst[a.Key] = sub
		return
	}
	switch x := v.Any().(type) {
	case error:
		dst[a.Key] = x.Error()
	case fmt.Stringer:
		dst[a.Key] = x.String()
	case nil, bool, string, int, int64, uint64, float64:
		dst[a.Key] = x
	default:
		if _, err := json.Marshal(x); err == nil {
			dst[a.Key] = x
		} else {
			dst[a.Key] = fmt.Sprint(x)
		}
	}
}

func (a *API) registerLogsRoutes(mux *http.ServeMux) {
	a.logs = NewLogBuffer(a.hub)
	base := slog.Default().Handler()
	if t, ok := base.(*logTee); ok {
		base = t.next
	}
	// slog.SetDefault routes the log package into the new handler, and slog's built-in default handler writes back through the log package, which would recurse.
	if fmt.Sprintf("%T", base) == "*slog.defaultHandler" {
		base = slog.NewTextHandler(os.Stderr, nil)
	}
	slog.SetDefault(slog.New(&logTee{next: base, buf: a.logs}))

	mux.HandleFunc("GET /api/logs", a.handleLogs)
	mux.HandleFunc("GET /api/logs/stream", a.handleLogStream)
	mux.HandleFunc("GET /api/system", a.handleSystem)
}

func (a *API) handleLogs(w http.ResponseWriter, r *http.Request) {
	after := parseIntParam(r, "after", 0)
	writeJSON(w, http.StatusOK, a.logs.Entries(after, r.URL.Query().Get("level")))
}

func (a *API) handleLogStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	after := parseIntParam(r, "after", 0)
	if id, err := strconv.Atoi(r.Header.Get("Last-Event-ID")); err == nil && id > after {
		after = id
	}
	min := levelRank(r.URL.Query().Get("level"))

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Write([]byte(": connected\n\n"))
	flusher.Flush()

	ch, unsubscribe := a.hub.Subscribe(logsTopic)
	defer unsubscribe()

	send := func(e LogEntry) {
		data, err := json.Marshal(e)
		if err != nil {
			return
		}
		fmt.Fprintf(w, "id: %d\ndata: %s\n\n", e.Seq, data)
		flusher.Flush()
		after = e.Seq
	}

	if r.URL.Query().Has("after") || r.Header.Get("Last-Event-ID") != "" {
		for _, e := range a.logs.Entries(after, "") {
			if levelRank(e.Level) >= min {
				send(e)
			}
		}
	}

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case msg := <-ch:
			var e LogEntry
			if json.Unmarshal(msg, &e) != nil || e.Seq <= after || levelRank(e.Level) < min {
				continue
			}
			send(e)
		case <-ticker.C:
			w.Write([]byte(": keepalive\n\n"))
			flusher.Flush()
		}
	}
}

func (a *API) handleSystem(w http.ResponseWriter, r *http.Request) {
	version := ""
	if bi, ok := debug.ReadBuildInfo(); ok {
		version = bi.Main.Version
	}
	dataDir := ""
	if a.files != nil {
		dataDir = a.files.root
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"version":    version,
		"go_version": runtime.Version(),
		"os":         runtime.GOOS,
		"arch":       runtime.GOARCH,
		"data_dir":   dataDir,
	})
}
