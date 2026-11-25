package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type runConfig struct {
	claudeBin      string
	artexBin       string
	dsn            string
	dataDir        string
	mcpConfig      string
	instances      int
	transport      string
	lease          string
	kill           *killSpec
	transcriptsDir string
}

type killSpec struct {
	Instance int
	After    time.Duration
}

// InstanceRun records how one claude process ended.
type InstanceRun struct {
	Instance        int     `json:"instance"`
	Transcript      string  `json:"transcript"`
	StartedAt       string  `json:"started_at"`
	DurationSeconds float64 `json:"duration_seconds"`
	ExitError       string  `json:"exit_error,omitempty"`
	KilledAt        string  `json:"killed_at,omitempty"`
}

func parseKillAfter(s string, instances int) (*killSpec, error) {
	if s == "" {
		return nil, nil
	}
	inst, dur, ok := strings.Cut(s, ":")
	if !ok {
		return nil, fmt.Errorf("expected INSTANCE:DURATION, got %q", s)
	}
	i, err := strconv.Atoi(inst)
	if err != nil || i < 1 || i > instances {
		return nil, fmt.Errorf("instance must be between 1 and %d, got %q", instances, inst)
	}
	d, err := time.ParseDuration(dur)
	if err != nil {
		return nil, err
	}
	return &killSpec{Instance: i, After: d}, nil
}

func buildPrompt(sc Scenario, instance, instances int, transport string) string {
	prompt := fmt.Sprintf(
		"You are a penetration tester. Your target is %s. Your goal: %s. "+
			"Use the ARTEX MCP tools to track your work. "+
			"Start by calling graph_overview to orient, then set goals, insert assets, and begin testing. "+
			"Record every finding with report_finding. Record coverage for every surface you test. "+
			"When done, call generate_report.",
		sc.Target, sc.Goal,
	)
	if sc.Credentials.Username != "" {
		prompt += fmt.Sprintf(" Known credentials: %s / %s.", sc.Credentials.Username, sc.Credentials.Password)
	}
	if instances > 1 {
		prompt += fmt.Sprintf(" You are instance %d of %d; other agents are working this engagement at the same time. "+
			"Coordinate through intent claims and list_changes as the engagement guide describes.", instance, instances)
	}
	if transport == "http" {
		prompt += " Choose a random hex owner label once, at the start, and pass it as owner on every " +
			"start_intent, complete_intent, cancel_intent and list_changes call."
	}
	return prompt
}

func serveEnv(lease string) []string {
	env := os.Environ()
	if lease != "" {
		env = append(env, "ARTEX_INTENT_LEASE="+lease)
	}
	return env
}

