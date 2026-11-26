package tools

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

type intentMockStore struct {
	mockStore
	nodes  map[int64]*Node
	claims map[int64]IntentClaim
}

func newIntentMockStore() *intentMockStore {
	return &intentMockStore{nodes: map[int64]*Node{}, claims: map[int64]IntentClaim{}}
}

func (m *intentMockStore) ClaimIntent(_ context.Context, id int64, owner string, lease time.Duration) (*IntentClaim, error) {
	n, ok := m.nodes[id]
	if !ok {
		return nil, fmt.Errorf("node %d not found", id)
	}
	if n.State != "open" && n.State != "running" {
		return nil, fmt.Errorf("intent %d is %q, must be open to start", id, n.State)
	}
	prev, held := m.claims[id]
	live := n.State == "running" && held && prev.ExpiresAt.After(time.Now())
	if live && prev.Owner != owner {
		return nil, &ClaimConflictError{NodeID: id, Owner: prev.Owner, ExpiresAt: prev.ExpiresAt}
	}
	n.State = "running"
	c := IntentClaim{NodeID: id, Owner: owner, ExpiresAt: time.Now().Add(lease), Renewed: live}
	m.claims[id] = c
	return &c, nil
}

func (m *intentMockStore) NodeDetail(_ context.Context, id int64) (*Node, error) {
	n, ok := m.nodes[id]
	if !ok {
		return nil, nil
	}
	return n, nil
}

func (m *intentMockStore) UpdateNodeState(_ context.Context, id int64, state string) error {
	n, ok := m.nodes[id]
	if !ok {
		return fmt.Errorf("node %d not found", id)
	}
	n.State = state
	delete(m.claims, id)
	return nil
}

func (m *intentMockStore) UpdateNodeFields(_ context.Context, id int64, state string, merge map[string]any) error {
	n, ok := m.nodes[id]
	if !ok {
		return fmt.Errorf("node %d not found", id)
	}
	n.State = state
	for k, v := range merge {
		if n.Payload == nil {
			n.Payload = map[string]any{}
		}
		n.Payload[k] = v
	}
	delete(m.claims, id)
	return nil
}

var testSessionCtx = WithSessionID(context.Background(), "sess-test")

func TestStartIntentLeaseDescription(t *testing.T) {
	previous := IntentLease
	t.Cleanup(func() { IntentLease = previous })
	for _, tc := range []struct {
		lease time.Duration
		want  string
	}{
		{time.Minute, "exclusive for 1 minute and renews"},
		{30 * time.Minute, "exclusive for 30 minutes and renews"},
	} {
		IntentLease = tc.lease
		if got := makeStartIntent(nil).Description; !strings.Contains(got, tc.want) {
			t.Errorf("lease %v: description %q does not contain %q", tc.lease, got, tc.want)
		}
	}
}

func TestStartIntentClaimConflictAndRenewal(t *testing.T) {
	t.Parallel()
	ms := newIntentMockStore()
	ms.nodes[10] = &Node{ID: 10, Kind: KindIntent, State: "open"}

	reg := NewRegistry()
	RegisterIntentTools(reg, ms, 1)
	start, _ := reg.Lookup("start_intent")

	ctxA := WithSessionID(context.Background(), "sess-a")
	ctxB := WithSessionID(context.Background(), "sess-b")
	in := json.RawMessage(`{"node_id": 10}`)

	r, _ := start.Run(ctxA, in)
	if r.IsError {
		t.Fatalf("session A claim failed: %s", r.Content)
	}
	if !strings.Contains(r.Content, "claimed by "+OwnerLabel("sess-a")) {
		t.Errorf("expected claim by sess-a, got: %s", r.Content)
	}

	r, _ = start.Run(ctxB, in)
	if !r.IsError {
		t.Fatal("expected session B to be refused")
	}
	if !strings.Contains(r.Content, "claimed by "+OwnerLabel("sess-a")+" until") {
		t.Errorf("expected error naming holder and expiry, got: %s", r.Content)
	}

	r, _ = start.Run(ctxA, in)
	if r.IsError {
		t.Fatalf("session A renewal failed: %s", r.Content)
	}
	if !strings.Contains(r.Content, "renewed by "+OwnerLabel("sess-a")) {
		t.Errorf("expected renewal, got: %s", r.Content)
	}
}

