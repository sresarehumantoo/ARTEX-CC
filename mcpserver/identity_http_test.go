package mcpserver_test

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SREsAreHumanToo/ARTEX-CC/config"
	"github.com/SREsAreHumanToo/ARTEX-CC/identity"
	"github.com/SREsAreHumanToo/ARTEX-CC/mcpserver"
	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
)

func newIntentStore(ids ...int64) *claimStore {
	st := &claimStore{nodes: map[int64]*tools.Node{}, claims: map[int64]tools.IntentClaim{}}
	for _, id := range ids {
		st.nodes[id] = &tools.Node{ID: id, Kind: tools.KindIntent, State: "open"}
	}
	return st
}

// startIdentityServer serves intent tools behind the identity middleware
// for the signer of dataDir, the way both HTTP mount paths do.
func startIdentityServer(t *testing.T, st *claimStore, dataDir string) string {
	t.Helper()
	reg := tools.NewRegistry()
	tools.RegisterIntentTools(reg, st, 1)
	srv := mcpserver.New(reg)
	signer, err := identity.NewSigner(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv.IdentityMiddleware(signer)(srv.StreamableHTTP()))
	t.Cleanup(ts.Close)
	return ts.URL + "/mcp"
}

func issue(t *testing.T, dataDir, name string) string {
	t.Helper()
	signer, err := identity.NewSigner(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	token, err := signer.Issue(name, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func identityHeader(token string) http.Header {
	return http.Header{mcpserver.IdentityHeader: {token}}
}

func modernTool(t *testing.T, url string, h http.Header, tool string, args map[string]any) (string, bool) {
	t.Helper()
	resp := postModern(t, url, h, tool, args)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("%s: HTTP %d %s", tool, resp.StatusCode, body)
	}
	return decodeToolResult(t, tool, resp)
}

func postLegacy(t *testing.T, url, sid string, h http.Header, method string, params any) *http.Response {
	t.Helper()
	raw, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 2, "method": method, "params": params})
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if sid != "" {
		req.Header.Set("Mcp-Session-Id", sid)
	}
	for k, v := range h {
		req.Header[k] = v
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func legacyInit(t *testing.T, url string, h http.Header) string {
	t.Helper()
	resp := postLegacy(t, url, "", h, "initialize", map[string]any{
		"protocolVersion": "2025-03-26",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "test", "version": "1.0"},
	})
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	sid := resp.Header.Get("Mcp-Session-Id")
	if resp.StatusCode != http.StatusOK || sid == "" {
		t.Fatalf("initialize: HTTP %d, session %q", resp.StatusCode, sid)
	}
	return sid
}

func legacyTool(t *testing.T, url, sid string, h http.Header, tool string, args map[string]any) (string, bool) {
	t.Helper()
	resp := postLegacy(t, url, sid, h, "tools/call", map[string]any{"name": tool, "arguments": args})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("%s: HTTP %d %s", tool, resp.StatusCode, body)
	}
	return decodeToolResult(t, tool, resp)
}