func writeMCPConfig(path string, server map[string]any) error {
	data, err := json.Marshal(map[string]any{"mcpServers": map[string]any{"artex": server}})
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// runInstances starts every claude instance, waits for all of them, and kills
// any still running when ctx is done or the timeout passes.
func runInstances(ctx context.Context, rc runConfig, sc Scenario, timeout time.Duration) ([]InstanceRun, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if err := os.MkdirAll(rc.transcriptsDir, 0755); err != nil {
		return nil, fmt.Errorf("create transcripts dir: %w", err)
	}

	mcpConfigPath := rc.mcpConfig
	if mcpConfigPath == "" {
		mcpConfigPath = filepath.Join(rc.dataDir, "mcp.json")
		var server map[string]any
		switch rc.transport {
		case "http":
			url, stop, err := startHTTPServer(rc)
			if err != nil {
				return nil, err
			}
			defer stop()
			server = map[string]any{"type": "http", "url": url}
		default:
			// Each claude process launches its own serve --stdio from this
			// config, so instances get separate MCP sessions on one database.
			server = map[string]any{
				"command": rc.artexBin,
				"args":    []string{"serve", "--stdio", "--pg", rc.dsn, "--data", rc.dataDir},
			}
			if rc.lease != "" {
				server["env"] = map[string]string{"ARTEX_INTENT_LEASE": rc.lease}
			}
		}
		if err := writeMCPConfig(mcpConfigPath, server); err != nil {
			return nil, fmt.Errorf("write mcp config: %w", err)
		}
	}

	runs := make([]InstanceRun, rc.instances)
	var wg sync.WaitGroup
	for i := range rc.instances {
		wg.Add(1)
		go func() {
			defer wg.Done()
			runs[i] = runInstance(ctx, rc, sc, mcpConfigPath, i+1, timeout)
		}()
	}
	wg.Wait()
	return runs, ctx.Err()
}

// claudeArgs builds the claude CLI arguments for one instance. Runs skip
// permission prompts, so --strict-mcp-config keeps the user's own MCP servers
// and connectors (mail, calendar, docs) out of the eval: only ARTEX is loaded.
func claudeArgs(mcpConfigPath, model, prompt string) []string {
	args := []string{
		"--print",
		"--dangerously-skip-permissions",
		"--mcp-config", mcpConfigPath,
		"--strict-mcp-config",
		"--output-format", "stream-json",
		"--verbose",
	}
	if model != "" {
		args = append(args, "--model", model)
	}
	return append(args, "-p", prompt)
}

func runInstance(ctx context.Context, rc runConfig, sc Scenario, mcpConfigPath string, instance int, timeout time.Duration) InstanceRun {
	transcript := filepath.Join(rc.transcriptsDir, fmt.Sprintf("instance-%d.jsonl", instance))
	run := InstanceRun{Instance: instance, Transcript: transcript}

	out, err := os.Create(transcript)
	if err != nil {
		run.ExitError = err.Error()
		return run
	}
	defer out.Close()

	args := claudeArgs(mcpConfigPath, sc.Model, buildPrompt(sc, instance, rc.instances, rc.transport))
	cmd := exec.CommandContext(ctx, rc.claudeBin, args...)
	cmd.Stdout = out
	cmd.Stderr = os.Stderr
	cmd.Dir = rc.dataDir
	cmd.Env = serveEnv(rc.lease)
	setProcessGroup(cmd)
	cmd.Cancel = func() error { return killProcessGroup(cmd) }
	cmd.WaitDelay = 10 * time.Second

	slog.Info("starting Claude", "instance", instance, "bin", rc.claudeBin, "model", sc.Model, "timeout", timeout, "transcript", transcript)
	started := time.Now()
	run.StartedAt = started.Format(time.RFC3339)
	if err := cmd.Start(); err != nil {
		run.ExitError = err.Error()
		return run
	}

	var killMu sync.Mutex
	var killedAt time.Time
	if rc.kill != nil && rc.kill.Instance == instance {
		timer := time.AfterFunc(rc.kill.After, func() {
			killMu.Lock()
			killedAt = time.Now()
			killMu.Unlock()
			slog.Info("killing instance", "instance", instance, "after", rc.kill.After)
			if err := killProcessGroup(cmd); err != nil {
				slog.Warn("kill instance failed", "instance", instance, "err", err)
			}
		})
		defer timer.Stop()
	}

	err = cmd.Wait()
	run.DurationSeconds = time.Since(started).Seconds()
	if err != nil {
		run.ExitError = err.Error()
	}
	killMu.Lock()
	if !killedAt.IsZero() {
		run.KilledAt = killedAt.Format(time.RFC3339Nano)
	}
	killMu.Unlock()
	slog.Info("instance finished", "instance", instance, "duration", time.Since(started).Round(time.Second), "err", run.ExitError)
	return run
}

// startHTTPServer runs one artex-cc serve --listen for every instance to share
// and returns its MCP endpoint.
func startHTTPServer(rc runConfig) (string, func(), error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", nil, err
	}
	addr := l.Addr().String()
	l.Close()

	logPath := filepath.Join(rc.transcriptsDir, "server.log")
	logFile, err := os.Create(logPath)
	if err != nil {
		return "", nil, err
	}

	cmd := exec.Command(rc.artexBin, "serve", "--listen", addr, "--pg", rc.dsn, "--data", rc.dataDir)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Env = serveEnv(rc.lease)
	setProcessGroup(cmd)
	if err := cmd.Start(); err != nil {
		logFile.Close()
		return "", nil, fmt.Errorf("start artex http server: %w", err)
	}
	exited := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(exited)
	}()
	stop := func() {
		_ = terminateProcessGroup(cmd)
		select {
		case <-exited:
		case <-time.After(15 * time.Second):
			_ = killProcessGroup(cmd)
			<-exited
		}
		logFile.Close()
	}

	deadline := time.Now().Add(30 * time.Second)
	for {
		conn, err := net.DialTimeout("tcp", addr, time.Second)
		if err == nil {
			conn.Close()
			break
		}
		select {
		case <-exited:
			logFile.Close()
			return "", nil, fmt.Errorf("artex http server exited during startup; see %s", logPath)
		case <-time.After(200 * time.Millisecond):
		}
		if time.Now().After(deadline) {
			stop()
			return "", nil, fmt.Errorf("artex http server did not listen on %s within 30s; see %s", addr, logPath)
		}
	}
	slog.Info("artex http server started", "addr", addr, "log", logPath)
	return "http://" + addr + "/mcp", stop, nil
}
