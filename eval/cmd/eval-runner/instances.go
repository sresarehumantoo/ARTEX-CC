package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/SREsAreHumanToo/ARTEX-CC/identity"
	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
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
	extraEnv       []string
	identity       bool
	runID          string
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
	TempDir         string  `json:"temp_dir,omitempty"`
	Scratch         Scratch `json:"scratch"`
}

// Scratch is what an instance left in its temp dir after the run.
type Scratch struct {
	Files       int           `json:"files"`
	Bytes       int64         `json:"bytes"`
	Listed      []ScratchFile `json:"listed,omitempty"`
	PrivateKeys []string      `json:"private_keys,omitempty"`
}

type ScratchFile struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
}

const (
	scratchListCap  = 50
	privateKeyProbe = 1 << 20
)

var privateKeyRe = regexp.MustCompile(`-----BEGIN [A-Z0-9 ]*PRIVATE KEY-----`)

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

func buildPrompt(sc Scenario, instance, instances int, askOwner bool) string {
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
	prompt += " Keep scratch files (scripts, payloads, downloads, keys) in $TMPDIR, not /tmp."
	if instances > 1 {
		prompt += fmt.Sprintf(" You are instance %d of %d; other agents are working this engagement at the same time. "+
			"Coordinate through intent claims and list_changes as the engagement guide describes.", instance, instances)
	}
	if askOwner {
		prompt += " Choose a random hex owner label once, at the start, and pass it as owner on every " +
			"start_intent, complete_intent, cancel_intent and list_changes call."
	}
	return prompt
}

// serveEnv is the environment for the servers the runner starts, directly
// or through claude's stdio config. Their anonymous modes (stdio, and HTTP
// with --identity=false) cannot satisfy ARTEX_REQUIRE_IDENTITY, so an
// inherited value is cleared there; with identities it passes through.
func serveEnv(rc runConfig) []string {
	env := os.Environ()
	if rc.lease != "" {
		env = append(env, "ARTEX_INTENT_LEASE="+rc.lease)
	}
	if rc.mcpConfig == "" && !rc.identity {
		env = append(env, "ARTEX_REQUIRE_IDENTITY=")
	}
	return env
}

// instanceEnv gives each claude instance (and any serve --stdio it starts) its
// own temp dir, so instances cannot clobber each other's scratch files and
// nothing lands in the operator's /tmp. --env comes last and can override it.
func instanceEnv(rc runConfig, tmpDir string) []string {
	env := append(serveEnv(rc), "TMPDIR="+tmpDir, "TMP="+tmpDir, "TEMP="+tmpDir)
	return append(env, rc.extraEnv...)
}

// scanScratch counts the files under dir, lists the first scratchListCap in
// path order and flags regular files whose first MiB holds a PEM private key.
// Symlinks are listed but not followed.
func scanScratch(dir string) Scratch {
	var s Scratch
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(dir, path)
		s.Files++
		s.Bytes += info.Size()
		if len(s.Listed) < scratchListCap {
			s.Listed = append(s.Listed, ScratchFile{Path: rel, Size: info.Size()})
		}
		if info.Mode().IsRegular() && hasPrivateKey(path) {
			s.PrivateKeys = append(s.PrivateKeys, rel)
		}
		return nil
	})
	return s
}

func hasPrivateKey(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	head, _ := io.ReadAll(io.LimitReader(f, privateKeyProbe))
	return privateKeyRe.Match(head)
}

func writeMCPConfig(path string, server map[string]any, perm os.FileMode) error {
	data, err := json.Marshal(map[string]any{"mcpServers": map[string]any{"artex": server}})
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, perm)
}

func identityName(runID string, instance int) string {
	return fmt.Sprintf("eval/%s/i%d", runID, instance)
}

// identityTTL covers the whole run plus one lease, so a claim taken just
// before the timeout can still be renewed or completed.
func identityTTL(timeout time.Duration, lease string) time.Duration {
	l := tools.DefaultIntentLease
	if d, err := time.ParseDuration(lease); err == nil && d > 0 {
		l = d
	}
	return timeout + l
}

