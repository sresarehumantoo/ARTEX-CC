package main

import (
	"encoding/json"
	"fmt"
	"os"
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
		t.Fatalf("single stdio prompt must stay unchanged: %s", single)
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
