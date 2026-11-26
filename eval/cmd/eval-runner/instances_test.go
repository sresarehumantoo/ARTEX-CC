package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SREsAreHumanToo/ARTEX-CC/identity"
	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
)

func TestServeEnvRequireIdentity(t *testing.T) {
	t.Setenv("ARTEX_REQUIRE_IDENTITY", "1")
	last := func(env []string) string {
		v := "unset"
		for _, kv := range env {
			if s, ok := strings.CutPrefix(kv, "ARTEX_REQUIRE_IDENTITY="); ok {
				v = s
			}
		}
		return v
	}
	for name, c := range map[string]struct {
		rc   runConfig
		want string
	}{
		"stdio":            {runConfig{transport: "stdio"}, ""},
		"http anonymous":   {runConfig{transport: "http"}, ""},
		"http identities":  {runConfig{transport: "http", identity: true}, "1"},
		"custom mcpConfig": {runConfig{transport: "stdio", mcpConfig: "x.json"}, "1"},
	} {
		if got := last(serveEnv(c.rc)); got != c.want {
			t.Errorf("%s: ARTEX_REQUIRE_IDENTITY = %q, want %q", name, got, c.want)
		}
	}
}

func TestParseKillAfter(t *testing.T) {
	k, err := parseKillAfter("2:3m", 3)
	if err != nil || k.Instance != 2 || k.After != 3*time.Minute {
		t.Fatalf("got %+v, %v", k, err)
	}
	if k, err := parseKillAfter("", 3); k != nil || err != nil {
		t.Fatalf("empty: got %+v, %v", k, err)
	}
	for _, bad := range []string{"2", "4:1m", "0:1m", "x:1m", "2:soon"} {
		if _, err := parseKillAfter(bad, 3); err == nil {
			t.Errorf("%q: expected an error", bad)
		}
	}
}

func TestBuildPrompt(t *testing.T) {
	sc := Scenario{Target: "http://localhost:8083", Goal: "find bugs"}
	single := buildPrompt(sc, 1, 1, false)
	if strings.Contains(single, "instance") || strings.Contains(single, "owner") {
		t.Fatalf("single stdio prompt must not mention instances or owners: %s", single)
	}
	if !strings.HasSuffix(single, " Keep scratch files (scripts, payloads, downloads, keys) in $TMPDIR, not /tmp.") {
		t.Fatalf("single prompt lacks the scratch line: %s", single)
	}
	multi := buildPrompt(sc, 2, 3, false)
	if !strings.HasPrefix(multi, single) || !strings.Contains(multi, "You are instance 2 of 3;") {
		t.Fatalf("multi prompt: %s", multi)
	}
	if strings.Contains(multi, "owner") {
		t.Fatalf("stdio and identity prompts should not ask for an owner: %s", multi)
	}
	if anon := buildPrompt(sc, 1, 3, true); !strings.Contains(anon, "pass it as owner on every start_intent, complete_intent, cancel_intent and list_changes call") {
		t.Fatalf("anonymous http prompt: %s", anon)
	}
}

func TestIdentityTTL(t *testing.T) {
	if got := identityTTL(15*time.Minute, ""); got != 15*time.Minute+tools.DefaultIntentLease {
		t.Errorf("default lease: %s", got)
	}
	if got := identityTTL(15*time.Minute, "2m"); got != 17*time.Minute {
		t.Errorf("2m lease: %s", got)
	}
}