// writeIdentityConfigs mints one identity per instance with the data
// directory's key, which the runner's serve --listen verifies against, and
// writes each into its own 0600 MCP config. Tokens never leave those files.
func writeIdentityConfigs(rc runConfig, url string, ttl time.Duration) ([]string, error) {
	signer, err := identity.NewSigner(rc.dataDir)
	if err != nil {
		return nil, fmt.Errorf("identity key: %w", err)
	}
	paths := make([]string, rc.instances)
	for i := range rc.instances {
		token, err := signer.Issue(identityName(rc.runID, i+1), ttl)
		if err != nil {
			return nil, fmt.Errorf("issue identity for instance %d: %w", i+1, err)
		}
		paths[i] = filepath.Join(rc.dataDir, fmt.Sprintf("mcp-i%d.json", i+1))
		server := map[string]any{"type": "http", "url": url, "headers": map[string]string{"X-Artex-Identity": token}}
		if err := writeMCPConfig(paths[i], server, 0600); err != nil {
			return nil, fmt.Errorf("write mcp config: %w", err)
		}
	}
	return paths, nil
}

// runInstances starts every claude instance, waits for all of them, and kills
// any still running when ctx is done or the timeout passes.
func runInstances(ctx context.Context, rc runConfig, sc Scenario, timeout time.Duration) ([]InstanceRun, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if err := os.MkdirAll(rc.transcriptsDir, 0700); err != nil {
		return nil, fmt.Errorf("create transcripts dir: %w", err)
	}

	configs := make([]string, rc.instances)
	if rc.mcpConfig != "" {
		for i := range configs {
			configs[i] = rc.mcpConfig
		}
	} else {
		var server map[string]any
		switch rc.transport {
		case "http":
			url, stop, err := startHTTPServer(rc)
			if err != nil {
				return nil, err
			}
			defer stop()
			if rc.identity {
				paths, err := writeIdentityConfigs(rc, url, identityTTL(timeout, rc.lease))
				if err != nil {
					return nil, err
				}
				configs = paths
				break
			}
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
		if server != nil {
			path := filepath.Join(rc.dataDir, "mcp.json")
			if err := writeMCPConfig(path, server, 0600); err != nil {
				return nil, fmt.Errorf("write mcp config: %w", err)
			}
			for i := range configs {
				configs[i] = path
			}
		}
	}

	runs := make([]InstanceRun, rc.instances)
	var wg sync.WaitGroup
	for i := range rc.instances {
		wg.Add(1)
		go func() {
			defer wg.Done()
			runs[i] = runInstance(ctx, rc, sc, configs[i], i+1, timeout)
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

func runInstance(ctx context.Context, rc runConfig, sc Scenario, mcpConfigPath string, instance int, timeout time.Duration) (run InstanceRun) {
	transcript := filepath.Join(rc.transcriptsDir, fmt.Sprintf("instance-%d.jsonl", instance))
	run = InstanceRun{Instance: instance, Transcript: transcript}

	out, err := os.OpenFile(transcript, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		run.ExitError = err.Error()
		return run
	}
	defer out.Close()

	tmpDir, err := filepath.Abs(filepath.Join(rc.transcriptsDir, fmt.Sprintf("tmp-instance-%d", instance)))
	if err == nil {
		err = os.MkdirAll(tmpDir, 0700)
	}
	if err != nil {
		run.ExitError = err.Error()
		return run
	}
	run.TempDir = tmpDir
	defer func() {
		run.Scratch = scanScratch(tmpDir)
		if len(run.Scratch.PrivateKeys) > 0 {
			slog.Warn("instance left private keys in its temp dir; delete them", "instance", instance, "dir", tmpDir, "files", run.Scratch.PrivateKeys)
		}
	}()

	args := claudeArgs(mcpConfigPath, sc.Model, buildPrompt(sc, instance, rc.instances, rc.transport == "http" && !rc.identity))
	cmd := exec.CommandContext(ctx, rc.claudeBin, args...)
	cmd.Stdout = out
	cmd.Stderr = os.Stderr
	cmd.Dir = rc.dataDir
	cmd.Env = instanceEnv(rc, tmpDir)
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
	logFile, err := os.OpenFile(logPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return "", nil, err
	}

	cmd := exec.Command(rc.artexBin, "serve", "--listen", addr, "--pg", rc.dsn, "--data", rc.dataDir)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	// The listener is loopback and the generated MCP config sends no
	// Authorization header, so an inherited ARTEX_MCP_TOKEN must not apply.
	cmd.Env = append(serveEnv(rc), "ARTEX_MCP_TOKEN=")
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