func TestIdentityHoldsClaimsModern(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	st := newIntentStore(10, 11)
	url := startIdentityServer(t, st, dir)
	alice := identityHeader(issue(t, dir, "alice"))
	bob := identityHeader(issue(t, dir, "bob"))

	out, isErr := modernTool(t, url, alice, "start_intent", map[string]any{"node_id": 10})
	if isErr || !strings.Contains(out, "claimed by alice until") {
		t.Fatalf("alice claim without owner = %q", out)
	}
	if got := st.owner(10); got != "id:alice" {
		t.Fatalf("stored owner = %q, want id:alice", got)
	}

	out, isErr = modernTool(t, url, bob, "start_intent", map[string]any{"node_id": 10})
	if !isErr || !strings.Contains(out, "claimed by alice until") {
		t.Fatalf("bob on alice's intent = %q, want conflict naming alice", out)
	}
	out, isErr = modernTool(t, url, bob, "complete_intent", map[string]any{"node_id": 10})
	if !isErr || !strings.Contains(out, "claimed by alice") {
		t.Fatalf("bob completing alice's intent = %q, want conflict", out)
	}

	out, isErr = modernTool(t, url, alice, "start_intent", map[string]any{"node_id": 11, "owner": "sub1"})
	if isErr || !strings.Contains(out, "claimed by alice/h-") || strings.Contains(out, "sub1") {
		t.Fatalf("alice claim with owner = %q, want alice/h- label without the owner", out)
	}
	if got := st.owner(11); got != "id:alice/owner:sub1" {
		t.Fatalf("stored owner = %q, want id:alice/owner:sub1", got)
	}
	out, isErr = modernTool(t, url, bob, "start_intent", map[string]any{"node_id": 11, "owner": "sub1"})
	if !isErr || !strings.Contains(out, "claimed by alice/h-") {
		t.Fatalf("bob copying alice's owner = %q, want conflict", out)
	}
	out, isErr = modernTool(t, url, alice, "start_intent", map[string]any{"node_id": 11})
	if !isErr {
		t.Fatalf("alice without owner on alice/sub1's intent = %q, want conflict", out)
	}
	out, isErr = modernTool(t, url, nil, "start_intent", map[string]any{"node_id": 10, "owner": "alice"})
	if !isErr || !strings.Contains(out, "claimed by alice") {
		t.Fatalf("anonymous owner \"alice\" = %q, want conflict", out)
	}

	out, isErr = modernTool(t, url, alice, "start_intent", map[string]any{"node_id": 10})
	if isErr || !strings.Contains(out, "renewed") {
		t.Fatalf("alice renewal = %q", out)
	}

	if out, isErr := modernTool(t, url, nil, "start_intent", map[string]any{"node_id": 10}); !isErr || !strings.Contains(out, "owner") {
		t.Fatalf("no header and no owner = %q, want refusal naming owner", out)
	}
}

func TestIdentityHoldsClaimsAcrossReconnectsAndEras(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	st := newIntentStore(10, 12)
	url := startIdentityServer(t, st, dir)
	alice := identityHeader(issue(t, dir, "alice"))
	bob := identityHeader(issue(t, dir, "bob"))

	first := legacyInit(t, url, alice)
	if out, isErr := legacyTool(t, url, first, alice, "start_intent", map[string]any{"node_id": 10}); isErr || !strings.Contains(out, "claimed by alice") {
		t.Fatalf("legacy alice claim = %q", out)
	}
	if got := st.owner(10); got != "id:alice" {
		t.Fatalf("stored owner = %q, want id:alice without the session", got)
	}

	second := legacyInit(t, url, alice)
	if second == first {
		t.Fatal("reconnect reused the session ID")
	}
	if out, isErr := legacyTool(t, url, second, alice, "start_intent", map[string]any{"node_id": 10}); isErr || !strings.Contains(out, "renewed") {
		t.Fatalf("alice after reconnect = %q, want renewal", out)
	}
	if out, isErr := modernTool(t, url, alice, "start_intent", map[string]any{"node_id": 10}); isErr || !strings.Contains(out, "renewed") {
		t.Fatalf("alice on 2026-07-28 = %q, want renewal", out)
	}

	if out, isErr := legacyTool(t, url, first, bob, "start_intent", map[string]any{"node_id": 10}); !isErr || !strings.Contains(out, "claimed by alice") {
		t.Fatalf("bob on alice's session ID = %q, want conflict", out)
	}

	anon := legacyInit(t, url, nil)
	if out, isErr := legacyTool(t, url, anon, nil, "start_intent", map[string]any{"node_id": 12}); isErr || strings.Contains(out, anon) {
		t.Fatalf("anonymous legacy claim = %q", out)
	}
	if got := st.owner(12); got != anon {
		t.Fatalf("anonymous stored owner = %q, want session %q", got, anon)
	}
	if out, isErr := legacyTool(t, url, anon, nil, "start_intent", map[string]any{"node_id": 12}); isErr || !strings.Contains(out, "renewed by "+tools.OwnerLabel(anon)) {
		t.Fatalf("anonymous label = %q, want hashed session label", out)
	}
}

