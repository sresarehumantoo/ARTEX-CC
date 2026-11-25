package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

// setRequireIdentity flips the process-wide setting. Tests using it must not
// call t.Parallel, so they finish before any parallel test reads it.
func setRequireIdentity(t *testing.T, on bool) {
	t.Helper()
	prev := RequireIdentity
	RequireIdentity = on
	t.Cleanup(func() { RequireIdentity = prev })
}

func TestClaimOwnerRequireIdentity(t *testing.T) {
	sess := WithSessionID(context.Background(), "sess-1")
	ident := WithIdentity(sess, "alice")

	setRequireIdentity(t, false)
	if got, err := ClaimOwner(sess, ""); err != nil || got != "sess-1" {
		t.Fatalf("off, session only = %q, %v", got, err)
	}
	if got, err := ClaimOwner(context.Background(), "f00d"); err != nil || got != "owner:f00d" {
		t.Fatalf("off, owner only = %q, %v", got, err)
	}

	setRequireIdentity(t, true)
	for name, c := range map[string]struct {
		ctx   context.Context
		owner string
	}{
		"session":           {sess, ""},
		"session and owner": {sess, "f00d"},
		"owner":             {context.Background(), "f00d"},
		"nothing":           {context.Background(), ""},
	} {
		if got, err := ClaimOwner(c.ctx, c.owner); !errors.Is(err, ErrIdentityRequired) {
			t.Errorf("%s: %q, %v, want ErrIdentityRequired", name, got, err)
		}
	}
	if got, err := ClaimOwner(ident, ""); err != nil || got != "id:alice" {
		t.Errorf("identity = %q, %v", got, err)
	}
	if got, err := ClaimOwner(ident, "sub"); err != nil || got != "id:alice/owner:sub" {
		t.Errorf("identity and owner = %q, %v", got, err)
	}
	if !strings.Contains(ErrIdentityRequired.Error(), "X-Artex-Identity") || !strings.Contains(ErrIdentityRequired.Error(), "artex-cc identity issue") {
		t.Errorf("error does not say how to fix it: %s", ErrIdentityRequired)
	}
}

func TestClaimToolsRequireIdentity(t *testing.T) {
	setRequireIdentity(t, true)
	ms := newIntentMockStore()
	ms.nodes[10] = &Node{ID: 10, Kind: KindIntent, State: "open"}
	ms.nodes[11] = &Node{ID: 11, Kind: KindIntent, State: "open"}
	ms.nodes[12] = &Node{ID: 12, Kind: KindIntent, State: "running"}
	reg := NewRegistry()
	RegisterIntentTools(reg, ms, 1)

	start, _ := reg.Lookup("start_intent")
	if !strings.Contains(start.Description, "requires a verified X-Artex-Identity") {
		t.Errorf("start_intent description does not mention the requirement: %s", start.Description)
	}

	for _, c := range []struct {
		tool string
		id   int
	}{
		{"start_intent", 10}, {"complete_intent", 11}, {"cancel_intent", 11},
		{"complete_intent", 12}, {"cancel_intent", 12},
	} {
		tool, _ := reg.Lookup(c.tool)
		r, _ := tool.Run(testSessionCtx, json.RawMessage(fmt.Sprintf(`{"node_id": %d, "owner": "f00d"}`, c.id)))
		if !r.IsError || r.Content != ErrIdentityRequired.Error() {
			t.Errorf("%s on %d with session and owner = %q, want ErrIdentityRequired", c.tool, c.id, r.Content)
		}
	}
	if ms.nodes[10].State != "open" || ms.nodes[11].State != "open" || ms.nodes[12].State != "running" || len(ms.claims) != 0 {
		t.Fatalf("refused calls changed state: %s %s %s, claims %v", ms.nodes[10].State, ms.nodes[11].State, ms.nodes[12].State, ms.claims)
	}

	ctx := WithIdentity(context.Background(), "alice")
	if r, _ := start.Run(ctx, json.RawMessage(`{"node_id": 10}`)); r.IsError || !strings.Contains(r.Content, "claimed by alice") {
		t.Fatalf("start with identity = %q", r.Content)
	}
	complete, _ := reg.Lookup("complete_intent")
	if r, _ := complete.Run(ctx, json.RawMessage(`{"node_id": 10}`)); r.IsError {
		t.Fatalf("complete with identity = %q", r.Content)
	}
	cancel, _ := reg.Lookup("cancel_intent")
	if r, _ := cancel.Run(ctx, json.RawMessage(`{"node_id": 11}`)); r.IsError {
		t.Fatalf("cancel with identity = %q", r.Content)
	}
}

func TestStartIntentDescriptionWithoutRequireIdentity(t *testing.T) {
	setRequireIdentity(t, false)
	if d := makeStartIntent(newIntentMockStore()).Description; strings.Contains(d, "X-Artex-Identity") {
		t.Errorf("description mentions identity with the setting off: %s", d)
	}
}

func TestListChangesRequireIdentity(t *testing.T) {
	setRequireIdentity(t, true)
	m := &changesMockStore{changes: []Change{{Kind: "fact", Fields: map[string]any{"id": 1}}}}

	_, out := runListChanges(t, m, testSessionCtx, `{"since": "2026-11-25T09:00:00Z", "owner": "f00d"}`)
	if out["claims_error"] != ErrIdentityRequired.Error() {
		t.Errorf("claims_error = %v, want ErrIdentityRequired", out["claims_error"])
	}
	if _, ok := out["my_claims"]; ok {
		t.Errorf("my_claims returned without an identity: %v", out)
	}
	if out["count"] != float64(1) {
		t.Errorf("changes not returned without an identity: %v", out)
	}

	_, out = runListChanges(t, m, WithIdentity(context.Background(), "alice"), `{"since": "2026-11-25T09:00:00Z"}`)
	if _, ok := out["my_claims"]; !ok || m.gotOwner != "id:alice" || out["session"] != "alice" {
		t.Errorf("identity list_changes = %v (owner %q)", out, m.gotOwner)
	}
}
