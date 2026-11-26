package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

type renewCall struct {
	ids    []int64
	holder string
	lease  time.Duration
}

type renewRecorder struct {
	*explorationMockStore
	calls []renewCall
}

func (m *renewRecorder) RenewIntentClaims(_ context.Context, ids []int64, holder string, lease time.Duration) error {
	m.calls = append(m.calls, renewCall{ids, holder, lease})
	return nil
}

func newRenewRecorder() *renewRecorder {
	ms := newExplorationMockStore()
	ms.nodeMap[7] = &Node{ID: 7, Kind: KindIntent, State: "running"}
	return &renewRecorder{explorationMockStore: ms}
}

func TestWritesRenewTheCallersClaim(t *testing.T) {
	t.Parallel()
	cases := []struct {
		tool string
		in   string
		want string
	}{
		{"record_fact", `{"facts": [{"summary": "a", "intent_id": 7}, {"summary": "b"}, {"summary": "c", "intent_id": "7"}]}`, "[7 7]"},
		{"record_fact", `{"facts": [{"summary": "a"}]}`, ""},
		{"record_fact", `{"facts": [{"summary": "a", "intent_id": 99}]}`, ""},
		{"report_finding", `{"vulnclass": "sqli", "severity": "high", "summary": "x", "intent_id": 7}`, "[7]"},
		{"report_finding", `{"vulnclass": "sqli", "severity": "high", "summary": "x"}`, ""},
		{"record_coverage", `{"surface": "/login", "risk_area": "sqli", "outcome": "ruled_out", "reason": "no db", "intent_id": 7}`, "[7]"},
	}
	for _, c := range cases {
		ms := newRenewRecorder()
		reg := NewRegistry()
		RegisterAll(reg, ms, 1, 1)
		tool, ok := reg.Lookup(c.tool)
		if !ok {
			t.Fatalf("%s not registered", c.tool)
		}

		r, _ := tool.Run(WithSessionID(context.Background(), "sess-a"), json.RawMessage(c.in))
		if r.IsError && c.want != "" {
			t.Fatalf("%s %s: %s", c.tool, c.in, r.Content)
		}
		if c.want == "" {
			if len(ms.calls) != 0 {
				t.Errorf("%s %s renewed %+v, want nothing", c.tool, c.in, ms.calls)
			}
			continue
		}
		if len(ms.calls) != 1 {
			t.Fatalf("%s %s: %d renewals, want 1", c.tool, c.in, len(ms.calls))
		}
		got := ms.calls[0]
		if fmt.Sprint(got.ids) != c.want || got.holder != "sess-a" || got.lease != IntentLease {
			t.Errorf("%s renewed %+v, want %s for sess-a over %v", c.tool, got, c.want, IntentLease)
		}
	}
}

func TestWritesWithoutHolderDoNotRenew(t *testing.T) {
	t.Parallel()
	ms := newRenewRecorder()
	reg := NewRegistry()
	RegisterAll(reg, ms, 1, 1)
	tool, _ := reg.Lookup("record_fact")

	r, _ := tool.Run(context.Background(), json.RawMessage(`{"facts": [{"summary": "a", "intent_id": 7}]}`))
	if r.IsError {
		t.Fatalf("record_fact without a session: %s", r.Content)
	}
	if len(ms.calls) != 0 {
		t.Errorf("renewed %+v without a holder", ms.calls)
	}

	r, _ = tool.Run(WithIdentity(context.Background(), "alice"), json.RawMessage(`{"facts": [{"summary": "a", "intent_id": 7}]}`))
	if r.IsError || len(ms.calls) != 1 || ms.calls[0].holder != "id:alice" {
		t.Errorf("identity renewal = %+v (%s), want holder id:alice", ms.calls, r.Content)
	}
}

func TestWritesRenewTheOwnerTheyName(t *testing.T) {
	t.Parallel()
	cases := []struct {
		tool string
		in   string
	}{
		{"record_fact", `{"facts": [{"summary": "a", "intent_id": 7}], "owner": " x "}`},
		{"report_finding", `{"vulnclass": "sqli", "severity": "high", "summary": "x", "intent_id": 7, "owner": "x"}`},
		{"record_coverage", `{"surface": "/login", "risk_area": "sqli", "outcome": "ruled_out", "reason": "no db", "intent_id": 7, "owner": "x"}`},
	}
	ctxs := []struct {
		ctx  context.Context
		want string
	}{
		{context.Background(), "owner:x"},
		{WithSessionID(context.Background(), "sess-a"), "sess-a/owner:x"},
		{WithIdentity(context.Background(), "alice"), "id:alice/owner:x"},
	}
	for _, c := range cases {
		for _, cc := range ctxs {
			ms := newRenewRecorder()
			reg := NewRegistry()
			RegisterAll(reg, ms, 1, 1)
			tool, _ := reg.Lookup(c.tool)

			r, _ := tool.Run(cc.ctx, json.RawMessage(c.in))
			if r.IsError {
				t.Fatalf("%s %s: %s", c.tool, c.in, r.Content)
			}
			if len(ms.calls) != 1 || ms.calls[0].holder != cc.want || fmt.Sprint(ms.calls[0].ids) != "[7]" {
				t.Errorf("%s renewed %+v, want [7] for %s", c.tool, ms.calls, cc.want)
			}
		}
	}
}

func TestClaimTextsExplainRenewal(t *testing.T) {
	t.Parallel()
	reg := NewRegistry()
	RegisterIntentTools(reg, newIntentMockStore(), 1)
	start, _ := reg.Lookup("start_intent")
	if !strings.Contains(start.Description, "renews whenever you pass this intent as intent_id to record_fact") {
		t.Errorf("start_intent description = %q", start.Description)
	}
	msg := (&ClaimConflictError{NodeID: 3, Owner: "x", ExpiresAt: time.Now().Add(time.Minute)}).Error()
	if !strings.Contains(msg, "pick another intent") || !strings.Contains(msg, "renews while its holder records results") {
		t.Errorf("conflict = %q", msg)
	}
}