func TestIdentitySharedAcrossServersOnOneDataDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	st := newIntentStore(10)
	one := startIdentityServer(t, st, dir)
	two := startIdentityServer(t, st, dir)
	alice := identityHeader(issue(t, dir, "alice"))
	bob := identityHeader(issue(t, dir, "bob"))

	if out, isErr := modernTool(t, one, alice, "start_intent", map[string]any{"node_id": 10}); isErr {
		t.Fatalf("alice on server one = %q", out)
	}
	if out, isErr := modernTool(t, two, bob, "start_intent", map[string]any{"node_id": 10}); !isErr || !strings.Contains(out, "claimed by alice") {
		t.Fatalf("bob on server two = %q, want conflict naming alice", out)
	}
	if out, isErr := modernTool(t, two, alice, "start_intent", map[string]any{"node_id": 10}); isErr || !strings.Contains(out, "renewed") {
		t.Fatalf("alice on server two = %q, want renewal", out)
	}
}

func TestChatIdentitiesHoldClaimsAcrossRestart(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	st := newIntentStore(10, 11)
	const convA, convB = "chat/0b7c3c4e-5d3f-4b8e-9a51-2f0c6d1e8a90", "chat/9f2e1d0c-4b3a-4c5d-8e7f-a1b2c3d4e5f6"
	before := startIdentityServer(t, st, dir)
	a, b := identityHeader(issue(t, dir, convA)), identityHeader(issue(t, dir, convB))

	if out, isErr := modernTool(t, before, a, "start_intent", map[string]any{"node_id": 10}); isErr || !strings.Contains(out, "claimed by "+convA) {
		t.Fatalf("conversation A claim without owner = %q", out)
	}
	if out, isErr := modernTool(t, before, b, "start_intent", map[string]any{"node_id": 11}); isErr || !strings.Contains(out, "claimed by "+convB) {
		t.Fatalf("conversation B claim without owner = %q", out)
	}
	if got := st.owner(10); got != "id:"+convA {
		t.Fatalf("stored owner = %q, want id:%s", got, convA)
	}
	if got := st.owner(11); got != "id:"+convB {
		t.Fatalf("stored owner = %q, want id:%s", got, convB)
	}
	if out, isErr := modernTool(t, before, b, "start_intent", map[string]any{"node_id": 10}); !isErr || !strings.Contains(out, "claimed by "+convA) {
		t.Fatalf("conversation B on A's intent = %q, want conflict naming A", out)
	}

	after := startIdentityServer(t, st, dir)
	reminted := identityHeader(issue(t, dir, convA))
	for name, h := range map[string]http.Header{"old token": a, "reminted token": reminted} {
		if out, isErr := modernTool(t, after, h, "start_intent", map[string]any{"node_id": 10}); isErr || !strings.Contains(out, "renewed") {
			t.Fatalf("conversation A with %s after restart = %q, want renewal", name, out)
		}
	}
	if out, isErr := modernTool(t, after, b, "start_intent", map[string]any{"node_id": 10}); !isErr || !strings.Contains(out, "claimed by "+convA) {
		t.Fatalf("conversation B on A's intent after restart = %q, want conflict naming A", out)
	}

	if _, _, err := config.RotateIdentityKey(dir); err != nil {
		t.Fatal(err)
	}
	rotated := startIdentityServer(t, st, dir)
	for name, h := range map[string]http.Header{"A": a, "A reminted": reminted, "B": b} {
		resp := postModern(t, rotated, h, "start_intent", map[string]any{"node_id": 10})
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest || !strings.Contains(string(body), "unknown key ID") {
			t.Errorf("%s after rotation: HTTP %d %s, want 400 unknown key ID", name, resp.StatusCode, body)
		}
	}
}

