package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const fakeEnv = "EVAL_RUNNER_TEST_FAKE"

// TestMain lets the test binary stand in for artex-cc (serve --listen) and
// for claude, so runInstances can run end to end without either.
func TestMain(m *testing.M) {
	if os.Getenv(fakeEnv) == "1" {
		fakeMain(os.Args[1:])
		return
	}
	os.Exit(m.Run())
}

func fakeMain(args []string) {
	if len(args) > 0 && args[0] == "serve" {
		for i, a := range args {
			if a == "--listen" && i+1 < len(args) {
				l, err := net.Listen("tcp", args[i+1])
				if err != nil {
					os.Exit(1)
				}
				_ = http.Serve(l, http.NotFoundHandler())
			}
		}
		os.Exit(1)
	}
	var out struct {
		Config string `json:"config"`
		Prompt string `json:"prompt"`
	}
	for i, a := range args {
		switch {
		case a == "--mcp-config" && i+1 < len(args):
			out.Config = args[i+1]
		case a == "-p" && i+1 < len(args):
			out.Prompt = args[i+1]
		}
	}
	_ = json.NewEncoder(os.Stdout).Encode(out)
	os.Exit(0)
}

type fakeRun struct {
	Config string
	Prompt string
	Server map[string]any
}

func runFake(t *testing.T, transport string, withIdentity bool) (runConfig, []fakeRun, string) {
	t.Helper()
	t.Setenv(fakeEnv, "1")
	var logs bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	dir := t.TempDir()
	rc := runConfig{
		claudeBin:      os.Args[0],
		artexBin:       os.Args[0],
		dataDir:        filepath.Join(dir, "data"),
		instances:      2,
		transport:      transport,
		lease:          "2m",
		identity:       withIdentity,
		runID:          "20261125-120000",
		transcriptsDir: filepath.Join(dir, "transcripts"),
	}
	if err := os.MkdirAll(rc.dataDir, 0755); err != nil {
		t.Fatal(err)
	}
	runs, err := runInstances(context.Background(), rc, Scenario{Target: "http://x", Goal: "g"}, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	var got []fakeRun
	for _, r := range runs {
		if r.ExitError != "" {
			t.Fatalf("instance %d: %s", r.Instance, r.ExitError)
		}
		data, err := os.ReadFile(r.Transcript)
		if err != nil {
			t.Fatal(err)
		}
		var fr fakeRun
		if err := json.Unmarshal(data, &fr); err != nil {
			t.Fatalf("transcript %d: %v: %s", r.Instance, err, data)
		}
		fr.Server = readMCPServer(t, fr.Config)
		got = append(got, fr)
	}
	scorecard, _ := json.Marshal(Scorecard{Runs: runs, Identity: rc.identity})
	return rc, got, logs.String() + string(scorecard)
}

func TestRunInstancesHTTPIdentity(t *testing.T) {
	rc, runs, output := runFake(t, "http", true)
	if runs[0].Config == runs[1].Config {
		t.Fatalf("instances share config %s", runs[0].Config)
	}
	for i, r := range runs {
		if strings.Contains(r.Prompt, "owner") {
			t.Errorf("instance %d prompt asks for an owner: %s", i+1, r.Prompt)
		}
		if filepath.Dir(r.Config) != rc.dataDir {
			t.Errorf("config %s outside the data dir", r.Config)
		}
		headers, _ := r.Server["headers"].(map[string]any)
		token, _ := headers["X-Artex-Identity"].(string)
		if !strings.HasPrefix(token, "ai1.") {
			t.Fatalf("instance %d has no identity header: %v", i+1, r.Server)
		}
		if strings.Contains(output, token) {
			t.Errorf("instance %d token leaked into logs or scorecard", i+1)
		}
		filepath.WalkDir(rc.transcriptsDir, func(path string, d os.DirEntry, err error) error {
			if err == nil && !d.IsDir() {
				if data, _ := os.ReadFile(path); bytes.Contains(data, []byte(token)) {
					t.Errorf("token found in %s", path)
				}
			}
			return nil
		})
	}
}

func TestRunInstancesHTTPAnonymous(t *testing.T) {
	rc, runs, _ := runFake(t, "http", false)
	want := filepath.Join(rc.dataDir, "mcp.json")
	for i, r := range runs {
		if r.Config != want {
			t.Errorf("instance %d config %s, want shared %s", i+1, r.Config, want)
		}
		if _, ok := r.Server["headers"]; ok {
			t.Errorf("anonymous config has headers: %v", r.Server)
		}
		if r.Server["type"] != "http" {
			t.Errorf("server: %v", r.Server)
		}
		if !strings.Contains(r.Prompt, fmt.Sprintf("You are instance %d of 2", i+1)) || !strings.Contains(r.Prompt, "pass it as owner") {
			t.Errorf("instance %d prompt: %s", i+1, r.Prompt)
		}
	}
	if _, err := os.Stat(filepath.Join(rc.dataDir, "identity.key")); !os.IsNotExist(err) {
		t.Errorf("anonymous run created an identity key: %v", err)
	}
}

func TestRunInstancesStdioUnchanged(t *testing.T) {
	rc, runs, _ := runFake(t, "stdio", false)
	for i, r := range runs {
		if r.Config != filepath.Join(rc.dataDir, "mcp.json") || r.Server["command"] != rc.artexBin {
			t.Errorf("instance %d: %+v", i+1, r)
		}
		if strings.Contains(r.Prompt, "owner") {
			t.Errorf("stdio prompt asks for an owner: %s", r.Prompt)
		}
	}
}
