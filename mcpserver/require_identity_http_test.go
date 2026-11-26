package mcpserver_test

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/SREsAreHumanToo/ARTEX-CC/identity"
	"github.com/SREsAreHumanToo/ARTEX-CC/mcpserver"
	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
)

// setRequireIdentity flips the process-wide setting. Tests using it must not
// call t.Parallel, so they finish before any parallel test reads it.
func setRequireIdentity(t *testing.T, on bool) {
	t.Helper()
	prev := tools.RequireIdentity
	tools.RequireIdentity = on
	t.Cleanup(func() { tools.RequireIdentity = prev })
}

type changesClaimStore struct {
	*claimStore
}

func (changesClaimStore) ListChanges(context.Context, int64, int64, tools.ChangeCursor, int) ([]tools.Change, error) {
	return nil, nil
}

func (changesClaimStore) ListClaims(context.Context, int64, string) ([]tools.HeldClaim, error) {
	return []tools.HeldClaim{}, nil
}

func (changesClaimStore) ListTargetChanges(context.Context, int64, bool) ([]tools.TargetChange, error) {
	return nil, nil
}

func TestRequireIdentityOverHTTP(t *testing.T) {
	setRequireIdentity(t, true)
	dir := t.TempDir()
	st := newIntentStore(10, 11, 12)
	reg := tools.NewRegistry()
	tools.RegisterIntentTools(reg, st, 1)
	tools.RegisterChangeTools(reg, changesClaimStore{st}, 1, 1)
	srv := mcpserver.New(reg)
	signer, err := identity.NewSigner(dir)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv.IdentityMiddleware(signer)(srv.StreamableHTTP()))
	t.Cleanup(ts.Close)
	url := ts.URL + "/mcp"

	const chat = "chat/0b7c3c4e-5d3f-4b8e-9a51-2f0c6d1e8a90"
	alice := identityHeader(issue(t, dir, "alice"))
	conv := identityHeader(issue(t, dir, chat))

	if out, isErr := modernTool(t, url, alice, "start_intent", map[string]any{"node_id": 10}); isErr || !strings.Contains(out, "claimed by alice") {
		t.Fatalf("alice claim = %q", out)
	}
	if out, isErr := modernTool(t, url, conv, "start_intent", map[string]any{"node_id": 11}); isErr || !strings.Contains(out, "claimed by "+chat) {
		t.Fatalf("chat claim = %q", out)
	}
	sid := legacyInit(t, url, alice)
	if out, isErr := legacyTool(t, url, sid, alice, "complete_intent", map[string]any{"node_id": 10}); isErr {
		t.Fatalf("legacy alice complete = %q", out)
	}

	anonSID := legacyInit(t, url, nil)
	refusals := map[string]func() (string, bool){
		"session start": func() (string, bool) {
			return legacyTool(t, url, anonSID, nil, "start_intent", map[string]any{"node_id": 12})
		},
		"session and owner start": func() (string, bool) {
			return legacyTool(t, url, anonSID, nil, "start_intent", map[string]any{"node_id": 12, "owner": "f00d"})
		},
		"owner start": func() (string, bool) {
			return modernTool(t, url, nil, "start_intent", map[string]any{"node_id": 12, "owner": "f00d"})
		},
		"owner cancel": func() (string, bool) {
			return modernTool(t, url, nil, "cancel_intent", map[string]any{"node_id": 11, "owner": "f00d"})
		},
		"session complete": func() (string, bool) {
			return legacyTool(t, url, anonSID, nil, "complete_intent", map[string]any{"node_id": 12})
		},
	}
	for name, call := range refusals {
		if out, isErr := call(); !isErr || out != tools.ErrIdentityRequired.Error() {
			t.Errorf("%s = %q, want ErrIdentityRequired", name, out)
		}
	}
	if st.owner(12) != "" || st.nodes[12].State != "open" || st.owner(11) != "id:"+chat {
		t.Fatalf("refused calls changed claims: 12 %q %s, 11 %q", st.owner(12), st.nodes[12].State, st.owner(11))
	}

	if out, isErr := legacyTool(t, url, anonSID, nil, "list_sessions", map[string]any{}); isErr || !strings.Contains(out, `"sessions"`) {
		t.Errorf("list_sessions without identity = %q", out)
	}
	const since = "2026-11-25T09:00:00Z"
	out, isErr := modernTool(t, url, nil, "list_changes", map[string]any{"since": since, "owner": "f00d"})
	var res map[string]any
	if isErr || json.Unmarshal([]byte(out), &res) != nil || res["claims_error"] != tools.ErrIdentityRequired.Error() || res["my_claims"] != nil {
		t.Errorf("list_changes without identity = %q, want changes with claims_error", out)
	}
	out, isErr = modernTool(t, url, conv, "list_changes", map[string]any{"since": since})
	res = nil
	if isErr || json.Unmarshal([]byte(out), &res) != nil || res["claims_error"] != nil || res["session"] != chat {
		t.Errorf("list_changes as chat identity = %q, want my_claims", out)
	}
}

func TestRequireIdentityOffKeepsAnonymousClaims(t *testing.T) {
	setRequireIdentity(t, false)
	st := newIntentStore(10, 11)
	url := startIdentityServer(t, st, t.TempDir())

	sid := legacyInit(t, url, nil)
	if out, isErr := legacyTool(t, url, sid, nil, "start_intent", map[string]any{"node_id": 10}); isErr || st.owner(10) != sid {
		t.Fatalf("session claim = %q, owner %q", out, st.owner(10))
	}
	if out, isErr := modernTool(t, url, nil, "start_intent", map[string]any{"node_id": 11, "owner": "f00d"}); isErr || st.owner(11) != "owner:f00d" {
		t.Fatalf("owner claim = %q, owner %q", out, st.owner(11))
	}
}

func TestOrchestrationGuideStatesRequireIdentity(t *testing.T) {
	store := &mockStore{rootGoal: "Goal"}
	read := func() string {
		addr := startTestServerWithOpts(t, tools.NewRegistry(), mcpserver.WithEngagement(store, 1, 100))
		_, rr := doRPC(t, addr, initSession(t, addr), 2, "resources/read", map[string]any{"uri": "artex://engagement"})
		var result struct {
			Contents []struct {
				Text string `json:"text"`
			} `json:"contents"`
		}
		if rr.Error != nil || json.Unmarshal(rr.Result, &result) != nil || len(result.Contents) == 0 {
			t.Fatalf("resources/read: %+v", rr)
		}
		return result.Contents[0].Text
	}

	setRequireIdentity(t, false)
	if strings.Contains(read(), "Identity required") {
		t.Error("guide mentions the identity requirement with the setting off")
	}
	setRequireIdentity(t, true)
	if text := read(); !strings.Contains(text, "## Identity required for claims") || !strings.Contains(text, "artex-cc identity issue") {
		t.Errorf("guide does not state the identity requirement:\n%s", text)
	}
}
