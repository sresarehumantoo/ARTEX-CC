package api

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestInterceptRulesCRUD(t *testing.T) {
	a := New(&mockStore{}, nil, 1, 1, WithDataDir(t.TempDir()))
	srv := httptest.NewServer(a.Handler())
	defer srv.Close()

	code, body := agentsDo(t, srv, "GET", "/api/intercept/rules", "")
	var rules []InterceptRule
	if err := json.Unmarshal(body, &rules); err != nil || code != 200 || len(rules) != 3 {
		t.Fatalf("seed: code=%d len=%d err=%v", code, len(rules), err)
	}
	if rules[0].Pattern != "Bash" || rules[0].Action != "ask" || !rules[0].Enabled {
		t.Fatalf("first rule = %+v", rules[0])
	}

	code, body = agentsDo(t, srv, "POST", "/api/intercept/rules", `{"pattern":"Write","action":"bogus"}`)
	if code != 400 {
		t.Fatalf("bad action: code=%d body=%s", code, body)
	}

	code, body = agentsDo(t, srv, "POST", "/api/intercept/rules", `{"pattern":"Write","action":"ask","description":"d"}`)
	var created InterceptRule
	if err := json.Unmarshal(body, &created); err != nil || code != 201 || created.Scope != "tool_name" || !created.Enabled {
		t.Fatalf("create: code=%d %s", code, body)
	}

	code, body = agentsDo(t, srv, "PATCH", "/api/intercept/rules/"+created.ID, `{"enabled":false,"action":"deny"}`)
	var updated InterceptRule
	if err := json.Unmarshal(body, &updated); err != nil || code != 200 || updated.Enabled || updated.Action != "deny" {
		t.Fatalf("update: code=%d %s", code, body)
	}

	if code, _ = agentsDo(t, srv, "DELETE", "/api/intercept/rules/"+created.ID, ""); code != 204 {
		t.Fatalf("delete: code=%d", code)
	}
	if code, _ = agentsDo(t, srv, "DELETE", "/api/intercept/rules/"+created.ID, ""); code != 404 {
		t.Fatalf("delete again: code=%d", code)
	}
}

func TestApprovalsDecide(t *testing.T) {
	a := New(&mockStore{}, nil, 1, 1, WithDataDir(t.TempDir()))
	srv := httptest.NewServer(a.Handler())
	defer srv.Close()

	seed := []Approval{
		{ID: "a1", ToolName: "Bash", ToolInput: json.RawMessage(`{"command":"ls"}`), Status: "pending"},
		{ID: "a2", ToolName: "Bash", ToolInput: json.RawMessage(`{}`), Status: "approved"},
	}
	if err := a.files.WriteJSON(approvalsPath, seed); err != nil {
		t.Fatal(err)
	}

	_, body := agentsDo(t, srv, "GET", "/api/approvals?status=pending", "")
	var list []Approval
	if err := json.Unmarshal(body, &list); err != nil || len(list) != 1 || list[0].ID != "a1" {
		t.Fatalf("filter: %s", body)
	}

	ch, cancel := a.hub.Subscribe(approvalsTopic)
	defer cancel()

	if code, body := agentsDo(t, srv, "POST", "/api/approvals/a1/decide", `{"action":"maybe"}`); code != 400 {
		t.Fatalf("bad action: code=%d %s", code, body)
	}
	code, body := agentsDo(t, srv, "POST", "/api/approvals/a1/decide", `{"action":"approve"}`)
	var got Approval
	if err := json.Unmarshal(body, &got); err != nil || code != 200 || got.Status != "approved" || got.DecidedAt == "" {
		t.Fatalf("decide: code=%d %s", code, body)
	}
	select {
	case <-ch:
	default:
		t.Fatal("no hub event published")
	}
	if code, _ = agentsDo(t, srv, "POST", "/api/approvals/a1/decide", `{"action":"deny"}`); code != 409 {
		t.Fatalf("redecide: code=%d", code)
	}
	if code, _ = agentsDo(t, srv, "POST", "/api/approvals/nope/decide", `{"action":"deny"}`); code != 404 {
		t.Fatalf("missing: code=%d", code)
	}
}
