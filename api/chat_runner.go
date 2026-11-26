package api

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/SREsAreHumanToo/ARTEX-CC/identity"
	"github.com/SREsAreHumanToo/ARTEX-CC/mcpserver"
)

const (
	defaultMaxConversations = 5
	stopGracePeriod         = 5 * time.Second
	maxStderrBytes          = 8 << 10
)

var ErrRunnerAtCapacity = errors.New("too many running conversations")
var ErrNotRunning = errors.New("conversation is not running")

type ChatRunner interface {
	Bind(store *ChatStore, hub *Hub)
	Available() bool
	Running(conversationID string) bool
	Start(ctx context.Context, conversationID, model, agent, systemPrompt string) error
	Send(conversationID, message string) error
	Stop(conversationID string) error
}

type Runner struct {
	claudeBin string
	mcpConfig string
	dataDir   string
	maxConvs  int
	settings  func() LLMSettings
	signer    *identity.Signer
	store     *ChatStore
	hub       *Hub
	mu        sync.Mutex
	processes map[string]*runningConv
	afterExit func(id string)
}

type runningConv struct {
	cmd       *exec.Cmd
	mcpConfig string
	stdin     io.WriteCloser
	cancel    context.CancelFunc
	sessionID string
	done      chan struct{}
	exited    atomic.Bool
	stopping  atomic.Bool
	stderr    *limitedBuffer
}

type limitedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if room := maxStderrBytes - b.buf.Len(); room > 0 {
		b.buf.Write(p[:min(room, len(p))])
	}
	return len(p), nil
}

