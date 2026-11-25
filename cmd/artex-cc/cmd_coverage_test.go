package main

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"flag"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestResolvePG_FlagOnly(t *testing.T) {
	t.Setenv("ARTEX_PG", "")
	if got := resolvePG("postgres://flag"); got != "postgres://flag" {
		t.Fatalf("got %q", got)
	}
}

func TestResolvePG_EnvFallback(t *testing.T) {
	t.Setenv("ARTEX_PG", "postgres://env")
	if got := resolvePG(""); got != "postgres://env" {
		t.Fatalf("got %q", got)
	}
}

func TestResolvePG_FlagOverridesEnv(t *testing.T) {
	t.Setenv("ARTEX_PG", "postgres://env")
	if got := resolvePG("postgres://flag"); got != "postgres://flag" {
		t.Fatalf("got %q", got)
	}
}

func TestResolvePG_Empty(t *testing.T) {
	t.Setenv("ARTEX_PG", "")
	if got := resolvePG(""); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestPgDSN(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	pg := pgDSN(fs)
	fs.Parse([]string{"--pg", "postgres://localhost/test"})
	if *pg != "postgres://localhost/test" {
		t.Fatalf("got %q", *pg)
	}
}

func TestPgDSN_Default(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	pg := pgDSN(fs)
	fs.Parse(nil)
	if *pg != "" {
		t.Fatalf("got %q", *pg)
	}
}

func TestCmdVersionPlain(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmdVersion(nil)

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	out := buf.String()

	if !strings.HasPrefix(out, "artex-cc ") {
		t.Fatalf("unexpected output: %q", out)
	}
	if !strings.Contains(out, version) {
		t.Fatalf("missing version in %q", out)
	}
}

func TestCmdVersionJSON(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmdVersion([]string{"--json"})

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)

	var info map[string]string
	if err := json.Unmarshal(buf.Bytes(), &info); err != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", err, buf.String())
	}
	if info["version"] != version {
		t.Fatalf("version = %q, want %q", info["version"], version)
	}
	if info["commit"] != commit {
		t.Fatalf("commit = %q", info["commit"])
	}
	if info["build_date"] != buildDate {
		t.Fatalf("build_date = %q", info["build_date"])
	}
	if info["go_version"] != runtime.Version() {
		t.Fatalf("go_version = %q", info["go_version"])
	}
	if info["os"] != runtime.GOOS {
		t.Fatalf("os = %q", info["os"])
	}
	if info["arch"] != runtime.GOARCH {
		t.Fatalf("arch = %q", info["arch"])
	}
}

func TestCmdCompletionBash(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmdCompletion([]string{"bash"})

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	out := buf.String()

	if !strings.Contains(out, "complete -F") {
		t.Fatal("bash completion missing complete -F")
	}
	if !strings.Contains(out, "complete -F _artex_cc artex-cc") {
		t.Fatal("bash completion missing artex-cc registration")
	}
}

func TestCmdCompletionZsh(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmdCompletion([]string{"zsh"})

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	out := buf.String()

	if !strings.Contains(out, "#compdef artex-cc\n") {
		t.Fatal("zsh completion missing #compdef artex-cc")
	}
}

func TestCmdCompletionFish(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmdCompletion([]string{"fish"})

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	out := buf.String()

	if !strings.Contains(out, "complete -c artex-cc -n '__fish_use_subcommand'") {
		t.Fatal("fish completion missing artex-cc registration")
	}
}

func TestCompletionScriptsContainSubcommands(t *testing.T) {
	for _, shell := range []struct {
		name   string
		script string
	}{
		{"bash", bashCompletion},
		{"zsh", zshCompletion},
		{"fish", fishCompletion},
	} {
		for _, cmd := range []string{"serve", "init", "health", "version", "completion"} {
			if !strings.Contains(shell.script, cmd) {
				t.Fatalf("%s completion missing subcommand %q", shell.name, cmd)
			}
		}
	}
}

func TestCompletionScriptsContainFlags(t *testing.T) {
	for _, want := range []string{"--pg", "--data"} {
		if !strings.Contains(bashCompletion, want) {
			t.Fatalf("bash completion missing %q", want)
		}
		if !strings.Contains(zshCompletion, strings.TrimPrefix(want, "--")) {
			t.Fatalf("zsh completion missing %q", want)
		}
		if !strings.Contains(fishCompletion, strings.TrimPrefix(want, "--")) {
			t.Fatalf("fish completion missing %q", want)
		}
	}
}

func TestServeFlagParsing(t *testing.T) {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	listen := fs.String("listen", "localhost:9400", "")
	stdio := fs.Bool("stdio", false, "")
	webUI := fs.Bool("web", false, "")
	debug := fs.Bool("debug", false, "")
	showVersion := fs.Bool("version", false, "")
	pg := pgDSN(fs)
	dataDir := fs.String("data", ".", "")
	proxyAddr := fs.String("proxy", "", "")
	shutdownSecs := fs.Int("timeout", 30, "")

	err := fs.Parse([]string{
		"--listen", "0.0.0.0:9500",
		"--stdio",
		"--web",
		"--debug",
		"--pg", "postgres://test",
		"--data", "/tmp/testdata",
		"--proxy", ":8788",
		"--timeout", "60",
	})
	if err != nil {
		t.Fatal(err)
	}
	if *listen != "0.0.0.0:9500" {
		t.Fatalf("listen = %q", *listen)
	}
	if !*stdio {
		t.Fatal("stdio not set")
	}
	if !*webUI {
		t.Fatal("web not set")
	}
	if !*debug {
		t.Fatal("debug not set")
	}
	if *showVersion {
		t.Fatal("version should not be set")
	}
	if *pg != "postgres://test" {
		t.Fatalf("pg = %q", *pg)
	}
	if *dataDir != "/tmp/testdata" {
		t.Fatalf("data = %q", *dataDir)
	}
	if *proxyAddr != ":8788" {
		t.Fatalf("proxy = %q", *proxyAddr)
	}
	if *shutdownSecs != 60 {
		t.Fatalf("timeout = %d", *shutdownSecs)
	}
}