func TestStartIntentExpiredClaimIsTakenOver(t *testing.T) {
	t.Parallel()
	ms := newIntentMockStore()
	ms.nodes[10] = &Node{ID: 10, Kind: KindIntent, State: "running"}
	ms.claims[10] = IntentClaim{NodeID: 10, Owner: "sess-a", ExpiresAt: time.Now().Add(-time.Minute)}

	reg := NewRegistry()
	RegisterIntentTools(reg, ms, 1)
	start, _ := reg.Lookup("start_intent")

	r, _ := start.Run(WithSessionID(context.Background(), "sess-b"), json.RawMessage(`{"node_id": 10}`))
	if r.IsError {
		t.Fatalf("expected takeover of expired claim, got: %s", r.Content)
	}
	if ms.claims[10].Owner != "sess-b" {
		t.Errorf("owner = %q, want sess-b", ms.claims[10].Owner)
	}
}

func TestCompleteAndCancelReleaseClaim(t *testing.T) {
	t.Parallel()
	for _, tool := range []string{"complete_intent", "cancel_intent"} {
		ms := newIntentMockStore()
		ms.nodes[10] = &Node{ID: 10, Kind: KindIntent, State: "open"}
		reg := NewRegistry()
		RegisterIntentTools(reg, ms, 1)
		start, _ := reg.Lookup("start_intent")
		finish, _ := reg.Lookup(tool)

		ctx := WithSessionID(context.Background(), "sess-a")
		if r, _ := start.Run(ctx, json.RawMessage(`{"node_id": 10}`)); r.IsError {
			t.Fatalf("start failed: %s", r.Content)
		}
		if r, _ := finish.Run(ctx, json.RawMessage(`{"node_id": 10}`)); r.IsError {
			t.Fatalf("%s failed: %s", tool, r.Content)
		}
		if _, held := ms.claims[10]; held {
			t.Errorf("%s left the claim in place", tool)
		}
	}
}