func readMCPServer(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		MCPServers map[string]map[string]any `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	return cfg.MCPServers["artex"]
}

func TestWriteIdentityConfigs(t *testing.T) {
	rc := runConfig{dataDir: t.TempDir(), instances: 3, runID: "20261125-120000"}
	ttl := identityTTL(15*time.Minute, "2m")
	before := time.Now()
	paths, err := writeIdentityConfigs(rc, "http://127.0.0.1:1234/mcp", ttl)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := identity.NewSigner(rc.dataDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 3 {
		t.Fatalf("paths: %v", paths)
	}
	tokens := map[string]bool{}
	for i, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0600 {
			t.Errorf("%s mode %v, want 0600", p, info.Mode().Perm())
		}
		server := readMCPServer(t, p)
		if server["type"] != "http" || server["url"] != "http://127.0.0.1:1234/mcp" {
			t.Errorf("server %d: %v", i+1, server)
		}
		headers, _ := server["headers"].(map[string]any)
		token, _ := headers["X-Artex-Identity"].(string)
		if tokens[token] {
			t.Fatalf("instance %d reuses a token", i+1)
		}
		tokens[token] = true
		claims, err := signer.Verify(token)
		if err != nil {
			t.Fatalf("instance %d token does not verify: %v", i+1, err)
		}
		if want := fmt.Sprintf("eval/20261125-120000/i%d", i+1); claims.Name != want {
			t.Errorf("name %q, want %q", claims.Name, want)
		}
		if !identity.Reserved(claims.Name) {
			t.Errorf("%q should be under a reserved prefix", claims.Name)
		}
		lo := before.Add(ttl).Truncate(time.Second)
		hi := time.Now().Add(ttl)
		if claims.ExpiresAt.Before(lo) || claims.ExpiresAt.After(hi) {
			t.Errorf("expiry %s outside [%s, %s]", claims.ExpiresAt, lo, hi)
		}
		if claims.ExpiresAt.Sub(claims.IssuedAt) > 17*time.Minute {
			t.Errorf("ttl %s exceeds timeout plus lease", claims.ExpiresAt.Sub(claims.IssuedAt))
		}
	}
}

func TestClaudeArgsLoadOnlyTheEvalMCPConfig(t *testing.T) {
	args := claudeArgs("/tmp/mcp.json", "claude-opus-5-5", "go")
	has := func(flag string) bool {
		for _, a := range args {
			if a == flag {
				return true
			}
		}
		return false
	}
	for _, flag := range []string{"--strict-mcp-config", "--mcp-config", "--dangerously-skip-permissions", "--print"} {
		if !has(flag) {
			t.Errorf("claude args %v missing %s", args, flag)
		}
	}
	if got := args[len(args)-2:]; got[0] != "-p" || got[1] != "go" {
		t.Errorf("prompt not last: %v", got)
	}
	if !has("--model") || has("") {
		t.Errorf("model handling wrong: %v", args)
	}
	if noModel := claudeArgs("/tmp/mcp.json", "", "go"); len(noModel) != len(args)-2 {
		t.Errorf("empty model should omit --model: %v", noModel)
	}
}

func TestCoverageEntries(t *testing.T) {
	for _, tc := range []struct {
		cov  map[string]any
		want int
	}{
		{map[string]any{"entries": 20, "gaps": 2}, 20},
		{map[string]any{"entries": float64(7)}, 7},
		{map[string]any{"total": 9}, 0},
		{nil, 0},
	} {
		if got := coverageEntries(tc.cov); got != tc.want {
			t.Errorf("coverageEntries(%v) = %d, want %d", tc.cov, got, tc.want)
		}
	}
}

func TestEnvFlag(t *testing.T) {
	var e envFlag
	for _, ok := range []string{"TELEPORT_HOME=/tmp/empty", "WINFORGE_TSH_IDENTITY=/run/eval/forge.pem", "EMPTY="} {
		if err := e.Set(ok); err != nil {
			t.Fatalf("Set(%q): %v", ok, err)
		}
	}
	for _, bad := range []string{"NOEQUALS", "=value", " =x"} {
		if err := e.Set(bad); err == nil {
			t.Errorf("Set(%q) accepted", bad)
		}
	}
	if len(e) != 3 || e[2] != "EMPTY=" {
		t.Fatalf("collected %v", e)
	}
}

func TestScopeKind(t *testing.T) {
	for entry, want := range map[string]string{
		"winforge":        "subdomain",
		"localhost:8083":  "subdomain",
		"app.example.com": "subdomain",
		"127.0.0.1":       "ip",
		"127.0.0.1:8080":  "ip",
		"[::1]:9400":      "ip",
		"203.0.113.0/24":  "cidr",
		" 198.51.100.7 ":  "ip",
	} {
		if got := scopeKind(entry); got != want {
			t.Errorf("scopeKind(%q) = %q, want %q", entry, got, want)
		}
	}
}

func TestInstanceEnv(t *testing.T) {
	t.Setenv("TMPDIR", "/tmp")
	lookup := func(env []string, key string) string {
		v := "unset"
		for _, kv := range env {
			if s, ok := strings.CutPrefix(kv, key+"="); ok {
				v = s
			}
		}
		return v
	}
	rc := runConfig{transport: "stdio", lease: "2m", extraEnv: []string{"TELEPORT_HOME=/empty"}}
	env := instanceEnv(rc, "/runs/t/tmp-instance-1")
	for _, k := range []string{"TMPDIR", "TMP", "TEMP"} {
		if got := lookup(env, k); got != "/runs/t/tmp-instance-1" {
			t.Errorf("%s = %q", k, got)
		}
	}
	if lookup(env, "TELEPORT_HOME") != "/empty" || lookup(env, "ARTEX_INTENT_LEASE") != "2m" || lookup(env, "ARTEX_REQUIRE_IDENTITY") != "" {
		t.Errorf("--env or serve env lost: %v", env)
	}
	rc.extraEnv = append(rc.extraEnv, "TMPDIR=/scratch")
	if got := lookup(instanceEnv(rc, "/runs/t/tmp-instance-1"), "TMPDIR"); got != "/scratch" {
		t.Errorf("--env TMPDIR should win, got %q", got)
	}
}

func TestScanScratch(t *testing.T) {
	dir := t.TempDir()
	for i := range scratchListCap + 5 {
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("f%03d.ps1", i)), []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	rsa := "junk\n-----BEGIN RSA PRIVATE KEY-----\nMIIE\n-----END RSA PRIVATE KEY-----\n"
	if err := os.WriteFile(filepath.Join(dir, "a.key"), []byte(rsa), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pub.pem"), []byte("-----BEGIN PUBLIC KEY-----\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(dir, "a.key"), filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	s := scanScratch(dir)
	if s.Files != scratchListCap+8 || len(s.Listed) != scratchListCap || s.Listed[0].Path != "a.key" {
		t.Fatalf("files %d, listed %d, first %+v", s.Files, len(s.Listed), s.Listed[0])
	}
	if len(s.PrivateKeys) != 1 || s.PrivateKeys[0] != "a.key" {
		t.Fatalf("private keys: %v", s.PrivateKeys)
	}
	if empty := scanScratch(filepath.Join(dir, "missing")); empty.Files != 0 {
		t.Fatalf("missing dir: %+v", empty)
	}
}
