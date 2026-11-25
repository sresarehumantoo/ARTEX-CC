package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SREsAreHumanToo/ARTEX-CC/config"
	"github.com/SREsAreHumanToo/ARTEX-CC/identity"
)

func runID(t *testing.T, stdin string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	var out, errOut bytes.Buffer
	code = runIdentity(args, strings.NewReader(stdin), &out, &errOut)
	return out.String(), errOut.String(), code
}

func issueToken(t *testing.T, dir, name string, extra ...string) string {
	t.Helper()
	out, errOut, code := runID(t, "", append([]string{"issue", "--data", dir, "--name", name}, extra...)...)
	if code != 0 {
		t.Fatalf("issue exit %d: %s", code, errOut)
	}
	return strings.TrimSuffix(out, "\n")
}

func verifyIn(t *testing.T, dir, token string) (identity.Claims, error) {
	t.Helper()
	s, err := identity.NewSigner(dir)
	if err != nil {
		t.Fatal(err)
	}
	return s.Verify(token)
}

func TestIdentityIssueToken(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	out, errOut, code := runID(t, "", "issue", "--data", dir, "--name", "alice")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	if strings.Count(out, "\n") != 1 || !strings.HasSuffix(out, "\n") {
		t.Fatalf("stdout is not exactly one line: %q", out)
	}
	token := strings.TrimSuffix(out, "\n")
	claims, err := verifyIn(t, dir, token)
	if err != nil {
		t.Fatal(err)
	}
	if claims.Name != "alice" {
		t.Errorf("name %q", claims.Name)
	}
	if got := claims.ExpiresAt.Sub(claims.IssuedAt); got != identity.DefaultTTL {
		t.Errorf("ttl %s, want %s", got, identity.DefaultTTL)
	}
	if strings.Contains(errOut, token) {
		t.Error("stderr echoes the token")
	}
	if !strings.Contains(errOut, claims.KeyID) {
		t.Errorf("stderr lacks the key ID: %q", errOut)
	}
}

func TestIdentityIssueHeader(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	out := issueToken(t, dir, "bob", "--format", "header")
	if strings.Contains(out, "\n") {
		t.Fatalf("stdout is not one line: %q", out)
	}
	token, ok := strings.CutPrefix(out, "X-Artex-Identity: ")
	if !ok {
		t.Fatalf("missing header name: %q", out)
	}
	if _, err := verifyIn(t, dir, token); err != nil {
		t.Fatal(err)
	}
}

func TestIdentityIssueClaudeJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	out := issueToken(t, dir, "carol", "--format", "claude-json", "--url", "https://artex.example:9400/mcp", "--bearer-env", "ARTEX_MCP_TOKEN")
	var cfg struct {
		MCPServers map[string]struct {
			Type    string            `json:"type"`
			URL     string            `json:"url"`
			Headers map[string]string `json:"headers"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal([]byte(out), &cfg); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	srv, ok := cfg.MCPServers["artex"]
	if !ok || len(cfg.MCPServers) != 1 {
		t.Fatalf("want one artex server: %s", out)
	}
	if srv.Type != "http" || srv.URL != "https://artex.example:9400/mcp" {
		t.Errorf("server %+v", srv)
	}
	if srv.Headers["Authorization"] != "Bearer ${ARTEX_MCP_TOKEN}" {
		t.Errorf("authorization %q", srv.Headers["Authorization"])
	}
	if _, err := verifyIn(t, dir, srv.Headers["X-Artex-Identity"]); err != nil {
		t.Fatal(err)
	}

	out = issueToken(t, dir, "carol", "--format", "claude-json")
	if err := json.Unmarshal([]byte(out), &cfg); err != nil {
		t.Fatal(err)
	}
	srv = cfg.MCPServers["artex"]
	if srv.URL != "http://localhost:9400/mcp" {
		t.Errorf("default url %q", srv.URL)
	}
	if _, ok := srv.Headers["Authorization"]; ok {
		t.Error("authorization header without --bearer-env")
	}
}

func TestIdentityIssueRejects(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cases := []struct {
		name string
		args []string
	}{
		{"no name", []string{"issue", "--data", dir}},
		{"invalid name", []string{"issue", "--data", dir, "--name", "Alice"}},
		{"name with space", []string{"issue", "--data", dir, "--name", "a b"}},
		{"reserved chat", []string{"issue", "--data", dir, "--name", "chat/123"}},
		{"reserved eval", []string{"issue", "--data", dir, "--name", "eval/run/i1"}},
		{"zero ttl", []string{"issue", "--data", dir, "--name", "a", "--ttl", "0"}},
		{"zero days", []string{"issue", "--data", dir, "--name", "a", "--ttl", "0d"}},
		{"negative ttl", []string{"issue", "--data", dir, "--name", "a", "--ttl", "-1h"}},
		{"over max", []string{"issue", "--data", dir, "--name", "a", "--ttl", "31d"}},
		{"over max hours", []string{"issue", "--data", dir, "--name", "a", "--ttl", "721h"}},
		{"bad ttl", []string{"issue", "--data", dir, "--name", "a", "--ttl", "soon"}},
		{"bad format", []string{"issue", "--data", dir, "--name", "a", "--format", "yaml"}},
		{"missing data dir", []string{"issue", "--data", filepath.Join(dir, "nope"), "--name", "a"}},
		{"stray argument", []string{"issue", "--data", dir, "--name", "a", "extra"}},
	}
	for _, tc := range cases {
		out, errOut, code := runID(t, "", tc.args...)
		if code == 0 {
			t.Errorf("%s: accepted", tc.name)
		}
		if out != "" {
			t.Errorf("%s: stdout %q", tc.name, out)
		}
		if errOut == "" {
			t.Errorf("%s: no error message", tc.name)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "nope")); !errors.Is(err, os.ErrNotExist) {
		t.Error("issue created a missing data directory")
	}
}

func TestIdentityIssueTTLBounds(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for _, tc := range []struct {
		flag string
		want time.Duration
	}{
		{"30d", identity.MaxTTL},
		{"720h", identity.MaxTTL},
		{"90m", 90 * time.Minute},
		{"1d", 24 * time.Hour},
	} {
		claims, err := verifyIn(t, dir, issueToken(t, dir, "alice", "--ttl", tc.flag))
		if err != nil {
			t.Fatal(err)
		}
		if got := claims.ExpiresAt.Sub(claims.IssuedAt); got != tc.want {
			t.Errorf("--ttl %s: got %s, want %s", tc.flag, got, tc.want)
		}
	}
}

func TestIdentityRotate(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, config.IdentityKeyFile)
	old := issueToken(t, dir, "alice")
	signer, err := identity.NewSigner(dir)
	if err != nil {
		t.Fatal(err)
	}
	oldID, _ := signer.KeyID()
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	out, errOut, code := runID(t, "", "rotate", "--data", dir)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if os.SameFile(before, after) {
		t.Error("rotate kept the inode")
	}
	if perm := after.Mode().Perm(); perm != 0o600 {
		t.Errorf("mode %o, want 600", perm)
	}
	if _, err := verifyIn(t, dir, old); !errors.Is(err, identity.ErrUnknownKey) {
		t.Errorf("old token: err %v, want ErrUnknownKey", err)
	}
	fresh := issueToken(t, dir, "alice")
	claims, err := verifyIn(t, dir, fresh)
	if err != nil {
		t.Fatal(err)
	}
	if claims.KeyID == oldID {
		t.Error("key ID unchanged after rotate")
	}
	if !strings.Contains(out, claims.KeyID) || !strings.Contains(out, "was "+oldID) {
		t.Errorf("rotate output lacks key IDs: %q", out)
	}
	if strings.Contains(out, "ai1.") {
		t.Error("rotate printed a token")
	}
}

func TestIdentityRotateCreatesKey(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	out, errOut, code := runID(t, "", "rotate", "--data", dir)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	if !strings.HasPrefix(out, "wrote ") {
		t.Errorf("output %q", out)
	}
	if _, err := verifyIn(t, dir, issueToken(t, dir, "alice")); err != nil {
		t.Fatal(err)
	}
}

func TestIdentityShowKeyOnly(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_, _, code := runID(t, "", "show", "--data", dir)
	if code == 0 {
		t.Error("show succeeded without identity.key")
	}
	if _, err := os.Stat(filepath.Join(dir, config.IdentityKeyFile)); !errors.Is(err, os.ErrNotExist) {
		t.Error("show created identity.key")
	}

	issueToken(t, dir, "alice")
	signer, err := identity.NewSigner(dir)
	if err != nil {
		t.Fatal(err)
	}
	keyID, _ := signer.KeyID()
	out, _, code := runID(t, "", "show", "--data", dir)
	if code != 0 || out != "Key ID:     "+keyID+"\n" {
		t.Errorf("exit %d, output %q", code, out)
	}
}

func TestIdentityShowValidToken(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	token := issueToken(t, dir, "alice", "--ttl", "2d")

	for _, input := range []struct {
		arg, stdin string
	}{
		{token, ""},
		{"-", token + "\n"},
		{"-", "X-Artex-Identity: " + token + "\n"},
	} {
		out, errOut, code := runID(t, input.stdin, "show", "--data", dir, input.arg)
		if code != 0 {
			t.Fatalf("exit %d: %s%s", code, out, errOut)
		}
		if strings.Contains(out+errOut, token) || strings.Contains(out+errOut, strings.Split(token, ".")[2]) {
			t.Error("show echoed the token")
		}
		for _, want := range []string{"Name:       alice", "Valid:      yes", "Expires:"} {
			if !strings.Contains(out, want) {
				t.Errorf("output lacks %q:\n%s", want, out)
			}
		}
	}

	out, _, code := runID(t, "", "show", "--data", dir, "--format", "json", token)
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	var r showReport
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if r.Token == nil || !r.Token.Valid || r.Token.Name != "alice" || r.Token.KeyID != r.KeyID {
		t.Errorf("report %+v", r)
	}
	issued, _ := time.Parse(time.RFC3339, r.Token.IssuedAt)
	expires, _ := time.Parse(time.RFC3339, r.Token.ExpiresAt)
	if expires.Sub(issued) != 48*time.Hour {
		t.Errorf("issued %s expires %s", r.Token.IssuedAt, r.Token.ExpiresAt)
	}
	if strings.Contains(out, token) {
		t.Error("json output echoed the token")
	}
}

// forge signs a payload with dir's key the way identity.Signer does, so a
// test can produce a token whose expiry is already past.
func forge(t *testing.T, dir string, name string, iat, exp time.Time) string {
	t.Helper()
	key, err := config.IdentityKey(dir)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]any{"n": name, "k": identity.KeyID(key), "iat": iat.Unix(), "exp": exp.Unix()})
	p := base64.RawURLEncoding.EncodeToString(body)
	m := hmac.New(sha256.New, key)
	m.Write([]byte("artex-identity-v1\x00" + p))
	return "ai1." + p + "." + base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}

func TestIdentityShowInvalidTokens(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	good := issueToken(t, dir, "alice")
	parts := strings.Split(good, ".")
	expired := forge(t, dir, "alice", time.Now().Add(-48*time.Hour), time.Now().Add(-24*time.Hour))
	if _, err := verifyIn(t, dir, forge(t, dir, "alice", time.Now(), time.Now().Add(time.Hour))); err != nil {
		t.Fatalf("forge helper out of sync with identity: %v", err)
	}

	other := t.TempDir()
	foreign := issueToken(t, other, "alice")

	badMAC := parts[0] + "." + parts[1] + "." + base64.RawURLEncoding.EncodeToString(make([]byte, 32))

	cases := []struct {
		name    string
		token   string
		want    string
		decoded bool
	}{
		{"expired", expired, identity.ErrExpired.Error(), true},
		{"bad mac", badMAC, identity.ErrBadSignature.Error(), true},
		{"other data dir", foreign, "unknown key ID", true},
		{"garbage", "not-a-token", "malformed", false},
		{"wrong version", "ai0." + parts[1] + "." + parts[2], "malformed", false},
	}
	for _, tc := range cases {
		out, errOut, code := runID(t, "", "show", "--data", dir, tc.token)
		if code != 1 {
			t.Errorf("%s: exit %d", tc.name, code)
		}
		if !strings.Contains(out, "Valid:      no") || !strings.Contains(out, tc.want) {
			t.Errorf("%s: output lacks %q:\n%s", tc.name, tc.want, out)
		}
		if strings.Contains(out, "Name:       alice") != tc.decoded {
			t.Errorf("%s: decoded name shown = %v:\n%s", tc.name, !tc.decoded, out)
		}
		if strings.Contains(out+errOut, tc.token) {
			t.Errorf("%s: token echoed", tc.name)
		}
		if mac := strings.Split(tc.token, "."); len(mac) == 3 && strings.Contains(out+errOut, mac[2]) {
			t.Errorf("%s: mac echoed", tc.name)
		}
	}

	out, _, code := runID(t, "", "show", "--data", dir, "--format", "json", expired)
	var r showReport
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if code != 1 || r.Token == nil || r.Token.Valid || r.Token.Error != identity.ErrExpired.Error() {
		t.Errorf("exit %d report %+v", code, r.Token)
	}
}

func TestIdentityUsage(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{nil, {"bogus"}} {
		out, errOut, code := runID(t, "", args...)
		if code == 0 || out != "" || !strings.Contains(errOut, "usage: artex-cc identity") {
			t.Errorf("%v: exit %d stdout %q stderr %q", args, code, out, errOut)
		}
	}
	out, errOut, code := runID(t, "", "issue", "-h")
	if code != 0 || out != "" || !strings.Contains(errOut, "-name") {
		t.Errorf("issue -h: exit %d stdout %q stderr %q", code, out, errOut)
	}
}

func TestParseTTL(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]time.Duration{"7d": 7 * 24 * time.Hour, "12h": 12 * time.Hour, "1h30m": 90 * time.Minute} {
		got, err := parseTTL(in)
		if err != nil || got != want {
			t.Errorf("%s: got %s, %v", in, got, err)
		}
	}
	for _, in := range []string{"", "d", "1.5d", "7days", "x"} {
		if _, err := parseTTL(in); err == nil {
			t.Errorf("%q accepted", in)
		}
	}
}
