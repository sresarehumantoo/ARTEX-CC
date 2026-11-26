package api

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/SREsAreHumanToo/ARTEX-CC/config"
	"github.com/SREsAreHumanToo/ARTEX-CC/identity"
	"github.com/SREsAreHumanToo/ARTEX-CC/mcpserver"
)

func newTestChatStore(t *testing.T) *ChatStore {
	t.Helper()
	return NewChatStore(NewFileStore(t.TempDir()))
}

func TestChatStoreLifecycle(t *testing.T) {
	s := newTestChatStore(t)

	first, err := s.Create(CreateConversationRequest{Title: "one", Model: "sonnet"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.Create(CreateConversationRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if second.Title == "" || second.Status != ChatStatusIdle {
		t.Fatalf("unexpected defaults: %+v", second)
	}

	if _, err := s.Update(first.ID, func(c *Conversation) { c.Title = "renamed" }); err != nil {
		t.Fatal(err)
	}
	list, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].ID != first.ID || list[0].Title != "renamed" {
		t.Fatalf("list order or content wrong: %+v", list)
	}

	for _, text := range []string{"a", "b", "c"} {
		if _, err := s.AppendEvent(first.ID, ChatEvent{Kind: ChatKindText, Text: text}); err != nil {
			t.Fatal(err)
		}
	}
	all, err := s.Events(first.ID, 0)
	if err != nil || len(all) != 3 || all[2].Seq != 3 {
		t.Fatalf("events = %+v, err = %v", all, err)
	}
	tail, err := s.Events(first.ID, 2)
	if err != nil || len(tail) != 1 || tail[0].Text != "c" {
		t.Fatalf("tail = %+v, err = %v", tail, err)
	}

	reopened := NewChatStore(s.files)
	ev, err := reopened.AppendEvent(first.ID, ChatEvent{Kind: ChatKindText, Text: "d"})
	if err != nil || ev.Seq != 4 {
		t.Fatalf("seq after reopen = %d, err = %v", ev.Seq, err)
	}

	if err := s.Delete(first.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(first.ID); err != ErrConversationNotFound {
		t.Fatalf("expected not found, got %v", err)
	}
	if _, err := s.Get("../etc"); err != ErrConversationNotFound {
		t.Fatalf("expected not found for bad id, got %v", err)
	}
}

func TestChatStoreListEmpty(t *testing.T) {
	list, err := newTestChatStore(t).List()
	if err != nil || list == nil || len(list) != 0 {
		t.Fatalf("list = %v, err = %v", list, err)
	}
}

func TestParseClaudeFixture(t *testing.T) {
	f, err := os.Open("testdata/claude_stream.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	var events []ChatEvent
	var sessionID string
	var turnDone bool
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		p, err := parseClaudeLine(sc.Bytes())
		if err != nil {
			t.Fatal(err)
		}
		if p.sessionID != "" {
			sessionID = p.sessionID
		}
		turnDone = turnDone || p.turnDone
		events = append(events, p.events...)
	}

	if sessionID != "sess-123" || !turnDone {
		t.Fatalf("sessionID = %q, turnDone = %v", sessionID, turnDone)
	}
	var kinds []string
	for _, e := range events {
		kinds = append(kinds, e.Kind)
	}
	want := "thinking,text,tool_use,tool_result,text,result"
	if got := strings.Join(kinds, ","); got != want {
		t.Fatalf("kinds = %s, want %s", got, want)
	}
	if events[2].ToolName != "mcp__artex__graph_overview" || string(events[2].Input) != `{"depth":2}` {
		t.Fatalf("tool_use = %+v", events[2])
	}
	if events[3].ToolID != "toolu_1" || events[3].Text != "3 nodes" {
		t.Fatalf("tool_result = %+v", events[3])
	}
	u := events[5].Usage
	if u == nil || u.InputTokens != 100 || u.OutputTokens != 50 || u.CacheReadTokens != 10 || u.CostUSD != 0.0123 || u.NumTurns != 2 {
		t.Fatalf("usage = %+v", u)
	}
}

func TestParseClaudeLineInvalid(t *testing.T) {
	if _, err := parseClaudeLine([]byte("not json")); err == nil {
		t.Fatal("expected error")
	}
}

func TestWriteMCPConfig(t *testing.T) {
	dir := t.TempDir()
	path, err := WriteMCPConfig(dir, "localhost:9400", "tok")
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	var cfg struct {
		MCPServers map[string]struct {
			URL     string            `json:"url"`
			Headers map[string]string `json:"headers"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	srv := cfg.MCPServers["artex"]
	if srv.URL != "http://127.0.0.1:9400/mcp" || srv.Headers["Authorization"] != "Bearer tok" {
		t.Fatalf("config = %+v", srv)
	}
	if _, err := WriteMCPConfig(dir, "nonsense", "tok"); err == nil {
		t.Fatal("expected error for bad address")
	}
}

type mockRunner struct {
	mu        sync.Mutex
	store     *ChatStore
	hub       *Hub
	available bool
	running   map[string]bool
	sent      []string
	stopped   []string
	startErr  error
}

func newMockRunner(available bool) *mockRunner {
	return &mockRunner{available: available, running: map[string]bool{}}
}

func (m *mockRunner) Bind(s *ChatStore, h *Hub) { m.store, m.hub = s, h }
func (m *mockRunner) Available() bool           { return m.available }
func (m *mockRunner) Running(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.running[id]
}
func (m *mockRunner) Start(_ context.Context, id, _, _, _ string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.startErr != nil {
		return m.startErr
	}
	m.running[id] = true
	return nil
}
func (m *mockRunner) Send(id, msg string) error {
	m.mu.Lock()
	m.sent = append(m.sent, msg)
	m.mu.Unlock()
	emitChatEvent(m.store, m.hub, id, ChatEvent{Kind: ChatKindText, Text: "echo " + msg})
	return nil
}
func (m *mockRunner) Stop(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopped = append(m.stopped, id)
	delete(m.running, id)
	return nil
}

func newChatTestServer(t *testing.T, runner *mockRunner) (*httptest.Server, *API) {
	t.Helper()
	opts := []Option{WithDataDir(t.TempDir())}
	if runner != nil {
		opts = append(opts, WithRunner(runner))
	}
	a := New(&mockStore{}, nil, 1, 1, opts...)
	srv := httptest.NewServer(a.Handler())
	t.Cleanup(srv.Close)
	return srv, a
}

func doJSON(t *testing.T, method, url string, body any) (*http.Response, []byte) {
	t.Helper()
	var payload string
	if body != nil {
		b, _ := json.Marshal(body)
		payload = string(b)
	}
	req, _ := http.NewRequest(method, url, strings.NewReader(payload))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	return resp, data
}

func createConv(t *testing.T, base string) Conversation {
	t.Helper()
	resp, body := doJSON(t, "POST", base+"/api/conversations", CreateConversationRequest{Title: "t"})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create status = %d: %s", resp.StatusCode, body)
	}
	var c Conversation
	if err := json.Unmarshal(body, &c); err != nil {
		t.Fatal(err)
	}
	return c
}

func TestChatRoutesCRUD(t *testing.T) {
	srv, _ := newChatTestServer(t, newMockRunner(true))
	c := createConv(t, srv.URL)

	resp, body := doJSON(t, "PATCH", srv.URL+"/api/conversations/"+c.ID, map[string]string{"title": "new", "model": "opus"})
	var updated Conversation
	json.Unmarshal(body, &updated)
	if resp.StatusCode != 200 || updated.Title != "new" || updated.Model != "opus" {
		t.Fatalf("patch: %d %s", resp.StatusCode, body)
	}

	resp, body = doJSON(t, "GET", srv.URL+"/api/conversations", nil)
	var list []Conversation
	json.Unmarshal(body, &list)
	if resp.StatusCode != 200 || len(list) != 1 {
		t.Fatalf("list: %d %s", resp.StatusCode, body)
	}

	if resp, _ = doJSON(t, "GET", srv.URL+"/api/conversations/"+c.ID, nil); resp.StatusCode != 200 {
		t.Fatalf("get: %d", resp.StatusCode)
	}
	if resp, _ = doJSON(t, "GET", srv.URL+"/api/conversations/not-a-uuid", nil); resp.StatusCode != 404 {
		t.Fatalf("bad id: %d", resp.StatusCode)
	}
	if resp, _ = doJSON(t, "DELETE", srv.URL+"/api/conversations/"+c.ID, nil); resp.StatusCode != 204 {
		t.Fatalf("delete: %d", resp.StatusCode)
	}
	if resp, _ = doJSON(t, "GET", srv.URL+"/api/conversations/"+c.ID, nil); resp.StatusCode != 404 {
		t.Fatalf("get after delete: %d", resp.StatusCode)
	}
}

func TestChatSendMessageAndMessages(t *testing.T) {
	runner := newMockRunner(true)
	srv, _ := newChatTestServer(t, runner)
	c := createConv(t, srv.URL)

	resp, body := doJSON(t, "POST", srv.URL+"/api/conversations/"+c.ID+"/messages", SendMessageRequest{Message: "hi"})
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("send: %d %s", resp.StatusCode, body)
	}
	if !runner.Running(c.ID) || len(runner.sent) != 1 || runner.sent[0] != "hi" {
		t.Fatalf("runner state: %+v", runner)
	}

	_, body = doJSON(t, "GET", srv.URL+"/api/conversations/"+c.ID+"/messages", nil)
	var events []ChatEvent
	json.Unmarshal(body, &events)
	if len(events) != 2 || events[0].Kind != ChatKindUser || events[1].Text != "echo hi" {
		t.Fatalf("events = %+v", events)
	}
	_, body = doJSON(t, "GET", srv.URL+"/api/conversations/"+c.ID+"/messages?after=1", nil)
	events = nil
	json.Unmarshal(body, &events)
	if len(events) != 1 || events[0].Seq != 2 {
		t.Fatalf("after=1 events = %+v", events)
	}

	if resp, _ = doJSON(t, "POST", srv.URL+"/api/conversations/"+c.ID+"/messages", SendMessageRequest{}); resp.StatusCode != 400 {
		t.Fatalf("empty message: %d", resp.StatusCode)
	}

	if resp, _ = doJSON(t, "POST", srv.URL+"/api/conversations/"+c.ID+"/stop", nil); resp.StatusCode != 204 {
		t.Fatalf("stop: %d", resp.StatusCode)
	}
	if runner.Running(c.ID) {
		t.Fatal("runner still running after stop")
	}
}

func TestChatSendMessageUnavailable(t *testing.T) {
	for name, runner := range map[string]*mockRunner{
		"no runner":   nil,
		"unavailable": newMockRunner(false),
	} {
		t.Run(name, func(t *testing.T) {
			srv, _ := newChatTestServer(t, runner)
			c := createConv(t, srv.URL)
			resp, _ := doJSON(t, "POST", srv.URL+"/api/conversations/"+c.ID+"/messages", SendMessageRequest{Message: "hi"})
			if resp.StatusCode != http.StatusServiceUnavailable {
				t.Fatalf("status = %d", resp.StatusCode)
			}
		})
	}
}

func TestChatSendMessageAtCapacity(t *testing.T) {
	runner := newMockRunner(true)
	runner.startErr = ErrRunnerAtCapacity
	srv, _ := newChatTestServer(t, runner)
	c := createConv(t, srv.URL)
	resp, _ := doJSON(t, "POST", srv.URL+"/api/conversations/"+c.ID+"/messages", SendMessageRequest{Message: "hi"})
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestChatDeleteStopsRunner(t *testing.T) {
	runner := newMockRunner(true)
	srv, _ := newChatTestServer(t, runner)
	c := createConv(t, srv.URL)
	doJSON(t, "DELETE", srv.URL+"/api/conversations/"+c.ID, nil)
	if len(runner.stopped) != 1 || runner.stopped[0] != c.ID {
		t.Fatalf("stopped = %v", runner.stopped)
	}
}

func TestClaudeStatus(t *testing.T) {
	a := New(&mockStore{}, nil, 1, 1, WithDataDir(t.TempDir()), WithClaudeBin("/nonexistent/claude"))
	srv := httptest.NewServer(a.Handler())
	defer srv.Close()
	_, body := doJSON(t, "GET", srv.URL+"/api/claude/status", nil)
	var st map[string]any
	json.Unmarshal(body, &st)
	if st["available"] != false {
		t.Fatalf("status = %s", body)
	}
}

func TestChatStream(t *testing.T) {
	srv, a := newChatTestServer(t, nil)
	c := createConv(t, srv.URL)
	for _, text := range []string{"one", "two"} {
		emitChatEvent(a.chat, a.hub, c.ID, ChatEvent{Kind: ChatKindText, Text: text})
	}

	req, _ := http.NewRequest("GET", srv.URL+"/api/conversations/"+c.ID+"/stream", nil)
	req.Header.Set("Last-Event-ID", "1")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
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
		emitChatEvent(a.chat, a.hub, c.ID, ChatEvent{Kind: ChatKindText, Text: "three"})
	}()
	var lines []string
	sc := bufio.NewScanner(resp.Body)
	for sc.Scan() {
		if line := sc.Text(); line != "" {
			lines = append(lines, line)
		}
		if len(lines) == 5 {
			break
		}
	}
	got := strings.Join(lines, "|")
	if !strings.HasPrefix(got, ": connected|id: 2|data: {") || !strings.Contains(got, "|id: 3|data: {") || strings.Contains(got, `"one"`) {
		t.Fatalf("stream = %s", got)
	}
}

func writeFakeClaude(t *testing.T, body string) string {
	t.Helper()
	path := t.TempDir() + "/claude"
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("condition not met in time")
}

func TestRunnerStreamsFixture(t *testing.T) {
	fixturePath, err := filepath.Abs("testdata/claude_stream.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	bin := writeFakeClaude(t, "read line\ncat "+fixturePath+"\nsleep 30\n")

	store := newTestChatStore(t)
	r := NewRunner(bin, "/tmp/mcp.json", t.TempDir(), 1)
	r.Bind(store, NewHub())
	conv, _ := store.Create(CreateConversationRequest{})

	if err := r.Start(context.Background(), conv.ID, "sonnet", "", "be brief"); err != nil {
		t.Fatal(err)
	}
	if !r.Running(conv.ID) {
		t.Fatal("expected running")
	}
	other, _ := store.Create(CreateConversationRequest{})
	if err := r.Start(context.Background(), other.ID, "", "", ""); err != ErrRunnerAtCapacity {
		t.Fatalf("expected capacity error, got %v", err)
	}
	if err := r.Send(conv.ID, "hello"); err != nil {
		t.Fatal(err)
	}

	waitFor(t, func() bool {
		evs, _ := store.Events(conv.ID, 0)
		return len(evs) == 6
	})
	waitFor(t, func() bool {
		c, _ := store.Get(conv.ID)
		return c.SessionID == "sess-123" && c.Status == ChatStatusIdle
	})

	if err := r.Stop(conv.ID); err != nil {
		t.Fatal(err)
	}
	if r.Running(conv.ID) {
		t.Fatal("still running after stop")
	}
	evs, _ := store.Events(conv.ID, 0)
	for _, e := range evs {
		if e.Kind == ChatKindError {
			t.Fatalf("unexpected error event after stop: %+v", e)
		}
	}
}

func TestRunnerEmitsErrorOnNonZeroExit(t *testing.T) {
	bin := writeFakeClaude(t, "echo boom >&2\nexit 3\n")
	store := newTestChatStore(t)
	r := NewRunner(bin, "/tmp/mcp.json", t.TempDir(), 0)
	r.Bind(store, NewHub())
	conv, _ := store.Create(CreateConversationRequest{})

	if err := r.Start(context.Background(), conv.ID, "", "", ""); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool {
		evs, _ := store.Events(conv.ID, 0)
		return len(evs) == 1 && evs[0].Kind == ChatKindError && strings.Contains(evs[0].Text, "boom")
	})
	waitFor(t, func() bool { return !r.Running(conv.ID) })
}

func TestRunnerSendAfterExitStartsNewProcess(t *testing.T) {
	bin := writeFakeClaude(t, `if [ ! -f started ]; then touch started; exit 0; fi
read line
echo '{"type":"assistant","message":{"content":[{"type":"text","text":"reply"}]}}'
sleep 30
`)
	r := NewRunner(bin, "/tmp/mcp.json", t.TempDir(), 1)
	reached := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	r.afterExit = func(string) {
		once.Do(func() {
			close(reached)
			<-release
		})
	}
	a := New(&mockStore{}, nil, 1, 1, WithDataDir(t.TempDir()), WithRunner(r))
	srv := httptest.NewServer(a.Handler())
	t.Cleanup(srv.Close)
	t.Cleanup(r.StopAll)
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(unblock)
	c := createConv(t, srv.URL)

	if err := r.Start(context.Background(), c.ID, "", "", ""); err != nil {
		t.Fatal(err)
	}
	<-reached
	if r.Running(c.ID) {
		t.Fatal("exited process still reported running")
	}
	if err := r.Send(c.ID, "hi"); !errors.Is(err, ErrNotRunning) {
		t.Fatalf("send to exited process: %v", err)
	}

	sent := make(chan *http.Response, 1)
	go func() {
		resp, err := http.Post(srv.URL+"/api/conversations/"+c.ID+"/messages", "application/json", strings.NewReader(`{"message":"hello"}`))
		if err != nil {
			sent <- nil
			return
		}
		resp.Body.Close()
		sent <- resp
	}()
	time.Sleep(100 * time.Millisecond)
	unblock()
	if resp := <-sent; resp == nil || resp.StatusCode != http.StatusAccepted {
		t.Fatalf("send: %+v", resp)
	}
	if !r.Running(c.ID) {
		t.Fatal("expected a new running process")
	}
	waitFor(t, func() bool {
		evs, _ := a.chat.Events(c.ID, 0)
		for _, e := range evs {
			if e.Kind == ChatKindText && e.Text == "reply" {
				return true
			}
		}
		return false
	})
	if conv, _ := a.chat.Get(c.ID); conv.Status != ChatStatusRunning {
		t.Fatalf("status = %s, want %s", conv.Status, ChatStatusRunning)
	}
}

func TestRunnerSetSettings(t *testing.T) {
	r := NewRunner("claude", "/tmp/mcp.json", t.TempDir(), 1)
	called := false
	r.SetSettings(func() LLMSettings {
		called = true
		return LLMSettings{MaxTurns: 99}
	})
	if r.settings == nil {
		t.Fatal("settings not set")
	}
	s := r.settings()
	if !called || s.MaxTurns != 99 {
		t.Fatalf("settings callback: called=%v maxTurns=%d", called, s.MaxTurns)
	}
}

func TestRunnerAvailable(t *testing.T) {
	r := NewRunner("/nonexistent/binary/that/wont/exist", "/tmp/mcp.json", t.TempDir(), 1)
	if r.Available() {
		t.Fatal("expected unavailable for nonexistent binary")
	}
	r2 := NewRunner("go", "/tmp/mcp.json", t.TempDir(), 1)
	if !r2.Available() {
		t.Fatal("expected available for 'go' binary")
	}
}

func TestRunnerStopAll(t *testing.T) {
	bin := writeFakeClaude(t, "read line\nsleep 60\n")
	s := newTestChatStore(t)
	r := NewRunner(bin, "/tmp/mcp.json", t.TempDir(), 5)
	r.Bind(s, NewHub())

	ids := make([]string, 3)
	for i := range ids {
		conv, err := s.Create(CreateConversationRequest{})
		if err != nil {
			t.Fatal(err)
		}
		ids[i] = conv.ID
		if err := r.Start(context.Background(), conv.ID, "", "", ""); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range ids {
		if !r.Running(id) {
			t.Fatalf("expected %s to be running", id)
		}
	}
	r.StopAll()
	for _, id := range ids {
		if r.Running(id) {
			t.Fatalf("expected %s to be stopped", id)
		}
	}
}

func TestClaudeStatusWithVersion(t *testing.T) {
	bin := writeFakeClaude(t, `echo "Claude Code v1.2.3"`)
	a := New(&mockStore{}, nil, 1, 1, WithDataDir(t.TempDir()), WithClaudeBin(bin))
	srv := httptest.NewServer(a.Handler())
	defer srv.Close()

	_, body := doJSON(t, "GET", srv.URL+"/api/claude/status", nil)
	var st map[string]any
	json.Unmarshal(body, &st)
	if st["available"] != true {
		t.Fatalf("want available=true, got %s", body)
	}
	if v, _ := st["version"].(string); v != "Claude Code v1.2.3" {
		t.Fatalf("version: %q", v)
	}
}

func TestClaudeStatusVersionError(t *testing.T) {
	bin := writeFakeClaude(t, "exit 1")
	a := New(&mockStore{}, nil, 1, 1, WithDataDir(t.TempDir()), WithClaudeBin(bin))
	srv := httptest.NewServer(a.Handler())
	defer srv.Close()

	_, body := doJSON(t, "GET", srv.URL+"/api/claude/status", nil)
	var st map[string]any
	json.Unmarshal(body, &st)
	if st["available"] != false {
		t.Fatalf("want available=false, got %s", body)
	}
	if _, ok := st["error"]; !ok {
		t.Fatal("expected error field")
	}
}

func TestChatListConversationsMultiple(t *testing.T) {
	srv, _ := newChatTestServer(t, nil)
	createConv(t, srv.URL)
	createConv(t, srv.URL)
	resp, body := doJSON(t, "GET", srv.URL+"/api/conversations", nil)
	var list []Conversation
	json.Unmarshal(body, &list)
	if resp.StatusCode != 200 || len(list) != 2 {
		t.Fatalf("list: %d len=%d", resp.StatusCode, len(list))
	}
}

func TestChatCreateBadBody(t *testing.T) {
	srv, _ := newChatTestServer(t, nil)
	resp, _ := doJSON(t, "POST", srv.URL+"/api/conversations", nil)
	req, _ := http.NewRequest("POST", srv.URL+"/api/conversations", strings.NewReader("{invalid"))
	req.Header.Set("Content-Type", "application/json")
	r, _ := http.DefaultClient.Do(req)
	r.Body.Close()
	if r.StatusCode != 400 {
		t.Fatalf("bad body: %d", resp.StatusCode)
	}
}

func TestChatUpdateBadID(t *testing.T) {
	srv, _ := newChatTestServer(t, nil)
	resp, _ := doJSON(t, "PATCH", srv.URL+"/api/conversations/not-a-uuid", map[string]string{"title": "x"})
	if resp.StatusCode != 404 {
		t.Fatalf("bad id update: %d", resp.StatusCode)
	}
}

func TestChatUpdateBadBody(t *testing.T) {
	srv, _ := newChatTestServer(t, nil)
	c := createConv(t, srv.URL)
	req, _ := http.NewRequest("PATCH", srv.URL+"/api/conversations/"+c.ID, strings.NewReader("{invalid"))
	req.Header.Set("Content-Type", "application/json")
	r, _ := http.DefaultClient.Do(req)
	r.Body.Close()
	if r.StatusCode != 400 {
		t.Fatalf("bad body: %d", r.StatusCode)
	}
}

func TestChatListMessagesBadID(t *testing.T) {
	srv, _ := newChatTestServer(t, nil)
	resp, _ := doJSON(t, "GET", srv.URL+"/api/conversations/not-a-uuid/messages", nil)
	if resp.StatusCode != 404 {
		t.Fatalf("bad id messages: %d", resp.StatusCode)
	}
}

func TestChatStopBadID(t *testing.T) {
	srv, _ := newChatTestServer(t, nil)
	resp, _ := doJSON(t, "POST", srv.URL+"/api/conversations/not-a-uuid/stop", nil)
	if resp.StatusCode != 404 {
		t.Fatalf("bad id stop: %d", resp.StatusCode)
	}
}

func TestChatDeleteBadID(t *testing.T) {
	srv, _ := newChatTestServer(t, nil)
	resp, _ := doJSON(t, "DELETE", srv.URL+"/api/conversations/not-a-uuid", nil)
	if resp.StatusCode != 404 {
		t.Fatalf("bad id delete: %d", resp.StatusCode)
	}
}

func TestChatStreamBadID(t *testing.T) {
	srv, _ := newChatTestServer(t, nil)
	resp, _ := doJSON(t, "GET", srv.URL+"/api/conversations/not-a-uuid/stream", nil)
	if resp.StatusCode != 404 {
		t.Fatalf("bad id stream: %d", resp.StatusCode)
	}
}

func TestChatSendBadID(t *testing.T) {
	runner := newMockRunner(true)
	srv, _ := newChatTestServer(t, runner)
	resp, _ := doJSON(t, "POST", srv.URL+"/api/conversations/not-a-uuid/messages", SendMessageRequest{Message: "hi"})
	if resp.StatusCode != 404 {
		t.Fatalf("bad id send: %d", resp.StatusCode)
	}
}

func TestChatStopNoRunner(t *testing.T) {
	srv, _ := newChatTestServer(t, nil)
	c := createConv(t, srv.URL)
	resp, _ := doJSON(t, "POST", srv.URL+"/api/conversations/"+c.ID+"/stop", nil)
	if resp.StatusCode != 204 {
		t.Fatalf("stop no runner: %d", resp.StatusCode)
	}
}

func TestFlattenContent_Array(t *testing.T) {
	raw := json.RawMessage(`[{"text":"hello "},{"text":"world"}]`)
	got := flattenContent(raw)
	if got != "hello world" {
		t.Fatalf("want 'hello world', got %q", got)
	}
}

func TestChatUpdateModelAgent(t *testing.T) {
	srv, _ := newChatTestServer(t, nil)
	c := createConv(t, srv.URL)
	model := "haiku"
	agent := "pentest-lead"
	resp, body := doJSON(t, "PATCH", srv.URL+"/api/conversations/"+c.ID, map[string]any{"model": model, "agent": agent})
	if resp.StatusCode != 200 {
		t.Fatalf("update: %d", resp.StatusCode)
	}
	var updated Conversation
	json.Unmarshal(body, &updated)
	if updated.Model != model || updated.Agent != agent {
		t.Fatalf("update: model=%s agent=%s", updated.Model, updated.Agent)
	}
}

func TestChatListMessagesAfter(t *testing.T) {
	srv, _ := newChatTestServer(t, nil)
	c := createConv(t, srv.URL)
	resp, body := doJSON(t, "GET", srv.URL+"/api/conversations/"+c.ID+"/messages?after=0", nil)
	if resp.StatusCode != 200 {
		t.Fatalf("messages: %d", resp.StatusCode)
	}
	var events []ChatEvent
	json.Unmarshal(body, &events)
	if events == nil {
		t.Fatal("expected non-nil events slice")
	}
}

func TestChatDeleteWithRunner(t *testing.T) {
	runner := newMockRunner(true)
	srv, _ := newChatTestServer(t, runner)
	c := createConv(t, srv.URL)
	resp, _ := doJSON(t, "DELETE", srv.URL+"/api/conversations/"+c.ID, nil)
	if resp.StatusCode != 204 {
		t.Fatalf("delete: %d", resp.StatusCode)
	}
	resp, _ = doJSON(t, "GET", srv.URL+"/api/conversations/"+c.ID, nil)
	if resp.StatusCode != 404 {
		t.Fatalf("get deleted: %d", resp.StatusCode)
	}
}

func TestChatSendEmptyMessage(t *testing.T) {
	runner := newMockRunner(true)
	srv, _ := newChatTestServer(t, runner)
	c := createConv(t, srv.URL)
	resp, _ := doJSON(t, "POST", srv.URL+"/api/conversations/"+c.ID+"/messages", SendMessageRequest{Message: "   "})
	if resp.StatusCode != 400 {
		t.Fatalf("empty message: %d", resp.StatusCode)
	}
}

func TestChatSendNoRunner(t *testing.T) {
	srv, _ := newChatTestServer(t, nil)
	c := createConv(t, srv.URL)
	resp, _ := doJSON(t, "POST", srv.URL+"/api/conversations/"+c.ID+"/messages", SendMessageRequest{Message: "hi"})
	if resp.StatusCode != 503 {
		t.Fatalf("no runner: %d", resp.StatusCode)
	}
}

func TestFlattenContent_RawFallback(t *testing.T) {
	raw := json.RawMessage(`12345`)
	got := flattenContent(raw)
	if got != "12345" {
		t.Fatalf("want '12345', got %q", got)
	}
}

func TestRunnerGivesEachConversationAChatIdentity(t *testing.T) {
	dataDir := t.TempDir()
	base, err := WriteMCPConfig(dataDir, "localhost:9400", "tok")
	if err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	bin := writeFakeClaude(t, `while [ $# -gt 0 ]; do
  if [ "$1" = --mcp-config ]; then
    echo "$2" > `+out+`/path
    cp "$2" `+out+`/config.json
    ls -ln "$2" | cut -c1-10 > `+out+`/mode
  fi
  shift
done
`)
	store := newTestChatStore(t)
	newRunner := func() *Runner {
		signer, err := identity.NewSigner(dataDir)
		if err != nil {
			t.Fatal(err)
		}
		r := NewRunner(bin, base, dataDir, 2)
		r.SetIdentitySigner(signer)
		r.Bind(store, NewHub())
		return r
	}
	start := func(r *Runner, id string) string {
		t.Helper()
		if err := r.Start(context.Background(), id, "", "", ""); err != nil {
			t.Fatal(err)
		}
		waitFor(t, func() bool { return !r.Running(id) })
		used, err := os.ReadFile(filepath.Join(out, "path"))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(strings.TrimSpace(string(used))); !os.IsNotExist(err) {
			t.Errorf("conversation config left behind: %v", err)
		}
		if mode, _ := os.ReadFile(filepath.Join(out, "mode")); strings.TrimSpace(string(mode)) != "-rw-------" {
			t.Errorf("conversation config mode = %q, want -rw-------", mode)
		}
		data, err := os.ReadFile(filepath.Join(out, "config.json"))
		if err != nil {
			t.Fatal(err)
		}
		var cfg mcpConfigFile
		if err := json.Unmarshal(data, &cfg); err != nil {
			t.Fatal(err)
		}
		h := cfg.MCPServers[builtinMCPName].Headers
		if h["Authorization"] != "Bearer tok" {
			t.Errorf("Authorization = %q", h["Authorization"])
		}
		if _, ok := h["X-Artex-Conversation"]; ok {
			t.Error("config still sends X-Artex-Conversation")
		}
		return h[mcpserver.IdentityHeader]
	}
	// A fresh signer on the same data directory stands in for a restarted server.
	verify := func(token string) (identity.Claims, error) {
		s, err := identity.NewSigner(dataDir)
		if err != nil {
			t.Fatal(err)
		}
		return s.Verify(token)
	}

	r := newRunner()
	a, _ := store.Create(CreateConversationRequest{})
	b, _ := store.Create(CreateConversationRequest{})
	tokens := map[string]string{a.ID: start(r, a.ID), b.ID: start(r, b.ID)}
	for id, token := range tokens {
		c, err := verify(token)
		if err != nil {
			t.Fatalf("token for %s: %v", id, err)
		}
		if c.Name != "chat/"+id {
			t.Errorf("token name = %q, want chat/%s", c.Name, id)
		}
		if ttl := c.ExpiresAt.Sub(c.IssuedAt); ttl != identity.DefaultTTL {
			t.Errorf("token TTL = %s, want %s", ttl, identity.DefaultTTL)
		}
	}

	resumed := start(newRunner(), a.ID)
	if c, err := verify(resumed); err != nil || c.Name != "chat/"+a.ID {
		t.Fatalf("token after restart = %+v, %v, want chat/%s", c, err, a.ID)
	}
	if _, err := verify(tokens[a.ID]); err != nil {
		t.Fatalf("token from before the restart: %v", err)
	}

	if _, _, err := config.RotateIdentityKey(dataDir); err != nil {
		t.Fatal(err)
	}
	for _, token := range []string{tokens[a.ID], tokens[b.ID], resumed} {
		if _, err := verify(token); !errors.Is(err, identity.ErrUnknownKey) {
			t.Errorf("chat token after rotation: %v, want ErrUnknownKey", err)
		}
	}
	if _, err := verify(start(newRunner(), b.ID)); err != nil {
		t.Fatalf("token minted after rotation: %v", err)
	}
}