func TestRetiredConversationHeaderRejected(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	st := newIntentStore(10)
	url := startIdentityServer(t, st, dir)
	for name, h := range map[string]http.Header{
		"alone":         {"X-Artex-Conversation": {"conv-a.deadbeef"}},
		"with identity": {"X-Artex-Conversation": {"conv-a.deadbeef"}, mcpserver.IdentityHeader: {issue(t, dir, "alice")}},
	} {
		resp := postModern(t, url, h, "start_intent", map[string]any{"node_id": 10, "owner": "x"})
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest || !strings.Contains(string(body), "start a new web chat conversation") {
			t.Errorf("%s: HTTP %d %s, want 400 telling the caller to start a new conversation", name, resp.StatusCode, body)
		}
	}
	if got := st.owner(10); got != "" {
		t.Fatalf("tool ran despite the retired header: owner %q", got)
	}
}

func forgeToken(key []byte, payload map[string]any) string {
	body, _ := json.Marshal(payload)
	p := base64.RawURLEncoding.EncodeToString(body)
	m := hmac.New(sha256.New, key)
	m.Write([]byte("artex-identity-v1\x00"))
	m.Write([]byte(p))
	return "ai1." + p + "." + base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}

func badIdentityTokens(t *testing.T, dir string) map[string]string {
	t.Helper()
	valid := issue(t, dir, "alice")
	parts := strings.Split(valid, ".")
	key, err := config.IdentityKey(dir)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	bob, _ := json.Marshal(map[string]any{"n": "bob", "k": identity.KeyID(key), "iat": now.Unix(), "exp": now.Add(time.Hour).Unix()})
	bobPayload := base64.RawURLEncoding.EncodeToString(bob)
	flipped := []byte(parts[2])
	if flipped[5] == 'A' {
		flipped[5] = 'B'
	} else {
		flipped[5] = 'A'
	}
	otherKey := make([]byte, 32)
	rand.Read(otherKey)

	return map[string]string{
		"tampered payload": "ai1." + bobPayload + "." + parts[2],
		"flipped mac":      parts[0] + "." + parts[1] + "." + string(flipped),
		"truncated":        parts[0] + "." + parts[1],
		"wrong version":    "ai0." + parts[1] + "." + parts[2],
		"garbage":          "not-a-token",
		"oversized":        "ai1." + strings.Repeat("A", 4096) + "." + parts[2],
		"expired": forgeToken(key, map[string]any{
			"n": "alice", "k": identity.KeyID(key), "iat": now.Add(-2 * time.Hour).Unix(), "exp": now.Add(-time.Hour).Unix(),
		}),
		"other data dir": issue(t, t.TempDir(), "alice"),
		"other key, same key ID": forgeToken(otherKey, map[string]any{
			"n": "alice", "k": identity.KeyID(key), "iat": now.Unix(), "exp": now.Add(time.Hour).Unix(),
		}),
	}
}

func TestIdentityRejectsBadTokens(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	st := newIntentStore(10)
	url := startIdentityServer(t, st, dir)
	alice := identityHeader(issue(t, dir, "alice"))
	sid := legacyInit(t, url, alice)
	if _, isErr := modernTool(t, url, alice, "start_intent", map[string]any{"node_id": 10}); isErr {
		t.Fatal("alice claim failed")
	}

	cases := badIdentityTokens(t, dir)
	for name, token := range cases {
		check := func(era string, resp *http.Response) {
			t.Helper()
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)
			if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("%s on %s: HTTP %d %s, want 400", name, era, resp.StatusCode, body)
			}
			if !strings.Contains(string(body), mcpserver.IdentityHeader) {
				t.Errorf("%s on %s: body %q does not name the header", name, era, body)
			}
			if len(token) > 8 && strings.Contains(string(body), token) {
				t.Errorf("%s on %s: body echoes the token", name, era)
			}
		}
		check("2026-07-28", postModern(t, url, identityHeader(token), "complete_intent", map[string]any{"node_id": 10}))
		check("legacy", postLegacy(t, url, sid, identityHeader(token), "tools/call", map[string]any{
			"name": "complete_intent", "arguments": map[string]any{"node_id": 10},
		}))
	}
	for name, h := range map[string]http.Header{
		"empty":    {mcpserver.IdentityHeader: {""}},
		"repeated": {mcpserver.IdentityHeader: {issue(t, dir, "bob"), issue(t, dir, "bob")}},
	} {
		resp := postModern(t, url, h, "complete_intent", map[string]any{"node_id": 10})
		resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("%s header: HTTP %d, want 400", name, resp.StatusCode)
		}
	}
	if got := st.owner(10); got != "id:alice" || st.nodes[10].State != "running" {
		t.Fatalf("intent 10 owner %q state %q, want alice's claim untouched", got, st.nodes[10].State)
	}
}

