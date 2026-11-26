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
	"runtime"
	"strings"
	"testing"
	"time"
)

const (
	fakeEnv        = "EVAL_RUNNER_TEST_FAKE"
	fakeCatalogEnv = "EVAL_RUNNER_TEST_CATALOG"
)

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
	if len(args) > 0 && args[0] == "tools" {
		fmt.Print(os.Getenv(fakeCatalogEnv))
		os.Exit(0)
	}
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
		TMPDIR string `json:"tmpdir"`
		TMP    string `json:"tmp"`
		TEMP   string `json:"temp"`
	}
	for i, a := range args {
		switch {
		case a == "--mcp-config" && i+1 < len(args):
			out.Config = args[i+1]
		case a == "-p" && i+1 < len(args):
			out.Prompt = args[i+1]
		}
	}
	out.TMPDIR, out.TMP, out.TEMP = os.Getenv("TMPDIR"), os.Getenv("TMP"), os.Getenv("TEMP")
	_ = os.WriteFile(filepath.Join(out.TMPDIR, "wf.sh"), []byte("echo hi\n"), 0644)
	if strings.Contains(out.Prompt, "instance 2 of") {
		_ = os.MkdirAll(filepath.Join(out.TMPDIR, "loot"), 0755)
		_ = os.WriteFile(filepath.Join(out.TMPDIR, "loot", "id_ed25519"), []byte("-----BEGIN OPENSSH PRIVATE KEY-----\nAAAA\n-----END OPENSSH PRIVATE KEY-----\n"), 0600)
	}
	_ = json.NewEncoder(os.Stdout).Encode(out)
	os.Exit(0)
}

type fakeRun struct {
	Config string
	Prompt string
	TMPDIR string
	TMP    string
	TEMP   string
	Server map[string]any
	Run    InstanceRun
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
		fr.Run = r
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

func TestRunInstancesWritesOwnerOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permissions not enforced on Windows")
	}
	for _, transport := range []string{"stdio", "http"} {
		t.Run(transport, func(t *testing.T) {
			rc, runs, _ := runFake(t, transport, false)
			paths := map[string]os.FileMode{
				filepath.Join(rc.dataDir, "mcp.json"): 0o600,
				rc.transcriptsDir:                     0o700,
			}
			for _, r := range runs {
				paths[r.Run.Transcript] = 0o600
			}
			if transport == "http" {
				paths[filepath.Join(rc.transcriptsDir, "server.log")] = 0o600
			}
			for path, want := range paths {
				info, err := os.Stat(path)
				if err != nil {
					t.Fatal(err)
				}
				if got := info.Mode().Perm(); got != want {
					t.Errorf("%s mode %o, want %o", path, got, want)
				}
			}
		})
	}
}

func TestRunInstancesTempDir(t *testing.T) {
	rc, runs, _ := runFake(t, "stdio", false)
	for i, r := range runs {
		want, _ := filepath.Abs(filepath.Join(rc.transcriptsDir, fmt.Sprintf("tmp-instance-%d", i+1)))
		if r.TMPDIR != want || r.TMP != want || r.TEMP != want || r.Run.TempDir != want {
			t.Errorf("instance %d temp dirs %q %q %q (run %q), want %q", i+1, r.TMPDIR, r.TMP, r.TEMP, r.Run.TempDir, want)
		}
		if !strings.Contains(r.Prompt, "in $TMPDIR, not /tmp") {
			t.Errorf("instance %d prompt lacks the scratch line: %s", i+1, r.Prompt)
		}
	}
	one, two := runs[0].Run.Scratch, runs[1].Run.Scratch
	if one.Files != 1 || len(one.Listed) != 1 || one.Listed[0] != (ScratchFile{Path: "wf.sh", Size: 8}) || len(one.PrivateKeys) != 0 {
		t.Errorf("instance 1 scratch: %+v", one)
	}
	key := filepath.Join("loot", "id_ed25519")
	if two.Files != 2 || len(two.PrivateKeys) != 1 || two.PrivateKeys[0] != key {
		t.Errorf("instance 2 scratch: %+v", two)
	}
}