func TestParseIntentLease(t *testing.T) {
	t.Parallel()
	good := map[string]time.Duration{"1m": time.Minute, "2m": 2 * time.Minute, " 45m ": 45 * time.Minute, "1h": time.Hour}
	for in, want := range good {
		got, err := ParseIntentLease(in)
		if err != nil || got != want {
			t.Errorf("ParseIntentLease(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	for _, in := range []string{"", "30", "30s", "90s", "0m", "-5m", "soon"} {
		if got, err := ParseIntentLease(in); err == nil {
			t.Errorf("ParseIntentLease(%q) = %v, want an error", in, got)
		}
	}
}

func TestClaimOwner(t *testing.T) {
	t.Parallel()
	bare := context.Background()
	sess := WithSessionID(bare, "stdio-ab12")
	cases := []struct {
		ctx   context.Context
		owner string
		want  string
	}{
		{sess, "", "stdio-ab12"},
		{sess, " sub1 ", "stdio-ab12/owner:sub1"},
		{bare, "f00d", "owner:f00d"},
		{WithSessionID(bare, ""), "f00d", "owner:f00d"},
		{WithIdentity(bare, "alice"), "", "id:alice"},
		{WithIdentity(bare, "alice"), " sub1 ", "id:alice/owner:sub1"},
		{WithIdentity(sess, "alice"), "", "id:alice"},
		{WithIdentity(sess, "alice"), "sub1", "id:alice/owner:sub1"},
		{WithIdentity(sess, "alice"), "   ", "id:alice"},
		{WithIdentity(sess, ""), "", "stdio-ab12"},
	}
	for _, c := range cases {
		got, err := ClaimOwner(c.ctx, c.owner)
		if err != nil || got != c.want {
			t.Errorf("ClaimOwner(%q, %q, %q) = %q, %v; want %q", IdentityName(c.ctx), SessionID(c.ctx), c.owner, got, err, c.want)
		}
	}
	if _, err := ClaimOwner(bare, "  "); !errors.Is(err, ErrNoClaimOwner) {
		t.Errorf("ClaimOwner with neither = %v, want ErrNoClaimOwner", err)
	}
}

func TestOwnerLabelerGolden(t *testing.T) {
	t.Parallel()
	l := NewOwnerLabeler(bytes.Repeat([]byte{1}, 32))
	cases := map[string]string{
		"mcp-session-1":               "h-06e9ddc04fee",
		"owner:f00d":                  "h-5e7da8e37e55",
		"id:alice":                    "alice",
		"id:eval/r1/i2":               "eval/r1/i2",
		"id:alice/owner:sub1":         "alice/h-",
		"id:chat/abc/owner:x/owner:y": "chat/abc/h-",
	}
	for owner, want := range cases {
		got := l.Label(owner)
		if strings.HasSuffix(want, "h-") {
			if !strings.HasPrefix(got, want) || len(got) != len(want)+12 || strings.Contains(got, "sub1") {
				t.Errorf("Label(%q) = %q, want %q plus a 12 hex hash", owner, got, want)
			}
			continue
		}
		if got != want {
			t.Errorf("Label(%q) = %q, want %q", owner, got, want)
		}
	}
	if l.Label("id:alice/owner:sub1") == l.Label("id:alice/owner:sub2") {
		t.Error("owners under one identity must get distinct labels")
	}
	if l.Label("id:alice/owner:sub1") == l.Label("id:bob/owner:sub1") {
		t.Error("the same owner under two identities must get distinct labels")
	}
}

func TestOwnerLabel(t *testing.T) {
	t.Parallel()
	a, b := OwnerLabel("mcp-session-1"), OwnerLabel("mcp-session-2")
	if a == b || !strings.HasPrefix(a, "h-") || len(a) != len("h-")+12 {
		t.Errorf("labels %q, %q", a, b)
	}
	if strings.Contains(a, "session") || OwnerLabel("mcp-session-1") != a {
		t.Errorf("label %q must be a stable hash of the owner", a)
	}
	if OwnerLabel("") != "" {
		t.Error("empty owner must have an empty label")
	}
	err := &ClaimConflictError{NodeID: 1, Owner: "mcp-session-1", ExpiresAt: time.Now().Add(time.Minute)}
	if strings.Contains(err.Error(), "mcp-session-1") || !strings.Contains(err.Error(), a) {
		t.Errorf("conflict error %q must show the label, not the owner", err)
	}
}

func TestOwnerLabelerKeyed(t *testing.T) {
	t.Parallel()
	k1 := bytes.Repeat([]byte{1}, 32)
	k2 := bytes.Repeat([]byte{2}, 32)
	a1, a1again := NewOwnerLabeler(k1), NewOwnerLabeler(k1)
	a2 := NewOwnerLabeler(k2)
	if a1.Label("agent1") != a1again.Label("agent1") {
		t.Error("labels must be stable for the same key")
	}
	if a1.Label("agent1") == a2.Label("agent1") {
		t.Error("labels must differ between keys")
	}
	sum := sha256.Sum256([]byte("agent1"))
	if a1.Label("agent1") == "h-"+hex.EncodeToString(sum[:6]) {
		t.Error("label must not be the unkeyed hash of the owner")
	}
}

func TestClaimToolsRequireOwnerWithoutSession(t *testing.T) {
	t.Parallel()
	ms := newIntentMockStore()
	ms.nodes[10] = &Node{ID: 10, Kind: KindIntent, State: "open"}
	ms.nodes[11] = &Node{ID: 11, Kind: KindIntent, State: "running"}
	reg := NewRegistry()
	RegisterIntentTools(reg, ms, 1)

	for _, c := range []struct {
		tool string
		id   int
	}{{"start_intent", 10}, {"complete_intent", 11}, {"cancel_intent", 11}} {
		tool, _ := reg.Lookup(c.tool)
		r, _ := tool.Run(context.Background(), json.RawMessage(fmt.Sprintf(`{"node_id": %d}`, c.id)))
		if !r.IsError || !strings.Contains(r.Content, "pass owner") {
			t.Errorf("%s without session or owner = %q, want refusal", c.tool, r.Content)
		}
	}
	if ms.nodes[10].State != "open" || ms.nodes[11].State != "running" {
		t.Errorf("refused calls changed state: %s, %s", ms.nodes[10].State, ms.nodes[11].State)
	}

	start, _ := reg.Lookup("start_intent")
	if r, _ := start.Run(context.Background(), json.RawMessage(`{"node_id": 10, "owner": "f00d"}`)); r.IsError {
		t.Fatalf("start with owner: %s", r.Content)
	}
	if ms.claims[10].Owner != "owner:f00d" {
		t.Errorf("owner = %q", ms.claims[10].Owner)
	}
}

func TestCompleteIntentClaimedByOtherSession(t *testing.T) {
	t.Parallel()
	ms := newIntentMockStore()
	ms.nodes[10] = &Node{ID: 10, Kind: KindIntent, State: "running"}
	ms.claims[10] = IntentClaim{NodeID: 10, Owner: "other", ExpiresAt: time.Now().Add(time.Hour)}
	reg := NewRegistry()
	RegisterIntentTools(reg, ms, 1)
	tool, _ := reg.Lookup("complete_intent")

	r, _ := tool.Run(WithSessionID(context.Background(), "me"), json.RawMessage(`{"node_id": 10, "result": "x"}`))
	if !r.IsError || !strings.Contains(r.Content, "claimed by "+OwnerLabel("other")) {
		t.Fatalf("expected claim conflict, got: %s", r.Content)
	}
	if ms.nodes[10].State != "running" {
		t.Errorf("state = %s, want running", ms.nodes[10].State)
	}

	r, _ = tool.Run(WithSessionID(context.Background(), "other"), json.RawMessage(`{"node_id": 10}`))
	if r.IsError || ms.nodes[10].State != "done" {
		t.Fatalf("holder complete = %q, state %s", r.Content, ms.nodes[10].State)
	}
}

func TestStartIntentHappyPath(t *testing.T) {
	t.Parallel()
	ms := newIntentMockStore()
	ms.nodes[10] = &Node{ID: 10, Kind: KindIntent, State: "open"}

	reg := NewRegistry()
	RegisterIntentTools(reg, ms, 1)

	tool, ok := reg.Lookup("start_intent")
	if !ok {
		t.Fatal("start_intent not found")
	}

	r, err := tool.Run(testSessionCtx, json.RawMessage(`{"node_id": 10}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
	if !strings.Contains(r.Content, "running") {
		t.Errorf("expected 'running' in result, got: %s", r.Content)
	}
	if ms.nodes[10].State != "running" {
		t.Errorf("expected state running, got %s", ms.nodes[10].State)
	}
}

func TestStartIntentWrongState(t *testing.T) {
	t.Parallel()
	ms := newIntentMockStore()
	ms.nodes[10] = &Node{ID: 10, Kind: KindIntent, State: "done"}

	reg := NewRegistry()
	RegisterIntentTools(reg, ms, 1)

	tool, _ := reg.Lookup("start_intent")
	r, err := tool.Run(testSessionCtx, json.RawMessage(`{"node_id": 10}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for starting a done intent")
	}
	if !strings.Contains(r.Content, "must be open") {
		t.Errorf("expected 'must be open' in error, got: %s", r.Content)
	}
}

func TestStartIntentWrongKind(t *testing.T) {
	t.Parallel()
	ms := newIntentMockStore()
	ms.nodes[10] = &Node{ID: 10, Kind: KindFact, State: "confirmed"}

	reg := NewRegistry()
	RegisterIntentTools(reg, ms, 1)

	tool, _ := reg.Lookup("start_intent")
	r, err := tool.Run(testSessionCtx, json.RawMessage(`{"node_id": 10}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for non-intent node")
	}
	if !strings.Contains(r.Content, "not an intent") {
		t.Errorf("expected 'not an intent' in error, got: %s", r.Content)
	}
}

func TestStartIntentMissingNodeID(t *testing.T) {
	t.Parallel()
	ms := newIntentMockStore()
	reg := NewRegistry()
	RegisterIntentTools(reg, ms, 1)

	tool, _ := reg.Lookup("start_intent")
	r, err := tool.Run(testSessionCtx, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for missing node_id")
	}
}

func TestStartIntentNotFound(t *testing.T) {
	t.Parallel()
	ms := newIntentMockStore()
	reg := NewRegistry()
	RegisterIntentTools(reg, ms, 1)

	tool, _ := reg.Lookup("start_intent")
	r, err := tool.Run(testSessionCtx, json.RawMessage(`{"node_id": 999}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for nonexistent node")
	}
	if !strings.Contains(r.Content, "not found") {
		t.Errorf("expected 'not found' in error, got: %s", r.Content)
	}
}

func TestCompleteIntentFromRunning(t *testing.T) {
	t.Parallel()
	ms := newIntentMockStore()
	ms.nodes[10] = &Node{ID: 10, Kind: KindIntent, State: "running"}

	reg := NewRegistry()
	RegisterIntentTools(reg, ms, 1)

	tool, _ := reg.Lookup("complete_intent")
	r, err := tool.Run(testSessionCtx, json.RawMessage(`{"node_id": 10, "result": "found XSS"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
	if ms.nodes[10].State != "done" {
		t.Errorf("expected state done, got %s", ms.nodes[10].State)
	}
}

func TestCompleteIntentFromOpen(t *testing.T) {
	t.Parallel()
	ms := newIntentMockStore()
	ms.nodes[10] = &Node{ID: 10, Kind: KindIntent, State: "open"}

	reg := NewRegistry()
	RegisterIntentTools(reg, ms, 1)

	tool, _ := reg.Lookup("complete_intent")
	r, err := tool.Run(testSessionCtx, json.RawMessage(`{"node_id": 10}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success (open -> done allowed), got error: %s", r.Content)
	}
}

func TestCompleteIntentFromDone(t *testing.T) {
	t.Parallel()
	ms := newIntentMockStore()
	ms.nodes[10] = &Node{ID: 10, Kind: KindIntent, State: "done"}

	reg := NewRegistry()
	RegisterIntentTools(reg, ms, 1)

	tool, _ := reg.Lookup("complete_intent")
	r, err := tool.Run(testSessionCtx, json.RawMessage(`{"node_id": 10}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for completing a done intent")
	}
}

func TestCancelIntentFromOpen(t *testing.T) {
	t.Parallel()
	ms := newIntentMockStore()
	ms.nodes[10] = &Node{ID: 10, Kind: KindIntent, State: "open"}

	reg := NewRegistry()
	RegisterIntentTools(reg, ms, 1)

	tool, _ := reg.Lookup("cancel_intent")
	r, err := tool.Run(testSessionCtx, json.RawMessage(`{"node_id": 10, "reason": "no longer needed"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
	if ms.nodes[10].State != "stopped" {
		t.Errorf("expected state stopped, got %s", ms.nodes[10].State)
	}
}

func TestCancelIntentFromRunning(t *testing.T) {
	t.Parallel()
	ms := newIntentMockStore()
	ms.nodes[10] = &Node{ID: 10, Kind: KindIntent, State: "running"}

	reg := NewRegistry()
	RegisterIntentTools(reg, ms, 1)

	tool, _ := reg.Lookup("cancel_intent")
	r, err := tool.Run(testSessionCtx, json.RawMessage(`{"node_id": 10}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
}

func TestCancelIntentClaimedByOtherSession(t *testing.T) {
	t.Parallel()
	ms := newIntentMockStore()
	ms.nodes[10] = &Node{ID: 10, Kind: KindIntent, State: "running"}
	ms.claims[10] = IntentClaim{NodeID: 10, Owner: "other", ExpiresAt: time.Now().Add(time.Hour)}

	reg := NewRegistry()
	RegisterIntentTools(reg, ms, 1)

	tool, _ := reg.Lookup("cancel_intent")
	ctx := WithSessionID(context.Background(), "me")
	r, err := tool.Run(ctx, json.RawMessage(`{"node_id": 10}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError || !strings.Contains(r.Content, "claimed by "+OwnerLabel("other")) {
		t.Fatalf("expected claim conflict, got: %s", r.Content)
	}
	if ms.nodes[10].State != "running" {
		t.Errorf("expected state running, got %s", ms.nodes[10].State)
	}
}

func TestCancelIntentExpiredClaim(t *testing.T) {
	t.Parallel()
	ms := newIntentMockStore()
	ms.nodes[10] = &Node{ID: 10, Kind: KindIntent, State: "running"}
	ms.claims[10] = IntentClaim{NodeID: 10, Owner: "other", ExpiresAt: time.Now().Add(-time.Minute)}

	reg := NewRegistry()
	RegisterIntentTools(reg, ms, 1)

	tool, _ := reg.Lookup("cancel_intent")
	ctx := WithSessionID(context.Background(), "me")
	r, err := tool.Run(ctx, json.RawMessage(`{"node_id": 10}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsError {
		t.Fatalf("expected success, got error: %s", r.Content)
	}
	if ms.nodes[10].State != "stopped" {
		t.Errorf("expected state stopped, got %s", ms.nodes[10].State)
	}
}

func TestCancelIntentAlreadyDone(t *testing.T) {
	t.Parallel()
	ms := newIntentMockStore()
	ms.nodes[10] = &Node{ID: 10, Kind: KindIntent, State: "done"}

	reg := NewRegistry()
	RegisterIntentTools(reg, ms, 1)

	tool, _ := reg.Lookup("cancel_intent")
	r, err := tool.Run(testSessionCtx, json.RawMessage(`{"node_id": 10}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for cancelling a done intent")
	}
	if !strings.Contains(r.Content, "already") {
		t.Errorf("expected 'already' in error, got: %s", r.Content)
	}
}

func TestStartIntentAlreadyRunning(t *testing.T) {
	t.Parallel()
	ms := newIntentMockStore()
	ms.nodes[10] = &Node{ID: 10, Kind: KindIntent, State: "running"}
	ms.claims[10] = IntentClaim{NodeID: 10, Owner: "other", ExpiresAt: time.Now().Add(time.Minute)}

	reg := NewRegistry()
	RegisterIntentTools(reg, ms, 1)

	tool, _ := reg.Lookup("start_intent")
	r, err := tool.Run(testSessionCtx, json.RawMessage(`{"node_id": 10}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for starting an intent claimed by another session")
	}
	if !strings.Contains(r.Content, "claimed by "+OwnerLabel("other")) {
		t.Errorf("expected holder in error, got: %s", r.Content)
	}
}

func TestStartIntentFromCancelled(t *testing.T) {
	t.Parallel()
	ms := newIntentMockStore()
	ms.nodes[10] = &Node{ID: 10, Kind: KindIntent, State: "stopped"}

	reg := NewRegistry()
	RegisterIntentTools(reg, ms, 1)

	tool, _ := reg.Lookup("start_intent")
	r, err := tool.Run(testSessionCtx, json.RawMessage(`{"node_id": 10}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for starting a cancelled intent")
	}
	if !strings.Contains(r.Content, "must be open") {
		t.Errorf("expected 'must be open' in error, got: %s", r.Content)
	}
}

func TestCompleteIntentFromStopped(t *testing.T) {
	t.Parallel()
	ms := newIntentMockStore()
	ms.nodes[10] = &Node{ID: 10, Kind: KindIntent, State: "stopped"}

	reg := NewRegistry()
	RegisterIntentTools(reg, ms, 1)

	tool, _ := reg.Lookup("complete_intent")
	r, err := tool.Run(testSessionCtx, json.RawMessage(`{"node_id": 10}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for completing a cancelled intent")
	}
	if !strings.Contains(r.Content, "must be running or open") {
		t.Errorf("expected 'must be running or open' in error, got: %s", r.Content)
	}
}

func TestCancelIntentAlreadyStopped(t *testing.T) {
	t.Parallel()
	ms := newIntentMockStore()
	ms.nodes[10] = &Node{ID: 10, Kind: KindIntent, State: "stopped"}

	reg := NewRegistry()
	RegisterIntentTools(reg, ms, 1)

	tool, _ := reg.Lookup("cancel_intent")
	r, err := tool.Run(testSessionCtx, json.RawMessage(`{"node_id": 10}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsError {
		t.Fatal("expected error for cancelling an already-stopped intent")
	}
	if !strings.Contains(r.Content, "already") {
		t.Errorf("expected 'already' in error, got: %s", r.Content)
	}
}

func TestIntentOpenToCancelledLifecycle(t *testing.T) {
	t.Parallel()
	ms := newIntentMockStore()
	ms.nodes[10] = &Node{ID: 10, Kind: KindIntent, State: "open"}

	reg := NewRegistry()
	RegisterIntentTools(reg, ms, 1)

	cancel, _ := reg.Lookup("cancel_intent")
	r, _ := cancel.Run(testSessionCtx, json.RawMessage(`{"node_id": 10, "reason": "obsolete"}`))
	if r.IsError {
		t.Fatalf("cancel from open failed: %s", r.Content)
	}
	if ms.nodes[10].State != "stopped" {
		t.Fatalf("expected stopped, got %s", ms.nodes[10].State)
	}

	start, _ := reg.Lookup("start_intent")
	r, _ = start.Run(testSessionCtx, json.RawMessage(`{"node_id": 10}`))
	if !r.IsError {
		t.Fatal("expected error for starting a cancelled intent")
	}
}

func TestIntentRunningToCancelledLifecycle(t *testing.T) {
	t.Parallel()
	ms := newIntentMockStore()
	ms.nodes[10] = &Node{ID: 10, Kind: KindIntent, State: "open"}

	reg := NewRegistry()
	RegisterIntentTools(reg, ms, 1)

	start, _ := reg.Lookup("start_intent")
	cancel, _ := reg.Lookup("cancel_intent")
	complete, _ := reg.Lookup("complete_intent")

	r, _ := start.Run(testSessionCtx, json.RawMessage(`{"node_id": 10}`))
	if r.IsError {
		t.Fatalf("start failed: %s", r.Content)
	}

	r, _ = cancel.Run(testSessionCtx, json.RawMessage(`{"node_id": 10, "reason": "pivoting"}`))
	if r.IsError {
		t.Fatalf("cancel from running failed: %s", r.Content)
	}
	if ms.nodes[10].State != "stopped" {
		t.Fatalf("expected stopped, got %s", ms.nodes[10].State)
	}

	r, _ = complete.Run(testSessionCtx, json.RawMessage(`{"node_id": 10}`))
	if !r.IsError {
		t.Fatal("expected error for completing a cancelled intent")
	}
}

func TestIntentFullLifecycle(t *testing.T) {
	t.Parallel()
	ms := newIntentMockStore()
	ms.nodes[10] = &Node{ID: 10, Kind: KindIntent, State: "open"}

	reg := NewRegistry()
	RegisterIntentTools(reg, ms, 1)

	start, _ := reg.Lookup("start_intent")
	complete, _ := reg.Lookup("complete_intent")

	r, _ := start.Run(testSessionCtx, json.RawMessage(`{"node_id": 10}`))
	if r.IsError {
		t.Fatalf("start failed: %s", r.Content)
	}
	if ms.nodes[10].State != "running" {
		t.Fatalf("expected running, got %s", ms.nodes[10].State)
	}

	r, _ = complete.Run(testSessionCtx, json.RawMessage(`{"node_id": 10, "result": "done"}`))
	if r.IsError {
		t.Fatalf("complete failed: %s", r.Content)
	}
	if ms.nodes[10].State != "done" {
		t.Fatalf("expected done, got %s", ms.nodes[10].State)
	}
}

type staleDetailStore struct {
	*intentMockStore
	state string
}

func (m staleDetailStore) NodeDetail(ctx context.Context, id int64) (*Node, error) {
	n, err := m.intentMockStore.NodeDetail(ctx, id)
	if n == nil || err != nil {
		return n, err
	}
	stale := *n
	stale.State = m.state
	return &stale, nil
}

func TestCompleteOpenIntentClaimsAndRecordsCompleter(t *testing.T) {
	t.Parallel()
	ms := newIntentMockStore()
	ms.nodes[10] = &Node{ID: 10, Kind: KindIntent, State: "open"}
	reg := NewRegistry()
	RegisterIntentTools(reg, ms, 1)
	complete, _ := reg.Lookup("complete_intent")

	r, _ := complete.Run(WithSessionID(context.Background(), "sess-a"), json.RawMessage(`{"node_id": 10, "result": "nothing there"}`))
	if r.IsError {
		t.Fatalf("complete open intent: %s", r.Content)
	}
	if !strings.HasPrefix(r.Content, "intent 10 marked done") || !strings.Contains(r.Content, "start_intent before working") {
		t.Errorf("result = %q, want done with a start_intent reminder", r.Content)
	}
	n := ms.nodes[10]
	if n.State != "done" || n.Payload["completed_by"] != OwnerLabel("sess-a") || n.Payload["result"] != "nothing there" {
		t.Errorf("node = %+v, want done and completed_by sess-a's label", n)
	}
	if _, held := ms.claims[10]; held {
		t.Error("claim left in place")
	}
}

func TestCompleteStartedIntentRecordsCompleter(t *testing.T) {
	t.Parallel()
	ms := newIntentMockStore()
	ms.nodes[10] = &Node{ID: 10, Kind: KindIntent, State: "open"}
	reg := NewRegistry()
	RegisterIntentTools(reg, ms, 1)
	start, _ := reg.Lookup("start_intent")
	complete, _ := reg.Lookup("complete_intent")
	ctx := WithSessionID(context.Background(), "sess-a")

	if r, _ := start.Run(ctx, json.RawMessage(`{"node_id": 10}`)); r.IsError {
		t.Fatalf("start: %s", r.Content)
	}
	r, _ := complete.Run(ctx, json.RawMessage(`{"node_id": 10}`))
	if r.IsError || r.Content != "intent 10 marked done" {
		t.Errorf("result = %q, want plain done", r.Content)
	}
	if ms.nodes[10].Payload["completed_by"] != OwnerLabel("sess-a") {
		t.Errorf("payload = %v", ms.nodes[10].Payload)
	}
}

func TestCloseOpenIntentRefusedWhenClaimedMeanwhile(t *testing.T) {
	t.Parallel()
	for _, tool := range []string{"complete_intent", "cancel_intent"} {
		ms := newIntentMockStore()
		ms.nodes[10] = &Node{ID: 10, Kind: KindIntent, State: "running"}
		ms.claims[10] = IntentClaim{NodeID: 10, Owner: "sess-b", ExpiresAt: time.Now().Add(time.Hour)}
		reg := NewRegistry()
		RegisterIntentTools(reg, staleDetailStore{ms, "open"}, 1)
		closer, _ := reg.Lookup(tool)

		r, _ := closer.Run(WithSessionID(context.Background(), "sess-a"), json.RawMessage(`{"node_id": 10}`))
		if !r.IsError || !strings.Contains(r.Content, "claimed by "+OwnerLabel("sess-b")) {
			t.Errorf("%s = %q, want claim conflict", tool, r.Content)
		}
		if ms.nodes[10].State != "running" || ms.claims[10].Owner != "sess-b" {
			t.Errorf("%s changed the intent: %s, %+v", tool, ms.nodes[10].State, ms.claims[10])
		}
	}
}

func TestCancelOpenIntentRecordsCanceller(t *testing.T) {
	t.Parallel()
	ms := newIntentMockStore()
	ms.nodes[10] = &Node{ID: 10, Kind: KindIntent, State: "open"}
	reg := NewRegistry()
	RegisterIntentTools(reg, ms, 1)
	cancel, _ := reg.Lookup("cancel_intent")

	if r, _ := cancel.Run(context.Background(), json.RawMessage(`{"node_id": 10}`)); !r.IsError || !strings.Contains(r.Content, "pass owner") {
		t.Errorf("cancel without session or owner = %q, want refusal", r.Content)
	}
	r, _ := cancel.Run(WithSessionID(context.Background(), "sess-a"), json.RawMessage(`{"node_id": 10, "reason": "duplicate"}`))
	if r.IsError || r.Content != "intent 10 cancelled" {
		t.Fatalf("cancel = %q", r.Content)
	}
	n := ms.nodes[10]
	if n.State != "stopped" || n.Payload["cancelled_by"] != OwnerLabel("sess-a") || n.Payload["cancel_reason"] != "duplicate" {
		t.Errorf("node = %+v", n)
	}
}