func (b *limitedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func NewRunner(claudeBin, mcpConfig, dataDir string, maxConversations int) *Runner {
	if claudeBin == "" {
		claudeBin = "claude"
	}
	if maxConversations <= 0 {
		maxConversations = defaultMaxConversations
	}
	return &Runner{
		claudeBin: claudeBin,
		mcpConfig: mcpConfig,
		dataDir:   dataDir,
		maxConvs:  maxConversations,
		processes: make(map[string]*runningConv),
	}
}

type mcpConfigFile struct {
	MCPServers map[string]mcpHTTPServer `json:"mcpServers"`
}

type mcpHTTPServer struct {
	Type    string            `json:"type"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
}

func writeMCPConfigFile(path string, cfg mcpConfigFile) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func WriteMCPConfig(dataDir, listenAddr, token string) (string, error) {
	_, port, err := net.SplitHostPort(listenAddr)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dataDir, 0o711); err != nil {
		return "", err
	}
	path, err := filepath.Abs(filepath.Join(dataDir, "mcp-config.json"))
	if err != nil {
		return "", err
	}
	cfg := mcpConfigFile{MCPServers: map[string]mcpHTTPServer{
		builtinMCPName: {
			Type:    "http",
			URL:     "http://127.0.0.1:" + port + "/mcp",
			Headers: map[string]string{"Authorization": "Bearer " + token},
		},
	}}
	if err := writeMCPConfigFile(path, cfg); err != nil {
		return "", err
	}
	return path, nil
}

// conversationMCPConfig writes a copy of the runner's MCP config whose artex
// server also sends a freshly minted chat/<id> identity, so the conversation
// holds its own claims and keeps them when it resumes after a server restart.
func (r *Runner) conversationMCPConfig(id string) (string, error) {
	token, err := r.signer.Issue("chat/"+id, identity.DefaultTTL)
	if err != nil {
		return "", err
	}
	var cfg mcpConfigFile
	data, err := os.ReadFile(r.mcpConfig)
	if err != nil {
		return "", err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return "", fmt.Errorf("parse %s: %w", r.mcpConfig, err)
	}
	srv, ok := cfg.MCPServers[builtinMCPName]
	if !ok {
		return "", fmt.Errorf("%s has no %s server", r.mcpConfig, builtinMCPName)
	}
	headers := map[string]string{mcpserver.IdentityHeader: token}
	for k, v := range srv.Headers {
		headers[k] = v
	}
	srv.Headers = headers
	cfg.MCPServers[builtinMCPName] = srv
	dir := filepath.Join(filepath.Dir(r.mcpConfig), "chat-mcp")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	path := filepath.Join(dir, id+".json")
	if err := writeMCPConfigFile(path, cfg); err != nil {
		return "", err
	}
	return path, nil
}

func (r *Runner) Bind(store *ChatStore, hub *Hub) {
	r.store = store
	r.hub = hub
}

func (r *Runner) SetSettings(fn func() LLMSettings) {
	r.settings = fn
}

// SetIdentitySigner gives each conversation's claude process its own
// chat/<conversation id> identity on the artex MCP server.
func (r *Runner) SetIdentitySigner(s *identity.Signer) {
	r.signer = s
}

func (r *Runner) Available() bool {
	_, err := exec.LookPath(r.claudeBin)
	return err == nil
}

func (r *Runner) Running(id string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	rc, ok := r.processes[id]
	return ok && !rc.exited.Load()
}

func (r *Runner) Start(ctx context.Context, id, model, agent, systemPrompt string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for old := r.processes[id]; old != nil; old = r.processes[id] {
		if !old.exited.Load() {
			return nil
		}
		// Wait out the old pump's final writes so they cannot overwrite the new process's status.
		r.mu.Unlock()
		<-old.done
		r.mu.Lock()
	}
	var settings LLMSettings
	if r.settings != nil {
		settings = r.settings()
	}
	maxConvs := r.maxConvs
	if settings.MaxConversations > 0 {
		maxConvs = settings.MaxConversations
	}
	if len(r.processes) >= maxConvs {
		return ErrRunnerAtCapacity
	}
	conv, err := r.store.Get(id)
	if err != nil {
		return err
	}

	maxTurns := 50
	if settings.MaxTurns > 0 {
		maxTurns = settings.MaxTurns
	}
	if model == "" && agent == "" {
		model = settings.MasterModel
	}
	if model == "" {
		model = settings.DefaultModel
	}
	mcpConfig := r.mcpConfig
	if r.signer != nil {
		if mcpConfig, err = r.conversationMCPConfig(id); err != nil {
			return fmt.Errorf("write conversation mcp config: %w", err)
		}
	}
	started := false
	defer func() {
		if !started && mcpConfig != r.mcpConfig {
			os.Remove(mcpConfig)
		}
	}()

	args := []string{
		"-p",
		"--output-format", "stream-json",
		"--verbose",
		"--input-format", "stream-json",
		"--max-turns", strconv.Itoa(maxTurns),
		"--mcp-config", mcpConfig,
		"--strict-mcp-config",
		"--allowedTools", "mcp__artex__*",
	}
	if model != "" {
		args = append(args, "--model", model)
	}
	if agent != "" {
		args = append(args, "--agent", agent)
	}
	if conv.SessionID != "" {
		args = append(args, "--resume", conv.SessionID)
	}
	var env []string
	if settings.SubagentModel != "" {
		env = append(env, "CLAUDE_CODE_SUBAGENT_MODEL="+settings.SubagentModel)
	}
	switch settings.Thinking {
	case "off":
		env = append(env, "MAX_THINKING_TOKENS=0")
	case "low", "medium", "high", "xhigh", "max":
		args = append(args, "--effort", settings.Thinking)
	}
	if settings.GlobalSystemPrompt != "" {
		args = append(args, "--system-prompt", settings.GlobalSystemPrompt)
	}
	var appendPrompts []string
	if systemPrompt != "" {
		appendPrompts = append(appendPrompts, systemPrompt)
	}
	switch settings.SubagentStrategy {
	case "parallel":
		appendPrompts = append(appendPrompts, "Dispatch independent subagent tasks in parallel when possible.")
		if settings.MaxParallelSubagents > 0 {
			appendPrompts = append(appendPrompts, fmt.Sprintf("Limit concurrent subagents to %d.", settings.MaxParallelSubagents))
		}
	case "sequential":
		appendPrompts = append(appendPrompts, "Run subagent tasks sequentially, one at a time.")
	case "per_agent":
		appendPrompts = append(appendPrompts, "Use each agent preset's configured model for subagents.")
	}
	for _, p := range appendPrompts {
		args = append(args, "--append-system-prompt", p)
	}

	runCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	cmd := exec.CommandContext(runCtx, r.claudeBin, args...)
	cmd.Dir = r.dataDir
	cmd.Env = append(os.Environ(), env...)
	setProcessGroup(cmd)
	cmd.Cancel = func() error { return killProcessGroup(cmd) }
	cmd.WaitDelay = stopGracePeriod
	stderr := &limitedBuffer{}
	cmd.Stderr = stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return err
	}
	if err := cmd.Start(); err != nil {
		cancel()
		return fmt.Errorf("start claude: %w", err)
	}
	started = true

	rc := &runningConv{
		cmd:       cmd,
		mcpConfig: mcpConfig,
		stdin:     stdin,
		cancel:    cancel,
		sessionID: conv.SessionID,
		done:      make(chan struct{}),
		stderr:    stderr,
	}
	r.processes[id] = rc
	go r.pump(id, rc, stdout)
	r.publishStatus(id, "started")
	return nil
}

func (r *Runner) Send(id, message string) error {
	r.mu.Lock()
	rc := r.processes[id]
	r.mu.Unlock()
	if rc == nil || rc.exited.Load() {
		return ErrNotRunning
	}
	line, err := json.Marshal(map[string]any{
		"type": "user",
		"message": map[string]any{
			"role":    "user",
			"content": []map[string]string{{"type": "text", "text": message}},
		},
	})
	if err != nil {
		return err
	}
	if _, err := rc.stdin.Write(append(line, '\n')); err != nil {
		return fmt.Errorf("write to claude: %w", err)
	}
	r.setStatus(id, ChatStatusRunning)
	return nil
}

func (r *Runner) Stop(id string) error {
	r.mu.Lock()
	rc := r.processes[id]
	r.mu.Unlock()
	if rc == nil {
		return nil
	}
	rc.stopping.Store(true)
	_ = interruptProcessGroup(rc.cmd)
	select {
	case <-rc.done:
	case <-time.After(stopGracePeriod):
		rc.cancel()
		<-rc.done
	}
	return nil
}

func (r *Runner) StopAll() {
	r.mu.Lock()
	ids := make([]string, 0, len(r.processes))
	for id := range r.processes {
		ids = append(ids, id)
	}
	r.mu.Unlock()
	var wg sync.WaitGroup
	for _, id := range ids {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r.Stop(id) //nolint:errcheck
		}()
	}
	wg.Wait()
}

func (r *Runner) pump(id string, rc *runningConv, stdout io.Reader) {
	sc := bufio.NewScanner(stdout)
	sc.Buffer(nil, 16<<20)
	for sc.Scan() {
		r.handleLine(id, rc, sc.Bytes())
	}
	if err := sc.Err(); err != nil {
		slog.Error("reading claude output", "conversation", id, "err", err)
		rc.cancel()
	}
	err := rc.cmd.Wait()
	rc.exited.Store(true)
	if r.afterExit != nil {
		r.afterExit(id)
	}
	rc.cancel()
	if rc.mcpConfig != r.mcpConfig {
		os.Remove(rc.mcpConfig)
	}

	if err != nil && !rc.stopping.Load() {
		msg := rc.stderr.String()
		if msg == "" {
			msg = err.Error()
		}
		r.emit(id, ChatEvent{Kind: ChatKindError, Text: msg, IsError: true})
	}
	r.setStatus(id, ChatStatusIdle)

	r.mu.Lock()
	if r.processes[id] == rc {
		delete(r.processes, id)
	}
	r.mu.Unlock()
	r.publishStatus(id, "stopped")
	close(rc.done)
}

func (r *Runner) handleLine(id string, rc *runningConv, line []byte) {
	parsed, err := parseClaudeLine(line)
	if err != nil {
		slog.Debug("skipping unparseable claude line", "conversation", id, "err", err)
		return
	}
	if parsed.sessionID != "" && parsed.sessionID != rc.sessionID {
		rc.sessionID = parsed.sessionID
		sid := parsed.sessionID
		if _, err := r.store.Update(id, func(c *Conversation) { c.SessionID = sid }); err != nil {
			slog.Error("storing session id", "conversation", id, "err", err)
		}
	}
	for _, ev := range parsed.events {
		r.emit(id, ev)
	}
	if parsed.turnDone {
		r.setStatus(id, ChatStatusIdle)
	}
}

func (r *Runner) emit(id string, ev ChatEvent) {
	emitChatEvent(r.store, r.hub, id, ev)
}

func (r *Runner) setStatus(id, status string) {
	var changed bool
	_, err := r.store.Update(id, func(c *Conversation) {
		changed = c.Status != status
		c.Status = status
	})
	if err != nil {
		if !errors.Is(err, ErrConversationNotFound) {
			slog.Error("updating conversation status", "conversation", id, "err", err)
		}
		return
	}
	if changed {
		r.publishStatus(id, status)
	}
}

func (r *Runner) publishStatus(id, status string) {
	publishGlobal(r.hub, "conversation_status", map[string]string{"id": id, "status": status})
}

func publishGlobal(hub *Hub, eventType string, data any) {
	payload, err := json.Marshal(map[string]any{"type": eventType, "data": data})
	if err != nil {
		return
	}
	hub.Publish(globalTopic, payload)
}

func chatTopic(id string) string {
	return "chat:" + id
}

func emitChatEvent(store *ChatStore, hub *Hub, id string, ev ChatEvent) {
	saved, err := store.AppendEvent(id, ev)
	if err != nil {
		slog.Error("appending chat event", "conversation", id, "err", err)
		return
	}
	payload, err := json.Marshal(saved)
	if err != nil {
		return
	}
	hub.Publish(chatTopic(id), payload)
}

type parsedLine struct {
	sessionID string
	events    []ChatEvent
	turnDone  bool
}

type claudeLine struct {
	Type      string `json:"type"`
	Subtype   string `json:"subtype"`
	SessionID string `json:"session_id"`
	Message   struct {
		Content json.RawMessage `json:"content"`
	} `json:"message"`
	IsError      bool    `json:"is_error"`
	Result       string  `json:"result"`
	DurationMS   int64   `json:"duration_ms"`
	NumTurns     int     `json:"num_turns"`
	TotalCostUSD float64 `json:"total_cost_usd"`
	Usage        struct {
		InputTokens         int `json:"input_tokens"`
		OutputTokens        int `json:"output_tokens"`
		CacheReadTokens     int `json:"cache_read_input_tokens"`
		CacheCreationTokens int `json:"cache_creation_input_tokens"`
	} `json:"usage"`
}

type claudeBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text"`
	Thinking  string          `json:"thinking"`
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Input     json.RawMessage `json:"input"`
	ToolUseID string          `json:"tool_use_id"`
	Content   json.RawMessage `json:"content"`
	IsError   bool            `json:"is_error"`
}

func parseClaudeLine(line []byte) (parsedLine, error) {
	var l claudeLine
	if err := json.Unmarshal(line, &l); err != nil {
		return parsedLine{}, err
	}
	var out parsedLine
	switch l.Type {
	case "system":
		if l.Subtype == "init" {
			out.sessionID = l.SessionID
		}
	case "assistant", "user":
		var blocks []claudeBlock
		if json.Unmarshal(l.Message.Content, &blocks) != nil {
			return out, nil
		}
		for _, b := range blocks {
			switch b.Type {
			case "text":
				if l.Type == "assistant" {
					out.events = append(out.events, ChatEvent{Kind: ChatKindText, Text: b.Text})
				}
			case "thinking":
				out.events = append(out.events, ChatEvent{Kind: ChatKindThinking, Text: b.Thinking})
			case "tool_use":
				out.events = append(out.events, ChatEvent{Kind: ChatKindToolUse, ToolID: b.ID, ToolName: b.Name, Input: b.Input})
			case "tool_result":
				out.events = append(out.events, ChatEvent{Kind: ChatKindToolResult, ToolID: b.ToolUseID, Text: flattenContent(b.Content), IsError: b.IsError})
			}
		}
	case "result":
		out.sessionID = l.SessionID
		out.turnDone = true
		out.events = append(out.events, ChatEvent{
			Kind:    ChatKindResult,
			Text:    l.Result,
			IsError: l.IsError,
			Usage: &ChatUsage{
				InputTokens:         l.Usage.InputTokens,
				OutputTokens:        l.Usage.OutputTokens,
				CacheReadTokens:     l.Usage.CacheReadTokens,
				CacheCreationTokens: l.Usage.CacheCreationTokens,
				CostUSD:             l.TotalCostUSD,
				DurationMS:          l.DurationMS,
				NumTurns:            l.NumTurns,
			},
		})
	}
	return out, nil
}

func flattenContent(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var parts []struct {
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &parts) != nil {
		return string(raw)
	}
	var b bytes.Buffer
	for _, p := range parts {
		b.WriteString(p.Text)
	}
	return b.String()
}