func TestInitFlagParsing(t *testing.T) {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	showVersion := fs.Bool("version", false, "")
	pg := pgDSN(fs)
	goal := fs.String("goal", "", "")
	desc := fs.String("description", "", "")
	dataDir := fs.String("data", ".", "")

	err := fs.Parse([]string{
		"--pg", "postgres://init-test",
		"--goal", "Find SQL injection",
		"--description", "Test target",
		"--data", "/tmp/engagement",
	})
	if err != nil {
		t.Fatal(err)
	}
	if *showVersion {
		t.Fatal("version should not be set")
	}
	if *pg != "postgres://init-test" {
		t.Fatalf("pg = %q", *pg)
	}
	if *goal != "Find SQL injection" {
		t.Fatalf("goal = %q", *goal)
	}
	if *desc != "Test target" {
		t.Fatalf("desc = %q", *desc)
	}
	if *dataDir != "/tmp/engagement" {
		t.Fatalf("data = %q", *dataDir)
	}
}

func TestHealthFlagParsing(t *testing.T) {
	fs := flag.NewFlagSet("health", flag.ContinueOnError)
	pg := pgDSN(fs)
	format := fs.String("format", "text", "")
	err := fs.Parse([]string{"--pg", "postgres://health", "--format", "json"})
	if err != nil {
		t.Fatal(err)
	}
	if *pg != "postgres://health" {
		t.Fatalf("pg = %q", *pg)
	}
	if *format != "json" {
		t.Fatalf("format = %q", *format)
	}
}

func TestVersionFlagParsing(t *testing.T) {
	fs := flag.NewFlagSet("version", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "")
	err := fs.Parse([]string{"--json"})
	if err != nil {
		t.Fatal(err)
	}
	if !*jsonOut {
		t.Fatal("json not set")
	}
}

func TestWriteHealthText_SchemaFail(t *testing.T) {
	r := healthResult{
		Status: "fail",
		Checks: map[string]hcEntry{
			"pg":     {Status: "ok", LatencyMS: 5},
			"schema": {Status: "fail", Detail: "not initialized (run artex-cc init)"},
		},
	}
	var buf bytes.Buffer
	writeHealthText(&buf, r)
	out := buf.String()
	if !strings.Contains(out, "not initialized") {
		t.Fatalf("got %q", out)
	}
}

func TestRunHealthChecks_InvalidDSN(t *testing.T) {
	t.Setenv("ARTEX_PG", "")
	r := runHealthChecks("postgres://invalid:99/db", t.TempDir(), 100*1e6)
	if r.Status != "fail" {
		t.Fatalf("status = %q, want fail", r.Status)
	}
}

func TestInitLogging(t *testing.T) {
	initLogging(false)
	if !slog.Default().Enabled(context.TODO(), slog.LevelInfo) {
		t.Fatal("info should be enabled")
	}
	initLogging(true)
	if !slog.Default().Enabled(context.TODO(), slog.LevelDebug) {
		t.Fatal("debug should be enabled in debug mode")
	}
}

func TestGenerateCA(t *testing.T) {
	dir := t.TempDir()
	certPath, err := generateCA(dir)
	if err != nil {
		t.Fatal(err)
	}
	if certPath == "" {
		t.Fatal("empty cert path")
	}
	if !filepath.IsAbs(certPath) {
		t.Fatalf("not absolute: %s", certPath)
	}
	raw, err := os.ReadFile(certPath)
	if err != nil {
		t.Fatal(err)
	}
	block, _ := pem.Decode(raw)
	if block == nil || block.Type != "CERTIFICATE" {
		t.Fatal("not a PEM certificate")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	if !cert.IsCA {
		t.Fatal("not a CA certificate")
	}
	if cert.Subject.CommonName != "artex CA" {
		t.Fatalf("CN = %q", cert.Subject.CommonName)
	}
}

func TestGenerateCA_Idempotent(t *testing.T) {
	dir := t.TempDir()
	p1, err := generateCA(dir)
	if err != nil {
		t.Fatal(err)
	}
	p2, err := generateCA(dir)
	if err != nil {
		t.Fatal(err)
	}
	if p1 != p2 {
		t.Fatalf("paths differ: %q vs %q", p1, p2)
	}
}

func TestBuildVarsExist(t *testing.T) {
	if version == "" {
		t.Fatal("version is empty")
	}
	if commit == "" {
		t.Fatal("commit is empty")
	}
	if buildDate == "" {
		t.Fatal("buildDate is empty")
	}
}
