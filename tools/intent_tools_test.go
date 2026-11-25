package tools

import (
	"context"
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

func (m *intentMockStore) UpdateNodeFields(_ context.Context, id int64, state string, _ map[string]any) error {
	n, ok := m.nodes[id]
	if !ok {
		return fmt.Errorf("node %d not found", id)
	}
	n.State = state
	delete(m.claims, id)
	return nil
}

var testSessionCtx = WithSessionID(context.Background(), "sess-test")

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
	}
	for _, c := range cases {
		got, err := ClaimOwner(c.ctx, c.owner)
		if err != nil || got != c.want {
			t.Errorf("ClaimOwner(%q, %q) = %q, %v; want %q", SessionID(c.ctx), c.owner, got, err, c.want)
		}
	}
	if _, err := ClaimOwner(bare, "  "); !errors.Is(err, ErrNoClaimOwner) {
		t.Errorf("ClaimOwner with neither = %v, want ErrNoClaimOwner", err)
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