func TestIdentityRejectsTokensAfterRotation(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	old := issue(t, dir, "alice")
	key := make([]byte, 32)
	rand.Read(key)
	path := filepath.Join(dir, config.IdentityKeyFile)
	if err := os.WriteFile(path, []byte(hex.EncodeToString(key)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	url := startIdentityServer(t, newIntentStore(10), dir)

	resp := postModern(t, url, identityHeader(old), "start_intent", map[string]any{"node_id": 10})
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest || !strings.Contains(string(body), "unknown key ID") {
		t.Fatalf("pre-rotation token: HTTP %d %s, want 400 unknown key ID", resp.StatusCode, body)
	}
	if out, isErr := modernTool(t, url, identityHeader(issue(t, dir, "alice")), "start_intent", map[string]any{"node_id": 10}); isErr {
		t.Fatalf("post-rotation token = %q", out)
	}
}

func TestListSessionsShowsIdentities(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	url := startIdentityServer(t, newIntentStore(), dir)
	alice := identityHeader(issue(t, dir, "alice"))
	bob := identityHeader(issue(t, dir, "bob"))
	modernTool(t, url, bob, "list_sessions", map[string]any{})

	out, _ := modernTool(t, url, alice, "list_sessions", map[string]any{})
	var res struct {
		Sessions   []json.RawMessage `json:"sessions"`
		Identities []struct {
			Name       string    `json:"name"`
			Self       bool      `json:"self"`
			LastActive time.Time `json:"last_active"`
		} `json:"identities"`
	}
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("list_sessions: %v\n%s", err, out)
	}
	if res.Sessions == nil {
		t.Errorf("sessions array missing: %s", out)
	}
	self := map[string]bool{}
	for _, id := range res.Identities {
		self[id.Name] = id.Self
		if id.LastActive.IsZero() {
			t.Errorf("identity %s has no last_active", id.Name)
		}
	}
	if len(res.Identities) != 2 || !self["alice"] || self["bob"] {
		t.Fatalf("identities = %+v, want alice (self) and bob", res.Identities)
	}

	anon, _ := modernTool(t, url, nil, "list_sessions", map[string]any{})
	if strings.Contains(anon, `"self":true`) {
		t.Errorf("anonymous caller marked as self: %s", anon)
	}
}

func TestIdentityTokensNeverLogged(t *testing.T) {
	buf := captureLog(t)
	dir := t.TempDir()
	url := startIdentityServer(t, newIntentStore(10), dir)
	valid := issue(t, dir, "alice")
	tokens := []string{valid}
	for _, token := range badIdentityTokens(t, dir) {
		tokens = append(tokens, token)
	}
	sid := legacyInit(t, url, identityHeader(valid))
	for _, token := range tokens {
		postModern(t, url, identityHeader(token), "start_intent", map[string]any{"node_id": 10}).Body.Close()
		postLegacy(t, url, sid, identityHeader(token), "tools/call", map[string]any{
			"name": "start_intent", "arguments": map[string]any{"node_id": 10},
		}).Body.Close()
	}
	out := buf.String()
	if out == "" {
		t.Fatal("no log output captured")
	}
	for _, token := range tokens {
		parts := strings.Split(token, ".")
		for _, p := range parts {
			if len(p) > 16 && strings.Contains(out, p) {
				t.Fatalf("token segment %q in log:\n%s", p, out)
			}
		}
	}
	if !strings.Contains(out, "tool called") {
		t.Fatalf("tool calls not logged at debug level:\n%s", out)
	}
}
